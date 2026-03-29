from __future__ import annotations

from pathlib import Path

from git_cognition.storage import read_session, resolve_session_id

from .common import format_currency, json_dump, session_commit_details, session_changed_files


def run(args) -> int:
    repo = str(Path(".").resolve())
    session_id = resolve_session_id(repo, args.session_id)
    session = read_session(repo, session_id)
    commit_details = session_commit_details(repo, session)

    if args.json:
        print(
            json_dump(
                {
                    "session": session.to_dict(),
                    "commit_details": commit_details,
                    "files_changed": session_changed_files(repo, session),
                }
            )
        )
        return 0

    print(
        f"{session.session_id[:8]}  ·  {session.agent.model}  ·  "
        f"{(session.completed_at or session.created_at).replace('T', ' ')[:16]}  ·  "
        f"{session.metrics.duration_seconds:.0f}s  ·  {format_currency(session.metrics.cost_usd)}"
    )
    print()
    print(f"task:    {session.task.prompt}")
    print()
    print("commits:")
    for detail in commit_details:
        print(f"  {detail['short_sha']}  {detail['message']}")
    print()
    print("tool calls:")
    if not session.tool_calls:
        print("  [none]")
    for call in session.tool_calls:
        target = f" ({', '.join(call.paths)})" if call.paths else ""
        print(f"  {call.sequence}.  {call.tool}{target}")
        if call.output_summary:
            print(f"      -> {call.output_summary}")
        if call.raw_output_excerpt:
            print(f"      excerpt: {call.raw_output_excerpt}")
    print()
    if args.thinking and session.thinking_blocks:
        print("thinking:")
        for block in session.thinking_blocks:
            print(f"  {block.get('sequence', '?')}. {block.get('text', '')}")
    else:
        print("thinking:  [none captured - v1]")
    print()
    print("rejected:")
    if not session.rejected_approaches:
        print("  [none]")
    for rejected in session.rejected_approaches:
        print(f"  - {rejected.what}")
        print(f"    {rejected.why}")
    print()
    metrics = session.metrics
    print(
        "metrics:  "
        f"{metrics.input_tokens:,} input · "
        f"{metrics.output_tokens:,} output · "
        f"{format_currency(metrics.cost_usd)}"
    )
    return 0
