# git-cognition v1 spec
## `git why` and `git session` — prototype specification

**Status:** ready for implementation  
**Language:** Python  
**Storage:** git notes (`refs/sessions/*`)  
**Scope:** `git why`, `git session {init,ls,show,stat,grep}`  
**Deferred to v2:** full thinking blocks, semantic embeddings, privacy/redaction model, `git replay`, `git diff-intent`

---

## Decisions log

| Question | Decision |
|---|---|
| Agent runner(s) | Claude Code first; architecture agent-agnostic |
| Session storage | Git notes (`refs/sessions/*`) — travels with repo on clone |
| Primary use case | All equal: debugging, code review, audit trail |
| Thinking traces in v1 | Schema supports full blocks; populated as `[]` until v2 |
| Verbosity stored | Full raw thinking blocks (schema ready, empty in v1) |
| `ls` filter priority | file > date > model > cost |
| No-session fallback | Silent fallback to plain `git blame` output |
| Default output | One-liner: task + 1 sentence why |
| Implementation | Python |
| Semantic retrieval | BM25/keyword only in v1 *(assumed — no answer received)* |
| Remote push | Local only for prototype *(assumed)* |
| Activation | Opt-in per repo via `git session init` *(assumed)* |

---

## Architecture overview

```
┌─────────────────────────────────────────────────────────┐
│  agent runner (Claude Code / any runner via adapter)    │
│                                                         │
│  AgentSessionWriter (agent-agnostic interface)          │
│    .begin_session(task, model, metadata)                │
│    .record_tool_call(name, input, output)               │
│    .record_thinking(blocks)     ← no-op in v1           │
│    .record_rejected(what, why)                          │
│    .commit_session(commit_sha)  ← writes git note       │
└───────────────┬─────────────────────────────────────────┘
                │ post-commit hook fires
                ▼
┌─────────────────────────────────────────────────────────┐
│  git notes (refs/sessions/*)                            │
│                                                         │
│  note on commit SHA: { "session_id": "a3f9c2..." }      │
│  blob at refs/sessions/<session_id>: session JSON       │
└───────────────┬─────────────────────────────────────────┘
                │
        ┌───────┴────────┐
        ▼                ▼
   git why           git session
   file:line         ls / show / stat / grep
```

The writer interface is the agent-agnostic seam. The Claude Code adapter implements it by parsing Claude's response stream. Any other runner implements the same interface — the storage layer is identical regardless of source.

---

## Session object schema

Stored as a JSON blob in `refs/sessions/<session_id>`.  
All fields present in v1. Thinking blocks stored as `[]` until v2.

```json
{
  "schema_version": "1.0",
  "session_id": "<uuid4-hex>",
  "created_at": "<iso8601>",
  "completed_at": "<iso8601>",

  "agent": {
    "runner": "claude-code",
    "model": "claude-sonnet-4-6",
    "model_version": "20250514"
  },

  "task": {
    "prompt": "Add rate limiting to the auth endpoint",
    "context_files": ["auth/middleware.py", "redis_client.py"]
  },

  "commits": [
    {
      "sha": "8b2e4f3",
      "message": "feat: token bucket rate limiter on /auth",
      "files_changed": ["auth/middleware.py", "tests/test_auth.py"]
    }
  ],

  "tool_calls": [
    {
      "sequence": 1,
      "tool": "read_file",
      "input": { "path": "redis_client.py" },
      "output_summary": "Found INCRBY/EXPIRE helpers in connection pool",
      "output_snapshot": null
    },
    {
      "sequence": 2,
      "tool": "edit_file",
      "input": { "path": "auth/middleware.py", "description": "add token bucket" },
      "output_summary": "Edit applied successfully",
      "output_snapshot": null
    }
  ],

  "thinking_blocks": [],

  "rejected_approaches": [
    {
      "what": "Sliding window via ZADD/ZRANGEBYSCORE",
      "why": "Requires Lua script for atomicity; 2 round trips on hot auth path"
    }
  ],

  "metrics": {
    "input_tokens": 4821,
    "output_tokens": 1203,
    "thinking_tokens": 0,
    "cost_usd": 0.09,
    "duration_seconds": 47
  }
}
```

