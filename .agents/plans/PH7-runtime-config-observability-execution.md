# PH7 Runtime Config And Observability Execution Plan

## Objective

- Finish PH7 after PH6 stabilizes runtime behavior, externalizing confirmed runtime knobs and improving user-visible operational feedback.
- Keep PH7 focused on configuration, logs, structured user errors, and supervisor signal handling.
- Execute PH7 on its own branch from a PH6-merged `develop` baseline.

## Source Context

- `NEW-NEXT-PHASE-PLAN.md` marks PH7-01 through PH7-04 as open/partial.
- `internal/config/config.go` currently externalizes only `AGENTCOM_HOME`.
- `internal/cli/errors.go` provides a structured user error type but rollout is incomplete.
- `internal/transport/uds.go` and `internal/message/router.go` still treat important routing events as debug-heavy.
- `internal/cli/up.go` currently handles SIGINT/SIGTERM but not broader supervisor signals.

## Scope Rules

- In scope: runtime config externalization, structured user error rollout, transport/message log rebalance, supervisor signal handling.
- Out of scope: new runtime behaviors beyond exposing or observing PH6-confirmed behavior.
- Out of scope: new MCP tools and PH9 standalone test closures except tests required by PH7 changes.

## Acceptance Criteria

1. A runtime config surface exists for PH6-established timeout/retry/interval values.
2. Representative CLI commands consistently use structured user-facing errors where appropriate.
3. Important transport/message failure transitions are logged at appropriate severity.
4. Supervisor signal handling is expanded per PH7 scope with tests or manual QA.
5. PH7 branch passes `go test ./... -count=1` and `go build ./...`.
6. Manual QA demonstrates config override behavior and at least one signal/logging path.

## Task Breakdown

### Task 1 - Lock PH7 config boundaries

- T1.1: Re-read PH6 results and list only values that are stable enough to externalize.
- T1.2: Define a minimal `RuntimeConfig` API and precedence order.
- T1.3: Record env names and defaults in this plan before coding.

### Task 2 - PH7-01 runtime config externalization (TDD first)

- T2.1: Add failing config tests for env override behavior.
- T2.2: Implement the minimal runtime config surface and thread it into affected packages.
- T2.3: Re-run affected package tests and manual override checks.

### Task 3 - PH7-02 structured user error rollout (TDD first)

- T3.1: Identify the highest-value CLI entry points first (`init`, `up`, `task`, `register`, `skill`, `root`).
- T3.2: Add failing tests or snapshot/string assertions for representative user-facing failures.
- T3.3: Apply `userError`/`commandError` consistently without changing internal error wrapping rules.

### Task 4 - PH7-03 transport/message log rebalance

- T4.1: Add tests or hook-based assertions for severity expectations where practical.
- T4.2: Promote final direct-send failure and fallback transitions to visible operational levels.
- T4.3: Keep healthy/normal direct-send flow at debug level.

### Task 5 - PH7-04 supervisor signal expansion

- T5.1: Decide the minimal PH7 signal scope after PH7-01 is in place.
- T5.2: Add tests or harness coverage for the chosen signal behavior.
- T5.3: Implement only the approved signal paths.

### Task 6 - Verification, docs, merge

- T6.1: Run `lsp_diagnostics` on changed files.
- T6.2: Run targeted config/cli/transport tests.
- T6.3: Run `go test ./... -count=1` and `go build ./...`.
- T6.4: Manual QA: env override, representative structured error, and one signal/logging path.
- T6.5: Update MEMORY/plan docs and merge PH7 back to `develop`.

## Verification Commands

```bash
go test ./internal/config/... -count=1
go test ./internal/cli/... -count=1
go test ./internal/transport/... -count=1
go test ./... -count=1
go build ./...
```
