from __future__ import annotations

from pathlib import Path

from git_cognition.storage import iter_sessions, resolve_session_id

from .common import json_dump, matches_time_window, parse_duration


def excerpt_for_match(text: str, query: str, limit: int = 120) -> str:
    lower = text.lower()
    needle = query.lower()
    index = lower.find(needle)
    if index < 0:
        return text[:limit]
    start = max(index - 20, 0)
    end = min(index + len(query) + 60, len(text))
    excerpt = text[start:end].replace("\n", " ")
    if len(excerpt) <= limit:
        return excerpt
    return f"{excerpt[: limit - 3]}..."


def scoped_entries(session, scope: str):
    if scope in {"tasks", "all"}:
        yield {
            "scope": "tasks",
            "label": "task",
            "text": session.task.search_text(),
        }
        if session.agent.external_session_id:
            yield {
                "scope": "tasks",
                "label": "claude session",
                "text": session.agent.external_session_id,
            }
    if scope in {"tools", "all"}:
        for call in session.tool_calls:
            yield {
                "scope": "tools",
                "label": f"tool: {call.tool}",
                "text": call.search_text(),
            }
    if scope in {"rejected", "all"}:
        for rejected in session.rejected_approaches:
            yield {
                "scope": "rejected",
                "label": f"rejected: {rejected.what}",
                "text": rejected.search_text(),
            }
    if scope in {"thinking", "all"}:
        for block in session.thinking_blocks:
            yield {
                "scope": "thinking",
                "label": f"thinking: {block.get('sequence', '?')}",
                "text": str(block.get("text", "")),
            }


def run(args) -> int:
    repo = str(Path(".").resolve())
    since = parse_duration(args.since)
    session_prefix = args.session
    if session_prefix:
        target_id = resolve_session_id(repo, session_prefix)
    else:
        target_id = None

    matches: list[dict[str, str]] = []
    session_count: set[str] = set()
    for record in iter_sessions(repo):
        session = record.session
        if target_id and session.session_id != target_id:
            continue
        if not matches_time_window(session.completed_at, since=since):
            continue
        for entry in scoped_entries(session, args.scope):
            if args.query.lower() not in entry["text"].lower():
                continue
            session_count.add(session.session_id)
            matches.append(
                {
                    "session_id": session.session_id,
                    "date": (session.completed_at or session.created_at)[:10],
                    "scope": entry["scope"],
                    "label": entry["label"],
                    "excerpt": excerpt_for_match(entry["text"], args.query),
                }
            )

    matches.sort(key=lambda item: (item["date"], item["session_id"]), reverse=True)

    if args.json:
        print(json_dump(matches))
        return 0

    print(
        f'searching: "{args.query}"  scope: {args.scope}  '
        f"({len(matches)} matches across {len(session_count)} sessions)"
    )
    if matches:
        print()
    for match in matches:
        print(f"{match['session_id'][:8]}  ·  {match['date']}  ·  {match['label']}")
        print(f'  "{match["excerpt"]}"')
        print()
    return 0
