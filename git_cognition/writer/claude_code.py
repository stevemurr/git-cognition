from __future__ import annotations

import re
from typing import Any

from .base import AgentSessionWriter


class ClaudeCodeSessionWriter(AgentSessionWriter):
    """Claude Code adapter using the explicit attach/finalize session lifecycle."""

    REJECTION_PATTERNS = [
        re.compile(r"(?:considered|rejected|ruled out|decided against)[:\s]+(.+?)(?:\.|;|$)", re.I),
        re.compile(r"(?:instead of|rather than)\s+(.+?)(?:,|\.|;|$)", re.I),
    ]

    def __init__(
        self,
        *,
        repo,
        task_prompt: str,
        model: str,
        model_version: str | None = None,
        context_files: list[str] | None = None,
        capture_thinking: bool = False,
    ) -> None:
        super().__init__(
            repo=repo,
            task_prompt=task_prompt,
            model=model,
            runner="claude-code",
            model_version=model_version,
            context_files=context_files,
            capture_thinking=capture_thinking,
        )

    def ingest_response(
        self,
        response: dict[str, Any],
        *,
        duration_seconds: float = 0.0,
        cost_usd: float | None = None,
    ) -> None:
        model = str(response.get("model", self.session.agent.model))
        self.session.agent.model = model
        usage = response.get("usage", {})
        self.record_metrics(
            input_tokens=int(usage.get("input_tokens", 0)),
            output_tokens=int(usage.get("output_tokens", 0)),
            thinking_tokens=int(usage.get("thinking_tokens", 0)),
            cost_usd=cost_usd if cost_usd is not None else self._estimate_cost(usage, model),
            duration_seconds=duration_seconds,
        )
        for block in response.get("content", []):
            block_type = block.get("type")
            if block_type == "tool_use":
                self.record_tool_call(
                    tool=str(block.get("name", "tool_use")),
                    kind=self._infer_kind(str(block.get("name", ""))),
                    paths=self._extract_paths(block.get("input")),
                    raw_input=block.get("input"),
                )
            elif block_type == "thinking":
                self.record_thinking(str(block.get("thinking", "")))
            elif block_type == "text":
                self.ingest_text(str(block.get("text", "")))

    def ingest_tool_result(
        self,
        sequence: int,
        *,
        summary: str,
        raw_output_excerpt: str | None = None,
        output_snapshot: str | None = None,
    ) -> None:
        self.update_tool_call(
            sequence,
            output_summary=summary,
            raw_output_excerpt=raw_output_excerpt,
            output_snapshot=output_snapshot,
        )

    def ingest_text(self, text: str) -> None:
        self._parse_rejections_from_text(text)

    def _parse_rejections_from_text(self, text: str) -> None:
        for pattern in self.REJECTION_PATTERNS:
            for match in pattern.finditer(text):
                what = match.group(1).strip()
                if len(what) >= 8:
                    self.record_rejected(what=what, why="extracted from response text")

    @staticmethod
    def _extract_paths(value: Any) -> list[str]:
        keys = {"path", "file", "filepath", "paths", "files"}
        paths: list[str] = []

        def walk(node: Any) -> None:
            if isinstance(node, dict):
                for key, item in node.items():
                    if key in keys:
                        if isinstance(item, list):
                            for list_item in item:
                                if isinstance(list_item, str):
                                    paths.append(list_item)
                        elif isinstance(item, str):
                            paths.append(item)
                    else:
                        walk(item)
            elif isinstance(node, list):
                for item in node:
                    walk(item)

        walk(value)
        deduped: list[str] = []
        seen: set[str] = set()
        for path in paths:
            if path in seen:
                continue
            seen.add(path)
            deduped.append(path)
        return deduped

    @staticmethod
    def _infer_kind(tool_name: str) -> str:
        name = tool_name.lower()
        if "read" in name or "open" in name or "show" in name:
            return "read"
        if "edit" in name or "write" in name or "patch" in name:
            return "write"
        if "grep" in name or "search" in name or "find" in name:
            return "search"
        if "test" in name:
            return "test"
        if "run" in name or "exec" in name:
            return "run"
        return "generic"

    @staticmethod
    def _estimate_cost(usage: dict[str, Any], model: str) -> float:
        rates = {
            "haiku": (0.25, 1.25),
            "sonnet": (3.0, 15.0),
            "opus": (15.0, 75.0),
        }
        normalized = model.lower()
        for key, (input_rate, output_rate) in rates.items():
            if key in normalized:
                return (
                    int(usage.get("input_tokens", 0)) * input_rate / 1_000_000
                    + int(usage.get("output_tokens", 0)) * output_rate / 1_000_000
                )
        return 0.0
