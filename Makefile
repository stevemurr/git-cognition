PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
PYTHON_BIN ?= python3
SESSION_BIN ?= $(BINDIR)/git-session
WHY_BIN ?= $(BINDIR)/git-why
PLUGIN_LAUNCHER_NAME ?= claude-git-cognition
CLAUDE_PLUGIN_BIN ?= $(BINDIR)/$(PLUGIN_LAUNCHER_NAME)

.PHONY: help install uninstall install-cli-shims uninstall-cli-shims test validate-claude-plugin install-claude-plugin uninstall-claude-plugin

help:
	@printf '%s\n' \
		'install                Install git-session and git-why wrappers under $(BINDIR)' \
		'uninstall              Remove installed git-session and git-why wrappers' \
		'test                   Run the unit test suite' \
		'validate-claude-plugin Validate the repo-local Claude plugin manifest' \
		'install-claude-plugin  Install the claude-git-cognition launcher' \
		'uninstall-claude-plugin Remove the installed launcher'

install: install-cli-shims

uninstall: uninstall-cli-shims

install-cli-shims:
	PYTHON_BIN="$(PYTHON_BIN)" \
	BINDIR="$(BINDIR)" \
	SESSION_TARGET="$(SESSION_BIN)" \
	WHY_TARGET="$(WHY_BIN)" \
	./scripts/install_cli_wrappers.sh

uninstall-cli-shims:
	BINDIR="$(BINDIR)" \
	SESSION_TARGET="$(SESSION_BIN)" \
	WHY_TARGET="$(WHY_BIN)" \
	./scripts/uninstall_cli_wrappers.sh

test:
	python3 -m unittest discover -s tests -p 'test*.py' -v

validate-claude-plugin:
	claude plugins validate claude-plugin

install-claude-plugin:
	PREFIX="$(PREFIX)" \
	BINDIR="$(BINDIR)" \
	TARGET="$(CLAUDE_PLUGIN_BIN)" \
	./scripts/install_claude_plugin.sh

uninstall-claude-plugin:
	PREFIX="$(PREFIX)" \
	BINDIR="$(BINDIR)" \
	TARGET="$(CLAUDE_PLUGIN_BIN)" \
	./scripts/uninstall_claude_plugin.sh
