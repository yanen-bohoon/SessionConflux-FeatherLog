"""Unified message model."""

import re
import hashlib
from dataclasses import dataclass, field
from datetime import datetime


@dataclass
class UnifiedMessage:
    """A single conversation message, unified across all agents."""

    timestamp: datetime          # When the message was created
    source_agent: str            # "workbuddy" | "codex" | "claude"
    session_id: str              # Original session ID from the agent
    message_index: int           # Sequential index within session
    role: str                    # "user" | "assistant"
    content: str                 # Cleaned text content
    source_computer: str         # Hostname or custom identifier
    project_tag: str = ""        # Optional project label (from Claude Code etc.)

    @property
    def fingerprint(self) -> str:
        """Unique fingerprint for deduplication."""
        raw = f"{self.session_id}:{self.message_index}:{self.content[:100]}"
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()[:16]

    def to_block_dict(self) -> list:
        """Convert to Feishu Docx block dicts for a single message.

        Uses text blocks (block_type=2) with bold headers for safety
        and reliability across all Feishu Docx API versions.
        """
        role_label = "👤 用户" if self.role == "user" else "🤖 助手"
        time_str = self.timestamp.strftime("%Y-%m-%d %H:%M:%S")

        blocks = []

        # Message header with role + timestamp (bold text)
        header_text = f"{role_label}  |  {time_str}  |  #{self.message_index}"
        blocks.append({
            "block_type": 2,
            "text": {
                "elements": [{
                    "text_run": {
                        "content": header_text,
                        "text_element_style": {"bold": True},
                    }
                }],
            },
        })

        # Message content
        content = self.content.strip()
        if not content:
            return blocks

        # Split content into sections (code blocks vs regular text)
        parts = re.split(r"(```[\s\S]*?```)", content)
        for part in parts:
            if not part.strip():
                continue
            if part.startswith("```") and part.endswith("```"):
                # Code block — use text block with inline code style
                inner = part.strip("`").strip("\n")
                # Remove language tag from first line
                lines = inner.split("\n")
                code_text = "\n".join(lines[1:]) if len(lines) > 1 else inner
                blocks.append({
                    "block_type": 2,
                    "text": {
                        "elements": [{
                            "text_run": {
                                "content": code_text.strip(),
                                "text_element_style": {"inline_code": True},
                            }
                        }],
                    },
                })
            else:
                # Regular text block
                blocks.append({
                    "block_type": 2,
                    "text": {
                        "elements": [{"text_run": {"content": part.strip()}}],
                    },
                })

        return blocks


def generate_session_title(messages: list["UnifiedMessage"]) -> str:
    """Generate a document title from session messages.

    Format: YYYY-MM-DD_first_user_message_preview
    """
    date_part = ""
    title_part = "会话"

    for msg in messages:
        if msg.role == "user":
            if not date_part:
                date_part = msg.timestamp.strftime("%Y-%m-%d")
            # Take first meaningful user message content
            text = msg.content.strip()[:80].replace("\n", " ")
            text = re.sub(r"[^\w一-鿿\s\-]", "", text)
            text = text.strip()[:40]
            if text:
                title_part = text
            break

    if not date_part and messages:
        date_part = messages[0].timestamp.strftime("%Y-%m-%d")

    return f"{date_part}_{title_part}" if date_part else title_part
