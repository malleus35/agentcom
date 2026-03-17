# PH5-02 MCP Handler Parameter Validation Plan

## Objective

- Finish PH5-02 from `.agents/plans/NEW-NEXT-PHASE-PLAN.md` by making MCP handler-level parameter validation behavior consistent across all registered tools.
- Ensure user-input failures are surfaced as `invalidParamsError` so `internal/mcp/server.go` can return JSON-RPC `error.code = -32602` instead of `-32000` runtime errors.
- Add targeted regression tests so malformed payloads, missing required fields, enum errors, and clearly invalid name-or-id inputs are covered per handler group.

## Evidence Snapshot

- `internal/mcp/handler.go:17` already defines `invalidParamsError` and `newInvalidParamsError()` as the canonical handler-level input validation mechanism.
- `internal/mcp/handler.go:312`, `internal/mcp/handler.go:411`, `internal/mcp/handler.go:437`, and `internal/mcp/handler.go:460` already use `newInvalidParamsError()` in parts of `create_task`, `update_task`, `approve_task`, and `reject_task`.
- `internal/mcp/handler.go:77`, `internal/mcp/handler.go:132`, `internal/mcp/handler.go:195`, `internal/mcp/handler.go:248`, `internal/mcp/handler.go:374`, `internal/mcp/handler.go:483`, and `internal/mcp/handler.go:541` still use mixed `fmt.Errorf(...)` paths for JSON unmarshal failures and missing required inputs.
- `internal/mcp/handler.go:607` resolves agent identifiers by trying project name first, then ID, and currently wraps failures generically.
- `internal/mcp/handler.go:640` falls back to returning the raw assignee string when resolution fails, which means some list-task input failures are intentionally non-fatal today.
- `internal/mcp/server.go:187` now maps `invalidParamsError` to JSON-RPC `-32602` and all other handler errors to `-32000`, so handler classification directly controls PH5-02 behavior.
- `internal/mcp/server_test.go` currently covers PH5-01 paths (unknown tool, invalid priority, runtime tool error) but does not yet provide a full invalid-argument matrix across the other handlers.

## Problem Statement

PH5-01 aligned server-side error routing, but handler-side validation is still inconsistent. Some malformed inputs correctly raise `invalidParamsError`, while others still bubble out as generic errors and therefore appear to MCP clients as `-32000` runtime failures. PH5-02 needs a clear policy for what counts as bad input versus legitimate runtime failure, then must apply that policy consistently across all MCP handlers.

## Scope Rules

- In scope: `internal/mcp/handler.go`, `internal/mcp/server_test.go`, and only the minimal adjacent helpers needed to classify input failures.
- In scope: JSON unmarshal/type failures, missing required fields, invalid enum/status values, invalid filter values, and obviously invalid identifier inputs when the failure is attributable to caller input.
- Out of scope: PH5-03 task state transitions, broader MCP tool additions, non-MCP CLI validation, and unrelated refactors.
- Out of scope: changing business rules inside task/agent/message packages unless a tiny adapter is required to expose user-input errors as `invalidParamsError`.

## Validation Policy To Lock Before Coding

1. Any JSON unmarshal/type mismatch in handler params returns `newInvalidParamsError(...)`.
2. Any missing required field returns `newInvalidParamsError(...)`.
3. Any bad enum/filter/status input supplied directly by the caller returns `newInvalidParamsError(...)`.
4. Name-or-ID resolution failures become `newInvalidParamsError(...)` only when the failure is clearly a caller input problem for that handler.
5. Missing project runtime state, DB failures, template manifest loading failures, and internal query/write failures remain ordinary runtime errors.
6. `list_tasks.assignee` fallback behavior must be explicitly decided before implementation because current behavior is permissive.

## Acceptance Criteria

1. Every handler in `registerTools()` has explicit and consistent parameter-validation behavior for malformed JSON and required inputs.
2. `list_agents`, `send_message`, `send_to_user`, `get_user_messages`, `broadcast`, `delegate_task`, `list_tasks`, and `get_status` no longer leak obvious caller-input errors as `-32000` if they should be `-32602`.
3. `create_task`, `update_task`, `approve_task`, and `reject_task` keep their existing valid behavior while expanding `-32602` coverage where currently incomplete.
4. The policy for name-or-id resolution failures is written down and reflected in tests.
5. `internal/mcp/server_test.go` contains a focused invalid-argument matrix covering each handler category.
6. Full verification passes: targeted MCP tests, `go test ./... -count=1`, `go build ./...`, and at least one manual MCP session that demonstrates a promoted `-32602` case.

## Tasks

### Task 1 - Classify Handler Input Surfaces

#### Subtasks

