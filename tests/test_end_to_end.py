from __future__ import annotations

import contextlib
from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from git_cognition.cli import session_main, why_main
from git_cognition.storage.git_notes import (
    CONFIG_ENABLED_KEY,
    CONFIG_SCHEMA_KEY,
    InvalidSessionBlobError,
    attach_session_to_commit,
    read_session,
    read_session_for_commit,
    run_git,
    write_session,
)
from git_cognition.storage.schema import AgentInfo, Metrics, RejectedApproach, Session, TaskInfo, ToolCall
from git_cognition.writer.claude_code import ClaudeCodeSessionWriter


@contextlib.contextmanager
def pushd(path: Path):
    old = Path.cwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(old)


def capture_output(fn, argv: list[str]) -> str:
    stream = io.StringIO()
    with redirect_stdout(stream):
        rc = fn(argv)
    if rc != 0:
        raise AssertionError(f"command failed with exit code {rc}")
    return stream.getvalue()


class GitRepoTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.repo = Path(self.temp_dir.name)
        self.git("init")
        self.git("config", "user.name", "Test User")
        self.git("config", "user.email", "test@example.com")

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def git(self, *args: str, input_text: str | None = None) -> str:
        completed = subprocess.run(
            ["git", *args],
            cwd=self.repo,
            text=True,
            input=input_text,
            capture_output=True,
            check=True,
        )
        return completed.stdout

    def write_file(self, relative_path: str, content: str) -> None:
        path = self.repo / relative_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def commit_all(self, message: str) -> str:
        self.git("add", ".")
        self.git("commit", "-m", message)
        return self.git("rev-parse", "HEAD").strip()

    def make_fake_claude(self, script_body: str) -> Path:
        path = self.repo / "fake_claude.py"
        path.write_text(script_body, encoding="utf-8")
        path.chmod(0o755)
        return path

    def build_session_fixture(self) -> tuple[str, str, str]:
        self.write_file(
            "app.py",
            'def greet(name):\n    return f"hello {name}"\n',
        )
        self.write_file("other.py", "def helper():\n    return 'helper'\n")
        human_commit = self.commit_all("initial app")

        writer = ClaudeCodeSessionWriter(
            repo=self.repo,
            task_prompt="Add rate limiting to the greeting path",
            model="claude-sonnet-4-6",
            capture_thinking=False,
        )
        writer.start_session()
        writer.record_tool_call(
            tool="read_file",
            kind="read",
            paths=["other.py"],
            raw_input={"path": "other.py"},
            output_summary="Inspected other.py for a token bucket helper",
        )
        writer.record_tool_call(
            tool="edit_file",
            kind="write",
            paths=["app.py"],
            raw_input={"path": "app.py"},
            output_summary="Inserted rate limiting into app.py",
            raw_output_excerpt="return rate_limit(name)",
        )
        writer.record_rejected(
            "in-process counter",
            "Does not survive multiple workers",
        )
        writer.record_metrics(
            input_tokens=4821,
            output_tokens=1203,
            thinking_tokens=0,
            cost_usd=0.09,
            duration_seconds=47,
        )

        self.write_file(
            "app.py",
            "def greet(name):\n    return rate_limit(name)\n",
        )
        ai_commit_one = self.commit_all("feat: add greeting rate limiting")
        writer.attach_commit(ai_commit_one)

        writer.record_tool_call(
            tool="edit_file",
            kind="write",
            paths=["other.py"],
            raw_input={"path": "other.py"},
            output_summary="Adjusted helper naming in other.py",
            raw_output_excerpt="def rate_limit(name):",
        )
        self.write_file(
            "other.py",
            "def rate_limit(name):\n    return f'hello {name}'\n",
        )
        ai_commit_two = self.commit_all("refactor: rename helper")
        writer.attach_commit(ai_commit_two)
        session_id = writer.finalize_session()

        self.assertFalse(writer.pending_path.exists())
        return human_commit, ai_commit_one, session_id


