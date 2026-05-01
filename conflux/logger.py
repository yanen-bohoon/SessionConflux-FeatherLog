"""Logging setup."""

import logging
import os
from datetime import datetime


def setup_logging(log_dir: str = ".conflux"):
    """Setup logging to file and console."""
    os.makedirs(log_dir, exist_ok=True)
    log_file = os.path.join(log_dir, f"conflux-{datetime.now().strftime('%Y-%m-%d')}.log")

    formatter = logging.Formatter(
        "%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )

    # File handler
    fh = logging.FileHandler(log_file, encoding="utf-8")
    fh.setLevel(logging.DEBUG)
    fh.setFormatter(formatter)

    # Console handler
    ch = logging.StreamHandler()
    ch.setLevel(logging.INFO)
    ch.setFormatter(formatter)

    root = logging.getLogger("conflux")
    root.setLevel(logging.DEBUG)
    root.addHandler(fh)
    root.addHandler(ch)

    return root