### Field notes

- `schema_version` enables forward-compatible migrations. Always check before reading.
- `output_snapshot` on tool calls is `null` in v1. In v2 (replay support), this stores the full file content returned by `read_file` calls.
- `thinking_blocks` is always `[]` in v1. In v2, each entry is `{"sequence": N, "text": "..."}`. Code that reads this field must treat an empty array as "not yet captured", not "agent did not think".
- `context_files` on the task is populated by the runner if the user passed files as context. Optional.
- `rejected_approaches` is populated by parsing the agent's final response for structured rejection language, or by the runner explicitly calling `record_rejected()`. May be empty.

---

## Storage layout

### Git notes namespace

```
refs/notes/sessions/       ← note namespace (one note per commit)
refs/sessions/<id>         ← session blob namespace (one blob per session)
```

**Note on a commit** — written at commit time by the post-commit hook:
```
session_id: a3f9c2d1e4f7b8c9...
```

**Session blob** — written at session completion:
```
refs/sessions/a3f9c2d1e4f7b8c9...  →  <session JSON>
```

### Why two namespaces

Git notes attach metadata to commits. They can only hold a string, not a multi-kilobyte JSON payload (technically they can, but it slows `git log` which reads notes). The note holds only the session ID pointer. The actual session data lives in its own ref, readable without touching the commit graph.

### Reading a session for a given commit

```python
# 1. Read the note on the commit
note = subprocess.run(
    ["git", "notes", "--ref=sessions", "show", commit_sha],
    capture_output=True, text=True
)
session_id = note.stdout.strip().replace("session_id: ", "")

# 2. Read the session blob
blob = subprocess.run(
    ["git", "cat-file", "blob", f"refs/sessions/{session_id}"],
    capture_output=True, text=True
)
session = json.loads(blob.stdout)
```

### Writing a session

```python
# 1. Write the blob
blob_hash = subprocess.run(
    ["git", "hash-object", "-w", "--stdin"],
    input=json.dumps(session), capture_output=True, text=True
).stdout.strip()

# 2. Create the ref
subprocess.run(["git", "update-ref",
    f"refs/sessions/{session_id}", blob_hash])

# 3. Attach note to commit
subprocess.run(["git", "notes", "--ref=sessions",
    "add", "-m", f"session_id: {session_id}", commit_sha])
```

### Push/fetch behaviour

By default git does not push custom refs. Users who want session traces to travel to a remote need:

```bash
# Push sessions to remote
git push origin 'refs/sessions/*:refs/sessions/*'
git push origin 'refs/notes/sessions:refs/notes/sessions'

# Fetch sessions from remote
git fetch origin 'refs/sessions/*:refs/sessions/*'
git fetch origin 'refs/notes/sessions:refs/notes/sessions'
```

`git session init` writes these as custom refspecs in `.git/config` so push/fetch work automatically. **This is opt-in and not set by default in v1**, since the repo is local-only for the prototype and remote privacy implications are deferred.

---

## `git session init`

Bootstraps a repo for session tracking. Must be run before any sessions are captured.

```bash
git session init
```

**What it does:**
1. Creates `.git/hooks/post-commit` (or appends to existing hook) with the session write call
2. Creates `.git/config` entries for the sessions refspec
3. Creates `.agentcognition` marker file at repo root (gitignored by default) recording init timestamp and schema version
4. Prints confirmation and next steps

**Idempotent** — safe to run multiple times, detects existing installation.

**Hook content written:**
```bash
#!/bin/sh
# git-cognition: write pending session note if one exists
if [ -f .git/agent-session-pending.json ]; then
  git-session-commit-hook
fi
```