- T1.1: Enumerate all registered MCP tools from `registerTools()` and map them to their handler functions.
- T1.2: For each handler, list every input field and mark it as required / optional / derived.
- T1.3: For each handler, classify failure points into JSON parse, required-field, enum/filter, identifier resolution, and downstream runtime buckets.
- T1.4: Record which of those failure points should become `invalidParamsError` and which should stay runtime errors.
- T1.5: Explicitly document the special-case policy for `list_tasks.assignee`, `get_user_messages.agent`, `send_message.from/to`, `broadcast.from`, and `delegate_task.to`.

#### Done When

- There is a per-handler validation map that can be implemented mechanically without policy ambiguity.

### Task 2 - Define Shared Validation Helpers

#### Subtasks

- T2.1: Decide whether existing inline `strings.TrimSpace(...) == ""` checks are sufficient or whether tiny helper functions should be added.
- T2.2: If helpers are added, keep them local to `internal/mcp/handler.go` and narrowly scoped (for example required string, optional trimmed string, or invalid agent reference wrappers).
- T2.3: Define a helper strategy for wrapping JSON unmarshal failures into `newInvalidParamsError(...)` without losing context.
- T2.4: Define a helper strategy for promoting clearly invalid identifier resolution failures into `newInvalidParamsError(...)` while preserving true runtime errors.
- T2.5: Decide whether `task.ValidatePriority`, status validation, and filter validation should be called directly in handlers or behind local wrappers.

#### Done When

- The implementation approach is consistent enough to avoid copy-paste drift across handlers.

### Task 3 - Harden Simple Query Handlers

#### Subtasks

- T3.1: Update `handleListAgents()` so malformed JSON/type mismatches become `newInvalidParamsError(...)`.
- T3.2: Decide whether `alive_only` needs any additional validation beyond JSON typing.
- T3.3: Update `handleGetStatus()` so malformed JSON/type mismatches become `newInvalidParamsError(...)`.
- T3.4: Verify optional `project` handling stays permissive and does not regress existing behavior.

#### Done When

- Simple read-only handlers cleanly separate bad params from runtime failures.

### Task 4 - Harden Message Send Handlers

#### Subtasks

- T4.1: Update `handleSendMessage()` unmarshal failures to `newInvalidParamsError(...)`.
- T4.2: Keep `from` and `to` required-field checks on `newInvalidParamsError(...)`.
- T4.3: Decide whether non-object `payload` should be treated purely via JSON schema/unmarshal failure or needs explicit validation.
- T4.4: Decide whether sender/recipient resolution failures should be promoted to invalid params when caller provided an unknown/invalid reference.
- T4.5: Update `handleBroadcast()` unmarshal and missing-`from` failures to `newInvalidParamsError(...)`.
- T4.6: Apply the same identifier-resolution policy to `handleBroadcast()`.
- T4.7: Update `handleSendToUser()` unmarshal and required-field failures to `newInvalidParamsError(...)`.
- T4.8: Decide whether invalid `priority` text to the user is intentionally free-form or should be validated.

#### Done When

- Message-producing handlers consistently return `-32602` for caller input mistakes.

### Task 5 - Harden Human Inbox Query Handler

#### Subtasks

- T5.1: Update `handleGetUserMessages()` unmarshal failures to `newInvalidParamsError(...)`.
- T5.2: Decide whether an invalid `agent` filter should become `newInvalidParamsError(...)` when name-or-id lookup fails.
- T5.3: Verify `unread_only` typing is already enforced by JSON unmarshaling and therefore covered once unmarshal failure mapping is fixed.
- T5.4: Keep missing user pseudo-agent/runtime setup as a runtime error, not invalid params.

#### Done When

- Querying user messages distinguishes bad filters from missing runtime state.

### Task 6 - Harden Task Mutation Handlers

#### Subtasks

- T6.1: Review `handleCreateTask()` for remaining input paths not yet classified as invalid params.
- T6.2: Decide whether unresolved `assigned_to` / `created_by` values should stay permissive fallback strings or become invalid params.
- T6.3: If `blocked_by` contains malformed element types or invalid task IDs, decide whether PH5-02 should validate that here or defer to manager-level behavior.
- T6.4: Update `handleDelegateTask()` unmarshal failure and required fields to `newInvalidParamsError(...)`.
- T6.5: Decide whether unknown `to` target in `handleDelegateTask()` is a caller input failure and promote accordingly if yes.
- T6.6: Review `handleUpdateTask()` for missing explicit status value validation before manager call if current error surfacing is ambiguous.
- T6.7: Review `handleApproveTask()` and `handleRejectTask()` for any remaining required-input or malformed-input gaps.

#### Done When

- Task mutation handlers have an explicit and test-backed invalid-params policy.

### Task 7 - Harden Task Query Handler

#### Subtasks

