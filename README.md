# agentcom

Translations: [한국어](README.ko.md) | [日本語](README.ja.md) | [中文](README.zh.md)

`agentcom` is a Go CLI for real-time communication between parallel AI coding agent sessions.

It uses SQLite in WAL mode as the durable source of truth and Unix Domain Sockets for low-latency local delivery, with SQLite inbox polling as a fallback path.

## Features

- Start and stop all template-defined agents with `agentcom up` and `agentcom down`
- Register and deregister long-running agent sessions
- Send direct messages and broadcasts between agents
- Persist messages, tasks, and agent liveness in SQLite
- Delegate tasks between agents with a simple state machine
- Generate project-scoped or user-scoped `SKILL.md` files for supported coding agents
- Scaffold built-in multi-agent templates with shared instructions and six role skills
- Inspect available built-in templates with `agentcom agents template`, including interactive search on TTY
- Expose an MCP JSON-RPC server over STDIO
- Run entirely on one machine with SQLite as the only external dependency

## Requirements

- Go 1.22+
- CGO enabled
- SQLite3 toolchain support for `github.com/mattn/go-sqlite3`

## Install

### Easiest install

If you want the shortest path per platform, use one of these:

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.ps1 | iex
```

If you prefer package managers:

```bash
# macOS / Linux (Homebrew tap)
brew tap malleus35/tap && brew install agentcom
```

```powershell
# Windows (Scoop without bucket add)
scoop install https://raw.githubusercontent.com/malleus35/agentcom/main/packaging/scoop/agentcom.json
```

The direct Scoop URL install is officially supported, but it does not give you the normal `scoop update agentcom` workflow because the app is not installed from a registered bucket.

Build locally:

```bash
make build
./bin/agentcom version
```

Install into your Go bin path:

```bash
make install
agentcom version
```

Install with Go directly:

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
agentcom version
```

This is the simplest option for Go users, but it requires Go and a working CGO/SQLite build environment on the target machine.

## Cross-platform installation options

If you want the easiest install path across macOS, Linux, and Windows, use one of these approaches.

### 1. GitHub Releases binaries

Recommended for most end users. Release archives can be built for:

- macOS: `darwin/amd64`, `darwin/arm64`
- Linux: `linux/amd64`, `linux/arm64`
- Windows: `windows/amd64`, `windows/arm64`

Download the matching archive from GitHub Releases, extract it, and put the binary on your `PATH`.

Typical examples:

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

### 2. Homebrew (configured in this repo)

This repository already includes a `.goreleaser.yml` Homebrew configuration. Once releases are published and the tap is available, macOS and Linux users can install with Homebrew.

Example flow:

```bash
brew tap malleus35/tap
brew install agentcom
```

Use this when you want easy upgrades on systems that already use Homebrew.

If you want `brew install agentcom` without `brew tap`, the formula would need to be accepted into `homebrew-core`.

### 2a. What would be needed for Homebrew core?

At a high level:

- a stable public release archive and checksum for each supported platform
- a formula that passes `brew audit` and `brew test`
- a submission to `homebrew-core`
- ongoing maintenance in line with Homebrew core policies

Until then, the tap-based command is the practical Homebrew path.

### 3. `go install`

Best for Go developers or contributors:

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
```

Pros:

- very simple for Go users
- no manual binary download needed

Trade-offs:

- requires Go to be installed
- depends on local CGO/SQLite build tooling

### 4. Manual local build

Good when you want full control or are packaging internally:

```bash
make build
./bin/agentcom version
```

### Which option should I choose?

- End users on macOS/Linux: install script or Homebrew tap
- End users on Windows: PowerShell install script or direct Scoop URL
- End users on any OS: GitHub Releases binary
- macOS/Linux users with Homebrew: Homebrew tap
- Go developers: `go install`
- Internal packaging or local development: `make build`

## How agentcom works

`agentcom` keeps all durable state in SQLite: registered agents, messages, tasks, and health-related timestamps. Each registered process also opens a Unix Domain Socket server, so direct message delivery tries the socket path first and falls back to SQLite inbox polling if the target socket is unavailable.

The normal lifecycle is:

1. Run `agentcom init` once per project to initialize local state, choose a project scope, and optionally scaffold a template.
2. Run `agentcom up` to start all roles defined by the active template as detached managed processes.
3. Use CLI commands or MCP tools to send messages, inspect inboxes, and manage tasks.
4. Run `agentcom down` to stop the managed template session cleanly.
5. Use `agentcom register` only when you want one low-level standalone agent process.

## Local state and configuration

By default, `agentcom` stores data under `~/.agentcom`.

- SQLite DB: `~/.agentcom/agentcom.db`
- Socket directory: `~/.agentcom/sockets/`

Per project, `agentcom` also uses local metadata files:

- Project config: `.agentcom.json`
- Template scaffold: `.agentcom/templates/<template>/COMMON.md` and `.agentcom/templates/<template>/template.json`
- Runtime state for `up`: `.agentcom/run/up.json`

You can override the base directory with `AGENTCOM_HOME`.

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
```

