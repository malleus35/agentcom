# PH5-01 MCP JSON-RPC Error Alignment Plan

## Objective

- Finish the remaining PH5-01 work from `.agents/plans/NEW-NEXT-PHASE-PLAN.md` by making all MCP tool-call error paths return JSON-RPC `error` objects instead of MCP `result.isError` payloads.
- Preserve the current success-path response shape for successful tool calls.
- Add focused regression coverage so unknown tool, invalid params, and runtime tool failures all round-trip through `internal/mcp/server.go` with the expected JSON-RPC semantics.

## Evidence Snapshot

- `internal/mcp/server.go:17` defines the existing JSON-RPC error constants `-32600`, `-32601`, `-32602`, and `-32603`.
- `internal/mcp/server.go:181` still maps unknown tool lookup failures to `Result: newToolResult(..., true)`.
- `internal/mcp/server.go:190` still maps general tool execution failures to `Result: newToolResult(..., true)`.
- `internal/mcp/server.go:192` already upgrades `invalidParamsError` to `Response.Error` with code `-32602`.
- `internal/mcp/server.go:209` uses `newToolResult(..., false)` for successful tool calls and should stay on the success path only.
- `internal/mcp/handler.go:17` defines `invalidParamsError`, which is already the narrow mechanism for handler-level invalid-parameter reporting.
- `internal/mcp/server_test.go:202` covers one invalid-params case, but there is no round-trip assertion yet for unknown tool or non-invalid-params tool failure.
- `README.md:791` and translated READMEs describe `agentcom mcp-server` and MCP tools, but do not currently state the error-format guarantee.
- `.agents/MEMORY.md:15` still says the next work is PH5~PH9 replanning/selection and does not record PH5-01 execution yet.

## Problem Statement

`agentcom` already behaves like JSON-RPC for malformed requests and invalid params, but `tools/call` still leaks two non-compliant error paths: unknown tools and general handler failures. Those paths currently return a tool-style success envelope with `isError: true`, which makes MCP behavior inconsistent and leaves PH5-01 only partially complete.

## Acceptance Criteria

1. Unknown tool requests return `Response.Error` with code `-32601` and no `result` field.
2. Invalid `tools/call` params continue to return `Response.Error` with code `-32602` and no `result` field.
3. Non-invalid-params handler failures return `Response.Error` using `-32000` as the PH5-01 tool-execution code and no `result` field.
4. Successful tool calls still return `result.content[*].text` plus `isError: false`.
5. `internal/mcp/server_test.go` covers unknown tool, invalid params, and runtime tool failure through the JSON-RPC round-trip path.
6. `lsp_diagnostics` is clean on changed files.
7. `go test ./...`, `go build ./...`, and a manual MCP server round-trip QA command complete successfully.

## Tasks

### Task 1 - Lock Error Mapping Rules

#### Subtasks

- T1.1: Confirm the PH5-01 scope boundary from `.agents/plans/NEW-NEXT-PHASE-PLAN.md` and keep PH5-02 validation-unification work out of scope.
- T1.2: Define the exact tool-runtime error code in the `-32000~-32099` range.
- T1.3: Keep unknown tool on JSON-RPC `-32601` rather than introducing a custom code.
- T1.4: Preserve `newToolResult(..., false)` for successful tool calls only.

#### Done When

- The server has one explicit mapping table for unknown tool, invalid params, runtime tool failure, and internal marshal failure.

### Task 2 - Expand Regression Coverage First

#### Subtasks

- T2.1: Add a round-trip test for an unknown MCP tool name.
- T2.2: Keep or refine the existing invalid-priority test so it verifies `error` semantics and lack of `result`.
- T2.3: Add a round-trip test for a handler failure that is not `invalidParamsError`.
- T2.4: Assert the runtime error uses `-32000`.
- T2.5: Assert success responses still use `result.isError == false`.

#### Done When

