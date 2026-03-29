"""Session writers for git-cognition."""

from .base import AgentSessionWriter
from .claude_code import ClaudeCodeSessionWriter

__all__ = ["AgentSessionWriter", "ClaudeCodeSessionWriter"]
