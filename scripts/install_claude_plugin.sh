#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
PREFIX=${PREFIX:-"$HOME/.local"}
BINDIR=${BINDIR:-"$PREFIX/bin"}
TARGET=${TARGET:-"$BINDIR/claude-git-cognition"}
PLUGIN_DIR="$REPO_ROOT/claude-plugin"

mkdir -p "$BINDIR"

cat >"$TARGET" <<EOF
#!/bin/sh
set -eu
exec "\${GIT_COGNITION_CLAUDE_BIN:-claude}" --plugin-dir "$PLUGIN_DIR" "\$@"
EOF

chmod +x "$TARGET"

printf 'Installed Claude launcher: %s\n' "$TARGET"
printf 'Plugin directory: %s\n' "$PLUGIN_DIR"
printf 'Run: %s --model claude-sonnet-4-6\n' "$TARGET"