- The test matrix distinguishes success, invalid params, unknown tool, and runtime tool failure before production-code changes are finalized.

### Task 3 - Align `internal/mcp/server.go`

#### Subtasks

- T3.1: Add the server-level `errToolExecution = -32000` constant.
- T3.2: Change the unknown-tool branch in `handleToolCall()` to return `newErrorResponse(req.ID, errMethodNotFound, ...)`.
- T3.3: Change the generic handler-error branch in `handleToolCall()` to return `newErrorResponse(req.ID, errToolExecution, ...)`.
- T3.4: Keep the `invalidParamsError` type assertion path on `-32602` unchanged except for any small cleanup needed for consistency.
- T3.5: Ensure every error branch returns `Response.Error` only, with `Response.Result` omitted.

#### Done When

- `handleToolCall()` has a clean split: success uses `result`, all failures use `error`.

### Task 4 - Verify Behavior End To End

- T4.1: Run `lsp_diagnostics` on the changed Go files.
- T4.2: Run targeted MCP tests first.
- T4.3: Run `go test ./... -count=1`.
- T4.4: Run `go build ./...`.
- T4.5: Run a manual STDIO MCP session with isolated `AGENTCOM_HOME` that proves one success path and one error path after `initialize`.
- T4.6: Confirm the success response includes `result.isError == false` and the unknown-tool response includes `error.code == -32601`, `error.message == "unknown tool: no_such_tool"`, and no `result` field.

#### Done When

- Automated verification and manual QA both confirm the new JSON-RPC error behavior.

### Task 5 - Update Project Memory And User Docs

#### Subtasks

- T5.1: Update `.agents/MEMORY.md` with the PH5-01 branch, implementation notes, verification evidence, and next recommended task.
- T5.2: Update the PH5 status in `.agents/plans/NEW-NEXT-PHASE-PLAN.md`.
- T5.3: Update the older `.agents/plans/NEXT-PHASE-PLAN.md` only if it still needs status synchronization.
- T5.4: Update `README.md`, `README.ko.md`, `README.ja.md`, and `README.zh.md` only with minimal MCP behavior wording that is actually user-visible.

#### Done When

- Memory and user-facing docs accurately reflect the completed PH5-01 behavior without broad roadmap rewrites.

## Execution Order

1. Task 1 - Lock Error Mapping Rules
2. Task 2 - Expand Regression Coverage First
3. Task 3 - Align `internal/mcp/server.go`
4. Task 4 - Verify Behavior End To End
5. Task 5 - Update Project Memory And User Docs

## Verification Commands

```bash
go test ./internal/mcp/... -count=1
go test ./... -count=1
go build ./...
AGENTCOM_HOME=/tmp/agentcom-ph5-01-manual-qa python - <<'PY'
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
    {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
    {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "no_such_tool", "arguments": {}}},
]

for payload in requests:
    proc.stdin.write(json.dumps(payload) + "\n")
proc.stdin.close()

for line in proc.stdout:
    print(line.rstrip())

proc.wait()
PY
```

Expected manual QA observations:

- Response to `id=2` contains a `result.tools` array and no `error` field.
- Response to `id=3` contains `error.code = -32601` and `error.message = "unknown tool: no_such_tool"`.
- Response to `id=3` does not contain a `result` field.

## Risks

- Picking a tool-runtime error code that is too generic could make later PH5-02/PH9 analysis harder if more granular categories are needed.
- Overreaching into handler validation during PH5-01 could blur the boundary with PH5-02 and create unnecessary churn.
- Manual QA via `go run ./cmd/agentcom mcp-server` may expose unrelated environment/setup issues if the local working directory has unexpected state.

## Mitigations

- Use one narrow runtime error code now and leave finer categorization for a later task only if the codebase needs it.
- Limit production-code edits to `internal/mcp/server.go` unless tests force a very small adjacent change.
- Use a manual QA flow that exercises only `initialize`, `tools/list`, and one deliberate error case.
