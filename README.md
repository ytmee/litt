# litt

[中文](./README-zh.md)

[![CI](https://github.com/ytmee/litt/actions/workflows/ci.yml/badge.svg)](https://github.com/ytmee/litt/actions/workflows/ci.yml) [![Go Version](https://img.shields.io/badge/Go-1.26-blue?logo=go)](https://go.dev) [![License](https://img.shields.io/badge/License-MIT-green)](./LICENSE)

A local-first task graph and execution tracker for AI agents.

## Features

- **SQLite-backed issue storage** — no Markdown files to corrupt
- **Parent/child hierarchy** — specs containing tasks
- **Blocking graph** — directed dependencies with cycle detection
- **Triage / category / custom labels** — mutual exclusion for triage labels
- **Derived ready query** — open + triaged + unblocked issues, computed on demand
- **CLI** — create, list, show, update, close, parent, block, ready
- **MCP stdio server** — agents interact through typed tools, not freeform text
- **Agent auto-install** — `litt agent install` injects managed instructions into AGENTS.md

## Installation

```bash
# Option 1: install via go
go install github.com/ytmee/litt@latest

# Option 2: build from source
git clone https://github.com/ytmee/litt.git
cd litt
go build -ldflags="-s -w" -o litt .
sudo mv litt /usr/local/bin/
```

Or download a pre-built binary from the [releases page](https://github.com/ytmee/litt/releases):

```bash
# Example: Linux amd64
curl -LO https://github.com/ytmee/litt/releases/latest/download/litt_linux_amd64
sudo mv litt_linux_amd64 /usr/local/bin/litt
sudo chmod +x /usr/local/bin/litt
```

## Quick start

```bash
# Initialize
litt init

# Create some issues
litt issue create "Add dark mode spec" --kind spec
litt issue create "Implement toggle" --kind task --body "..."

# See what's ready for work
litt issue ready

# Structure work
litt issue parent set 2 1
litt issue block 2 1
```

## CLI reference

| Command | Description |
|---|---|
| `litt init` | Initialize a litt repository |
| `litt issue create <title>` | Create an issue (`--kind`, `--body`, `--label`) |
| `litt issue list` | List issues (`--state`, `--kind`, `--label`, `--json`) |
| `litt issue show <n>` | Show issue detail |
| `litt issue update <n>` | Update issue (`--title`, `--body`, `--state`, `--add-label`, `--remove-label`) |
| `litt issue close <n>` | Close an issue |
| `litt issue ready` | List ready-to-work issues (`--json`) |
| `litt issue parent set/clear` | Manage parent/child hierarchy |
| `litt issue children <n>` | List children |
| `litt issue block/unblock` | Manage blocking edges |
| `litt label list` | List labels (`--json`) |
| `litt mcp` | Start MCP stdio server |
| `litt agent install` | Inject agent instructions into AGENTS.md |

## AI agent integration

**MCP server** — `litt mcp` starts a stdio MCP server with tools for all issue
operations: `create_issue`, `update_issue`, `query_issues`, `get_issue`,
`get_ready_issues`, `set_parent`, `clear_parent`, `add_blocking`, `remove_blocking`.

Add to your agent's MCP configuration:

- **opencode** — `opencode.json`:
  ```json
  {
    "$schema": "https://opencode.ai/config.json",
    "mcp": {
      "servers": {
        "litt": {
          "type": "local",
          "command": ["litt", "mcp"]
        }
      }
    }
  }
  ```

- **Claude Code / Cursor / Windsurf** — `mcpServers`:
  ```json
  {
    "mcpServers": {
      "litt": {
        "command": "litt",
        "args": ["mcp"]
      }
    }
  }
  ```

**Agent instructions** — `litt agent install` injects a managed block into
`AGENTS.md` (use `--target CLAUDE.md` for Claude Code) that tells agents to
use litt instead of Markdown files for issue tracking.

## License

MIT
