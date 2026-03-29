from __future__ import annotations

import json
from pathlib import Path

from git_cognition.storage.git_notes import attach_session_to_commit, git_dir, run_git, write_session
from git_cognition.storage.schema import (
    AgentInfo,
    Metrics,
    RejectedApproach,
    Session,
    TaskInfo,
    ToolCall,
)


class AgentSessionWriter:
    """Agent-agnostic writer with explicit attach/finalize lifecycle."""

    def __init__(
        self,
        *,
        repo: str | Path,
        task_prompt: str,
        model: str,
        runner: str,
        model_version: str | None = None,
        context_files: list[str] | None = None,
        capture_thinking: bool = False,
    ) -> None:
        self.repo = Path(repo).resolve()
        self.capture_thinking = capture_thinking
        self.session = Session(
            agent=AgentInfo(runner=runner, model=model, model_version=model_version),
            task=TaskInfo(prompt=task_prompt, context_files=context_files or []),
        )
        self._started = False
        self._finished = False

    @property
    def session_id(self) -> str:
        return self.session.session_id

    @property
    def pending_path(self) -> Path:
        return git_dir(self.repo) / "git-cognition" / "pending" / f"{self.session_id}.json"

    @classmethod
    def pending_path_for(cls, repo: str | Path, session_id: str) -> Path:
        return git_dir(repo) / "git-cognition" / "pending" / f"{session_id}.json"

    @classmethod
    def load_pending(cls, repo: str | Path, session_id: str) -> "AgentSessionWriter":
        pending_path = cls.pending_path_for(repo, session_id)
        payload = json.loads(pending_path.read_text(encoding="utf-8"))
        writer = cls.__new__(cls)
        writer.repo = Path(repo).resolve()
        writer.capture_thinking = bool(payload.get("thinking_blocks"))
        writer.session = Session.from_dict(payload)
        writer._started = True
        writer._finished = False
        return writer

    def start_session(self) -> str:
        if self._finished:
            raise RuntimeError("session already finalized")
        if self._started:
            return self.session_id
        self._started = True
        self._persist_pending_state()
        return self.session_id

    def _ensure_started(self) -> None:
        if not self._started:
            self.start_session()

    def _persist_pending_state(self) -> None:
        pending_path = self.pending_path
        pending_path.parent.mkdir(parents=True, exist_ok=True)
        pending_path.write_text(
            json.dumps(self.session.to_dict(), indent=2, sort_keys=True),
            encoding="utf-8",
        )

    def _flush_pending_state(self) -> None:
        if self.pending_path.exists():
            self.pending_path.unlink()
        parent = self.pending_path.parent
        if parent.exists():
            try:
                parent.rmdir()
                parent.parent.rmdir()
            except OSError:
                pass

    def record_tool_call(
        self,
        *,
        tool: str,
        kind: str = "generic",
        paths: list[str] | None = None,
        raw_input=None,
        output_summary: str = "",
        raw_output_excerpt: str | None = None,
        output_snapshot: str | None = None,
    ) -> int:
        self._ensure_started()
        call = ToolCall(
            sequence=len(self.session.tool_calls) + 1,
            tool=tool,
            kind=kind,
            paths=paths or [],
            raw_input=raw_input,
            output_summary=output_summary,
            raw_output_excerpt=raw_output_excerpt,
            output_snapshot=output_snapshot,
        )
        self.session.tool_calls.append(call)
        self._persist_pending_state()
        return call.sequence

    def update_tool_call(
        self,
        sequence: int,
        *,
        output_summary: str | None = None,
        raw_output_excerpt: str | None = None,
        output_snapshot: str | None = None,
    ) -> None:
        self._ensure_started()
        if not (0 < sequence <= len(self.session.tool_calls)):
            raise IndexError("tool call sequence out of range")
        current = self.session.tool_calls[sequence - 1]
        updated = ToolCall(
            sequence=current.sequence,
            tool=current.tool,
            kind=current.kind,
            paths=current.paths,
            raw_input=current.raw_input,
            output_summary=output_summary if output_summary is not None else current.output_summary,
            raw_output_excerpt=(
                raw_output_excerpt
                if raw_output_excerpt is not None
                else current.raw_output_excerpt
            ),
            output_snapshot=(
                output_snapshot if output_snapshot is not None else current.output_snapshot
            ),
        )
        self.session.tool_calls[sequence - 1] = updated
        self._persist_pending_state()

    def record_thinking(self, text: str) -> None:
        self._ensure_started()
        if not self.capture_thinking or not text:
            return
        self.session.thinking_blocks.append(
            {
                "sequence": len(self.session.thinking_blocks) + 1,
                "text": text,
            }
        )
        self._persist_pending_state()

    def record_rejected(self, what: str, why: str) -> None:
        self._ensure_started()
        self.session.rejected_approaches.append(RejectedApproach(what=what, why=why))
        self._persist_pending_state()

    def record_user_prompt(self, text: str) -> None:
        self._ensure_started()
        if not self.session.task.prompt:
            self.session.task.prompt = TaskInfo(prompt=text).prompt
        else:
            self.session.task.add_follow_up_prompt(text)
        self._persist_pending_state()

    def set_external_session_id(self, value: str | None) -> None:
        self._ensure_started()
        cleaned = str(value).strip() if value is not None else ""
        self.session.agent.external_session_id = cleaned or None
        self._persist_pending_state()

    def record_metrics(
        self,
        *,
        input_tokens: int,
        output_tokens: int,
        thinking_tokens: int = 0,
        cost_usd: float = 0.0,
        duration_seconds: float = 0.0,
    ) -> None:
        self._ensure_started()
        self.session.metrics = Metrics(
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            thinking_tokens=thinking_tokens,
            cost_usd=cost_usd,
            duration_seconds=duration_seconds,
        )
        self._persist_pending_state()

    def attach_commit(self, commit_sha: str) -> None:
        self._ensure_started()
        resolved = run_git(self.repo, ["rev-parse", f"{commit_sha}^{{commit}}"]).stdout.strip()
        self.session.add_commit(resolved)
        self._persist_pending_state()

    def finalize_session(self) -> str:
        self._ensure_started()
        if self._finished:
            return self.session_id
        self.session.finalize()
        write_session(self.repo, self.session)
        for commit_sha in self.session.commits:
            attach_session_to_commit(self.repo, commit_sha, self.session_id)
        self._finished = True
        self._flush_pending_state()
        return self.session_id

    def abort_session(self) -> None:
        self._finished = True
        self._flush_pending_state()
