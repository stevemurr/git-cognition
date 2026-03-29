from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
import json
from pathlib import Path
import subprocess

from .schema import SCHEMA_VERSION, SchemaError, Session

CONFIG_ENABLED_KEY = "git-cognition.enabled"
CONFIG_SCHEMA_KEY = "git-cognition.schema-version"


class GitCognitionError(RuntimeError):
    """Base error for Git-backed storage failures."""


class MissingSessionNoteError(GitCognitionError):
    """Raised when a commit has no session note."""


class MissingSessionBlobError(GitCognitionError):
    """Raised when a session blob ref is missing."""


class InvalidSessionBlobError(GitCognitionError):
    """Raised when a session blob cannot be parsed or validated."""


@dataclass(slots=True)
class SessionRecord:
    session_id: str
    session: Session


def repo_path(path: str | Path | None = None) -> Path:
    return Path(path or ".").resolve()


def run_git(
    repo: str | Path,
    args: list[str],
    *,
    input_text: str | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        ["git", *args],
        cwd=repo_path(repo),
        input=input_text,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and completed.returncode != 0:
        raise GitCognitionError(completed.stderr.strip() or completed.stdout.strip())
    return completed


def ensure_git_repo(repo: str | Path) -> None:
    run_git(repo, ["rev-parse", "--git-dir"])


def git_dir(repo: str | Path) -> Path:
    completed = run_git(repo, ["rev-parse", "--git-dir"])
    return (repo_path(repo) / completed.stdout.strip()).resolve()


def init_tracking(repo: str | Path) -> dict[str, str]:
    ensure_git_repo(repo)
    run_git(repo, ["config", "--local", CONFIG_ENABLED_KEY, "true"])
    run_git(repo, ["config", "--local", CONFIG_SCHEMA_KEY, SCHEMA_VERSION])
    return {
        "enabled": "true",
        "schema_version": SCHEMA_VERSION,
    }


def is_tracking_enabled(repo: str | Path) -> bool:
    completed = run_git(
        repo,
        ["config", "--local", "--get", CONFIG_ENABLED_KEY],
        check=False,
    )
    return completed.returncode == 0 and completed.stdout.strip().lower() == "true"


def write_session(repo: str | Path, session: Session) -> str:
    ensure_git_repo(repo)
    session.validate_for_write()
    payload = json.dumps(session.to_dict(), indent=2, sort_keys=True)
    blob = run_git(repo, ["hash-object", "-w", "--stdin"], input_text=payload).stdout.strip()
    run_git(repo, ["update-ref", f"refs/sessions/{session.session_id}", blob])
    return blob


def attach_session_to_commit(repo: str | Path, commit_sha: str, session_id: str) -> None:
    ensure_git_repo(repo)
    run_git(
        repo,
        ["notes", "--ref=sessions", "add", "-f", "-m", f"session_id: {session_id}", commit_sha],
    )


def read_session_id_for_commit(repo: str | Path, commit_sha: str) -> str:
    completed = run_git(
        repo,
        ["notes", "--ref=sessions", "show", commit_sha],
        check=False,
    )
    if completed.returncode != 0:
        raise MissingSessionNoteError(f"no session note for commit {commit_sha}")
    text = completed.stdout.strip()
    if not text:
        raise MissingSessionNoteError(f"empty session note for commit {commit_sha}")
    prefix = "session_id:"
    if text.startswith(prefix):
        return text[len(prefix) :].strip()
    return text.strip()


def read_session(repo: str | Path, session_id: str) -> Session:
    completed = run_git(
        repo,
        ["cat-file", "-p", f"refs/sessions/{session_id}"],
        check=False,
    )
    if completed.returncode != 0:
        raise MissingSessionBlobError(f"missing session blob for {session_id}")
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise InvalidSessionBlobError(f"invalid JSON for session {session_id}: {exc}") from exc
    try:
        session = Session.from_dict(payload)
    except SchemaError as exc:
        raise InvalidSessionBlobError(f"invalid session schema for {session_id}: {exc}") from exc
    if session.schema_version != SCHEMA_VERSION:
        raise InvalidSessionBlobError(
            f"unsupported schema version for {session_id}: {session.schema_version}"
        )
    return session


def read_session_for_commit(repo: str | Path, commit_sha: str) -> tuple[str, Session]:
    session_id = read_session_id_for_commit(repo, commit_sha)
    return session_id, read_session(repo, session_id)


def list_session_ids(repo: str | Path) -> list[str]:
    completed = run_git(
        repo,
        ["for-each-ref", "refs/sessions", "--format=%(refname)"],
        check=False,
    )
    if completed.returncode != 0:
        return []
    session_ids: list[str] = []
    for line in completed.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        session_ids.append(line.rsplit("/", 1)[-1])
    return session_ids


def resolve_commit(repo: str | Path, commitish: str) -> str:
    return run_git(repo, ["rev-parse", f"{commitish}^{{commit}}"]).stdout.strip()


def current_head(repo: str | Path) -> str | None:
    completed = run_git(repo, ["rev-parse", "HEAD"], check=False)
    if completed.returncode != 0:
        return None
    return completed.stdout.strip()


def commits_between(
    repo: str | Path,
    before_commit: str | None,
    after_commit: str | None,
) -> list[str]:
    if not after_commit:
        return []
    if before_commit:
        args = ["rev-list", "--reverse", f"{before_commit}..{after_commit}"]
    else:
        args = ["rev-list", "--reverse", after_commit]
    completed = run_git(repo, args, check=False)
    if completed.returncode != 0:
        return []
    return [line.strip() for line in completed.stdout.splitlines() if line.strip()]


def resolve_session_id(repo: str | Path, prefix: str) -> str:
    session_ids = list_session_ids(repo)
    if prefix in session_ids:
        return prefix
    matches = [session_id for session_id in session_ids if session_id.startswith(prefix)]
    if not matches:
        raise MissingSessionBlobError(f"no session found for prefix {prefix!r}")
    if len(matches) > 1:
        raise GitCognitionError(f"ambiguous session prefix {prefix!r}")
    return matches[0]


def iter_sessions(repo: str | Path) -> list[SessionRecord]:
    records: list[SessionRecord] = []
    for session_id in list_session_ids(repo):
        records.append(SessionRecord(session_id=session_id, session=read_session(repo, session_id)))
    return records


def get_commit_message(repo: str | Path, commit_sha: str) -> str:
    return run_git(repo, ["show", "--quiet", "--format=%s", commit_sha]).stdout.strip()


def get_commit_date(repo: str | Path, commit_sha: str) -> datetime:
    timestamp = run_git(repo, ["show", "--quiet", "--format=%cI", commit_sha]).stdout.strip()
    return datetime.fromisoformat(timestamp)


def get_changed_files(repo: str | Path, commit_sha: str) -> list[str]:
    completed = run_git(
        repo,
        ["show", "--format=", "--name-only", commit_sha],
        check=False,
    )
    if completed.returncode != 0:
        return []
    return [line.strip() for line in completed.stdout.splitlines() if line.strip()]


def get_line_blame(repo: str | Path, file_path: str, line_number: int) -> str:
    completed = run_git(
        repo,
        ["blame", "--porcelain", "-L", f"{line_number},{line_number}", "--", file_path],
    )
    first_line = completed.stdout.splitlines()[0]
    return first_line.split()[0]


def get_line_blame_text(repo: str | Path, file_path: str, line_number: int) -> str:
    return run_git(repo, ["blame", "-L", f"{line_number},{line_number}", "--", file_path]).stdout


def get_tracked_line_context(
    repo: str | Path,
    commit_sha: str,
    file_path: str,
    line_number: int,
    radius: int = 5,
) -> str:
    completed = run_git(repo, ["show", f"{commit_sha}:{file_path}"], check=False)
    if completed.returncode != 0:
        return file_path
    lines = completed.stdout.splitlines()
    if not lines:
        return file_path
    start = max(line_number - radius - 1, 0)
    end = min(line_number + radius, len(lines))
    snippet = "\n".join(lines[start:end])
    return f"{file_path}\n{snippet}"
