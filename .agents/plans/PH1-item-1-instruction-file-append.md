# PH1 Item 1 - Instruction File Append Logic

## Scope

- Base source: `.agents/plans/PHASE1-critical-fixes.md`
- Target tasks: `PH1-T01-S01` through `PH1-T01-S04`
- Goal: make instruction and memory file generation idempotent on re-init without destroying user content.

## TDD Order

1. Add table-driven tests for marker helpers in `internal/cli/instruction_test.go`.
2. Add failing tests for mode-aware `writeInstructionFile()` behavior.
3. Implement marker constants, helper functions, and `writeMode` support in `internal/cli/instruction.go`.
4. Update callers in `internal/cli/instruction.go`, `internal/cli/init_setup.go`, and `internal/cli/init.go`.
5. Expand integration-style tests for repeated writes and user-content preservation.

## Acceptance Criteria

- New instruction files are written with agentcom markers.
- Existing files without markers keep user content and receive one appended marker block.
- Existing files with markers update in place without duplication.
- Re-running instruction generation is idempotent.
- Current init flows default to append/update behavior.

## Verification

- `go test ./internal/cli/... -run 'TestWrapWithMarkers|TestFindMarkerBounds|TestReplaceMarkerBlock|TestAppendMarkerBlock|TestWriteInstructionFileWithMode|TestWriteAgentInstructions|TestWriteAgentMemoryFiles|TestWriteInstructionPreservesUserContent' -count=1`
