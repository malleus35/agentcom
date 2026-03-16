# Phase 4: Enhanced UX

> **Phase ID**: PH4
> **Priority**: Medium-High
> **Estimated Effort**: 16 hours
> **Prerequisites**: Phase 1-3 completed
> **Branch Strategy**: `feature/PH4-enhanced-ux` from `develop`

---

## Table of Contents

1. [Phase Overview](#phase-overview)
2. [PH4-T01: Unified Error Message Format](#ph4-t01)
3. [PH4-T02: agentcom doctor Command](#ph4-t02)
4. [PH4-T03: Built-in Template Differentiation](#ph4-t03)
5. [PH4-T04: Template Export/Import](#ph4-t04)
6. [PH4-T05: Status Command Enhancement](#ph4-t05)
7. [PH4-T06: Dry-Run Mode](#ph4-t06)
8. [PH4-T07: Skill Validate Command](#ph4-t07)
9. [Parallelization Map](#parallelization-map)
10. [Inter-Phase Dependencies](#inter-phase-dependencies)

---

## Phase Overview

Phase 4 delivers polish features that improve the day-to-day UX of agentcom. These are non-critical but significantly improve usability and debuggability.

### Goals

- Every user-facing error follows a What/Why/How format.
- `agentcom doctor` provides a comprehensive health check of the project setup.
- Built-in templates have clearly differentiated workflows.
- Templates can be exported/imported for team sharing.
- `agentcom status` shows project name, template, and per-role agent state.
- `agentcom init --dry-run` previews changes without writing files.
- `agentcom skill validate` checks SKILL.md quality against standards.

### Design Decision: Merge doctor + verify

The improvement proposal suggests separate `doctor` (Epic 6) and `verify` (Epic 2) commands. These have ~70% overlap in checks. This plan merges them into a single `agentcom doctor` command with subgroups:
- **Environment checks**: home dir, DB, sockets
- **Project checks**: .agentcom.json, project name, template
- **Communication checks**: graph symmetry, self-reference, orphans
- **Documentation checks**: SKILL.md existence, quality
- **Runtime checks**: agent liveness (post `up`)

---

<a id="ph4-t01"></a>
## PH4-T01: Unified Error Message Format

> **Epic**: 6 (Task 6.1.1)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Implementation

#### PH4-T01-S01: Define error formatting helpers

**File**: `internal/cli/errors.go` (**new file**)

```go
package cli

import "fmt"

type userError struct {
    What string
    Why  string
    How  string
}

func (e *userError) Error() string {
    return fmt.Sprintf("Error: %s\nReason: %s\nHint: %s", e.What, e.Why, e.How)
}

func newUserError(what, why, how string) *userError {
    return &userError{What: what, Why: why, How: how}
}
```

**Acceptance Criteria**:
- [x] Error type provides structured What/Why/How output
- [x] Error implements the `error` interface

**Commit**: `feat(cli): PH4-T01-S01 add structured user error type with what/why/how format`

---

#### PH4-T01-S02: Apply to key error paths

**File**: Multiple CLI files

Progressively replace user-facing errors in the most common paths:

1. `instruction.go` — file exists error (already improved in PH1-T04, now formalize)
2. `init.go` — missing project flag in non-interactive mode
3. `agents.go` — unknown template name
4. `skill.go` — invalid skill name, existing skill file
5. `template_store.go` — template name conflicts

For each, wrap the existing error in `newUserError(what, why, how)`.

**Acceptance Criteria**:
- [x] At least 8 key error paths use structured format
- [x] All hints are actionable (provide an actual command to run)
- [x] JSON output is unaffected (error format applies to human-readable output only)

**Commit**: `refactor(cli): PH4-T01-S02 apply structured error format to key user-facing error paths`

---

#### PH4-T01-S03: Tests for error formatting

**File**: `internal/cli/errors_test.go` (**new file**)

```go
func TestUserErrorFormat(t *testing.T) {
    err := newUserError("Cannot init", "AGENTS.md exists", "Use --force to overwrite")
    got := err.Error()
    if !strings.Contains(got, "Error:") || !strings.Contains(got, "Hint:") {
        t.Fatalf("unexpected format: %s", got)
    }
}
```

**Commit**: `test(cli): PH4-T01-S03 add tests for structured error format`

---

<a id="ph4-t02"></a>
## PH4-T02: agentcom doctor Command

> **Epic**: 6 (Task 6.1.2) + Epic 2 (Task 2.3 — verify merged here)
> **Estimated Effort**: 4 hours
> **Parallel Group**: A (independent)

### Implementation

#### PH4-T02-S01: Create doctor command with check framework

**File**: `internal/cli/doctor.go` (**new file**)

```go
type doctorCheck struct {
    Category string
    Name     string
    Status   string // "pass", "fail", "warn"
    Message  string
    Fix      string // suggested fix command
}

func newDoctorCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "doctor",
        Short: "Diagnose project setup and agent configuration",
        RunE: func(cmd *cobra.Command, args []string) error {
            checks := runDoctorChecks()
            return writeDoctorReport(cmd, checks)
        },
    }
    return cmd
}

func runDoctorChecks() []doctorCheck {
    var checks []doctorCheck
    checks = append(checks, checkEnvironment()...)
    checks = append(checks, checkProjectConfig()...)
    checks = append(checks, checkCommunicationGraph()...)
    checks = append(checks, checkDocumentation()...)
    checks = append(checks, checkRuntime()...)
    return checks
}
```

Check categories:

**Environment**:
- Home directory exists
- SQLite DB exists and is accessible
- Sockets directory exists

**Project**:
- `.agentcom.json` exists
- Project name is non-empty
- Active template is set
- Template files exist on disk

**Communication** (from PH2-T01 `validateCommunicationGraph`):
- Graph is symmetric
- No self-references
- No orphan roles

**Documentation**:
- Shared SKILL.md exists for all 4 agent targets
- Role SKILL.md files exist for all template roles
- COMMON.md exists
- Template manifest (template.json) exists

**Runtime** (only if `.agentcom/run/up.json` exists):
- All managed agents are alive
- No stale PIDs

**Acceptance Criteria**:
- [x] `agentcom doctor` outputs checklist with pass/fail/warn indicators
- [x] Each failed check includes a suggested fix command
- [x] `agentcom --json doctor` outputs structured JSON array of checks
- [x] Command works even when project is not initialized (reports missing items)

**Commit**: `feat(cli): PH4-T02-S01 add agentcom doctor command with comprehensive health checks`

---

#### PH4-T02-S02: Register and test doctor command

**File**: `internal/cli/root.go` — Register `newDoctorCmd()`

**File**: `internal/cli/doctor_test.go` (**new file**)

```go
func TestDoctorOnEmptyProject(t *testing.T) { ... }   // all checks fail gracefully
func TestDoctorOnInitializedProject(t *testing.T) { ... }  // env + project pass
func TestDoctorOnFullTemplate(t *testing.T) { ... }   // all checks pass
func TestDoctorJSONOutput(t *testing.T) { ... }   // JSON format
```

**Acceptance Criteria**:
- [x] Doctor registered in root command
- [x] 4 test scenarios cover empty → partial → full setup
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH4-T02-S02 register doctor command and add tests`

---

<a id="ph4-t03"></a>
## PH4-T03: Built-in Template Differentiation

> **Epic**: 6 (Task 6.2.1)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Problem

Both `company` and `oh-my-opencode` have the same 6 roles with the same communication map. Their differences are only in agent names and descriptions — not in workflow philosophy.

### Implementation

#### PH4-T03-S01: Differentiate template communication maps

**File**: `internal/cli/agents.go`

Give `oh-my-opencode` a different communication topology that reflects its planning-heavy philosophy:

```go
omoCommMap := map[string][]string{
    "frontend":  {"design", "backend", "plan"},          // reports to plan, not directly to architect
    "backend":   {"frontend", "plan", "review"},          // reports to plan
    "plan":      {"architect", "frontend", "backend", "design", "review"},  // hub role
    "review":    {"frontend", "backend", "plan"},          // reports findings to plan
    "architect": {"plan", "review"},                       // advisory, not direct to impl roles
    "design":    {"plan", "frontend"},                     // works through plan
}
```

This reflects OMO's "plan as hub" philosophy where the planner coordinates everything.

Keep `company` template with the existing fully-connected map.

**Acceptance Criteria**:
- [x] `company` uses mesh topology (current — all-to-all with some exclusions)
- [x] `oh-my-opencode` uses hub-spoke topology (plan as central coordinator)
- [x] Both templates pass `validateCommunicationGraph()`
- [x] `agentcom agents template company` vs `oh-my-opencode` show different communication patterns

**Commit**: `feat(cli): PH4-T03-S01 differentiate built-in template communication topologies`

---

#### PH4-T03-S02: Update tests for differentiated templates

**File**: `internal/cli/agents_test.go`

Update `TestResolveTemplateDefinition` and scaffold tests to verify the different maps.

**Commit**: `test(cli): PH4-T03-S02 update tests for differentiated template topologies`

---

<a id="ph4-t04"></a>
## PH4-T04: Template Export/Import

> **Epic**: 6 (Task 6.2.2)
> **Estimated Effort**: 3 hours
> **Parallel Group**: B (depends on PH3-T05 YAML support)

### Implementation

#### PH4-T04-S01: Add template export command

**File**: `internal/cli/agents.go`

Add `agentcom agents template export <name>` subcommand:

```go
func newTemplateExportCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "export <name>",
        Short: "Export a template definition as YAML",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            definition, err := resolveTemplateDefinition(args[0])
            if err != nil {
                return err
            }
            // Marshal to YAML and write to stdout
            data, err := yaml.Marshal(templateToExportFormat(definition))
            if err != nil {
                return fmt.Errorf("cli.newTemplateExportCmd: marshal: %w", err)
            }
            _, err = cmd.OutOrStdout().Write(data)
            return err
        },
    }
    return cmd
}
```

**Acceptance Criteria**:
- [x] `agentcom agents template export company > template.yaml` produces valid YAML
- [x] Exported YAML can be re-imported via `agentcom init --from-file`
- [x] Roundtrip: export → import produces identical template

**Commit**: `feat(cli): PH4-T04-S01 add template export command`

---

#### PH4-T04-S02: Tests for export/import roundtrip

**File**: `internal/cli/agents_test.go`

```go
func TestTemplateExportImportRoundtrip(t *testing.T) {
    // Export company template to YAML
    // Import from that YAML
    // Compare: name, roles, communication maps should match
}
```

**Commit**: `test(cli): PH4-T04-S02 add template export/import roundtrip test`

---

<a id="ph4-t05"></a>
## PH4-T05: Status Command Enhancement

> **Epic**: 6 (Task 6.3)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Implementation

#### PH4-T05-S01: Add project and template info to status output

**File**: `internal/cli/status.go` (or wherever the status command is defined)

Add to status output:
- Project name (from `.agentcom.json`)
- Active template name
- Per-role agent status when `up.json` exists (alive/stopped)
- Unread messages grouped by recipient agent

**Acceptance Criteria**:
- [x] `agentcom status` shows project name when configured
- [x] `agentcom status` shows active template when set
- [x] `agentcom --json status` includes project/template fields

**Commit**: `feat(cli): PH4-T05-S01 enhance status command with project and template info`

---

#### PH4-T05-S02: Tests for enhanced status

**Commit**: `test(cli): PH4-T05-S02 add tests for enhanced status output`

---

<a id="ph4-t06"></a>
## PH4-T06: Dry-Run Mode

> **Epic**: 1 (Task 1.3.2)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Implementation

#### PH4-T06-S01: Add --dry-run flag to init command

**File**: `internal/cli/init.go`

```go
var dryRun bool
cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
```

When `--dry-run`:
1. Compute all paths that would be written (same logic as normal init)
2. For each path, check if file exists
3. Report: "Would create: /path/to/CLAUDE.md" or "Would update: /path/to/AGENTS.md (marker block)" or "Would overwrite: /path/to/AGENTS.md (--force)"
4. Do NOT write any files
5. Exit with status 0

**File**: `internal/cli/init_setup.go`

Add `dryRun` field to `initSetupExecutor`. When true, skip all write operations but still compute and collect paths for reporting.

**Acceptance Criteria**:
- [x] `agentcom init --dry-run --agents-md claude,codex --template company` lists all files that would be created/updated
- [x] No files are written to disk
- [x] Output distinguishes create vs update vs overwrite actions
- [x] `--json --dry-run` returns structured preview

**Commit**: `feat(cli): PH4-T06-S01 add --dry-run flag for init preview mode`

---

#### PH4-T06-S02: Tests for dry-run mode

**File**: `internal/cli/init_setup_test.go`

```go
func TestInitDryRunNoFileWrites(t *testing.T) {
    // Set up project dir with existing AGENTS.md
    // Run Apply with dryRun=true
    // Verify AGENTS.md is unchanged
    // Verify report lists expected actions
}
```

**Commit**: `test(cli): PH4-T06-S02 add dry-run mode tests`

---

<a id="ph4-t07"></a>
## PH4-T07: Skill Validate Command

> **Epic**: 5 (Task 5.4)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Implementation

#### PH4-T07-S01: Add agentcom skill validate command

**File**: `internal/cli/skill.go`

Add `agentcom skill validate` subcommand:

```go
func newSkillValidateCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "validate",
        Short: "Validate skill documentation quality",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Find all SKILL.md files in project
            // Check each for:
            //   - Minimum line count (shared: 60, role: 50)
            //   - Required sections
            //   - No placeholder strings
            //   - CLI command examples present
            // Report results
        },
    }
    return cmd
}
```

Validation checks:
1. **Line count**: shared SKILL.md ≥ 60, role SKILL.md ≥ 50
2. **Required sections**: Check for `## Communication`, `## Workflow` (role only), `## Lifecycle` (shared only)
3. **Placeholder absence**: No `<sender>`, `<target>`, `<name>` in generated content
4. **CLI examples**: At least one `agentcom` command present

**Acceptance Criteria**:
- [x] `agentcom skill validate` reports pass/fail per skill file
- [x] `agentcom --json skill validate` returns structured report
- [x] Validation works on both built-in and custom template skills

**Commit**: `feat(cli): PH4-T07-S01 add skill validate command for documentation quality checks`

---

#### PH4-T07-S02: Register and test skill validate

**File**: `internal/cli/skill.go` — Register `newSkillValidateCmd()`

**File**: `internal/cli/skill_test.go`

```go
func TestSkillValidatePassesAfterPhase3(t *testing.T) {
    // Generate template scaffold
    // Run validation
    // All checks should pass
}

func TestSkillValidateDetectsLowQuality(t *testing.T) {
    // Write a minimal SKILL.md (< 50 lines)
    // Run validation
    // Should report failure
}
```

**Commit**: `test(cli): PH4-T07-S02 register skill validate and add tests`

---

## Parallelization Map

```
[PARALLEL GROUP A]  (all independent — maximum parallelization)
├── PH4-T01-S01: Error type definition
├── PH4-T01-S02: Error format application
├── PH4-T01-S03: Error format tests
├── PH4-T02-S01: Doctor command framework
├── PH4-T02-S02: Doctor tests
├── PH4-T03-S01: Template differentiation
├── PH4-T03-S02: Differentiation tests
├── PH4-T05-S01: Status enhancement
├── PH4-T05-S02: Status tests
├── PH4-T06-S01: Dry-run flag
├── PH4-T06-S02: Dry-run tests
├── PH4-T07-S01: Skill validate command
└── PH4-T07-S02: Skill validate tests

[PARALLEL GROUP B]  (depends on PH3-T05 YAML support)
├── PH4-T04-S01: Template export
└── PH4-T04-S02: Export/import roundtrip tests
```

## Inter-Phase Dependencies

```
Phase 1 ─── marker system ──────────────> Phase 2 (--force, agent branching)
                                           Phase 4 (dry-run understands modes)
Phase 1 ─── escalation fix ─────────────> Phase 2 (enhanced communication)

Phase 2 ─── graph validation ───────────> Phase 4 (doctor uses it)
Phase 2 ─── role defaults ─────────────-> Phase 3 (template edit, YAML import)
Phase 2 ─── simplified wizard ──────────> Phase 3 (template edit builds on it)

Phase 3 ─── YAML import ───────────────-> Phase 4 (export/import roundtrip)
Phase 3 ─── SKILL.md quality ──────────-> Phase 4 (skill validate checks it)
Phase 3 ─── COMMON.md quality ─────────-> Phase 4 (doctor checks it)
```

## Full Dependency Graph

```
PH1-T01 (markers) ─┬─> PH1-T03 (scaffold/skill append)
                    ├─> PH2-T04 (--force expansion)
                    ├─> PH2-T05 (agent branching)
                    └─> PH4-T06 (dry-run)

PH1-T02 (escalation) ──> PH2-T02 (communication sections)

PH2-T01 (graph validation) ─┬─> PH2-T02 (validates contacts before rendering)
                             └─> PH4-T02 (doctor uses graph validation)

PH2-T03 (wizard simplification) ─┬─> PH3-T04 (template edit uses defaults)
                                  └─> PH3-T05 (YAML import uses defaults)

PH3-T01+T02+T03 (SKILL/COMMON quality) ──> PH4-T07 (validate checks quality)

PH3-T05 (YAML import) ──> PH4-T04 (export/import roundtrip)
```

## Test Plan Summary

| Test Type | Count | Coverage Target |
|-----------|-------|----------------|
| Error format tests | ~3 cases | Error type output |
| Doctor checks | ~4 scenarios | Empty → full setup |
| Template differentiation | ~3 cases | Topology differences |
| Export/import | ~2 cases | Roundtrip integrity |
| Status enhancement | ~3 cases | Project/template display |
| Dry-run | ~2 cases | No writes, correct preview |
| Skill validate | ~3 cases | Pass/fail detection |
| **Total new test cases** | **~20** | |

## New Files Summary (Phase 4)

| File | Purpose |
|------|---------|
| `internal/cli/errors.go` | Structured error type |
| `internal/cli/errors_test.go` | Error format tests |
| `internal/cli/doctor.go` | Doctor command |
| `internal/cli/doctor_test.go` | Doctor tests |
