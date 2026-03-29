# git-cognition v5 spec
## Claude's words, stored verbatim. No secondary model.

**Status:** ready for implementation
**Language:** Go
**Storage:** git notes (`refs/sessions/*`)
**Capture:** PostToolUse (action log) + Stop (transcript final message)
**Runtime model calls:** none — git-cognition never calls a model
**Binary:** single `git-cognition` binary

---

## The design in one paragraph

Claude Code's Stop hook delivers the full session transcript. The last
assistant message in that transcript is Claude's own closing summary —
it describes what was built, what was considered, what was rejected, and
why. git-cognition extracts that message verbatim, pairs it with the
action log accumulated during the session, and writes everything to git
notes. `git why` surfaces it directly. No model is involved at any point
after the Claude Code session ends.

---

## Decisions log

| Question | Decision |
|---|---|
| Reasoning source | Claude's final message, verbatim — no processing |
| Secondary model | None — cut entirely |
| Action log | Tool call sequence via PostToolUse |
| Runtime API calls | Zero |
| Long-running process | None |
| Voluntary tool calls | None |
| CLAUDE.md modifications | None |
| Storage | git notes `refs/sessions/*` |
| Config | `~/.config/git-cognition/config.toml` + `.git/gc-config` override |
| Distribution | Single Go binary |
| Go structure | Monorepo, one module, subpackages |
| Agent-agnostic ingest | `git-cognition ingest` reads NDJSON — final_message field optional |

---

## Capture flow

```
Claude Code TUI (completely unmodified)
        │
        ├── PostToolUse  (after each tool call)
        │       └── git-cognition hook post-tool-use
        │               appends one NDJSON line to
        │               .git/gc-sessions/<session_id>.ndjson
        │               exits in < 5ms
        │
        └── Stop  (session ends)
                └── git-cognition hook stop
                        reads .git/gc-sessions/<session_id>.ndjson
                        extracts final assistant message from transcript
                        runs git log → commits made during session
                        assembles session JSON
                        writes git notes
                        deletes session file
                        exits silently
```

No model calls. No server. No network. Pure file I/O and git operations.

---

## What the hooks give us

### PostToolUse payload

```json
{
  "session_id": "abc123",
  "tool_name": "Read",
  "tool_input": { "file_path": "redis_client.py" },
  "tool_response": "<complete file contents>",
  "cwd": "/home/skeet/myproject"
}
```

`tool_response` is the full raw output — complete file contents on Read,
full stdout/stderr on Bash, confirmation on Write/Edit. Truncated to
4000 chars when written to the session file.

Known tool names: `Read`, `Write`, `Edit`, `MultiEdit`, `Bash`, `Glob`,
`Grep`, `LS`, `TodoRead`, `TodoWrite`, `WebFetch`, `Task`

### Stop payload

```json
{
  "session_id": "abc123",
  "cwd": "/home/skeet/myproject",
  "transcript": [
    { "role": "user", "content": "Add rate limiting to the auth endpoint" },
    {
      "role": "assistant",
      "content": [
        { "type": "text", "text": "I'll start by reading the existing middleware..." },
        { "type": "tool_use", "name": "Read", "input": { "file_path": "redis_client.py" } }
      ]
    },
    "...",
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "I've added a token bucket rate limiter to the auth endpoint.\n\nI used the INCRBY/EXPIRE pattern already present in redis_client.py — this maps cleanly to token bucket semantics and means a single round trip per request with no new dependencies.\n\nI considered two alternatives:\n- A sliding window using ZADD/ZRANGEBYSCORE, but this requires a Lua script for atomicity and two round trips on what is already a hot path\n- An in-process counter, but this wouldn't survive multiple workers or restarts\n\nAll existing tests pass. I added two new tests covering the rate limit boundary conditions."
        }
      ]
    }
  ]
}
```

**Final message extraction:** find the last entry in `transcript` where
`role == "assistant"`, collect all `content` blocks of `type == "text"`,
join them. That text is stored verbatim as `reasoning.final_message`.

Also extract the first user message as `task.prompt`.

---

## Session file (in-flight)

```
.git/gc-sessions/<session_id>.ndjson
```

Gitignored via `.git/info/exclude`. Deleted by Stop hook after success.

