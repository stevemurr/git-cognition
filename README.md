# git-cognition

`git-cognition` is a prototype for storing and querying AI session provenance in
Git. It keeps per-session JSON blobs in `refs/sessions/<id>` and attaches the
session ID to commits through Git notes in `refs/notes/sessions`.

## Commands

- `git session init`
- `git session attach <id> [commit...]`
- `git session claude <prompt>`
- `git session ls`
- `git session show <id>`
- `git session grep <query>`
- `git session stat`
- `git why <file>:<line>`

## Storage Model

- Notes: `refs/notes/sessions`
- Session blobs: `refs/sessions/<id>`
- Local enablement: `.git/config`

`git session init` only writes local Git config:

```bash
git config --local git-cognition.enabled true
git config --local git-cognition.schema-version 1.0
```

It does not modify tracked files or configure remote refspecs.

## Quick Start

### 1. Install the package

From the repo root:

```bash
python3 -m pip install -e .
```

That puts `git-session` and `git-why` on your `PATH`, which Git will expose as
`git session` and `git why`.

### 2. Create or enter a Git repo

```bash
mkdir /tmp/git-cognition-demo
cd /tmp/git-cognition-demo
git init
git config user.name "Test User"
git config user.email "test@example.com"
```

### 3. Enable git-cognition in that repo

```bash
git session init
```

This only writes local Git config:

```bash
git config --local --get git-cognition.enabled
git config --local --get git-cognition.schema-version
```

### 4. Create a commit to inspect

```bash
cat > app.py <<'EOF'
def greet(name):
    return f"hello {name}"
EOF

git add app.py
git commit -m "initial app"
```

### 5. Make sure Claude Code can run in print mode

`git session claude` wraps `claude -p --verbose --output-format stream-json`.
Before using it, confirm the `claude` CLI is installed and authenticated.

Typical checks:

```bash
command -v claude
claude --help
```

If Claude is not authenticated yet, complete login in the Claude CLI first.

### 6. Run Claude through the wrapper

Ask Claude to both edit code and commit it during the wrapped run:

```bash
git session claude \
  --model claude-sonnet-4-6 \
  "Update app.py to add a simple rate-limited path for greet(), then commit the change with message 'feat: add rate limiting'"
```

What the wrapper does:

- runs Claude in non-interactive print mode
- parses Claude's JSONL event stream
- records tool calls, text rationale, metrics, and rejected approaches
- detects any new commits created during the Claude run
- attaches the session to those commits automatically

If no new commit is created during the wrapped run, the session is still stored,
but `git why` will not have a commit note to follow until you attach one later.

### 7. Inspect the recorded session

```bash
git session ls
git session show <session-id-prefix>
git session grep "rate limit"
git why app.py:2
git why app.py:2 --json
```

If `git why` finds a session note on the blamed commit, it will return the
recorded session context. If not, it falls back to plain `git blame`.

### 8. Attach a session later if needed

If Claude changed files but did not commit during the wrapped run, you can
commit afterward and attach the stored session manually:

```bash
git add app.py
git commit -m "feat: add rate limiting"
git session attach <session-id-prefix> HEAD
```

## Claude Code Status

The repo now includes an automatic wrapper for one-shot Claude runs:

```bash
git session claude "<prompt>"
```

What exists now:

- a `git session claude` wrapper around `claude -p`
- a write-side adapter class for Claude-shaped responses
- explicit lifecycle methods: `start_session()`, `attach_commit()`,
  `finalize_session()`, `abort_session()`
- pending session state under `.git/git-cognition/pending/`
- automatic attachment of commits created during the wrapped Claude run

What does not exist yet:

- automatic capture of a live interactive Claude TUI session
- automatic background attachment to commits made after the wrapped run exits
- a first-class Claude plugin; the current integration is a wrapper command plus
  library adapter

Today, the supported automatic path is the non-interactive wrapper. The adapter
API still exists for deeper integrations.

## Claude Code Flow

If your goal is "run Claude Code, make a commit, then inspect it", the current
flow is:

1. Install `git-cognition` and run `git session init` in the target repo.
2. Make sure the `claude` CLI is installed and authenticated.
3. Run `git session claude "<prompt>"` and tell Claude to commit during that run.
4. The wrapper records the Claude event stream and auto-attaches any new commits
   created during the run.
5. Inspect the result with:
   `git session ls`, `git session show <id>`, and `git why <file>:<line>`.
6. If no commit happened during the wrapped run, commit normally and then attach
   the session with `git session attach <id> HEAD`.

In other words: the turnkey path is available for non-interactive `claude -p`
runs, while the adapter remains available for custom integrations.

## Adapter API

The write-side API is explicit rather than hook-driven:

```python
from pathlib import Path

from git_cognition.writer.claude_code import ClaudeCodeSessionWriter

writer = ClaudeCodeSessionWriter(
    repo=Path("."),
    task_prompt="Add rate limiting",
    model="claude-sonnet-4-6",
)
writer.start_session()
writer.record_tool_call(
    tool="read_file",
    kind="read",
    paths=["auth/middleware.py"],
    raw_input={"path": "auth/middleware.py"},
    output_summary="Located middleware entrypoint",
)
writer.attach_commit("0123456789abcdef0123456789abcdef01234567")
writer.finalize_session()
```

Pending write state is stored under `.git/git-cognition/pending/` so an
integration can recover or clean up interrupted runs without touching the work
tree.

## Development

Run the test suite with:

```bash
python3 -m unittest discover -s tests -p 'test*.py' -v
```
