"""WorkBuddy session parser."""

import json
import logging
from datetime import datetime, timezone
from pathlib import Path
from ..models import UnifiedMessage
from .base import ParserBase

logger = logging.getLogger("conflux.parsers.workbuddy")


class WorkBuddyParser(ParserBase):
    """Parse WorkBuddy JSONL session files.
    
    Format: ~/.workbuddy/projects/{workspace_path_hash}/{sessionId}.jsonl
    Each line is a JSON object. Key types:
    - type=message: user/assistant conversation (core extraction target)
    - type=function_call: tool invocation (summary extraction)
    - type=reasoning: internal reasoning (optional)
    - Others: skipped
    """

    @property
    def name(self) -> str:
        return "workbuddy"

    def find_sessions(self, base_path: str) -> list[Path]:
        """Find all .jsonl files under the projects directory."""
        base = Path(base_path)
        if not base.exists():
            logger.warning(f"WorkBuddy projects path not found: {base_path}")
            return []
        
        sessions = []
        # Recursively find all .jsonl files
        for jsonl_file in base.rglob("*.jsonl"):
            if jsonl_file.is_file():
                sessions.append(jsonl_file)
        
        return sorted(sessions)

    def parse_session(self, file_path: Path, hostname: str) -> list[UnifiedMessage]:
        """Parse a WorkBuddy JSONL file into UnifiedMessages."""
        messages: list[UnifiedMessage] = []
        msg_index = 0
        
        # Extract session ID from filename
        session_id = file_path.stem  # e.g., "28e23e7a-5379-4b1a-8481-7a0f891f753f"
        
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
                    
                    if event_type == "message":
                        role = event.get("role", "")
                        if role not in ("user", "assistant"):
                            continue
                        
                        # Extract content text
                        content_list = event.get("content", [])
                        text_parts = []
                        for item in content_list:
                            if isinstance(item, dict):
                                t = item.get("type", "")
                                if t in ("input_text", "output_text"):
                                    text = item.get("text", "")
                                    if text:
                                        text_parts.append(text)
                        
                        if not text_parts:
                            continue
                        
                        content = "\n\n".join(text_parts)
                        
                        # Extract timestamp
                        ts = event.get("timestamp", 0)
                        if ts > 0:
                            timestamp = datetime.fromtimestamp(ts / 1000, tz=timezone.utc)
                        else:
                            timestamp = datetime.now(tz=timezone.utc)
                        
                        messages.append(UnifiedMessage(
                            timestamp=timestamp,
                            source_agent=self.name,
                            session_id=session_id,
                            message_index=msg_index,
                            role=role,
                            content=content,
                            source_computer=hostname,
                            project_tag=project_tag,
                        ))
                        msg_index += 1
                        
                    elif event_type == "function_call":
                        # Extract tool call as summary
                        tool_name = event.get("name", "")
                        arguments_display = event.get("argumentsDisplayText", "")
                        
                        if tool_name:
                            content = f"[工具调用: {tool_name}]"
                            if arguments_display:
                                content += f"({arguments_display[:200]})"
                            
                            ts = event.get("timestamp", 0)
                            timestamp = datetime.fromtimestamp(ts / 1000, tz=timezone.utc) if ts > 0 else datetime.now(tz=timezone.utc)
                            
                            messages.append(UnifiedMessage(
                                timestamp=timestamp,
                                source_agent=self.name,
                                session_id=session_id,
                                message_index=msg_index,
                                role="assistant",
                                content=content,
                                source_computer=hostname,
                                project_tag=project_tag,
                            ))
                            msg_index += 1

        except IOError as e:
            logger.error(f"Failed to read {file_path}: {e}")
        
        logger.debug(f"Parsed {len(messages)} messages from {file_path}")
        return messages
