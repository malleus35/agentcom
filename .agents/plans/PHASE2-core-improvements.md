# Phase 2: Core Improvements

> **Phase ID**: PH2
> **Priority**: High
> **Estimated Effort**: 22 hours
> **Prerequisites**: Phase 1 completed (marker system, escalation fix)
> **Branch Strategy**: `feature/PH2-core-improvements` from `develop`

---

## Table of Contents

1. [Phase Overview](#phase-overview)
2. [PH2-T01: Communication Graph Validation](#ph2-t01)
3. [PH2-T02: SKILL.md Communication Section Enhancement](#ph2-t02)
4. [PH2-T03: Custom Template Wizard Simplification](#ph2-t03)
5. [PH2-T04: --force Flag Expansion](#ph2-t04)
6. [PH2-T05: Agent-Specific File Branching for Shared Paths](#ph2-t05)
7. [Parallelization Map](#parallelization-map)
8. [Inter-Phase Dependencies](#inter-phase-dependencies)

---

## Phase Overview

Phase 2 builds on the Phase 1 foundation to deliver structural improvements to the template system, communication validation, and custom template creation UX.

### Goals

- Communication graphs are validated for symmetry, self-reference, and orphan roles at build time and load time.
- Each role SKILL.md contains concrete communication examples with real role names — no placeholders.
- Custom template creation requires only 2 inputs (name + role list) instead of 47 fields.
- `--force` flag applies consistently across all file types.
- Multiple agents sharing the same file path (e.g., codex/opencode/amp/devin → AGENTS.md) coexist via agent-specific marker subsections.

### Files Modified

| File | Changes |
|------|---------|
| `internal/cli/agents.go` | Graph validation, enhanced SKILL.md rendering |
| `internal/cli/agents_test.go` | Validation tests, rendering tests |
| `internal/cli/role_defaults.go` | **New file** — default metadata for role names |
| `internal/cli/role_defaults_test.go` | **New file** — tests for role defaults |
| `internal/cli/init_prompter.go` | Simplified wizard flow, --advanced mode |
| `internal/cli/init_prompter_test.go` | Updated wizard tests |
| `internal/cli/init.go` | --advanced flag, --force threading |
| `internal/cli/init_setup.go` | Force mode threading |
| `internal/cli/instruction.go` | Agent-specific sub-markers |
| `internal/cli/instruction_test.go` | Sub-marker tests |

---

<a id="ph2-t01"></a>
## PH2-T01: Communication Graph Validation

> **Epic**: 2 (Skill Interconnection — Tasks 2.1.1, 2.1.3)
> **Estimated Effort**: 4 hours
> **Parallel Group**: A (independent)

### Current Behavior

`communicationMap` at `agents.go:396-403` is a static `map[string][]string`. No runtime or build-time validation checks whether:
- The graph is symmetric (A→B implies B→A)
- Any role references itself in `CommunicatesWith`
- Any role is orphaned (not referenced by any other role)

Custom templates loaded from disk (`loadCustomTemplates`) also skip these checks.

### Expected Behavior

A `validateCommunicationGraph()` function checks all three conditions. Validation runs:
1. When `builtInTemplateDefinitions()` constructs templates (caught at compile/test time)
2. When `loadCustomTemplates()` loads from disk (caught at init time)
3. When `validateCustomTemplateDefinition()` validates user input (caught at wizard time)

### Implementation

#### PH2-T01-S01: Define validation types and core function

**File**: `internal/cli/agents.go` (add after `renderResponsibilities`, before `builtInTemplateDefinitions`)

```go
type graphIssue struct {
    Severity string // "error" or "warning"
    Role     string
    Message  string
}

func validateCommunicationGraph(roles []templateRole) []graphIssue {
    issues := make([]graphIssue, 0)
    roleNames := make(map[string]struct{}, len(roles))
    commMap := make(map[string][]string, len(roles))
    for _, role := range roles {
        roleNames[role.Name] = struct{}{}
        commMap[role.Name] = role.CommunicatesWith
    }

    for _, role := range roles {
        // Check 1: Self-reference
        for _, target := range role.CommunicatesWith {
            if target == role.Name {
                issues = append(issues, graphIssue{
                    Severity: "error",
                    Role:     role.Name,
                    Message:  fmt.Sprintf("role %q lists itself in CommunicatesWith", role.Name),
                })
            }
        }

        // Check 2: Symmetry
        for _, target := range role.CommunicatesWith {
            if _, exists := roleNames[target]; !exists {
                issues = append(issues, graphIssue{
                    Severity: "error",
                    Role:     role.Name,
                    Message:  fmt.Sprintf("role %q references unknown role %q", role.Name, target),
                })
                continue
            }
            if !containsString(commMap[target], role.Name) {
                issues = append(issues, graphIssue{
                    Severity: "warning",
                    Role:     role.Name,
                    Message:  fmt.Sprintf("asymmetric: %q→%q exists but %q→%q missing", role.Name, target, target, role.Name),
                })
            }
        }
    }

    // Check 3: Orphan detection
    referenced := make(map[string]struct{})
    for _, role := range roles {
        for _, target := range role.CommunicatesWith {
            referenced[target] = struct{}{}
        }
    }
    for _, role := range roles {
        if _, ok := referenced[role.Name]; !ok {
            if len(role.CommunicatesWith) == 0 {
                issues = append(issues, graphIssue{
                    Severity: "warning",
                    Role:     role.Name,
                    Message:  fmt.Sprintf("role %q is isolated: no incoming or outgoing connections", role.Name),
                })
            }
        }
    }

    return issues
}

func hasGraphErrors(issues []graphIssue) bool {
    for _, issue := range issues {
        if issue.Severity == "error" {
            return true
        }
    }
    return false
}
```

**Acceptance Criteria**:
- [x] Symmetric graph passes with zero issues
- [x] Self-reference detected as error
- [x] Unknown role reference detected as error
- [x] Asymmetric edge detected as warning
- [x] Isolated role detected as warning

**Commit**: `feat(cli): PH2-T01-S01 add communication graph validation function`

---

#### PH2-T01-S02: Integrate validation into template loading

**File**: `internal/cli/agents.go`

In `builtInTemplateDefinitions()`, add validation after constructing each template definition:

```go
// At end of builtInTemplateDefinitions(), before return:
for _, def := range definitions {
    if issues := validateCommunicationGraph(def.Roles); hasGraphErrors(issues) {
        // This should never happen for built-in templates.
        // Panic in dev, log in production.
        for _, issue := range issues {
            slog.Warn("built-in template graph issue",
                "template", def.Name, "severity", issue.Severity,
                "role", issue.Role, "message", issue.Message)
        }
    }
}
```

**File**: `internal/cli/template_store.go`

In `validateCustomTemplateDefinition()` (line 107-143), add after role validation:

```go
// After the role validation loop (line 141):
if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
    msgs := make([]string, 0)
    for _, issue := range issues {
        if issue.Severity == "error" {
            msgs = append(msgs, issue.Message)
        }
    }
    return fmt.Errorf("communication graph errors: %s", strings.Join(msgs, "; "))
}
```

**Acceptance Criteria**:
- [x] Built-in templates log warnings for any graph issues (should be none currently)
- [x] Custom template validation rejects templates with graph errors
- [x] Custom template validation allows templates with only warnings

**Commit**: `feat(cli): PH2-T01-S02 integrate graph validation into template loading and custom validation`

---

#### PH2-T01-S03: Tests for communication graph validation

**File**: `internal/cli/agents_test.go`

```go
func TestValidateCommunicationGraph(t *testing.T) {
    tests := []struct {
        name       string
        roles      []templateRole
        wantErrors int
        wantWarns  int
    }{
        {
            name: "symmetric graph passes",
            roles: []templateRole{
                {Name: "a", CommunicatesWith: []string{"b"}},
                {Name: "b", CommunicatesWith: []string{"a"}},
            },
            wantErrors: 0, wantWarns: 0,
        },
        {
            name: "self reference detected",
            roles: []templateRole{
                {Name: "a", CommunicatesWith: []string{"a", "b"}},
                {Name: "b", CommunicatesWith: []string{"a"}},
            },
            wantErrors: 1, wantWarns: 0,
        },
        {
            name: "asymmetric edge warned",
            roles: []templateRole{
                {Name: "a", CommunicatesWith: []string{"b"}},
                {Name: "b", CommunicatesWith: []string{}},
            },
            wantErrors: 0, wantWarns: 1,
        },
        {
            name: "unknown role reference",
            roles: []templateRole{
                {Name: "a", CommunicatesWith: []string{"nonexistent"}},
            },
            wantErrors: 1, wantWarns: 0,
        },
        {
            name: "isolated role warned",
            roles: []templateRole{
                {Name: "a", CommunicatesWith: []string{"b"}},
                {Name: "b", CommunicatesWith: []string{"a"}},
                {Name: "c", CommunicatesWith: []string{}},
            },
            wantErrors: 0, wantWarns: 1,
        },
        {
            name: "built-in company template passes",
            // use roles from builtInTemplateDefinitions()[0]
        },
        {
            name: "built-in oh-my-opencode template passes",
            // use roles from builtInTemplateDefinitions()[1]
        },
    }
    // ... standard table-driven execution
}
```

**Acceptance Criteria**:
- [x] All 7+ test cases pass
- [x] Both built-in templates validated in test
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH2-T01-S03 add communication graph validation tests`

---

<a id="ph2-t02"></a>
## PH2-T02: SKILL.md Communication Section Enhancement

> **Epic**: 2 (Task 2.2)
> **Estimated Effort**: 4 hours
> **Parallel Group**: B (depends on PH1-T02 for escalation pattern, PH2-T01 for validation)

### Current Behavior

`renderRoleSkillContent()` at `agents.go:355-381` generates a Communication section with:
- `Primary contacts: design, backend, review, architect` (role names, correct)
- `agentcom send --from <sender> <target> <message-or-json>` (**placeholder** `<sender>`)
- Hardcoded escalation (fixed in PH1-T02)

### Expected Behavior

- `<sender>` replaced with actual `role.AgentName`
- Collaboration protocol subsections: Request, Response, Escalation, Report
- Concrete CLI command examples using real role names
- Each contact listed with their role description

### Implementation

#### PH2-T02-S01: Build contact detail renderer

**File**: `internal/cli/agents.go`

Add function to render detailed contact list:

```go
func renderContactDetails(role templateRole, allRoles []templateRole) string {
    var sb strings.Builder
    for _, contactName := range role.CommunicatesWith {
        for _, other := range allRoles {
            if other.Name == contactName {
                sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", other.Name, other.AgentName, other.Description))
                break
            }
        }
    }
    return sb.String()
}
```

**Acceptance Criteria**:
- [x] Each contact shows role name, agent name, and description
- [x] Only roles in `CommunicatesWith` are listed

**Commit**: `feat(cli): PH2-T02-S01 add contact detail renderer for role SKILL.md`

---

#### PH2-T02-S02: Add collaboration protocol sections

**File**: `internal/cli/agents.go`

Add function to render protocol examples:

```go
func renderCollaborationProtocol(role templateRole) string {
    agentName := role.AgentName
    var sb strings.Builder

    sb.WriteString("### Request\n\n")
    sb.WriteString(fmt.Sprintf("When you need work from another role, create a task:\n"))
    sb.WriteString(fmt.Sprintf("```\nagentcom task create \"<description>\" --creator %s --assign <target-role> --priority medium\n```\n\n", agentName))

    sb.WriteString("### Response\n\n")
    sb.WriteString(fmt.Sprintf("When completing a task assigned to you, update status and notify:\n"))
    sb.WriteString(fmt.Sprintf("```\nagentcom task update <task-id> --status completed --result \"<summary>\"\nagentcom send --from %s <requester> '{\"type\":\"response\",\"task_id\":\"<id>\",\"status\":\"completed\"}'\n```\n\n", agentName))

    sb.WriteString("### Escalation\n\n")
    targets := computeEscalationTargets(role.Name, role.CommunicatesWith)
    if len(targets) > 0 {
        sb.WriteString(fmt.Sprintf("When blocked or when decisions exceed your role scope, escalate to %s:\n",
            strings.Join(targets, " or ")))
        sb.WriteString(fmt.Sprintf("```\nagentcom send --from %s %s '{\"type\":\"escalation\",\"blocker\":\"<description>\"}'\n```\n\n",
            agentName, targets[0]))
    } else {
        sb.WriteString("No escalation targets defined for this role. Resolve blockers independently or broadcast for help.\n\n")
    }

    sb.WriteString("### Report\n\n")
    sb.WriteString(fmt.Sprintf("Broadcast progress updates to the team:\n"))
    sb.WriteString(fmt.Sprintf("```\nagentcom broadcast --from %s --topic progress '{\"status\":\"in_progress\",\"summary\":\"<what-you-did>\"}'\n```\n", agentName))

    return sb.String()
}
```

**Acceptance Criteria**:
- [x] All 4 protocol sections present in each role SKILL.md
- [x] CLI examples use real agent names (not placeholders)
- [x] Escalation section adapts per role (uses PH1-T02 logic)

**Commit**: `feat(cli): PH2-T02-S02 add collaboration protocol sections to role SKILL.md`

---

#### PH2-T02-S03: Integrate enhanced sections into renderRoleSkillContent

**File**: `internal/cli/agents.go`

Modify `renderRoleSkillContent()` to include the new sections. Update the signature to accept all roles for contact detail rendering:

```go
func renderRoleSkillContent(definition templateDefinition, role templateRole, generatedSkillName string, commonPath string) string {
```

In the Communication section, replace the current flat list with:

```
## Communication

### Primary Contacts

[output of renderContactDetails]

### Coordination Commands

- Direct message: `agentcom send --from {agentName} <target> <message-or-json>`
- Check inbox: `agentcom inbox --agent {agentName} --unread`
- For template-based teams, use `agentcom up` and `agentcom down` as the default lifecycle.

[output of renderCollaborationProtocol]

[output of renderEscalationLine — already from PH1-T02]
```

**Acceptance Criteria**:
- [x] No `<sender>` or `<target>` placeholders in any generated SKILL.md
- [x] Each role SKILL.md uses its actual agent name in examples
- [x] Contact details include descriptions from the template definition
- [x] Line count per role SKILL.md increases from ~24 to ~45+

**Commit**: `feat(cli): PH2-T02-S03 integrate enhanced communication sections into role skill rendering`

---

#### PH2-T02-S04: Tests for enhanced communication sections

**File**: `internal/cli/agents_test.go`

Update `TestWriteTemplateScaffold`:
- Assert no `<sender>` placeholder in any generated skill file
- Assert no `<target>` placeholder in any generated skill file
- Assert presence of "### Request", "### Response", "### Escalation", "### Report"
- Assert `agentcom send --from frontend` (actual agent name) in frontend skill
- Assert `agentcom send --from prometheus` in oh-my-opencode plan skill

Add:
```go
func TestRenderContactDetails(t *testing.T) { ... }
func TestRenderCollaborationProtocol(t *testing.T) { ... }
```

**Acceptance Criteria**:
- [x] Placeholder absence verified across all generated files
- [x] Protocol sections verified in at least 2 different role skills
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH2-T02-S04 add tests for enhanced SKILL.md communication sections`

---

<a id="ph2-t03"></a>
## PH2-T03: Custom Template Wizard Simplification

> **Epic**: 4 (Custom Template — Task 4.1)
> **Estimated Effort**: 6 hours
> **Parallel Group**: A (independent)

### Current Behavior

`runCustomTemplateWizard()` at `init_prompter.go:275-368` requires 47 fields for a 6-role template:
- Form 1: template name, description, reference (3 fields)
- Form 2 (per role): role name, description, agent name, agent type, responsibilities CSV, communicates_with CSV, add another? (7 × N fields)
- Form 3: common title, common body (2 fields)

### Expected Behavior

Default wizard requires only 2 inputs:
1. Template name (or auto-generated from first role name)
2. Comma-separated role names (e.g., "frontend, backend, plan, review")

Everything else is auto-generated from a role defaults map. The existing detailed wizard is preserved as `--advanced` mode.

### Implementation

#### PH2-T03-S01: Create role_defaults.go with default metadata map

**File**: `internal/cli/role_defaults.go` (**new file**)

```go
package cli

type roleMetadata struct {
    Description      string
    AgentNameSuffix  string // appended to template agent naming
    AgentType        string
    Responsibilities []string
}

var knownRoleDefaults = map[string]roleMetadata{
    "frontend": {
        Description:      "Frontend implementation specialist for UI delivery and design handoff.",
        AgentNameSuffix:  "frontend",
        AgentType:        "engineer-frontend",
        Responsibilities: []string{
            "Implement UI components and pages from design direction.",
            "Coordinate API contracts with backend.",
            "Send review-ready updates with file and state summaries.",
        },
    },
    "backend": {
        Description:      "Backend implementation specialist for APIs, services, and data flows.",
        AgentNameSuffix:  "backend",
        AgentType:        "engineer-backend",
        Responsibilities: []string{
            "Implement services, schemas, and API endpoints.",
            "Confirm payload contracts with frontend.",
            "Escalate system risks and migration needs.",
        },
    },
    "plan": {
        Description:      "Planning specialist for task breakdown, sequencing, and coordination.",
        AgentNameSuffix:  "planner",
        AgentType:        "pm",
        Responsibilities: []string{
            "Turn requests into deliverable task breakdowns.",
            "Coordinate handoffs between execution roles.",
            "Track blockers and completion signals across the team.",
        },
    },
    "review": {
        Description:      "Review specialist for QA, regression checks, and feedback loops.",
        AgentNameSuffix:  "reviewer",
        AgentType:        "qa",
        Responsibilities: []string{
            "Review delivered changes for correctness and risk.",
            "Request missing context from implementation roles.",
            "Report approval status and follow-up tasks.",
        },
    },
    "architect": {
        Description:      "Architecture specialist for system boundaries and design reviews.",
        AgentNameSuffix:  "architect",
        AgentType:        "cto",
        Responsibilities: []string{
            "Define system-level constraints and interfaces.",
            "Review cross-cutting tradeoffs before implementation.",
            "Advise on architectural risk and migration paths.",
        },
    },
    "design": {
        Description:      "Design specialist for UX direction and visual handoff quality.",
        AgentNameSuffix:  "designer",
        AgentType:        "designer",
        Responsibilities: []string{
            "Produce UI intent, states, and interaction direction.",
            "Resolve ambiguities with frontend and architect.",
            "Support review with expected behavior and acceptance notes.",
        },
    },
    "qa": {
        Description:      "Quality assurance specialist for testing and verification.",
        AgentNameSuffix:  "qa",
        AgentType:        "qa",
        Responsibilities: []string{
            "Write and maintain test suites.",
            "Verify implementation against acceptance criteria.",
            "Report defects with reproduction steps.",
        },
    },
    "devops": {
        Description:      "Infrastructure and deployment specialist.",
        AgentNameSuffix:  "devops",
        AgentType:        "devops",
        Responsibilities: []string{
            "Manage CI/CD pipelines and deployment processes.",
            "Monitor system health and respond to incidents.",
            "Maintain infrastructure as code.",
        },
    },
    "security": {
        Description:      "Security specialist for auditing and threat modeling.",
        AgentNameSuffix:  "security",
        AgentType:        "security",
        Responsibilities: []string{
            "Perform security reviews on code changes.",
            "Define and enforce security policies.",
            "Assess and mitigate security risks.",
        },
    },
}

func generateDefaultRole(roleName string, allRoleNames []string) templateRole {
    meta, known := knownRoleDefaults[roleName]
    if !known {
        meta = roleMetadata{
            Description:      fmt.Sprintf("Specialist role for %s tasks.", roleName),
            AgentNameSuffix:  roleName,
            AgentType:        "specialist",
            Responsibilities: []string{fmt.Sprintf("Handle %s-related tasks as assigned.", roleName)},
        }
    }

    // Build mesh communication: connect to all other roles
    communicatesWith := make([]string, 0, len(allRoleNames)-1)
    for _, other := range allRoleNames {
        if other != roleName {
            communicatesWith = append(communicatesWith, other)
        }
    }

    return templateRole{
        Name:             roleName,
        Description:      meta.Description,
        AgentName:        meta.AgentNameSuffix,
        AgentType:        meta.AgentType,
        CommunicatesWith: communicatesWith,
        Responsibilities: meta.Responsibilities,
    }
}

func isKnownRole(name string) bool {
    _, ok := knownRoleDefaults[name]
    return ok
}
```

**Acceptance Criteria**:
- [x] 9 known roles pre-defined with rich metadata
- [x] Unknown role names produce reasonable generic defaults
- [x] Communication graph is automatically generated as a mesh (all-to-all minus self)
- [x] Generated mesh graph passes `validateCommunicationGraph()`

**Commit**: `feat(cli): PH2-T03-S01 create role defaults map for simplified template creation`

---

#### PH2-T03-S02: Implement simplified wizard flow

**File**: `internal/cli/init_prompter.go`

Add new simplified wizard method:

```go
func (p *initPrompter) runSimplifiedCustomTemplateWizard(ctx context.Context, existing []templateDefinition) (*onboard.TemplateDefinition, error) {
    name := ""
    rolesInput := ""

    form := huh.NewForm(
        huh.NewGroup(
            huh.NewInput().Title("Template name").
                Description("Short lowercase name for this template (e.g., my-team)").
                Value(&name).
                Validate(func(v string) error {
                    trimmed := strings.TrimSpace(v)
                    if trimmed == "" {
                        return errors.New("template name is required")
                    }
                    return validateSkillName(trimmed)
                }),
            huh.NewInput().Title("Role names").
                Description("Comma-separated list (e.g., frontend, backend, plan, review)").
                Value(&rolesInput).
                Validate(func(v string) error {
                    roles := splitCSVValues(v)
                    if len(roles) == 0 {
                        return errors.New("at least one role is required")
                    }
                    return nil
                }),
        ).Title("Quick Custom Template"),
    ).WithAccessible(p.accessible).WithInput(p.input).WithOutput(p.output)

    if err := form.RunWithContext(ctx); err != nil {
        if errors.Is(err, huh.ErrUserAborted) {
            return nil, onboard.ErrAborted
        }
        return nil, fmt.Errorf("cli.initPrompter.runSimplifiedCustomTemplateWizard: %w", err)
    }

    trimmedName := strings.TrimSpace(name)
    roleNames := splitCSVValues(rolesInput)

    // Warn about unknown roles
    for _, rn := range roleNames {
        if !isKnownRole(rn) {
            slog.Info("using generic defaults for unknown role", "role", rn)
        }
    }

    // Generate roles
    roles := make([]onboard.TemplateRole, 0, len(roleNames))
    for _, rn := range roleNames {
        generated := generateDefaultRole(rn, roleNames)
        roles = append(roles, onboard.TemplateRole{
            Name:             generated.Name,
            Description:      generated.Description,
            AgentName:        generated.AgentName,
            AgentType:        generated.AgentType,
            CommunicatesWith: generated.CommunicatesWith,
            Responsibilities: generated.Responsibilities,
        })
    }

    definition := &onboard.TemplateDefinition{
        Name:        trimmedName,
        Description: fmt.Sprintf("Custom %d-role template.", len(roles)),
        Reference:   "custom",
        CommonTitle: fmt.Sprintf("%s Common Instructions", titleWords(strings.ReplaceAll(trimmedName, "-", " "))),
        CommonBody:  "Coordinate through agentcom. Use `agentcom up` to start the team and `agentcom down` to stop.",
        Roles:       roles,
    }

    // Validate
    if err := validateCustomTemplateDefinition(templateDefinitionFromOnboard(*definition)); err != nil {
        return nil, fmt.Errorf("cli.initPrompter.runSimplifiedCustomTemplateWizard: %w", err)
    }
    for _, item := range existing {
        if item.Name == definition.Name {
            return nil, fmt.Errorf("cli.initPrompter.runSimplifiedCustomTemplateWizard: template %q already exists", definition.Name)
        }
    }
    return definition, nil
}
```

**Acceptance Criteria**:
- [x] 2-field form (name + role list) generates a complete template
- [x] Known roles get rich metadata; unknown roles get generic defaults
- [x] Communication graph is full mesh (minus self)
- [x] Template passes `validateCustomTemplateDefinition`

**Commit**: `feat(cli): PH2-T03-S02 implement simplified 2-field custom template wizard`

---

#### PH2-T03-S03: Add --advanced flag and wire up wizard selection

**File**: `internal/cli/init.go`

Add `--advanced` flag to `newInitCmd()`:

```go
var advanced bool
// In flag definitions:
cmd.Flags().BoolVar(&advanced, "advanced", false, "Use detailed custom template wizard with all fields")
```

**File**: `internal/cli/init_prompter.go`

Modify `Run()` method (line 251-257) to select wizard based on an `advanced` field on `initPrompter`:

```go
type initPrompter struct {
    accessible bool
    advanced   bool  // NEW
    input      io.Reader
    output     io.Writer
}
```

In the template choice handling (line 251-257):
```go
if templateChoice == "custom" {
    if p.advanced {
        customTemplate, err = p.runCustomTemplateWizard(ctx, templates)
    } else {
        customTemplate, err = p.runSimplifiedCustomTemplateWizard(ctx, templates)
    }
    // ...
}
```

**File**: `internal/cli/init_setup.go`

Update `newOnboardPrompter` and `newInitPrompter` to accept `advanced` parameter:

```go
func newInitPrompter(accessible bool, advanced bool, input io.Reader, output io.Writer) onboard.Prompter {
    return &initPrompter{accessible: accessible, advanced: advanced, input: input, output: output}
}
```

**Acceptance Criteria**:
- [x] `agentcom init --template custom` → simplified wizard (2 fields)
- [x] `agentcom init --template custom --advanced` → original detailed wizard (47 fields)
- [x] Both wizards produce valid templates
- [x] `--advanced` has no effect when `--template` is not `custom`

**Commit**: `feat(cli): PH2-T03-S03 add --advanced flag for detailed custom template wizard`

---

#### PH2-T03-S04: Tests for simplified wizard and role defaults

**File**: `internal/cli/role_defaults_test.go` (**new file**)

```go
func TestGenerateDefaultRole(t *testing.T) {
    tests := []struct {
        name     string
        roleName string
        allRoles []string
        wantType string
        wantComm int
    }{
        {"known frontend", "frontend", []string{"frontend", "backend"}, "engineer-frontend", 1},
        {"known backend", "backend", []string{"frontend", "backend", "plan"}, "engineer-backend", 2},
        {"unknown custom", "ops", []string{"ops", "dev"}, "specialist", 1},
        {"single role", "solo", []string{"solo"}, "specialist", 0},
    }
    // ...
}

func TestIsKnownRole(t *testing.T) { ... }
```

**File**: `internal/cli/init_prompter_test.go`

Add test for simplified wizard output validation:
```go
func TestSimplifiedWizardGeneratesValidTemplate(t *testing.T) {
    // Simulate input: name="test-team", roles="frontend, backend, plan"
    // Verify output template has 3 roles with correct metadata
    // Verify communication graph is valid
}
```

**Acceptance Criteria**:
- [x] All known roles produce correct metadata
- [x] Unknown roles produce reasonable defaults
- [x] Mesh communication graph is symmetric and self-reference-free
- [x] Simplified wizard produces valid template definitions
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH2-T03-S04 add tests for role defaults and simplified wizard`

---

<a id="ph2-t04"></a>
## PH2-T04: --force Flag Expansion

> **Epic**: 1 (Task 1.1.3 expanded)
> **Estimated Effort**: 3 hours
> **Parallel Group**: C (depends on PH1 marker system)

### Current Behavior

`--force` flag in `init.go:205`:
```go
cmd.Flags().BoolVar(&force, "force", false, "Overwrite project configuration if it already exists")
```

Currently only passed to `ensureInitProjectConfig()` (init.go:136). Not threaded to instruction file writing, scaffold writing, or skill file writing.

### Expected Behavior

`--force` applies to ALL file generation:
- `.agentcom.json` (current behavior — keep)
- Instruction files (AGENTS.md, CLAUDE.md, etc.) → `writeModeOverwrite`
- Scaffold files (COMMON.md, template.json) → `writeModeOverwrite`
- Template skill files → `writeModeOverwrite`

Without `--force`, all use `writeModeAppend` (marker-based).

### Implementation

#### PH2-T04-S01: Thread force flag through all file writing paths

**File**: `internal/cli/init.go`

In the batch-mode branch (lines 96-130), pass `force` to all write functions:

```go
mode := writeModeAppend
if force {
    mode = writeModeOverwrite
}
instructionFiles, err = writeAgentInstructions(cwd, selectedAgents, mode)
// ...
generatedFiles, err = writeTemplateScaffold(cwd, templateSelection, mode)
```

Update `writeTemplateScaffold` signature in `agents.go`:
```go
func writeTemplateScaffold(projectDir string, templateName string, mode writeMode) ([]string, error) {
```

**File**: `internal/cli/init_setup.go`

In `Apply()`, determine mode from a force parameter. Add `force` field to `initSetupExecutor`:

```go
type initSetupExecutor struct {
    projectDir string
    force      bool
}
```

Thread through `newInitSetupExecutor`:
```go
func newInitSetupExecutor(projectDir string, force bool) onboard.Applier {
    return &initSetupExecutor{projectDir: projectDir, force: force}
}
```

In `Apply()`, compute mode:
```go
mode := writeModeAppend
if e.force {
    mode = writeModeOverwrite
}
```

Pass to all write calls.

**File**: `internal/cli/init.go`

Update wizard path to pass force:
```go
wizard := onboard.NewWizard(
    newOnboardPrompter(accessible, advanced, cmd.InOrStdin(), cmd.OutOrStdout()),
    newInitSetupExecutor(cwd, force),
)
```

**Acceptance Criteria**:
- [x] `agentcom init --force --agents-md claude` overwrites existing CLAUDE.md
- [x] `agentcom init --force --template company` overwrites all scaffold and skill files
- [x] Without `--force`, all files use marker-based append
- [x] `--force` help text updated to reflect expanded scope

**Commit**: `feat(cli): PH2-T04-S01 thread --force flag through all file writing paths`

---

#### PH2-T04-S02: Update help text and tests

**File**: `internal/cli/init.go`

```go
cmd.Flags().BoolVar(&force, "force", false, "Overwrite all generated files (project config, instructions, scaffold, skills)")
```

**File**: `internal/cli/init_setup_test.go`

Add test:
```go
func TestInitSetupForceOverwritesAllFiles(t *testing.T) {
    // 1. First init creates files
    // 2. Modify a generated file
    // 3. Re-init with force=true
    // 4. Verify file is overwritten (modification gone)
}
```

**Acceptance Criteria**:
- [x] Help text describes full scope of --force
- [x] Force mode tested end-to-end
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH2-T04-S02 update --force help text and add force mode tests`

---

<a id="ph2-t05"></a>
## PH2-T05: Agent-Specific File Branching for Shared Paths

> **Epic**: 1 (Task 1.2)
> **Estimated Effort**: 5 hours
> **Parallel Group**: C (depends on PH1 marker system)

### Problem

Four agent IDs share the same file path (`AGENTS.md`):
- `codex` → `AGENTS.md`
- `opencode` → `AGENTS.md`
- `amp` → `AGENTS.md`
- `devin` → `AGENTS.md`

Currently, `writeAgentInstructions()` uses `seenPaths` to skip duplicate writes within a single run. But each agent's instruction content is identical (all render the same workflow body). If in the future agents diverge, or if a user selects `codex,opencode`, both agents should have their configuration present.

### Design Decision

Use agent-specific sub-markers within the same file:

```markdown
<!-- AGENTCOM:codex:START -->
# AGENTS.md
## agentcom Workflow (codex)
...
<!-- AGENTCOM:codex:END -->

<!-- AGENTCOM:opencode:START -->
# AGENTS.md
## agentcom Workflow (opencode)
...
<!-- AGENTCOM:opencode:END -->
```

However, since the current instruction content is identical for all agents sharing AGENTS.md, a simpler approach is preferred for now:

**Chosen approach**: Use a single marker block for shared paths. The `seenPaths` map already handles dedup correctly. The marker system from PH1-T01 handles the re-init case. The only change needed is to make the `seenPaths` logic compatible with the append-mode flow (currently it skips entirely; with append mode, it should still skip to avoid duplicate markers in the same file).

### Implementation

#### PH2-T05-S01: Ensure seenPaths compatibility with append mode

**File**: `internal/cli/instruction.go`

In `writeAgentInstructions()` (line 164-198), the `seenPaths` logic at line 186-188:

```go
if _, ok := seenPaths[path]; ok {
    continue
}
```

This correctly skips duplicate writes within a single init call. With append mode, this is still correct — we only want one marker block per file path, not one per agent that maps to the same path.

Verify this with a test but no code change needed.

**Acceptance Criteria**:
- [x] `writeAgentInstructions(dir, ["codex", "opencode"], writeModeAppend)` produces exactly one AGENTS.md marker block
- [x] The marker block is not duplicated

**Commit**: `test(cli): PH2-T05-S01 verify seenPaths dedup with append mode for shared file paths`

---

#### PH2-T05-S02: Add agent-specific sub-markers for future extensibility

**File**: `internal/cli/instruction.go`

Modify `renderInstructionContent()` to include the agent ID in the marker:

```go
func renderInstructionContent(agentID string, projectName string) (string, error) {
    // ... existing rendering logic ...
    // Wrap with agent-specific markers instead of generic markers
    return wrapWithAgentMarkers(agentID, content), nil
}
```

Add helper:
```go
func agentMarkerStart(agentID string) string {
    return fmt.Sprintf("<!-- AGENTCOM:%s:START -->", agentID)
}

func agentMarkerEnd(agentID string) string {
    return fmt.Sprintf("<!-- AGENTCOM:%s:END -->", agentID)
}

func wrapWithAgentMarkers(agentID string, content string) string {
    return fmt.Sprintf("%s\n%s\n%s\n", agentMarkerStart(agentID), strings.TrimRight(content, "\n"), agentMarkerEnd(agentID))
}
```

Update `writeInstructionFile()` and `findMarkerBounds()` to support agent-specific markers:
- `findMarkerBounds` gains an optional `agentID` parameter
- When `agentID` is non-empty, it searches for agent-specific markers
- When empty, it searches for generic markers (backward compat for scaffold/skill files)

Update `writeAgentInstructions()`:
- Remove `seenPaths` skip logic
- Instead, each agent writes its own sub-marker block
- Multiple agents sharing AGENTS.md each get their own section

**Acceptance Criteria**:
- [x] `agentcom init --agents-md codex,opencode` produces AGENTS.md with two agent-specific marker blocks
- [x] Re-running the same command updates each block idempotently
- [x] `agentcom init --agents-md codex` on a file with both blocks only updates the codex block
- [x] Backward compatibility: generic markers still work for scaffold/skill files

**Commit**: `feat(cli): PH2-T05-S02 add agent-specific sub-markers for shared instruction file paths`

---

#### PH2-T05-S03: Tests for agent-specific branching

**File**: `internal/cli/instruction_test.go`

```go
func TestMultiAgentSharedPath(t *testing.T) {
    projectDir := t.TempDir()

    // Write codex + opencode (both target AGENTS.md)
    paths, err := writeAgentInstructions(projectDir, []string{"codex", "opencode"}, writeModeAppend)
    // Verify 1 path returned (AGENTS.md)
    // Read AGENTS.md
    // Verify contains AGENTCOM:codex:START and AGENTCOM:opencode:START
    // Verify contains AGENTCOM:codex:END and AGENTCOM:opencode:END

    // Re-run
    paths2, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
    // Verify codex block updated, opencode block preserved
}
```

**Acceptance Criteria**:
- [x] Multi-agent shared path creates separate marker blocks
- [x] Selective re-init updates only targeted agent block
- [x] `go test ./internal/cli/... -count=1` passes

**Commit**: `test(cli): PH2-T05-S03 add tests for agent-specific shared path handling`

---

## Parallelization Map

```
[PARALLEL GROUP A]  (independent, can run simultaneously)
├── PH2-T01-S01: Graph validation function
├── PH2-T01-S02: Integration into template loading
├── PH2-T01-S03: Graph validation tests
├── PH2-T03-S01: role_defaults.go
├── PH2-T03-S02: Simplified wizard
├── PH2-T03-S03: --advanced flag wiring
└── PH2-T03-S04: Wizard and defaults tests

[PARALLEL GROUP B]  (depends on PH1-T02, PH2-T01)
├── PH2-T02-S01: Contact detail renderer
├── PH2-T02-S02: Collaboration protocol sections
├── PH2-T02-S03: Integration into renderRoleSkillContent
└── PH2-T02-S04: Communication section tests

[PARALLEL GROUP C]  (depends on Phase 1 marker system)
├── PH2-T04-S01: --force threading
├── PH2-T04-S02: --force tests
├── PH2-T05-S01: seenPaths compat verification
├── PH2-T05-S02: Agent-specific sub-markers
└── PH2-T05-S03: Agent branching tests
```

## Inter-Phase Dependencies

- **PH1 → PH2-T04**: Marker system and writeMode type must exist
- **PH1 → PH2-T05**: Marker system must exist for agent-specific sub-markers
- **PH1-T02 → PH2-T02**: Dynamic escalation enables protocol section
- **PH2-T01 → PH2-T02**: Graph validation ensures generated skills have valid contacts
- **PH2-T03 → PH3-T04**: Simplified wizard enables the template edit feature
- **PH2-T01 → PH4-T02**: Doctor command uses graph validation

## Test Plan Summary

| Test Type | Count | Coverage Target |
|-----------|-------|----------------|
| Unit tests (graph validation) | ~7 cases | All graph conditions |
| Unit tests (role defaults) | ~4 cases | Known + unknown roles |
| Unit tests (contact/protocol rendering) | ~6 cases | All sections |
| Unit tests (--force modes) | ~4 cases | All file types |
| Unit tests (agent branching) | ~4 cases | Shared/separate paths |
| Integration tests (simplified wizard) | ~2 cases | End-to-end template generation |
| **Total new test cases** | **~27** | |
