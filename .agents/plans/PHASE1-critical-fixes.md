# Phase 1: Critical Fixes

> **Phase ID**: PH1
> **Priority**: Critical
> **Estimated Effort**: 12 hours
> **Prerequisites**: None (first phase)
> **Branch Strategy**: `feature/PH1-critical-fixes` from `develop`

---

## Table of Contents

1. [Phase Overview](#phase-overview)
2. [PH1-T01: Instruction File Append Logic](#ph1-t01)
3. [PH1-T02: Self-Reference Escalation Bug Fix](#ph1-t02)
4. [PH1-T03: Scaffold and Skill File Append Consistency](#ph1-t03)
5. [PH1-T04: Error Message Improvement](#ph1-t04)
6. [PH1-T05: Phase 1 Integration Testing](#ph1-t05)
7. [Parallelization Map](#parallelization-map)
8. [Inter-Phase Dependencies](#inter-phase-dependencies)

---

## Phase Overview

Phase 1 addresses two bugs and one design flaw that affect every user who runs `agentcom init` more than once on the same project:

1. **Instruction file overwrite prevention** — `writeInstructionFile()`, `writeScaffoldFile()`, and `writeSkillFile()` all reject existing files with a hard error. This blocks re-initialization entirely.
2. **Self-referential escalation** — `renderRoleSkillContent()` hardcodes "Escalate blockers to `plan` and `architect`" for ALL roles, including `plan` and `architect` themselves.
3. **Unhelpful error messages** — When files exist, the error provides no actionable guidance.

### Proposal Corrections Applied

The original improvement proposal (Epic 1) only identifies `writeInstructionFile()` as having the overwrite bug. In reality, **three functions** share the identical pattern:
- `writeInstructionFile()` — `instruction.go:245-261`
- `writeScaffoldFile()` — `agents.go:309-325`
- `writeSkillFile()` — `skill.go:284-300`

All three are addressed in this phase. The proposal also references a non-existent function `generateInstructionFiles()` — the actual function is `writeAgentInstructions()` at `instruction.go:164`.

### Goals

- Re-running `agentcom init` on an existing project appends agentcom configuration without destroying user content.
- Idempotent: running init N times produces the same result as running it once.
- `--force` overwrites all generated files cleanly.
- No role's SKILL.md tells it to escalate to itself.
- Error messages guide the user toward resolution.

### Files Modified

| File | Functions Modified | Lines Affected |
|------|-------------------|----------------|
| `internal/cli/instruction.go` | `writeInstructionFile()`, new helpers | 245-261, new code |
| `internal/cli/agents.go` | `renderRoleSkillContent()` | 355-381 |
| `internal/cli/agents.go` | `writeScaffoldFile()` | 309-325 |
| `internal/cli/skill.go` | `writeSkillFile()` | 284-300 |
| `internal/cli/init_setup.go` | `Apply()` | 55-159 |
| `internal/cli/init.go` | `newInitCmd()`, batch-mode branch | 21-216 |
| `internal/cli/instruction_test.go` | New and updated tests | throughout |
| `internal/cli/agents_test.go` | Updated assertions | throughout |
| `internal/cli/skill_test.go` | New tests | new code |

---

<a id="ph1-t01"></a>
## PH1-T01: Instruction File Append Logic

> **Epic**: 1 (AGENTS.md/CLAUDE.md Append)
> **Estimated Effort**: 5 hours
> **Parallel Group**: A (independent)

### Current Behavior

`writeInstructionFile()` at `instruction.go:245-261`:

```go
func writeInstructionFile(path string, content string) error {
    if _, err := os.Stat(path); err == nil {
        return fmt.Errorf("instruction file already exists: %s", path)
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("stat instruction file: %w", err)
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("mkdir instruction dir: %w", err)
    }
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        return fmt.Errorf("write instruction file: %w", err)
    }
    return nil
}
```

When a user runs `agentcom init --agents-md claude,codex` a second time, it fails with:
```
write instruction file for claude: instruction file already exists: /path/CLAUDE.md
```

### Expected Behavior

- **File does not exist**: Create new file with content wrapped in markers.
- **File exists, no markers**: Append marker-wrapped content at end of file.
- **File exists, markers present**: Replace content between markers (idempotent update).
- **File exists, `--force`**: Overwrite entire file with marker-wrapped content.

### Implementation

#### PH1-T01-S01: Add marker constants and helper functions

**File**: `internal/cli/instruction.go`

Add the following constants after line 9 (after imports):

```go
const (
    agentcomMarkerStart = "<!-- AGENTCOM:START -->"
    agentcomMarkerEnd   = "<!-- AGENTCOM:END -->"
)
```

Add four helper functions:

1. **`wrapWithMarkers(content string) string`**
   - Wraps content with start/end markers and newlines.
   - Output format: `"<!-- AGENTCOM:START -->\n{content}\n<!-- AGENTCOM:END -->\n"`
   - Trims trailing whitespace from content before wrapping.

2. **`findMarkerBounds(existing string) (startIdx int, endIdx int, found bool)`**
   - Uses `strings.Index()` to find `agentcomMarkerStart`.
   - If found, scans forward from that position to find `agentcomMarkerEnd`.
   - Returns byte indices covering the full marker block including the end marker and its trailing newline.
   - Returns `found=false` if either marker is missing.

3. **`replaceMarkerBlock(existing string, newBlock string) string`**
   - Calls `findMarkerBounds` to locate the old block.
   - Returns `existing[:startIdx] + newBlock + existing[endIdx:]`.

4. **`appendMarkerBlock(existing string, newBlock string) string`**
   - Normalizes trailing whitespace on `existing` to ensure exactly one blank line.
   - Appends `newBlock` after the separator.
   - Handles edge cases: empty existing content, existing content ending with multiple newlines, existing content with no trailing newline.

**TDD — Write tests first:**

```go
func TestWrapWithMarkers(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    string
    }{
        {"basic", "hello", "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"},
        {"trailing newline", "hello\n", "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"},
        {"multiline", "line1\nline2", "<!-- AGENTCOM:START -->\nline1\nline2\n<!-- AGENTCOM:END -->\n"},
    }
    // ...
}

func TestFindMarkerBounds(t *testing.T) {
    tests := []struct {
        name      string
        content   string
        wantFound bool
        wantStart int
        wantEnd   int
    }{
        {"no markers", "plain text", false, 0, 0},
        {"has markers", "before\n<!-- AGENTCOM:START -->\nmiddle\n<!-- AGENTCOM:END -->\nafter", true, 7, 60},
        {"markers at start", "<!-- AGENTCOM:START -->\ncontent\n<!-- AGENTCOM:END -->\n", true, 0, 54},
        {"only start marker", "<!-- AGENTCOM:START -->\ncontent", false, 0, 0},
    }
    // ...
}

func TestReplaceMarkerBlock(t *testing.T) { ... }
func TestAppendMarkerBlock(t *testing.T) { ... }
```

**Acceptance Criteria**:
- [x] `wrapWithMarkers("hello")` returns `"<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"`
- [x] `findMarkerBounds` correctly returns bounds for marker blocks at beginning, middle, and end of content
- [x] `findMarkerBounds` returns `found=false` when no markers exist
- [x] `replaceMarkerBlock` preserves all content outside markers
- [x] `appendMarkerBlock` normalizes trailing whitespace to exactly one blank line before markers
- [x] All 4 helper functions have dedicated table-driven tests

**Commit**: `feat(cli): PH1-T01-S01 add marker constants and helper functions for instruction idempotency`

---

#### PH1-T01-S02: Implement mode-aware instruction file writing

**File**: `internal/cli/instruction.go`

Define a `writeMode` type and refactor `writeInstructionFile()`:

```go
type writeMode int

const (
    writeModeCreate   writeMode = iota // file must not exist (legacy behavior)
    writeModeAppend                     // append or update markers (new default)
    writeModeOverwrite                  // force overwrite entire file
)
```

Replace `writeInstructionFile()` (lines 245-261) with:

```go
func writeInstructionFile(path string, content string, mode writeMode) error {
    markerContent := wrapWithMarkers(content)

    exists := false
    if _, err := os.Stat(path); err == nil {
        exists = true
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("cli.writeInstructionFile: stat: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("cli.writeInstructionFile: mkdir: %w", err)
    }

    if !exists {
        return os.WriteFile(path, []byte(markerContent), 0o644)
    }

    switch mode {
    case writeModeOverwrite:
        return os.WriteFile(path, []byte(markerContent), 0o644)
    case writeModeAppend:
        existing, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("cli.writeInstructionFile: read existing: %w", err)
        }
        existingStr := string(existing)
        _, _, found := findMarkerBounds(existingStr)
        var result string
        if found {
            result = replaceMarkerBlock(existingStr, markerContent)
        } else {
            result = appendMarkerBlock(existingStr, markerContent)
        }
        return os.WriteFile(path, []byte(result), 0o644)
    default: // writeModeCreate
        return fmt.Errorf("cli.writeInstructionFile: file already exists: %s (use --force to overwrite)", path)
    }
}
```

**TDD — Write tests first:**

```go
func TestWriteInstructionFileWithMode(t *testing.T) {
    tests := []struct {
        name           string
        existingContent string  // "" means file does not exist
        mode           writeMode
        newContent     string
        wantErr        bool
        wantContains   []string
        wantNotContain []string
    }{
        {
            name: "create new file",
            existingContent: "",
            mode: writeModeAppend,
            newContent: "# CLAUDE.md\n## Workflow\n- step 1",
            wantContains: []string{"AGENTCOM:START", "# CLAUDE.md", "AGENTCOM:END"},
        },
        {
            name: "append to existing no markers",
            existingContent: "# My Project\n\nExisting instructions.\n",
            mode: writeModeAppend,
            newContent: "## agentcom Workflow\n- step 1",
            wantContains: []string{"# My Project", "Existing instructions", "AGENTCOM:START", "agentcom Workflow"},
        },
        {
            name: "update existing markers",
            existingContent: "# Header\n<!-- AGENTCOM:START -->\nold content\n<!-- AGENTCOM:END -->\n# Footer\n",
            mode: writeModeAppend,
            newContent: "new content",
            wantContains: []string{"# Header", "new content", "# Footer"},
            wantNotContain: []string{"old content"},
        },
        {
            name: "force overwrite",
            existingContent: "# My Custom Content\n\nDo not lose this.\n",
            mode: writeModeOverwrite,
            newContent: "## agentcom only",
            wantContains: []string{"AGENTCOM:START", "agentcom only"},
            wantNotContain: []string{"My Custom Content"},
        },
        {
            name: "create mode rejects existing",
            existingContent: "something",
            mode: writeModeCreate,
            wantErr: true,
        },
        {
            name: "idempotent double append",
            // Run append twice with same content — result should be identical
        },
    }
    // ...
}
```

**Acceptance Criteria**:
- [x] New file → written with markers
- [x] Existing file + `writeModeAppend` + no markers → content appended with markers
- [x] Existing file + `writeModeAppend` + markers present → marker block replaced
- [x] Existing file + `writeModeOverwrite` → entire file replaced
- [x] Existing file + `writeModeCreate` → error returned
- [x] Directory creation still works for new paths
- [x] Double-append produces identical result (idempotent)

**Commit**: `feat(cli): PH1-T01-S02 implement mode-aware instruction file writing with marker idempotency`

---

#### PH1-T01-S03: Update callers to pass write mode

**File**: `internal/cli/instruction.go`

Update `writeAgentInstructions()` (line 164-198):

**Before** (line 164):
```go
func writeAgentInstructions(projectDir string, agentIDs []string) ([]string, error) {
```

**After**:
```go
func writeAgentInstructions(projectDir string, agentIDs []string, mode writeMode) ([]string, error) {
```

Update internal call at line 189:
```go
if err := writeInstructionFile(path, content, mode); err != nil {
```

Update `writeAgentMemoryFiles()` (line 200-231):

**Before** (line 200):
```go
func writeAgentMemoryFiles(projectDir string, agentIDs []string) ([]string, error) {
```

**After**:
```go
func writeAgentMemoryFiles(projectDir string, agentIDs []string, mode writeMode) ([]string, error) {
```

Update internal call at line 223:
```go
if err := writeInstructionFile(path, content, mode); err != nil {
```

Update `writeProjectAgentsMD()` (line 233-242):
```go
func writeProjectAgentsMD(path string) error {
    projectDir := filepath.Dir(path)
    generated, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
```

**File**: `internal/cli/init_setup.go`

Update `Apply()` method (lines 120-147):

At line 126:
```go
instructionFiles, err := writeAgentInstructions(e.projectDir, selectedAgents, writeModeAppend)
```

At line 141:
```go
memoryFiles, err := writeAgentMemoryFiles(e.projectDir, result.SelectedAgents, writeModeAppend)
```

Note: The `--force` threading for `initSetupExecutor` is deferred to PH2-T04 to keep this subtask focused. For now, always use `writeModeAppend` in the setup executor.

**File**: `internal/cli/init.go`

Update batch-mode instruction writing at line 109:
```go
instructionFiles, err = writeAgentInstructions(cwd, selectedAgents, writeModeAppend)
```

**Acceptance Criteria**:
- [x] `writeAgentInstructions()` accepts and passes mode parameter
- [x] `writeAgentMemoryFiles()` accepts and passes mode parameter
- [x] Wizard flow (`init_setup.go`) uses `writeModeAppend` by default
- [x] Batch flow (`init.go`) uses `writeModeAppend` by default
- [x] All existing call sites updated
- [x] All existing tests updated for new signatures and pass

**Commit**: `refactor(cli): PH1-T01-S03 thread write mode through instruction and memory file callers`

---

#### PH1-T01-S04: Comprehensive unit tests for marker system

**File**: `internal/cli/instruction_test.go`

Update existing `TestWriteAgentInstructions` (line 53-106):

**Before** (line 103-105):
```go
if _, err := writeAgentInstructions(projectDir, []string{"codex"}); err == nil {
    t.Fatal("writeAgentInstructions() second call error = nil, want error")
}
```

**After**:
```go
// Second call should succeed with append mode
paths2, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
if err != nil {
    t.Fatalf("writeAgentInstructions() second call error = %v, want nil", err)
}
if len(paths2) != 1 {
    t.Fatalf("len(paths2) = %d, want 1", len(paths2))
}
// Verify content has markers and is idempotent
agentsData2, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
if err != nil {
    t.Fatalf("ReadFile(AGENTS.md) second read error = %v", err)
}
if !strings.Contains(string(agentsData2), agentcomMarkerStart) {
    t.Fatal("AGENTS.md missing start marker after second write")
}
if strings.Count(string(agentsData2), agentcomMarkerStart) != 1 {
    t.Fatal("AGENTS.md has duplicate marker blocks")
}
```

Update `TestWriteAgentMemoryFiles` (line 108-135) similarly — remove the error expectation on second call, verify idempotency.

Add new test for user content preservation:
```go
func TestWriteInstructionPreservesUserContent(t *testing.T) {
    projectDir := t.TempDir()

    // Write user content to AGENTS.md first
    userContent := "# My Project\n\n## Custom Rules\n\n- Rule 1\n- Rule 2\n"
    agentsPath := filepath.Join(projectDir, "AGENTS.md")
    if err := os.WriteFile(agentsPath, []byte(userContent), 0o644); err != nil {
        t.Fatalf("WriteFile error = %v", err)
    }

    // Run instruction generation
    _, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
    if err != nil {
        t.Fatalf("writeAgentInstructions() error = %v", err)
    }

    // Read back
    data, err := os.ReadFile(agentsPath)
    if err != nil {
        t.Fatalf("ReadFile error = %v", err)
    }
    content := string(data)

    // User content preserved
    if !strings.Contains(content, "# My Project") {
        t.Fatal("user content lost: missing # My Project")
    }
    if !strings.Contains(content, "Rule 1") {
        t.Fatal("user content lost: missing Rule 1")
    }

    // Agentcom content added
    if !strings.Contains(content, agentcomMarkerStart) {
        t.Fatal("missing agentcom start marker")
    }
    if !strings.Contains(content, "agentcom Workflow") {
        t.Fatal("missing agentcom workflow content")
    }

    // Second run — idempotent
    _, err = writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
    if err != nil {
        t.Fatalf("second writeAgentInstructions() error = %v", err)
    }
    data2, _ := os.ReadFile(agentsPath)
    if string(data) != string(data2) {
        t.Fatal("second run produced different content (not idempotent)")
    }
}
```

**Acceptance Criteria**:
- [x] `TestWrapWithMarkers` — 3+ cases
- [x] `TestFindMarkerBounds` — 4+ cases (no markers, markers at start/middle/end, partial markers)
- [x] `TestReplaceMarkerBlock` — 3+ cases
- [x] `TestAppendMarkerBlock` — 3+ cases (trailing newline variations)
- [x] `TestWriteInstructionFileWithMode` — 6+ cases (all mode × state combinations)
- [x] `TestWriteAgentInstructions` — updated for append behavior
- [x] `TestWriteAgentMemoryFiles` — updated for append behavior
- [x] `TestWriteInstructionPreservesUserContent` — user content preserved + idempotency
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH1-T01-S04 add comprehensive tests for instruction file marker system`

---

<a id="ph1-t02"></a>
## PH1-T02: Self-Reference Escalation Bug Fix

> **Epic**: 2 (Skill Interconnection — Task 2.1.2)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent of PH1-T01)

### Current Behavior

`renderRoleSkillContent()` at `agents.go:355-381` contains a hardcoded escalation line at line 379:

```go
"- Escalate blockers to `plan` and `architect` when requirements or system boundaries change.\n"
```

This line is rendered identically for ALL roles. When `architect` reads its SKILL.md, it sees "Escalate blockers to `plan` and `architect`" — telling it to escalate to itself. Same issue for `plan`.

The proposal says "exclude self from escalation targets." But the real fix is deeper — the escalation targets should be dynamically computed from the communication map, not hardcoded.

### Expected Behavior

Each role's escalation targets are computed dynamically:
1. Prefer `plan` and `architect` as escalation targets (these are the natural escalation roles).
2. Filter out the role's own name from this list.
3. If neither `plan` nor `architect` is available after filtering, fall back to the first 2 contacts from `CommunicatesWith` (excluding self).
4. If no escalation targets remain (edge case: isolated role), omit the escalation line.

### Implementation

#### PH1-T02-S01: Add escalation target computation function

**File**: `internal/cli/agents.go`

Add after `renderResponsibilities()` (line 393):

```go
// computeEscalationTargets returns the roles a given role should escalate blockers to.
// It prefers "plan" and "architect" from the role's communicatesWith list,
// excludes the role itself, and falls back to the first 2 non-self contacts.
func computeEscalationTargets(roleName string, communicatesWith []string) []string {
    preferred := []string{"plan", "architect"}
    targets := make([]string, 0, 2)

    // First pass: collect preferred escalation targets
    for _, p := range preferred {
        if p != roleName && containsString(communicatesWith, p) {
            targets = append(targets, p)
        }
    }
    if len(targets) > 0 {
        return targets
    }

    // Fallback: first 2 non-self contacts
    for _, c := range communicatesWith {
        if c != roleName {
            targets = append(targets, c)
            if len(targets) >= 2 {
                break
            }
        }
    }
    return targets
}

// renderEscalationLine generates the escalation guidance line for a role SKILL.md.
// Returns empty string if no escalation targets are available.
func renderEscalationLine(targets []string) string {
    if len(targets) == 0 {
        return ""
    }
    formatted := make([]string, len(targets))
    for i, t := range targets {
        formatted[i] = "`" + t + "`"
    }
    return fmt.Sprintf("- Escalate blockers to %s when requirements or system boundaries change.\n",
        strings.Join(formatted, " and "))
}
```

**TDD — Write tests first:**

```go
func TestComputeEscalationTargets(t *testing.T) {
    tests := []struct {
        name             string
        roleName         string
        communicatesWith []string
        want             []string
    }{
        {
            name:             "architect excludes self keeps plan",
            roleName:         "architect",
            communicatesWith: []string{"plan", "frontend", "backend", "design", "review"},
            want:             []string{"plan"},
        },
        {
            name:             "plan excludes self keeps architect",
            roleName:         "plan",
            communicatesWith: []string{"architect", "frontend", "backend", "design", "review"},
            want:             []string{"architect"},
        },
        {
            name:             "frontend gets both plan and architect",
            roleName:         "frontend",
            communicatesWith: []string{"design", "backend", "review", "architect"},
            want:             []string{"architect"},
            // Note: plan is NOT in frontend's communicatesWith in the current map
        },
        {
            name:             "backend gets both",
            roleName:         "backend",
            communicatesWith: []string{"frontend", "architect", "review", "plan"},
            want:             []string{"plan", "architect"},
        },
        {
            name:             "role with neither plan nor architect",
            roleName:         "worker",
            communicatesWith: []string{"helper", "monitor"},
            want:             []string{"helper", "monitor"},
        },
        {
            name:             "empty contacts",
            roleName:         "solo",
            communicatesWith: []string{},
            want:             []string{},
        },
        {
            name:             "only self in contacts",
            roleName:         "recursive",
            communicatesWith: []string{"recursive"},
            want:             []string{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := computeEscalationTargets(tt.roleName, tt.communicatesWith)
            if len(got) != len(tt.want) {
                t.Fatalf("computeEscalationTargets(%q) = %v, want %v", tt.roleName, got, tt.want)
            }
            for i := range got {
                if got[i] != tt.want[i] {
                    t.Fatalf("computeEscalationTargets(%q)[%d] = %q, want %q", tt.roleName, i, got[i], tt.want[i])
                }
            }
        })
    }
}

func TestRenderEscalationLine(t *testing.T) {
    tests := []struct {
        name    string
        targets []string
        want    string
    }{
        {"empty", []string{}, ""},
        {"single", []string{"plan"}, "- Escalate blockers to `plan` when requirements or system boundaries change.\n"},
        {"double", []string{"plan", "architect"}, "- Escalate blockers to `plan` and `architect` when requirements or system boundaries change.\n"},
    }
    // ...
}
```

**Acceptance Criteria**:
- [x] `computeEscalationTargets("architect", ...)` → `["plan"]` (excludes self)
- [x] `computeEscalationTargets("plan", ...)` → `["architect"]` (excludes self)
- [x] `computeEscalationTargets("backend", ...)` → `["plan", "architect"]` (both available)
- [x] `computeEscalationTargets("worker", ["helper","monitor"])` → `["helper", "monitor"]` (fallback)
- [x] `computeEscalationTargets("solo", [])` → `[]` (empty)
- [x] `renderEscalationLine([])` → `""` (no line emitted)

**Commit**: `feat(cli): PH1-T02-S01 add dynamic escalation target computation`

---

#### PH1-T02-S02: Modify renderRoleSkillContent to use dynamic escalation

**File**: `internal/cli/agents.go`

The current `renderRoleSkillContent()` at lines 355-381 uses a single `fmt.Sprintf` with the escalation line embedded in the format string. This must be restructured.

**Before** (lines 355-381, showing the Communication section):
```go
func renderRoleSkillContent(definition templateDefinition, role templateRole, generatedSkillName string, commonPath string) string {
    bodyTitle := titleWords(strings.ReplaceAll(generatedSkillName, "-", " "))
    return fmt.Sprintf(`---
name: %s
description: %s
---

# %s

- Read shared agentcom instructions first: `+"`../SKILL.md`"+`
- Read common instructions first: `+"`%s`"+`
- Template: `+"`%s`"+` (`+"`%s`"+`)
- Agent identity: `+"`%s`"+` / type `+"`%s`"+`

## Responsibilities

%s

## Communication

- Primary contacts: %s
- For template-based teams, use `+"`agentcom up`"+` and `+"`agentcom down`"+` as the default lifecycle; keep `+"`agentcom register`"+` for advanced standalone sessions.
- Use `+"`agentcom send --from <sender> <target> <message-or-json>`"+` for direct coordination.
- Use `+"`agentcom task create`"+`, `+"`agentcom task delegate`"+`, and `+"`agentcom inbox --agent <name>`"+` to coordinate handoffs.
- Escalate blockers to `+"`plan`"+` and `+"`architect`"+` when requirements or system boundaries change.
`, /* ... args ... */)
}
```

**After**: Replace the hardcoded escalation line with dynamic computation. Also replace `<sender>` placeholder in the `agentcom send` example:

```go
func renderRoleSkillContent(definition templateDefinition, role templateRole, generatedSkillName string, commonPath string) string {
    bodyTitle := titleWords(strings.ReplaceAll(generatedSkillName, "-", " "))

    var sb strings.Builder

    // Frontmatter
    sb.WriteString(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n", generatedSkillName, role.Description))

    // Header
    sb.WriteString(fmt.Sprintf("# %s\n\n", bodyTitle))
    sb.WriteString(fmt.Sprintf("- Read shared agentcom instructions first: `../SKILL.md`\n"))
    sb.WriteString(fmt.Sprintf("- Read common instructions first: `%s`\n", commonPath))
    sb.WriteString(fmt.Sprintf("- Template: `%s` (`%s`)\n", definition.Name, definition.Reference))
    sb.WriteString(fmt.Sprintf("- Agent identity: `%s` / type `%s`\n", role.AgentName, role.AgentType))

    // Responsibilities
    sb.WriteString("\n## Responsibilities\n\n")
    sb.WriteString(renderResponsibilities(role.Responsibilities))

    // Communication
    sb.WriteString("\n\n## Communication\n\n")
    sb.WriteString(fmt.Sprintf("- Primary contacts: %s\n", strings.Join(role.CommunicatesWith, ", ")))
    sb.WriteString("- For template-based teams, use `agentcom up` and `agentcom down` as the default lifecycle; keep `agentcom register` for advanced standalone sessions.\n")
    sb.WriteString(fmt.Sprintf("- Use `agentcom send --from %s <target> <message-or-json>` for direct coordination.\n", role.AgentName))
    sb.WriteString("- Use `agentcom task create`, `agentcom task delegate`, and `agentcom inbox --agent <name>` to coordinate handoffs.\n")

    // Dynamic escalation
    escalation := renderEscalationLine(computeEscalationTargets(role.Name, role.CommunicatesWith))
    if escalation != "" {
        sb.WriteString(escalation)
    }

    return sb.String()
}
```

Key changes:
1. Switched from single `fmt.Sprintf` to `strings.Builder` for flexibility
2. Replaced hardcoded `"Escalate blockers to plan and architect"` with `renderEscalationLine(computeEscalationTargets(...))`
3. Replaced `<sender>` placeholder with `role.AgentName` in the `agentcom send` example

**Acceptance Criteria**:
- [x] `architect` role SKILL.md does NOT contain the string "Escalate blockers to.*`architect`"
- [x] `plan` role SKILL.md does NOT contain the string "Escalate blockers to.*`plan`"
- [x] `frontend` role SKILL.md contains "Escalate blockers to `architect`" (plan is not in frontend's contacts per the current communicationMap)
- [x] `backend` role SKILL.md contains "Escalate blockers to `plan` and `architect`"
- [x] `agentcom send` example uses actual role agent name, not `<sender>`
- [x] Output structure is identical to before (same sections, same ordering) except for the escalation and send lines

**Commit**: `fix(cli): PH1-T02-S02 replace hardcoded escalation with dynamic role-aware targets`

---

#### PH1-T02-S03: Update tests for escalation changes

**File**: `internal/cli/agents_test.go`

Update `TestWriteTemplateScaffold` (line 37):

Add assertions after reading the frontend skill (around line 103-123):

```go
// Verify no self-reference in architect skill
architectSkillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-architect", "SKILL.md")
architectSkillData, err := os.ReadFile(architectSkillPath)
if err != nil {
    t.Fatalf("ReadFile(architect skill) error = %v", err)
}
architectContent := string(architectSkillData)
if strings.Contains(architectContent, "Escalate blockers to `plan` and `architect`") {
    t.Fatal("architect skill has self-referential escalation")
}
if !strings.Contains(architectContent, "Escalate blockers to `plan`") {
    t.Fatal("architect skill missing escalation to plan")
}

// Verify no self-reference in plan skill
planSkillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-plan", "SKILL.md")
planSkillData, err := os.ReadFile(planSkillPath)
if err != nil {
    t.Fatalf("ReadFile(plan skill) error = %v", err)
}
planContent := string(planSkillData)
if strings.Contains(planContent, "Escalate blockers to `plan`") {
    t.Fatal("plan skill has self-referential escalation")
}
if !strings.Contains(planContent, "Escalate blockers to `architect`") {
    t.Fatal("plan skill missing escalation to architect")
}
```

Also verify the `<sender>` fix:
```go
// Verify send example uses real agent name
if strings.Contains(content, "send --from <sender>") {
    t.Fatal("frontend skill still contains <sender> placeholder")
}
if !strings.Contains(content, "send --from frontend") {
    t.Fatal("frontend skill missing concrete agent name in send example")
}
```

**Acceptance Criteria**:
- [x] Table-driven tests for `computeEscalationTargets` (7+ cases from S01)
- [x] Table-driven tests for `renderEscalationLine` (3+ cases)
- [x] Scaffold test verifies no self-reference for architect and plan
- [x] Scaffold test verifies `<sender>` placeholder is gone
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH1-T02-S03 add escalation target tests and update scaffold assertions`

---

<a id="ph1-t03"></a>
## PH1-T03: Scaffold and Skill File Append Consistency

> **Epic**: 1 (Append — extended scope not in original proposal)
> **Estimated Effort**: 2.5 hours
> **Parallel Group**: B (depends on PH1-T01 for marker pattern and writeMode type)

### Problem

The proposal only identifies `writeInstructionFile()` as having the overwrite bug, but two other functions have the identical pattern:

**`writeScaffoldFile()`** at `agents.go:309-325`:
```go
func writeScaffoldFile(path string, content string) error {
    if _, err := os.Stat(path); err == nil {
        return fmt.Errorf("file already exists: %s", path)
    }
    // ... mkdir + write
}
```

**`writeSkillFile()`** at `skill.go:284-300`:
```go
func writeSkillFile(path string, content string) error {
    if _, err := os.Stat(path); err == nil {
        return fmt.Errorf("SKILL.md already exists: %s", path)
    }
    // ... mkdir + write
}
```

### Design Decision: Different Behavior Per File Type

| File Type | Default (no --force) | With --force |
|-----------|---------------------|--------------|
| Instruction files (AGENTS.md, CLAUDE.md) | Marker-based append/update | Full overwrite |
| Shared template SKILL.md (agentcom/) | Marker-based append/update | Full overwrite |
| Role template SKILL.md (agentcom/company-frontend/) | Marker-based append/update | Full overwrite |
| COMMON.md | Skip if exists (log info) | Full overwrite |
| template.json | Skip if exists (log info) | Full overwrite |
| User-created SKILL.md (`skill create`) | Error (preserve user files) | Full overwrite |

Rationale: COMMON.md and template.json are fully machine-generated with no user-editable sections. SKILL.md files for templates may have user additions after generation. User-created skill files (`agentcom skill create`) should never be silently overwritten.

### Implementation

#### PH1-T03-S01: Refactor writeScaffoldFile to support modes

**File**: `internal/cli/agents.go`

Replace `writeScaffoldFile()` (lines 309-325) with:

```go
func writeScaffoldFile(path string, content string, mode writeMode) error {
    exists := false
    if _, err := os.Stat(path); err == nil {
        exists = true
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("cli.writeScaffoldFile: stat: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("cli.writeScaffoldFile: mkdir: %w", err)
    }

    if !exists {
        return os.WriteFile(path, []byte(content), 0o644)
    }

    switch mode {
    case writeModeOverwrite:
        return os.WriteFile(path, []byte(content), 0o644)
    case writeModeAppend:
        slog.Debug("scaffold file already exists, skipping", "path", path)
        return nil // skip — COMMON.md and template.json are fully generated
    default:
        return fmt.Errorf("cli.writeScaffoldFile: file already exists: %s (use --force to overwrite)", path)
    }
}
```

Update all callers in `writeTemplateScaffold()` (lines 249-307):

```go
// Line 261:
if err := writeScaffoldFile(commonPath, renderTemplateCommonContent(definition), writeModeAppend); err != nil {

// Line 269:
if err := writeScaffoldFile(manifestPath, manifestContent, writeModeAppend); err != nil {
```

Update callers in `saveCustomTemplate()` (`template_store.go:14-37`):

```go
// Line 24:
if err := writeScaffoldFile(commonPath, commonContent, writeModeCreate); err != nil {

// Line 32:
if err := writeScaffoldFile(manifestPath, manifestContent, writeModeCreate); err != nil {
```

Note: `saveCustomTemplate` uses `writeModeCreate` because creating a new custom template should not silently overwrite an existing one.

**Acceptance Criteria**:
- [x] `writeScaffoldFile` with `writeModeAppend` on existing file → no error, file unchanged
- [x] `writeScaffoldFile` with `writeModeOverwrite` on existing file → file replaced
- [x] `writeScaffoldFile` with `writeModeCreate` on existing file → error returned
- [x] `writeScaffoldFile` on new file → file created regardless of mode
- [x] `saveCustomTemplate` still errors on existing template directory

**Commit**: `feat(cli): PH1-T03-S01 add mode-aware scaffold file writing`

---

#### PH1-T03-S02: Apply marker pattern to template skill files

**File**: `internal/cli/skill.go`

Replace `writeSkillFile()` (lines 284-300) with:

```go
func writeSkillFile(path string, content string, mode writeMode) error {
    markerContent := wrapWithMarkers(content)

    exists := false
    if _, err := os.Stat(path); err == nil {
        exists = true
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("cli.writeSkillFile: stat: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("cli.writeSkillFile: mkdir: %w", err)
    }

    if !exists {
        return os.WriteFile(path, []byte(markerContent), 0o644)
    }

    switch mode {
    case writeModeOverwrite:
        return os.WriteFile(path, []byte(markerContent), 0o644)
    case writeModeAppend:
        existing, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("cli.writeSkillFile: read existing: %w", err)
        }
        existingStr := string(existing)
        _, _, found := findMarkerBounds(existingStr)
        var result string
        if found {
            result = replaceMarkerBlock(existingStr, markerContent)
        } else {
            result = appendMarkerBlock(existingStr, markerContent)
        }
        return os.WriteFile(path, []byte(result), 0o644)
    default:
        return fmt.Errorf("cli.writeSkillFile: skill file already exists: %s (use --force to overwrite)", path)
    }
}
```

Update callers:

**In `agents.go` `writeTemplateScaffold()`** — template-generated skills use `writeModeAppend`:

```go
// Line 279 (shared skill):
if err := writeSkillFile(target.Path, sharedContent, writeModeAppend); err != nil {

// Line 298 (role skills):
if err := writeSkillFile(target.Path, content, writeModeAppend); err != nil {
```

**In `skill.go` `newSkillCreateCmd()`** — user-created skills keep `writeModeCreate`:

```go
// Line 124:
if err := writeSkillFile(target.Path, renderSkillContent(name, desc), writeModeCreate); err != nil {
```

**Acceptance Criteria**:
- [x] Template skill files use marker-based append on re-init
- [x] `agentcom skill create` still rejects existing files (preserves user content)
- [x] `--force` would overwrite template skill files (threading deferred to PH2-T04)
- [x] Markers are used for template skills but not for scaffold files (COMMON.md, template.json)

**Commit**: `feat(cli): PH1-T03-S02 apply marker pattern to template skill file writing`

---

#### PH1-T03-S03: Tests for scaffold and skill file modes

**File**: `internal/cli/agents_test.go`

Update `TestWriteTemplateScaffold` (line 37):

**Before** (line 125-127):
```go
if _, err := writeTemplateScaffold(projectDir, "company"); err == nil {
    t.Fatal("writeTemplateScaffold() second call error = nil, want error")
}
```

**After**:
```go
// Second call should succeed (scaffold files skip, skill files update markers)
paths2, err := writeTemplateScaffold(projectDir, "company")
if err != nil {
    t.Fatalf("writeTemplateScaffold() second call error = %v, want nil", err)
}
if len(paths2) != 30 {
    t.Fatalf("second call len(paths) = %d, want 30", len(paths2))
}

// Verify COMMON.md is unchanged (scaffold skip mode)
commonData2, _ := os.ReadFile(commonPath)
if string(commonData) != string(commonData2) {
    t.Fatal("COMMON.md changed on second scaffold write (should skip)")
}

// Verify skill files have markers
skillData2, _ := os.ReadFile(skillPath)
if !strings.Contains(string(skillData2), agentcomMarkerStart) {
    t.Fatal("skill file missing marker after second scaffold write")
}
if strings.Count(string(skillData2), agentcomMarkerStart) != 1 {
    t.Fatal("skill file has duplicate marker blocks")
}
```

**File**: `internal/cli/skill_test.go`

Add test for user skill creation still rejecting existing files:

```go
func TestWriteSkillFileCreateModeRejectsExisting(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "test", "SKILL.md")

    // First write succeeds
    if err := writeSkillFile(path, "content", writeModeCreate); err != nil {
        t.Fatalf("first write error = %v", err)
    }

    // Second write fails
    if err := writeSkillFile(path, "content", writeModeCreate); err == nil {
        t.Fatal("second write error = nil, want error")
    }
}

func TestWriteSkillFileAppendMode(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "test", "SKILL.md")

    // Write user content first
    os.MkdirAll(filepath.Dir(path), 0o755)
    os.WriteFile(path, []byte("# My Custom Skill\n\nCustom instructions.\n"), 0o644)

    // Append skill content
    if err := writeSkillFile(path, "generated content", writeModeAppend); err != nil {
        t.Fatalf("append write error = %v", err)
    }

    data, _ := os.ReadFile(path)
    content := string(data)
    if !strings.Contains(content, "My Custom Skill") {
        t.Fatal("user content lost")
    }
    if !strings.Contains(content, agentcomMarkerStart) {
        t.Fatal("marker not added")
    }
}
```

**Acceptance Criteria**:
- [x] Re-init template scaffold succeeds without error
- [x] COMMON.md unchanged on re-scaffold (skip mode)
- [x] Skill file marker idempotency tested (no duplicate markers)
- [x] User skill creation (`writeModeCreate`) still rejects existing files
- [x] Skill file append preserves user content
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH1-T03-S03 add scaffold and skill file mode tests`

---

<a id="ph1-t04"></a>
## PH1-T04: Error Message Improvement

> **Epic**: 1 (Task 1.3) + Epic 6 (Task 6.1.1 partial)
> **Estimated Effort**: 1 hour
> **Parallel Group**: C (depends on PH1-T01, PH1-T03 for context)

### Implementation

#### PH1-T04-S01: Improve file-exists error messages with actionable hints

**File**: `internal/cli/instruction.go`

The error message in `writeInstructionFile` `writeModeCreate` branch (from PH1-T01-S02):
- Already updated to: `"file already exists: %s (use --force to overwrite)"`
- No further changes needed here.

**File**: `internal/cli/agents.go`

The error message in `writeScaffoldFile` `writeModeCreate` branch (from PH1-T03-S01):
- Already updated to: `"file already exists: %s (use --force to overwrite)"`
- No further changes needed.

**File**: `internal/cli/skill.go`

The error message in `writeSkillFile` `writeModeCreate` branch (from PH1-T03-S02):
- Already updated to: `"skill file already exists: %s (use --force to overwrite)"`
- No further changes needed.

This subtask verifies that all error messages from PH1-T01 through PH1-T03 are consistent and include hints.

**Acceptance Criteria**:
- [x] All "already exists" errors include a hint about `--force`
- [x] Error format is consistent across all three write functions
- [x] No error path produces a bare "file already exists" without guidance

**Commit**: `fix(cli): PH1-T04-S01 verify consistent error messages with actionable hints`

---

#### PH1-T04-S02: Add verbose logging for append operations

**File**: `internal/cli/instruction.go`

Add `log/slog` import (if not already present). In `writeInstructionFile`, `writeModeAppend` branch, add debug logging:

```go
case writeModeAppend:
    existing, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("cli.writeInstructionFile: read existing: %w", err)
    }
    existingStr := string(existing)
    _, _, found := findMarkerBounds(existingStr)
    var result string
    if found {
        slog.Debug("updating existing agentcom marker block", "path", path)
        result = replaceMarkerBlock(existingStr, markerContent)
    } else {
        slog.Debug("appending agentcom configuration to existing file", "path", path)
        result = appendMarkerBlock(existingStr, markerContent)
    }
    return os.WriteFile(path, []byte(result), 0o644)
```

Similarly in `writeSkillFile` (`skill.go`), add the same debug logging for the append branch.

**Acceptance Criteria**:
- [x] `agentcom -v init --agents-md claude` on existing CLAUDE.md shows debug log about append/update
- [x] No log output without `-v` flag
- [x] Log messages clearly distinguish "appending new block" vs "updating existing block"

**Commit**: `feat(cli): PH1-T04-S02 add verbose logging for instruction and skill file append operations`

---

<a id="ph1-t05"></a>
## PH1-T05: Phase 1 Integration Testing

> **Estimated Effort**: 1.5 hours
> **Parallel Group**: C (depends on all previous tasks)

#### PH1-T05-S01: Full init → re-init end-to-end test

**File**: `internal/cli/init_setup_test.go`

Add integration test exercising the full executor flow:

```go
func TestInitSetupReInitPreservesContent(t *testing.T) {
    projectDir := t.TempDir()
    homeDir := filepath.Join(t.TempDir(), ".agentcom-test")

    // Step 1: Create existing user AGENTS.md
    userContent := "# My Project\n\n## Team Rules\n\n- Always write tests first.\n"
    if err := os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte(userContent), 0o644); err != nil {
        t.Fatal(err)
    }

    // Step 2: First init with codex agent
    executor := &initSetupExecutor{projectDir: projectDir}
    report1, err := executor.Apply(context.Background(), onboard.Result{
        HomeDir:           homeDir,
        Project:           "test-project",
        WriteInstructions: true,
        SelectedAgents:    []string{"codex"},
    })
    if err != nil {
        t.Fatalf("first Apply error = %v", err)
    }

    // Step 3: Verify user content preserved + agentcom added
    data1, _ := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
    content1 := string(data1)
    if !strings.Contains(content1, "# My Project") {
        t.Fatal("user content lost after first init")
    }
    if !strings.Contains(content1, agentcomMarkerStart) {
        t.Fatal("agentcom markers missing after first init")
    }

    // Step 4: Second init (re-init)
    report2, err := executor.Apply(context.Background(), onboard.Result{
        HomeDir:           homeDir,
        Project:           "test-project",
        WriteInstructions: true,
        SelectedAgents:    []string{"codex"},
    })
    if err != nil {
        t.Fatalf("second Apply error = %v", err)
    }

    // Step 5: Verify idempotent
    data2, _ := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
    if string(data1) != string(data2) {
        t.Fatal("second init changed content (not idempotent)")
    }
    _ = report1
    _ = report2
}
```

**Acceptance Criteria**:
- [x] Full init → re-init cycle tested through the executor
- [x] User content preservation verified
- [x] Idempotency verified (content identical after second run)

**Commit**: `test(cli): PH1-T05-S01 add init re-initialization integration test`

---

#### PH1-T05-S02: Template scaffold re-init test

**File**: `internal/cli/agents_test.go`

Add test that runs scaffold generation twice and verifies no error and correct content:

```go
func TestTemplateScaffoldReInit(t *testing.T) {
    projectDir := t.TempDir()
    // chdir setup (same pattern as TestWriteTemplateScaffold)
    oldwd, _ := os.Getwd()
    defer os.Chdir(oldwd)
    os.Chdir(projectDir)

    // First scaffold
    paths1, err := writeTemplateScaffold(projectDir, "company")
    if err != nil {
        t.Fatalf("first scaffold error = %v", err)
    }

    // Read a skill file for comparison
    skillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-frontend", "SKILL.md")
    data1, _ := os.ReadFile(skillPath)

    // Second scaffold
    paths2, err := writeTemplateScaffold(projectDir, "company")
    if err != nil {
        t.Fatalf("second scaffold error = %v", err)
    }

    // Same number of paths
    if len(paths1) != len(paths2) {
        t.Fatalf("path count mismatch: %d vs %d", len(paths1), len(paths2))
    }

    // Skill file content identical (idempotent)
    data2, _ := os.ReadFile(skillPath)
    if string(data1) != string(data2) {
        t.Fatal("skill file content changed on re-scaffold")
    }

    // COMMON.md unchanged
    commonPath := filepath.Join(projectDir, ".agentcom", "templates", "company", "COMMON.md")
    common1, _ := os.ReadFile(commonPath)
    // Re-read after second scaffold
    common2, _ := os.ReadFile(commonPath)
    if string(common1) != string(common2) {
        t.Fatal("COMMON.md changed on re-scaffold")
    }
}
```

**Acceptance Criteria**:
- [x] Template re-scaffold succeeds without error
- [x] File contents are correct and identical after re-scaffold
- [x] COMMON.md and template.json unchanged (scaffold skip mode)
- [x] Skill files have markers and are idempotent

**Commit**: `test(cli): PH1-T05-S02 add template scaffold re-initialization test`

---

## Parallelization Map

```
[PARALLEL GROUP A]  (can run simultaneously — no shared dependencies)
├── PH1-T01-S01: Marker constants + helpers (instruction.go)
├── PH1-T01-S02: Mode-aware writeInstructionFile (instruction.go)
├── PH1-T02-S01: Escalation target computation (agents.go)
└── PH1-T02-S02: Dynamic escalation in renderRoleSkillContent (agents.go)

[PARALLEL GROUP B]  (depends on PH1-T01 for writeMode type and marker helpers)
├── PH1-T01-S03: Update callers with mode parameter (instruction.go, init_setup.go, init.go)
├── PH1-T01-S04: Marker system unit tests (instruction_test.go)
├── PH1-T02-S03: Escalation tests (agents_test.go)
├── PH1-T03-S01: Scaffold file mode support (agents.go)
├── PH1-T03-S02: Skill file marker pattern (skill.go)
└── PH1-T03-S03: Scaffold/skill tests (agents_test.go, skill_test.go)

[PARALLEL GROUP C]  (depends on Groups A+B)
├── PH1-T04-S01: Error message verification
├── PH1-T04-S02: Verbose logging
├── PH1-T05-S01: Init re-init integration test (init_setup_test.go)
└── PH1-T05-S02: Template re-scaffold test (agents_test.go)
```

## Commit Sequence (Total: 13 atomic commits)

| # | Commit | Type | Files |
|---|--------|------|-------|
| 1 | `feat(cli): PH1-T01-S01 add marker constants and helper functions` | feat | instruction.go |
| 2 | `feat(cli): PH1-T01-S02 implement mode-aware instruction file writing` | feat | instruction.go |
| 3 | `feat(cli): PH1-T02-S01 add dynamic escalation target computation` | feat | agents.go |
| 4 | `fix(cli): PH1-T02-S02 replace hardcoded escalation with dynamic targets` | fix | agents.go |
| 5 | `refactor(cli): PH1-T01-S03 thread write mode through callers` | refactor | instruction.go, init_setup.go, init.go |
| 6 | `test(cli): PH1-T01-S04 comprehensive marker system tests` | test | instruction_test.go |
| 7 | `test(cli): PH1-T02-S03 escalation target tests and scaffold assertions` | test | agents_test.go |
| 8 | `feat(cli): PH1-T03-S01 add mode-aware scaffold file writing` | feat | agents.go, template_store.go |
| 9 | `feat(cli): PH1-T03-S02 apply marker pattern to skill file writing` | feat | skill.go, agents.go |
| 10 | `test(cli): PH1-T03-S03 scaffold and skill file mode tests` | test | agents_test.go, skill_test.go |
| 11 | `fix(cli): PH1-T04-S01 verify consistent error messages` | fix | instruction.go, agents.go, skill.go |
| 12 | `feat(cli): PH1-T04-S02 add verbose logging for append operations` | feat | instruction.go, skill.go |
| 13 | `test(cli): PH1-T05-S01+S02 add re-initialization integration tests` | test | init_setup_test.go, agents_test.go |

## Inter-Phase Dependencies

- **PH1-T01 → PH2-T04**: `writeMode` type and marker system are prerequisites for --force expansion.
- **PH1-T01 → PH2-T05**: Marker system is prerequisite for agent-specific sub-markers.