class StorageTests(GitRepoTestCase):
    def test_write_and_read_session_round_trip(self) -> None:
        self.write_file("demo.py", "print('hello')\n")
        commit_sha = self.commit_all("demo")

        session = Session(
            agent=AgentInfo(runner="claude-code", model="claude-sonnet-4-6"),
            task=TaskInfo(prompt="Write demo"),
            commits=[commit_sha],
            tool_calls=[
                ToolCall(
                    sequence=1,
                    tool="write_file",
                    kind="write",
                    paths=["demo.py"],
                    raw_input={"path": "demo.py"},
                    output_summary="Created demo.py",
                )
            ],
            rejected_approaches=[RejectedApproach(what="shell script", why="Python package requested")],
            metrics=Metrics(input_tokens=10, output_tokens=20, cost_usd=0.01, duration_seconds=1.5),
        )
        session.finalize()
        write_session(self.repo, session)
        attach_session_to_commit(self.repo, commit_sha, session.session_id)

        session_id, loaded = read_session_for_commit(self.repo, commit_sha)
        self.assertEqual(session_id, session.session_id)
        self.assertEqual(loaded.task.prompt, "Write demo")
        self.assertEqual(loaded.commits, [commit_sha])
        self.assertEqual(loaded.tool_calls[0].paths, ["demo.py"])

    def test_read_session_rejects_corrupt_json(self) -> None:
        blob = run_git(self.repo, ["hash-object", "-w", "--stdin"], input_text="not json").stdout.strip()
        run_git(self.repo, ["update-ref", "refs/sessions/bad", blob])

        with self.assertRaises(InvalidSessionBlobError):
            read_session(self.repo, "bad")


