# PH8 MCP Surface Rebaseline Execution Plan

## Objective

- Finish the remaining CLI-first MCP parity gaps without widening PH8 beyond the tools explicitly called out in `NEW-NEXT-PHASE-PLAN.md`.
- Execute PH8 only after PH5 and PH6/PH7 foundations are stable enough that MCP handlers can be added without churn.

## Source Context

- `internal/mcp/tools.go` currently exposes 12 tools, but not `inbox`, `health`, `deregister`, `doctor`, `version`, or `user_reply`.
- Existing MCP patterns already exist for list/query/update style handlers in `internal/mcp/handler.go` and regression patterns in `internal/mcp/server_test.go`.
- The CLI already provides the missing PH8 commands, so PH8 should mirror those semantics rather than inventing new ones.

## Scope Rules

- In scope: PH8-01 through PH8-06 only.
- In scope: MCP tool definition, handler wiring, tests, docs, and manual QA for the missing parity surfaces.
- Out of scope: new MCP concepts not backed by existing CLI commands.

## Acceptance Criteria

1. `inbox`, `health`, `deregister`, `doctor`, `version`, and `user_reply` are available via MCP.
2. Each new MCP tool has input schema, handler wiring, and server regression coverage.
3. Tool semantics align with the existing CLI behavior rather than diverging.
4. `go test ./internal/mcp/... -count=1`, `go test ./... -count=1`, and `go build ./...` pass.
5. Manual QA exercises at least one request for each new MCP tool family.

## Task Breakdown

### Task 1 - Lock PH8 parity surface

- T1.1: Map each missing MCP tool to its existing CLI command and output contract.
- T1.2: Group PH8 work into atomic vertical slices: inbox/user reply, health/version, deregister/doctor.

### Task 2 - PH8-01 and PH8-06 user communication parity (TDD first)

- T2.1: Add failing MCP tests for `inbox` and `user_reply`.
- T2.2: Implement tool schemas and handlers.
- T2.3: Verify CLI-equivalent behavior through server tests and manual QA.

### Task 3 - PH8-02 and PH8-05 status/version parity (TDD first)

- T3.1: Add failing MCP tests for `health` and `version`.
- T3.2: Implement minimal schemas and handlers.
- T3.3: Confirm outputs are stable and documented.

### Task 4 - PH8-03 and PH8-04 lifecycle/support parity (TDD first)

- T4.1: Add failing MCP tests for `deregister` and `doctor`.
- T4.2: Implement minimal schemas and handlers.
- T4.3: Re-run full MCP regression coverage.

### Task 5 - Verification, docs, merge

- T5.1: Run `lsp_diagnostics` on modified MCP files.
- T5.2: Run targeted MCP tests, then full test/build.
- T5.3: Manual QA via real `mcp-server` round trips for each tool family.
- T5.4: Update README/MEMORY/plan docs and merge PH8 back to `develop`.

## Verification Commands

```bash
go test ./internal/mcp/... -count=1
go test ./... -count=1
go build ./...
```
