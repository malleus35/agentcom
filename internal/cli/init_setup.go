package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/onboard"
	"github.com/spf13/cobra"
)

var newOnboardPrompter = func(accessible bool, advanced bool, input io.Reader, output io.Writer) onboard.Prompter {
	return newInitPrompter(accessible, advanced, input, output)
}

var isInteractiveInput = func(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func shouldRunWizard(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Name() != "init" {
		return false
	}
	if jsonOutput {
		return false
	}
	batch, err := cmd.Flags().GetBool("batch")
	if err == nil && batch {
		return false
	}
	return isInteractiveInput(cmd.InOrStdin())
}

type initSetupExecutor struct {
	projectDir string
	force      bool
	dryRun     bool
}

func newInitSetupExecutor(projectDir string, force bool, dryRun bool) onboard.Applier {
	return &initSetupExecutor{projectDir: projectDir, force: force, dryRun: dryRun}
}

func (e *initSetupExecutor) Apply(ctx context.Context, result onboard.Result) (onboard.ApplyReport, error) {
	status := "initialized"
	if info, err := os.Stat(result.HomeDir); err == nil && info.IsDir() {
		status = "already_initialized"
	} else if err != nil && !os.IsNotExist(err) {
		return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: stat home: %w", err)
	}

	cfg := &config.Config{
		HomeDir:     result.HomeDir,
		DBPath:      filepath.Join(result.HomeDir, config.DBFileName),
		SocketsPath: filepath.Join(result.HomeDir, config.SocketsDir),
	}
	report := onboard.ApplyReport{
		HomeDir:  cfg.HomeDir,
		DBPath:   cfg.DBPath,
		Status:   status,
		DryRun:   e.dryRun,
		Project:  result.Project,
		Template: result.Template,
	}

	mode := writeModeAppend
	if e.force {
		mode = writeModeOverwrite
	}
	if e.dryRun {
		report.PreviewActions = append(report.PreviewActions, onboard.PreviewAction{Action: "create", Path: cfg.DBPath})
		previewActions, generatedFiles, instructionFiles, memoryFiles, agentsMDPath, projectConfigPath, customTemplatePath, err := previewInitSetup(e.projectDir, result, mode)
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: preview init setup: %w", err)
		}
		report.PreviewActions = append(report.PreviewActions, previewActions...)
		report.GeneratedFiles = append(report.GeneratedFiles, generatedFiles...)
		report.InstructionFiles = append(report.InstructionFiles, instructionFiles...)
		report.MemoryFiles = append(report.MemoryFiles, memoryFiles...)
		report.AgentsMDPath = agentsMDPath
		report.ProjectConfigPath = projectConfigPath
		report.CustomTemplatePath = customTemplatePath
		sort.Strings(report.GeneratedFiles)
		return report, nil
	}

	if err := cfg.EnsureDirs(); err != nil {
		return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: ensure dirs: %w", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: open db: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: migrate: %w", err)
	}

	if err := database.EnsureProject(ctx, result.Project); err != nil {
		return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: ensure project: %w", err)
	}

	if result.Project != "" || result.Template != "" {
		path, err := config.SaveProjectConfig(e.projectDir, config.ProjectConfig{
			Project: result.Project,
			Template: config.ProjectTemplateConfig{
				Active: result.Template,
			},
		})
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: save project config: %w", err)
		}
		report.ProjectConfigPath = path
		report.GeneratedFiles = append(report.GeneratedFiles, path)
	}

	if result.CustomTemplate != nil {
		customTemplatePath, err := saveCustomTemplate(e.projectDir, templateDefinitionFromOnboard(*result.CustomTemplate))
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: save custom template: %w", err)
		}
		report.CustomTemplatePath = customTemplatePath
		report.GeneratedFiles = append(report.GeneratedFiles,
			filepath.Join(customTemplatePath, "COMMON.md"),
			filepath.Join(customTemplatePath, "template.json"),
		)
	}

	if result.WriteInstructions || result.WriteAgentsMD {
		selectedAgents := append([]string(nil), result.SelectedAgents...)
		if len(selectedAgents) == 0 && result.WriteAgentsMD {
			selectedAgents = []string{"codex"}
		}

		instructionFiles, err := writeAgentInstructions(e.projectDir, selectedAgents, mode)
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: write instruction files: %w", err)
		}
		report.InstructionFiles = append(report.InstructionFiles, instructionFiles...)
		report.GeneratedFiles = append(report.GeneratedFiles, instructionFiles...)
		for _, path := range instructionFiles {
			if filepath.Base(path) == "AGENTS.md" {
				report.AgentsMDPath = path
				break
			}
		}
	}

	if result.WriteMemory {
		memoryFiles, err := writeAgentMemoryFiles(e.projectDir, result.SelectedAgents, mode)
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: write memory files: %w", err)
		}
		report.MemoryFiles = append(report.MemoryFiles, memoryFiles...)
		report.GeneratedFiles = append(report.GeneratedFiles, memoryFiles...)
	}

	if result.Template != "" {
		generatedFiles, err := writeTemplateScaffold(e.projectDir, result.Template, mode)
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: write template scaffold: %w", err)
		}
		report.GeneratedFiles = append(report.GeneratedFiles, generatedFiles...)
	}

	sort.Strings(report.GeneratedFiles)
	return report, nil
}

