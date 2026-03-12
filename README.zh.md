# agentcom

`agentcom` 是一个用 Go 编写的 CLI 工具，用于并行 AI 编码代理会话之间的实时通信。

它使用 SQLite WAL 模式保存持久状态，使用 Unix Domain Socket 做低延迟本地投递；如果目标 socket 不可用，则回退到 SQLite inbox polling。

## 功能特性

- 注册和注销长时间运行的代理会话
- 代理之间的单播消息和广播消息
- 将消息、任务和代理状态持久化到 SQLite
- 基于简单状态机的任务委派
- 为支持的编码代理生成 project/user 级别的 `SKILL.md`
- 通过 STDIO 暴露 MCP JSON-RPC 服务
- 仅依赖 SQLite，适合单机使用

## 环境要求

- Go 1.22+
- 启用 CGO
- 为 `github.com/mattn/go-sqlite3` 准备好 SQLite3 构建环境

## 安装

### 最简单的安装方式

如果你想按平台用最短命令安装，可以直接用下面这些方式。

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.ps1 | iex
```

如果你更喜欢包管理器：

```bash
# macOS / Linux
brew tap malleus35/tap && brew install agentcom
```

```powershell
# Windows（无需 bucket add）
scoop install https://raw.githubusercontent.com/malleus35/agentcom/main/packaging/scoop/agentcom.json
```

Scoop 直接从 URL 安装是官方支持的，但因为它不是从已注册 bucket 安装，所以不能享受正常的 `scoop update agentcom` 更新流程。

本地构建：

```bash
make build
./bin/agentcom version
```

安装到 Go bin 目录：

```bash
make install
agentcom version
```

直接通过 Go 安装：

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
agentcom version
```

对于 Go 用户来说这是最简单的方法，但目标机器必须安装 Go，并具备 CGO/SQLite 构建环境。

## 跨平台安装方式

如果你希望在 macOS、Linux、Windows 上尽量方便地安装，一般使用以下方式之一。

### 1. GitHub Releases 二进制包

这是最适合普通用户的方式。按当前发布配置，可生成以下目标包：

- macOS: `darwin/amd64`, `darwin/arm64`
- Linux: `linux/amd64`, `linux/arm64`
- Windows: `windows/amd64`, `windows/arm64`

从 GitHub Releases 下载对应系统和架构的压缩包，解压后将二进制放到 `PATH` 中即可。

```bash
# macOS / Linux
tar -xzf agentcom_<version>_darwin_arm64.tar.gz
chmod +x agentcom
mv agentcom /usr/local/bin/
```

```powershell
# Windows PowerShell
Expand-Archive .\agentcom_<version>_windows_amd64.zip -DestinationPath .\agentcom
$env:Path += ";C:\path\to\agentcom"
```

### 2. Homebrew

当前仓库的 `.goreleaser.yml` 已包含 Homebrew 配置。发布后如果 tap 可用，macOS 和 Linux 用户可以通过 Homebrew 安装。

```bash
brew tap malleus35/tap
brew install agentcom
```

如果想做到不需要 `brew tap`、直接 `brew install agentcom`，就需要把 formula 合并进 `homebrew-core`。

### 2a. 进入 Homebrew core 需要什么？

大致需要满足这些前提：

- 为各支持平台提供稳定的公开发布压缩包和校验值
- formula 能通过 `brew audit` 和 `brew test`
- 向 `homebrew-core` 提交并被接受
- 后续持续按照 Homebrew 的要求维护

在那之前，tap 方式仍然是最现实的 Homebrew 安装路径。

### 3. `go install`

适合 Go 开发者或贡献者：

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
```

### 4. 手动本地构建

适合内部打包或者你希望完全自己控制安装过程。

```bash
make build
./bin/agentcom version
```

### 如何选择

- macOS/Linux 普通用户：安装脚本或 Homebrew tap
- Windows 普通用户：PowerShell 安装脚本或 Scoop URL 直接安装
- 任意系统的手动安装：GitHub Releases 二进制包
- 已经使用 Homebrew 的用户：Homebrew tap
- Go 开发者：`go install`
- 本地开发或内部打包：`make build`

## agentcom 如何工作

`agentcom` 会把已注册代理、消息、任务以及状态检查相关时间戳全部存入 SQLite。每个已注册进程还会打开自己的 Unix Domain Socket 服务，因此 direct message 会优先尝试通过 socket 投递；如果失败，就回退到 SQLite inbox polling。

常见使用流程如下：

1. 初始化本地 agentcom home 目录
2. 启动一个或多个长时间运行的已注册代理进程
3. 使用 CLI 命令或 MCP 工具处理消息、inbox 和任务
4. 正常关闭进程，让代理自动注销

## 本地状态和配置

默认使用 `~/.agentcom`：

- SQLite DB: `~/.agentcom/agentcom.db`
- Socket 目录: `~/.agentcom/sockets/`

可以用 `AGENTCOM_HOME` 覆盖基础目录：

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
```

