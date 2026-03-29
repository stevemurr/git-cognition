from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
import json
import uuid
from typing import Any

SCHEMA_VERSION = "1.0"

MAX_PROMPT_CHARS = 4_000
MAX_SUMMARY_CHARS = 600
MAX_REASON_CHARS = 1_200
MAX_TEXT_EXCERPT_CHARS = 4_000
MAX_OUTPUT_SNAPSHOT_CHARS = 20_000
MAX_JSON_CHARS = 4_000
MAX_PATHS = 64
MAX_CONTEXT_FILES = 128


class SchemaError(ValueError):
    """Raised when a session payload does not match the expected schema."""


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def truncate_text(value: str | None, limit: int) -> str | None:
    if value is None:
        return None
    if len(value) <= limit:
        return value
    if limit <= 16:
        return value[:limit]
    return f"{value[: limit - 15]}... [truncated]"


def sanitize_json_value(value: Any, limit: int = MAX_JSON_CHARS) -> Any:
    if value is None:
        return None
    try:
        serialized = json.dumps(value, ensure_ascii=True, sort_keys=True)
    except TypeError:
        serialized = json.dumps(str(value), ensure_ascii=True)
        value = str(value)
    if len(serialized) <= limit:
        return value
    return {
        "truncated": True,
        "excerpt": truncate_text(serialized, limit),
    }


def normalize_paths(paths: list[str] | tuple[str, ...] | None) -> list[str]:
    if not paths:
        return []
    normalized: list[str] = []
    seen: set[str] = set()
    for raw in paths:
        path = str(raw).strip()
        if not path or path in seen:
            continue
        normalized.append(path)
        seen.add(path)
        if len(normalized) >= MAX_PATHS:
            break
    return normalized


@dataclass(slots=True)
class AgentInfo:
    runner: str
    model: str
    model_version: str | None = None

    def to_dict(self) -> dict[str, Any]:
        data = {
            "runner": self.runner,
            "model": self.model,
        }
        if self.model_version:
            data["model_version"] = self.model_version
        return data

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "AgentInfo":
        return cls(
            runner=str(data.get("runner", "")),
            model=str(data.get("model", "")),
            model_version=data.get("model_version"),
        )


