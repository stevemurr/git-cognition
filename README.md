# git-cognition

A single Go binary that captures Claude Code session context — tool calls, reasoning, decisions — and stores it in git notes. Query any line of code to see *why* it was written, with the surrounding code and Claude's verbatim explanation.

No model calls. No network. No database. Pure git.

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

### 3. Use Claude Code normally

```bash
claude
```

Ask Claude to do some work — edit files, run tests, whatever:

```
> Add input validation to the signup endpoint
```

Make sure a commit happens during the session (Claude can commit, or you can).

### 4. Exit Claude

When the session ends, the Stop hook fires automatically and captures:
- Every tool call Claude made (files read, edits, commands run)
- Claude's final message — its own summary of what it did and why
- Which commits were made during the session
- The model used (e.g. `claude-opus-4-6`)

Everything is written to git notes. No model is called. The final message is stored verbatim.

### 5. Ask why

```bash
git why src/auth/signup.go:42
```

Output shows the code in context with Claude's reasoning below:

```
a1b2c3d  src/auth/signup.go:42  ·  claude-opus-4-6  ·  2026-03-29

  39   }
  40
  41   func handleSignup(w http.ResponseWriter, r *http.Request) {
  42 →     if !isValidEmail(r.FormValue("email")) {
  43           http.Error(w, "invalid email", 400)
  44           return
  45       }

  "I added email format validation before the database insert —
   the existing regex in utils.go already handles RFC 5322, so
   I reused that rather than adding a new dependency."
```

The target line is highlighted with `→` and surrounded by 3 lines of context. The quote is the most relevant excerpt from Claude's final message, selected by BM25 ranking.

If there's no session data for a commit, `git why` falls back to plain `git blame` output silently.

## Commands

### `git why <file>:<line>`

Traces a line back through git blame to the session that produced it.

```bash
git why app.js:12              # code snippet + reasoning excerpt
git why app.js:12 --verbose    # full reasoning + action log
git why app.js:12 --full       # everything including file contents read
git why app.js:12 --json       # machine-readable output
```

**Verbose** adds the full action log showing every tool call Claude made:

```
a1b2c3d  app.js:12  ·  claude-opus-4-6  ·  2026-03-29
session: abc123  ·  task: Add input validation to the signup endpoint

  ...code snippet...

claude's reasoning:
  "I added email format validation..."

action log:
  Read  src/utils.go
  Read  src/auth/signup.go
  Edit  src/auth/signup.go
  Bash  go test ./...  →  ok  0.51s
```

**Full** also shows the file contents Claude was reading when it made decisions.

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

Shows task prompt, commits, Claude's full reasoning, and the complete action log with sequence numbers.

### `git session grep <query>`

Search across all sessions:

```bash
git session grep "validation"              # search Claude's reasoning (default)
git session grep "redis" --scope tools     # search tool call inputs/outputs
git session grep "auth" --scope all        # search everything
git session grep "rate limit" --since 14d  # with time filter
```

Matching terms are highlighted in the output.

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

If `final_message` is present, it's stored verbatim with `source: "ingest_provided"`. If absent, reasoning is empty with `source: "none"`. No inference, no fallback.

### `git-cognition version`

Prints the version (embedded at build time from `git describe`).

## How it works

```
Claude Code session
    │
    ├── PostToolUse hook (after each tool call)
    │       └── appends one NDJSON line to .git/gc-sessions/<id>.ndjson
    │           (file read, edit made, command run — with truncated output)
    │
    └── Stop hook (session ends)
            ├── reads .git/gc-sessions/<id>.ndjson
            ├── captures Claude's final message from the hook payload
            ├── extracts task prompt and model from transcript file
            ├── finds commits made during the session via git log
            ├── assembles session JSON (schema v5.0)
            ├── writes session blob to refs/sessions/<session_id>
            ├── annotates each commit with session ID via git notes
            └── deletes the session file
```

**Storage** uses two git mechanisms:
- **Session blobs** at `refs/sessions/<session_id>` — full session JSON
- **Commit notes** via `git notes --ref=sessions` — maps each commit SHA to its session ID

**Lookup chain** for `git why`:
`git blame` → commit SHA → session note → session blob → BM25 excerpt → render with code context

Hooks always exit 0. Errors go to stderr. Orphaned session files older than 24 hours are cleaned up automatically.

## Colored output

Output uses semantic colors following git conventions:
- **Yellow** — commit SHAs, session IDs
- **Cyan** — model names, tool names
- **Green** — Claude's reasoning quotes
- **Bold** — section headers, target line, file paths
- **Dim** — dates, separators, hints

Colors auto-disable when output is piped. Override with `--no-color` or the `NO_COLOR` environment variable.

## Configuration

Global config at `~/.config/git-cognition/config.toml`:

```toml
[display]
max_message_lines = 20    # 0 = no truncation
```

Per-repo overrides at `.git/gc-config` (same format).

Resolution order: `.git/gc-config` > `GC_*` env vars > global config > defaults.

## Testing it end to end

```bash
# Build and install
cd git-cognition
make install
git-cognition version              # verify build

# Set up
git-cognition init                 # global hooks + aliases
mkdir /tmp/test-project && cd /tmp/test-project
git-cognition init --repo          # auto-runs git init + enables capture

# Use Claude Code
claude
# > "Create a hello.go that prints hello world, then commit it"
# (exit claude)

# Query
git session ls                     # see captured session
git session show <session-id>      # full detail
git why hello.go:3                 # code snippet + reasoning
git session grep "hello"           # search across sessions
```

## Command reference

| Command | Description |
|---|---|
| `git-cognition init` | Install hooks and aliases globally |
| `git-cognition init --repo` | Enable capture in current repo (auto git init) |
| `git-cognition why <file>:<line>` | Show why a line was written with code context |
| `git-cognition session ls` | List sessions (`--file`, `--since`, `--model`, `--limit`) |
| `git-cognition session show <id>` | Show session detail |
| `git-cognition session stat` | Aggregate statistics |
| `git-cognition session grep <query>` | Search sessions (`--scope`, `--since`) |
| `git-cognition ingest` | Ingest NDJSON from stdin |
| `git-cognition version` | Print version |

All commands support `--json` for machine-readable output and `--no-color` to disable colors.