```jsonl
{"type":"session_start","session_id":"abc123","started_at":"2026-03-27T14:33:00Z","cwd":"/home/skeet/myproject"}
{"type":"tool_call","sequence":1,"tool":"Read","input":{"file_path":"redis_client.py"},"output":"<contents, truncated 4000 chars>","timestamp":"2026-03-27T14:33:22Z"}
{"type":"tool_call","sequence":2,"tool":"Read","input":{"file_path":"auth/middleware.py"},"output":"<contents, truncated 4000 chars>","timestamp":"2026-03-27T14:33:35Z"}
{"type":"tool_call","sequence":3,"tool":"Edit","input":{"file_path":"auth/middleware.py","description":"add token bucket rate limiter after JWT validation"},"output":"Edit applied successfully","timestamp":"2026-03-27T14:33:58Z"}
{"type":"tool_call","sequence":4,"tool":"Bash","input":{"command":"pytest tests/test_auth.py -v"},"output":"4 passed in 0.51s","timestamp":"2026-03-27T14:34:12Z"}
```

---

## Session object schema

```json
{
  "schema_version": "5.0",
  "session_id": "abc123",
  "created_at": "2026-03-27T14:33:00Z",
  "completed_at": "2026-03-27T14:34:47Z",

  "agent": {
    "runner": "claude-code",
    "model": "claude-sonnet-4-6"
  },

  "task": {
    "prompt": "Add rate limiting to the auth endpoint"
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
      "tool": "Read",
      "input": { "file_path": "redis_client.py" },
      "output_truncated": "# Redis connection pool\ndef incrby(key, amount):\ndef expire(key, ttl, nx=False):",
      "timestamp": "2026-03-27T14:33:22Z"
    },
    {
      "sequence": 2,
      "tool": "Read",
      "input": { "file_path": "auth/middleware.py" },
      "output_truncated": "def authenticate(request):\n    token = extract_token(request)\n    payload = validate_jwt(token)\n    return payload",
      "timestamp": "2026-03-27T14:33:35Z"
    },
    {
      "sequence": 3,
      "tool": "Edit",
      "input": {
        "file_path": "auth/middleware.py",
        "description": "add token bucket rate limiter after JWT validation"
      },
      "output_truncated": "Edit applied successfully",
      "timestamp": "2026-03-27T14:33:58Z"
    },
    {
      "sequence": 4,
      "tool": "Bash",
      "input": { "command": "pytest tests/test_auth.py -v" },
      "output_truncated": "4 passed in 0.51s",
      "timestamp": "2026-03-27T14:34:12Z"
    }
  ],

  "reasoning": {
    "final_message": "I've added a token bucket rate limiter to the auth endpoint.\n\nI used the INCRBY/EXPIRE pattern already present in redis_client.py — this maps cleanly to token bucket semantics and means a single round trip per request with no new dependencies.\n\nI considered two alternatives:\n- A sliding window using ZADD/ZRANGEBYSCORE, but this requires a Lua script for atomicity and two round trips on what is already a hot path\n- An in-process counter, but this wouldn't survive multiple workers or restarts\n\nAll existing tests pass. I added two new tests covering the rate limit boundary conditions.",
    "source": "claude_final_message"
  },

  "thinking_blocks": [],

  "metrics": {
    "tool_call_count": 4,
    "duration_seconds": 107
  }
}
```

### Schema notes

**`reasoning.final_message`** — Claude's verbatim closing text. This is
the primary field surfaced by `git why`. Never processed, never
paraphrased.

**`reasoning.source`** — `"claude_final_message"` for Claude Code hook
capture. `"ingest_provided"` when a runner explicitly sends a
`final_message` event via `git-cognition ingest`. `"none"` if Stop fired
but the transcript had no final assistant text (e.g. session ended
without Claude producing a closing message — uncommon but possible).

**`tool_calls[*].output_truncated`** — named explicitly to signal
truncation. `git why --full` shows these so reviewers can see what
Claude was reading when it made decisions.

**`thinking_blocks`** — always `[]`. Present for forward compatibility.

**No `capture_error` complexity** — if Stop hook fails, it logs to
stderr and exits 0. Session file stays in place. Next Stop hook in this
repo checks for orphaned files older than 24h and retries or cleans up.

---

## `git why`

```bash
git why <file>:<line>
git why <file>:<line> --verbose
git why <file>:<line> --full
git why <file>:<line> --json
```

### Lookup chain

```
1. git blame -L <line>,<line> <file>  →  commit SHA
2. git notes --ref=sessions show <SHA>
   → no note: silent fallback to plain git blame output
   → found: extract session_id
3. git cat-file blob refs/sessions/<session_id>  →  session JSON
4. BM25 over tool_calls to find calls most relevant to this file/line
5. Render
```

### Default output

```
8b2e4f3  auth/middleware.py:47  ·  claude-sonnet-4-6  ·  2026-03-27

"I used the INCRBY/EXPIRE pattern already present in redis_client.py —
 maps cleanly to token bucket semantics, single round trip per request,
 no new dependencies."
```

