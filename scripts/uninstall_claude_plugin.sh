#!/bin/sh
set -eu

PREFIX=${PREFIX:-"$HOME/.local"}
BINDIR=${BINDIR:-"$PREFIX/bin"}
TARGET=${TARGET:-"$BINDIR/claude-git-cognition"}

if [ -f "$TARGET" ]; then
	rm -f "$TARGET"
	printf 'Removed Claude launcher: %s\n' "$TARGET"
else
	printf 'Claude launcher not found: %s\n' "$TARGET"
fi
