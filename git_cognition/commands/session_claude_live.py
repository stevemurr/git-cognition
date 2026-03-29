from __future__ import annotations

import os
from pathlib import Path
import subprocess


def _build_command(args) -> list[str]:
    binary = args.claude_bin or os.environ.get("GIT_COGNITION_CLAUDE_BIN", "claude")
    repo = Path(".").resolve()
    plugin_dir = repo / "claude-plugin"
    claude_args = list(args.claude_args or [])
    if claude_args[:1] == ["--"]:
        claude_args = claude_args[1:]
    return [binary, "--plugin-dir", str(plugin_dir), *claude_args]


def run(args) -> int:
    repo = Path(".").resolve()
    plugin_dir = repo / "claude-plugin"
    if not plugin_dir.exists():
        print(f"missing plugin directory: {plugin_dir}")
        return 1
    try:
        completed = subprocess.run(_build_command(args), cwd=repo, check=False)
    except OSError as exc:
        print(f"Failed to run Claude: {exc}")
        return 1
    return completed.returncode