The quote is the relevant excerpt from `reasoning.final_message`,
identified by BM25 match against the blamed file. If BM25 finds no
strong match, the full final message is shown.

### `--verbose`

```
8b2e4f3  auth/middleware.py:47  ·  claude-sonnet-4-6  ·  2026-03-27
session: abc123  ·  task: Add rate limiting to the auth endpoint

claude's reasoning:
  "I used the INCRBY/EXPIRE pattern already present in redis_client.py —
   this maps cleanly to token bucket semantics and means a single round
   trip per request with no new dependencies.

   I considered two alternatives:
   - A sliding window using ZADD/ZRANGEBYSCORE, but this requires a Lua
     script for atomicity and two round trips on what is already a hot path
   - An in-process counter, but this wouldn't survive multiple workers
     or restarts"

action log:
  Read  redis_client.py
  Read  auth/middleware.py
  Edit  auth/middleware.py  — "add token bucket rate limiter after JWT validation"
  Bash  pytest tests/test_auth.py -v  →  4 passed in 0.51s

  git why auth/middleware.py:47 --full  ·  git session show abc123
```

### `--full`

Adds the complete `reasoning.final_message` if verbose showed an excerpt,
plus `output_truncated` for each tool call touching this file.

```
[verbose output]

files read during session:

  redis_client.py:
    # Redis connection pool
    def incrby(key, amount):
    def expire(key, ttl, nx=False):
    [truncated — 4000 chars stored]

  auth/middleware.py:
    def authenticate(request):
        token = extract_token(request)
        payload = validate_jwt(token)
        return payload
    [truncated — 4000 chars stored]
```

### No-session fallback

No note on commit → silent fallback to plain `git blame` output.

---

## `git session`

### `git session ls`

```
SESSION   DATE        MODEL              FILES  TASK
abc123    2026-03-27  claude-sonnet-4-6    2    Add rate limiting to auth endpoint
b71e8d    2026-03-26  claude-sonnet-4-6    7    Refactor database connection pooling
c4a1f0    2026-03-25  claude-opus-4-6      2    Fix JWT token expiry edge case
──────────────────────────────────────────────────────────────
3 sessions  ·  git session show <id> for detail
```

Filters: `--file <path>`, `--since <duration>`, `--model <substr>`,
`--limit <n>`, `--json`

Enumeration: `git for-each-ref refs/sessions/` → fetch + filter in Go.

### `git session show <id>`

```
abc123  ·  claude-sonnet-4-6  ·  2026-03-27 14:33  ·  107s

task:    Add rate limiting to the auth endpoint

commits:
  8b2e4f3  feat: token bucket rate limiter on /auth

claude's reasoning:
  "I've added a token bucket rate limiter to the auth endpoint.

   I used the INCRBY/EXPIRE pattern already present in redis_client.py —
   this maps cleanly to token bucket semantics and means a single round
   trip per request with no new dependencies.

   I considered two alternatives:
   - A sliding window using ZADD/ZRANGEBYSCORE, but this requires a Lua
     script for atomicity and two round trips on what is already a hot path
   - An in-process counter, but this wouldn't survive multiple workers
     or restarts

   All existing tests pass. I added two new tests covering the rate
   limit boundary conditions."

action log:
  1.  Read  redis_client.py
  2.  Read  auth/middleware.py
  3.  Edit  auth/middleware.py  — "add token bucket rate limiter after JWT validation"
  4.  Bash  pytest tests/test_auth.py -v  →  4 passed in 0.51s

  git why <file>:<line> to trace a specific line
```

### `git session stat`

Session count, most-touched files, breakdown by model, activity over time.

### `git session grep <query>`

```bash
git session grep "token bucket"          # searches final_message (default)
git session grep "redis" --scope tools   # tool call inputs + outputs
git session grep "rate limit" --scope all
git session grep "auth" --since 14d
```

Default scope searches `reasoning.final_message` — Claude's own words.
This is immediately useful from the first session.

---

## `git-cognition ingest` — NDJSON protocol

Agent-agnostic path for other runners.

```bash
my-agent run "add rate limiting" | git-cognition ingest
```

```jsonl
{"type":"session_start","session_id":"xyz","started_at":"...","task":"...","model":"gpt-4o","runner":"my-agent","cwd":"/path"}
{"type":"tool_call","sequence":1,"tool":"read_file","input":{"path":"redis_client.py"},"output":"<contents>","timestamp":"..."}
{"type":"tool_call","sequence":2,"tool":"edit_file","input":{"path":"auth/middleware.py"},"output":"Edit applied","timestamp":"..."}
{"type":"final_message","content":"I added a token bucket rate limiter...","timestamp":"..."}
{"type":"session_end","timestamp":"...","metrics":{"duration_seconds":107}}
```

