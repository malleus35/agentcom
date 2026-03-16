package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type templateDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Reference   string         `json:"reference"`
	CommonTitle string         `json:"common_title"`
	CommonBody  string         `json:"-"`
	Roles       []templateRole `json:"roles"`
}

type templateRole struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	AgentName        string   `json:"agent_name"`
	AgentType        string   `json:"agent_type"`
	CommunicatesWith []string `json:"communicates_with"`
	Responsibilities []string `json:"responsibilities"`
}

type templateSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Roles       []string `json:"roles"`
}

var templateSelectionEnabled = shouldPromptTemplateSelection

func newAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Agent template commands",
	}

	cmd.AddCommand(newAgentsTemplateCmd())

	return cmd
}

func newAgentsTemplateCmd() *cobra.Command {
	var listFlag bool
	var deleteName string

	cmd := &cobra.Command{
		Use:   "template [name]",
		Short: "List or inspect built-in and custom agent templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deleteName != "" {
				if len(args) > 0 || listFlag {
					return fmt.Errorf("cli.newAgentsTemplateCmd: --delete cannot be combined with template name or --list")
				}
				return deleteCustomTemplate(cmd, deleteName)
			}

			if listFlag || len(args) == 0 {
				summaries := listTemplateSummaries()
				if !listFlag && templateSelectionEnabled(cmd) {
					selectedName, err := selectTemplateSummary(cmd, summaries)
					if err != nil {
						return fmt.Errorf("cli.newAgentsTemplateCmd: select template: %w", err)
					}
					definition, err := resolveTemplateDefinition(selectedName)
					if err != nil {
						return fmt.Errorf("cli.newAgentsTemplateCmd: %w", err)
					}
					return writeTemplateDefinition(cmd, definition)
				}
				if jsonOutput {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{"templates": summaries})
				}

				for _, summary := range summaries {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", summary.Name, summary.Description); err != nil {
						return fmt.Errorf("cli.newAgentsTemplateCmd: write template summary: %w", err)
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  roles: %s\n", strings.Join(summary.Roles, ", ")); err != nil {
						return fmt.Errorf("cli.newAgentsTemplateCmd: write template roles: %w", err)
					}
				}
				return nil
			}

			definition, err := resolveTemplateDefinition(args[0])
			if err != nil {
				return fmt.Errorf("cli.newAgentsTemplateCmd: %w", err)
			}
			return writeTemplateDefinition(cmd, definition)
		},
	}

	cmd.Flags().BoolVar(&listFlag, "list", false, "List built-in and custom templates")
	cmd.Flags().StringVar(&deleteName, "delete", "", "Delete a custom template by name")
	cmd.AddCommand(newTemplateEditCmd())

	return cmd
}

func writeTemplateDefinition(cmd *cobra.Command, definition templateDefinition) error {

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(definition)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\nreference: %s\n", definition.Name, definition.Description, definition.Reference); err != nil {
		return fmt.Errorf("cli.newAgentsTemplateCmd: write template header: %w", err)
	}
	for _, role := range definition.Roles {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s): talks to %s\n", role.Name, role.AgentName, strings.Join(role.CommunicatesWith, ", ")); err != nil {
			return fmt.Errorf("cli.newAgentsTemplateCmd: write template role: %w", err)
		}
	}

	return nil
}

func shouldPromptTemplateSelection(cmd *cobra.Command) bool {
	if jsonOutput {
		return false
	}
	return streamIsInteractive(cmd.InOrStdin())
}

func streamIsInteractive(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func selectTemplateSummary(cmd *cobra.Command, summaries []templateSummary) (string, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Search templates (blank for all): "); err != nil {
		return "", fmt.Errorf("prompt search query: %w", err)
	}
	query, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read search query: %w", err)
	}
	query = strings.TrimSpace(strings.ToLower(query))

	filtered := filterTemplateSummaries(summaries, query)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no templates matched %q", query)
	}

	for i, summary := range filtered {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%d. %s - %s\n", i+1, summary.Name, summary.Description); err != nil {
			return "", fmt.Errorf("write template option: %w", err)
		}
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Select template number: "); err != nil {
		return "", fmt.Errorf("prompt selection: %w", err)
	}
	selection, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read selection: %w", err)
	}
	selection = strings.TrimSpace(selection)
	index, err := strconv.Atoi(selection)
	if err != nil || index < 1 || index > len(filtered) {
		return "", fmt.Errorf("invalid selection %q", selection)
	}

	return filtered[index-1].Name, nil
}

func filterTemplateSummaries(summaries []templateSummary, query string) []templateSummary {
	if query == "" {
		return summaries
	}
	filtered := make([]templateSummary, 0, len(summaries))
	for _, summary := range summaries {
		haystack := strings.ToLower(summary.Name + " " + summary.Description + " " + strings.Join(summary.Roles, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, summary)
		}
	}
	return filtered
}

