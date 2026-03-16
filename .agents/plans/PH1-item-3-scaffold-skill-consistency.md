# PH1 Item 3 - Scaffold And Skill Append Consistency

## Scope

- Base source: `.agents/plans/PHASE1-critical-fixes.md`
- Target tasks: `PH1-T03-S01` through `PH1-T05-S02`
- Goal: align scaffold and template skill writes with Phase-1 append semantics, improve guidance/logging, and cover the full re-init flow with tests.

## TDD Order

1. Add failing tests for scaffold re-run behavior in `internal/cli/agents_test.go`.
2. Add failing tests for template skill create-vs-append behavior in `internal/cli/skill_test.go`.
3. Implement mode-aware `writeScaffoldFile()` and `writeSkillFile()` behavior.
4. Add/adjust integration tests for `initSetupExecutor` re-init and template scaffold re-init.
5. Add debug logging assertions and verify file-exists errors remain actionable.

## Acceptance Criteria

- Template scaffold reruns succeed without duplicating generated skill content.
- `COMMON.md` and `template.json` are skipped unless overwrite mode is used later.
- Template-generated skill files append/update via markers while user-created skills still reject existing files.
- File-exists errors include `--force` guidance.
- Verbose mode distinguishes append vs update operations.
- Full init -> re-init flow preserves user content and remains idempotent.

## Verification

- `go test ./internal/cli/... -run 'TestWriteTemplateScaffold|TestTemplateScaffoldReInit|TestWriteSkillFileCreateModeRejectsExisting|TestWriteSkillFileAppendMode|TestInitSetupReInitPreservesContent' -count=1`
