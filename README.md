# git-cognition

A single Go binary that captures Claude Code session context — tool calls, reasoning, decisions — and stores it in git notes. Query any line of code to see *why* it was written, with syntax-highlighted code and structured reasoning in a two-pane terminal UI.

Optionally uses a local LLM to extract structured reasoning (per-file annotations, key decisions, rejected approaches) at session end via any OpenAI-compatible endpoint.

## Install

```bash
go install github.com/stevemurr/git-cognition@latest
```

Or build from source:

```bash
git clone git@github.com:stevemurr/git-cognition.git
cd git-cognition
make install
```

Verify:

```bash
git-cognition version
```

## Quick start

### 1. Global init

Installs Claude Code hooks and git aliases (`git why`, `git session`):

```bash
git-cognition init
```

This writes to:
- `~/.claude/settings.json` — PostToolUse and Stop hooks
- `~/.gitconfig` — `git why` and `git session` aliases
- `~/.config/git-cognition/config.toml` — default config

### 2. Enable capture in your repo

```bash
cd your-project
git-cognition init --repo
```

If the directory isn't a git repo yet, this runs `git init` automatically. It creates `.git/gc-enabled` and excludes session files from tracking.

### 3. (Optional) Enable LLM reasoning extraction

Set up a local LLM endpoint for structured reasoning extraction. Any OpenAI-compatible API works (LiteLLM, vLLM, Ollama, etc.):

```bash
export GC_LLM_ENDPOINT="http://localhost:4000"
export GC_LLM_API_KEY="your-key"
export GC_LLM_MODEL="your-model"       # default: nemotron3-nano
export GC_LLM_ENABLED=true
```

Or add to `~/.config/git-cognition/config.toml`:

```toml
[llm]
endpoint = "http://localhost:4000"
model = "nemotron3-nano"
enabled = true
timeout_seconds = 30
```

The API key should be set via `GC_LLM_API_KEY` env var rather than config files.

When enabled, the Stop hook sends session data to the LLM at session end to extract:
- A concise summary of what was done and why
- Per-file annotations (what changed, why)
- Key decisions made during the session
- Rejected approaches and alternatives considered

If the LLM is unreachable, extraction is silently skipped and the session is stored with Claude's raw final message only.

### 4. Use Claude Code normally

```bash
claude
```

Ask Claude to do some work — edit files, run tests, whatever:

```
> Add input validation to the signup endpoint
```

Make sure a commit happens during the session (Claude can commit, or you can).

### 5. Exit Claude

When the session ends, the Stop hook fires automatically and captures:
- Every tool call Claude made (files read, edits, commands run)
- Claude's final message — its own summary of what it did and why
- Which commits were made during the session
- The model used (e.g. `claude-opus-4-6`)
- LLM-extracted structured reasoning (if enabled)

Everything is written to git notes.

### 6. Ask why

```bash
git why src/auth/signup.go:42
```

Output shows the code in context with reasoning below:

```
a1b2c3d  src/auth/signup.go:42  ·  claude-opus-4-6  ·  2026-03-29

  39   }
  40
  41   func handleSignup(w http.ResponseWriter, r *http.Request) {
  42 →     if !isValidEmail(r.FormValue("email")) {
  43           http.Error(w, "invalid email", 400)
  44           return
  45       }

  "Added email format validation before the database insert —
   reused the existing RFC 5322 regex in utils.go rather than
   adding a new dependency."
```

When LLM extraction is enabled, the excerpt comes from per-file annotations (what + why). Without LLM data, it falls back to the task prompt, then Claude's final message.

If there's no session data for a commit, `git why` falls back to plain `git blame` output silently.

## Commands

### `git why <file>:<line>`

Traces a line back through git blame to the session that produced it.

```bash
git why app.js:12              # code snippet + reasoning excerpt
git why app.js:12 --verbose    # full reasoning + action log
git why app.js:12 --full       # everything including file contents read
git why app.js:12 --rich       # two-pane layout with syntax highlighting
git why app.js:12 --json       # machine-readable output
```

**Verbose** shows LLM-extracted reasoning (summary, key decisions, rejected approaches) when available, Claude's final message, and the full action log:

```
a1b2c3d  app.js:12  ·  claude-opus-4-6  ·  2026-03-29
session: abc123  ·  task: Add input validation to the signup endpoint

  ...code snippet...

reasoning:
  "Added email validation to the signup endpoint to prevent invalid data..."

key decisions:
  · Reused existing RFC 5322 regex from utils.go
  · Placed validation before database insert

rejected:
  · External validation library — adds dependency for a single check

claude's message:
  "I added email format validation..."

action log:
  Read  src/utils.go
  Read  src/auth/signup.go
  Edit  src/auth/signup.go
  Bash  go test ./...  →  ok  0.51s
```

