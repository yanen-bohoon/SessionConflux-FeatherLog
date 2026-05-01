"""Unified message model."""

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

    def to_bitable_record(self) -> dict:
        """Convert to Feishu Bitable record format."""
        return {
            "fields": {
                "消息时间": self.timestamp.strftime("%Y-%m-%dT%H:%M:%S+08:00"),
                "来源Agent": self.source_agent,
                "会话ID": self.session_id,
                "消息序号": self.message_index,
                "角色": "用户" if self.role == "user" else "助手",
                "内容": self.content,
                "来源电脑": self.source_computer,
                "项目标签": self.project_tag,
                "消息指纹": self.fingerprint,
            }
        }