func listTemplateSummaries() []templateSummary {
	definitions := builtInTemplateDefinitions()
	if cwd, err := os.Getwd(); err == nil {
		if resolved, resolveErr := allTemplateDefinitions(cwd); resolveErr == nil {
			definitions = resolved
		}
	}
	summaries := make([]templateSummary, 0, len(definitions))
	for _, definition := range definitions {
		roles := make([]string, 0, len(definition.Roles))
		for _, role := range definition.Roles {
			roles = append(roles, role.Name)
		}
		summaries = append(summaries, templateSummary{
			Name:        definition.Name,
			Description: definition.Description,
			Roles:       roles,
		})
	}
	return summaries
}

func resolveTemplateDefinition(name string) (templateDefinition, error) {
	definitions := builtInTemplateDefinitions()
	if cwd, err := os.Getwd(); err == nil {
		if resolved, resolveErr := allTemplateDefinitions(cwd); resolveErr == nil {
			definitions = resolved
		}
	}

	for _, definition := range definitions {
		if definition.Name == name {
			return definition, nil
		}
	}

	available := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		available = append(available, definition.Name)
	}
	sort.Strings(available)

	return templateDefinition{}, fmt.Errorf("unknown template %q: must be one of %s", name, strings.Join(available, ", "))
}

func writeTemplateScaffold(projectDir string, templateName string, mode writeMode) ([]string, error) {
	definition, err := resolveTemplateDefinition(templateName)
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(projectDir, ".agentcom", "templates", definition.Name)
	commonPath := filepath.Join(baseDir, "COMMON.md")
	manifestPath := filepath.Join(baseDir, "template.json")

	generatedPaths := []string{commonPath, manifestPath}

	if err := writeScaffoldFile(commonPath, renderTemplateCommonContent(definition), mode); err != nil {
		return nil, fmt.Errorf("write common markdown: %w", err)
	}

	manifestContent, err := renderTemplateManifest(definition)
	if err != nil {
		return nil, fmt.Errorf("render template manifest: %w", err)
	}
	if err := writeScaffoldFile(manifestPath, manifestContent, mode); err != nil {
		return nil, fmt.Errorf("write template manifest: %w", err)
	}

	sharedTargets, err := resolveTemplateSkillTargets("project", "agentcom")
	if err != nil {
		return nil, fmt.Errorf("resolve shared agentcom skill targets: %w", err)
	}
	sharedContent := renderAgentcomSharedSkillContent()
	for _, target := range sharedTargets {
		if err := writeSkillFile(target.Path, sharedContent, mode); err != nil {
			return nil, fmt.Errorf("write shared %s agentcom skill: %w", target.Agent, err)
		}
		generatedPaths = append(generatedPaths, target.Path)
	}

	commonRelPath, err := filepath.Rel(projectDir, commonPath)
	if err != nil {
		return nil, fmt.Errorf("relative common path: %w", err)
	}

	for _, role := range definition.Roles {
		generatedSkillName := templateRoleSkillName(definition.Name, role.Name)
		targets, err := resolveTemplateSkillTargets("project", filepath.Join("agentcom", generatedSkillName))
		if err != nil {
			return nil, fmt.Errorf("resolve role skill targets for %s: %w", role.Name, err)
		}
		content := renderRoleSkillContent(definition, role, generatedSkillName, filepath.ToSlash(commonRelPath))
		for _, target := range targets {
			if err := writeSkillFile(target.Path, content, mode); err != nil {
				return nil, fmt.Errorf("write %s skill for role %s: %w", target.Agent, role.Name, err)
			}
			generatedPaths = append(generatedPaths, target.Path)
		}
	}

	sort.Strings(generatedPaths)
	return generatedPaths, nil
}

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
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("cli.writeScaffoldFile: write: %w", err)
		}
		return nil
	}

	switch mode {
	case writeModeOverwrite:
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("cli.writeScaffoldFile: overwrite: %w", err)
		}
		return nil
	case writeModeAppend:
		slog.Debug("scaffold file already exists, skipping", "path", path)
		return nil
	default:
		return fmt.Errorf("cli.writeScaffoldFile: file already exists: %s (use --force to overwrite)", path)
	}
}

func renderTemplateManifest(definition templateDefinition) (string, error) {
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal template manifest: %w", err)
	}
	return string(data) + "\n", nil
}

func renderTemplateCommonContent(definition templateDefinition) string {
	return fmt.Sprintf("# %s\n\n%s\n", definition.CommonTitle, definition.CommonBody)
}