This is useful for tests, demos, per-project isolation, or running multiple independent environments on one machine.

## Global flags

Every command supports these global flags:

- `--json` - machine-readable JSON output where supported
- `-v`, `--verbose` - enable debug logging via `log/slog`
- `--project <name>` - override the current project scope
- `--all-projects` - bypass project scoping and show all projects

Examples:

```bash
agentcom --json list
agentcom --verbose health
agentcom --project myapp list
```

## Quickstart

Initialize a project and choose an active template:

```bash
agentcom init --project myapp --template company
agentcom --json init --project myapp --template company
```

Project-scoped commands read `.agentcom.json` from the current directory or any parent directory. `agentcom init --project myapp --template company` writes the project name and `template.active`, so later commands automatically scope agent lookup and `up`/`down` to that project.

Generate agent-specific instruction files in the current directory:

```bash
agentcom init --agents-md
agentcom init --batch --agents-md claude,codex
```

If the target file already exists, agentcom appends or updates only its own marker-wrapped block so your existing notes stay intact. Shared paths such as `AGENTS.md` now use one block per agent ID (for example `codex` and `opencode`) instead of collapsing to a single shared block. Re-running the same command is idempotent.

Scaffold a built-in project template with shared instructions and six role skills:

```bash
agentcom init --template company
agentcom init --template oh-my-opencode
agentcom init --template custom
agentcom init --template custom --advanced
```

The generated `COMMON.md`, shared `agentcom/SKILL.md`, and namespaced role skills all describe the default template workflow as `init -> up -> down`. Any remaining `register` guidance is framed as the low-level path for a standalone manually managed agent.

For `--template custom`, the default wizard now asks only for the template name and a comma-separated role list. Use `--advanced` to open the original fully detailed role-by-role wizard.

Re-running template scaffold generation now updates marker-managed `SKILL.md` content in place and skips regenerating unchanged scaffold files such as `COMMON.md` and `template.json`. Generated role skills now include validated communication graphs, detailed primary contacts, and concrete request/response/escalation/report command examples.

If you want to create a custom template without the interactive wizard, import it from a YAML or JSON file:

```bash
agentcom init --batch --from-file template.yaml
agentcom --json init --batch --project myapp --from-file template.json
```

Imported templates are validated, saved under `.agentcom/templates/<name>/`, and then scaffolded as the active template for the current project.

