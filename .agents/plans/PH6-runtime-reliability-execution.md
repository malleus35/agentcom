# PH6 Runtime Reliability Execution Plan

## Objective

- Finish PH6 from `.agents/plans/NEW-NEXT-PHASE-PLAN.md` by removing the most serious runtime reliability risks in `up/down`, UDS transport, registry cleanup, and SQLite health paths.
- Keep PH6 scoped to runtime safety and operational correctness; do not fold PH7 config externalization or PH8 MCP surface work into this phase.
- Execute PH6 on a dedicated branch from `develop` with strict TDD, full verification, manual QA, and merge-back before PH7 starts.

## Source Context

- `.agents/MEMORY.md` points to PH6-02 as the next task after PH5-03 merge.
- `.agents/plans/NEW-NEXT-PHASE-PLAN.md` defines PH6-01 through PH6-08 and orders PH6-02 before the rest of PH6.
- `internal/cli/up.go` still uses `deregisterUserPseudoAgent(context.Background(), ...)` in defer, force, and non-force shutdown paths.
- `internal/transport/uds.go` has no accept deadline, no read deadline, and only two immediate send attempts with debug logging.
- `internal/agent/registry.go` marks dead agents but does not remove stale socket paths on `MarkInactive()`.
- `internal/db/sqlite.go` pings on open but has no explicit health helper or runtime integrity verification surface.

## Scope Rules

- In scope: PH6-01 through PH6-08 only.
- In scope: runtime reliability fixes, transport lifecycle improvements, registry cleanup, validation, and health helpers.
- In scope: targeted tests and manual QA proving the runtime paths behave correctly.
- Out of scope: PH7 config externalization, PH8 MCP parity additions, PH9 standalone test-only backlog beyond PH6 coverage needs.
- Out of scope: unrelated refactors, CLI UX redesign, new persistence schema unless a PH6 task strictly requires it.

## Acceptance Criteria

1. `internal/cli/up.go` no longer uses bare `context.Background()` in the three user pseudo-agent deregistration shutdown paths.
2. UDS server accept/read paths can time out without noisy failure loops and can still stop cleanly.
3. UDS client send uses bounded retries with backoff and jitter instead of immediate retry only.
4. Message routing has explicit overflow/rate protection behavior with tests proving enforcement.
5. Stale runtime/socket recovery is stronger than the current baseline and explicitly tested.
6. Agent registration rejects invalid names according to the PH6 regex/policy.
7. SQLite runtime health helpers exist and are verified by unit tests.
8. `go test ./internal/... -count=1` for affected packages passes.
9. `go test ./... -count=1` and `go build ./...` pass.
10. Manual QA demonstrates at least shutdown timeout safety and one transport/runtime recovery scenario.

## Task Breakdown

### Task 1 - Lock PH6 execution order and branch protocol

#### Subtasks

- T1.1: Create `feature/PH6-runtime-reliability` from `develop`.
- T1.2: Record PH6 acceptance criteria and verification commands in this plan before edits.
- T1.3: Keep PH6 implementation commits split by concern: shutdown safety, UDS runtime, routing protection, registry/health.

#### Done When

- PH6 work is isolated on its branch with a locked execution order.

### Task 2 - PH6-02 shutdown timeout context (TDD first)

#### Subtasks

- T2.1: Add a failing `internal/cli/up_test.go` regression covering timeout-backed deregistration helper behavior.
- T2.2: Verify the new test fails for the expected reason before implementation.
- T2.3: Introduce the smallest helper needed to wrap `deregisterUserPseudoAgent` with `context.WithTimeout(context.Background(), 5*time.Second)`.
- T2.4: Replace the three `context.Background()` shutdown-path calls with the helper.
- T2.5: Re-run targeted CLI tests and confirm green.

#### Done When

- All three PH6-02 shutdown paths use timeout-backed deregistration and tests prove it.

### Task 3 - PH6-03/04 UDS lifecycle and retry hardening (TDD first)

#### Subtasks