func previewInitSetup(projectDir string, result onboard.Result, mode writeMode) ([]onboard.PreviewAction, []string, []string, []string, string, string, string, error) {
	preview := make([]onboard.PreviewAction, 0)
	generatedFiles := make([]string, 0)
	instructionFiles := make([]string, 0)
	memoryFiles := make([]string, 0)
	agentsMDPath := ""
	projectConfigPath := ""
	customTemplatePath := ""

	if result.Project != "" || result.Template != "" {
		path := filepath.Join(projectDir, config.ProjectConfigFileName)
		action := previewActionForPath(path, writeModeAppend)
		if _, existingPath, err := config.LoadProjectConfig(projectDir); err == nil && existingPath == "" {
			action = previewActionForPath(path, mode)
		}
		if _, err := os.Stat(path); err == nil {
			if mode == writeModeOverwrite {
				action = "overwrite"
			} else {
				action = "update"
			}
		}
		projectConfigPath = path
		generatedFiles = append(generatedFiles, path)
		preview = append(preview, onboard.PreviewAction{Action: action, Path: path})
	}

	if result.CustomTemplate != nil {
		definition := templateDefinitionFromOnboard(*result.CustomTemplate)
		baseDir := filepath.Join(projectDir, ".agentcom", "templates", definition.Name)
		customTemplatePath = baseDir
		for _, path := range []string{filepath.Join(baseDir, "COMMON.md"), filepath.Join(baseDir, "template.json")} {
			preview = append(preview, onboard.PreviewAction{Action: previewActionForPath(path, mode), Path: path})
			generatedFiles = append(generatedFiles, path)
		}
	}

	if result.WriteInstructions || result.WriteAgentsMD {
		selectedAgents := append([]string(nil), result.SelectedAgents...)
		if len(selectedAgents) == 0 && result.WriteAgentsMD {
			selectedAgents = []string{"codex"}
		}
		seenPaths := make(map[string]struct{})
		for _, agentID := range selectedAgents {
			definition, ok := findInstructionDefinition(agentID)
			if !ok {
				return nil, nil, nil, nil, "", "", "", fmt.Errorf("unsupported instruction agent %q", agentID)
			}
			path := filepath.Join(projectDir, definition.RelativePath)
			if _, ok := seenPaths[path]; ok {
				continue
			}
			seenPaths[path] = struct{}{}
			instructionFiles = append(instructionFiles, path)
			generatedFiles = append(generatedFiles, path)
			preview = append(preview, onboard.PreviewAction{Action: previewActionForPath(path, mode), Path: path})
			if filepath.Base(path) == "AGENTS.md" {
				agentsMDPath = path
			}
		}
	}

	if result.WriteMemory {
		seen := make(map[string]struct{})
		for _, agentID := range result.SelectedAgents {
			definition, ok := findInstructionDefinition(agentID)
			if !ok || !definition.SupportsMemory {
				continue
			}
			path := filepath.Join(projectDir, definition.MemoryRelativePath)
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			memoryFiles = append(memoryFiles, path)
			generatedFiles = append(generatedFiles, path)
			preview = append(preview, onboard.PreviewAction{Action: previewActionForPath(path, mode), Path: path})
		}
	}

	if result.Template != "" {
		paths, err := previewTemplateScaffold(projectDir, result.Template, mode)
		if err != nil {
			return nil, nil, nil, nil, "", "", "", err
		}
		for _, action := range paths {
			preview = append(preview, action)
			generatedFiles = append(generatedFiles, action.Path)
		}
	}

	sort.Strings(generatedFiles)
	sort.Strings(instructionFiles)
	sort.Strings(memoryFiles)
	return preview, generatedFiles, instructionFiles, memoryFiles, agentsMDPath, projectConfigPath, customTemplatePath, nil
}

func previewTemplateScaffold(projectDir string, templateName string, mode writeMode) ([]onboard.PreviewAction, error) {
	definition, err := resolveTemplateDefinition(templateName)
	if err != nil {
		return nil, err
	}
	paths := []string{
		filepath.Join(projectDir, ".agentcom", "templates", definition.Name, "COMMON.md"),
		filepath.Join(projectDir, ".agentcom", "templates", definition.Name, "template.json"),
	}
	sharedTargets, err := resolveTemplateSkillTargets("project", "agentcom")
	if err != nil {
		return nil, err
	}
	for _, target := range sharedTargets {
		paths = append(paths, target.Path)
	}
	for _, role := range definition.Roles {
		targets, err := resolveTemplateSkillTargets("project", filepath.Join("agentcom", templateRoleSkillName(definition.Name, role.Name)))
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			paths = append(paths, target.Path)
		}
	}
	actions := make([]onboard.PreviewAction, 0, len(paths))
	for _, path := range paths {
		action := previewActionForPath(path, mode)
		if (strings.HasSuffix(path, "COMMON.md") || strings.HasSuffix(path, "template.json")) && action == "update" {
			continue
		}
		actions = append(actions, onboard.PreviewAction{Action: action, Path: path})
	}
	return actions, nil
}

func previewActionForPath(path string, mode writeMode) string {
	if _, err := os.Stat(path); err == nil {
		switch mode {
		case writeModeOverwrite:
			return "overwrite"
		default:
			return "update"
		}
	}
	return "create"
}
