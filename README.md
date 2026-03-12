# agentcom

`agentcom` is a Go CLI for real-time communication between parallel AI coding agent sessions.

It uses SQLite in WAL mode as the durable source of truth and Unix Domain Sockets for low-latency local delivery, with SQLite inbox polling as a fallback path.

## Features

- Register and deregister long-running agent sessions
- Send direct messages and broadcasts between agents
- Persist messages, tasks, and agent liveness in SQLite
- Delegate tasks between agents with a simple state machine
- Expose an MCP JSON-RPC server over STDIO
- Run entirely on one machine with SQLite as the only external dependency

## Requirements

- Go 1.22+
- CGO enabled
- SQLite3 toolchain support for `github.com/mattn/go-sqlite3`

## Install

```bash
make build
./bin/agentcom version
```

Or install directly:

```bash
make install
agentcom version
```

## Quickstart

Initialize local state:

```bash
agentcom init
```

Generate a project-level usage guide for other agents:

```bash
agentcom init --agents-md
```

Start two agents in separate terminals:

```bash
agentcom register --name alpha --type codex
agentcom register --name beta --type claude
```

Send a direct message:

```bash
agentcom send --from alpha beta '{"text":"hello"}'
```

Broadcast an update:

```bash
agentcom broadcast --from alpha --topic sync '{"status":"ready"}'
```

Create and delegate a task:

```bash
agentcom task create "Implement MCP tests" --creator alpha
agentcom task delegate <task-id> --to beta
agentcom task update <task-id> --status in_progress --result "started"
```

Inspect inbox and status:

```bash
agentcom inbox --agent beta --unread
agentcom status
```

## CLI Reference

- `agentcom init` - initialize local home, DB, and sockets directory
- `agentcom register --name <name> --type <type>` - register current process as an agent
- `agentcom deregister <name-or-id> --force` - remove an agent record
- `agentcom list [--alive]` - list agents
- `agentcom send --from <sender> <target> <message>` - send one direct message
- `agentcom broadcast --from <sender> <message>` - send to all alive agents
- `agentcom inbox --agent <name-or-id> [--unread]` - inspect agent inbox
- `agentcom task create|list|update|delegate` - manage tasks
- `agentcom status` - print aggregate counts
- `agentcom health` - basic health checks
- `agentcom mcp-server` - start MCP over STDIO
- `agentcom version` - show build metadata

Use `--json` on commands that support machine-readable output.

## MCP Setup

Start the MCP server:

```bash
agentcom mcp-server
```

Handshake outline:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

Available tools:

- `list_agents`
- `send_message`
- `broadcast`
- `create_task`
- `delegate_task`
- `list_tasks`
- `get_status`

## Architecture

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

## Development

```bash
make build
make test
make lint
make vet
```

The current test suite includes DB CRUD tests, registry tests, transport integration tests, task manager tests, MCP roundtrip tests, and an end-to-end CLI scenario.
