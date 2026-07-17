# litt

[English](./README.md)

AI agent 的本地优先任务图与执行追踪器。

## 功能

- **SQLite 存储 issue** — 不再用易损坏的 Markdown 文件
- **父子层级** — feature 包含 task
- **阻塞图** — 带环检测的有向依赖
- **分类 / 类别 / 自定义标签** — 分类标签互斥
- **派生 ready 查询** — open + 已分类 + 未阻塞，按需计算
- **CLI** — create, list, show, update, close, parent, block, ready
- **MCP stdio 服务** — agent 通过类型化工具交互，而非自由文本
- **Agent 自动安装** — `litt agent install` 向 AGENTS.md 注入托管指令

## 安装

```bash
git clone https://github.com/ytmee/litt.git
cd litt
go build -o litt .
```

或从 [releases 页面](https://github.com/ytmee/litt/releases) 下载二进制。

## 快速开始

```bash
# 初始化
litt init

# 创建 issue
litt issue create "添加深色模式" --kind feature
litt issue create "实现切换按钮" --kind task --body "..."

# 查看待办事项
litt issue ready

# 组织任务结构
litt issue parent set 2 1
litt issue block 2 1
```

## CLI 参考

| 命令 | 说明 |
|---|---|
| `litt init` | 初始化 litt 仓库 |
| `litt issue create <title>` | 创建 issue（`--kind`, `--body`, `--label`） |
| `litt issue list` | 列出 issue（`--state`, `--kind`, `--label`, `--json`） |
| `litt feature create <title>` | `issue create --kind feature` 的快捷别名 |
| `litt feature list` | `issue list --kind feature` 的快捷别名 |
| `litt issue show <n>` | 查看 issue 详情 |
| `litt issue update <n>` | 更新 issue（`--title`, `--body`, `--state`, `--add-label`, `--remove-label`） |
| `litt issue close <n>` | 关闭 issue |
| `litt issue ready` | 列出可执行的任务（`--json`） |
| `litt issue parent set/clear` | 管理父子层级 |
| `litt issue children <n>` | 列出子 issue |
| `litt issue block/unblock` | 管理阻塞关系 |
| `litt label list` | 列出标签（`--json`） |
| `litt mcp` | 启动 MCP stdio 服务 |
| `litt agent install` | 向 AGENTS.md 注入 agent 指令 |

## AI agent 集成

**MCP 服务** — `litt mcp` 启动一个 stdio MCP 服务，提供所有 issue 操作工具：
`create_issue`, `update_issue`, `query_issues`, `get_issue`,
`get_ready_issues`, `set_parent`, `clear_parent`, `add_blocking`, `remove_blocking`。

在 agent 的 MCP 配置中添加：

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

**Agent 指令** — `litt agent install` 向 `AGENTS.md` 注入一个托管文本块
（用 `--target CLAUDE.md` 指定 Claude Code），告知 agent 使用 litt
而非 Markdown 文件来管理 issue。

## 许可证

MIT
