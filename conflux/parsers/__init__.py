"""Agent session parsers."""

from .workbuddy import WorkBuddyParser
from .codex import CodexParser
from .claude import ClaudeParser

PARSERS = {
    "workbuddy": WorkBuddyParser,
    "codex": CodexParser,
    "claude": ClaudeParser,
}


def get_parser(name: str):
    """Get parser class by name."""
    cls = PARSERS.get(name)
    if cls is None:
        raise ValueError(f"Unknown parser: {name}")
    return cls()
