from __future__ import annotations

import sys
from pathlib import Path
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from git_cognition.storage.schema import (
    MAX_FOLLOW_UP_PROMPTS,
    MAX_OUTPUT_SNAPSHOT_CHARS,
    MAX_PROMPT_CHARS,
    TaskInfo,
    ToolCall,
)


class SchemaTests(unittest.TestCase):
    def test_tool_call_truncates_large_fields(self) -> None:
        call = ToolCall(
            sequence=1,
            tool="read_file",
            paths=["app.py", "app.py"],
            raw_input={"payload": "x" * 17000},
            output_summary="y" * 1000,
            raw_output_excerpt="z" * 18000,
            output_snapshot="s" * 70000,
        )

        self.assertEqual(call.paths, ["app.py"])
        self.assertIsInstance(call.raw_input, dict)
        self.assertTrue(call.raw_input.get("truncated"))
        self.assertIn("truncated", call.output_summary)
        self.assertIn("truncated", call.raw_output_excerpt)
        self.assertLessEqual(len(call.output_snapshot or ""), MAX_OUTPUT_SNAPSHOT_CHARS)

    def test_task_info_tracks_follow_up_prompts_with_caps(self) -> None:
        task = TaskInfo(
            prompt="p" * (MAX_PROMPT_CHARS + 100),
            follow_up_prompts=[f"prompt-{index}" for index in range(MAX_FOLLOW_UP_PROMPTS + 5)],
        )

        self.assertLessEqual(len(task.prompt), MAX_PROMPT_CHARS)
        self.assertEqual(len(task.follow_up_prompts), MAX_FOLLOW_UP_PROMPTS)
        self.assertEqual(task.follow_up_prompts[0], "prompt-0")
        self.assertEqual(task.follow_up_prompts[-1], f"prompt-{MAX_FOLLOW_UP_PROMPTS - 1}")
        self.assertIn("prompt-0", task.search_text())


if __name__ == "__main__":
    unittest.main()
