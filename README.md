# git-cognition

`git-cognition` is a prototype for storing and querying AI session provenance in
Git. It keeps per-session JSON blobs in `refs/sessions/<id>` and attaches the
session ID to commits through Git notes in `refs/notes/sessions`.

## Tooling

- `make install` installs `git-session` and `git-why` wrappers under `~/.local/bin` by default
- `make uninstall` removes those wrappers
- `make test` runs the test suite
- `make validate-claude-plugin` validates the repo-local Claude plugin
- `make install-claude-plugin` installs a `claude-git-cognition` launcher under
  `~/.local/bin` by default
- `make uninstall-claude-plugin` removes that launcher

## Commands

- `git session init`
- `git session attach <id> [commit...]`
- `git session claude <prompt>`
- `git session claude-live [-- <claude args>...]`
- `git session ls`
- `git session show <id>`
- `git session grep <query>`
- `git session stat`
- `git why <file>:<line>`

## Storage Model

- Notes: `refs/notes/sessions`
- Session blobs: `refs/sessions/<id>`
- Local enablement: `.git/config`
- Pending writer state: `.git/git-cognition/pending/`
- Interactive Claude runtime state: `.git/git-cognition/claude/`

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
make install
make validate-claude-plugin
make install-claude-plugin
```

`make install` does not use `pip install`. It installs small wrapper scripts
that run this repo directly via `python3`, which avoids Homebrew Python's PEP
668 restrictions on macOS.

If `~/.local/bin` is on your `PATH`, Git will expose those wrappers as
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

### 5. Make sure Claude Code is installed

Before using either Claude integration, confirm the `claude` CLI is installed
and authenticated.

Typical checks:

```bash
command -v claude
claude --help
```

If Claude is not authenticated yet, complete login in the Claude CLI first.

### 6. Run Claude interactively with session capture

Launch Claude with the repo-local `git-cognition` plugin loaded:

```bash
git session claude-live -- --model claude-sonnet-4-6
```

That runs:

```bash
claude --plugin-dir <repo>/claude-plugin --model claude-sonnet-4-6
```

If you installed the launcher with `make install-claude-plugin`, you can also
run:

```bash
claude-git-cognition --model claude-sonnet-4-6
```

That launcher is just a convenience wrapper around Claude's documented
`--plugin-dir` mechanism. It does not register a marketplace plugin in
`~/.claude/plugins`.

Inside the interactive Claude session:

- ask Claude to edit code
- ask Claude to commit before exiting if you want automatic commit attachment
- exit Claude normally

The plugin records:

- the first prompt plus later follow-up prompts
- tool calls from `PostToolUse`
- new commits created between `SessionStart` and `SessionEnd`/`Stop`

Then inspect the result:

```bash
git session ls
git session show <session-id-prefix>
git session grep "rate limit"
git why app.py:2
```

### 7. Run Claude through the one-shot wrapper

`git session claude` still wraps `claude -p --verbose --output-format stream-json`.

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

### 8. Inspect the recorded session

```bash
git session ls
git session show <session-id-prefix>
git session grep "rate limit"
git why app.py:2
git why app.py:2 --json
```

If `git why` finds a session note on the blamed commit, it will return the
recorded session context. If not, it falls back to plain `git blame`.

### 9. Attach a session later if needed

If Claude changed files but did not commit until after the interactive or
wrapped Claude run ended, commit afterward and attach the stored session
manually:

```bash
git add app.py
git commit -m "feat: add rate limiting"
git session attach <session-id-prefix> HEAD
```

## Claude Code Flow

If your goal is "run Claude Code, make a commit, then inspect it", there are
two supported paths:

1. Install `git-cognition` and run `git session init` in the target repo.
2. Make sure the `claude` CLI is installed and authenticated.
3. Interactive:
   `git session claude-live -- --model claude-sonnet-4-6`
4. Non-interactive:
   `git session claude "<prompt>"`
5. Tell Claude to commit before the session exits if you want automatic commit
   attachment.
6. Inspect the result with:
   `git session ls`, `git session show <id>`, and `git why <file>:<line>`.
7. If no commit happened until after Claude exited, commit normally and then
   attach the session with `git session attach <id> HEAD`.

The interactive path uses a repo-local Claude plugin loaded through
`--plugin-dir`. The one-shot path uses the existing `claude -p` stream parser.

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
tree. Interactive Claude runtime state lives under `.git/git-cognition/claude/`
until the session finalizes.

## Development

Run the test suite with:

```bash
make test
```
