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
- **WebUI** — 浏览器界面，用于扫描任务板和轻量三审（`litt web`）
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
| `litt web` | 在 `127.0.0.1:57664` 启动 WebUI，被占时自动顺延到下一空闲端口（`--addr` 严格绑定） |
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

## 基准测试

环境: SQLite (WAL), ext4, AMD64, 5,752 issues 数据集
CLI 开销: ~6ms/调用 (进程启动)

### 顺序吞吐量 (单进程)

| 操作 | 吞吐量 | 延迟 |
|---|---|---|
| Issue create | 282/s | 3.5ms |
| Issue update | 269/s | 3.7ms |
| Issue show (by ID) | 240 qps | 4.2ms |
| Issue list (全部 5,752) | 7 qps | 151ms |
| Blocking edge create | 274/s | 3.6ms |
| Add comment | 272/s | 3.7ms |
| Label create | 304/s | 3.3ms |
| Mixed C+R+U+C pipeline | 265 ops/s | 15ms/pipe |
| Large body (100KB) | — | 6ms |
| Large body (1MB) | — | 13ms |

### 并发吞吐量 (WAL, 并行进程)

| 工作数 | 场景 | 吞吐量 | 相比顺序 |
|---|---|---|---|
| 4 | Create | 747/s | 2.8× |
| 16 | Create | 1,011/s | 3.8× |
| 32 | Create | 1,019/s | 3.8× (饱和) |
| 4 | Mixed R+W | 840/s | 3.1× |
| 4 | Same-issue race | 746/s | 2.8× (零死锁) |
| 8 | Query (list) | 62 qps | 2,600 issues |

### 磁盘占用

| 数据集 | 大小 | 单 issue |
|---|---|---|
| 2,600 issues (基础) | 232 KB | ~90 B |
| 5,752 issues (含评论和边) | 3.6 MB | ~640 B |

**瓶颈**: CLI 进程启动 (~6ms)，非 SQLite。MCP 持连接模式消除此开销，预期 3–5× 提升。

## 许可证

MIT
