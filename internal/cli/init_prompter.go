package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/onboard"
)

type initPrompter struct {
	accessible bool
	input      io.Reader
	output     io.Writer
}

const (
	agentToolsTitle       = "Agent tools"
	agentToolsDescription = "Select the agents you want to generate project instructions for. Space to select, Enter to continue."
	agentToolsError       = "select at least one agent tool to continue"
	projectNameError      = "project name is required"
)

func validateAgentToolsSelection(value []string) error {
	if len(value) == 0 {
		return errors.New(agentToolsError)
	}
	return nil
}

func defaultWizardProjectName(projectDir string, configured string) string {
	trimmed := strings.TrimSpace(configured)
	if trimmed != "" {
		return trimmed
	}

	suggested := strings.ToLower(filepath.Base(projectDir))
	if err := config.ValidateProjectName(suggested); err != nil {
		return ""
	}
	return suggested
}

func normalizeWizardProjectName(current string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(current)
}

func validateWizardProjectName(projectDir string, homeDir string, value string) error {
	trimmed := normalizeWizardProjectName("", value)
	if trimmed == "" {
		return errors.New(projectNameError)
	}
	if err := config.ValidateProjectName(trimmed); err != nil {
		return err
	}

	existingCfg, err := loadExactProjectConfig(projectDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if existingCfg.Project == trimmed {
		return nil
	}

	projects, err := knownProjectNames(homeDir)
	if err != nil {
		return fmt.Errorf("list known projects: %w", err)
	}
	if _, ok := projects[trimmed]; ok {
		return fmt.Errorf("project %q already exists", trimmed)
	}

	return nil
}

func loadExactProjectConfig(projectDir string) (config.ProjectConfig, error) {
	path := filepath.Join(projectDir, config.ProjectConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config.ProjectConfig{}, nil
		}
		return config.ProjectConfig{}, err
	}

	var cfg config.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config.ProjectConfig{}, err
	}
	return cfg, nil
}

func knownProjectNames(homeDir string) (map[string]struct{}, error) {
	dbPath := filepath.Join(homeDir, config.DBFileName)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer database.Close()

	projects, err := database.ListProjects(context.Background())
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		result[project] = struct{}{}
	}
	return result, nil
}

func newInitPrompter(accessible bool, input io.Reader, output io.Writer) onboard.Prompter {
	return &initPrompter{accessible: accessible, input: input, output: output}
}

