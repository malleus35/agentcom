# PH1 Item 2 - Dynamic Escalation Rendering

## Scope

- Base source: `.agents/plans/PHASE1-critical-fixes.md`
- Target tasks: `PH1-T02-S01` through `PH1-T02-S03`
- Goal: remove self-referential escalation guidance from generated role skills and use concrete sender names.

## TDD Order

1. Add table-driven tests for escalation target selection and escalation line rendering in `internal/cli/agents_test.go`.
2. Add scaffold assertions covering architect/plan self-reference removal and concrete `agentcom send --from ...` output.
3. Implement escalation target helpers in `internal/cli/agents.go`.
4. Refactor `renderRoleSkillContent()` to render communication lines dynamically.
5. Re-run scaffold-focused tests to confirm generated content is stable.

## Acceptance Criteria

- `plan` and `architect` never escalate to themselves.
- Roles prefer `plan` and `architect` when those contacts exist.
- Fallback contacts exclude self and preserve a usable escalation path.
- Generated `agentcom send` examples use the actual role agent name.

## Verification

- `go test ./internal/cli/... -run 'TestComputeEscalationTargets|TestRenderEscalationLine|TestWriteTemplateScaffold' -count=1`