Inspect the built-in templates before generating one:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
```

When run on an interactive terminal without a template name, `agentcom agents template` now prompts for a search term and lets you select a matching template by number.

You can also edit an existing custom template after creation:

```bash
agentcom agents template edit my-team add-role devops
agentcom agents template edit my-team remove-role design
```

Template edits update `.agentcom/templates/<name>/template.json`, keep the communication graph valid, and regenerate the affected role skills.

Start all template-defined agents:

```bash
agentcom up
agentcom up --only frontend,plan
agentcom --json up
```

The default `up` mode detaches immediately and writes runtime metadata to `.agentcom/run/up.json`. Use `agentcom down` to stop the session:

```bash
agentcom down
agentcom down --only plan
agentcom down --timeout 15
agentcom --json down
```

Inspect the managed agents:

```bash
agentcom list
agentcom health
```

Send a direct message between managed agents:

```bash
agentcom send --from frontend plan '{"text":"hello"}'
```

Send a message to the human operator and reply from the user inbox:

```bash
agentcom send --from plan user '{"text":"Should I proceed with the refactor?"}'
agentcom user inbox
agentcom user reply plan '{"text":"Yes, proceed"}'
```

Broadcast an update:

```bash
agentcom broadcast --from alpha --topic sync '{"status":"ready"}'
```

Create, delegate, and update a task:

```bash
agentcom task create "Implement MCP tests" --creator frontend --assign plan --priority high
agentcom task list --assignee plan
agentcom task delegate <task-id> --to plan
agentcom task update <task-id> --status in_progress --result "started"
```

Inspect inbox and system status:

```bash
agentcom inbox --agent plan --unread
agentcom status
```

Generate a project-level skill for all supported agent CLIs:

```bash
agentcom skill create review-pr --agent all --scope project --description "Review pull requests consistently"
```

Run diagnostics, skill validation, and dry-run previews:

```bash
agentcom doctor
agentcom skill validate
agentcom --json init --batch --dry-run --agents-md codex --template company
```

## Detailed command usage

### `agentcom init`

Initializes the home directory, ensures subdirectories exist, opens the SQLite database, and applies migrations.

Usage:

```bash
agentcom init
agentcom init --batch
agentcom init --batch --project myapp
agentcom init --agents-md
agentcom init --batch --agents-md claude,codex
agentcom init --template company
agentcom init --template oh-my-opencode
agentcom init --template custom
agentcom init --template custom --advanced
agentcom init --batch --from-file template.yaml
agentcom --json init
```

Notes:

- Running it repeatedly is safe.
- Re-running `init --agents-md ...` preserves existing user content and updates the agentcom-managed marker block in place.
- On an interactive terminal, `agentcom init` now runs the onboarding wizard by default.
- `--batch` forces the legacy non-interactive flow and is also implied by `--json`.
- `--project <name>` writes `.agentcom.json` in the current directory and scopes later commands to that project.
- `--force` overwrites every generated artifact for `init`, including `.agentcom.json`, instruction files, scaffold files, and generated skills.
- `--dry-run` previews every file that `init` would create, update, or overwrite without writing anything.
- `--accessible` switches the setup wizard to accessible text prompts.
- `--agents-md` now accepts `all` or a comma-separated agent list such as `claude,codex,cursor`; `agentcom init --batch --agents-md` without a value keeps the legacy `AGENTS.md` behavior.
- When multiple selected agents map to the same instruction path, `init --agents-md` keeps a separate marker block per agent so shared files can be updated independently later.
- `--template` writes `.agentcom/templates/<template>/COMMON.md`, `.agentcom/templates/<template>/template.json`, a shared `agentcom/SKILL.md` per supported agent, and six namespaced role skills: `agentcom/<template>-frontend`, `agentcom/<template>-backend`, `agentcom/<template>-plan`, `agentcom/<template>-review`, `agentcom/<template>-architect`, and `agentcom/<template>-design`.
- Re-running `--template` updates generated shared/role `SKILL.md` files idempotently while leaving existing `COMMON.md` and `template.json` in place.
- Generated scaffold instructions treat `agentcom init --template <name>` -> `agentcom up` -> `agentcom down` as the default team lifecycle, with `agentcom register` reserved for low-level standalone use.
- When `--template` is set, `.agentcom.json` also records `template.active` so `agentcom up` can start the same template later without repeating the flag.
- Supported built-in templates are `company` and `oh-my-opencode`; `custom` launches a template-creation wizard in interactive mode, with a quick 2-field flow by default and the original detailed flow behind `--advanced`.
- `--from-file <path>` imports a custom template definition from YAML or JSON, validates it, stores it under `.agentcom/templates/<name>/`, and scaffolds it as the current active template.
- `agentcom agents template --list` shows built-in and custom templates, and `agentcom agents template --delete <name>` removes a custom template after confirmation.
- `agentcom agents template edit <name> add-role <role>` and `remove-role <role>` update an existing custom template, refresh its communication graph, and regenerate the affected skill files.
- `agentcom agents template export <name>` exports the current template definition as YAML for sharing or roundtrip import.
- JSON output includes `path`, `status`, `project`, `project_config_path`, `template`, `active_template`, `instruction_files`, `agents_md`, `memory_files`, `custom_template_path`, and `generated_files` when applicable.
- `--dry-run --json` also includes `dry_run: true` and a `preview` array of `{action,path}` entries.
- Because the current implementation prepares the home directory before `init` checks it, `status` may appear as `already_initialized` even for a newly prepared path.

### `agentcom up`

Starts the active template for the current project. By default, `up` detaches immediately, starts a background supervisor, and launches one `register` child process per template role.

Usage:

```bash
agentcom up
agentcom up --template company
agentcom up --only frontend,plan
agentcom up --force
agentcom --json up
```

Flags:

- `--template <name>` - override `.agentcom.json` `template.active` and persist the new active template
- `--only <roles>` - start only the listed role names from the template
- `--force` - stop an existing managed session before starting a new one

Notes:

- On an interactive terminal, if the project is not initialized yet, `up` runs `agentcom init` first.
- On a non-interactive terminal, `up` fails fast if `.agentcom.json` is missing.
- Runtime metadata is written to `.agentcom/run/up.json`.
- `agentcom up` is the recommended high-level way to run template-based teams.

### `agentcom down`

Stops agents started by `agentcom up`.

Usage:

```bash
agentcom down
agentcom down --only plan
agentcom down --timeout 15
agentcom down --force
agentcom --json down
```

Flags:

- `--only <roles>` - stop only the listed role names
- `--timeout <seconds>` - wait for graceful shutdown before giving up
- `--force` - skip graceful shutdown and kill the managed processes immediately

Notes:

- `down` reads `.agentcom/run/up.json` as its primary runtime source.
- If all managed roles are stopped, the runtime state file is removed.

### `agentcom register`

Registers the current process as a live agent, starts heartbeat updates, opens a Unix Domain Socket server, and starts a fallback poller. This command is intentionally long-running: it stays alive until interrupted.

Use this as the low-level or advanced interface when you want to run one standalone agent manually instead of starting a whole template with `agentcom up`.

Usage:

```bash
agentcom register --name alpha --type codex
agentcom register --name alpha --type codex --cap plan,execute --workdir /path/to/project
agentcom --json register --name alpha --type codex
```

Flags:

- `--name` - required unique agent name
- `--type` - required free-form type string
- `--cap` - optional comma-separated capability list
- `--workdir` - optional working directory; defaults to current working directory

Project behavior:

- Agent names are unique per project, not globally.
- The same agent name can be reused in different projects.
- Use `--all-projects` to inspect all projects from one shell.

Notes:

- The process auto-deregisters on `SIGINT`/`SIGTERM`.
- `--json` prints registration metadata first, then the process stays running.
- A registered agent gets an `agt_...` ID and a socket path under the configured sockets directory.

### `agentcom deregister`

Removes an agent by name or ID.

Usage:

```bash
agentcom deregister alpha
agentcom deregister agt_xxx --force
agentcom --json deregister alpha --force
```

Flags:

- `--force` - skip the interactive confirmation prompt

Notes:

- By default it prompts before deletion.
- Agent history in messages and tasks remains stored even after deregistration.

### `agentcom list`

Lists registered agents.

Usage:

```bash
agentcom list
agentcom list --alive
agentcom --all-projects list
agentcom --json list
```

Flags:

- `--alive` - only show agents currently marked `alive`

### `agentcom send`

Sends a direct message from one registered sender to one target agent.

Usage:

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom send --from alpha beta 'plain text message'
agentcom send --from alpha --type request --topic review beta '{"file":"README.md"}'
agentcom send --from alpha --correlation-id corr-123 beta '{"step":1}'
agentcom --json send --from alpha beta '{"text":"hello"}'
```