The hook is minimal — the real logic lives in `git-session-commit-hook`, a Python script installed on PATH by the package.

---

## AgentSessionWriter — the adapter interface

```python
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Optional
import uuid, time, json, subprocess

@dataclass
class ToolCall:
    sequence: int
    tool: str
    input: dict
    output_summary: str
    output_snapshot: Optional[str] = None  # null in v1, used in v2 replay

@dataclass
class RejectedApproach:
    what: str
    why: str

class AgentSessionWriter(ABC):
    """
    Agent-agnostic interface for recording a session.
    Implement this for each runner. The storage layer is shared.
    """

    def __init__(self, task_prompt: str, model: str, runner: str):
        self.session_id = uuid.uuid4().hex
        self.task_prompt = task_prompt
        self.model = model
        self.runner = runner
        self.created_at = time.time()
        self._tool_calls: list[ToolCall] = []
        self._thinking_blocks: list[dict] = []   # always empty in v1
        self._rejected: list[RejectedApproach] = []
        self._commits: list[dict] = []
        self._metrics: dict = {}

    def record_tool_call(self, tool: str, input: dict, output_summary: str,
                          output_snapshot: Optional[str] = None):
        seq = len(self._tool_calls) + 1
        self._tool_calls.append(ToolCall(seq, tool, input,
                                          output_summary, output_snapshot))

    def record_thinking(self, text: str):
        """No-op in v1. Called by runner if thinking blocks become available."""
        seq = len(self._thinking_blocks) + 1
        self._thinking_blocks.append({"sequence": seq, "text": text})

    def record_rejected(self, what: str, why: str):
        self._rejected.append(RejectedApproach(what, why))

    def record_metrics(self, input_tokens: int, output_tokens: int,
                        thinking_tokens: int, cost_usd: float, duration_seconds: float):
        self._metrics = {
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "thinking_tokens": thinking_tokens,
            "cost_usd": cost_usd,
            "duration_seconds": duration_seconds,
        }

    def commit_session(self, commit_sha: str):
        """Call after git commit. Writes blob + note."""
        self._commits.append({"sha": commit_sha[:7]})
        session = self._build_session_object(commit_sha)
        self._write_to_git(session)

    def _build_session_object(self, commit_sha: str) -> dict:
        return {
            "schema_version": "1.0",
            "session_id": self.session_id,
            "created_at": self._iso(self.created_at),
            "completed_at": self._iso(time.time()),
            "agent": {
                "runner": self.runner,
                "model": self.model,
            },
            "task": {"prompt": self.task_prompt},
            "commits": self._commits,
            "tool_calls": [self._tc_to_dict(tc) for tc in self._tool_calls],
            "thinking_blocks": self._thinking_blocks,
            "rejected_approaches": [
                {"what": r.what, "why": r.why} for r in self._rejected
            ],
            "metrics": self._metrics,
        }

    def _write_to_git(self, session: dict):
        payload = json.dumps(session, indent=2)
        blob = subprocess.run(
            ["git", "hash-object", "-w", "--stdin"],
            input=payload, capture_output=True, text=True
        ).stdout.strip()
        subprocess.run(["git", "update-ref",
            f"refs/sessions/{self.session_id}", blob], check=True)
        subprocess.run(["git", "notes", "--ref=sessions", "add",
            "-m", f"session_id: {self.session_id}",
            self._commits[-1]["sha"]], check=True)

    @staticmethod
    def _iso(ts: float) -> str:
        from datetime import datetime, timezone
        return datetime.fromtimestamp(ts, tz=timezone.utc).isoformat()

    @staticmethod
    def _tc_to_dict(tc: ToolCall) -> dict:
        return {
            "sequence": tc.sequence,
            "tool": tc.tool,
            "input": tc.input,
            "output_summary": tc.output_summary,
            "output_snapshot": tc.output_snapshot,
        }
```

### Claude Code adapter (v1)

Claude Code exposes tool calls and final response text. The adapter parses these.