## 全局参数

所有命令都支持以下全局参数：

- `--json` - 在支持时输出机器可读的 JSON
- `-v`, `--verbose` - 打开基于 `log/slog` 的调试日志

```bash
agentcom --json list
agentcom --verbose health
```

## 快速开始

```bash
agentcom init
agentcom init --agents-md
```

在不同终端启动两个代理：

```bash
agentcom register --name alpha --type codex --cap plan,execute
agentcom register --name beta --type claude --cap review,test
```

发送消息：

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom broadcast --from alpha --topic sync '{"status":"ready"}'
```

操作任务：

```bash
agentcom task create "Implement MCP tests" --creator alpha --assign beta --priority high
agentcom task list --assignee beta
agentcom task delegate <task-id> --to beta
agentcom task update <task-id> --status in_progress --result "started"
```

查看状态：

```bash
agentcom inbox --agent beta --unread
agentcom status
agentcom health
```

为所有支持的代理 CLI 生成 project 级技能文件：

```bash
agentcom skill create review-pr --agent all --scope project --description "统一的 PR 审查"
```

## 各命令详细说明

### `agentcom init`

初始化 home 目录，确保子目录存在，打开 SQLite 数据库并应用迁移。

```bash
agentcom init
agentcom init --agents-md
agentcom --json init
```

- 可重复执行
- `--agents-md` 会在当前目录生成 `AGENTS.md`
- JSON 输出可能包含 `path`、`status`、`agents_md`
- 当前实现会先准备 home 目录再检查状态，因此即使是新路径，`status` 也可能显示为 `already_initialized`

### `agentcom register`

将当前进程注册为 live agent，并启动 heartbeat、Unix Domain Socket 服务和 fallback poller。这个命令本身是长时间运行的。

```bash
agentcom register --name alpha --type codex
agentcom register --name alpha --type codex --cap plan,execute --workdir /path/to/project
agentcom --json register --name alpha --type codex
```

- `--name` - 必填，唯一代理名
- `--type` - 必填，自由字符串类型
- `--cap` - 可选，逗号分隔的 capability 列表
- `--workdir` - 可选，工作目录，默认当前目录

### `agentcom deregister`

按名称或 ID 注销代理。

```bash
agentcom deregister alpha
agentcom deregister agt_xxx --force
agentcom --json deregister alpha --force
```

### `agentcom list`

查看已注册代理。

```bash
agentcom list
agentcom list --alive
agentcom --json list
```

### `agentcom send`

从已注册 sender 向 target agent 发送 direct message。

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom send --from alpha beta 'plain text message'
agentcom send --from alpha --type request --topic review beta '{"file":"README.md"}'
agentcom send --from alpha --correlation-id corr-123 beta '{"step":1}'
```

- 如果第二个参数是有效 JSON object/array，就原样存储
- 否则会包装成 `{"text":"..."}`

### `agentcom broadcast`

向除 sender 外的所有 alive agent 发送同一条消息。

```bash
agentcom broadcast --from alpha '{"status":"ready"}'
agentcom broadcast --from alpha --topic sync '{"phase":"wave-9"}'
```

### `agentcom inbox`

从 SQLite 读取某个 agent 的 inbox 消息。

```bash
agentcom inbox --agent beta
agentcom inbox --agent beta --unread
agentcom inbox --agent agt_xxx --from agt_sender_id
agentcom --json inbox --agent beta --unread
```

### `agentcom task`

任务管理分为 4 个子命令。

```bash
agentcom task create "Implement docs" --desc "Expand README" --creator alpha --assign beta --priority high
agentcom task list --status pending
agentcom task update <task-id> --status completed --result "done"
agentcom task delegate <task-id> --to beta
```

