from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "runtimes" / "python"))

from pluginart import serve


CONTRACT_HASH = "sha256:pluginart-bench"


def handle(payload: bytes) -> bytes:
    return payload


if __name__ == "__main__":
    serve(handle, contract_hash=CONTRACT_HASH)
