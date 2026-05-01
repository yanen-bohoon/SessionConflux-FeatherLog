"""Claude Code session parser."""

import json
import logging
from datetime import datetime, timezone
from pathlib import Path
from ..models import UnifiedMessage
from .base import ParserBase

logger = logging.getLogger("conflux.parsers.claude")


class ClaudeParser(ParserBase):
    """Parse Claude Code JSONL session files.
    
    Format: ~/.claude/projects/{workspace_hash}/{sessionId}.jsonl
    Each line is a JSON object. Key types:
    - type=user: user message (content is plain string)
    - type=assistant: assistant message (content is array with text/tool_use/tool_result)
    - type=system: system metadata
    - Others: file-history-snapshot, permission-mode, attachment, last-prompt (skipped)
    """

    @property
    def name(self) -> str:
        return "claude"

    def find_sessions(self, base_path: str) -> list[Path]:
        """Find all .jsonl files under the projects directory."""
        base = Path(base_path)
        if not base.exists():
            logger.warning(f"Claude Code projects path not found: {base_path}")
            return []
        
        sessions = []
        for jsonl_file in base.rglob("*.jsonl"):
            if jsonl_file.is_file():
                sessions.append(jsonl_file)
        
        return sorted(sessions)

    def parse_session(self, file_path: Path, hostname: str) -> list[UnifiedMessage]:
        """Parse a Claude Code JSONL file into UnifiedMessages."""
        messages: list[UnifiedMessage] = []
        msg_index = 0
        
        # Extract session ID from filename
        session_id = file_path.stem  # e.g., "0ce8600d-c3a7-4ec3-af45-e8c34c166adb"
        
        # Extract project tag from parent directory name
        project_tag = file_path.parent.name.replace("-", " ").replace("_", " ")

        try:
            with open(file_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    
                    try:
                        event = json.loads(line)
                    except json.JSONDecodeError:
                        logger.debug(f"Skipping malformed line in {file_path}")
                        continue
                    
                    event_type = event.get("type", "")

                    # Skip non-conversation events
                    if event_type in ("system", "file-history-snapshot", "permission-mode", 
                                      "attachment", "last-prompt", "result"):
                        continue

                    # Extract timestamp
                    ts_str = event.get("timestamp", "")
                    timestamp = self._parse_timestamp(ts_str)

                    if event_type == "user":
                        # User message - content is a plain string
                        content = event.get("message", {}).get("content", "")
                        if content and isinstance(content, str):
                            messages.append(UnifiedMessage(
                                timestamp=timestamp,
                                source_agent=self.name,
                                session_id=session_id,
                                message_index=msg_index,
                                role="user",
                                content=content,
                                source_computer=hostname,
                                project_tag=project_tag,
                            ))
                            msg_index += 1

                    elif event_type == "assistant":
                        # Assistant message - content is an array
                        content_list = event.get("message", {}).get("content", [])
                        if not isinstance(content_list, list):
                            continue
                        
                        # Process each content item
                        for item in content_list:
                            if not isinstance(item, dict):
                                continue
                            
                            item_type = item.get("type", "")
                            
                            if item_type == "text":
                                text = item.get("text", "")
                                if text:
                                    messages.append(UnifiedMessage(
                                        timestamp=timestamp,
                                        source_agent=self.name,
                                        session_id=session_id,
                                        message_index=msg_index,
                                        role="assistant",
                                        content=text,
                                        source_computer=hostname,
                                        project_tag=project_tag,
                                    ))
                                    msg_index += 1
                            
                            elif item_type == "tool_use":
                                tool_name = item.get("name", "")
                                if tool_name:
                                    # Create a summary of the tool call
                                    tool_input = item.get("input", {})
                                    summary = self._summarize_tool_call(tool_name, tool_input)
                                    messages.append(UnifiedMessage(
                                        timestamp=timestamp,
                                        source_agent=self.name,
                                        session_id=session_id,
                                        message_index=msg_index,
                                        role="assistant",
                                        content=f"[工具调用: {tool_name}]{summary}",
                                        source_computer=hostname,
                                        project_tag=project_tag,
                                    ))
                                    msg_index += 1
                            
                            # Skip tool_result - usually too verbose

        except IOError as e:
            logger.error(f"Failed to read {file_path}: {e}")
        
        logger.debug(f"Parsed {len(messages)} messages from {file_path}")
        return messages

    @staticmethod
    def _parse_timestamp(ts_str: str) -> datetime:
        """Parse ISO 8601 timestamp string."""
        if not ts_str:
            return datetime.now(tz=timezone.utc)
        try:
            return datetime.fromisoformat(ts_str.replace("Z", "+00:00"))
        except (ValueError, AttributeError):
            return datetime.now(tz=timezone.utc)

    @staticmethod
    def _summarize_tool_call(tool_name: str, tool_input: dict) -> str:
        """Create a human-readable summary of a tool call."""
        if tool_name == "Bash":
            cmd = tool_input.get("command", "")
            if cmd:
                # Truncate long commands
                cmd_preview = cmd[:100] + "..." if len(cmd) > 100 else cmd
                return f"(`{cmd_preview}`)"
        elif tool_name == "Read":
            path = tool_input.get("file_path", "")
            if path:
                return f"({path})"
        elif tool_name == "Write":
            path = tool_input.get("file_path", "")
            if path:
                return f"({path})"
        elif tool_name in ("WebFetch", "WebSearch"):
            query = tool_input.get("query", tool_input.get("url", ""))
            if query:
                return f"({query[:80]})"
        
        return ""