```python
class ClaudeCodeSessionWriter(AgentSessionWriter):
    """
    Adapter for Claude Code. Call from a post-run hook or wrapper script.
    Parses the Claude API response to extract tool calls and rejected approaches.
    """

    REJECTION_PATTERNS = [
        r"(?:considered|rejected|ruled out|decided against)[:\s]+(.+?)(?:\.|—|;|$)",
        r"(?:instead of|rather than)\s+(.+?)(?:,|\.|—|$)",
    ]

    def __init__(self, task_prompt: str, model: str):
        super().__init__(task_prompt, model, runner="claude-code")

    def ingest_api_response(self, response: dict):
        """
        Pass the raw Anthropic API response object.
        Extracts tool_use blocks, thinking blocks (if present), and metrics.
        """
        usage = response.get("usage", {})
        self.record_metrics(
            input_tokens=usage.get("input_tokens", 0),
            output_tokens=usage.get("output_tokens", 0),
            thinking_tokens=usage.get("thinking_tokens", 0),
            cost_usd=self._estimate_cost(usage, response.get("model", "")),
            duration_seconds=0,  # caller should time this externally
        )

        for block in response.get("content", []):
            if block["type"] == "tool_use":
                # Tool result must be passed separately — see ingest_tool_result
                self.record_tool_call(
                    tool=block["name"],
                    input=block.get("input", {}),
                    output_summary="",  # filled by ingest_tool_result
                )
            elif block["type"] == "thinking":
                # No-op in v1 — data captured for v2
                self.record_thinking(block.get("thinking", ""))
            elif block["type"] == "text":
                self._parse_rejections_from_text(block.get("text", ""))

    def ingest_tool_result(self, sequence: int, summary: str):
        """Update output_summary for a tool call after result is known."""
        if 0 < sequence <= len(self._tool_calls):
            self._tool_calls[sequence - 1].output_summary = summary

    def _parse_rejections_from_text(self, text: str):
        import re
        for pattern in self.REJECTION_PATTERNS:
            for match in re.finditer(pattern, text, re.IGNORECASE):
                what = match.group(1).strip()
                if len(what) > 10:  # skip noise
                    self.record_rejected(what=what, why="(extracted from response)")

    @staticmethod
    def _estimate_cost(usage: dict, model: str) -> float:
        # Approximate pricing per million tokens
        rates = {
            "claude-sonnet": (3.0, 15.0),
            "claude-opus":   (15.0, 75.0),
            "claude-haiku":  (0.25, 1.25),
        }
        for key, (inp_rate, out_rate) in rates.items():
            if key in model.lower():
                return (
                    usage.get("input_tokens", 0) * inp_rate / 1_000_000 +
                    usage.get("output_tokens", 0) * out_rate / 1_000_000
                )
        return 0.0
```

---

## `git why` — CLI specification

### Invocation

```bash
git why <file>:<line>
git why <file>:<line> --verbose
git why <file>:<line> --full
git why <file>:<line> --json
```

### Lookup chain

```
file:line
  → git blame → commit SHA
  → git notes --ref=sessions show <SHA>
      → no note: silent fallback to git blame output
      → note found: read session_id
  → git cat-file blob refs/sessions/<session_id>
      → parse session JSON
  → BM25 retrieval: find most relevant tool calls for this file/line
  → render output at requested verbosity
```

### Retrieval (v1: BM25 keyword matching)

The line number alone isn't enough context. The retrieval step finds which part of the session is most relevant to the specific line being queried.

**Input to retrieval:** file path + surrounding 5 lines of code (extracted via `git show`)

**Corpus to search:** all `output_summary` strings from tool calls in the session, plus any rejected approach descriptions

**Algorithm:** BM25 over tokenized strings. Python implementation via `rank_bm25` library (no external service, no embeddings in v1).

**Output:** ranked list of tool calls and rejected approaches by relevance. Top result(s) used to populate `--verbose` and `--full` output.

