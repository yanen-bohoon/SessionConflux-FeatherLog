"""Local fingerprint cache for deduplication."""

import json
import os
from collections import OrderedDict


class FingerprintCache:
    """LRU cache of uploaded message fingerprints, persisted to disk."""

    def __init__(self, cache_path: str = ".conflux/fingerprints.json", max_size: int = 10000):
        self.cache_path = cache_path
        self.max_size = max_size
        self.fingerprints: OrderedDict[str, bool] = OrderedDict()
        self._load()

    def _load(self):
        if os.path.exists(self.cache_path):
            try:
                with open(self.cache_path, "r", encoding="utf-8") as f:
                    data = json.load(f)
                    for fp in data:
                        self.fingerprints[fp] = True
            except (json.JSONDecodeError, IOError):
                pass

    def _save(self):
        os.makedirs(os.path.dirname(self.cache_path), exist_ok=True)
        with open(self.cache_path, "w", encoding="utf-8") as f:
            json.dump(list(self.fingerprints.keys()), f)

    def has(self, fingerprint: str) -> bool:
        return fingerprint in self.fingerprints

    def add(self, fingerprint: str):
        self.fingerprints[fingerprint] = True
        # Evict oldest if over capacity
        while len(self.fingerprints) > self.max_size:
            self.fingerprints.popitem(last=False)
        self._save()

    def add_many(self, fingerprints: list[str]):
        for fp in fingerprints:
            self.fingerprints[fp] = True
        while len(self.fingerprints) > self.max_size:
            self.fingerprints.popitem(last=False)
        self._save()
