"""Main sync engine — session-level sync to Feishu Wiki."""

import logging
import time
from .config import load_config
from .logger import setup_logging
from .cache import SessionStateCache
from .uploaders.feishu import FeishuWikiClient
from .parsers import get_parser

logger = logging.getLogger("conflux.engine")


class Engine:
    """Orchestrates session parsing and incremental upload to Feishu Wiki."""

    def __init__(self, config_path: str):
        self.config = load_config(config_path)
        setup_logging()
        self.state = SessionStateCache()
        self.client = FeishuWikiClient(self.config.feishu)
        self._resolve_wiki_space()

    def _resolve_wiki_space(self):
        """Resolve the wiki space so the client is ready for writes."""
        feishu = self.config.feishu
        if feishu.wiki_token:
            sid = self.client.resolve_space_from_token(feishu.wiki_token)
            logger.info(f"Resolved wiki space_id={sid} from token")
        elif feishu.wiki_space_id:
            self.client.resolve_space(feishu.wiki_space_id)
            logger.info(f"Using wiki space_id={feishu.wiki_space_id}")
        else:
            logger.warning("No wiki_space_id or wiki_token configured — "
                           "will retry on first sync")

    def run(self):
        """Continuous sync loop."""
        logger.info("Starting Conflux sync engine (v2 — Feishu Wiki)")
        logger.info(f"Hostname: {self.config.effective_hostname}")
        logger.info(f"Active agents: {[name for name, a in self.config.agents.items() if a.enabled]}")

        while True:
            try:
                self.run_once()
            except Exception as e:
                logger.error(f"Sync loop error: {e}", exc_info=True)

            logger.debug(f"Sleeping {self.config.sync.poll_interval}s")
            time.sleep(self.config.sync.poll_interval)

    def run_once(self):
        """Execute a single sync pass — session by session."""
        synced_any = False

        for name, agent_conf in self.config.agents.items():
            if not agent_conf.enabled:
                continue

            try:
                parser = get_parser(name)
                sessions = parser.find_sessions(agent_conf.effective_path)
                logger.info(f"[{name}] Found {len(sessions)} session files")

                for session_file in sessions:
                    messages = parser.parse_session(
                        session_file, self.config.effective_hostname)

                    if not messages:
                        continue

                    session_id = messages[0].session_id
                    skip_tool_calls = agent_conf.skip_tool_calls

                    # Filter tool calls if configured
                    filtered = [
                        m for m in messages
                        if not (skip_tool_calls and m.content.startswith("[工具调用:"))
                    ] if skip_tool_calls else messages

                    if not filtered:
                        continue

                    # Check if there are new messages to upload
                    if not self.state.has_new_messages(session_id, len(filtered)):
                        continue

                    # Sync session to Feishu Wiki
                    success = self.client.sync_session(
                        messages=filtered,
                        session_id=session_id,
                        agent=name,
                        computer=self.config.effective_hostname,
                        session_state_cache=self.state,
                        max_retries=self.config.sync.max_retries,
                        initial_delay=self.config.sync.initial_retry_delay,
                    )

                    if success:
                        synced_any = True
                        logger.info(
                            f"[{name}] {session_file.name}: synced "
                            f"{len(filtered)} messages"
                        )
                    else:
                        logger.error(
                            f"[{name}] {session_file.name}: sync failed"
                        )

            except Exception as e:
                logger.error(f"[{name}] Error processing: {e}", exc_info=True)

        if not synced_any:
            logger.debug("Sync pass complete: nothing new")

    def shutdown(self):
        """Clean shutdown."""
        logger.info("Shutting down engine")
        self.client.close()