Flags:

- `--from` - required sender agent name
- `--type` - optional message type, default `notification`
- `--topic` - optional topic string
- `--correlation-id` - optional correlation ID

Payload behavior:

- If the second argument is valid JSON object/array text, it is stored as-is.
- Otherwise the command wraps it as `{"text":"..."}`.

### `agentcom broadcast`

Sends one message to all alive agents except the sender.

Usage:

```bash
agentcom broadcast --from alpha '{"status":"ready"}'
agentcom broadcast --from alpha --topic sync '{"phase":"wave-9"}'
agentcom --json broadcast --from alpha '{"status":"ready"}'
```

Flags:

- `--from` - required sender agent name
- `--topic` - optional topic string

### `agentcom inbox`

Reads messages for one agent inbox from SQLite.

Usage:

```bash
agentcom inbox --agent beta
agentcom inbox --agent beta --unread
agentcom inbox --agent agt_xxx --from agt_sender_id
agentcom --json inbox --agent beta --unread
```

Flags:

- `--agent` - required agent name or ID
- `--unread` - only show unread messages
- `--from` - filter by sender agent ID

### `agentcom task`

Task management is split into four subcommands.

Create a task:

```bash
agentcom task create "Implement docs" --desc "Expand README" --creator alpha --assign beta --priority high
agentcom task create "Ship release" --blocked-by P7-01,P7-02
agentcom --json task create "Implement docs" --creator alpha
```

