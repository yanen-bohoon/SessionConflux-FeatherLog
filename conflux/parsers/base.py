"""Parser base class."""

from abc import ABC, abstractmethod
from pathlib import Path
from conflux.models import UnifiedMessage


class ParserBase(ABC):
    """Base class for all agent session parsers."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Agent name identifier."""
        ...

    @abstractmethod
    def find_sessions(self, base_path: str) -> list[Path]:
        """Find all session files that need processing."""
        ...

    @abstractmethod
    def parse_session(self, file_path: Path, hostname: str) -> list[UnifiedMessage]:
        """Parse a single session file into unified messages."""
        ...