- T3.1: Add failing transport tests for accept-loop shutdown under idle conditions.
- T3.2: Add failing transport tests for read deadline behavior on idle/slow connections.
- T3.3: Add failing transport tests for retry count/backoff behavior.
- T3.4: Implement periodic accept deadline handling in `internal/transport/uds.go`.
- T3.5: Implement read deadline handling per connection decode loop.
- T3.6: Implement bounded retry policy with exponential backoff and jitter.
- T3.7: Keep timeout-triggered paths quiet unless they represent real failures.
- T3.8: Adjust logging to align final failure and fallback visibility with the new retry semantics only if PH6 requires it.

#### Done When

- Transport tests prove the server stops cleanly under idle conditions and the client uses bounded backoff retry.

### Task 4 - PH6-05 message overflow and rate protection

#### Subtasks

- T4.1: Audit current router/message persistence surfaces and lock the minimum policy that satisfies PH6 without inventing extra knobs.
- T4.2: Add failing tests covering inbox overflow and sender rate violations.
- T4.3: Implement the smallest DB/router changes needed to enforce the policy.
- T4.4: Add or update tests for cleanup/FIFO behavior where messages are dropped or capped.
- T4.5: Verify direct send and broadcast still work within limits.

#### Done When

- The router/database enforces explicit bounded behavior with regression coverage.

### Task 5 - PH6-06 stale runtime/socket cleanup strengthening

#### Subtasks

- T5.1: Add failing tests for stale runtime state handling on `up` restart.
- T5.2: Add failing tests for stale socket cleanup when agents are marked inactive.
- T5.3: Implement minimal runtime-state cleanup or warning behavior in `internal/cli/up.go`.
- T5.4: Extend `internal/agent/registry.go` cleanup only where PH6 requires it.
- T5.5: Re-run affected CLI/agent/transport tests.

#### Done When

- Recovery from stale runtime artifacts is better than the current baseline and covered by tests.

### Task 6 - PH6-07 agent name validation

#### Subtasks

- T6.1: Add failing table-driven tests for valid/invalid names and reserved-name behavior.
- T6.2: Implement minimal validation in the registration path and any required shared helper.
- T6.3: Verify register-facing behavior returns consistent errors without widening scope into PH7 UX work.

#### Done When

- Invalid agent names are rejected by tests and production code.

### Task 7 - PH6-08 SQLite runtime health checks

#### Subtasks

- T7.1: Add failing tests around the new health helper contract.
- T7.2: Implement a minimal health probe helper in `internal/db/sqlite.go`.
- T7.3: Verify WAL/integrity-related checks are bounded and do not overcomplicate open-path behavior.

#### Done When

- SQLite has an explicit health-check surface with tests.

### Task 8 - Verification, docs, merge

#### Subtasks

- T8.1: Run `lsp_diagnostics` on modified PH6 files.
- T8.2: Run targeted package tests after each logical PH6 unit.
- T8.3: Run `go test ./... -count=1`.
- T8.4: Run `go build ./...`.
- T8.5: Run manual QA for `agentcom down --force` and one transport/runtime recovery path.
- T8.6: Update `.agents/MEMORY.md` and `.agents/plans/NEW-NEXT-PHASE-PLAN.md` with PH6 completion details.
- T8.7: Commit PH6 work in atomic units and merge back into `develop`.

#### Done When

- PH6 is verified, documented, committed, and merged to `develop`.

## Suggested Execution Order

1. Task 1 - Lock execution order and branch protocol
2. Task 2 - PH6-02 shutdown timeout context
3. Task 3 - PH6-03/04 UDS lifecycle and retry hardening
4. Task 4 - PH6-05 message overflow and rate protection
5. Task 5 - PH6-06 stale runtime/socket cleanup
6. Task 6 - PH6-07 agent name validation
7. Task 7 - PH6-08 SQLite runtime health checks
8. Task 8 - Verification, docs, merge

## Verification Commands

```bash
go test ./internal/cli/... -run 'Test.*Up|Test.*Register' -count=1
go test ./internal/transport/... -count=1
go test ./internal/agent/... -count=1
go test ./internal/db/... -count=1
go test ./... -count=1
go build ./...
```

## Manual QA Targets

- `agentcom up` -> `agentcom down --force` on a temp `AGENTCOM_HOME`, proving the runtime state is removed and the user pseudo-agent cleanup path returns promptly.
- At least one UDS slow/idle or stale-runtime scenario executed through a real command or minimal integration harness, with captured output.