List tasks:

```bash
agentcom task list
agentcom task list --status pending
agentcom task list --assignee beta
agentcom --json task list --status in_progress
```

Update a task:

```bash
agentcom task update <task-id> --status in_progress --result "started"
agentcom task update <task-id> --status completed --result "done"
```

Delegate a task:

```bash
agentcom task delegate <task-id> --to beta
agentcom --json task delegate <task-id> --to agt_xxx
```

Important details:

- `task create` stores new tasks with status `pending`.
- `--assign` and `--creator` accept agent name or ID.
- `task list --assignee` tries to resolve names to IDs before querying.
- `task update` writes status/result directly.
- `task delegate` updates `assigned_to` to the resolved target agent.

### `agentcom skill`

Creates skill files for supported coding-agent CLIs.

Usage:

```bash
agentcom skill create review-pr --agent claude --scope project --description "Review pull requests consistently"
agentcom skill create pairing-notes --agent cursor --scope project
agentcom skill create docs-sync --agent github-copilot --scope user
agentcom skill create release-check --agent all --scope user
agentcom --json skill create docs-sync --agent gemini-cli --scope project
agentcom skill validate
agentcom --json skill validate
```

Flags:

- `--agent` - target agent identifier or `all` (default `all`)
- `--scope` - output scope: `project|user` (default `project`)
- `--description` - optional skill description; defaults to `Skill generated by agentcom`

Generated paths:

- `claude` - project: `.claude/skills/<name>/SKILL.md`, user: `~/.claude/skills/<name>/SKILL.md`
- `codex` - project: `.agents/skills/<name>/SKILL.md`, user: `~/.agents/skills/<name>/SKILL.md`
- `gemini` - project: `.gemini/skills/<name>/SKILL.md`, user: `~/.gemini/skills/<name>/SKILL.md`
- `opencode` - project: `.opencode/skills/<name>/SKILL.md`, user: `~/.config/opencode/skills/<name>/SKILL.md`
- `cursor` - project: `.cursor/skills/<name>.mdc`, user: `~/.cursor/skills/<name>.mdc`
- `github-copilot` - project: `.github/skills/<name>.md`, user: `~/.github/skills/<name>.md`
- `universal` - project: `skills/<name>/SKILL.md`, user: `~/skills/<name>/SKILL.md`

Additional supported agent identifiers include aliases such as `claude-code`, `gemini-cli`, and `droid`, plus catalog-backed targets like `cline`, `continue`, `roo-code`, `kilo-code`, `windsurf`, `devin`, `replit-agent`, `bolt`, `lovable`, `tabnine`, `tabby`, `amazon-q`, `sourcegraph-cody`, `augment-code`, `factory`, `goose`, `openhands`, `qwen`, and others.

Notes:

- Skill names must use lowercase letters, numbers, and single hyphens only.
- Existing skill files are never overwritten.
- `--agent all` writes one file per supported agent and stops on the first write failure.
- Output format depends on the target agent: most use `SKILL.md`, `cursor` uses `.mdc`, and Markdown-based agents such as `github-copilot`, `windsurf`, `devin`, `replit-agent`, `bolt`, `lovable`, and `playcode-agent` use `.md`.
- `skill validate` checks generated `SKILL.md` files for minimum length, required sections, placeholder leakage, and `agentcom` CLI examples.

### `agentcom agents template`

Lists built-in templates or shows one template definition in detail.

