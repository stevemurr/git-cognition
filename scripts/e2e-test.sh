#!/usr/bin/env bash
set -euo pipefail

# E2E test for git-cognition
# Runs 6 separate claude -p sessions (simulating interactive mode),
# each producing its own commit and session with distinct reasoning.
# Then queries every file with git why to verify per-file context.

ALLOWED="Edit,Write,Bash,Read"
TEST_DIR=$(mktemp -d)
echo "=== test directory: $TEST_DIR ==="

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

# 2. Run 6 separate claude -p sessions, one per phase
echo ""
echo "=== Step 2: Building Deno todo app in 6 phases (separate sessions) ==="

echo ""
echo "--- Phase 1: Types and store ---"
claude -p "Create a Deno todo CLI app. Start with Phase 1: Create types.ts with a Todo interface (id, title, completed, createdAt) and store.ts with an in-memory TodoStore class that has add, getById, list, complete, and delete methods. Commit your changes." \
    --allowedTools "$ALLOWED"

echo ""
echo "--- Phase 2: CLI parsing ---"
claude -p "Phase 2 of the Deno todo app: Add cli.ts with an argument parser that extracts a command name, positional args, and --flag options. Include a printUsage/help function. Commit your changes." \
    --allowedTools "$ALLOWED"

echo ""
echo "--- Phase 3: Commands ---"
claude -p "Phase 3 of the Deno todo app: Add commands.ts with handler functions for add, list, complete, and delete. Add main.ts that parses CLI args and dispatches to the right command handler. Commit your changes." \
    --allowedTools "$ALLOWED"

echo ""
echo "--- Phase 4: Persistence ---"
claude -p "Phase 4 of the Deno todo app: Add persistence.ts that saves and loads todos from a JSON file. Wire it into main.ts so todos persist between runs. Add todos.json to .gitignore. Commit your changes." \
    --allowedTools "$ALLOWED"

echo ""
echo "--- Phase 5: Delete and filter ---"
claude -p "Phase 5 of the Deno todo app: Add a filter command that lists todos filtered by --done or --pending flags. Make sure delete works end-to-end with persistence. Commit your changes." \
    --allowedTools "$ALLOWED"

echo ""
echo "--- Phase 6: Tests ---"
claude -p "Phase 6 of the Deno todo app: Add store_test.ts and cli_test.ts with comprehensive tests for the store and CLI parser. Run them with 'deno test' to make sure they pass. Commit your changes." \
    --allowedTools "$ALLOWED"

# 3. Show session summary
echo ""
echo "=== Step 3: Session summary ==="
git log --oneline
echo ""
git-cognition session ls

# 4. Run git why on every tracked file
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

echo ""
echo "=== Done. Test directory: $TEST_DIR ==="
