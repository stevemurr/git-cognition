from __future__ import annotations

import argparse

from git_cognition.commands import (
    session_attach,
    session_claude,
    session_claude_live,
    session_grep,
    session_init,
    session_ls,
    session_show,
    session_stat,
    why,
)


def build_session_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="git session")
    subparsers = parser.add_subparsers(dest="command", required=True)

    init_parser = subparsers.add_parser("init", help="enable session tracking in this repo")
    init_parser.set_defaults(func=session_init.run)

    attach_parser = subparsers.add_parser("attach", help="attach an existing session to one or more commits")
    attach_parser.add_argument("session_id")
    attach_parser.add_argument("commits", nargs="*")
    attach_parser.set_defaults(func=session_attach.run)

    ls_parser = subparsers.add_parser("ls", help="list stored sessions")
    ls_parser.add_argument("--file")
    ls_parser.add_argument("--since")
    ls_parser.add_argument("--until")
    ls_parser.add_argument("--model")
    ls_parser.add_argument("--min-cost", type=float)
    ls_parser.add_argument("--max-cost", type=float)
    ls_parser.add_argument("--limit", type=int, default=50)
    ls_parser.add_argument("--json", action="store_true")
    ls_parser.set_defaults(func=session_ls.run)

    show_parser = subparsers.add_parser("show", help="show one session")
    show_parser.add_argument("session_id")
    show_parser.add_argument("--thinking", action="store_true")
    show_parser.add_argument("--json", action="store_true")
    show_parser.set_defaults(func=session_show.run)

    stat_parser = subparsers.add_parser("stat", help="aggregate session metrics")
    stat_parser.add_argument("--since")
    stat_parser.add_argument("--json", action="store_true")
    stat_parser.set_defaults(func=session_stat.run)

    grep_parser = subparsers.add_parser("grep", help="search across sessions")
    grep_parser.add_argument("query")
    grep_parser.add_argument(
        "--scope",
        choices=["all", "tasks", "tools", "rejected", "thinking"],
        default="all",
    )
    grep_parser.add_argument("--since")
    grep_parser.add_argument("--session")
    grep_parser.add_argument("--json", action="store_true")
    grep_parser.set_defaults(func=session_grep.run)

    claude_parser = subparsers.add_parser("claude", help="run Claude Code in print mode and record the session")
    claude_parser.add_argument("prompt")
    claude_parser.add_argument("--model")
    claude_parser.add_argument(
        "--permission-mode",
        choices=["acceptEdits", "bypassPermissions", "default", "dontAsk", "plan", "auto"],
    )
    claude_parser.add_argument("--max-budget-usd", type=float)
    claude_parser.add_argument("--system-prompt")
    claude_parser.add_argument("--append-system-prompt")
    claude_parser.add_argument("--add-dir", action="append")
    claude_parser.add_argument("--allowed-tools", action="append")
    claude_parser.add_argument("--disallowed-tools", action="append")
    claude_parser.add_argument("--tools", action="append")
    claude_parser.add_argument("--capture-thinking", action="store_true")
    claude_parser.add_argument("--attach-head", action="store_true")
    claude_parser.add_argument("--dangerously-skip-permissions", action="store_true")
    claude_parser.add_argument("--bare", action="store_true")
    claude_parser.add_argument("--claude-bin")
    claude_parser.add_argument("--json", action="store_true")
    claude_parser.set_defaults(func=session_claude.run)

    claude_live_parser = subparsers.add_parser(
        "claude-live",
        help="run interactive Claude Code with the repo-local git-cognition plugin",
    )
    claude_live_parser.add_argument("--claude-bin")
    claude_live_parser.add_argument("claude_args", nargs=argparse.REMAINDER)
    claude_live_parser.set_defaults(func=session_claude_live.run)

    return parser


def build_why_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="git why")
    parser.add_argument("location")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--full", action="store_true")
    parser.add_argument("--json", action="store_true")
    parser.set_defaults(func=why.run)
    return parser


def session_main(argv: list[str] | None = None) -> int:
    parser = build_session_parser()
    args = parser.parse_args(argv)
    return args.func(args)


def why_main(argv: list[str] | None = None) -> int:
    parser = build_why_parser()
    args = parser.parse_args(argv)
    return args.func(args)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="git-cognition")
    subparsers = parser.add_subparsers(dest="command", required=True)

    session_parser = subparsers.add_parser("session")
    session_parser.add_argument("args", nargs=argparse.REMAINDER)

    why_parser = subparsers.add_parser("why")
    why_parser.add_argument("args", nargs=argparse.REMAINDER)

    args = parser.parse_args(argv)
    if args.command == "session":
        return session_main(args.args)
    return why_main(args.args)


if __name__ == "__main__":
    raise SystemExit(main())