func (p *initPrompter) Run(ctx context.Context, defaults onboard.Result) (onboard.Result, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return onboard.Result{}, fmt.Errorf("cli.initPrompter.Run: getwd: %w", err)
	}

	templates, err := allTemplateDefinitions(projectDir)
	if err != nil {
		return onboard.Result{}, fmt.Errorf("cli.initPrompter.Run: load templates: %w", err)
	}

	homeDir := defaults.HomeDir
	projectName := defaultWizardProjectName(projectDir, defaults.Project)
	selectedAgents := append([]string(nil), defaults.SelectedAgents...)
	writeInstructions := defaults.WriteInstructions || defaults.WriteAgentsMD || len(selectedAgents) > 0
	writeMemory := defaults.WriteMemory
	templateChoice := defaults.Template
	if templateChoice == "" {
		templateChoice = "none"
	}
	confirmed := defaults.Confirmed

	summary := func() string {
		agents := "none"
		if len(selectedAgents) > 0 {
			agents = strings.Join(selectedAgents, ", ")
		}
		instructions := "no"
		if writeInstructions {
			instructions = "yes"
		}
		memory := "no"
		if writeMemory {
			memory = "yes"
		}
		return fmt.Sprintf("home: %s\nproject: %s\nagents: %s\nwrite instructions: %s\nwrite memory: %s\ntemplate: %s", homeDir, projectName, agents, instructions, memory, templateChoice)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("agentcom setup").Description("Prepare your local agentcom home, agent instructions, and optional project scaffold."),
			huh.NewInput().
				Title("Agentcom home directory").
				Placeholder(filepath.Clean(homeDir)).
				Value(&homeDir).
				Validate(func(value string) error {
					if strings.TrimSpace(value) == "" {
						return errors.New("home directory is required")
					}
					if !filepath.IsAbs(value) {
						return errors.New("home directory must be an absolute path")
					}
					return nil
				}),
		).Title("Step 1: Environment"),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(agentToolsTitle).
				Description(agentToolsDescription).
				Options(initInstructionOptions(selectedAgents)...).
				Validate(validateAgentToolsSelection).
				Value(&selectedAgents),
		).Title("Step 2: Agent Tools"),
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Placeholder(defaultWizardProjectName(projectDir, "")).
				Value(&projectName).
				Validate(func(value string) error {
					return validateWizardProjectName(projectDir, homeDir, normalizeWizardProjectName(projectName, value))
				}),
			huh.NewConfirm().
				Title("Generate instruction files for the selected agents?").
				Affirmative("Yes").
				Negative("No").
				Value(&writeInstructions),
			huh.NewConfirm().
				Title("Generate memory files where supported?").
				Affirmative("Yes").
				Negative("No").
				Value(&writeMemory),
		).Title("Step 3: Project Instructions"),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Project template").
				Options(initTemplateOptions(templates)...).
				Value(&templateChoice),
		).Title("Step 4: Template"),
		huh.NewGroup(
			huh.NewNote().Title("Review selections").DescriptionFunc(summary, []any{&homeDir, &projectName, &selectedAgents, &writeInstructions, &writeMemory, &templateChoice}),
			huh.NewConfirm().
				Title("Apply these settings?").
				Affirmative("Apply").
				Negative("Cancel").
				Value(&confirmed),
		).Title("Step 5: Confirm"),
	).
		WithAccessible(p.accessible).
		WithInput(p.input).
		WithOutput(p.output)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return onboard.Result{}, onboard.ErrAborted
		}
		return onboard.Result{}, fmt.Errorf("cli.initPrompter.Run: %w", err)
	}

	if !writeInstructions {
		selectedAgents = nil
		writeMemory = false
	}

	var customTemplate *onboard.TemplateDefinition
	if templateChoice == "custom" {
		customTemplate, err = p.runCustomTemplateWizard(ctx, templates)
		if err != nil {
			return onboard.Result{}, err
		}
		templateChoice = customTemplate.Name
	}
	if templateChoice == "none" {
		templateChoice = ""
	}

	return onboard.Result{
		HomeDir:           homeDir,
		Project:           strings.TrimSpace(projectName),
		Template:          templateChoice,
		WriteAgentsMD:     containsString(selectedAgents, "codex"),
		SelectedAgents:    selectedAgents,
		WriteMemory:       writeMemory,
		WriteInstructions: writeInstructions,
		CustomTemplate:    customTemplate,
		Confirmed:         confirmed,
	}, nil
}

