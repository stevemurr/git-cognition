from __future__ import annotations

from pathlib import Path

from git_cognition.storage import iter_sessions

from .common import (
    format_currency,
    json_dump,
    matches_time_window,
    parse_duration,
    session_changed_files,
    truncate_display,
)


def run(args) -> int:
    repo = str(Path(".").resolve())
    since = parse_duration(args.since)
    until = parse_duration(args.until)
    rows: list[dict[str, object]] = []

    for record in iter_sessions(repo):
        session = record.session
        changed_files = session_changed_files(repo, session)
        if args.file and args.file not in changed_files:
            continue
        if args.model and args.model.lower() not in session.agent.model.lower():
            continue
        if args.min_cost is not None and session.metrics.cost_usd < args.min_cost:
            continue
        if args.max_cost is not None and session.metrics.cost_usd > args.max_cost:
            continue
        if not matches_time_window(session.completed_at, since=since, until=until):
            continue
        rows.append(
            {
                "session_id": session.session_id,
                "date": session.completed_at,
                "model": session.agent.model,
                "external_session_id": session.agent.external_session_id,
                "file_count": len(changed_files),
                "files": changed_files,
                "cost_usd": session.metrics.cost_usd,
                "task_prompt": session.task.prompt,
                "follow_up_prompts": list(session.task.follow_up_prompts),
                "commit_count": len(session.commits),
            }
        )

    rows.sort(key=lambda row: str(row["date"]), reverse=True)
    limit = args.limit if args.limit is not None else 50
    rows = rows[:limit]

    if args.json:
        print(json_dump(rows))
        return 0

    if not rows:
        print("No sessions found.")
        return 0

    print("SESSION   DATE        MODEL                  FILES  COST    TASK")
    for row in rows:
        date = str(row["date"])[:10]
        task = truncate_display(str(row["task_prompt"]), 48)
        print(
            f"{str(row['session_id'])[:8]:8}  "
            f"{date:10}  "
            f"{truncate_display(str(row['model']), 22):22}  "
            f"{int(row['file_count']):5d}  "
            f"{format_currency(float(row['cost_usd'])):6}  "
            f"{task}"
        )
    total = sum(float(row["cost_usd"]) for row in rows)
    print(f"{len(rows)} sessions · {format_currency(total)} total")
    return 0
