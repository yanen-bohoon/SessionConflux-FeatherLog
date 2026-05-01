"""Main sync engine."""

import logging
import time
from .config import load_config, Config
from .logger import setup_logging
from .cache import FingerprintCache
from .uploaders.feishu import FeishuClient
from .parsers import get_parser

logger = logging.getLogger("conflux.engine")


class Engine:
    """Orchestrates parsing, deduplication, and uploading."""

    def __init__(self, config_path: str):
        self.config = load_config(config_path)
        setup_logging()
        self.cache = FingerprintCache()
        self.buffer: list[dict] = []  # Buffer of bitable record dicts
        self.last_flush = time.time()
        self.client = FeishuClient(self.config.feishu)

    def run(self):
        """Continuous sync loop."""
        logger.info("Starting Conflux sync engine")
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
        """Execute a single sync pass."""
        total_new = 0

        for name, agent_conf in self.config.agents.items():
            if not agent_conf.enabled:
                continue

            try:
                parser = get_parser(name)
                sessions = parser.find_sessions(agent_conf.effective_path)
                logger.info(f"[{name}] Found {len(sessions)} session files")

                for session_file in sessions:
                    messages = parser.parse_session(session_file, self.config.effective_hostname)
                    new_messages = []

                    for msg in messages:
                        if not self.cache.has(msg.fingerprint):
                            self.buffer.append(msg.to_bitable_record())
                            self.cache.add(msg.fingerprint)
                            new_messages.append(msg)

                    if new_messages:
                        logger.info(f"[{name}] {session_file.name}: {len(new_messages)} new messages")
                        total_new += len(new_messages)

            except Exception as e:
                logger.error(f"[{name}] Error processing: {e}", exc_info=True)

        # Flush buffer
        self._flush()

        if total_new > 0:
            logger.info(f"Sync pass complete: {total_new} new messages processed")
        else:
            logger.debug("Sync pass complete: no new messages")

    def _flush(self):
        """Upload buffered records to Feishu."""
        if not self.buffer:
            return

        # Check timeout condition
        elapsed = time.time() - self.last_flush
        if len(self.buffer) < self.config.sync.batch_size and elapsed < self.config.sync.flush_timeout:
            return

        logger.info(f"Flushing {len(self.buffer)} records to Feishu")
        success = self.client.create_records(
            self.buffer,
            max_retries=self.config.sync.max_retries,
            initial_delay=self.config.sync.initial_retry_delay,
        )

        if success:
            self.buffer.clear()
            self.last_flush = time.time()
        else:
            logger.error("Failed to flush records, will retry next pass")

    def shutdown(self):
        """Clean shutdown."""
        logger.info("Shutting down engine")
        self._flush()
        self.client.close()
