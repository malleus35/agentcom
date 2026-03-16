# Init Step 3 Known Projects Error Recovery Plan

## Objective

- Eliminate the Step 3 `Project Instructions` validation failure surfaced as `list known projects` during interactive `agentcom init`.
- Make project-name validation resilient when the local SQLite state is missing, stale, partially migrated, or otherwise not ready for `projects` lookups.
- Preserve duplicate-project protection when the underlying project registry is healthy.

## Evidence Snapshot

- `internal/cli/init_prompter.go:63` validates the Step 3 project name and wraps lookup failures as `list known projects`.
- `internal/cli/init_prompter.go:108` opens `<home>/agentcom.db` and calls `ListProjects()` to build the known-project set.
- `internal/db/agent.go:375` currently assumes the `projects` table exists and runs `SELECT name FROM projects`.
- `internal/db/migrations.go:76` creates the `projects` table only through migrations, which means pre-migration or legacy DB states can break the lookup path.
- `internal/cli/init_prompter_test.go:112` covers valid/duplicate/current-project behavior, but does not cover missing-table, migration-gap, or degraded startup states.

## Problem Statement

Step 3 currently treats any failure while enumerating known projects as a hard validation error. That is safe for duplicate detection, but brittle for onboarding because the wizard can fail before the user finishes setup if the DB exists without the expected schema or if project enumeration cannot run yet.

## Working Hypotheses

1. A legacy or partially initialized `agentcom.db` exists, but the `projects` table has not been created yet.
2. The wizard is opening a DB file successfully, but `ListProjects()` fails on schema/query mismatch and the error is surfaced directly in Step 3.
3. The validation path lacks a graceful fallback for "cannot enumerate projects yet" even though duplicate detection is a best-effort safeguard during onboarding.

## Acceptance Criteria

1. Interactive Step 3 no longer fails with a raw `list known projects` validation error in recoverable DB states.
2. Duplicate project names are still rejected when the project registry is available and healthy.
3. Missing or unmigrated `projects` table states are either auto-recovered or downgraded to a safe fallback with deterministic behavior.
4. Error messaging distinguishes between duplicate-name rejection and infrastructure/setup failures.
5. Targeted tests cover healthy, missing-table, and current-project rerun scenarios.
6. Full verification records whether the fix preserves existing `init` wizard behavior outside the Step 3 validation path.

## Tasks

### Task 1 - Reproduce And Bound The Failure

#### Subtasks

- T1.1: Trace the exact Step 3 validation call chain from `validateWizardProjectName()` through `knownProjectNames()` and `db.ListProjects()`.
- T1.2: Reproduce the failing condition with a controlled temp `AGENTCOM_HOME` and a deliberately incomplete or legacy DB state.
- T1.3: Confirm whether the failure occurs before migrations, after opening an old DB, or under another setup edge case.
- T1.4: Capture the exact error text and the minimum environment needed to trigger it.

#### Done When

- The team has one concrete reproduction path and knows whether the issue is schema absence, migration timing, or another lookup failure.

### Task 2 - Define Recovery Behavior For Known Project Lookup

#### Subtasks

- T2.1: Decide which lookup failures should be treated as recoverable during Step 3 and which should still block the wizard.
- T2.2: Decide whether recovery should happen in `knownProjectNames()`, `db.ListProjects()`, or a migration/bootstrap layer before validation runs.
- T2.3: Define fallback behavior when the current directory already has `.agentcom.json` and the project name matches the saved value.
- T2.4: Define the user-facing message strategy for duplicate-name conflicts versus non-fatal project-registry lookup degradation.

#### Done When

- Recovery rules are explicit enough that implementation can be minimal and testable.

### Task 3 - Harden The Implementation

#### Subtasks

- T3.1: Update the Step 3 known-project lookup path to handle missing-schema or pre-migration states safely.
- T3.2: If needed, introduce a narrow DB helper to detect the presence/readiness of the `projects` table before querying it.
- T3.3: Ensure duplicate validation still uses the current directory exception path when `.agentcom.json` already matches the candidate name.
- T3.4: Refine wrapped errors so unexpected infrastructure failures remain debuggable without leaking a confusing validation string to the UI.
- T3.5: Keep the rest of the Step 3 instruction/memory/template behavior unchanged.

#### Done When

- The code path tolerates recoverable lookup failures and preserves duplicate checks in healthy environments.

### Task 4 - Expand Automated Coverage

#### Subtasks

- T4.1: Add a test for Step 3 validation when the DB file is absent.
- T4.2: Add a test for Step 3 validation when the DB exists but the `projects` table is unavailable or not migrated.
- T4.3: Add a test confirming duplicate rejection still works with a healthy migrated DB.
- T4.4: Add a test confirming the current project remains allowed when `.agentcom.json` already points to the same name.
- T4.5: Add helper-level DB tests if implementation introduces table-readiness detection or special query handling.

#### Done When

- Tests fail before the fix and pass after the fix for both degraded and healthy states.

### Task 5 - Verify End-To-End Behavior

#### Subtasks

- T5.1: Run targeted tests for `internal/cli` and any touched DB package tests.
- T5.2: Run `go test ./...`.
- T5.3: Run `go build ./...`.
- T5.4: Manually run interactive `agentcom init` against the previously failing Step 3 state.
- T5.5: Verify duplicate-name rejection still appears for an actually known project.
- T5.6: Record the final observed behavior and any residual follow-up work.

#### Done When

- Automated verification passes and manual Step 3 onboarding no longer exposes the raw lookup failure.

### Task 6 - Close The Loop In Project Memory

#### Subtasks

- T6.1: Record the confirmed root cause in `.agents/MEMORY.md`.
- T6.2: Record the chosen recovery rule and why it was selected.
- T6.3: Record the verification commands and manual QA used to validate the fix.
- T6.4: Note any follow-up cleanup if a broader migration/bootstrap issue remains.

#### Done When

- Session memory clearly documents both the bug and the chosen remediation.

## Execution Order

1. Task 1 - Reproduce And Bound The Failure
2. Task 2 - Define Recovery Behavior For Known Project Lookup
3. Task 3 - Harden The Implementation
4. Task 4 - Expand Automated Coverage
5. Task 5 - Verify End-To-End Behavior
6. Task 6 - Close The Loop In Project Memory

## Verification Commands

```bash
go test ./internal/cli/... -count=1
go test ./internal/db/... -count=1
go test ./... -count=1
go build ./...
AGENTCOM_HOME=/tmp/agentcom-step3-repro agentcom init
```

## Risks

- Overly broad fallback behavior could silently allow duplicate project names when the registry is actually readable.
- Fixing the issue only in the CLI layer could mask a more general DB bootstrap/migration problem.
- Manual reproduction may depend on a legacy DB shape that is not currently captured in tests.

## Mitigations

- Limit fallback handling to explicitly identified recoverable lookup states.
- Add tests at both the CLI validation boundary and the DB helper/query boundary.
- Preserve exact duplicate-check coverage in healthy migrated DB scenarios.
