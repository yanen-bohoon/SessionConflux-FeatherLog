"""Configuration loading and validation."""

import os
import platform
import yaml
from dataclasses import dataclass, field


@dataclass
class FeishuConfig:
    app_id: str = ""
    app_secret: str = ""
    app_token: str = ""
    table_id: str = ""

    @property
    def is_configured(self) -> bool:
        return bool(self.app_id and self.app_secret and self.app_token and self.table_id)


@dataclass
class AgentConfig:
    enabled: bool = True
    path: str = ""  # Empty means default path

    @property
    def effective_path(self) -> str:
        return self.path if self.path else self.default_path

    @property
    def default_path(self) -> str:
        home = os.path.expanduser("~")
        return self._default_path_map.get(self._key, "")

    _default_path_map: dict = field(default_factory=dict, repr=False)

    def __post_init__(self):
        home = os.path.expanduser("~")
        self._default_path_map = {
            "workbuddy": os.path.join(home, ".workbuddy", "projects"),
            "codex": os.path.join(home, ".codex", "sessions"),
            "claude": os.path.join(home, ".claude", "projects"),
        }

    @property
    def _key(self) -> str:
        return ""


@dataclass
class SyncConfig:
    poll_interval: int = 10
    batch_size: int = 10
    flush_timeout: int = 30
    api_batch_max: int = 500
    max_retries: int = 5
    initial_retry_delay: int = 1


@dataclass
class Config:
    hostname: str = ""
    feishu: FeishuConfig = field(default_factory=FeishuConfig)
    agents: dict = field(default_factory=dict)  # name -> AgentConfig
    sync: SyncConfig = field(default_factory=SyncConfig)

    @property
    def effective_hostname(self) -> str:
        return self.hostname if self.hostname else platform.node()


def load_config(path: str) -> Config:
    """Load config from YAML file."""
    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}

    feishu_data = data.get("feishu", {})
    feishu = FeishuConfig(
        app_id=feishu_data.get("app_id", ""),
        app_secret=feishu_data.get("app_secret", ""),
        app_token=feishu_data.get("app_token", ""),
        table_id=feishu_data.get("table_id", ""),
    )

    agents_data = data.get("agents", {})
    agents = {}
    for name, agent_conf in agents_data.items():
        agents[name] = AgentConfig(
            enabled=agent_conf.get("enabled", True),
            path=agent_conf.get("path", ""),
        )

    sync_data = data.get("sync", {})
    sync = SyncConfig(
        poll_interval=sync_data.get("poll_interval", 10),
        batch_size=sync_data.get("batch_size", 10),
        flush_timeout=sync_data.get("flush_timeout", 30),
        api_batch_max=sync_data.get("api_batch_max", 500),
        max_retries=sync_data.get("max_retries", 5),
        initial_retry_delay=sync_data.get("initial_retry_delay", 1),
    )

    return Config(
        hostname=data.get("hostname", ""),
        feishu=feishu,
        agents=agents,
        sync=sync,
    )