**v2 upgrade path:** replace BM25 with embedding lookup against local vLLM/Ollama. The retrieval interface takes `(query_text: str, corpus: list[str]) -> list[int]` — swap the implementation, keep the call sites.

### Output tiers

**Default (no flags)** — one-liner:

```
8b2e4f3  auth/middleware.py:47
task:    Add rate limiting to the auth endpoint
why:     Chose token bucket over sliding window — Redis INCRBY/EXPIRE already
         available in connection pool, no extra dependencies needed.
```

**`--verbose`** — task + ranked tool calls + rejected approaches:

```
8b2e4f3  auth/middleware.py:47  ·  claude-sonnet-4-6  ·  2026-03-27
session: a3f9c2d1

task:    Add rate limiting to the auth endpoint

relevant tool calls:
  1. read_file(redis_client.py)  →  Found INCRBY/EXPIRE helpers in connection pool
  2. edit_file(auth/middleware.py)  →  Token bucket logic inserted at line 44

rejected:
  · Sliding window via ZADD/ZRANGEBYSCORE — Lua script required for atomicity;
    2 round trips on hot auth path

  git why auth/middleware.py:47 --full  for full session trace
```

**`--full`** — everything, including thinking blocks when available:

```
[same header as --verbose]

tool call sequence (4 total):
  1. read_file(redis_client.py)  →  Found INCRBY/EXPIRE helpers
  2. read_file(auth/middleware.py)  →  Located insertion point at line 44
  3. edit_file(auth/middleware.py)  →  Token bucket logic written
  4. run_tests(tests/test_auth.py)  →  3/3 passed

thinking blocks:  [none captured — v1]

rejected:
  · Sliding window via ZADD/ZRANGEBYSCORE — see above
  · In-process counter — doesn't survive multiple workers

metrics:  4,821 input · 1,203 output · $0.09 · 47s

  git session show a3f9c2d1  for full session
```

**`--json`** — machine-readable, full session object plus retrieval metadata:

```json
{
  "commit": "8b2e4f3",
  "file": "auth/middleware.py",
  "line": 47,
  "session_id": "a3f9c2d1...",
  "session": { ... },
  "retrieval": {
    "method": "bm25",
    "top_matches": [
      { "type": "tool_call", "sequence": 2, "score": 0.91 }
    ]
  }
}
```

### No-session fallback

If no note exists on the commit, `git why` runs `git blame` on the same file/line and outputs it verbatim, with no additional markers. The output is indistinguishable from plain `git blame`. This is intentional — no annotation for human-written lines.

---

## `git session` — CLI specification

### Subcommands

```bash
git session init               # enable session tracking in this repo
git session ls [options]       # list sessions
git session show <id>          # full detail for one session
git session stat               # aggregate analytics
git session grep <query>       # search across sessions
```

---

### `git session ls`

```bash
git session ls
git session ls --file auth/middleware.py
git session ls --since 7d
git session ls --model sonnet
git session ls --limit 20
git session ls --json
```

**Filter flags (in priority order per spec):**

| Flag | Description |
|---|---|
| `--file <path>` | sessions that touched this file (matches `files_changed` across all commits) |
| `--since <duration>` | e.g. `7d`, `2w`, `1m`; parsed relative to now |
| `--until <duration>` | upper bound, same format |
| `--model <substring>` | matches against `agent.model` (e.g. `sonnet`, `opus`) |
| `--min-cost <n>` | filter by `metrics.cost_usd` |
| `--max-cost <n>` | upper bound on cost |
| `--limit <n>` | default 50 |

**Default output (tabular):**

```
SESSION   DATE        MODEL            FILES  COST   TASK
a3f9c2    2026-03-27  sonnet-4-6         3   $0.09  Add rate limiting to auth endpoint
b71e8d    2026-03-26  sonnet-4-6         7   $0.34  Refactor database connection pooling
c4a1f0    2026-03-25  opus-4-6           2   $0.21  Fix JWT token expiry edge case
────────────────────────────────────────────────────────
3 sessions · $0.64 total
```

