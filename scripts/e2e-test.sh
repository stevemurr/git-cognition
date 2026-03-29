#!/usr/bin/env bash
set -euo pipefail

# E2E test for git-cognition
# Creates a temp project, runs Claude to build it, then queries every file with git why

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

# 2. Run Claude to build a project
echo ""
echo "=== Step 2: Running Claude to build a Deno todo app ==="
claude -p "Build a Deno todo CLI app in 6 phases. After each phase, commit your changes with a descriptive message. Phase 1: types and basic store. Phase 2: CLI argument parsing. Phase 3: add/list/complete commands. Phase 4: file persistence. Phase 5: delete and filter commands. Phase 6: tests." \
    --allowedTools "Edit,Write,Bash,Read"

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