func (p *initPrompter) runCustomTemplateWizard(ctx context.Context, existing []templateDefinition) (*onboard.TemplateDefinition, error) {
	name := ""
	description := ""
	reference := "local"
	commonTitle := "Custom Template Common Instructions"
	commonBody := "Coordinate through agentcom."

	basicForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Template name").Value(&name),
			huh.NewInput().Title("Description").Value(&description),
			huh.NewInput().Title("Reference").Value(&reference),
		).Title("Custom Template: Basics"),
	).WithAccessible(p.accessible).WithInput(p.input).WithOutput(p.output)
	if err := basicForm.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, onboard.ErrAborted
		}
		return nil, fmt.Errorf("cli.initPrompter.runCustomTemplateWizard: %w", err)
	}

	roles := make([]onboard.TemplateRole, 0, 1)
	for {
		roleName := ""
		roleDescription := ""
		agentName := ""
		agentType := ""
		responsibilities := ""
		communicatesWith := ""
		addAnother := false

		roleForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Role name").Value(&roleName),
				huh.NewInput().Title("Role description").Value(&roleDescription),
				huh.NewInput().Title("Agent name").Value(&agentName),
				huh.NewInput().Title("Agent type").Value(&agentType),
				huh.NewInput().Title("Responsibilities (comma separated)").Value(&responsibilities),
				huh.NewInput().Title("Communicates with (comma separated)").Value(&communicatesWith),
				huh.NewConfirm().Title("Add another role?").Affirmative("Yes").Negative("No").Value(&addAnother),
			).Title(fmt.Sprintf("Custom Template: Role %d", len(roles)+1)),
		).WithAccessible(p.accessible).WithInput(p.input).WithOutput(p.output)
		if err := roleForm.RunWithContext(ctx); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil, onboard.ErrAborted
			}
			return nil, fmt.Errorf("cli.initPrompter.runCustomTemplateWizard: %w", err)
		}

		roles = append(roles, onboard.TemplateRole{
			Name:             strings.TrimSpace(roleName),
			Description:      strings.TrimSpace(roleDescription),
			AgentName:        strings.TrimSpace(agentName),
			AgentType:        strings.TrimSpace(agentType),
			Responsibilities: splitCSVValues(responsibilities),
			CommunicatesWith: splitCSVValues(communicatesWith),
		})

		if !addAnother {
			break
		}
	}

	commonForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Common title").Value(&commonTitle),
			huh.NewText().Title("Common instructions").Value(&commonBody),
		).Title("Custom Template: Common Instructions"),
	).WithAccessible(p.accessible).WithInput(p.input).WithOutput(p.output)
	if err := commonForm.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, onboard.ErrAborted
		}
		return nil, fmt.Errorf("cli.initPrompter.runCustomTemplateWizard: %w", err)
	}

	definition := &onboard.TemplateDefinition{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Reference:   strings.TrimSpace(reference),
		CommonTitle: strings.TrimSpace(commonTitle),
		CommonBody:  strings.TrimSpace(commonBody),
		Roles:       roles,
	}
	if err := validateCustomTemplateDefinition(templateDefinitionFromOnboard(*definition)); err != nil {
		return nil, fmt.Errorf("cli.initPrompter.runCustomTemplateWizard: %w", err)
	}
	for _, item := range existing {
		if item.Name == definition.Name {
			return nil, fmt.Errorf("cli.initPrompter.runCustomTemplateWizard: template %q already exists", definition.Name)
		}
	}
	return definition, nil
}

func initInstructionOptions(selected []string) []huh.Option[string] {
	definitions := append([]instructionFileDefinition(nil), instructionFileDefinitions...)
	sort.SliceStable(definitions, func(i, j int) bool {
		return instructionPriority(definitions[i].AgentID) < instructionPriority(definitions[j].AgentID)
	})

	options := make([]huh.Option[string], 0, len(definitions))
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.AgentID == "universal" {
			continue
		}
		if _, ok := seen[definition.AgentID]; ok {
			continue
		}
		seen[definition.AgentID] = struct{}{}
		label := titleWords(strings.ReplaceAll(definition.AgentID, "-", " "))
		option := huh.NewOption(label, definition.AgentID)
		if containsString(selected, definition.AgentID) {
			option = option.Selected(true)
		}
		options = append(options, option)
	}
	return options
}

func initTemplateOptions(definitions []templateDefinition) []huh.Option[string] {
	options := []huh.Option[string]{huh.NewOption("None", "none")}
	for _, definition := range definitions {
		options = append(options, huh.NewOption(titleWords(strings.ReplaceAll(definition.Name, "-", " ")), definition.Name))
	}
	options = append(options, huh.NewOption("Create custom template...", "custom"))
	return options
}

func instructionPriority(agentID string) int {
	switch agentID {
	case "claude", "codex", "gemini", "cursor", "github-copilot", "windsurf", "cline", "roo-code", "amazon-q", "augment-code", "continue", "kilo-code", "trae", "goose":
		return 0
	default:
		return 1
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func splitCSVValues(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}