**How sessions are enumerated:**

All refs under `refs/sessions/*` are listed via `git for-each-ref refs/sessions/`. Each ref name is the session ID. The session JSON is fetched and filtered in Python. This is O(n) over all sessions — acceptable for a prototype. If a repo accumulates thousands of sessions, add a local index (SQLite) in v2.

---

### `git session show <id>`

```bash
git session show a3f9c2
git session show a3f9c2 --thinking    # show thinking blocks when available (v2)
git session show a3f9c2 --json
```

**Output:**

```
a3f9c2  ·  claude-sonnet-4-6  ·  2026-03-27 14:33  ·  47s  ·  $0.09

task:    Add rate limiting to the auth endpoint

commits:
  8b2e4f3  feat: token bucket rate limiter on /auth
  c91a3d1  test: rate limiter unit tests

tool calls:
  1.  read_file(redis_client.py)
      → Found INCRBY/EXPIRE helpers in connection pool
  2.  read_file(auth/middleware.py)
      → Located insertion point at line 44
  3.  edit_file(auth/middleware.py)
      → Token bucket logic written
  4.  run_tests(tests/test_auth.py)
      → 3/3 passed

thinking:  [none captured — v1]

rejected:
  · Sliding window via ZADD/ZRANGEBYSCORE
    Lua script required for atomicity; 2 round trips on hot auth path
  · In-process counter
    Doesn't survive multiple workers or restarts

metrics:  4,821 input · 1,203 output · $0.09

  git why auth/middleware.py:<line>  to trace a specific line
```

---

### `git session stat`

```bash
git session stat
git session stat --since 30d
```

**Output:**

```
sessions:    18 total  ·  12 last 30d
spend:       $4.23 total  ·  $0.24 avg per session
commits:     47 agent commits

by model:
  sonnet-4-6   14 sessions  $3.12  ████████████████████░░░░░
  opus-4-6      3 sessions  $0.94  ██████░░░░░░░░░░░░░░░░░░░
  haiku-4-5     1 session   $0.17  █░░░░░░░░░░░░░░░░░░░░░░░░

most-touched files (by session count):
  auth/middleware.py    6 sessions  14 commits
  db/connection.py      4 sessions   9 commits
  tests/test_auth.py    4 sessions   7 commits
  api/routes.py         3 sessions   5 commits

human/agent commit ratio:  1.7 : 1
```

**Implementation note:** `stat` iterates all session blobs, aggregates in Python dicts. No database needed for prototype-scale repos (< 500 sessions).

---

### `git session grep <query>`

```bash
git session grep "token bucket"
git session grep "foreign key" --scope thinking    # v2: when thinking blocks present
git session grep "rate limit" --scope tasks
git session grep "redis" --scope tools
git session grep "cache" --since 14d
git session grep "auth" --session a3f9c2           # scope to one session
```

**Scope values:**

| Scope | Searches |
|---|---|
| `thinking` (default) | `thinking_blocks[*].text` — empty in v1, no-op |
| `tasks` | `task.prompt` |
| `tools` | `tool_calls[*].input` + `tool_calls[*].output_summary` |
| `rejected` | `rejected_approaches[*].what` + `*.why` |
| `all` | all of the above |

**v1 practical note:** since `thinking_blocks` is empty in v1, the default scope `thinking` returns no results. Users should use `--scope tools` or `--scope all` for v1. This is called out in the help text.

**Output:**

```
searching: "token bucket"  scope: tools  (3 matches across 2 sessions)

a3f9c2  ·  2026-03-27  ·  tool: read_file
  "Found INCRBY/EXPIRE helpers — token bucket pattern fits cleanly"

a3f9c2  ·  2026-03-27  ·  tool: edit_file
  "Token bucket logic written at auth/middleware.py:44"

b71e8d  ·  2026-03-26  ·  tool: read_file
  "Considered token bucket at app layer — rejected for connection pooling"

  git session show <id>  to open a session
```