- `task create` 会以 `pending` 状态保存新任务
- `--assign`、`--creator` 可接受 agent 名称或 ID
- `task delegate` 会把 `assigned_to` 更新为目标代理

### `agentcom status`

显示代理总数、消息总数、未读数、任务总数以及各状态任务数量。

```bash
agentcom status
agentcom --json status
```

### `agentcom skill`

为支持的编码代理 CLI 创建 `SKILL.md` 文件。

用法示例：

```bash
agentcom skill create review-pr --agent claude --scope project --description "统一的 PR 审查"
agentcom skill create release-check --agent all --scope user
agentcom --json skill create docs-sync --agent gemini --scope project
```

参数：

- `--agent` - 目标代理：`claude|codex|gemini|opencode|all`（默认 `all`）
- `--scope` - 输出范围：`project|user`（默认 `project`）
- `--description` - 可选技能描述，默认值为 `Skill generated by agentcom`

生成路径：

- `claude` - project: `.claude/skills/<name>/SKILL.md`, user: `~/.claude/skills/<name>/SKILL.md`
- `codex` - project: `.agents/skills/<name>/SKILL.md`, user: `~/.agents/skills/<name>/SKILL.md`
- `gemini` - project: `.gemini/skills/<name>/SKILL.md`, user: `~/.gemini/skills/<name>/SKILL.md`
- `opencode` - project: `.opencode/skills/<name>/SKILL.md`, user: `~/.config/opencode/skills/<name>/SKILL.md`

说明：

- 技能名只能使用小写字母、数字和单个连字符。
- 不会覆盖已有的 `SKILL.md` 文件。
- `--agent all` 会依次为所有支持的代理写入文件，并在第一次写入失败时立即停止。

### `agentcom health`

显示已注册代理的健康状态视图。

```bash
agentcom health
agentcom --json health
```

空环境下 JSON 输出为 `[]`。

### `agentcom version`

输出构建元信息。

```bash
agentcom version
agentcom --json version
```

### `agentcom mcp-server`

通过 STDIO 启动 MCP JSON-RPC 服务。

```bash
agentcom mcp-server
agentcom mcp-server --register mcp-agent --type mcp
```

- 调用 `tools/list`、`tools/call` 之前必须先 `initialize`
- 提供的工具包括代理列表、消息发送、广播、任务创建/委派、任务列表、状态查询

## JSON 输出示例

```bash
agentcom --json init
agentcom --json list
agentcom --json send --from alpha beta '{"text":"hello"}'
agentcom --json task list --status pending
```

```json
{
  "path": "/Users/name/.agentcom",
  "status": "initialized or already_initialized"
}
```

## 端到端示例流程

终端 1：

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
agentcom register --name alpha --type codex --cap plan,execute
```

终端 2：

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom register --name beta --type claude --cap review,test
```

终端 3：

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom send --from alpha beta '{"text":"please review README"}'
agentcom inbox --agent beta --unread
agentcom task create "Review README" --creator alpha --assign beta --priority high
agentcom task list --assignee beta
agentcom status
agentcom health
```

完成后可通过 `Ctrl+C` 停止注册中的进程。

## MCP 用法

```bash
agentcom mcp-server
```

握手示例：

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

可用工具：

- `list_agents`
- `send_message`
- `broadcast`
- `create_task`
- `delegate_task`
- `list_tasks`
- `get_status`

## 架构

```text
                  +----------------------+
                  |     agentcom CLI     |
                  +----------+-----------+
                             |
          +------------------+------------------+
          |                                     |
          v                                     v
 +-------------------+                 +-------------------+
 |   SQLite (WAL)    |                 | Unix Domain Socket|
 | agents/messages/  |                 | server + client   |
 | tasks source      |                 | low-latency IPC   |
 +---------+---------+                 +---------+---------+
           |                                     |
           +------------------+------------------+
                              |
                              v
                    +-------------------+
                    | fallback poller   |
                    | unread inbox scan |
                    +-------------------+
```

## 开发

```bash
make build
make test
make lint
make vet
```

当前测试套件包含 DB CRUD、registry、transport integration、task manager、MCP roundtrip，以及 end-to-end CLI 场景测试。
