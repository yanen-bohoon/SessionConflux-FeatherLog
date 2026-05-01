"""Codex CLI session parser."""

import json
import logging
import re
from datetime import datetime, timezone
from pathlib import Path
from ..models import UnifiedMessage
from .base import ParserBase

logger = logging.getLogger("conflux.parsers.codex")


class CodexParser(ParserBase):
    """Parse Codex CLI JSONL session files.
    
    Format: ~/.codex/sessions/YYYY/MM/DD/rollout-{timestamp}-{sessionId}.jsonl
    Each line is a JSON object. Key types:
    - type=session_meta: session metadata (first line, extract session info)
    - type=event_msg: internal events (agent_message, user_message, tool_call, etc.)
    - type=response_item: OpenAI response stream (message with role=user/assistant/developer)
    - type=turn_context: per-turn config (model, policies, etc.)
    """

    @property
    def name(self) -> str:
        return "codex"

    def find_sessions(self, base_path: str) -> list[Path]:
        """Find all .jsonl files under the sessions directory."""
        base = Path(base_path)
        if not base.exists():
            logger.warning(f"Codex sessions path not found: {base_path}")
            return []
        
        sessions = []
        for jsonl_file in base.rglob("*.jsonl"):
            if jsonl_file.is_file():
                sessions.append(jsonl_file)
        
        return sorted(sessions)

    def parse_session(self, file_path: Path, hostname: str) -> list[UnifiedMessage]:
        """Parse a Codex JSONL file into UnifiedMessages."""
        messages: list[UnifiedMessage] = []
        msg_index = 0
        session_id = ""
        cwd = ""

        # Extract session ID from filename: rollout-{timestamp}-{sessionId}.jsonl
        filename_match = re.search(r"rollout-[^-]+-([a-f0-9-]+)\.jsonl$", file_path.name)
        if filename_match:
            session_id = filename_match.group(1)

        try:
            with open(file_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue

                    try:
                        event = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    event_type = event.get("type", "")

                    # Extract session metadata
                    if event_type == "session_meta":
                        payload = event.get("payload", {})
                        session_id = payload.get("id", session_id)
                        cwd = payload.get("cwd", "")
                        continue

                    # Skip turn_context, token_count, etc.
                    if event_type in ("turn_context",):
                        continue

                    # Process event_msg (preferred - human-readable)
                    if event_type == "event_msg":
                        payload = event.get("payload", {})
                        sub_type = payload.get("type", "")

                        if sub_type == "agent_message":
                            msg = payload.get("message", "")
                            phase = payload.get("phase", "")
                            # Prefer final_answer over commentary to reduce redundancy
                            if msg and phase == "final_answer":
                                timestamp = self._extract_timestamp(event)
                                messages.append(UnifiedMessage(
                                    timestamp=timestamp,
                                    source_agent=self.name,
                                    session_id=session_id,
                                    message_index=msg_index,
                                    role="assistant",
                                    content=msg,
                                    source_computer=hostname,
                                    project_tag=self._extract_project_tag(cwd),
                                ))
                                msg_index += 1
                            elif msg and phase == "commentary":
                                # Include commentary as it provides the thinking process
                                timestamp = self._extract_timestamp(event)
                                messages.append(UnifiedMessage(
                                    timestamp=timestamp,
                                    source_agent=self.name,
                                    session_id=session_id,
                                    message_index=msg_index,
                                    role="assistant",
                                    content=msg,
                                    source_computer=hostname,
                                    project_tag=self._extract_project_tag(cwd),
                                ))
                                msg_index += 1

                        elif sub_type == "user_message":
                            msg = payload.get("message", "")
                            if msg:
                                timestamp = self._extract_timestamp(event)
                                messages.append(UnifiedMessage(
                                    timestamp=timestamp,
                                    source_agent=self.name,
                                    session_id=session_id,
                                    message_index=msg_index,
                                    role="user",
                                    content=msg,
                                    source_computer=hostname,
                                    project_tag=self._extract_project_tag(cwd),
                                ))
                                msg_index += 1

                        elif sub_type == "tool_call":
                            tool_name = payload.get("tool_name", "")
                            status = payload.get("status", "")
                            if tool_name and status == "completed":
                                timestamp = self._extract_timestamp(event)
                                messages.append(UnifiedMessage(
                                    timestamp=timestamp,
                                    source_agent=self.name,
                                    session_id=session_id,
                                    message_index=msg_index,
                                    role="assistant",
                                    content=f"[工具调用: {tool_name}]",
                                    source_computer=hostname,
                                    project_tag=self._extract_project_tag(cwd),
                                ))
                                msg_index += 1

                    # Fallback: process response_item for messages not covered by event_msg
                    elif event_type == "response_item":
                        payload = event.get("payload", {})
                        role = payload.get("role", "")
                        if role in ("user", "assistant"):
                            content_list = payload.get("content", [])
                            for item in content_list:
                                if isinstance(item, dict):
                                    ct = item.get("type", "")
                                    if ct in ("input_text", "output_text"):
                                        text = item.get("text", "")
                                        if text:
                                            timestamp = self._extract_timestamp(event)
                                            messages.append(UnifiedMessage(
                                                timestamp=timestamp,
                                                source_agent=self.name,
                                                session_id=session_id,
                                                message_index=msg_index,
                                                role=role,
                                                content=text,
                                                source_computer=hostname,
                                                project_tag=self._extract_project_tag(cwd),
                                            ))
                                            msg_index += 1

        except IOError as e:
            logger.error(f"Failed to read {file_path}: {e}")

        logger.debug(f"Parsed {len(messages)} messages from {file_path}")
        return messages

    @staticmethod
    def _extract_timestamp(event: dict) -> datetime:
        """Extract timestamp from event dict."""
        ts = event.get("timestamp", "")
        if ts:
            try:
                # Handle ISO 8601 format
                if isinstance(ts, str):
                    return datetime.fromisoformat(ts.replace("Z", "+00:00"))
                elif isinstance(ts, (int, float)):
                    return datetime.fromtimestamp(ts / 1000, tz=timezone.utc)
            except (ValueError, OSError):
                pass
        return datetime.now(tz=timezone.utc)

    @staticmethod
    def _extract_project_tag(cwd: str) -> str:
        """Extract project name from working directory."""
        if not cwd:
            return ""
        return Path(cwd).name
