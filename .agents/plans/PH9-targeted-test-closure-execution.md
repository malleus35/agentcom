# PH9 Targeted Test Closure Execution Plan

## Objective

- Finish PH9 by closing the highest-value test gaps that remain after PH6-PH8 implementation settles.
- Keep PH9 strictly test-focused unless a tiny production change is required to make testing practical.

## Source Context

- `internal/onboard` has `wizard_test.go`, `result_test.go`, and `detect_test.go`, but gaps remain in prompter/template-definition surfaces.
- `internal/task/query.go` has no dedicated test file.
- `internal/transport/transport_test.go` covers roundtrip/stale socket/poller, but not the richer lifecycle cases PH6 will introduce.
- `internal/mcp/server_test.go` has improved invalid-params coverage but still needs a fuller matrix.

## Scope Rules

- In scope: PH9-01 through PH9-04 only.
- In scope: test additions and the smallest production seams required for testability.
- Out of scope: new product behavior not needed to support the tests.

## Acceptance Criteria

1. `internal/onboard` has dedicated coverage for the remaining PH9 surfaces.
2. `internal/task/query.go` has direct query tests.
3. Transport/heartbeat lifecycle tests cover the intended runtime scenarios introduced by PH6.
4. MCP error response matrix covers unknown tool, runtime error, empty params, malformed request, and boundary cases called out by the plan.
5. `go test ./... -count=1` and `go build ./...` pass after PH9.

## Task Breakdown

### Task 1 - PH9-01 onboard package tests

- T1.1: Inventory remaining untested onboard helpers.
- T1.2: Add focused tests for `huh_prompter.go`, `prompter.go`, and `template_definition.go` behavior.
- T1.3: Keep tests table-driven and avoid broad fixture sprawl.

### Task 2 - PH9-02 task query tests

- T2.1: Add a dedicated `internal/task/query_test.go`.
- T2.2: Cover `NewQuery`, `ListAll`, `ListByStatus`, `ListByAssignee`, and `FindByID` directly.
- T2.3: Reuse in-memory DB helpers where practical.

### Task 3 - PH9-03 transport and heartbeat lifecycle tests

- T3.1: Extend transport tests to cover lifecycle scenarios added in PH6.
- T3.2: Add heartbeat lifecycle assertions where current coverage is missing.
- T3.3: Keep these tests scoped to behavior, not log internals unless required.

### Task 4 - PH9-04 MCP error response matrix

- T4.1: Inventory current server test coverage.
- T4.2: Add the missing malformed/empty/unknown/runtime boundary cases.
- T4.3: Confirm the matrix matches actual server semantics after PH8.

### Task 5 - Verification, docs, merge

- T5.1: Run `lsp_diagnostics` on changed test files.
- T5.2: Run targeted package tests, then full test/build.
- T5.3: Update MEMORY/plan docs with PH9 completion and remaining risk notes.
- T5.4: Merge PH9 back into `develop`.

## Verification Commands

```bash
go test ./internal/onboard/... -count=1
go test ./internal/task/... -count=1
go test ./internal/transport/... -count=1
go test ./internal/mcp/... -count=1
go test ./... -count=1
go build ./...
```
