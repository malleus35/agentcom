# P12 PRD — User Endpoint: Human-in-the-Loop Communication

> **Status**: Planning (implementation not started)
> **Dependencies**: P11 up/down lifecycle (completed), P10 project column (completed)
> **Purpose**: Enable human operators to exchange messages with AI agents via CLI and MCP
> **Task ID prefix**: P12
> **Estimated effort**: 10 tasks / 32 subtasks / ~24 hours

---

## 1. Problem Definition

### Current State (as-is)

agentcom supports only agent-to-agent communication. All participants must be
registered process agents with PID, heartbeat, and UDS socket. There is no way
for a human operator to:

1. Receive messages from AI agents ("agent needs human approval")
2. Send responses back to requesting agents ("user approves the plan")
3. Monitor agent requests without watching raw `inbox` output
4. Be part of the communication graph as a first-class participant

### MEMORY.md Reference (line 266)

> "CEO 중심 라우팅 vs direct-to-user 응답 모델은 아직 계획 단계이며,
> 현 구현에는 특수 `user` recipient를 추가하지 않음"

This plan resolves that deferred decision.

### Desired State (to-be)

```bash
# Agent sends a question to the human operator
agentcom send --from plan user '{"text":"Should I proceed with the refactor?"}'

# OR via MCP tool (from AI agent session)
# send_to_user(from="plan", text="Should I proceed with the refactor?")

# Human checks their inbox
agentcom user inbox                    # convenience alias
agentcom inbox --agent user --unread   # also works (backward compatible)

# Human replies
agentcom user reply plan '{"text":"Yes, proceed"}'

# Agent reads the response
agentcom inbox --agent plan --from user --unread
```

---

## 2. Design Option Analysis

### 2.1 Option A: Pure Pseudo-Agent (atm/AutoGen Pattern)

**Mechanism:**
- `agentcom up` auto-registers `user` as a regular agent with `type="human"`,
  `socket_path=nil`, `pid=0`
- Agents send to `user` via existing `send --to user`
- Human reads via `inbox --agent user`, replies via `send --from user`

**Evidence from codebase (why this is feasible):**
- `db.Agent.SocketPath` is nullable (`TEXT` without NOT NULL) — `registry.go:58`
  stores socket path, but Router.Send (`router.go:78-83`) already handles
  `target.SocketPath == ""` by falling back to SQLite inbox
- `db.Agent.PID` is nullable (`INTEGER` without NOT NULL)
- `agents.type` is free-form string — no enum validation
- `Router.Send()` resolves targets by name, checks DB existence only, no liveness check
- `inbox` is purely DB-based, works for any registered agent regardless of alive/dead

**Pros:**
- Zero schema changes
- Zero routing logic changes
- Existing CLI commands (send, inbox) work unchanged
- MCP tools (send_message, broadcast) work unchanged

**Cons:**
- **Heartbeat dead marking**: `MarkInactive()` in `registry.go:136-159` checks
  `processAlive(a.PID)` — with PID=0, `kill(0, 0)` signals the calling process
  group, returning true on Unix. This is **unreliable behavior** depending on OS.
  With the agent's actual PID stored as 0, MarkInactive may or may not mark user
  as dead.
- **Broadcast spam**: `Router.Broadcast()` calls `ListAlive()` which returns all
  alive agents — user would receive every broadcast, cluttering the human inbox
- **UX confusion**: Commands like `inbox --agent user` and `send --from user` are
  agent-oriented, not human-oriented
- **No special affordances**: No notification mechanism, no reply threading

### 2.2 Option B: Special Recipient + Dedicated CLI (LangGraph Pattern)

**Mechanism:**
- Router gets special-case handling for `"user"` recipient
- New CLI commands: `agentcom notify`, `agentcom respond`
- MCP gets new tools: `notify_user`, `get_user_response`

**Pros:**
- Optimal human UX
- Clear intent separation (notify vs send, respond vs reply)
- Can add notification hooks (bell, OS notification)

**Cons:**
- **Router changes**: `Router.Send()` currently resolves target from DB only.
  Adding special-case string matching breaks the clean resolution pattern
- **New CLI surface**: Two entirely new commands with their own flag sets
- **MCP tool proliferation**: Adding tools that overlap with existing `send_message`
- **Not backward-compatible**: Existing `send --to user` wouldn't work until
  special routing is added
- **Breaks the "agents are just agents" design principle**: From AGENTS.md —
  "에이전트 유형 자유: type 필드는 자유 문자열. enum 제한 없음"