@dataclass(slots=True)
class TaskInfo:
    prompt: str
    context_files: list[str] = field(default_factory=list)

    def __post_init__(self) -> None:
        self.prompt = truncate_text(str(self.prompt), MAX_PROMPT_CHARS) or ""
        self.context_files = normalize_paths(self.context_files[:MAX_CONTEXT_FILES])

    def to_dict(self) -> dict[str, Any]:
        return {
            "prompt": self.prompt,
            "context_files": self.context_files,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "TaskInfo":
        return cls(
            prompt=str(data.get("prompt", "")),
            context_files=list(data.get("context_files", [])),
        )


@dataclass(slots=True)
class ToolCall:
    sequence: int
    tool: str
    kind: str = "generic"
    paths: list[str] = field(default_factory=list)
    raw_input: Any = None
    output_summary: str = ""
    raw_output_excerpt: str | None = None
    output_snapshot: str | None = None

    def __post_init__(self) -> None:
        self.sequence = int(self.sequence)
        if self.sequence <= 0:
            raise SchemaError("tool call sequence must be positive")
        self.tool = str(self.tool)
        self.kind = str(self.kind or "generic")
        self.paths = normalize_paths(self.paths)
        self.raw_input = sanitize_json_value(self.raw_input)
        self.output_summary = truncate_text(str(self.output_summary), MAX_SUMMARY_CHARS) or ""
        self.raw_output_excerpt = truncate_text(self.raw_output_excerpt, MAX_TEXT_EXCERPT_CHARS)
        self.output_snapshot = truncate_text(self.output_snapshot, MAX_OUTPUT_SNAPSHOT_CHARS)

    def to_dict(self) -> dict[str, Any]:
        return {
            "sequence": self.sequence,
            "tool": self.tool,
            "kind": self.kind,
            "paths": self.paths,
            "raw_input": self.raw_input,
            "output_summary": self.output_summary,
            "raw_output_excerpt": self.raw_output_excerpt,
            "output_snapshot": self.output_snapshot,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ToolCall":
        return cls(
            sequence=int(data.get("sequence", 0)),
            tool=str(data.get("tool", "")),
            kind=str(data.get("kind", "generic")),
            paths=list(data.get("paths", [])),
            raw_input=data.get("raw_input"),
            output_summary=str(data.get("output_summary", "")),
            raw_output_excerpt=data.get("raw_output_excerpt"),
            output_snapshot=data.get("output_snapshot"),
        )

    def search_text(self) -> str:
        parts = [
            self.tool,
            self.kind,
            " ".join(self.paths),
            json.dumps(self.raw_input, ensure_ascii=True, sort_keys=True)
            if self.raw_input is not None
            else "",
            self.output_summary,
            self.raw_output_excerpt or "",
            self.output_snapshot or "",
        ]
        return "\n".join(part for part in parts if part)


@dataclass(slots=True)
class RejectedApproach:
    what: str
    why: str

    def __post_init__(self) -> None:
        self.what = truncate_text(str(self.what), MAX_SUMMARY_CHARS) or ""
        self.why = truncate_text(str(self.why), MAX_REASON_CHARS) or ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "what": self.what,
            "why": self.why,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "RejectedApproach":
        return cls(
            what=str(data.get("what", "")),
            why=str(data.get("why", "")),
        )

    def search_text(self) -> str:
        return f"{self.what}\n{self.why}".strip()


@dataclass(slots=True)
class Metrics:
    input_tokens: int = 0
    output_tokens: int = 0
    thinking_tokens: int = 0
    cost_usd: float = 0.0
    duration_seconds: float = 0.0

    def __post_init__(self) -> None:
        self.input_tokens = int(self.input_tokens)
        self.output_tokens = int(self.output_tokens)
        self.thinking_tokens = int(self.thinking_tokens)
        self.cost_usd = float(self.cost_usd)
        self.duration_seconds = float(self.duration_seconds)

    def to_dict(self) -> dict[str, Any]:
        return {
            "input_tokens": self.input_tokens,
            "output_tokens": self.output_tokens,
            "thinking_tokens": self.thinking_tokens,
            "cost_usd": round(self.cost_usd, 6),
            "duration_seconds": round(self.duration_seconds, 6),
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Metrics":
        return cls(
            input_tokens=int(data.get("input_tokens", 0)),
            output_tokens=int(data.get("output_tokens", 0)),
            thinking_tokens=int(data.get("thinking_tokens", 0)),
            cost_usd=float(data.get("cost_usd", 0.0)),
            duration_seconds=float(data.get("duration_seconds", 0.0)),
        )


@dataclass(slots=True)
class Session:
    agent: AgentInfo
    task: TaskInfo
    schema_version: str = SCHEMA_VERSION
    session_id: str = field(default_factory=lambda: uuid.uuid4().hex)
    created_at: str = field(default_factory=utc_now_iso)
    completed_at: str | None = None
    commits: list[str] = field(default_factory=list)
    tool_calls: list[ToolCall] = field(default_factory=list)
    thinking_blocks: list[dict[str, Any]] = field(default_factory=list)
    rejected_approaches: list[RejectedApproach] = field(default_factory=list)
    metrics: Metrics = field(default_factory=Metrics)

    def __post_init__(self) -> None:
        self.schema_version = str(self.schema_version)
        if not self.session_id:
            raise SchemaError("session_id is required")
        self.commits = [str(commit).strip() for commit in self.commits if str(commit).strip()]

    def finalize(self) -> None:
        self.completed_at = utc_now_iso()

    def add_commit(self, commit_sha: str) -> None:
        commit_sha = str(commit_sha).strip()
        if commit_sha and commit_sha not in self.commits:
            self.commits.append(commit_sha)

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "session_id": self.session_id,
            "created_at": self.created_at,
            "completed_at": self.completed_at,
            "agent": self.agent.to_dict(),
            "task": self.task.to_dict(),
            "commits": self.commits,
            "tool_calls": [call.to_dict() for call in self.tool_calls],
            "thinking_blocks": list(self.thinking_blocks),
            "rejected_approaches": [item.to_dict() for item in self.rejected_approaches],
            "metrics": self.metrics.to_dict(),
        }

    def validate_for_write(self) -> None:
        if self.schema_version != SCHEMA_VERSION:
            raise SchemaError(
                f"unsupported schema version {self.schema_version!r}; expected {SCHEMA_VERSION!r}"
            )
        if not self.completed_at:
            raise SchemaError("completed_at must be set before writing a session")
        if not self.agent.runner or not self.agent.model:
            raise SchemaError("agent runner and model are required")
        if not self.task.prompt:
            raise SchemaError("task prompt is required")

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Session":
        try:
            session = cls(
                schema_version=str(data.get("schema_version", "")),
                session_id=str(data.get("session_id", "")),
                created_at=str(data.get("created_at", "")),
                completed_at=data.get("completed_at"),
                agent=AgentInfo.from_dict(dict(data.get("agent", {}))),
                task=TaskInfo.from_dict(dict(data.get("task", {}))),
                commits=list(data.get("commits", [])),
                tool_calls=[ToolCall.from_dict(item) for item in data.get("tool_calls", [])],
                thinking_blocks=list(data.get("thinking_blocks", [])),
                rejected_approaches=[
                    RejectedApproach.from_dict(item)
                    for item in data.get("rejected_approaches", [])
                ],
                metrics=Metrics.from_dict(dict(data.get("metrics", {}))),
            )
        except (TypeError, ValueError) as exc:
            raise SchemaError(f"invalid session payload: {exc}") from exc
        return session

