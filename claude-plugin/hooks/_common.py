#!/usr/bin/env python3
from __future__ import annotations

import json
from pathlib import Path
import sys

REPO_ROOT = Path(__file__).resolve().parents[2]
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from git_cognition.claude_plugin import (  # noqa: E402
    finalize_claude_session,
    handle_post_tool_use,
    handle_session_start,
    handle_user_prompt_submit,
    hook_response,
)

HANDLERS = {
    "session_start": handle_session_start,
    "user_prompt_submit": handle_user_prompt_submit,
    "post_tool_use": handle_post_tool_use,
    "session_end": finalize_claude_session,
    "stop": finalize_claude_session,
}


def run_handler(name: str) -> None:
    try:
        payload = json.load(sys.stdin)
    except Exception as exc:  # pragma: no cover - Claude controls stdin
        print(
            json.dumps(
                hook_response(
                    suppress_output=False,
                    system_message=f"git-cognition hook error: invalid JSON payload ({exc})",
                )
            )
        )
        raise SystemExit(0)

    try:
        response = HANDLERS[name](payload)
    except Exception as exc:  # pragma: no cover - surfaced to Claude on failure
        response = hook_response(
            suppress_output=False,
            system_message=f"git-cognition hook error: {exc}",
        )
    print(json.dumps(response))
    raise SystemExit(0)