func renderAgentcomSharedSkillContent() string {
	return `---
name: agentcom
description: Shared agentcom skill instructions for generated template roles
---

# Agentcom

## Overview

agentcom is a CLI for real-time coordination between parallel AI coding agents.
It stores durable team state in SQLite and uses Unix Domain Sockets for low-latency local delivery.
Each role registers with a stable agent name and communicates through messages and tasks.
The generated template files in this repository are designed to give every role the same shared operating model.
Read this file first, then read your template ` + "`COMMON.md`" + `, then read your role-specific skill.

## Lifecycle

Default template lifecycle:

1. Run ` + "`agentcom init --template company`" + ` or ` + "`agentcom init --template oh-my-opencode`" + ` once per project.
2. Start the active template roles with ` + "`agentcom up`" + `.
3. Coordinate work with messages, inbox polling, broadcasts, and task handoffs.
4. Stop the managed team with ` + "`agentcom down`" + ` when the session ends.
5. Use ` + "`agentcom register --name frontend --type engineer-frontend`" + ` only as the low-level path for manually running one standalone agent session.

## Message Format

Send structured JSON messages whenever another role needs clear context:

~~~bash
agentcom send --from frontend backend '{"type":"request","subject":"need endpoint","body":"Please add GET /api/users"}'
~~~

Standard message types:
- ` + "`request`" + ` - ask another role to perform work.
- ` + "`response`" + ` - return a result to a prior request.
- ` + "`escalation`" + ` - report blockers or scope conflicts.
- ` + "`report`" + ` - broadcast progress, status, or completion.

Prefer JSON payloads over plain text when the receiver needs stable fields.
Keep the sender field aligned with your generated role agent name.

## Task Lifecycle

1. Create work with ` + "`agentcom task create \"Implement header\" --creator frontend --assign review --priority medium`" + `.
2. Check inbox with ` + "`agentcom inbox --agent review --unread`" + ` to discover assigned work and follow-up messages.
3. Mark active work with ` + "`agentcom task update task_123 --status in_progress --result \"started\"`" + `.
4. Mark completed work with ` + "`agentcom task update task_123 --status completed --result \"verified locally\"`" + `.
5. Reassign work with ` + "`agentcom task delegate task_123 --to backend`" + ` when ownership changes.

Tasks are the durable handoff mechanism.
Messages provide context around the task, but the task status is the team-visible source of truth.

## Decision Guide

- Work independently when the task stays inside your role scope and no other role is blocked by your decision.
- Send a direct message when you need one specific role to act on a concrete dependency.
- Broadcast when the whole team should know about a milestone, blocker removal, or state change.
- Escalate when architecture, sequencing, or priority decisions exceed your role scope.
- Prefer ` + "`agentcom up`" + ` and ` + "`agentcom down`" + ` for template teams; keep ` + "`register`" + ` as the advanced manual path.

## Quick Reference

| Action | Command |
|--------|---------|
| Send message | ` + "`agentcom send --from frontend backend '{\"type\":\"request\",\"subject\":\"need endpoint\"}'`" + ` |
| Broadcast update | ` + "`agentcom broadcast --from plan --topic status '{\"phase\":\"review\"}'`" + ` |
| Check inbox | ` + "`agentcom inbox --agent review --unread`" + ` |
| Create task | ` + "`agentcom task create \"Review API contract\" --creator frontend --assign backend --priority high`" + ` |
| Update task | ` + "`agentcom task update task_123 --status completed --result \"done\"`" + ` |
| Delegate task | ` + "`agentcom task delegate task_123 --to architect`" + ` |
| Show status | ` + "`agentcom status`" + ` |

## Role Skills

Every generated role skill in this directory adds role-specific workflow, examples, communication rules, and handoff guidance.
Read the shared skill first so all roles operate from the same lifecycle and message model.
Then read the role skill that matches your assigned identity.
`
}

var roleWorkflows = map[string]string{
	"frontend": `## Workflow

1. Check inbox for task assignments and design handoffs with ` + "`agentcom inbox --agent frontend --unread`" + `.
2. Review the latest design direction and confirm any ambiguous states with design.
3. Confirm API assumptions with backend before changing UI contracts.
4. Implement components and pages within the agreed scope.
5. Run local verification before handoff.
6. Send a review-ready update to review and notify plan if scope changed.`,
	"backend": `## Workflow

1. Check inbox for assigned work and dependency requests.
2. Review the requested contract and confirm payload shape with frontend.
3. Implement services, schemas, endpoints, or migrations.
4. Run build, test, and migration verification locally.
5. Notify frontend and review about any contract changes.
6. Update the task with verification notes before completion.`,
	"plan": `## Workflow

1. Check inbox for new requests, blockers, and completion reports.
2. Break requests into concrete tasks with clear owners.
3. Sequence frontend, backend, design, review, and architect handoffs.
4. Track progress and unblock stalled work.
5. Broadcast major plan changes to the full team.
6. Close the loop when all dependent tasks are complete.`,
	"review": `## Workflow

1. Check inbox for review-ready updates and task assignments.
2. Review the implementation summary, changed files, and verification notes.
3. Reproduce the expected behavior or run the requested checks.
4. Send clear approval or follow-up requests.
5. Escalate cross-role gaps to plan or architect when needed.
6. Mark the review task complete only when evidence is sufficient.`,
	"architect": `## Workflow

1. Check inbox for escalations, system-boundary questions, and risky proposals.
2. Review the current template workflow and impacted roles.
3. Clarify interfaces, constraints, and architectural tradeoffs.
4. Respond with stable guidance that execution roles can follow.
5. Coordinate with plan if the decision changes sequencing.
6. Notify review when the architecture guidance becomes part of acceptance criteria.`,
	"design": `## Workflow

1. Check inbox for design requests, open questions, and handoff feedback.
2. Clarify user flows, states, copy, and visual expectations.
3. Coordinate closely with frontend on implementation feasibility.
4. Record expected behavior for review and architect when edge cases matter.
5. Send explicit handoff notes once the direction is stable.
6. Stay available for refinement until review closes the work.`,
}

