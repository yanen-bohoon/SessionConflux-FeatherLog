"""Feishu Wiki + Docx API client.

Uploads session records as structured documents in a Feishu Wiki space,
organised as Computer → Agent → Session Documents.
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
        # Cache for folder node tokens: {(parent_token, name): node_token}
        self._folder_cache: dict[tuple[str, str], str] = {}
        self._space_id: str | None = None
        self._root_token: str | None = None

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

    # ── Request helpers ───────────────────────────────────────────────

    def _get(self, path: str, params: dict | None = None) -> dict:
        token = self._get_tenant_token()
        resp = self._client.get(
            f"{self.BASE_URL}{path}",
            headers={"Authorization": f"Bearer {token}"},
            params=params,
        )
        resp.raise_for_status()
        data = resp.json()
        if data.get("code") != 0:
            raise RuntimeError(f"API error [{path}]: {data}")
        return data.get("data", {})

    def _post(self, path: str, body: dict) -> dict:
        token = self._get_tenant_token()
        resp = self._client.post(
            f"{self.BASE_URL}{path}",
            headers={"Authorization": f"Bearer {token}",
                      "Content-Type": "application/json"},
            content=json.dumps(body),
        )
        resp.raise_for_status()
        data = resp.json()
        if data.get("code") != 0:
            raise RuntimeError(f"API error [{path}]: {data}")
        return data.get("data", {})

    def _retry_call(self, fn, max_retries: int = 3,
                    initial_delay: float = 1.0):
        """Call fn with retries and exponential backoff."""
        delay = initial_delay
        for attempt in range(max_retries):
            try:
                return fn()
            except Exception as e:
                logger.warning(f"Attempt {attempt + 1}/{max_retries} failed: {e}")
                if attempt < max_retries - 1:
                    time.sleep(delay)
                    delay *= 2
                    self._access_token = None  # Force token refresh
                else:
                    raise

    # ── Wiki space ────────────────────────────────────────────────────

    def _ensure_space(self):
        """Find the wiki space by name and cache its id and root token."""
        if self._space_id:
            return

        data = self._get("/wiki/v2/spaces", {"page_size": 50})
        spaces = data.get("items", [])

        target = self.config.wiki_space_name
        for space in spaces:
            if space.get("name") == target:
                self._space_id = space["space_id"]
                break
        else:
            raise RuntimeError(
                f"Wiki space '{target}' not found. "
                f"Available: {[s.get('name') for s in spaces]}"
            )

        # Get space root node token from settings
        settings = self._get(f"/wiki/v2/spaces/{self._space_id}/setting")
        self._root_token = settings.get("node_token") or self._space_id
        logger.info(f"Wiki space '{target}' (id={self._space_id}) ready")

    # ── Folder management ─────────────────────────────────────────────

    def _ensure_folder(self, parent_token: str, name: str) -> str:
        """Get or create a folder under parent_token. Returns node_token."""
        cache_key = (parent_token, name)
        cached = self._folder_cache.get(cache_key)
        if cached:
            return cached

        # Check if folder already exists
        data = self._get(
            "/wiki/v2/spaces/{}/nodes".format(self._space_id),
            {"page_size": 50, "parent_node_token": parent_token},
        )
        for item in data.get("items", []):
            if (item.get("obj_type") == 1  # Folder type
                    and item.get("title") == name):
                node_token = item["node_token"]
                self._folder_cache[cache_key] = node_token
                logger.info(f"Found existing folder '{name}' ({node_token[:12]}...)")
                return node_token

        # Create folder
        result = self._post(
            "/wiki/v2/spaces/{}/nodes".format(self._space_id),
            {
                "obj_type": 1,
                "parent_node_token": parent_token,
                "node_type": "origin",
                "title": name,
            },
        )
        node_token = result["node"]["node_token"]
        self._folder_cache[cache_key] = node_token
        logger.info(f"Created folder '{name}' ({node_token[:12]}...)")
        return node_token

    # ── Session document ──────────────────────────────────────────────

    def _create_document(self, parent_token: str, title: str) -> tuple[str, str]:
        """Create a wiki page (document). Returns (node_token, doc_token).

        The node_token is the wiki node identifier. The doc_token is the
        underlying document identifier (same as node_token in most cases).
        """
        result = self._post(
            "/wiki/v2/spaces/{}/nodes".format(self._space_id),
            {
                "obj_type": 2,
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

    def _append_blocks(self, doc_token: str, blocks: list[dict],
                       block_id: str | None = None):
        """Append content blocks to a document.

        block_id defaults to the document_id (root page block).
        """
        target_block = block_id or doc_token
        result = self._post(
            "/docx/v1/documents/{}/blocks/{}/children".format(
                doc_token, target_block),
            {"children": blocks},
        )
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

        * First call: creates a document in Computer → Agent folder
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
        self._ensure_space()

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
            logger.info(f"Appended {len(new_msgs)} msgs to session {session_id[:12]}...")
        else:
            # ── Create new document ──
            # Ensure folder hierarchy: Computer → Agent
            root = self._ensure_root()
            computer_folder = self._ensure_folder(root, computer)
            agent_folder = self._ensure_folder(computer_folder, agent.capitalize())

            # Create the document
            node_token, doc_token = self._create_document(agent_folder, title)

            # Add all message blocks
            all_blocks = []
            for msg in messages:
                all_blocks.extend(msg.to_block_dict())

            if all_blocks:
                self._append_blocks(doc_token, all_blocks)

            state_cache.mark_created(
                session_id, doc_token, node_token,
                agent, computer, title, len(messages),
            )
            logger.info(f"Created doc for session {session_id[:12]}... ({len(messages)} msgs)")

        return True

    def _ensure_root(self) -> str:
        """Return the wiki root node token, ensuring space is resolved."""
        self._ensure_space()
        return self._root_token

    # ── Lifecycle ─────────────────────────────────────────────────────

    def close(self):
        self._client.close()


# Scoped retry config helper
_retry_defaults = {"max_retries": 3, "initial_delay": 1.0}
