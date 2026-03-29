from __future__ import annotations

from pathlib import Path

from git_cognition.storage import SCHEMA_VERSION, init_tracking


def run(args) -> int:
    repo = Path(".").resolve()
    result = init_tracking(repo)
    print(f"Initialized git-cognition in {repo}")
    print(f"tracking: {result['enabled']}")
    print(f"schema:   {result['schema_version']}")
    print("remote session refs remain opt-in")
    return 0
