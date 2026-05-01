"""Configuration loading and validation."""

import os
import platform
import yaml
from dataclasses import dataclass, field


@dataclass
class FeishuConfig:
    app_id: str = ""
    app_secret: str = ""
    wiki_space_name: str = "SessionConflux 对话库"  # Feishu wiki space name

    @property
    def is_configured(self) -> bool:
        return bool(self.app_id and self.app_secret)


@dataclass
class AgentConfig:
    name: str = ""
    enabled: bool = True
    path: str = ""  # Empty means default path
    skip_tool_calls: bool = True  # Skip [工具调用: xxx] messages to reduce record count

    _default_path_map: dict = field(default_factory=dict, repr=False)

    def __post_init__(self):
        home = os.path.expanduser("~")
        self._default_path_map = {
            "workbuddy": os.path.join(home, ".workbuddy", "projects"),
            "codex": os.path.join(home, ".codex", "sessions"),
            "claude": os.path.join(home, ".claude", "projects"),
        }

    @property
    def effective_path(self) -> str:
        return self.path if self.path else self._default_path_map.get(self.name, "")


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
        wiki_space_name=feishu_data.get("wiki_space_name", "SessionConflux 对话库"),
    )

    agents_data = data.get("agents", {})
    agents = {}
    for name, agent_conf in agents_data.items():
        agents[name] = AgentConfig(
            name=name,
            enabled=agent_conf.get("enabled", True),
            path=agent_conf.get("path", ""),
            skip_tool_calls=agent_conf.get("skip_tool_calls", True),
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
