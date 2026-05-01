"""Feishu Wiki + Docx API client.

Uploads session records as structured documents in a Feishu Wiki space,
organised as Computer → Agent → Session Documents.

Uses tree-based hierarchy: container pages (computer/agent) can hold
child pages. All nodes use obj_type="docx".
"""

import logging
import time
import json
import httpx
from conflux.config import FeishuConfig
from conflux.models import generate_session_title

logger = logging.getLogger("conflux.feishu")


class FeishuWikiClient:
    """Client for Feishu Open API — Wiki + Docx operations."""

    BASE_URL = "https://open.feishu.cn/open-apis"

    def __init__(self, config: FeishuConfig):
        self.config = config
        self._access_token: str | None = None
        self._token_expires_at: float = 0
        self._client = httpx.Client(timeout=30)

        # Resolved and cached during sync
        self._space_id: str | None = None
        self._root_token: str | None = None
        # {(parent_token, name): node_token}
        self._page_cache: dict[tuple[str, str], str] = {}

    # ── Auth ──────────────────────────────────────────────────────────

    def _get_tenant_token(self) -> str:
        if self._access_token and time.time() < self._token_expires_at:
            return self._access_token

        resp = self._client.post(
            f"{self.BASE_URL}/auth/v3/tenant_access_token/internal",
            json={
                "app_id": self.config.app_id,
                "app_secret": self.config.app_secret,
            },
        )
        resp.raise_for_status()
        data = resp.json()

        if data.get("code") != 0:
            raise RuntimeError(f"Failed to get tenant token: {data}")

        self._access_token = data["tenant_access_token"]
        self._token_expires_at = time.time() + data.get("expire", 7200) - 300
        return self._access_token

    # ── Low-level HTTP helpers ────────────────────────────────────────

    def _get(self, path: str, params: dict | None = None) -> dict:
        token = self._get_tenant_token()
        resp = self._client.get(
            f"{self.BASE_URL}{path}",
            headers={"Authorization": f"Bearer {token}"},
            params=params,
        )
        # raise_for_status first, then check business code
        resp.raise_for_status()
        data = resp.json()
        code = data.get("code")
        if code not in (0, None):
            raise RuntimeError(f"API error [{path}]: code={code} msg={data.get('msg')}")
        return data.get("data", {})

    def _post(self, path: str, body: dict) -> dict:
        token = self._get_tenant_token()
        resp = self._client.post(
            f"{self.BASE_URL}{path}",
            headers={
                "Authorization": f"Bearer {token}",
                "Content-Type": "application/json",
            },
            content=json.dumps(body),
        )
        resp.raise_for_status()
        data = resp.json()
        code = data.get("code")
        if code not in (0, None):
            raise RuntimeError(f"API error [{path}]: code={code} msg={data.get('msg')}")
        return data.get("data", {})

    def _retry_call(self, fn, max_retries: int = 3,
                    initial_delay: float = 1.0):
        delay = initial_delay
        for attempt in range(max_retries):
            try:
                return fn()
            except Exception as e:
                logger.warning(f"Attempt {attempt + 1}/{max_retries} failed: {e}")
                if attempt < max_retries - 1:
                    time.sleep(delay)
                    delay *= 2
                    self._access_token = None
                else:
                    raise

    # ── Space resolution ──────────────────────────────────────────────

    def resolve_space(self, space_id: str, root_token: str | None = None):
        """Set the wiki space ID directly (avoids listing spaces).

        Args:
            space_id: The wiki space ID.
            root_token: A page token to use as root parent for the tree.
                         If None, the home page must be resolved separately.
        """
        self._space_id = space_id
        if root_token:
            self._root_token = root_token
        logger.info(f"Using wiki space_id={space_id}")

    def resolve_space_from_token(self, wiki_token: str) -> str:
        """Resolve space_id and root parent token from any page token.

        The root parent is the existing wiki page (usually the home page)
        under which new top-level containers are created.
        Uses wiki/v2/spaces/get_node which works with minimal permissions.
        """
        data = self._get("/wiki/v2/spaces/get_node", {"token": wiki_token})
        node = data.get("node", {})
        self._space_id = node["space_id"]
        self._root_token = node["node_token"]
        logger.info(
            f"Resolved space_id={self._space_id}, root={self._root_token[:12]}..."
        )
        return self._space_id

    def _space(self) -> str:
        if not self._space_id:
            raise RuntimeError(
                "Wiki space not resolved. Call resolve_space() or "
                "resolve_space_from_token() first."
            )
        return self._space_id

    # ── Container (folder) management ─────────────────────────────────

    def _ensure_container(self, parent_token: str, name: str) -> str:
        """Find or create a page container under parent_token.

        All nodes in Feishu Wiki are 'docx' pages; those with children
        act as folders in the tree view. Returns node_token.
        """
        cache_key = (parent_token, name)
        cached = self._page_cache.get(cache_key)
        if cached:
            return cached

        space_id = self._space()
        # Check existing children
        data = self._get(
            f"/wiki/v2/spaces/{space_id}/nodes",
            {"page_size": 50, "parent_node_token": parent_token},
        )
        for item in data.get("items", []):
            if item.get("title") == name and item.get("obj_type") == "docx":
                node_token = item["node_token"]
                self._page_cache[cache_key] = node_token
                logger.info(f"Found container '{name}' ({node_token[:12]}...)")
                return node_token

        # Create new page as container
        result = self._post(
            f"/wiki/v2/spaces/{space_id}/nodes",
            {
                "obj_type": "docx",
                "parent_node_token": parent_token,
                "node_type": "origin",
                "title": name,
            },
        )
        node = result["node"]
        node_token = node["node_token"]
        self._page_cache[cache_key] = node_token
        logger.info(f"Created container '{name}' ({node_token[:12]}...)")
        return node_token

    # ── Session document ──────────────────────────────────────────────

    def _create_document(self, parent_token: str, title: str) -> tuple[str, str]:
        """Create a wiki page (document). Returns (node_token, doc_token)."""
        space_id = self._space()
        result = self._post(
            f"/wiki/v2/spaces/{space_id}/nodes",
            {
                "obj_type": "docx",
                "parent_node_token": parent_token,
                "node_type": "origin",
                "title": title,
            },
        )
        node = result["node"]
        node_token = node["node_token"]
        doc_token = node.get("obj_token", node_token)
        logger.info(f"Created document '{title}' ({doc_token[:12]}...)")
        return node_token, doc_token

    def _append_blocks(self, doc_token: str, blocks: list[dict]):
        """Append content blocks to a document."""
        target_block = doc_token
        result = self._post(
            f"/docx/v1/documents/{doc_token}/blocks/{target_block}/children",
            {"children": blocks},
        )
        # If the API returns items, use that count; otherwise assume all were added
        added = len(result.get("items", blocks))
        logger.debug(f"Appended {added} blocks to doc {doc_token[:12]}...")
        return added

    # ── Public sync method ────────────────────────────────────────────

    def sync_session(self, messages: list, session_id: str,
                     agent: str, computer: str,
                     session_state_cache,
                     max_retries: int = 3,
                     initial_delay: float = 1.0) -> bool:
        """Upload a session's messages to Feishu Wiki.

        * First call: creates a document in Computer → Agent container
        * Subsequent calls: appends new messages to the existing document

        Returns True on success.
        """
        if not messages:
            return True

        try:
            return self._retry_call(
                lambda: self._do_sync(messages, session_id,
                                      agent, computer, session_state_cache),
                max_retries=max_retries,
                initial_delay=initial_delay,
            )
        except Exception as e:
            logger.error(f"Failed to sync session {session_id[:12]}...: {e}")
            return False

    def _do_sync(self, messages, session_id, agent, computer, state_cache):
        space_id = self._space()
        root = self._root_token or space_id
        title = generate_session_title(messages)
        existing = state_cache.get(session_id)

        if existing:
            # ── Append new messages ──
            new_msgs = [
                m for m in messages
                if m.message_index > existing["last_message_index"]
            ]
            if not new_msgs:
                return True

            blocks = []
            for msg in new_msgs:
                blocks.extend(msg.to_block_dict())

            self._append_blocks(existing["doc_token"], blocks)
            state_cache.mark_appended(session_id, new_msgs[-1].message_index)
            logger.info(
                f"Appended {len(new_msgs)} msgs to session "
                f"{session_id[:12]}..."
            )
        else:
            # ── Create new document ──
            # Ensure container hierarchy: Computer → Agent
            computer_node = self._ensure_container(root, computer)
            agent_node = self._ensure_container(
                computer_node, agent.capitalize()
            )

            node_token, doc_token = self._create_document(agent_node, title)

            all_blocks = []
            for msg in messages:
                all_blocks.extend(msg.to_block_dict())

            if all_blocks:
                self._append_blocks(doc_token, all_blocks)

            state_cache.mark_created(
                session_id, doc_token, node_token,
                agent, computer, title, len(messages),
            )
            logger.info(
                f"Created doc for session {session_id[:12]}... "
                f"({len(messages)} msgs)"
            )

        return True

    # ── Lifecycle ─────────────────────────────────────────────────────

    def close(self):
        self._client.close()
