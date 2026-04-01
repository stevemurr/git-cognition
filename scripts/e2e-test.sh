#!/usr/bin/env bash
set -euo pipefail

# E2E test for git-cognition
# Runs separate claude -p sessions (simulating interactive mode),
# each producing its own commit and session with LLM-extracted reasoning.
# Then queries every file with git why to verify per-file context.
#
# Usage: e2e-test.sh [PHASES]
#   PHASES  number of phases to run (1-6, default: 6)
#
# Environment:
#   GC_LLM_ENDPOINT  LLM endpoint URL (required for LLM extraction)
#   GC_LLM_API_KEY   Bearer token for the LLM endpoint (required)
#   GC_LLM_MODEL     Model name (default: nemotron3-nano)

PHASES=${1:-6}
if [[ ! "$PHASES" =~ ^[1-6]$ ]]; then
    echo "Usage: e2e-test.sh [1-6]" >&2
    exit 1
fi

# LLM config — extraction is skipped if endpoint/key are not set
if [ -n "${GC_LLM_ENDPOINT:-}" ] && [ -n "${GC_LLM_API_KEY:-}" ]; then
    export GC_LLM_ENDPOINT
    export GC_LLM_API_KEY
    export GC_LLM_MODEL="${GC_LLM_MODEL:-nemotron3-nano}"
    export GC_LLM_ENABLED=true
else
    echo "NOTE: GC_LLM_ENDPOINT and/or GC_LLM_API_KEY not set — LLM extraction disabled"
    export GC_LLM_ENABLED=false
fi

ALLOWED="Edit,Write,Bash,Read"
TEST_DIR=$(mktemp -d)
echo "=== test directory: $TEST_DIR ==="
if [ "${GC_LLM_ENABLED:-false}" = "true" ]; then
    echo "=== LLM: $GC_LLM_ENDPOINT ($GC_LLM_MODEL) ==="
fi

cleanup() {
    echo ""
    echo "=== test directory preserved at: $TEST_DIR ==="
}
trap cleanup EXIT

cd "$TEST_DIR"

# 1. Init repo with git-cognition
echo ""
echo "=== Step 1: git-cognition init --repo ==="
git-cognition init --repo
git commit --allow-empty -m "initial"

# 2. Run claude -p sessions, one per phase (up to $PHASES)
echo ""
echo "=== Step 2: Building Deno todo app in $PHASES phase(s) (separate sessions) ==="

if [ "$PHASES" -ge 1 ]; then
echo ""
echo "--- Phase 1: Types and store ---"
claude -p "Create a Deno todo CLI app. Start with Phase 1: Create types.ts with a Todo interface (id, title, completed, createdAt) and store.ts with an in-memory TodoStore class that has add, getById, list, complete, and delete methods. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

if [ "$PHASES" -ge 2 ]; then
echo ""
echo "--- Phase 2: CLI parsing ---"
claude -p "Phase 2 of the Deno todo app: Add cli.ts with an argument parser that extracts a command name, positional args, and --flag options. Include a printUsage/help function. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

if [ "$PHASES" -ge 3 ]; then
echo ""
echo "--- Phase 3: Commands ---"
claude -p "Phase 3 of the Deno todo app: Add commands.ts with handler functions for add, list, complete, and delete. Add main.ts that parses CLI args and dispatches to the right command handler. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

if [ "$PHASES" -ge 4 ]; then
echo ""
echo "--- Phase 4: Persistence ---"
claude -p "Phase 4 of the Deno todo app: Add persistence.ts that saves and loads todos from a JSON file. Wire it into main.ts so todos persist between runs. Add todos.json to .gitignore. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

if [ "$PHASES" -ge 5 ]; then
echo ""
echo "--- Phase 5: Delete and filter ---"
claude -p "Phase 5 of the Deno todo app: Add a filter command that lists todos filtered by --done or --pending flags. Make sure delete works end-to-end with persistence. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

if [ "$PHASES" -ge 6 ]; then
echo ""
echo "--- Phase 6: Tests ---"
claude -p "Phase 6 of the Deno todo app: Add store_test.ts and cli_test.ts with comprehensive tests for the store and CLI parser. Run them with 'deno test' to make sure they pass. Commit your changes." \
    --allowedTools "$ALLOWED"
fi

# 3. Show session summary
echo ""
echo "=== Step 3: Session summary ==="
git log --oneline
echo ""
git-cognition session ls

# 4. Run git why on every tracked file (default + verbose)
echo ""
echo "=== Step 4: git why for every file ==="

git ls-files | while IFS= read -r file; do
    # Get total lines in file
    total=$(wc -l < "$file" | tr -d ' ')
    if [ "$total" -eq 0 ]; then
        continue
    fi

    # Pick the middle line
    line=$(( (total + 1) / 2 ))

    echo ""
    echo "─────────────────────────────────────────────"
    echo ">>> $file:$line"
    echo "─────────────────────────────────────────────"
    git-cognition why "$file:$line" || true
done

# 5. Test --verbose on first file (shows key decisions + rejected)
echo ""
echo "=== Step 5: git why --verbose ==="
FIRST_FILE=$(git ls-files | head -1)
if [ -n "$FIRST_FILE" ]; then
    total=$(wc -l < "$FIRST_FILE" | tr -d ' ')
    line=$(( (total + 1) / 2 ))
    git-cognition why "$FIRST_FILE:$line" --verbose || true
fi

# 6. Test --json on first file (verify llm_reasoning field)
echo ""
echo "=== Step 6: git why --json (checking llm_reasoning) ==="
if [ -n "$FIRST_FILE" ]; then
    JSON_OUT=$(git-cognition why "$FIRST_FILE:$line" --json 2>/dev/null || true)
    if echo "$JSON_OUT" | grep -q '"llm_reasoning"'; then
        echo "PASS: llm_reasoning field present in JSON output"
    else
        echo "WARN: llm_reasoning field missing — LLM extraction may not have run"
    fi
    echo "$JSON_OUT" | head -30
fi

# 7. Test --rich on first file
echo ""
echo "=== Step 7: git why --rich ==="
if [ -n "$FIRST_FILE" ]; then
    git-cognition why "$FIRST_FILE:$line" --rich || true
fi

echo ""
echo "=== Done. Test directory: $TEST_DIR ==="