var roleExamples = map[string]string{
	"frontend": `## Examples

### Example 1: start a UI task

~~~bash
agentcom inbox --agent frontend --unread
agentcom task update task_101 --status in_progress --result "starting header implementation"
~~~

### Example 2: request backend support

~~~bash
agentcom task create "Add GET /api/users endpoint" --creator frontend --assign backend --priority medium
agentcom send --from frontend backend '{"type":"request","subject":"need endpoint","endpoint":"GET /api/users"}'
~~~`,
	"backend": `## Examples

### Example 1: acknowledge an API request

~~~bash
agentcom inbox --agent backend --unread
agentcom task update task_202 --status in_progress --result "implementing users endpoint"
~~~

### Example 2: notify frontend about a contract

~~~bash
agentcom send --from backend frontend '{"type":"response","subject":"endpoint ready","endpoint":"GET /api/users","status":"ready for integration"}'
agentcom broadcast --from backend --topic api '{"status":"users endpoint ready"}'
~~~`,
	"plan": `## Examples

### Example 1: create coordinated work

~~~bash
agentcom task create "Design dashboard filters" --creator plan --assign design --priority high
agentcom task create "Implement dashboard filters" --creator plan --assign frontend --priority high
~~~

### Example 2: broadcast a sequence change

~~~bash
agentcom broadcast --from plan --topic priority '{"status":"backend contract must land before frontend handoff"}'
agentcom send --from plan review '{"type":"report","subject":"review later","body":"Hold review until backend task_310 completes"}'
~~~`,
	"review": `## Examples

### Example 1: request missing evidence

~~~bash
agentcom send --from review frontend '{"type":"request","subject":"need verification","body":"Please share build output and screenshots"}'
agentcom task update task_404 --status in_progress --result "awaiting verification details"
~~~

### Example 2: approve work

~~~bash
agentcom send --from review plan '{"type":"response","subject":"approved","task_id":"task_404","status":"approved"}'
agentcom task update task_404 --status completed --result "review approved"
~~~`,
	"architect": `## Examples

### Example 1: answer an escalation

~~~bash
agentcom inbox --agent architect --unread
agentcom send --from architect backend '{"type":"response","subject":"architecture guidance","body":"Keep the existing service boundary and add a dedicated repository method"}'
~~~

### Example 2: notify plan about a boundary change

~~~bash
agentcom send --from architect plan '{"type":"report","subject":"sequence change","body":"Frontend should wait until backend schema migration lands"}'
agentcom broadcast --from architect --topic architecture '{"status":"service boundary updated"}'
~~~`,
	"design": `## Examples

### Example 1: hand off design direction

~~~bash
agentcom send --from design frontend '{"type":"request","subject":"design handoff","body":"Use a two-column settings layout with inline validation states"}'
agentcom task update task_515 --status in_progress --result "shared first-pass UX direction"
~~~

### Example 2: clarify review expectations

~~~bash
agentcom send --from design review '{"type":"report","subject":"expected behavior","body":"Empty state should show action copy and a primary CTA"}'
agentcom broadcast --from design --topic ux '{"status":"interaction notes shared"}'
~~~`,
}

var roleAntiPatterns = map[string]string{
	"frontend": `## Anti-patterns

- Do not implement backend persistence or schema changes yourself.
- Do not skip review-ready verification notes before handoff.
- Do not change product direction without design or plan alignment.`,
	"backend": `## Anti-patterns

- Do not change request or response contracts silently.
- Do not push architectural tradeoffs onto frontend without architect input.
- Do not mark a task complete before verification succeeds.`,
	"plan": `## Anti-patterns

- Do not assign ambiguous tasks without clear owners and outcomes.
- Do not hide priority changes from the rest of the team.
- Do not make architectural calls that belong to architect.`,
	"review": `## Anti-patterns

- Do not approve work without concrete verification evidence.
- Do not rewrite requirements during review without involving plan.
- Do not hold feedback privately when another role is blocked.`,
	"architect": `## Anti-patterns

- Do not expand scope with speculative refactors.
- Do not override role ownership for routine implementation details.
- Do not leave escalations unresolved when system boundaries are affected.`,
	"design": `## Anti-patterns

- Do not hand off vague UX direction without concrete states.
- Do not bypass frontend when implementation constraints change.
- Do not treat review as optional for user-facing behavior changes.`,
}

func defaultRoleWorkflow(roleName string) string {
	return fmt.Sprintf(`## Workflow

1. Check inbox for assigned work with `+"`agentcom inbox --agent %s --unread`"+`.
2. Review the request and confirm the expected outcome.
3. Execute the work within your role scope.
4. Run verification steps appropriate to your domain.
5. Report completion and any follow-up needs to the requesting role.`, roleName)
}

func defaultRoleExamples(roleName string) string {
	return fmt.Sprintf(`## Examples

### Example 1: start assigned work

~~~bash
agentcom inbox --agent %[1]s --unread
agentcom task update task_100 --status in_progress --result "starting %[1]s work"
~~~

### Example 2: report completion

~~~bash
agentcom send --from %[1]s plan '{"type":"report","subject":"work complete","body":"%[1]s task is ready for the next handoff"}'
agentcom task update task_100 --status completed --result "completed %[1]s scope"
~~~`, roleName)
}

