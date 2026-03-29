#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
PYTHON_BIN=${PYTHON_BIN:-python3}
BINDIR=${BINDIR:-"$HOME/.local/bin"}
SESSION_TARGET=${SESSION_TARGET:-"$BINDIR/git-session"}
WHY_TARGET=${WHY_TARGET:-"$BINDIR/git-why"}

mkdir -p "$BINDIR"

cat >"$SESSION_TARGET" <<EOF
#!/bin/sh
set -eu
export PYTHONPATH="$REPO_ROOT\${PYTHONPATH:+:\$PYTHONPATH}"
exec "$PYTHON_BIN" -m git_cognition.cli session "\$@"
EOF

cat >"$WHY_TARGET" <<EOF
#!/bin/sh
set -eu
export PYTHONPATH="$REPO_ROOT\${PYTHONPATH:+:\$PYTHONPATH}"
exec "$PYTHON_BIN" -m git_cognition.cli why "\$@"
EOF

chmod +x "$SESSION_TARGET" "$WHY_TARGET"

printf 'Installed CLI wrappers:\n'
printf '  %s\n' "$SESSION_TARGET"
printf '  %s\n' "$WHY_TARGET"

case ":$PATH:" in
  *:"$BINDIR":*)
    ;;
  *)
    printf 'Note: %s is not currently on PATH.\n' "$BINDIR"
    ;;
esac
