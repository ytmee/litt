# litt

[English](./README.md)

[![CI](https://github.com/ytmee/litt/actions/workflows/ci.yml/badge.svg)](https://github.com/ytmee/litt/actions/workflows/ci.yml) [![Go Version](https://img.shields.io/badge/Go-1.26-blue?logo=go)](https://go.dev) [![License](https://img.shields.io/badge/License-MIT-green)](./LICENSE)

AI agent 的本地优先任务图与执行追踪器。

## 功能

- **SQLite 存储 issue** — 不再用易损坏的 Markdown 文件
- **父子层级** — spec 包含 task
- **阻塞图** — 带环检测的有向依赖
- **分类 / 类别 / 自定义标签** — 分类标签互斥
- **派生 ready 查询** — open + 已分类 + 未阻塞，按需计算
- **CLI** — create, list, show, update, close, parent, block, ready
- **MCP stdio 服务** — agent 通过类型化工具交互，而非自由文本
- **Agent 自动安装** — `litt agent install` 向 AGENTS.md 注入托管指令

## 安装

```bash
# 方式一：通过 go 安装
go install github.com/ytmee/litt@latest

# 方式二：从源码编译
git clone https://github.com/ytmee/litt.git
cd litt
go build -ldflags="-s -w" -o litt .
sudo mv litt /usr/local/bin/
```

或从 [releases 页面](https://github.com/ytmee/litt/releases) 下载预编译的二进制：

```bash
# 示例：Linux amd64
curl -LO https://github.com/ytmee/litt/releases/latest/download/litt_linux_amd64
sudo mv litt_linux_amd64 /usr/local/bin/litt
sudo chmod +x /usr/local/bin/litt
```

## 快速开始

```bash
# 初始化
litt init

# 创建 issue
litt issue create "添加深色模式设计" --kind spec
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
`get_ready_issues`, `set_parent`, `clear_parent`, `add_blocking`, `remove_blocking`,
`add_comment`, `get_comments`, `create_label`, `delete_label`, `list_labels`。

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

## 性能

```
环境:          SQLite (WAL 模式), ext4, AMD64, 5,600 issues 数据集
CLI 开销:      ~6ms/调用 (进程启动)
MCP 模式:      消除进程开销，预期 3-5x 提升

顺序吞吐量 (单进程):
  Issue create  ..............  266/s       (1.9ms each)
  Issue update  ..............  270/s       (3.7ms each)
  Issue show (by ID) .........  278 qps     (3.6ms each)
  Issue list (700 issues) ....   41 qps     (55ms each)
  Issue list (5,600 issues) ..    6 qps     (147ms each)
  Create edge (blocking) .....  267/s       (3.7ms each)
  Add comment ................  258/s       (3.9ms each)
  Create label ...............  235/s       (4.3ms each)
  Mixed pipeline (C+R+U+C) ..  257 ops/s   (15ms per 4-op pipeline)
  Large body (100KB) .........    9ms       (single issue)
  Large body (1MB) ...........   20ms       (single issue)

并发吞吐量 (并行进程, WAL 模式):
  4 workers create ...........  769/s       (3.0x vs sequential)
  16 workers create .......... 1,059/s      (4.1x)
  32 workers create .......... 1,161/s      (4.5x, 接近饱和)
  4 workers mixed R+W ........  806 ops/s   (3.1x)
  4 workers same-issue race ..  735/s       (2.8x, 零死锁)
  8 workers query ............  213 qps     (5.2x)

磁盘占用:
  5,600 issues ...............  1.3 MB      (~230 B/issue)
  5,751 issues + blobs .......  1.7 MB

瓶颈: 进程启动 (6ms)，非 SQLite。MCP 持连接模式消除此开销。
```

## 许可证

MIT
