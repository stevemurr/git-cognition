from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from git_cognition.retrieval import rank_documents
from git_cognition.storage import (
    MissingSessionNoteError,
    get_line_blame,
    get_line_blame_text,
    get_tracked_line_context,
    read_session_for_commit,
)

from .common import format_currency, json_dump, session_prompt_lines


@dataclass(slots=True)
class Evidence:
    kind: str
    score: float
    label: str
    body: str
    sequence: int | None = None


def parse_file_line(value: str) -> tuple[str, int]:
    if ":" not in value:
        raise ValueError("expected <file>:<line>")
    file_path, line_text = value.rsplit(":", 1)
    return file_path, int(line_text)


def path_matches(query_path: str, candidate_paths: list[str]) -> bool:
    normalized = query_path.strip("/")
    for candidate in candidate_paths:
        cleaned = candidate.strip("/")
        if cleaned == normalized or cleaned.endswith(f"/{normalized}") or normalized.endswith(f"/{cleaned}"):
            return True
    return False


def rank_evidence(session, file_path: str, line_number: int, query_text: str) -> list[Evidence]:
    preferred_calls = [call for call in session.tool_calls if path_matches(file_path, call.paths)]
    candidates = preferred_calls if preferred_calls else session.tool_calls

    documents: list[str] = []
    evidence: list[Evidence] = []
    for call in candidates:
        documents.append(call.search_text())
        label = call.tool
        if call.paths:
            label = f"{call.tool}({', '.join(call.paths)})"
        evidence.append(
            Evidence(
                kind="tool_call",
                sequence=call.sequence,
                score=0.0,
                label=label,
                body=call.output_summary or call.raw_output_excerpt or call.search_text(),
            )
        )
    for rejected in session.rejected_approaches:
        documents.append(rejected.search_text())
        evidence.append(
            Evidence(
                kind="rejected",
                sequence=None,
                score=0.0,
                label=rejected.what,
                body=rejected.why,
            )
        )

    rankings = rank_documents(query_text, documents)
    ordered: list[Evidence] = []
    for index, score in rankings:
        item = evidence[index]
        item.score = score
        ordered.append(item)
    return ordered


def render_default(commit_sha: str, file_path: str, line_number: int, session, ranked: list[Evidence]) -> None:
    print(f"{commit_sha[:7]}  {file_path}:{line_number}")
    for line in session_prompt_lines(session):
        print(line)
    if session.agent.external_session_id:
        print(f"claude:  {session.agent.external_session_id}")
    if ranked:
        top = ranked[0]
        print(f"why:     {top.body}")
    else:
        print("why:     No ranked session evidence found.")


def render_verbose(commit_sha: str, file_path: str, line_number: int, session_id: str, session, ranked: list[Evidence], *, full: bool) -> None:
    print(
        f"{commit_sha[:7]}  {file_path}:{line_number}  ·  "
        f"{session.agent.model}  ·  {(session.completed_at or session.created_at)[:10]}"
    )
    print(f"session: {session_id[:8]}")
    if session.agent.external_session_id:
        print(f"claude:  {session.agent.external_session_id}")
    print()
    for line in session_prompt_lines(session):
        print(line)
    print()
    if full:
        print(f"tool call sequence ({len(session.tool_calls)} total):")
        for call in session.tool_calls:
            label = call.tool
            if call.paths:
                label = f"{call.tool}({', '.join(call.paths)})"
            print(f"  {call.sequence}. {label}  ->  {call.output_summary or call.raw_output_excerpt or '[no summary]'}")
    else:
        print("relevant tool calls:")
        for item in [e for e in ranked if e.kind == "tool_call"][:3]:
            sequence = item.sequence or 0
            print(f"  {sequence}. {item.label}  ->  {item.body}")
    print()
    print("rejected:")
    rejected_items = [item for item in ranked if item.kind == "rejected"] if ranked else []
    if rejected_items:
        for item in rejected_items[:3]:
            print(f"  - {item.label} - {item.body}")
    else:
        print("  [none]")
    print()
    print(
        "metrics:  "
        f"{session.metrics.input_tokens:,} input · "
        f"{session.metrics.output_tokens:,} output · "
        f"{format_currency(session.metrics.cost_usd)} · "
        f"{session.metrics.duration_seconds:.0f}s"
    )


def run(args) -> int:
    repo = str(Path(".").resolve())
    file_path, line_number = parse_file_line(args.location)
    commit_sha = get_line_blame(repo, file_path, line_number)
    try:
        session_id, session = read_session_for_commit(repo, commit_sha)
    except MissingSessionNoteError:
        print(get_line_blame_text(repo, file_path, line_number), end="")
        return 0

    query_text = get_tracked_line_context(repo, commit_sha, file_path, line_number)
    ranked = rank_evidence(session, file_path, line_number, query_text)

    if args.json:
        print(
            json_dump(
                {
                    "commit": commit_sha,
                    "file": file_path,
                    "line": line_number,
                    "session_id": session_id,
                    "external_session_id": session.agent.external_session_id,
                    "session": session.to_dict(),
                    "retrieval": {
                        "method": "path-filtered-bm25",
                        "top_matches": [
                            {
                                "type": item.kind,
                                "sequence": item.sequence,
                                "score": round(item.score, 6),
                                "label": item.label,
                            }
                            for item in ranked[:5]
                        ],
                    },
                }
            )
        )
        return 0

    if args.full:
        render_verbose(commit_sha, file_path, line_number, session_id, session, ranked, full=True)
    elif args.verbose:
        render_verbose(commit_sha, file_path, line_number, session_id, session, ranked, full=False)
    else:
        render_default(commit_sha, file_path, line_number, session, ranked)
    return 0
