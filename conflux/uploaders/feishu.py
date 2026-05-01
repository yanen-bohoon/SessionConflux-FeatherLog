"""Feishu Bitable API client."""

import logging
import time
import httpx
from .config import FeishuConfig

logger = logging.getLogger("conflux.feishu")


class FeishuClient:
    """Client for Feishu Open API, handling auth and Bitable operations."""

    BASE_URL = "https://open.feishu.cn/open-apis"

    def __init__(self, config: FeishuConfig):
        self.config = config
        self._access_token: str | None = None
        self._token_expires_at: float = 0
        self._client = httpx.Client(timeout=30)

    def _get_tenant_token(self) -> str:
        """Get tenant access token, refreshing if expired."""
        if self._access_token and time.time() < self._token_expires_at:
            return self._access_token

        resp = self._client.post(
            f"{self.BASE_URL}/auth/v3/tenant_access_token/internal",
            json={
                "app_id": self.config.app_id,
                "app_secret": self.config.app_secret,
            },
        )
        resp.raise_for_status()
        data = resp.json()

        if data.get("code") != 0:
            raise RuntimeError(f"Failed to get tenant token: {data}")

        self._access_token = data["tenant_access_token"]
        self._token_expires_at = time.time() + data.get("expire", 7200) - 300  # 5min buffer
        return self._access_token

    def create_records(self, records: list[dict], max_retries: int = 5, initial_delay: int = 1) -> bool:
        """Batch create records in Bitable. Returns True on success."""
        if not records:
            return True

        url = f"{self.BASE_URL}/bitable/v1/apps/{self.config.app_token}/tables/{self.config.table_id}/records/batch_create"
        token = self._get_tenant_token()

        delay = initial_delay
        for attempt in range(max_retries):
            try:
                # Feishu API max 500 records per batch
                for i in range(0, len(records), 500):
                    batch = records[i : i + 500]
                    resp = self._client.post(
                        url,
                        headers={"Authorization": f"Bearer {token}"},
                        json={"records": batch},
                    )
                    resp.raise_for_status()
                    data = resp.json()

                    if data.get("code") == 0:
                        logger.info(f"Uploaded {len(batch)} records (batch {i // 500 + 1})")
                    elif data.get("code") == 99991400:
                        # Rate limited - exponential backoff
                        logger.warning(f"Rate limited, retrying in {delay}s (attempt {attempt + 1})")
                        time.sleep(delay)
                        delay *= 2
                        continue
                    else:
                        logger.error(f"API error: {data}")
                        return False

                return True

            except httpx.HTTPError as e:
                logger.error(f"HTTP error on attempt {attempt + 1}: {e}")
                if attempt < max_retries - 1:
                    time.sleep(delay)
                    delay *= 2
                    # Refresh token on retry
                    self._access_token = None
                else:
                    return False

        return False

    def close(self):
        self._client.close()