If `final_message` is present: stored verbatim, `source: "ingest_provided"`.
If absent: `reasoning.final_message` is empty, `source: "none"`.
No inference, no fallback model call — honest about what was captured.

---

## Go package structure

```
git-cognition/
├── go.mod
├── go.sum
├── main.go
│
├── cmd/
│   ├── root.go
│   ├── init.go          git-cognition init [--repo] [flags]
│   ├── hook.go          git-cognition hook <post-tool-use|stop>
│   ├── why.go           git-cognition why <file>:<line> [flags]
│   ├── ingest.go        git-cognition ingest
│   └── session/
│       ├── root.go
│       ├── ls.go
│       ├── show.go
│       ├── stat.go
│       └── grep.go
│
└── internal/
    ├── config/
    │   └── config.go         load + merge global + per-repo config
    ├── hooks/
    │   ├── post_tool_use.go  parse stdin → append to session file
    │   └── stop.go           extract final message → build session → git notes
    ├── capture/
    │   ├── sessionfile.go    read/write .git/gc-sessions/*.ndjson
    │   ├── transcript.go     extract final assistant message from Stop payload
    │   └── commits.go        git log between session timestamps
    ├── storage/
    │   ├── gitnotes.go       read/write git notes blobs
    │   └── schema.go         Session struct + JSON marshal/unmarshal
    ├── retrieval/
    │   └── bm25.go           BM25 over session fields for git why excerpt
    └── output/
        ├── why.go            render git why tiers
        └── session.go        render git session output
```

### Dependencies

```toml
require (
    github.com/spf13/cobra     v1.8.0
    github.com/BurntSushi/toml v1.3.2
    github.com/google/uuid     v1.6.0
)
```

No model SDK. No HTTP client. No database. No embedding library.
git-cognition makes zero network calls at runtime.

---

## Config

### `~/.config/git-cognition/config.toml`

No `[capture]` section in v5 — there is no capture model to configure.
Config is minimal:

```toml
[display]
max_message_lines = 20    # truncate long final messages in default output
                          # 0 = no truncation
```

### `.git/gc-config`

Per-repo overrides. Same structure.

### Resolution order

```
1. .git/gc-config
2. GC_* environment variables
3. ~/.config/git-cognition/config.toml
4. built-in defaults
```

---

## `git-cognition init`

### Global

```
git-cognition init
──────────────────────────────────────────────
Config:  ~/.config/git-cognition/config.toml  [created]
Hooks:   ~/.claude/settings.json              [updated]
Aliases: ~/.gitconfig                         [updated]

Enable capture in each repo:
  cd your-project && git-cognition init --repo
──────────────────────────────────────────────
```

Writes to `~/.claude/settings.json` (merges, idempotent):

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "git-cognition hook post-tool-use" }]
      }
    ],
    "Stop": [
      {
        "hooks": [{ "type": "command", "command": "git-cognition hook stop" }]
      }
    ]
  }
}
```

Writes to `~/.gitconfig`:

```ini
[alias]
    why     = !git-cognition why
    session = !git-cognition session
```

### Per-repo

```
git-cognition init --repo
──────────────────────────────────────────────
Enabled:  .git/gc-enabled      [created]
Excluded: .git/info/exclude    [updated]
──────────────────────────────────────────────
```

---

## Build and install

```bash
go install github.com/yourusername/git-cognition@latest

git-cognition init               # global: hooks + aliases
cd my-project
git-cognition init --repo        # per-repo: enable capture

# use Claude Code normally
git why auth/middleware.py:47
git session ls
git session show abc123
git session grep "sliding window"
```

---

## V6 deferral register

| Feature | Note |
|---|---|
| Raw thinking blocks | `thinking_blocks: []` in schema; requires SDK stream path |
| Full file snapshots for replay | `output_truncated` → `output_full` when replay lands |
| `git replay` | Blocked on full snapshots |
| `git diff-intent` | Natural next step once sessions accumulate |
| Retroactive reasoning from tool log | Secondary model call when `final_message` absent — useful for non-Claude runners |
| Privacy / redaction | Per-session policy using `reasoning.source` |
| SQLite index for scale | `internal/storage/` behind interface |
| Local embeddings for `git why` | `internal/retrieval/bm25.go` behind interface |
| Remote push config | Not auto-configured until privacy model settled |
