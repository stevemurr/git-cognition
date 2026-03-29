from __future__ import annotations

from pathlib import Path

from git_cognition.storage import (
    attach_session_to_commit,
    read_session,
    resolve_commit,
    resolve_session_id,
    write_session,
)


def run(args) -> int:
    repo = str(Path(".").resolve())
    session_id = resolve_session_id(repo, args.session_id)
    session = read_session(repo, session_id)

    commits = args.commits or ["HEAD"]
    attached: list[str] = []
    for commitish in commits:
        commit_sha = resolve_commit(repo, commitish)
        session.add_commit(commit_sha)
        attach_session_to_commit(repo, commit_sha, session.session_id)
        attached.append(commit_sha)

    write_session(repo, session)

    print(f"session: {session.session_id[:8]}")
    print("attached commits:")
    for commit_sha in attached:
        print(f"  {commit_sha[:7]}")
    return 0
