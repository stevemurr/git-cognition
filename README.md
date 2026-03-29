# git-cognition

A single Go binary that captures Claude Code session context — tool calls, reasoning, decisions — and stores it in git notes. Query any line of code to see *why* it was written.

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

## Quick start

### 1. Run global init

This installs Claude Code hooks and git aliases:

```bash
git-cognition init
```

You should see output confirming hooks were added to `~/.claude/settings.json` and aliases to `~/.gitconfig`.

### 2. Enable capture in your repo

```bash
cd your-project
git-cognition init --repo
```

This creates `.git/gc-enabled` and excludes session files from git tracking.

### 3. Use Claude Code normally

```bash
claude
```

Ask Claude to do some work — edit files, run tests, whatever. For example:

```
> Add input validation to the signup endpoint
```

### 4. Let Claude commit

Make sure Claude commits the changes during the session (or commit them yourself before exiting).

### 5. Exit Claude

When the session ends, the Stop hook fires automatically. It captures:
- Every tool call Claude made (files read, edits, commands run)
- Claude's final message — its own summary of what it did and why
- Which commits were made during the session

All of this is written to git notes. No model is called. The final message is stored verbatim.

### 6. Ask why

```bash
git why src/auth/signup.go:42
```

Output:

```
a1b2c3d  src/auth/signup.go:42  ·  claude-sonnet-4-6  ·  2026-03-29

  "I added email format validation before the database insert —
   the existing regex in utils.go already handles RFC 5322, so
   I reused that rather than adding a new dependency."
```

Add `--verbose` for the full reasoning and action log:

```bash
git why src/auth/signup.go:42 --verbose
```

Add `--full` to also see the file contents Claude was reading when it made its decisions:

```bash
git why src/auth/signup.go:42 --full
```

## Querying sessions

List all captured sessions:

```bash
git session ls
git session ls --since 7d
git session ls --file signup.go
```

Show details of a specific session:

```bash
git session show <session-id>
```

Search Claude's reasoning across all sessions:

```bash
git session grep "validation"
git session grep "redis" --scope tools
git session grep "auth" --scope all --since 14d
```

Session statistics:

```bash
git session stat
```

## Agent-agnostic ingest

Other AI coding tools can pipe NDJSON into git-cognition:

```bash
my-agent run "add rate limiting" | git-cognition ingest
```

See `spec.md` for the NDJSON protocol.

## How it works

```
Claude Code session
    │
    ├── PostToolUse hook (after each tool call)
    │       └── appends one NDJSON line to .git/gc-sessions/<session_id>.ndjson
    │
    └── Stop hook (session ends)
            └── reads session file
                extracts final assistant message from transcript
                finds commits made during session
                writes everything to git notes
                deletes session file
```

Storage uses two git mechanisms:
- **Session blobs** at `refs/sessions/<session_id>` — the full session JSON
- **Commit notes** via `git notes --ref=sessions` — maps each commit to its session ID

`git why` follows the chain: `git blame` → commit note → session blob → BM25 excerpt selection → render.

## Commands

| Command | Description |
|---|---|
| `git-cognition init` | Install hooks and aliases globally |
| `git-cognition init --repo` | Enable capture in current repo |
| `git-cognition why <file>:<line>` | Show why a line was written |
| `git-cognition session ls` | List sessions |
| `git-cognition session show <id>` | Show session detail |
| `git-cognition session stat` | Aggregate statistics |
| `git-cognition session grep <query>` | Search session content |
| `git-cognition ingest` | Ingest NDJSON from stdin |
| `git-cognition hook post-tool-use` | PostToolUse hook entry point |
| `git-cognition hook stop` | Stop hook entry point |

All commands support `--json` for machine-readable output.

## Testing it end to end

Here's a concrete walkthrough to verify everything works:

```bash
# 1. Build and install
cd git-cognition
make install

# 2. Set up a test repo
mkdir /tmp/test-project && cd /tmp/test-project
git init
git commit --allow-empty -m "initial"

# 3. Global init (hooks + aliases)
git-cognition init

# 4. Enable capture in this repo
git-cognition init --repo

# 5. Verify setup
ls .git/gc-enabled          # should exist
cat ~/.claude/settings.json # should contain git-cognition hooks

# 6. Run Claude Code and do some work
claude
# > "Create a hello.go that prints hello world, then commit it"
# Claude writes the file, commits it, you exit

# 7. Check that a session was captured
git session ls

# 8. Show the session
git session show <session-id-from-above>

# 9. Ask why a specific line was written
git why hello.go:5

# 10. Search across sessions
git session grep "hello"
```

If step 7 shows your session and step 9 shows Claude's reasoning, everything is working.
