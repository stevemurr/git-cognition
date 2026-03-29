from __future__ import annotations

from dataclasses import asdict, dataclass
import json
from pathlib import Path
from typing import Any

from git_cognition.storage import commits_between, current_head, is_tracking_enabled
from git_cognition.storage.git_notes import git_dir, run_git
from git_cognition.writer.claude_code import ClaudeCodeSessionWriter


@dataclass(slots=True)
class ClaudePluginState:
    repo: str
    git_cognition_session_id: str | None
    start_head: str | None
    transcript_path: str | None
    finalized: bool = False


def hook_response(
    *,
    suppress_output: bool = True,
    system_message: str | None = None,
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "continue": True,
        "suppressOutput": suppress_output,
    }
    if system_message:
        payload["systemMessage"] = system_message
    return payload


def _string_value(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def _resolve_repo(cwd: str | None) -> Path | None:
    if not cwd:
        return None
    completed = run_git(cwd, ["rev-parse", "--show-toplevel"], check=False)
    if completed.returncode != 0:
        return None
    root = completed.stdout.strip()
    if not root:
        return None
    return Path(root).resolve()


def runtime_dir(repo: str | Path) -> Path:
    return git_dir(repo) / "git-cognition" / "claude"


def runtime_path(repo: str | Path, claude_session_id: str) -> Path:
    safe_name = claude_session_id.replace("/", "_")
    return runtime_dir(repo) / f"{safe_name}.json"


def load_runtime_state(repo: str | Path, claude_session_id: str) -> ClaudePluginState | None:
    path = runtime_path(repo, claude_session_id)
    if not path.exists():
        return None
    return ClaudePluginState(**json.loads(path.read_text(encoding="utf-8")))


def save_runtime_state(repo: str | Path, claude_session_id: str, state: ClaudePluginState) -> None:
    path = runtime_path(repo, claude_session_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(asdict(state), indent=2, sort_keys=True), encoding="utf-8")


def remove_runtime_state(repo: str | Path, claude_session_id: str) -> None:
    path = runtime_path(repo, claude_session_id)
    if path.exists():
        path.unlink()
    parent = path.parent
    if parent.exists():
        try:
            parent.rmdir()
        except OSError:
            pass


def _stringify_tool_value(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    try:
        return json.dumps(value, ensure_ascii=True, sort_keys=True)
    except TypeError:
        return str(value)


def _summarize_tool_result(tool_name: str, tool_result: Any, limit: int = 140) -> str:
    text = " ".join(_stringify_tool_value(tool_result).split())
    if not text:
        return f"{tool_name} completed"
    if len(text) <= limit:
        return text
    return f"{text[: limit - 3]}..."


def _ensure_runtime_state(payload: dict[str, Any]) -> tuple[Path | None, str | None, ClaudePluginState | None]:
    claude_session_id = _string_value(payload.get("session_id"))
    repo = _resolve_repo(_string_value(payload.get("cwd")))
    if not claude_session_id or repo is None:
        return None, None, None
    if not is_tracking_enabled(repo):
        return repo, claude_session_id, None
    state = load_runtime_state(repo, claude_session_id)
    if state is None:
        state = ClaudePluginState(
            repo=str(repo),
            git_cognition_session_id=None,
            start_head=current_head(repo),
            transcript_path=_string_value(payload.get("transcript_path")),
            finalized=False,
        )
        save_runtime_state(repo, claude_session_id, state)
    else:
        transcript_path = _string_value(payload.get("transcript_path"))
        if transcript_path and transcript_path != state.transcript_path:
            state.transcript_path = transcript_path
            save_runtime_state(repo, claude_session_id, state)
    return repo, claude_session_id, state


def _load_or_create_writer(
    repo: Path,
    claude_session_id: str,
    state: ClaudePluginState,
    *,
    task_prompt: str | None = None,
) -> tuple[ClaudeCodeSessionWriter | None, bool]:
    created = False
    writer: ClaudeCodeSessionWriter | None = None
    if state.git_cognition_session_id:
        try:
            writer = ClaudeCodeSessionWriter.load_pending(repo, state.git_cognition_session_id)
        except FileNotFoundError:
            writer = None
    if writer is None and task_prompt:
        writer = ClaudeCodeSessionWriter(
            repo=repo,
            task_prompt=task_prompt,
            model="claude",
            capture_thinking=False,
        )
        writer.start_session()
        state.git_cognition_session_id = writer.session_id
        created = True
        save_runtime_state(repo, claude_session_id, state)
    if writer is None:
        return None, False
    writer.set_external_session_id(claude_session_id)
    return writer, created


def handle_session_start(payload: dict[str, Any]) -> dict[str, Any]:
    repo, claude_session_id, state = _ensure_runtime_state(payload)
    if repo is None or claude_session_id is None or state is None:
        return hook_response()
    state.start_head = current_head(repo)
    state.finalized = False
    save_runtime_state(repo, claude_session_id, state)
    return hook_response()


def handle_user_prompt_submit(payload: dict[str, Any]) -> dict[str, Any]:
    repo, claude_session_id, state = _ensure_runtime_state(payload)
    if repo is None or claude_session_id is None or state is None or state.finalized:
        return hook_response()
    prompt = _string_value(payload.get("user_prompt"))
    if not prompt:
        return hook_response()
    writer, created = _load_or_create_writer(repo, claude_session_id, state, task_prompt=prompt)
    if writer is None:
        return hook_response()
    if not created:
        writer.record_user_prompt(prompt)
    return hook_response()


def handle_post_tool_use(payload: dict[str, Any]) -> dict[str, Any]:
    repo, claude_session_id, state = _ensure_runtime_state(payload)
    if repo is None or claude_session_id is None or state is None or state.finalized:
        return hook_response()
    writer, _ = _load_or_create_writer(repo, claude_session_id, state)
    if writer is None:
        return hook_response()
    tool_name = _string_value(payload.get("tool_name")) or "tool"
    tool_input = payload.get("tool_input")
    tool_result = payload.get("tool_result")
    rendered_result = _stringify_tool_value(tool_result)
    writer.record_tool_call(
        tool=tool_name,
        kind=writer._infer_kind(tool_name),
        paths=writer._extract_paths(tool_input),
        raw_input=tool_input,
        output_summary=_summarize_tool_result(tool_name, tool_result),
        raw_output_excerpt=rendered_result or None,
        output_snapshot=rendered_result or None,
    )
    return hook_response()


def finalize_claude_session(payload: dict[str, Any]) -> dict[str, Any]:
    repo, claude_session_id, state = _ensure_runtime_state(payload)
    if repo is None or claude_session_id is None or state is None or state.finalized:
        return hook_response()
    if not state.git_cognition_session_id:
        remove_runtime_state(repo, claude_session_id)
        return hook_response()
    try:
        writer = ClaudeCodeSessionWriter.load_pending(repo, state.git_cognition_session_id)
    except FileNotFoundError:
        remove_runtime_state(repo, claude_session_id)
        return hook_response()

    for commit_sha in commits_between(repo, state.start_head, current_head(repo)):
        writer.attach_commit(commit_sha)
    writer.set_external_session_id(claude_session_id)
    writer.finalize_session()

    state.finalized = True
    save_runtime_state(repo, claude_session_id, state)
    remove_runtime_state(repo, claude_session_id)
    return hook_response()