func defaultRoleAntiPatterns(roleName string) string {
	return fmt.Sprintf(`## Anti-patterns

- Do not work outside the agreed %[1]s scope without coordination.
- Do not close tasks without recording verification notes.
- Do not keep blockers local when another role can unblock them.`, roleName)
}

func roleWorkflowContent(role templateRole) string {
	if content, ok := roleWorkflows[role.Name]; ok {
		return content
	}
	return defaultRoleWorkflow(role.AgentName)
}

func roleExamplesContent(role templateRole) string {
	if content, ok := roleExamples[role.Name]; ok {
		return content
	}
	return defaultRoleExamples(role.AgentName)
}

func roleAntiPatternsContent(role templateRole) string {
	if content, ok := roleAntiPatterns[role.Name]; ok {
		return content
	}
	return defaultRoleAntiPatterns(role.AgentName)
}

func renderHandoffProtocol(role templateRole) string {
	return fmt.Sprintf(`## Handoff Protocol

1. Update your current task before the handoff with `+"`agentcom task update task_123 --status in_progress --result \"handoff prepared\"`"+`.
2. Create or delegate the next task with clear ownership and priority.
3. Send a direct message with the exact context the next role needs.
4. Watch your inbox with `+"`agentcom inbox --agent %s --unread`"+` for follow-up questions.
5. Do not treat the handoff as complete until the receiving role has enough context to continue.`, role.AgentName)
}

