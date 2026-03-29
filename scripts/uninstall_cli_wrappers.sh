#!/bin/sh
set -eu

BINDIR=${BINDIR:-"$HOME/.local/bin"}
SESSION_TARGET=${SESSION_TARGET:-"$BINDIR/git-session"}
WHY_TARGET=${WHY_TARGET:-"$BINDIR/git-why"}

for target in "$SESSION_TARGET" "$WHY_TARGET"; do
  if [ -f "$target" ]; then
    rm -f "$target"
    printf 'Removed CLI wrapper: %s\n' "$target"
  else
    printf 'CLI wrapper not found: %s\n' "$target"
  fi
done
