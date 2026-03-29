from __future__ import annotations

import sys
from pathlib import Path
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from git_cognition.storage.schema import MAX_OUTPUT_SNAPSHOT_CHARS, ToolCall


class SchemaTests(unittest.TestCase):
    def test_tool_call_truncates_large_fields(self) -> None:
        call = ToolCall(
            sequence=1,
            tool="read_file",
            paths=["app.py", "app.py"],
            raw_input={"payload": "x" * 5000},
            output_summary="y" * 1000,
            raw_output_excerpt="z" * 6000,
            output_snapshot="s" * 25000,
        )

        self.assertEqual(call.paths, ["app.py"])
        self.assertIsInstance(call.raw_input, dict)
        self.assertTrue(call.raw_input.get("truncated"))
        self.assertIn("truncated", call.output_summary)
        self.assertIn("truncated", call.raw_output_excerpt)
        self.assertLessEqual(len(call.output_snapshot or ""), MAX_OUTPUT_SNAPSHOT_CHARS)


if __name__ == "__main__":
    unittest.main()
