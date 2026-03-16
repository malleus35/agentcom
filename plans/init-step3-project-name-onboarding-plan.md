# Init Step 3 Project Name Onboarding Plan

## Objective

- Extend `agentcom init` onboarding so Step 3 collects a project name explicitly.
- Default the project name to the current directory name when it is valid, while still letting the user edit it.
- Prevent duplicate project names against known existing projects before onboarding completes.
- Keep the existing instruction/memory/template flow intact and document the behavior tradeoffs.

## Scope

### In Scope

- `internal/cli/init_prompter.go` Step 3 onboarding UX and validation.
- Supporting config/DB helpers required to validate duplicate project names.
- Onboarding/apply tests needed to verify project persistence and validation.
- A small DB/query helper if needed to enumerate known project names.
- Manual QA and implementation notes for Step 3 toggles.

### Out of Scope

- New top-level CLI flags or batch-mode redesign.
- Project rename/migration flows for already initialized directories.
- Broader wizard layout changes beyond Step 3 additions needed for this feature.
- Release/docs updates outside this focused implementation unless a test forces them.

## Acceptance Criteria

1. Step 3 asks for a project name before the instruction/memory confirms.
2. The Step 3 project input defaults to the current folder name when that value is valid.
3. Users can edit the default project name directly in the wizard.
4. Duplicate project names are rejected before onboarding completes.
5. The saved `.agentcom.json` contains the Step 3 project value.
6. That saved project value is the one later used by project-scoped agent registration flows.
7. Existing instruction/memory confirmation behavior stays consistent unless explicitly changed by this task.
8. Targeted tests, full tests, build, and manual QA all pass.

## Work Breakdown

### Task T0 - Source Mapping

#### Goal

Locate every code path involved in project-name defaults, onboarding validation, config persistence, and project-scoped agent registration.

#### Subtasks

- T0.1: Inspect `internal/cli/init_prompter.go` for current Step 3 structure.
- T0.2: Inspect `internal/cli/init.go` and `internal/cli/init_setup.go` for wizard defaults and apply behavior.
- T0.3: Inspect `internal/config/project.go` for project-name validation and config persistence.
- T0.4: Inspect `internal/cli/root.go` and `internal/cli/register.go` to confirm how saved project scope reaches agent registration.
- T0.5: Identify whether duplicate project-name checking already exists anywhere.
- T0.6: Identify tests that currently cover project config writes and wizard behavior.

#### QA

- A complete file/function map exists for Step 3 project flow from prompt to DB-scoped registration.

### Task T1 - Behavior Rules

#### Goal

Define exact Step 3 behavior before changing code.

#### Subtasks

- T1.1: Treat project name as a required Step 3 input for interactive onboarding.
- T1.2: Use the current folder name as the default only when it passes existing project-name validation.
- T1.3: Allow direct user edits to that default value.
- T1.4: Reject duplicate names using a deterministic source of known projects.
- T1.5: Preserve the two existing Step 3 confirms for instruction and memory generation.
- T1.6: Keep `WriteMemory` dependent on `WriteInstructions` unless explicit behavior changes are required.

#### QA

- The final rule set is specific enough to implement without hidden assumptions.

### Task T2 - Duplicate Project Validation Design

#### Goal

Choose the narrowest reliable way to detect duplicate project names.

#### Subtasks

- T2.1: Decide the canonical source of “known existing projects” for validation.
- T2.2: Add a helper in config/DB/CLI only if needed to query existing project names.
- T2.3: Define how to handle empty legacy project values.
- T2.4: Define whether the current directory’s already-saved project should be treated as allowed or conflicting.

#### QA

- Duplicate detection has a precise source, edge-case policy, and test strategy.

### Task T3 - TDD RED

#### Goal

Write failing tests before implementation.

#### Subtasks

- T3.1: Add a unit test for Step 3 default project behavior.
- T3.2: Add a unit test for Step 3 duplicate-name rejection.
- T3.3: Add or extend an onboarding/apply test to verify the project value is written to `.agentcom.json`.
- T3.4: Add a DB/helper test if a new project-query helper is introduced.
- T3.5: Run targeted tests and confirm they fail for the expected missing behavior.

#### QA

- New tests fail before production code changes.

### Task T4 - TDD GREEN

#### Goal

Implement the smallest code change that satisfies the new tests.

#### Subtasks

- T4.1: Add Step 3 project input to `internal/cli/init_prompter.go`.
- T4.2: Wire the default project value into the wizard state.
- T4.3: Add validation for required + valid + non-duplicate project names.
- T4.4: Ensure the returned `onboard.Result` carries the chosen project value.
- T4.5: Add any config/DB helper needed for duplicate detection.
- T4.6: Keep current instruction/memory/template logic unchanged except where project wiring requires touching it.

#### QA

- The new tests pass with minimal code churn.

### Task T5 - Verification

#### Goal

Verify behavior end to end and capture the exact implications of the Step 3 booleans.

#### Subtasks

- T5.1: Run targeted tests first.
- T5.2: Run `go test ./...`.
- T5.3: Run `go build ./...`.
- T5.4: Run a manual interactive `agentcom init` session and capture Step 3 output.
- T5.5: Confirm the project value lands in `.agentcom.json`.
- T5.6: Confirm a later project-scoped registration path resolves the same project value.
- T5.7: Record whether `Generate instruction files...` must be `Yes`, whether `Generate memory files...` must be `Yes`, and why/why not.
- T5.8: Record the practical benefit of auto-generating instruction/memory artifacts.

#### QA

- Verification includes actual command output and concrete findings for the Step 3 toggles.

### Task T6 - Memory Closure

#### Goal

Update project memory after implementation completes.

#### Subtasks

- T6.1: Record the Step 3 project-name onboarding change.
- T6.2: Record any new helper or duplicate-validation rule.
- T6.3: Record tests and manual QA commands executed.
- T6.4: Update next-step context if any follow-up remains.

#### QA

- `.agents/MEMORY.md` reflects the final state accurately.

## Execution Order

1. T0 Source Mapping
2. T1 Behavior Rules
3. T2 Duplicate Project Validation Design
4. T3 TDD RED
5. T4 TDD GREEN
6. T5 Verification
7. T6 Memory Closure

## Verification Commands

```bash
go test ./internal/cli/...
go test ./internal/config/...
go test ./internal/db/...
go test ./...
go build ./...
agentcom init
agentcom register --name <name> --type <type>
```

## Risks

- Duplicate validation may be underspecified if project names only exist in `.agentcom.json` files and not yet in SQLite.
- Wizard validation could reject legitimate reruns if current-project exceptions are mishandled.
- Step 3 UX could become confusing if project input and two confirms are not ordered clearly.

## Mitigations

- Keep the duplicate source explicit and test the edge cases.
- Preserve existing Step 3 confirm semantics unless a failing test proves a required change.
- Verify the interactive flow manually, not just with unit tests.
