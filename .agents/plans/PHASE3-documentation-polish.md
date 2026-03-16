# Phase 3: Documentation & Polish

> **Phase ID**: PH3
> **Priority**: High
> **Estimated Effort**: 18 hours
> **Prerequisites**: Phase 1 (marker system), Phase 2 (graph validation, enhanced rendering)
> **Branch Strategy**: `feature/PH3-documentation-polish` from `develop`

---

## Table of Contents

1. [Phase Overview](#phase-overview)
2. [PH3-T01: Root agentcom SKILL.md Enhancement](#ph3-t01)
3. [PH3-T02: Role-Specific SKILL.md Enhancement](#ph3-t02)
4. [PH3-T03: COMMON.md Enhancement](#ph3-t03)
5. [PH3-T04: Template Edit Command](#ph3-t04)
6. [PH3-T05: Non-Interactive Template Creation via YAML](#ph3-t05)
7. [Parallelization Map](#parallelization-map)
8. [Inter-Phase Dependencies](#inter-phase-dependencies)

---

## Phase Overview

Phase 3 focuses on documentation quality — the content that AI agents actually read to understand their role. Currently, the root SKILL.md is 10 lines and role SKILL.md files are ~24 lines. Both are too sparse for agents to operate effectively.

### Goals

- Root agentcom SKILL.md expands to 60+ lines with architecture overview, message format spec, and decision flowchart.
- Role SKILL.md files expand to 50+ lines with workflow steps, examples, anti-patterns, and handoff protocol.
- COMMON.md expands to 30+ lines with team-wide standards.
- Templates can be edited post-creation (add/remove roles).
- Templates can be created from YAML files for CI/CD use.

### Files Modified

| File | Changes |
|------|---------|
| `internal/cli/agents.go` | `renderAgentcomSharedSkillContent()`, `renderRoleSkillContent()`, `renderTemplateCommonContent()` |
| `internal/cli/agents_test.go` | Content quality assertions |
| `internal/cli/template_edit.go` | **New file** — template edit subcommands |
| `internal/cli/template_edit_test.go` | **New file** — edit command tests |
| `internal/cli/template_import.go` | **New file** — YAML/JSON import |
| `internal/cli/template_import_test.go` | **New file** — import tests |

---

<a id="ph3-t01"></a>
## PH3-T01: Root agentcom SKILL.md Enhancement

> **Epic**: 5 (Task 5.1)
> **Estimated Effort**: 3 hours
> **Parallel Group**: A (independent)

### Current Content

`renderAgentcomSharedSkillContent()` at `agents.go:339-352` returns ~10 lines:

```markdown
---
name: agentcom
description: Shared agentcom skill instructions for generated template roles
---
# Agentcom
- Use this shared skill as the common base...
- Default template lifecycle: run `agentcom init --template <template>`...
- Use `agentcom register` only as the low-level path...
- Coordinate with `agentcom send`, `agentcom inbox`...
- Read the role-specific skill under this directory...
```

### Expected Content

The shared SKILL.md should serve as a comprehensive reference document that any AI agent can read and immediately understand how to use agentcom. Target: 70+ lines.

### Implementation

#### PH3-T01-S01: Expand renderAgentcomSharedSkillContent with framework overview

**File**: `internal/cli/agents.go`

Replace `renderAgentcomSharedSkillContent()` with an expanded version containing these sections:

```markdown
---
name: agentcom
description: Shared agentcom skill instructions for generated template roles
---

# Agentcom

## Overview

agentcom is a CLI tool for real-time communication between parallel AI coding agent sessions.
It uses SQLite as the durable state store and Unix Domain Sockets for low-latency delivery.
Each agent registers with a unique name and communicates through messages and tasks.

## Lifecycle

1. `agentcom init --template <template>` — scaffold project with template roles
2. `agentcom up` — start all template-defined agents as managed processes
3. Work: send messages, create tasks, coordinate handoffs
4. `agentcom down` — stop managed agents cleanly
5. `agentcom register` — advanced: manually run one standalone agent

## Message Format

Send structured JSON messages between agents:
```
agentcom send --from <you> <target> '{"type":"request","subject":"...","body":"..."}'
```

Standard message types:
- `request` — ask another agent to do something
- `response` — reply to a request with results
- `escalation` — report a blocker that exceeds your role scope
- `report` — broadcast progress or completion status

## Task Lifecycle

1. **Create**: `agentcom task create "<title>" --creator <you> --assign <target> --priority <high|medium|low>`
2. **Accept**: assignee checks inbox with `agentcom inbox --agent <name> --unread`
3. **Progress**: `agentcom task update <id> --status in_progress --result "<note>"`
4. **Complete**: `agentcom task update <id> --status completed --result "<summary>"`
5. **Delegate**: `agentcom task delegate <id> --to <other-agent>` if reassignment needed

## Decision Guide

- **Work independently** when the task is within your role scope and has no cross-role dependencies.
- **Communicate** when your work produces output another role needs, or you need input from another role.
- **Escalate** when you face a blocker outside your scope, an architectural decision is needed, or priorities conflict.

## Quick Reference

| Action | Command |
|--------|---------|
| Send message | `agentcom send --from <you> <target> '<json>'` |
| Broadcast | `agentcom broadcast --from <you> --topic <topic> '<json>'` |
| Check inbox | `agentcom inbox --agent <you> --unread` |
| Create task | `agentcom task create "<title>" --creator <you> --assign <target>` |
| Update task | `agentcom task update <id> --status <status> --result "<note>"` |
| List tasks | `agentcom task list --assignee <you>` |
| Check status | `agentcom status` |

## Role Skills

Read your role-specific skill file for template and responsibility details.
Each role skill is located alongside this file (e.g., `company-frontend/SKILL.md`).
```

**Acceptance Criteria**:
- [x] Content is 70+ lines
- [x] Contains sections: Overview, Lifecycle, Message Format, Task Lifecycle, Decision Guide, Quick Reference
- [x] All CLI examples are syntactically correct
- [x] AI agent can read and understand the agentcom workflow without external context

**Commit**: `docs(cli): PH3-T01-S01 expand shared agentcom SKILL.md with comprehensive framework guide`

---

#### PH3-T01-S02: Tests for enhanced root SKILL.md

**File**: `internal/cli/agents_test.go`

Add content quality test:

```go
func TestSharedSkillContentQuality(t *testing.T) {
    content := renderAgentcomSharedSkillContent()
    lines := strings.Split(content, "\n")
    if len(lines) < 60 {
        t.Fatalf("shared SKILL.md has %d lines, want at least 60", len(lines))
    }
    requiredSections := []string{"## Overview", "## Lifecycle", "## Message Format", "## Task Lifecycle", "## Decision Guide", "## Quick Reference"}
    for _, section := range requiredSections {
        if !strings.Contains(content, section) {
            t.Fatalf("shared SKILL.md missing section: %s", section)
        }
    }
    // No placeholders
    if strings.Contains(content, "<sender>") || strings.Contains(content, "<target>") {
        t.Fatal("shared SKILL.md contains placeholders")
    }
}
```

**Acceptance Criteria**:
- [x] Minimum line count enforced
- [x] Required sections verified
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH3-T01-S02 add content quality tests for shared SKILL.md`

---

<a id="ph3-t02"></a>
## PH3-T02: Role-Specific SKILL.md Enhancement

> **Epic**: 5 (Task 5.2)
> **Estimated Effort**: 6 hours
> **Parallel Group**: A (independent — can parallelize with PH3-T01)

### Current Content

Each role SKILL.md from `renderRoleSkillContent()` is ~24 lines with frontmatter, responsibilities, and a brief communication section.

### Expected Content

Each role SKILL.md should be 55+ lines including:
- Existing: frontmatter, identity, responsibilities, communication contacts
- New: Workflow steps, Examples (2+), Anti-patterns, Handoff Protocol
- Enhanced (from PH2-T02): Collaboration protocol sections

### Implementation

#### PH3-T02-S01: Add role-specific workflow templates

**File**: `internal/cli/agents.go`

Add per-role workflow content to `renderRoleSkillContent()`. Create a map of role workflows:

```go
var roleWorkflows = map[string]string{
    "frontend": `## Workflow

1. Check inbox for design handoffs and task assignments.
2. Review the design direction and clarify ambiguities with design role.
3. Implement UI components, ensuring API contracts match backend specs.
4. Run local verification (build, lint, tests).
5. Send review-ready update to review role with file list and test status.
6. Address review feedback and update task status on completion.`,

    "backend": `## Workflow

1. Check inbox for task assignments and API contract requests.
2. Review requirements and confirm interfaces with frontend.
3. Implement services, schemas, and endpoints.
4. Run local verification (build, lint, tests, migration checks).
5. Notify frontend of available endpoints and any contract changes.
6. Send review-ready update to review role with verification details.`,

    // ... similar for plan, review, architect, design
}
```

For unknown roles (custom templates), generate a generic workflow:

```go
func defaultRoleWorkflow(roleName string) string {
    return fmt.Sprintf(`## Workflow

1. Check inbox for task assignments: ` + "`agentcom inbox --agent %s --unread`" + `
2. Review the assigned task requirements.
3. Execute the work within your role scope.
4. Run verification steps appropriate to your domain.
5. Report completion and send results to the requesting role.`, roleName)
}
```

**Acceptance Criteria**:
- [x] All 6 built-in roles have specific workflow content
- [x] Custom/unknown roles get a generic workflow
- [x] Workflow steps reference actual agentcom commands where appropriate

**Commit**: `docs(cli): PH3-T02-S01 add role-specific workflow templates for SKILL.md generation`

---

#### PH3-T02-S02: Add role-specific examples and anti-patterns

**File**: `internal/cli/agents.go`

Add example scenarios and anti-patterns per role:

```go
var roleExamples = map[string]string{
    "frontend": `## Examples

### Example 1: Receiving a design handoff
Design sends you UI specs. Review them and start implementation:
` + "```" + `
agentcom inbox --agent frontend --unread
agentcom task update <task-id> --status in_progress --result "Starting header component"
` + "```" + `

### Example 2: Requesting an API endpoint
You need a new endpoint from backend:
` + "```" + `
agentcom task create "Add GET /api/users endpoint" --creator frontend --assign backend --priority medium
agentcom send --from frontend backend '{"type":"request","subject":"New API needed","endpoint":"/api/users"}'
` + "```" + `

## Anti-patterns

- Do NOT implement backend logic or database queries — delegate to backend.
- Do NOT skip review — always send review-ready updates before marking tasks complete.
- Do NOT make architectural decisions (e.g., state management strategy) — escalate to architect.`,

    // ... similar for each role
}
```

**Acceptance Criteria**:
- [x] Each built-in role has at least 2 specific examples with CLI commands
- [x] Each built-in role has at least 3 anti-patterns relevant to that role
- [x] Examples use real agentcom commands with realistic parameters

**Commit**: `docs(cli): PH3-T02-S02 add role-specific examples and anti-patterns for SKILL.md`

---

#### PH3-T02-S03: Integrate all sections into renderRoleSkillContent

**File**: `internal/cli/agents.go`

Modify `renderRoleSkillContent()` to append all new sections:

```go
func renderRoleSkillContent(definition templateDefinition, role templateRole, generatedSkillName string, commonPath string) string {
    var sb strings.Builder

    // Frontmatter + header (existing)
    // Responsibilities (existing)
    // Workflow (new — PH3-T02-S01)
    // Examples + Anti-patterns (new — PH3-T02-S02)
    // Communication section (enhanced in PH2-T02)
    // Handoff Protocol (new)

    sb.WriteString("## Handoff Protocol\n\n")
    sb.WriteString("When passing work to another role:\n")
    sb.WriteString("1. Update your current task status to indicate handoff.\n")
    sb.WriteString("2. Create a new task assigned to the target role with clear requirements.\n")
    sb.WriteString("3. Send a direct message with context the target needs.\n")
    sb.WriteString(fmt.Sprintf("4. Monitor inbox for questions: `agentcom inbox --agent %s --unread`\n", role.AgentName))

    return sb.String()
}
```

**Acceptance Criteria**:
- [x] Each generated role SKILL.md is 55+ lines
- [x] Sections appear in logical order: Identity → Responsibilities → Workflow → Examples → Anti-patterns → Communication → Handoff
- [x] All content is role-specific (not generic boilerplate)

**Commit**: `feat(cli): PH3-T02-S03 integrate workflow, examples, and handoff into role SKILL.md rendering`

---

#### PH3-T02-S04: Tests for enhanced role SKILL.md content

**File**: `internal/cli/agents_test.go`

```go
func TestRoleSkillContentQuality(t *testing.T) {
    definitions := builtInTemplateDefinitions()
    for _, def := range definitions {
        for _, role := range def.Roles {
            t.Run(def.Name+"/"+role.Name, func(t *testing.T) {
                content := renderRoleSkillContent(def, role, templateRoleSkillName(def.Name, role.Name), ".agentcom/templates/"+def.Name+"/COMMON.md")
                lines := strings.Split(content, "\n")
                if len(lines) < 50 {
                    t.Fatalf("role %s/%s SKILL.md has %d lines, want at least 50", def.Name, role.Name, len(lines))
                }
                for _, section := range []string{"## Workflow", "## Examples", "## Anti-patterns", "## Communication", "## Handoff Protocol"} {
                    if !strings.Contains(content, section) {
                        t.Fatalf("role %s/%s SKILL.md missing section: %s", def.Name, role.Name, section)
                    }
                }
            })
        }
    }
}
```

**Acceptance Criteria**:
- [x] All 12 role skills (6 × 2 templates) pass quality checks
- [x] Line count, required sections, no placeholders verified
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH3-T02-S04 add content quality tests for all role SKILL.md files`

---

<a id="ph3-t03"></a>
## PH3-T03: COMMON.md Enhancement

> **Epic**: 5 (Task 5.3)
> **Estimated Effort**: 2 hours
> **Parallel Group**: A (independent)

### Current Content

`renderTemplateCommonContent()` at `agents.go:335-337` produces `"# {Title}\n\n{Body}\n"`. The body for `company` template is ~8 lines of guidance.

### Implementation

#### PH3-T03-S01: Expand template CommonBody content

**File**: `internal/cli/agents.go`

Expand the `CommonBody` strings in `builtInTemplateDefinitions()` for both templates.

For `company` (line 411-418), expand to include:
- Team coordination rules (who leads, escalation chain)
- Coding standards (commit convention, branch strategy)
- Communication norms (when to broadcast vs direct message)
- Priority system (how to handle competing priorities)

For `oh-my-opencode` (line 475-482), expand similarly but with OMO-specific guidance:
- Planning-first approach (always consult plan before starting)
- Review gates (review must approve before completion)
- Architecture checkpoints (consult architect for system-boundary changes)

Target: Each CommonBody should be 25+ lines of substantive content.

**Acceptance Criteria**:
- [x] Company COMMON.md is 30+ lines
- [x] Oh-My-OpenCode COMMON.md is 30+ lines
- [x] Content is specific to each template's philosophy (not identical)
- [x] Includes concrete conventions (commit messages, branching)

**Commit**: `docs(cli): PH3-T03-S01 expand COMMON.md content for built-in templates`

---

#### PH3-T03-S02: Tests for COMMON.md quality

**File**: `internal/cli/agents_test.go`

```go
func TestTemplateCommonContentQuality(t *testing.T) {
    for _, def := range builtInTemplateDefinitions() {
        t.Run(def.Name, func(t *testing.T) {
            content := renderTemplateCommonContent(def)
            lines := strings.Split(content, "\n")
            if len(lines) < 25 {
                t.Fatalf("COMMON.md for %s has %d lines, want at least 25", def.Name, len(lines))
            }
        })
    }
}
```

**Commit**: `test(cli): PH3-T03-S02 add COMMON.md content quality tests`

---

<a id="ph3-t04"></a>
## PH3-T04: Template Edit Command

> **Epic**: 4 (Task 4.2)
> **Estimated Effort**: 4 hours
> **Parallel Group**: B (depends on PH2-T03 for role defaults)

### Implementation

#### PH3-T04-S01: Create template edit subcommand structure

**File**: `internal/cli/template_edit.go` (**new file**)

Add `agentcom agents template edit <name> add-role <rolename>` and `remove-role`:

```go
func newTemplateEditCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "edit <template-name>",
        Short: "Edit an existing custom template",
    }
    cmd.AddCommand(newTemplateAddRoleCmd())
    cmd.AddCommand(newTemplateRemoveRoleCmd())
    return cmd
}
```

`add-role`: Load template from disk → generate new role via `generateDefaultRole()` → add to roles → update communication graph → save → regenerate skill files.

`remove-role`: Load template → remove role → update all other roles' `CommunicatesWith` → save → remove orphaned skill files.

**Acceptance Criteria**:
- [x] `agentcom agents template edit my-team add-role devops` adds devops with defaults
- [x] `agentcom agents template edit my-team remove-role design` removes design and cleans up references
- [x] Communication graph stays valid after edit
- [x] Skill files regenerated after edit
- [x] Cannot edit built-in templates (error message)

**Commit**: `feat(cli): PH3-T04-S01 add template edit command for role add/remove`

---

#### PH3-T04-S02: Register edit command and add tests

**File**: `internal/cli/agents.go` — Register `newTemplateEditCmd()` in `newAgentsTemplateCmd()`

**File**: `internal/cli/template_edit_test.go` (**new file**)

```go
func TestTemplateAddRole(t *testing.T) { ... }
func TestTemplateRemoveRole(t *testing.T) { ... }
func TestTemplateEditBuiltInRejected(t *testing.T) { ... }
func TestTemplateEditGraphStaysValid(t *testing.T) { ... }
```

**Commit**: `test(cli): PH3-T04-S02 add template edit command tests`

---

<a id="ph3-t05"></a>
## PH3-T05: Non-Interactive Template Creation via YAML

> **Epic**: 4 (Task 4.3)
> **Estimated Effort**: 3 hours
> **Parallel Group**: B (depends on PH2-T03 for role defaults)

### Implementation

#### PH3-T05-S01: Add YAML template import

**File**: `internal/cli/template_import.go` (**new file**)

Support `agentcom init --from-file template.yaml`:

Minimal YAML:
```yaml
name: my-team
roles: [frontend, backend, plan, review]
```

Full YAML:
```yaml
name: my-team
description: "Custom team for project X"
reference: "internal"
roles:
  - name: frontend
    description: "Custom frontend description"
    agent_name: "fe-agent"
    agent_type: "engineer"
    responsibilities:
      - "Build UI components"
    communicates_with: [backend, review]
```

Implementation:
- Parse YAML using `gopkg.in/yaml.v3` (add dependency)
- For minimal format: use `generateDefaultRole()` from PH2-T03
- For full format: use fields directly, validate with `validateCustomTemplateDefinition()`
- Wire into `newInitCmd()` as `--from-file` flag

**Acceptance Criteria**:
- [x] Minimal YAML with only name + role names creates complete template
- [x] Full YAML with all fields creates exact template
- [x] Invalid YAML produces clear error message
- [x] Works in non-interactive mode (`--batch --from-file`)

**Commit**: `feat(cli): PH3-T05-S01 add YAML-based template import for non-interactive creation`

---

#### PH3-T05-S02: Add --from-file flag and tests

**File**: `internal/cli/init.go`

Add flag:
```go
var fromFile string
cmd.Flags().StringVar(&fromFile, "from-file", "", "Create template from YAML/JSON file")
```

**File**: `internal/cli/template_import_test.go` (**new file**)

Test both minimal and full YAML formats, invalid YAML, and missing file.

**Commit**: `test(cli): PH3-T05-S02 add YAML import flag and tests`

---

## Parallelization Map

```
[PARALLEL GROUP A]  (all independent)
├── PH3-T01-S01: Root SKILL.md content expansion
├── PH3-T01-S02: Root SKILL.md quality tests
├── PH3-T02-S01: Role workflow templates
├── PH3-T02-S02: Role examples and anti-patterns
├── PH3-T02-S03: Integrate into renderRoleSkillContent
├── PH3-T02-S04: Role SKILL.md quality tests
├── PH3-T03-S01: COMMON.md expansion
└── PH3-T03-S02: COMMON.md quality tests

[PARALLEL GROUP B]  (depends on PH2-T03 for role defaults)
├── PH3-T04-S01: Template edit command
├── PH3-T04-S02: Template edit tests
├── PH3-T05-S01: YAML import
└── PH3-T05-S02: YAML import tests
```

## Test Plan Summary

| Test Type | Count | Coverage Target |
|-----------|-------|----------------|
| Content quality tests (shared SKILL) | 1 function, ~6 assertions | Sections, line count, no placeholders |
| Content quality tests (role SKILLs) | 12 role × ~5 assertions | All roles × both templates |
| Content quality tests (COMMON.md) | 2 templates × ~3 assertions | Line count, differentiation |
| Template edit tests | ~4 scenarios | add/remove/reject/graph |
| YAML import tests | ~4 scenarios | minimal/full/invalid/missing |
| **Total new test cases** | **~30** | |