func renderContactDetails(role templateRole, allRoles []templateRole) string {
	var sb strings.Builder
	for _, contactName := range role.CommunicatesWith {
		for _, other := range allRoles {
			if other.Name != contactName {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", other.Name, other.AgentName, other.Description))
			break
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func primaryCommunicationTarget(role templateRole) string {
	if len(role.CommunicatesWith) > 0 {
		return role.CommunicatesWith[0]
	}
	return role.Name
}

func renderCollaborationProtocol(role templateRole) string {
	agentName := role.AgentName
	requestTarget := primaryCommunicationTarget(role)
	responseTarget := requestTarget
	escalationTargets := computeEscalationTargets(role.Name, role.CommunicatesWith)

	var sb strings.Builder
	sb.WriteString("### Request\n\n")
	sb.WriteString("When you need work from another role, create a task:\n")
	sb.WriteString(fmt.Sprintf("```\nagentcom task create \"Coordinate dependency\" --creator %s --assign %s --priority medium\n```\n\n", agentName, requestTarget))

	sb.WriteString("### Response\n\n")
	sb.WriteString("When completing a task assigned to you, update status and notify:\n")
	sb.WriteString(fmt.Sprintf("```\nagentcom task update task_123 --status completed --result \"verified and ready\"\nagentcom send --from %s %s '{\"type\":\"response\",\"task_id\":\"task_123\",\"status\":\"completed\"}'\n```\n\n", agentName, responseTarget))

	sb.WriteString("### Escalation\n\n")
	if len(escalationTargets) > 0 {
		sb.WriteString(fmt.Sprintf("When blocked or when decisions exceed your role scope, escalate to %s:\n", strings.Join(escalationTargets, " or ")))
		sb.WriteString(fmt.Sprintf("```\nagentcom send --from %s %s '{\"type\":\"escalation\",\"blocker\":\"need decision on system boundary\"}'\n```\n\n", agentName, escalationTargets[0]))
	} else {
		sb.WriteString("No escalation targets defined for this role. Resolve blockers independently or broadcast for help.\n\n")
	}

	sb.WriteString("### Report\n\n")
	sb.WriteString("Broadcast progress updates to the team:\n")
	sb.WriteString(fmt.Sprintf("```\nagentcom broadcast --from %s --topic progress '{\"status\":\"in_progress\",\"summary\":\"finished first verification pass\"}'\n```\n", agentName))

	return sb.String()
}

func renderRoleSkillContent(definition templateDefinition, role templateRole, generatedSkillName string, commonPath string) string {
	bodyTitle := titleWords(strings.ReplaceAll(generatedSkillName, "-", " "))
	directTarget := primaryCommunicationTarget(role)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n", generatedSkillName, role.Description))
	sb.WriteString(fmt.Sprintf("# %s\n\n", bodyTitle))
	sb.WriteString("- Read shared agentcom instructions first: `../SKILL.md`\n")
	sb.WriteString(fmt.Sprintf("- Read common instructions first: `%s`\n", commonPath))
	sb.WriteString(fmt.Sprintf("- Template: `%s` (`%s`)\n", definition.Name, definition.Reference))
	sb.WriteString(fmt.Sprintf("- Agent identity: `%s` / type `%s`\n", role.AgentName, role.AgentType))
	sb.WriteString("\n## Responsibilities\n\n")
	sb.WriteString(renderResponsibilities(role.Responsibilities))
	sb.WriteString("\n\n")
	sb.WriteString(roleWorkflowContent(role))
	sb.WriteString("\n\n")
	sb.WriteString(roleExamplesContent(role))
	sb.WriteString("\n\n")
	sb.WriteString(roleAntiPatternsContent(role))
	sb.WriteString("\n\n## Communication\n\n")
	sb.WriteString("### Primary Contacts\n\n")
	if details := renderContactDetails(role, definition.Roles); details != "" {
		sb.WriteString(details)
		sb.WriteString("\n\n")
	}
	sb.WriteString("### Coordination Commands\n\n")
	sb.WriteString(fmt.Sprintf("- Direct message: `agentcom send --from %s %s '{\"type\":\"request\",\"subject\":\"coordination\"}'`\n", role.AgentName, directTarget))
	sb.WriteString(fmt.Sprintf("- Check inbox: `agentcom inbox --agent %s --unread`\n", role.AgentName))
	sb.WriteString("- For template-based teams, use `agentcom up` and `agentcom down` as the default lifecycle.\n")
	sb.WriteString("- Use `agentcom register` only for advanced standalone sessions.\n\n")
	sb.WriteString(renderCollaborationProtocol(role))
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteString("\n")
	}
	if escalation := renderEscalationLine(computeEscalationTargets(role.Name, role.CommunicatesWith)); escalation != "" {
		sb.WriteString("\n")
		sb.WriteString(escalation)
	}
	sb.WriteString("\n")
	sb.WriteString(renderHandoffProtocol(role))
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

func templateRoleSkillName(templateName string, roleName string) string {
	return templateName + "-" + roleName
}

func renderResponsibilities(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

type graphIssue struct {
	Severity string
	Role     string
	Message  string
}

func validateCommunicationGraph(roles []templateRole) []graphIssue {
	issues := make([]graphIssue, 0)
	roleNames := make(map[string]struct{}, len(roles))
	communicationMap := make(map[string][]string, len(roles))
	referenced := make(map[string]struct{}, len(roles))

	for _, role := range roles {
		roleNames[role.Name] = struct{}{}
		communicationMap[role.Name] = role.CommunicatesWith
	}

	for _, role := range roles {
		for _, target := range role.CommunicatesWith {
			if target == role.Name {
				issues = append(issues, graphIssue{
					Severity: "error",
					Role:     role.Name,
					Message:  fmt.Sprintf("role %q lists itself in CommunicatesWith", role.Name),
				})
			}

			if _, ok := roleNames[target]; !ok {
				issues = append(issues, graphIssue{
					Severity: "error",
					Role:     role.Name,
					Message:  fmt.Sprintf("role %q references unknown role %q", role.Name, target),
				})
				continue
			}

			referenced[target] = struct{}{}
			if !containsString(communicationMap[target], role.Name) {
				issues = append(issues, graphIssue{
					Severity: "warning",
					Role:     role.Name,
					Message:  fmt.Sprintf("asymmetric: %q→%q exists but %q→%q missing", role.Name, target, target, role.Name),
				})
			}
		}
	}

	for _, role := range roles {
		if _, ok := referenced[role.Name]; ok {
			continue
		}
		if len(role.CommunicatesWith) == 0 {
			issues = append(issues, graphIssue{
				Severity: "warning",
				Role:     role.Name,
				Message:  fmt.Sprintf("role %q is isolated: no incoming or outgoing connections", role.Name),
			})
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

func computeEscalationTargets(roleName string, communicatesWith []string) []string {
	preferred := []string{"plan", "architect"}
	targets := make([]string, 0, 2)
	for _, name := range preferred {
		if name != roleName && containsString(communicatesWith, name) {
			targets = append(targets, name)
		}
	}
	if len(targets) > 0 {
		return targets
	}
	for _, contact := range communicatesWith {
		if contact == roleName {
			continue
		}
		targets = append(targets, contact)
		if len(targets) == 2 {
			break
		}
	}
	return targets
}

func renderEscalationLine(targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	formatted := make([]string, 0, len(targets))
	for _, target := range targets {
		formatted = append(formatted, fmt.Sprintf("`%s`", target))
	}
	return fmt.Sprintf("- Escalate blockers to %s when requirements or system boundaries change.\n", strings.Join(formatted, " and "))
}

func builtInTemplateDefinitions() []templateDefinition {
	communicationMap := map[string][]string{
		"frontend":  {"design", "backend", "review", "architect", "plan"},
		"backend":   {"frontend", "architect", "review", "plan"},
		"plan":      {"architect", "frontend", "backend", "design", "review"},
		"review":    {"frontend", "backend", "architect", "plan", "design"},
		"architect": {"plan", "frontend", "backend", "design", "review"},
		"design":    {"plan", "frontend", "architect", "review"},
	}

	definitions := []templateDefinition{
		{
			Name:        "company",
			Description: "Company-style multi-agent template inspired by Paperclip org roles.",
			Reference:   "paperclip",
			CommonTitle: "Company Template Common Instructions",
			CommonBody: strings.TrimSpace(`Use this template when a small product team needs clear functional ownership.

## Team Model

- Keep agent names stable across sessions so task and message history stays readable.
- The planning role leads sequencing and cross-role coordination.
- The architect role handles system-boundary decisions and high-risk tradeoffs.
- The review role is the quality gate before work is considered complete.
- Frontend, backend, and design own their delivery domains and should coordinate directly on dependencies.

## Standard Lifecycle

- Run ` + "`agentcom init --template company`" + ` once per project.
- Start managed roles with ` + "`agentcom up`" + `.
- Use ` + "`agentcom down`" + ` to stop the session cleanly.
- Use ` + "`agentcom register --name frontend --type engineer-frontend`" + ` only for low-level manual runs of a single standalone role.

## Communication Norms

- Use direct messages for one-to-one dependency work.
- Use broadcasts for team-wide milestones, blockers, or sequencing changes.
- Use tasks for durable ownership and status tracking.
- Store structured payloads as JSON so review and architect can audit decisions.
- If a dependency changes another role's plan, notify plan immediately.

## Coding Standards

- Prefer focused changes over broad refactors.
- Keep commit messages scoped and descriptive.
- Use branch names that map to one task or feature at a time.
- Include verification notes when handing work to review.
- Preserve existing project conventions unless architect approves a deviation.

## Priority Rules

- Follow plan's current sequence when priorities conflict.
- Escalate to architect when a technical constraint invalidates the current plan.
- Ask review to focus on the highest-risk change first when multiple items land together.
- Do not start speculative work while a blocking dependency is unresolved.

This template is inspired by Paperclip's company/org model, but uses six delivery-focused roles: frontend, backend, plan, review, architect, and design.`),
			Roles: []templateRole{
				{
					Name:             "frontend",
					Description:      "Frontend implementation specialist for UI delivery, design handoff, and agentcom coordination.",
					AgentName:        "frontend",
					AgentType:        "engineer-frontend",
					CommunicatesWith: communicationMap["frontend"],
					Responsibilities: []string{"Implement UI work from design direction.", "Coordinate API contracts with backend.", "Send review-ready updates with file and state summaries."},
				},
				{
					Name:             "backend",
					Description:      "Backend implementation specialist for APIs, data flows, and agentcom coordination.",
					AgentName:        "backend",
					AgentType:        "engineer-backend",
					CommunicatesWith: communicationMap["backend"],
					Responsibilities: []string{"Implement services, schemas, and interfaces.", "Confirm payload contracts with frontend.", "Escalate system risks and migration needs to architect and plan."},
				},
				{
					Name:             "plan",
					Description:      "Planning specialist for breaking work into milestones, sequencing tasks, and routing updates through agentcom.",
					AgentName:        "plan",
					AgentType:        "pm",
					CommunicatesWith: communicationMap["plan"],
					Responsibilities: []string{"Turn requests into deliverable task breakdowns.", "Coordinate handoffs between execution roles.", "Track blockers and completion signals across the team."},
				},
				{
					Name:             "review",
					Description:      "Review specialist for QA, regression checks, and cross-role feedback loops using agentcom.",
					AgentName:        "review",
					AgentType:        "qa",
					CommunicatesWith: communicationMap["review"],
					Responsibilities: []string{"Review delivered changes for correctness and risk.", "Request missing context from frontend, backend, or architect.", "Report approval status and follow-up tasks back to plan."},
				},
				{
					Name:             "architect",
					Description:      "Architecture specialist for system boundaries, design reviews, and escalations via agentcom.",
					AgentName:        "architect",
					AgentType:        "cto",
					CommunicatesWith: communicationMap["architect"],
					Responsibilities: []string{"Define system-level constraints and interfaces.", "Review cross-cutting tradeoffs before implementation expands.", "Advise plan and review on architectural risk."},
				},
				{
					Name:             "design",
					Description:      "Design specialist for UX direction, handoff quality, and collaboration through agentcom.",
					AgentName:        "design",
					AgentType:        "designer",
					CommunicatesWith: communicationMap["design"],
					Responsibilities: []string{"Produce UI intent, states, and interaction direction.", "Resolve ambiguities with frontend and architect.", "Support review with expected behavior and acceptance notes."},
				},
			},
		},
		{
			Name:        "oh-my-opencode",
			Description: "Oh-My-OpenCode-inspired template with planner, reviewer, architect, and execution specialists.",
			Reference:   "oh-my-opencode",
			CommonTitle: "Oh-My-OpenCode Template Common Instructions",
			CommonBody: strings.TrimSpace(`Use this template when you want a planning-heavy workflow inspired by Oh-My-OpenCode.

## Team Model

- Keep planner, reviewer, and architect roles distinct from execution roles.
- Plan owns decomposition, sequencing, and routing.
- Review is the explicit quality gate before completion.
- Architect handles boundary changes, risky tradeoffs, and escalations.
- Frontend, backend, and design execute once the plan is clear.

## Standard Lifecycle

- Run ` + "`agentcom init --template oh-my-opencode`" + ` once per project.
- Start the managed team with ` + "`agentcom up`" + `.
- Stop the managed team with ` + "`agentcom down`" + `.
- Use ` + "`agentcom register --name oracle --type architect`" + ` only as the advanced manual path for a single role.

## Planning-First Rules

- Do not begin implementation before plan has defined the work and handoff order.
- Break large work into explicit tasks before execution starts.
- Broadcast plan changes when they affect more than one role.
- If implementation reveals a hidden dependency, route it back through plan.

## Review Gates

- Review must see verification evidence before approval.
- Execution roles should send concise file lists, outputs, and risk notes.
- Review can reopen a task if the evidence is incomplete.
- Plan should treat review feedback as part of the active sequence, not as optional commentary.

## Architecture Checkpoints

- Consult architect for system-boundary changes, major dependency shifts, or cross-cutting policy decisions.
- Architect guidance should be shared early enough that frontend and backend do not diverge.
- Review should verify that architectural guidance was actually followed.
- Plan should re-sequence work if architect changes the dependency order.

This template references official Oh-My-OpenCode agent patterns such as Prometheus (planning), Momus (review), Oracle (architecture), and Sisyphus-Junior style execution specialists.`),
			Roles: []templateRole{
				{
					Name:             "frontend",
					Description:      "Frontend execution specialist aligned with visual-engineering style delivery and agentcom handoffs.",
					AgentName:        "sisyphus-junior-frontend",
					AgentType:        "sisyphus-junior/visual-engineering",
					CommunicatesWith: communicationMap["frontend"],
					Responsibilities: []string{"Execute UI work after plan or design handoff.", "Sync API assumptions with backend and architect.", "Return review-ready updates with concrete verification notes."},
				},
				{
					Name:             "backend",
					Description:      "Backend execution specialist aligned with Sisyphus-Junior implementation work and agentcom handoffs.",
					AgentName:        "sisyphus-junior-backend",
					AgentType:        "sisyphus-junior/unspecified-high",
					CommunicatesWith: communicationMap["backend"],
					Responsibilities: []string{"Execute service and data-layer changes after planning.", "Confirm interfaces with frontend and architect.", "Report verification details back to review and plan."},
				},
				{
					Name:             "plan",
					Description:      "Planner specialist modeled after Prometheus for decomposition, sequencing, and agentcom task routing.",
					AgentName:        "prometheus",
					AgentType:        "planner",
					CommunicatesWith: communicationMap["plan"],
					Responsibilities: []string{"Create the initial execution plan and handoff order.", "Coordinate dependencies between specialists.", "Request architectural or review input before major expansions."},
				},
				{
					Name:             "review",
					Description:      "Review specialist modeled after Momus for QA, gap detection, and agentcom feedback loops.",
					AgentName:        "momus",
					AgentType:        "reviewer",
					CommunicatesWith: communicationMap["review"],
					Responsibilities: []string{"Check whether work matches the plan and acceptance bar.", "Request missing evidence from execution roles.", "Send concise approval or follow-up tasks back through plan."},
				},
				{
					Name:             "architect",
					Description:      "Architecture specialist modeled after Oracle for read-mostly system guidance and escalation handling.",
					AgentName:        "oracle",
					AgentType:        "architect",
					CommunicatesWith: communicationMap["architect"],
					Responsibilities: []string{"Advise on system boundaries and risky tradeoffs.", "Unblock plan when implementation paths diverge.", "Provide stable interface guidance to frontend and backend."},
				},
				{
					Name:             "design",
					Description:      "Design execution specialist aligned with visual-engineering style work and agentcom collaboration.",
					AgentName:        "sisyphus-junior-design",
					AgentType:        "sisyphus-junior/visual-engineering",
					CommunicatesWith: communicationMap["design"],
					Responsibilities: []string{"Translate product intent into design-ready direction.", "Align closely with frontend on final handoff quality.", "Provide expected UX outcomes to review and architect."},
				},
			},
		},
	}

	for _, definition := range definitions {
		for _, issue := range validateCommunicationGraph(definition.Roles) {
			slog.Warn("built-in template graph issue",
				"template", definition.Name,
				"severity", issue.Severity,
				"role", issue.Role,
				"message", issue.Message,
			)
		}
	}

	return definitions
}

func deleteCustomTemplate(cmd *cobra.Command, name string) error {
	for _, definition := range builtInTemplateDefinitions() {
		if definition.Name == name {
			return fmt.Errorf("cli.deleteCustomTemplate: cannot delete built-in template %q", name)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cli.deleteCustomTemplate: getwd: %w", err)
	}
	templatePath := filepath.Join(projectDir, ".agentcom", "templates", name)
	if _, err := os.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cli.deleteCustomTemplate: custom template %q not found", name)
		}
		return fmt.Errorf("cli.deleteCustomTemplate: stat template: %w", err)
	}

	if !jsonOutput {
		reader := bufio.NewReader(cmd.InOrStdin())
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Delete custom template %s? [y/N]: ", name); err != nil {
			return fmt.Errorf("cli.deleteCustomTemplate: prompt confirmation: %w", err)
		}
		response, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("cli.deleteCustomTemplate: read confirmation: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("cli.deleteCustomTemplate: delete cancelled")
		}
	}

	if err := os.RemoveAll(templatePath); err != nil {
		return fmt.Errorf("cli.deleteCustomTemplate: remove template: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"deleted": name, "path": templatePath})
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "deleted custom template %s\n", name)
	return err
}