**Rich** (`--rich`) renders a two-pane terminal layout with syntax-highlighted code on the left and context on the right (commit info, task, tool sequence, reasoning, key decisions, rejected approaches, and Claude's message). Falls back to verbose output when the terminal is narrower than 80 columns or when piped.

**Full** also shows the file contents Claude was reading when it made decisions.

**JSON** includes all session data plus `llm_reasoning` when available.

### `git session ls`

List captured sessions with optional filters:

```bash
git session ls                     # all sessions
git session ls --since 7d          # last 7 days
git session ls --file signup.go    # sessions touching a file
git session ls --model opus        # filter by model
git session ls --limit 10          # cap results
git session ls --json              # JSON output
```

### `git session show <id>`

Full detail view of a session:

```bash
git session show abc123
```

Shows task prompt, commits, Claude's full reasoning (with markdown rendered), and the complete action log with sequence numbers.

### `git session grep <query>`

Search across all sessions:

```bash
git session grep "validation"              # search Claude's reasoning (default)
git session grep "redis" --scope tools     # search tool call inputs/outputs
git session grep "auth" --scope all        # search everything
git session grep "rate limit" --since 14d  # with time filter
```

Matching terms are highlighted in the output. Markdown in matched lines is rendered.

### `git session stat`

Aggregate statistics across all sessions — session count, tool call count, breakdown by model, and most-touched files.

### `git-cognition ingest`

Agent-agnostic capture path. Any AI coding tool can pipe NDJSON:

```bash
my-agent run "add rate limiting" | git-cognition ingest
```

Protocol:

```jsonl
{"type":"session_start","session_id":"xyz","started_at":"...","task":"...","model":"gpt-4o","runner":"my-agent","cwd":"/path"}
{"type":"tool_call","sequence":1,"tool":"read_file","input":{"path":"foo.py"},"output":"<contents>","timestamp":"..."}
{"type":"final_message","content":"I added a rate limiter...","timestamp":"..."}
{"type":"session_end","timestamp":"...","metrics":{"duration_seconds":107}}
```

### `git-cognition version`

Prints the version (embedded at build time from `git describe`).

## How it works

```
Claude Code session
    |
    |-- PostToolUse hook (after each tool call)
    |       \-- appends one NDJSON line to .git/gc-sessions/<id>.ndjson
    |           (file read, edit made, command run -- with truncated output)
    |
    \-- Stop hook (session ends)
            |-- reads .git/gc-sessions/<id>.ndjson
            |-- captures Claude's final message from the hook payload
            |-- extracts task prompt and model from transcript file
            |-- finds commits made during the session via git log
            |-- (if LLM enabled) sends session data to LLM for structured extraction
            |-- assembles session JSON (schema v6.0)
            |-- writes session blob to refs/sessions/<session_id>
            |-- annotates each commit with session ID via git notes
            \-- deletes the session file
```

**Storage** uses two git mechanisms:
- **Session blobs** at `refs/sessions/<session_id>` — full session JSON
- **Commit notes** via `git notes --ref=sessions` — maps each commit SHA to its session ID

**Lookup chain** for `git why`:
`git blame` -> commit SHA -> session note -> session blob -> LLM annotation (or task prompt fallback) -> render with syntax-highlighted code context

Hooks always exit 0. Errors go to stderr. Orphaned session files older than 24 hours are cleaned up automatically.

## Output styling

The `--rich` view uses a two-pane layout with:
- **Left pane** — syntax-highlighted code (via Chroma) with the target line marked
- **Right pane** — commit info, task, tool sequence, reasoning, key decisions, rejected approaches, Claude's message

All views use semantic colors:
- **Yellow** — commit SHAs, session IDs, target line numbers
- **Cyan** — model names, tool names, inline `code`
- **Green** — section headers, arrow marker, title bars
- **Bold** — section headers, target line, file paths, `**emphasis**`
- **Dim** — dates, separators, hints, line numbers

Markdown in Claude's reasoning is rendered inline:
- `**bold text**` renders as bold
- `` `code references` `` render as cyan
- `- bullet items` render with styled markers

Colors auto-disable when output is piped. Override with `--no-color` or the `NO_COLOR` environment variable.

## Configuration

Global config at `~/.config/git-cognition/config.toml`:

```toml
[display]
max_message_lines = 20    # 0 = no truncation

[llm]
endpoint = "http://localhost:4000"
model = "nemotron3-nano"
enabled = true
timeout_seconds = 30
# api_key via GC_LLM_API_KEY env var (preferred)
```

Per-repo overrides at `.git/gc-config` (same format).

Resolution order: `.git/gc-config` > `GC_*` env vars > global config > defaults.

Environment variables:
- `GC_MAX_MESSAGE_LINES` — override display truncation
- `GC_LLM_ENDPOINT` — LLM endpoint URL
- `GC_LLM_API_KEY` — bearer token
- `GC_LLM_MODEL` — model name
- `GC_LLM_ENABLED` — `true` to enable extraction

## Testing

```bash
make test           # Go unit tests
make test-llm       # LLM extraction tests (requires GC_LLM_ENDPOINT and GC_LLM_API_KEY)
make test-e2e       # full e2e with 6 phases (requires Claude Code)
make test-e2e PHASES=2  # run only 2 phases
```

The e2e test creates a temp directory, runs separate Claude sessions to build a Deno todo app in phases, then queries every file with `git why` across all output modes to verify per-file reasoning.

## Command reference

| Command | Description |
|---|---|
| `git-cognition init` | Install hooks and aliases globally |
| `git-cognition init --repo` | Enable capture in current repo (auto git init) |
| `git-cognition why <file>:<line>` | Show why a line was written with code context |
| `git-cognition why --rich` | Two-pane layout with syntax highlighting |
| `git-cognition session ls` | List sessions (`--file`, `--since`, `--model`, `--limit`) |
| `git-cognition session show <id>` | Show session detail |
| `git-cognition session stat` | Aggregate statistics |
| `git-cognition session grep <query>` | Search sessions (`--scope`, `--since`) |
| `git-cognition ingest` | Ingest NDJSON from stdin |
| `git-cognition version` | Print version |

All commands support `--json` for machine-readable output and `--no-color` to disable colors.