### 2.3 Option C: Hybrid (Internal Pseudo-Agent + External UX Layer) ✅ RECOMMENDED

**Mechanism:**
- **Internal layer**: User is a pseudo-agent (Option A) — zero infrastructure changes
- **External layer**: Convenience CLI subcommand `agentcom user` (inbox/reply/pending)
- **MCP layer**: `send_to_user` / `get_user_responses` tools (thin wrappers)
- **Guard layer**: Heartbeat exemption + broadcast opt-out for type="human"

**Pros:**
- Zero schema changes (pseudo-agent pattern)
- Zero routing changes (existing Send/Broadcast work)
- Human-optimized UX (dedicated `user` subcommand)
- MCP tools with clear intent (`send_to_user` instead of `send_message` with to="user")
- Backward compatible: `send --to user` and `inbox --agent user` still work
- Broadcast opt-out prevents inbox spam
- Heartbeat exemption prevents false dead marking

**Cons:**
- Slightly more code than pure Option A (convenience commands + guards)
- Two ways to do the same thing (user subcommand vs raw send/inbox)

**Why this is the right choice for agentcom:**
1. **Design principle preserved**: User IS an agent internally. No special routing.
2. **Minimal blast radius**: Only additive changes. No existing code paths modified
   for the happy path.
3. **Two guard changes are surgical**: heartbeat skip (1 condition) + broadcast
   filter (1 condition). Both are in hot paths that already have filtering logic.
4. **MCP ergonomics**: AI agents call `send_to_user` which is self-documenting.
   They don't need to know `user` is a pseudo-agent.
5. **Matches industry consensus**: AutoGen (pseudo-agent) + atm (CLI convenience)
   + LangGraph (async with checkpoints) patterns combined.

---

## 3. Hidden Requirements and Risks

### 3.1 Heartbeat Dead Marking (CRITICAL)

**Risk**: `MarkInactive()` in `registry.go:136-159` iterates all alive agents.
For each, it checks `processAlive(a.PID)`. User pseudo-agent has `PID=0`.
On Unix, `kill(0, 0)` sends signal 0 to the process group — this returns success
if any process in the group exists, meaning the user agent would appear "alive"
as long as the calling process is alive. But this is fragile and OS-dependent.

**Mitigation**: Add explicit type check: `if a.Type == "human" { continue }` in
`MarkInactive()`. This is a 1-line guard, but must be tested.

**Alternative mitigation**: Set user agent PID to the supervisor PID (from `up`).
This ties user liveness to the supervisor, which is correct semantics. But it
means `down` would deregister user too, requiring re-registration on next `up`.

**Decision for plan**: Use type-based skip. Simpler, more robust, decoupled from
supervisor lifecycle.

### 3.2 Broadcast Spam to User (HIGH)

**Risk**: `Router.Broadcast()` calls `ListAlive()` which returns ALL alive agents
including user. Every agent broadcast fills the human inbox.

**Evidence**: `router.go:109` — `r.finder.ListAlive(ctx, r.project)` with sender
exclusion only at line 116. No type-based filtering.

**Mitigation options**:
- **A. Filter in Broadcast()**: Skip agents where `Type == "human"` in broadcast
  loop. Simple but hardcodes the convention.
- **B. Broadcast opt-out flag**: Add `exclude_from_broadcast BOOLEAN DEFAULT 0`
  to agents table. More general but requires schema migration.
- **C. Agent capabilities filter**: Use existing `capabilities` JSON field to
  store `["no-broadcast"]`. No schema change, flexible.

**Decision for plan**: Option A for MVP (type-based filter in broadcast). Add a
`// TODO: generalize to capabilities-based opt-out` comment. Keeps MVP scope
minimal while leaving extension path clear.

### 3.3 Multi-Project User Identity (MEDIUM)

**Risk**: With project scoping (P10), should there be one `user` agent per project
or one global user?

**Analysis**:
- `UNIQUE(name, project)` constraint allows `user` to exist in multiple projects
- Each project has independent agent registrations via `agentcom up`
- `agentcom up` starts per-project — registering a per-project user is natural

**Decision**: One `user` pseudo-agent per project. Registered automatically by
`agentcom up`. This matches the existing per-project agent model.

### 3.4 `agentcom up` Auto-Registration (MEDIUM)

**Risk**: User pseudo-agent must be created when `up` starts. But `up` starts
agents from `template.json` roles. User is not a template role.

