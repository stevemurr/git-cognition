from __future__ import annotations

from dataclasses import dataclass, field
import json
import os
from pathlib import Path
import subprocess
import time
from typing import Any

from git_cognition.storage import commits_between, current_head, is_tracking_enabled
from git_cognition.writer.claude_code import ClaudeCodeSessionWriter

from .common import json_dump


def _usage_to_metrics(usage: dict[str, Any]) -> dict[str, float | int]:
    return {
        "input_tokens": int(usage.get("input_tokens", 0)),
        "output_tokens": int(usage.get("output_tokens", 0)),
        "thinking_tokens": int(usage.get("thinking_tokens", 0)),
    }


def _stringify_content(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(part for part in (_stringify_content(item) for item in value) if part)
    if isinstance(value, dict):
        if "text" in value and isinstance(value["text"], str):
            return value["text"]
        return json.dumps(value, ensure_ascii=True, sort_keys=True)
    return str(value)


def _summarize_text(text: str, limit: int = 140) -> str:
    normalized = " ".join(text.split())
    if len(normalized) <= limit:
        return normalized
    return f"{normalized[: limit - 3]}..."


@dataclass(slots=True)
class ToolResultRecord:
    summary: str
    excerpt: str | None = None
    snapshot: str | None = None


@dataclass(slots=True)
class ClaudeStreamTranscript:
    model: str | None = None
    session_id: str | None = None
    assistant_blocks: list[dict[str, Any]] = field(default_factory=list)
    tool_results_by_id: dict[str, ToolResultRecord] = field(default_factory=dict)
    usage: dict[str, Any] = field(default_factory=dict)
    result_text: str = ""
    total_cost_usd: float = 0.0
    duration_seconds: float = 0.0
    is_error: bool = False
    error_message: str | None = None
    raw_events: list[dict[str, Any]] = field(default_factory=list)

    @classmethod
    def from_stream(cls, stdout: str) -> "ClaudeStreamTranscript":
        transcript = cls()
        for line in stdout.splitlines():
            stripped = line.strip()
            if not stripped:
                continue
            try:
                event = json.loads(stripped)
            except json.JSONDecodeError:
                continue
            transcript.raw_events.append(event)
            transcript._ingest_event(event)
        transcript._finalize()
        return transcript

    def _ingest_event(self, event: dict[str, Any]) -> None:
        event_type = event.get("type")
        if not self.session_id and isinstance(event.get("session_id"), str):
            self.session_id = event["session_id"]
        if event_type == "system":
            if isinstance(event.get("model"), str):
                self.model = event["model"]
            return
        if event_type in {"assistant", "user"}:
            message = event.get("message") or {}
            if not self.model and isinstance(message.get("model"), str):
                self.model = message["model"]
            usage = message.get("usage")
            if isinstance(usage, dict):
                self.usage = usage
            if event_type == "assistant":
                for block in message.get("content", []):
                    self.assistant_blocks.append(block)
            else:
                self._ingest_user_blocks(message.get("content", []))
            if isinstance(event.get("error"), str):
                self.is_error = True
                self.error_message = _stringify_content(message.get("content", [])) or event["error"]
            return
        if event_type == "result":
            self.is_error = bool(event.get("is_error"))
            self.result_text = str(event.get("result", ""))
            self.total_cost_usd = float(event.get("total_cost_usd", 0.0))
            self.duration_seconds = float(event.get("duration_ms", 0.0)) / 1000.0
            usage = event.get("usage")
            if isinstance(usage, dict):
                self.usage = usage
            if self.is_error and not self.error_message:
                self.error_message = self.result_text or "Claude run failed"

    def _ingest_user_blocks(self, content_blocks: list[dict[str, Any]]) -> None:
        for block in content_blocks:
            if block.get("type") != "tool_result":
                continue
            tool_use_id = block.get("tool_use_id")
            if not isinstance(tool_use_id, str):
                continue
            text = _stringify_content(block.get("content"))
            self.tool_results_by_id[tool_use_id] = ToolResultRecord(
                summary=_summarize_text(text or "tool call completed"),
                excerpt=text or None,
                snapshot=text or None,
            )

    def _finalize(self) -> None:
        if not self.result_text:
            texts = [
                str(block.get("text", ""))
                for block in self.assistant_blocks
                if block.get("type") == "text"
            ]
            self.result_text = "\n".join(text for text in texts if text).strip()
        if self.is_error and not self.error_message:
            self.error_message = self.result_text or "Claude run failed"


def _build_claude_command(args) -> list[str]:
    binary = args.claude_bin or os.environ.get("GIT_COGNITION_CLAUDE_BIN", "claude")
    command = [
        binary,
        "-p",
        "--verbose",
        "--output-format",
        "stream-json",
    ]
    if args.model:
        command.extend(["--model", args.model])
    if args.permission_mode:
        command.extend(["--permission-mode", args.permission_mode])
    if args.max_budget_usd is not None:
        command.extend(["--max-budget-usd", str(args.max_budget_usd)])
    if args.system_prompt:
        command.extend(["--system-prompt", args.system_prompt])
    if args.append_system_prompt:
        command.extend(["--append-system-prompt", args.append_system_prompt])
    for directory in args.add_dir or []:
        command.extend(["--add-dir", directory])
    if args.allowed_tools:
        command.extend(["--allowed-tools", *args.allowed_tools])
    if args.disallowed_tools:
        command.extend(["--disallowed-tools", *args.disallowed_tools])
    if args.tools:
        command.extend(["--tools", *args.tools])
    if args.dangerously_skip_permissions:
        command.append("--dangerously-skip-permissions")
    if args.bare:
        command.append("--bare")
    command.append(args.prompt)
    return command


def run(args) -> int:
    repo = Path(".").resolve()
    if not is_tracking_enabled(repo):
        print("git-cognition is not initialized in this repo. Run: git session init")
        return 2

    before_head = current_head(repo)
    command = _build_claude_command(args)
    writer = ClaudeCodeSessionWriter(
        repo=repo,
        task_prompt=args.prompt,
        model=args.model or "claude",
        capture_thinking=args.capture_thinking,
    )
    writer.start_session()

    started = time.monotonic()
    try:
        completed = subprocess.run(
            command,
            cwd=repo,
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError as exc:
        writer.abort_session()
        print(f"Failed to run Claude: {exc}")
        return 1
    duration_seconds = time.monotonic() - started
    transcript = ClaudeStreamTranscript.from_stream(completed.stdout)

    if transcript.model:
        writer.session.agent.model = transcript.model
    if transcript.session_id:
        writer.set_external_session_id(transcript.session_id)
    if transcript.usage or transcript.total_cost_usd or transcript.duration_seconds:
        metrics = _usage_to_metrics(transcript.usage)
        writer.record_metrics(
            input_tokens=int(metrics["input_tokens"]),
            output_tokens=int(metrics["output_tokens"]),
            thinking_tokens=int(metrics["thinking_tokens"]),
            cost_usd=transcript.total_cost_usd,
            duration_seconds=transcript.duration_seconds or duration_seconds,
        )

    for block in transcript.assistant_blocks:
        block_type = block.get("type")
        if block_type == "tool_use":
            seq = writer.record_tool_call(
                tool=str(block.get("name", "tool_use")),
                kind=writer._infer_kind(str(block.get("name", ""))),
                paths=writer._extract_paths(block.get("input")),
                raw_input=block.get("input"),
            )
            tool_result = transcript.tool_results_by_id.get(str(block.get("id", "")))
            if tool_result:
                writer.ingest_tool_result(
                    seq,
                    summary=tool_result.summary,
                    raw_output_excerpt=tool_result.excerpt,
                    output_snapshot=tool_result.snapshot,
                )
        elif block_type == "thinking":
            writer.record_thinking(str(block.get("thinking", "")))
        elif block_type == "text":
            writer.ingest_text(str(block.get("text", "")))
    if transcript.result_text and not any(
        block.get("type") == "text" for block in transcript.assistant_blocks
    ):
        writer.ingest_text(transcript.result_text)

    if transcript.is_error or completed.returncode != 0:
        writer.abort_session()
        message = transcript.error_message or completed.stderr.strip() or "Claude run failed"
        print(message)
        return completed.returncode or 1

    after_head = current_head(repo)
    attached_commits = commits_between(repo, before_head, after_head)
    if not attached_commits and args.attach_head and after_head:
        attached_commits = [after_head]
    for commit_sha in attached_commits:
        writer.attach_commit(commit_sha)
    session_id = writer.finalize_session()

    payload = {
        "session_id": session_id,
        "claude_session_id": transcript.session_id,
        "attached_commits": attached_commits,
        "result": transcript.result_text,
        "tool_call_count": len(writer.session.tool_calls),
    }

    if args.json:
        print(json_dump(payload))
        return 0

    print(f"session: {session_id[:8]}")
    if transcript.session_id:
        print(f"claude session: {transcript.session_id}")
    if attached_commits:
        print("attached commits:")
        for commit_sha in attached_commits:
            print(f"  {commit_sha[:7]}")
    else:
        print("attached commits: [none detected]")
        print("tip: ask Claude to commit during the wrapped run, or attach later with `git session attach <id> HEAD`")
    print()
    if transcript.result_text:
        print(transcript.result_text)
        print()
    print(f"inspect: git session show {session_id[:8]}")
    if attached_commits:
        print("inspect line history with: git why <file>:<line>")
    return 0
