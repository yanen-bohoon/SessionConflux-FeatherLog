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

    def to_block_dict(self) -> dict:
        """Convert to Feishu Docx block dict for a single message."""
        role_label = "👤 用户" if self.role == "user" else "🤖 助手"
        time_str = self.timestamp.strftime("%Y-%m-%d %H:%M:%S")

        blocks = []

        # Message header with role + timestamp
        header_text = f"{role_label}  |  {time_str}  |  #{self.message_index}"
        blocks.append({
            "block_type": 3,  # Heading2
            "heading2": {
                "elements": [{"text_run": {"content": header_text}}],
            },
        })

        # Message content
        content = self.content
        # Detect code blocks
        parts = re.split(r"(```[\s\S]*?```)", content)
        for part in parts:
            if part.startswith("```") and part.endswith("```"):
                # Code block
                lines = part.strip("`").strip("\n").split("\n")
                lang = lines[0].strip() if lines else ""
                code = "\n".join(lines[1:]) if lang and len(lines) > 1 else "\n".join(lines)
                blocks.append({
                    "block_type": 16,  # Code
                    "code": {
                        "elements": [{"text_run": {"content": code.strip()}}],
                        "style": {"language": 0, "wrap": True},
                    },
                })
            elif part.strip():
                # Regular text
                blocks.append({
                    "block_type": 2,  # Text
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