**Algorithm:** Python `str.lower()` substring match across serialized field values. BM25 ranking available as opt-in (`--bm25`) via `rank_bm25` library for better result ordering.

---

## Python package structure

```
git-cognition/
├── pyproject.toml
├── README.md
├── git_cognition/
│   ├── __init__.py
│   ├── writer/
│   │   ├── base.py               # AgentSessionWriter ABC
│   │   └── claude_code.py        # ClaudeCodeSessionWriter
│   ├── storage/
│   │   ├── git_notes.py          # read/write session blobs
│   │   └── schema.py             # Session dataclass + validation
│   ├── retrieval/
│   │   └── bm25.py               # BM25 retrieval, v2 swap point
│   ├── commands/
│   │   ├── why.py                # git why implementation
│   │   ├── session_ls.py
│   │   ├── session_show.py
│   │   ├── session_stat.py
│   │   ├── session_grep.py
│   │   └── session_init.py
│   └── cli.py                    # argparse entry point
└── scripts/
    └── git-session-commit-hook   # minimal script called by post-commit
```

### Entry points (`pyproject.toml`)

```toml
[project.scripts]
git-why = "git_cognition.cli:why_main"
git-session = "git_cognition.cli:session_main"
git-session-commit-hook = "git_cognition.cli:hook_main"
```

These register as git subcommands automatically when on PATH — `git why` invokes `git-why`, `git session` invokes `git-session`.

### Dependencies

```toml
[project.dependencies]
click = ">=8.0"          # CLI framework
rank-bm25 = ">=0.2"      # BM25 retrieval
rich = ">=13.0"          # terminal output formatting
```

No external services, no database, no embeddings library in v1.

---

## v2 deferral register

These decisions are explicitly deferred. The v1 schema and interfaces are designed to accommodate them without migration.

| Feature | Why deferred | v1 hook point |
|---|---|---|
| Full thinking blocks | Claude Code API integration needed; extended thinking must be explicitly enabled per run | `thinking_blocks` field exists in schema, writer has `record_thinking()`, parser calls it (no-op) |
| Semantic embeddings for `git why` | Requires embedding model integration (local vLLM or API); BM25 is good enough for prototype | `retrieval/bm25.py` is behind an interface — swap implementation without touching call sites |
| `output_snapshot` on tool calls | Needed for `git replay`; adds significant storage | Field exists in schema as `null`; writer accepts it as optional param |
| `git replay` | Needs snapshot data + non-determinism model | Blocked on `output_snapshot` population |
| `git diff-intent` | Needs reasoning trace diffing research | Blocked on thinking blocks + replay |
| Privacy / redaction | Differential privacy is a research track; remote push policy needs design | `git session init` does not configure remote refspecs by default in v1 |
| Remote push defaults | Privacy model not settled | Refspecs documented but not auto-configured |
| Performance index | SQLite sidecar for `ls`/`stat` at scale | `storage/git_notes.py` is abstracted; index layer can be added behind same interface |
| Multi-agent provenance | Session DAG for collaborative sessions | `commits` array supports multiple entries; `agent` field supports extension |

---

## Open assumptions (awaiting confirmation)

These three questions were not answered during spec collection. Decisions made are conservative and easily changed:

1. **Semantic retrieval** — assumed BM25. If you want local embeddings from the DGX from day one, change `retrieval/bm25.py` to call your Ollama/vLLM endpoint. The interface is `retrieve(query: str, corpus: list[str]) -> list[int]`.

2. **Remote push** — assumed local-only. No remote refspecs configured by `git session init`. Change by adding push/fetch refspecs to the init command when remote policy is settled.

3. **Activation model** — assumed opt-in per repo via `git session init`. Change to global config by adding `~/.gitconfig` read in `storage/git_notes.py` before any write operation.
