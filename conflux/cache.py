"""Session sync state cache.

Tracks which sessions have been uploaded to Feishu Wiki and the
upload progress (last message index) for incremental appends.
"""

import json
import os
import logging

logger = logging.getLogger("conflux.cache")


class SessionStateCache:
    """Persistent cache tracking the upload state of each session.

    Schema (saved to .conflux/session_state.json):
    {
      "<session_id>": {
        "doc_token": "<feishu doc token>",
        "node_token": "<wiki node token>",
        "last_message_index": <int>,
        "agent": "<agent name>",
        "computer": "<hostname>",
        "title": "<document title>",
        "created_at": <timestamp>,
        "updated_at": <timestamp>
      }
    }
    """

    def __init__(self, cache_path: str = ".conflux/session_state.json"):
        self.cache_path = cache_path
        self._state: dict = {}
        self._load()

    def _load(self):
        if os.path.exists(self.cache_path):
            try:
                with open(self.cache_path, "r", encoding="utf-8") as f:
                    self._state = json.load(f)
                logger.debug(f"Loaded {len(self._state)} session states")
            except (json.JSONDecodeError, IOError) as e:
                logger.warning(f"Failed to load session state: {e}")
                self._state = {}

    def _save(self):
        os.makedirs(os.path.dirname(self.cache_path), exist_ok=True)
        with open(self.cache_path, "w", encoding="utf-8") as f:
            json.dump(self._state, f, ensure_ascii=False, indent=2)

    def get(self, session_id: str) -> dict | None:
        """Get state for a session, or None if not tracked."""
        return self._state.get(session_id)

    def get_last_index(self, session_id: str) -> int:
        """Get the last uploaded message index for a session."""
        state = self._state.get(session_id)
        return state["last_message_index"] if state else -1

    def mark_created(self, session_id: str, doc_token: str, node_token: str,
                     agent: str, computer: str, title: str,
                     message_count: int):
        """Record that a new session document was created."""
        now = __import__("time").time()
        self._state[session_id] = {
            "doc_token": doc_token,
            "node_token": node_token,
            "last_message_index": message_count - 1,
            "agent": agent,
            "computer": computer,
            "title": title,
            "created_at": now,
            "updated_at": now,
        }
        self._save()
        logger.info(f"Session {session_id[:12]}... → doc {doc_token[:12]}... ({message_count} msgs)")

    def mark_appended(self, session_id: str, last_message_index: int):
        """Update the last uploaded message index for a session."""
        state = self._state.get(session_id)
        if state:
            state["last_message_index"] = last_message_index
            state["updated_at"] = __import__("time").time()
            self._save()
            logger.debug(f"Session {session_id[:12]}... → index {last_message_index}")

    def has_new_messages(self, session_id: str, total_count: int) -> bool:
        """Check if a session has messages not yet uploaded."""
        state = self._state.get(session_id)
        if state is None:
            return total_count > 0
        return total_count > state["last_message_index"] + 1

    def needs_upload(self, session_id: str, message_index: int) -> bool:
        """Check if a specific message index needs uploading."""
        state = self._state.get(session_id)
        if state is None:
            return True
        return message_index > state["last_message_index"]

    def all_sessions(self) -> dict:
        """Return all tracked sessions."""
        return dict(self._state)

    def clear(self):
        """Clear all state (for testing)."""
        self._state = {}
        self._save()
