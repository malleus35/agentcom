package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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

func writeTemplateScaffold(projectDir string, templateName string) ([]string, error) {
	definition, err := resolveTemplateDefinition(templateName)
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(projectDir, ".agentcom", "templates", definition.Name)
	commonPath := filepath.Join(baseDir, "COMMON.md")
	manifestPath := filepath.Join(baseDir, "template.json")

	generatedPaths := []string{commonPath, manifestPath}

	if err := writeScaffoldFile(commonPath, renderTemplateCommonContent(definition)); err != nil {
		return nil, fmt.Errorf("write common markdown: %w", err)
	}

	manifestContent, err := renderTemplateManifest(definition)
	if err != nil {
		return nil, fmt.Errorf("render template manifest: %w", err)
	}
	if err := writeScaffoldFile(manifestPath, manifestContent); err != nil {
		return nil, fmt.Errorf("write template manifest: %w", err)
	}

	sharedTargets, err := resolveTemplateSkillTargets("project", "agentcom")
	if err != nil {
		return nil, fmt.Errorf("resolve shared agentcom skill targets: %w", err)
	}
	sharedContent := renderAgentcomSharedSkillContent()
	for _, target := range sharedTargets {
		if err := writeSkillFile(target.Path, sharedContent); err != nil {
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
			if err := writeSkillFile(target.Path, content); err != nil {
				return nil, fmt.Errorf("write %s skill for role %s: %w", target.Agent, role.Name, err)
			}
			generatedPaths = append(generatedPaths, target.Path)
		}
	}

	sort.Strings(generatedPaths)
	return generatedPaths, nil
}

func writeScaffoldFile(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir scaffold dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write scaffold file: %w", err)
	}

	return nil
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

- Use this shared skill as the common base for generated agentcom template role skills.
- Default template lifecycle: run ` + "`agentcom init --template <template>`" + ` once, start the managed roles with ` + "`agentcom up`" + `, and stop them with ` + "`agentcom down`" + `.
- Use ` + "`agentcom register`" + ` only as the low-level path for manually running one standalone agent session.
- Coordinate with ` + "`agentcom send`" + `, ` + "`agentcom inbox`" + `, ` + "`agentcom task create`" + `, and ` + "`agentcom task delegate`" + `.
- Read the role-specific skill under this directory for template and responsibility details.
`
}

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
`, generatedSkillName, role.Description, bodyTitle, commonPath, definition.Name, definition.Reference, role.AgentName, role.AgentType, renderResponsibilities(role.Responsibilities), strings.Join(role.CommunicatesWith, ", "))
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

func builtInTemplateDefinitions() []templateDefinition {
	communicationMap := map[string][]string{
		"frontend":  {"design", "backend", "review", "architect"},
		"backend":   {"frontend", "architect", "review", "plan"},
		"plan":      {"architect", "frontend", "backend", "design", "review"},
		"review":    {"frontend", "backend", "architect", "plan"},
		"architect": {"plan", "frontend", "backend", "design", "review"},
		"design":    {"plan", "frontend", "architect", "review"},
	}

	return []templateDefinition{
		{
			Name:        "company",
			Description: "Company-style multi-agent template inspired by Paperclip org roles.",
			Reference:   "paperclip",
			CommonTitle: "Company Template Common Instructions",
			CommonBody: strings.TrimSpace(`Use this template when a small product team needs clear functional ownership.

- Keep agent names stable across sessions.
- Use ` + "`agentcom init --template company`" + ` to scaffold the template, ` + "`agentcom up`" + ` to start the managed roles, and ` + "`agentcom down`" + ` to stop them cleanly.
- Use ` + "`agentcom register --name <name> --type <type>`" + ` only for low-level manual runs of a single standalone role.
- Prefer direct role-to-role communication for execution details, and keep planning updates visible to the planning role.
- Store structured payloads as JSON so review and architect can audit decisions.
- This template is inspired by Paperclip's company/org model, but uses six delivery-focused roles: frontend, backend, plan, review, architect, and design.`),
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

- Keep the planner, reviewer, and architect roles distinct from implementation roles.
- Use ` + "`agentcom init --template oh-my-opencode`" + ` to scaffold the template, ` + "`agentcom up`" + ` to start the managed roles, and ` + "`agentcom down`" + ` to stop them cleanly.
- Use ` + "`agentcom register --name <name> --type <type>`" + ` only as the advanced/manual path for a single standalone role.
- Use ` + "`agentcom send`" + ` for targeted messages and ` + "`agentcom task`" + ` for explicit handoffs.
- Treat role skills as execution guidance layered on top of the shared agentcom workflow.
- This template references official Oh-My-OpenCode agent patterns such as Prometheus (planning), Momus (review), Oracle (architecture), and Sisyphus-Junior style execution specialists.`),
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
