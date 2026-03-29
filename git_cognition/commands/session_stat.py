from __future__ import annotations

from collections import Counter, defaultdict
from pathlib import Path

from git_cognition.storage import get_changed_files, iter_sessions

from .common import format_currency, json_dump, matches_time_window, parse_duration, session_changed_files


def run(args) -> int:
    repo = str(Path(".").resolve())
    since = parse_duration(args.since)
    records = [
        record
        for record in iter_sessions(repo)
        if matches_time_window(record.session.completed_at, since=since)
    ]

    total_sessions = len(records)
    total_cost = sum(record.session.metrics.cost_usd for record in records)
    total_commits = sum(len(record.session.commits) for record in records)

    by_model_count: Counter[str] = Counter()
    by_model_cost: defaultdict[str, float] = defaultdict(float)
    file_session_count: Counter[str] = Counter()
    file_commit_count: Counter[str] = Counter()

    for record in records:
        model = record.session.agent.model
        by_model_count[model] += 1
        by_model_cost[model] += record.session.metrics.cost_usd
        changed_files = session_changed_files(repo, record.session)
        for file_path in changed_files:
            file_session_count[file_path] += 1
        for commit_sha in record.session.commits:
            for file_path in get_changed_files(repo, commit_sha):
                file_commit_count[file_path] += 1

    payload = {
        "sessions": total_sessions,
        "total_cost_usd": round(total_cost, 6),
        "average_cost_usd": round(total_cost / total_sessions, 6) if total_sessions else 0.0,
        "commits": total_commits,
        "by_model": [
            {"model": model, "sessions": by_model_count[model], "cost_usd": round(by_model_cost[model], 6)}
            for model in sorted(by_model_count, key=by_model_count.get, reverse=True)
        ],
        "most_touched_files": [
            {
                "path": path,
                "sessions": file_session_count[path],
                "commit_occurrences": file_commit_count[path],
            }
            for path, _ in file_session_count.most_common(5)
        ],
    }

    if args.json:
        print(json_dump(payload))
        return 0

    print(f"sessions:    {total_sessions} total")
    print(
        f"spend:       {format_currency(total_cost)} total  ·  "
        f"{format_currency(total_cost / total_sessions) if total_sessions else '$0.00'} avg per session"
    )
    print(f"commits:     {total_commits} agent commits")
    print()
    print("by model:")
    if not payload["by_model"]:
        print("  [none]")
    for row in payload["by_model"]:
        print(
            f"  {row['model'][:20]:20}  "
            f"{row['sessions']:3d} sessions  "
            f"{format_currency(row['cost_usd'])}"
        )
    print()
    print("most-touched files:")
    if not payload["most_touched_files"]:
        print("  [none]")
    for row in payload["most_touched_files"]:
        print(
            f"  {row['path'][:30]:30}  "
            f"{row['sessions']:3d} sessions  "
            f"{row['commit_occurrences']:3d} commits"
        )
    return 0