class CommandTests(GitRepoTestCase):
    def test_session_init_is_idempotent_and_local_only(self) -> None:
        self.write_file("app.py", "print('hello')\n")
        self.commit_all("initial")

        with pushd(self.repo):
            first = capture_output(session_main, ["init"])
            second = capture_output(session_main, ["init"])

        self.assertIn("Initialized git-cognition", first)
        self.assertIn("schema:   1.0", second)
        enabled = self.git("config", "--local", "--get", CONFIG_ENABLED_KEY).strip()
        schema = self.git("config", "--local", "--get", CONFIG_SCHEMA_KEY).strip()
        self.assertEqual(enabled, "true")
        self.assertEqual(schema, "1.0")
        self.assertEqual(self.git("status", "--short").strip(), "")

    def test_session_commands_and_git_why(self) -> None:
        human_commit, ai_commit, session_id = self.build_session_fixture()

        with pushd(self.repo):
            ls_output = capture_output(session_main, ["ls", "--file", "app.py"])
            show_output = capture_output(session_main, ["show", session_id[:8]])
            grep_output = capture_output(session_main, ["grep", "other.py"])
            stat_output = capture_output(session_main, ["stat"])
            why_json = capture_output(why_main, ["app.py:2", "--json"])
            why_fallback = capture_output(why_main, ["app.py:1"])

        self.assertIn(session_id[:8], ls_output)
        self.assertIn("Add rate limiting to the greeting path", show_output)
        self.assertIn("feat: add greeting rate limiting", show_output)
        self.assertIn('searching: "other.py"  scope: all', grep_output)
        self.assertIn("claude-sonnet-4-6", stat_output)
        self.assertIn("app.py", stat_output)

        why_payload = json.loads(why_json)
        self.assertEqual(why_payload["commit"], ai_commit)
        self.assertEqual(why_payload["retrieval"]["top_matches"][0]["sequence"], 2)

        expected_blame = self.git("blame", "-L", "1,1", "--", "app.py")
        self.assertEqual(why_fallback, expected_blame)
        self.assertNotEqual(human_commit, ai_commit)

    def test_session_claude_wrapper_auto_attaches_new_commit(self) -> None:
        self.write_file(
            "app.py",
            'def greet(name):\n    return f"hello {name}"\n',
        )
        self.commit_all("initial app")

        fake_claude = self.make_fake_claude(
            """#!/usr/bin/env python3
import json
from pathlib import Path
import subprocess

repo = Path.cwd()
app = repo / "app.py"
app.write_text("def greet(name):\\n    return rate_limit(name)\\n", encoding="utf-8")
subprocess.run(["git", "add", "app.py"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
subprocess.run(
    ["git", "commit", "-m", "feat: wrapped change"],
    check=True,
    stdout=subprocess.DEVNULL,
    stderr=subprocess.DEVNULL,
)

usage = {
    "input_tokens": 111,
    "output_tokens": 42,
    "thinking_tokens": 0,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "server_tool_use": {"web_search_requests": 0, "web_fetch_requests": 0},
    "service_tier": "standard",
    "cache_creation": {"ephemeral_1h_input_tokens": 0, "ephemeral_5m_input_tokens": 0},
    "inference_geo": "",
    "iterations": [],
    "speed": "standard",
}
events = [
    {
        "type": "system",
        "subtype": "init",
        "session_id": "claude-session-123",
        "model": "claude-sonnet-4-6",
        "tools": ["Read", "Edit"],
    },
    {
        "type": "assistant",
        "session_id": "claude-session-123",
        "message": {
            "id": "msg-1",
            "model": "claude-sonnet-4-6",
            "role": "assistant",
            "type": "message",
            "usage": usage,
            "content": [
                {"type": "tool_use", "id": "toolu-read", "name": "Read", "input": {"path": "app.py"}},
                {
                    "type": "tool_use",
                    "id": "toolu-edit",
                    "name": "Edit",
                    "input": {"path": "app.py", "new_string": "return rate_limit(name)"},
                },
            ],
        },
    },
    {
        "type": "user",
        "session_id": "claude-session-123",
        "message": {
            "role": "user",
            "type": "message",
            "content": [
                {"type": "tool_result", "tool_use_id": "toolu-read", "content": "Found greet() in app.py"},
                {"type": "tool_result", "tool_use_id": "toolu-edit", "content": "Updated app.py successfully"},
            ],
        },
    },
    {
        "type": "assistant",
        "session_id": "claude-session-123",
        "message": {
            "id": "msg-2",
            "model": "claude-sonnet-4-6",
            "role": "assistant",
            "type": "message",
            "usage": usage,
            "content": [
                {"type": "text", "text": "Used a direct edit rather than an in-process counter."}
            ],
        },
    },
    {
        "type": "result",
        "subtype": "success",
        "is_error": False,
        "duration_ms": 1200,
        "result": "Implemented the change and committed it.",
        "session_id": "claude-session-123",
        "total_cost_usd": 0.02,
        "usage": usage,
    },
]
for event in events:
    print(json.dumps(event))
"""
        )

        with pushd(self.repo):
            capture_output(session_main, ["init"])
            old_value = os.environ.get("GIT_COGNITION_CLAUDE_BIN")
            os.environ["GIT_COGNITION_CLAUDE_BIN"] = str(fake_claude)
            try:
                wrapper_json = capture_output(
                    session_main,
                    ["claude", "--json", "Update app.py and commit the change"],
                )
            finally:
                if old_value is None:
                    del os.environ["GIT_COGNITION_CLAUDE_BIN"]
                else:
                    os.environ["GIT_COGNITION_CLAUDE_BIN"] = old_value

            wrapper_payload = json.loads(wrapper_json)
            show_output = capture_output(session_main, ["show", wrapper_payload["session_id"][:8]])
            why_json = capture_output(why_main, ["app.py:2", "--json"])

        self.assertEqual(wrapper_payload["claude_session_id"], "claude-session-123")
        self.assertEqual(wrapper_payload["tool_call_count"], 2)
        self.assertEqual(len(wrapper_payload["attached_commits"]), 1)
        self.assertIn("feat: wrapped change", show_output)

        why_payload = json.loads(why_json)
        self.assertEqual(why_payload["commit"], wrapper_payload["attached_commits"][0])
        self.assertEqual(why_payload["retrieval"]["top_matches"][0]["sequence"], 2)

    def test_session_attach_adds_commit_note_after_the_fact(self) -> None:
        self.write_file("late.py", "print('late')\n")
        commit_sha = self.commit_all("late commit")

        session = Session(
            agent=AgentInfo(runner="claude-code", model="claude-sonnet-4-6"),
            task=TaskInfo(prompt="Attach later"),
            tool_calls=[
                ToolCall(
                    sequence=1,
                    tool="write_file",
                    kind="write",
                    paths=["late.py"],
                    raw_input={"path": "late.py"},
                    output_summary="Created late.py",
                )
            ],
        )
        session.finalize()
        write_session(self.repo, session)

        with pushd(self.repo):
            attach_output = capture_output(session_main, ["attach", session.session_id[:8], "HEAD"])

        attached_session_id, loaded = read_session_for_commit(self.repo, commit_sha)
        self.assertEqual(attached_session_id, session.session_id)
        self.assertIn(commit_sha, loaded.commits)
        self.assertIn(session.session_id[:8], attach_output)


if __name__ == "__main__":
    unittest.main()
