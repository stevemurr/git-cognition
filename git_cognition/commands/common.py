from __future__ import annotations

from datetime import datetime, timedelta, timezone
import json
from typing import Any

from git_cognition.storage.git_notes import get_changed_files, get_commit_message
from git_cognition.storage.schema import Session


def parse_iso(value: str | None) -> datetime:
    if not value:
        return datetime.fromtimestamp(0, tz=timezone.utc)
    return datetime.fromisoformat(value)


def parse_duration(value: str | None) -> timedelta | None:
    if not value:
        return None
    units = {
        "s": 1,
        "min": 60,
        "h": 3600,
        "d": 86400,
        "w": 7 * 86400,
        "m": 30 * 86400,
    }
    if value.endswith("min"):
        unit = "min"
        amount_text = value[:-3]
    else:
        unit = value[-1]
        amount_text = value[:-1]
    if unit not in units or not amount_text:
        raise ValueError(f"unsupported duration {value!r}")
    amount = int(amount_text)
    return timedelta(seconds=amount * units[unit])


def matches_time_window(
    completed_at: str | None,
    *,
    since: timedelta | None = None,
    until: timedelta | None = None,
) -> bool:
    when = parse_iso(completed_at)
    now = datetime.now(timezone.utc)
    if since is not None and when < now - since:
        return False
    if until is not None and when > now - until:
        return False
    return True


def format_currency(amount: float) -> str:
    return f"${amount:.2f}"


def truncate_display(value: str, limit: int = 54) -> str:
    if len(value) <= limit:
        return value
    return f"{value[: limit - 3]}..."


def session_changed_files(repo: str, session: Session) -> list[str]:
    files: list[str] = []
    seen: set[str] = set()
    for commit_sha in session.commits:
        for path in get_changed_files(repo, commit_sha):
            if path in seen:
                continue
            seen.add(path)
            files.append(path)
    return files


def session_commit_details(repo: str, session: Session) -> list[dict[str, Any]]:
    details: list[dict[str, Any]] = []
    for commit_sha in session.commits:
        details.append(
            {
                "sha": commit_sha,
                "short_sha": commit_sha[:7],
                "message": get_commit_message(repo, commit_sha),
                "files_changed": get_changed_files(repo, commit_sha),
            }
        )
    return details


def json_dump(data: Any) -> str:
    return json.dumps(data, indent=2, sort_keys=True)