- T7.1: Update `handleListTasks()` unmarshal failures to `newInvalidParamsError(...)`.
- T7.2: Decide whether `status` should be validated against the allowed task states before querying.
- T7.3: Decide whether `assignee` resolution failure should return invalid params or preserve current permissive fallback.
- T7.4: If both `status` and `assignee` are provided, ensure the chosen validation policy is applied consistently in both the primary query and the post-filter path.

#### Done When

- `list_tasks` filter handling is explicit, deterministic, and covered by tests.

### Task 8 - Expand Server-Level Regression Matrix

#### Subtasks

- T8.1: Add table-driven or grouped tests in `internal/mcp/server_test.go` for malformed JSON/tool arguments across representative handlers.
- T8.2: Add tests for missing required strings in message/task mutation handlers.
- T8.3: Add tests for invalid enum/filter values such as task priority or task status, depending on final PH5-02 policy.
- T8.4: Add tests for identifier-resolution failures that are intentionally promoted to `-32602`.
- T8.5: Add tests for at least one resolution/runtime path that must remain `-32000` to prove the distinction still exists.
- T8.6: Keep tests grouped by handler category so future MCP tools can extend the matrix without confusion.

#### Done When

- The MCP test suite proves both promotion to `-32602` and retention of legitimate runtime failures.

### Task 9 - Manual QA And Full Verification

#### Subtasks

- T9.1: Run `lsp_diagnostics` on changed MCP files.
- T9.2: Run targeted MCP tests first.
- T9.3: Run `go test ./... -count=1`.
- T9.4: Run `go build ./...`.
- T9.5: Run a manual `agentcom mcp-server` STDIO session that demonstrates one newly promoted `-32602` response.
- T9.6: Run a second manual case that still returns a runtime error (`-32000`) to confirm classification boundaries remain intact.
- T9.7: Capture the exact observed JSON snippets for memory/doc updates.

#### Done When

- Automated and manual verification both prove the new validation policy works end-to-end.

### Task 10 - Update Memory And Plan Status

#### Subtasks

- T10.1: Update `.agents/MEMORY.md` with PH5-02 branch start, policy decisions, verification evidence, and next task.
- T10.2: Update `.agents/plans/NEW-NEXT-PHASE-PLAN.md` PH5-02 status and remaining PH5 effort.
- T10.3: Update any README wording only if PH5-02 changes user-visible MCP semantics beyond what PH5-01 already documented.
- T10.4: Record any intentionally deferred validation decisions as follow-ups instead of silently omitting them.

#### Done When

- Memory and planning docs accurately reflect what PH5-02 changed and what remains.

## Suggested Execution Order

1. Task 1 - Classify Handler Input Surfaces
2. Task 2 - Define Shared Validation Helpers
3. Task 3 - Harden Simple Query Handlers
4. Task 4 - Harden Message Send Handlers
5. Task 5 - Harden Human Inbox Query Handler
6. Task 6 - Harden Task Mutation Handlers
7. Task 7 - Harden Task Query Handler
8. Task 8 - Expand Server-Level Regression Matrix
9. Task 9 - Manual QA And Full Verification
10. Task 10 - Update Memory And Plan Status

## Verification Commands

```bash
go test ./internal/mcp/... -count=1
go test ./... -count=1
go build ./...
AGENTCOM_HOME=/tmp/agentcom-ph5-02-qa python - <<'PY'
import json, subprocess

proc = subprocess.Popen(
    ["go", "run", "./cmd/agentcom", "mcp-server"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True,
)

requests = [
    {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
    {"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}},
    {"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "send_message", "arguments": {"from": "", "to": "plan"}}},
]

for payload in requests:
    proc.stdin.write(json.dumps(payload) + "\n")
proc.stdin.close()

for line in proc.stdout:
    print(line.rstrip())

proc.wait()
PY
```

## Expected Verification Outcomes

- Representative caller-input mistakes now return JSON-RPC `error.code = -32602`.
- At least one true runtime failure path still returns `error.code = -32000`.
- No PH5-01 regression: unknown tool remains `-32601` and successful tool calls still use `result`.

## Risks

- Over-classifying identifier lookup failures as invalid params could hide legitimate runtime/setup failures.
- Under-classifying filter/status validation could leave PH5-02 incomplete even if a few tests pass.
- Changing permissive fallback behavior in `create_task` or `list_tasks` without explicit policy may break existing clients unexpectedly.
- Broad helper refactors in `handler.go` could inflate risk for what should stay a focused validation task.

## Mitigations

- Lock the invalid-params policy in Task 1 before editing code.
- Prefer narrow local helpers over structural refactors.
- Preserve runtime errors for environment/setup/DB/template failures.
- Add at least one test that proves a failure path intentionally remains `-32000`.