**Mechanism**: After all template roles are started in `runUpSupervisor()`, add
a user pseudo-agent registration step. This happens inside the supervisor, not
as a child process (since there's no process to manage).

**Key detail**: User agent should be recorded in `up.json` runtime state so
`down` can deregister it.

### 3.5 `agentcom down` Cleanup (MEDIUM)

**Risk**: When `down` runs, it signals child processes to terminate. User has no
process. `down` must explicitly deregister the user pseudo-agent from DB.

**Mechanism**: After stopping all role processes, call `Registry.Deregister()`
for the user agent. Read user agent ID from runtime state.

### 3.6 Existing CLI Backward Compatibility (LOW)

**Risk**: Adding `user` subcommand could shadow `agentcom user` if someone has
an agent named "user" already registered.

**Mitigation**: `agentcom user` is a new cobra subcommand, not a positional
argument. It cannot conflict with existing commands. The agent name `user` in
`send --to user` works via the existing name resolution path.

### 3.7 MCP Tool Naming (LOW)

**Risk**: Adding `send_to_user` alongside `send_message` could confuse AI agents
about which to use.

**Mitigation**: Clear tool descriptions. `send_to_user` description states
"Send a message to the human operator" while `send_message` says "Send a message
to a target agent." The `send_message` tool also works with `to="user"`.

---

## 4. Scope

### Must Have (MVP)
- User pseudo-agent auto-registered by `agentcom up`
- Heartbeat exemption for `type="human"` agents
- Broadcast exclusion for `type="human"` agents
- `agentcom user inbox` — view messages to user
- `agentcom user reply <agent> <payload>` — send response
- `agentcom user pending` — show messages awaiting response
- MCP `send_to_user` tool
- MCP `get_user_messages` tool
- `agentcom up`/`down` user agent lifecycle management

### Should Have (Post-MVP Enhancements, but tracked)
- `correlation_id` auto-generation in `send_to_user` MCP tool for request-response
  matching (agents can match responses to their specific questions)
- Warning or audit log when `send --from user` is used directly instead of
  `user reply` (prevents agent spoofing as human)

### Must NOT Have (Explicit Exclusions)
- WebSocket or HTTP notification server
- Terminal bell or OS notification integration
- Real-time polling/watch mode for user inbox
- Multiple human users per project (single `user` agent only)
- Schema changes to agents table
- Changes to `Router.Send()` core logic
- Changes to existing CLI command signatures
- Agent-side blocking/waiting for user response
- User authentication or authorization

### Future Considerations (Post-MVP)
- `agentcom user watch` — live-updating inbox view (bubbletea TUI)
- Notification hooks (terminal-notifier, webhook)
- Multiple named users per project
- Response timeout / deadline tracking
- User message priority / urgency levels

---

## 5. Task Decomposition

### Phase 12-1: Heartbeat & Broadcast Guards

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-01-01 | Heartbeat exemption for human agents | Add `a.Type == "human"` skip in `MarkInactive()` loop. When iterating alive agents, skip heartbeat staleness check for agents with `type="human"`. | — | `internal/agent/registry.go` |
| P12-01-02 | Heartbeat exemption tests | TDD: table-driven tests verifying human-type agent is never marked dead by MarkInactive(), even with stale heartbeat and PID=0. Also verify non-human agents are still marked dead normally. | P12-01-01 | `internal/agent/registry_test.go` |
| P12-01-03 | Broadcast exclusion for human agents | In `Router.Broadcast()`, skip agents with `Type == "human"` in the broadcast loop (after sender exclusion check). | — | `internal/message/router.go` |
| P12-01-04 | Broadcast exclusion tests | TDD: table-driven tests verifying broadcast does not deliver to human-type agents. Also verify human agents can still receive direct messages via Send(). | P12-01-03 | `internal/message/router_test.go` |
| P12-01-05 | MCP broadcast handler exclusion | In `handleBroadcast()`, add same type="human" filter to the alive agents loop. | P12-01-03 | `internal/mcp/handler.go` |
| P12-01-06 | MCP broadcast exclusion tests | TDD: MCP broadcast roundtrip test with human agent registered — verify 0 messages to human. | P12-01-05 | `internal/mcp/server_test.go` |

**Atomic Commit**: `feat(agent): P12-01 heartbeat and broadcast guards for human agents`

---

### Phase 12-2: User Pseudo-Agent Lifecycle in `up`/`down`

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-02-01 | Register user pseudo-agent in supervisor | After all role child processes start in `runUpSupervisor()`, call `Registry.Register()` with `name="user"`, `type="human"`, `pid=supervisorPID`, `socket_path=""`, `project=projectName`. Store agent ID in runtime state. | P12-01 | `internal/cli/up.go` |
| P12-02-02 | Extend runtime state struct | Add `UserAgent *upRuntimeStateAgent` field to `upRuntimeState` struct. Serialize/deserialize with `up.json`. | P12-02-01 | `internal/cli/up_state.go` |
| P12-02-03 | Deregister user agent on `down` | In `stopRuntimeState()`, after stopping all role processes, deregister user agent by ID from runtime state. If `--only` is used for specific roles, do NOT deregister user. Only deregister on full `down`. | P12-02-02 | `internal/cli/up.go` |
| P12-02-04 | Handle duplicate user registration | If `up` is called and a `user` agent already exists for the project (from a previous crashed session), deregister the stale one first before re-registering. Use `FindByName` + delete if exists. | P12-02-01 | `internal/cli/up.go` |
| P12-02-05 | Up/down lifecycle tests | TDD: Test that `runUpSupervisor` registers user agent. Test that full `down` deregisters user. Test that `down --only plan` does NOT deregister user. Test duplicate user cleanup. | P12-02-01~04 | `internal/cli/up_test.go` |

**Atomic Commit**: `feat(cli): P12-02 user pseudo-agent lifecycle in up/down`

---

### Phase 12-3: `agentcom user` CLI Subcommand

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-03-01 | Create `user` cobra command group | New `internal/cli/user.go` file. Root subcommand `agentcom user` with subcommands: `inbox`, `reply`, `pending`. Register in `root.go`. | — | `internal/cli/user.go`, `internal/cli/root.go` |
| P12-03-02 | `user inbox` subcommand | List messages addressed to the `user` agent in current project. Flags: `--unread` (default true), `--from <agent>`, `--json`. Internally calls `db.ListMessagesForAgent()` with resolved user agent ID. Display: table format with from_agent name, topic, payload preview, timestamp, read status. | P12-03-01 | `internal/cli/user.go` |
| P12-03-03 | `user reply` subcommand | `agentcom user reply <target-agent> '<payload>'`. Resolves `user` agent as sender and target agent as recipient. Internally calls `Router.Send()` with from=user_id, to=target, type="response". Payload JSON normalization (same as `send` command). | P12-03-01 | `internal/cli/user.go` |
| P12-03-04 | `user pending` subcommand | List unread messages to user that are of type `request` — messages explicitly asking for human input. Flags: `--json`. This is a filtered view of inbox showing only actionable items. | P12-03-01 | `internal/cli/user.go` |
| P12-03-05 | User agent resolution helper | Shared helper `resolveUserAgent(ctx, db, project) (*db.Agent, error)` that finds the user pseudo-agent for the current project. Returns clear error if user agent not found ("no user agent registered; run `agentcom up` first"). | P12-03-01 | `internal/cli/user.go` |
| P12-03-06 | `user` CLI tests | TDD: Test inbox with unread/read messages, reply message creation, pending filter, JSON output, error when no user agent exists. Use in-memory SQLite. | P12-03-01~05 | `internal/cli/user_test.go` |

**Atomic Commit**: `feat(cli): P12-03 agentcom user subcommand (inbox/reply/pending)`

---

### Phase 12-4: MCP Tools for User Communication

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-04-01 | `send_to_user` MCP tool definition | Add tool to `AllTools()`: name="send_to_user", params={from: string (required), text: string (required), topic: string, priority: string, project: string}. Description: "Send a message to the human operator and store it in their inbox." | — | `internal/mcp/tools.go` |
| P12-04-02 | `send_to_user` handler | Resolve sender agent. Find `user` agent by name+project. Create message with `type="request"`, payload=`{"text": text, "priority": priority}`. Insert via `db.InsertMessage()`. Return `{message_id, status: "delivered_to_inbox"}`. | P12-04-01 | `internal/mcp/handler.go` |
| P12-04-03 | `get_user_messages` MCP tool definition | Add tool: name="get_user_messages", params={agent: string (optional, filter responses from specific agent), unread_only: bool (default true), project: string}. Description: "Read messages from the human operator (responses to agent requests)." | — | `internal/mcp/tools.go` |
| P12-04-04 | `get_user_messages` handler | Find `user` agent by name+project. Query messages where `from_agent=user_id`. Optional filter by `to_agent` if `agent` param provided. Optional `unread_only` filter. Return message list with read status. Mark returned messages as read. | P12-04-03 | `internal/mcp/handler.go` |
| P12-04-05 | MCP tool registration | Register both handlers in `registerTools()`. | P12-04-02, P12-04-04 | `internal/mcp/handler.go` |
| P12-04-06 | MCP user tools tests | TDD: Full JSON-RPC roundtrip tests. Test send_to_user creates message in user inbox. Test get_user_messages returns user-sent messages. Test unread_only filter. Test error when user agent not registered. | P12-04-01~05 | `internal/mcp/server_test.go` |

**Atomic Commit**: `feat(mcp): P12-04 send_to_user and get_user_messages MCP tools`

---

### Phase 12-5: DB Query Helpers

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-05-01 | `ListMessagesFromAgent` DB method | New query: `SELECT * FROM messages WHERE from_agent = ? ORDER BY created_at DESC`. Used by `get_user_messages` MCP tool and potentially `user pending`. | — | `internal/db/message.go` |
| P12-05-02 | `ListUnreadRequestsForAgent` DB method | New query: `SELECT * FROM messages WHERE to_agent = ? AND type = 'request' AND read_at IS NULL ORDER BY created_at DESC`. Used by `user pending` to show actionable items. | — | `internal/db/message.go` |
| P12-05-03 | `FindAgentByTypeAndProject` DB method | New query: `SELECT ... FROM agents WHERE type = ? AND project = ? LIMIT 1`. Used to find user agent without hardcoding name. Provides fallback resolution. | — | `internal/db/agent.go` |
| P12-05-04 | DB query tests | TDD: Table-driven tests for all three new query methods. Test empty results, multiple results, ordering, filter accuracy. In-memory SQLite. | P12-05-01~03 | `internal/db/message_test.go`, `internal/db/agent_test.go` |

**Atomic Commit**: `feat(db): P12-05 query helpers for user message flows`

---

### Phase 12-6: Integration & Documentation

| ID | Task | Description | Dependencies | Files |
|----|------|-------------|--------------|-------|
| P12-06-01 | E2E scenario test | Register 2 agents + user. Agent sends request to user. User replies. Agent reads response. Verify full roundtrip via CLI commands. | P12-01~05 | `cmd/agentcom/e2e_test.go` |
| P12-06-02 | Up/down E2E test | `agentcom up` creates user agent. Send message to user. `agentcom down` cleans up user agent. Verify user agent exists during session, gone after. | P12-02, P12-03 | `cmd/agentcom/e2e_test.go` |
| P12-06-03 | MCP E2E test | Full JSON-RPC roundtrip: initialize → send_to_user → get_user_messages. Verify tool list includes new tools. | P12-04 | `internal/mcp/server_test.go` |
| P12-06-04 | README documentation | Add "Human Operator Communication" section to README.md. Document `agentcom user` subcommands, MCP tools, and E2E example workflow. | P12-01~05 | `README.md` |
| P12-06-05 | MEMORY.md update | Record design decision: Option C hybrid chosen. Update user-endpoint status. Record task completion. | P12-06-01~04 | `.agents/MEMORY.md` |

**Atomic Commit**: `docs: P12-06 user endpoint E2E tests and documentation`

---

## 6. Dependency Graph

```
P12-05 (DB queries)  ─────────────────────────┐
                                                │
P12-01 (heartbeat/broadcast guards) ──────┐    │
                                           │    │
P12-02 (up/down lifecycle) ───────────┐   │    │
                                       │   │    │
P12-03 (CLI subcommand) ─────────┐    │   │    │
                                  │    │   │    │
P12-04 (MCP tools) ──────────┐  │    │   │    │
                               │  │    │   │    │
                               v  v    v   v    v
                          P12-06 (integration & docs)
```

**Parallelization for ultrawork**:
- **Wave 1** (fully parallel): P12-01, P12-05
- **Wave 2** (depends on P12-01): P12-02
- **Wave 3** (parallel, depends on P12-02 + P12-05): P12-03, P12-04
- **Wave 4** (depends on all): P12-06

---

## 7. Test Strategy (TDD-Oriented)

### Test-First Approach per Phase

Each phase follows this cycle:
1. Write failing test (Red)
2. Implement minimum code to pass (Green)
3. Refactor if needed

### Test Categories

| Category | Tool | Location |
|----------|------|----------|
| Unit: heartbeat guard | `go test ./internal/agent/...` | `registry_test.go` |
| Unit: broadcast guard | `go test ./internal/message/...` | `router_test.go` |
| Unit: DB queries | `go test ./internal/db/...` | `message_test.go`, `agent_test.go` |
| Unit: CLI user cmd | `go test ./internal/cli/...` | `user_test.go` |
| Unit: MCP tools | `go test ./internal/mcp/...` | `server_test.go` |
| Integration: up/down | `go test ./internal/cli/...` | `up_test.go` |
| E2E: full roundtrip | `go test ./cmd/agentcom/...` | `e2e_test.go` |

### Test Fixtures

All tests use **in-memory SQLite** (`":memory:"`) with full migration applied.
Agent fixtures include:
- `alpha` (type="codex", alive, with socket)
- `beta` (type="claude", alive, with socket)
- `user` (type="human", alive, socket_path="", pid=0)

### Verification Commands

```bash
# Per-phase verification
go test ./internal/agent/... -run TestMarkInactive -count=1 -v
go test ./internal/message/... -run TestBroadcast -count=1 -v
go test ./internal/db/... -run TestListMessagesFromAgent -count=1 -v
go test ./internal/cli/... -run TestUser -count=1 -v
go test ./internal/mcp/... -run TestSendToUser -count=1 -v

# Full suite
go test ./... -count=1
go build ./...
```

---

## 8. Atomic Commit Strategy

Each phase produces exactly ONE commit. This enables:
- Clean `git bisect` if regressions appear
- Independent code review per concern
- Ultrawork agents working on different phases in parallel worktrees

| Order | Commit Message | Branch |
|-------|---------------|--------|
| 1 | `feat(db): P12-05 query helpers for user message flows` | `feature/P12-user-endpoint` |
| 2 | `feat(agent): P12-01 heartbeat and broadcast guards for human agents` | same |
| 3 | `feat(cli): P12-02 user pseudo-agent lifecycle in up/down` | same |
| 4 | `feat(cli): P12-03 agentcom user subcommand (inbox/reply/pending)` | same |
| 5 | `feat(mcp): P12-04 send_to_user and get_user_messages MCP tools` | same |
| 6 | `docs: P12-06 user endpoint E2E tests and documentation` | same |

**Branch strategy**: Single feature branch `feature/P12-user-endpoint` from `develop`.
Merge to `develop` with `--no-ff` after all 6 commits pass CI.

---

## 9. AI Implementation Failure Points

### 9.1 MarkInactive Guard Placement (HIGH RISK)

**What will go wrong**: AI agent may add the type check AFTER the heartbeat
staleness check instead of BEFORE, causing the human agent to still enter the
`processAlive()` code path before being skipped.

**Correct placement** (line ~143 in registry.go):
```go
for _, a := range agents {
    if a.Type == "human" {  // ← MUST be first check
        continue
    }
    if now.Sub(a.LastHeartbeat) <= heartbeatStaleThreshold {
        continue
    }
    // ...
}
```

**Guard**: Test must verify that human agent with PID=0 and stale heartbeat is
NOT marked dead. The test naturally catches wrong placement.

### 9.2 Broadcast Filter in TWO Places (HIGH RISK)

**What will go wrong**: AI agent will add broadcast exclusion in `Router.Broadcast()`
but forget the SEPARATE implementation in `handleBroadcast()` (MCP handler). The
MCP handler has its OWN broadcast loop that does NOT call `Router.Broadcast()`.

**Evidence**: `handler.go:145-163` has an independent loop over alive agents.
`router.go:115-127` has a different loop. Both need the filter.

**Guard**: Separate tests for both paths. MCP test must create a human agent and
verify 0 broadcast messages to it.

### 9.3 Runtime State Serialization (MEDIUM RISK)

**What will go wrong**: AI agent may add `UserAgent` field to `upRuntimeState`
struct but forget to handle backward compatibility with existing `up.json` files
that don't have the field. Old `up.json` + new binary = potential crash.

**Guard**: Use pointer type `*upRuntimeStateAgent` (nil when absent). JSON
`omitempty` tag. Test with old-format JSON input.

### 9.4 User Agent Registration Race (MEDIUM RISK)

**What will go wrong**: In `runUpSupervisor()`, AI agent may register user agent
BEFORE writing runtime state, but if a role child process fails and triggers
cleanup, the user agent is leaked (registered in DB but not tracked for cleanup).

**Correct order**:
1. Start all role child processes
2. Register user agent
3. Write runtime state (including user agent ID)
4. On any failure → cleanup includes user deregistration

**Guard**: Test that failed supervisor startup does not leave orphan user agent.

### 9.5 `user reply` Sender Resolution (MEDIUM RISK)

**What will go wrong**: AI agent may hardcode `from="user"` string in the reply
command instead of resolving the actual user agent ID from DB. This breaks if the
user agent was re-registered (different ID) or if project scoping is involved.

**Correct approach**: Always resolve via `db.FindAgentByNameAndProject(ctx, "user", project)`.

**Guard**: Test reply with project scoping.

### 9.6 MCP `send_to_user` Error When No User Agent (LOW RISK)

**What will go wrong**: AI agent may return a generic "agent not found" error
instead of a helpful message like "no user agent registered; start a session
with `agentcom up` first".

**Guard**: Test error message content, not just error existence.

### 9.7 Import Cycle Risk (LOW RISK)

**What will go wrong**: `internal/cli/user.go` may import `internal/agent` or
`internal/message` directly, creating import cycles if those packages already
import `cli` (they don't currently, but agent sub-packages might in future).

**Guard**: Use the same DB-direct approach as existing CLI commands (all CLI
commands go through `db.DB` directly, not through domain packages).

---

## 10. Oracle Cross-Validation Notes

The plan was independently reviewed by Oracle (GPT-5.4). Key findings:

### Agreed Points (confirmed by both Metis and Oracle)
- Heartbeat dead marking with PID=0 is CRITICAL risk
- Broadcast filter needed in TWO independent code paths
- Per-project user agent is correct scoping
- Option C is viable (Oracle scored it highest on UX: 9/10)

### Additional Risks Identified by Oracle (incorporated into plan)
1. **`--from user` spoofing**: Any process can currently send `--from user` via CLI,
   impersonating the human. Mitigation: Add "Should Have" item for audit warning.
2. **correlation_id not enforced**: Parallel agent questions without correlation_id
   cause response mismatch. Mitigation: `send_to_user` MCP tool auto-generates
   correlation_id; documented as best practice.
3. **Read status ambiguity**: `messages.read_at` exists but "human read" vs "agent
   processed" semantics need clarification. Decision: `user inbox` marks as read
   on display (matching existing `inbox` behavior). `user pending` shows only
   `read_at IS NULL` messages.
4. **MCP `list_agents` confusion**: Oracle warns LLMs may `delegate_task(to=user)`.
   Mitigation: `send_to_user` tool description explicitly states "use this tool
   instead of send_message when communicating with the human operator".

### Oracle's Option B Recommendation — Why We Diverge
Oracle recommended Option B (special recipient + dedicated CLI) for better
long-term extensibility (Slack/Discord channels). We chose Option C because:
1. agentcom's design principle explicitly avoids enum/type restrictions on agents
2. Schema migration is against project philosophy ("SQLite가 유일한 진실의 원천")
3. The pseudo-agent pattern with guards achieves the same isolation with less code
4. Future channel bridges can still work: a Slack bridge registers as its own agent
   that forwards messages between `user` inbox and Slack API

---

## 11. Design Decisions Log (formerly Section 10)

| Decision | Rationale |
|----------|-----------|
| Option C (Hybrid) chosen over A and B | Combines zero-infrastructure changes (A) with human-optimized UX (B). Guards are surgical (2 one-line conditions). |
| One `user` agent per project | Matches existing per-project agent model from P10. UNIQUE(name, project) naturally supports this. |
| `type="human"` convention for guard checks | Free-form type field already exists. No enum. Convention-based, not schema-enforced. |
| Broadcast exclusion via type check, not capabilities | MVP simplicity. Capabilities-based opt-out is a clean future extension. |
| User agent registered by supervisor, not as child process | No process to manage. Direct DB insert in supervisor. Cleanup on `down`. |
| `agentcom user` subcommand, not top-level commands | Groups human-specific commands under one namespace. Doesn't pollute top-level. |
| MCP tools named `send_to_user`/`get_user_messages` | Self-documenting for AI agents. Clearly distinct from generic `send_message`. |
| PID=supervisor PID for user agent | Provides meaningful PID for any diagnostic tools while making it clear the user agent is tied to the managed session. |

---

## 12. Acceptance Criteria

### P12-01: Guards
```bash
go test ./internal/agent/... -run TestMarkInactiveSkipsHuman -count=1 -v
# PASS: human agent with PID=0 and stale heartbeat stays alive

go test ./internal/message/... -run TestBroadcastExcludesHuman -count=1 -v
# PASS: broadcast to 3 agents (1 human) delivers to 2 non-human only

go test ./internal/mcp/... -run TestMCPBroadcastExcludesHuman -count=1 -v
# PASS: MCP broadcast skips human agent
```

### P12-02: Lifecycle
```bash
go test ./internal/cli/... -run TestUpRegistersUserAgent -count=1 -v
# PASS: runtime state includes UserAgent field after up

go test ./internal/cli/... -run TestDownDeregistersUserAgent -count=1 -v
# PASS: user agent removed from DB after full down

go test ./internal/cli/... -run TestDownOnlyDoesNotDeregisterUser -count=1 -v
# PASS: down --only plan leaves user agent in DB
```

### P12-03: CLI
```bash
go test ./internal/cli/... -run TestUserInbox -count=1 -v
# PASS: shows messages addressed to user, respects --unread and --from filters

go test ./internal/cli/... -run TestUserReply -count=1 -v
# PASS: creates message from user to target agent with type=response

go test ./internal/cli/... -run TestUserPending -count=1 -v
# PASS: shows only type=request unread messages

go test ./internal/cli/... -run TestUserNoAgent -count=1 -v
# PASS: clear error "no user agent registered; run agentcom up first"
```

### P12-04: MCP
```bash
go test ./internal/mcp/... -run TestSendToUser -count=1 -v
# PASS: message created in user inbox with type=request

go test ./internal/mcp/... -run TestGetUserMessages -count=1 -v
# PASS: returns messages sent by user, supports unread_only filter
```

### P12-05: DB
```bash
go test ./internal/db/... -run TestListMessagesFromAgent -count=1 -v
# PASS: correct ordering and filtering

go test ./internal/db/... -run TestListUnreadRequestsForAgent -count=1 -v
# PASS: returns only unread request-type messages

go test ./internal/db/... -run TestFindAgentByTypeAndProject -count=1 -v
# PASS: finds agent by type within project scope
```

### P12-06: Integration
```bash
go test ./cmd/agentcom/... -run TestUserEndpointE2E -count=1 -v
# PASS: full agent→user→agent roundtrip via CLI

go test ./... -count=1
# ALL PASS

go build ./...
# SUCCESS
```

---

## 13. File Change Summary

| File | Change Type | Phase |
|------|------------|-------|
| `internal/agent/registry.go` | Modify (1 guard condition) | P12-01 |
| `internal/agent/registry_test.go` | Modify (add test cases) | P12-01 |
| `internal/message/router.go` | Modify (1 guard condition) | P12-01 |
| `internal/message/router_test.go` | Modify (add test cases) | P12-01 |
| `internal/mcp/handler.go` | Modify (broadcast guard + 2 new handlers) | P12-01, P12-04 |
| `internal/mcp/tools.go` | Modify (2 new tool definitions) | P12-04 |
| `internal/mcp/server_test.go` | Modify (add test cases) | P12-01, P12-04, P12-06 |
| `internal/cli/up.go` | Modify (user registration in supervisor + deregistration in down) | P12-02 |
| `internal/cli/up_state.go` | Modify (UserAgent field in runtime state) | P12-02 |
| `internal/cli/up_test.go` | Modify (add lifecycle test cases) | P12-02 |
| **`internal/cli/user.go`** | **NEW** (user subcommand: inbox/reply/pending) | P12-03 |
| **`internal/cli/user_test.go`** | **NEW** (user CLI tests) | P12-03 |
| `internal/cli/root.go` | Modify (register user subcommand) | P12-03 |
| `internal/db/message.go` | Modify (2 new query methods) | P12-05 |
| `internal/db/agent.go` | Modify (1 new query method) | P12-05 |
| `internal/db/message_test.go` | Modify (add test cases) | P12-05 |
| `internal/db/agent_test.go` | Modify (add test cases) | P12-05 |
| `cmd/agentcom/e2e_test.go` | Modify (E2E scenarios) | P12-06 |
| `README.md` | Modify (documentation) | P12-06 |
| `.agents/MEMORY.md` | Modify (decision log) | P12-06 |

**New files**: 2 (`user.go`, `user_test.go`)
**Modified files**: 18
**Schema changes**: 0