Usage:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
agentcom agents template export company > company.yaml
agentcom agents template edit my-team add-role devops
agentcom agents template edit my-team remove-role design
```

Interactive behavior:

- On an interactive terminal with no template name, the command prompts for a search string and then a numbered selection.
- In non-interactive or `--json` mode, it keeps the existing list/detail output behavior.

Built-in templates:

- `company` - company-style multi-agent workflow inspired by Paperclip role structure
- `oh-my-opencode` - planning-heavy workflow inspired by Prometheus, Momus, Oracle, and Sisyphus-Junior patterns, with `plan` acting as the coordination hub

Generated scaffold details:

- Common instructions live at `.agentcom/templates/<template>/COMMON.md`.
- Template metadata lives at `.agentcom/templates/<template>/template.json`.
- Project-level shared template skills are generated at `.claude/skills/agentcom/SKILL.md`, `.agents/skills/agentcom/SKILL.md`, `.gemini/skills/agentcom/SKILL.md`, and `.opencode/skills/agentcom/SKILL.md`.
- Role skills are generated under the same namespace, for example `.agents/skills/agentcom/company-frontend/SKILL.md`.
- Each generated role skill reads the shared `../SKILL.md` first, then the template `COMMON.md`, and includes role-specific workflow, examples, anti-patterns, handoff guidance, and the communication map.
- Generated scaffold wording is aligned so the default onboarding path is `init -> up -> down`; `register` only appears as an advanced/manual standalone workflow.
- `agentcom agents template edit` is only available for custom templates and supports `add-role` and `remove-role` operations.
- `agentcom init --from-file <path>` is the non-interactive path for importing a custom template definition from YAML or JSON.
- `agentcom agents template export <name>` writes a YAML template that can be re-imported with `agentcom init --from-file`.

### `agentcom status`

Shows project name, active template, managed role status (when `.agentcom/run/up.json` exists), aggregate counts for agents/messages/tasks, and unread messages grouped by recipient agent.

Usage:

```bash
agentcom status
agentcom --json status
```

JSON output includes `template`, `role_status`, and `unread_by_agent`.

### `agentcom health`

Runs a health-oriented view over registered agents.

Usage:

```bash
agentcom health
agentcom --json health
```

Use this when you want a quick check of live agent state instead of full task/message counts.

JSON output is an array of health entries. In an empty environment it returns `[]`.

### `agentcom doctor`

Runs a comprehensive setup diagnostic across the environment, project config, communication graph, generated documentation, and managed runtime state.

Usage:

```bash
agentcom doctor
agentcom --json doctor
```

Doctor reports each check as `pass`, `warn`, or `fail`, and includes actionable fix commands for failures.

### `agentcom version`

Prints build metadata.

Usage:

```bash
agentcom version
agentcom --json version
```

### `agentcom mcp-server`

Starts the MCP JSON-RPC server over STDIO.

Usage:

```bash
agentcom mcp-server
agentcom mcp-server --register mcp-agent --type mcp
```

Flags:

- `--register <name>` - optionally register the MCP server as an agent
- `--type <type>` - agent type used with `--register`

Notes:

- `initialize` must happen before `tools/list` or `tools/call`.
- The server exposes tools for listing agents, sending messages, broadcasting, creating/delegating tasks, listing tasks, and reading status.

## JSON output examples

Initialize:

```bash
agentcom --json init
```

Example shape:

```json
{
  "active_template": "company",
  "path": "/Users/name/.agentcom",
  "status": "initialized or already_initialized"
}
```

Managed session start:

```bash
agentcom --json up
```

Example shape:

```json
{
  "status": "started",
  "project": "myapp",
  "template": "company",
  "supervisor_pid": 12345,
  "runtime_state": "/path/to/project/.agentcom/run/up.json"
}
```

List agents:

```bash
agentcom --json list
```

Send message:

```bash
agentcom --json send --from alpha beta '{"text":"hello"}'
```

Task list:

```bash
agentcom --json task list --status pending
```

## End-to-end example workflow

Project terminal:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init --project demo --template company
agentcom up --only frontend,plan
```

Work terminal:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom send --from frontend plan '{"text":"please review README"}'
agentcom inbox --agent plan --unread
agentcom task create "Review README" --creator frontend --assign plan --priority high
agentcom task list --assignee plan
agentcom status
agentcom health
agentcom down
```

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
- `send_to_user`
- `get_user_messages`
- `broadcast`
- `create_task`
- `delegate_task`
- `list_tasks`
- `get_status`

## Human Operator Communication

`agentcom` now treats the human operator as a per-project pseudo-agent named `user`.
`agentcom up` registers that pseudo-agent automatically, direct sends to `user` work through the existing message path, and `agentcom down` cleans the pseudo-agent up with the rest of the managed session.

Human-oriented CLI helpers:

```bash
agentcom user inbox
agentcom user pending
agentcom user reply plan '{"text":"Approved"}'
```

MCP clients can use:

```text
send_to_user(from="plan", text="Should I proceed?", topic="approval")
get_user_messages(agent="plan", unread_only=true)
```

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
