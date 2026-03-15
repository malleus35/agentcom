package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/onboard"
	"github.com/spf13/cobra"
)

const (
	promptInstructionSelection = "__prompt__"
	promptTemplateSelection    = "__prompt__"
)

func newInitCmd() *cobra.Command {
	var batch bool
	var agentsValue string
	var templateName string
	var accessible bool
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize agentcom home and optionally run onboarding wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentsSelection, templateSelection, remainingArgs := consumeInitOptionalValues(agentsValue, templateName, args)
			if len(remainingArgs) > 0 {
				return fmt.Errorf("cli.newInitCmd: unexpected arguments: %s", strings.Join(remainingArgs, ", "))
			}

			if shouldRunWizard(cmd) {
				defaults, err := onboard.DetectDefaults()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: detect onboarding defaults: %w", err)
				}

				if agentsSelection != "" {
					defaults.WriteAgentsMD = true
				}
				if agentsSelection != "" && agentsSelection != promptInstructionSelection {
					defaults.SelectedAgents, err = resolveInstructionAgents(agentsSelection)
					if err != nil {
						return fmt.Errorf("cli.newInitCmd: resolve instruction agents: %w", err)
					}
				}
				if templateSelection != "" && templateSelection != promptTemplateSelection {
					defaults.Template = templateSelection
				}

				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd for setup: %w", err)
				}
				defaults.Project, err = defaultInitProject(cwd)
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: default project: %w", err)
				}
				if projectFlag != "" {
					defaults.Project = projectFlag
				}

				wizard := onboard.NewWizard(
					newOnboardPrompter(accessible, cmd.InOrStdin(), cmd.OutOrStdout()),
					newInitSetupExecutor(cwd),
				)
				report, err := wizard.Run(cmd.Context(), defaults)
				if err != nil {
					if errors.Is(err, onboard.ErrAborted) {
						return fmt.Errorf("cli.newInitCmd: setup cancelled")
					}
					return fmt.Errorf("cli.newInitCmd: run setup wizard: %w", err)
				}

				return writeInitReport(cmd, report)
			}

			info, err := os.Stat(app.cfg.HomeDir)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("cli.newInitCmd: stat home: %w", err)
			}

			status := "initialized"
			if err == nil && info.IsDir() {
				status = "already_initialized"
			}

			instructionFiles := []string{}
			agentsMDPath := ""
			generatedFiles := []string{}
			if agentsSelection != "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd: %w", err)
				}

				selectedAgents := []string{"codex"}
				if agentsSelection != promptInstructionSelection {
					selectedAgents, err = resolveInstructionAgents(agentsSelection)
					if err != nil {
						return fmt.Errorf("cli.newInitCmd: resolve instruction agents: %w", err)
					}
				}
				instructionFiles, err = writeAgentInstructions(cwd, selectedAgents)
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: write instruction files: %w", err)
				}
				for _, path := range instructionFiles {
					if filepath.Base(path) == "AGENTS.md" {
						agentsMDPath = path
						break
					}
				}
			}

			if templateSelection != "" && templateSelection != promptTemplateSelection {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd for template scaffold: %w", err)
				}
				generatedFiles, err = writeTemplateScaffold(cwd, templateSelection)
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: write template scaffold: %w", err)
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cli.newInitCmd: getwd for project config: %w", err)
			}
			projectConfigPath, projectCfg, err := ensureInitProjectConfig(cwd, force, templateSelection)
			if err != nil {
				return fmt.Errorf("cli.newInitCmd: ensure project config: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				payload := map[string]any{
					"path":   app.cfg.HomeDir,
					"status": status,
				}
				if len(instructionFiles) > 0 {
					payload["instruction_files"] = instructionFiles
				}
				if agentsMDPath != "" {
					payload["agents_md"] = agentsMDPath
				}
				if templateSelection != "" && templateSelection != promptTemplateSelection {
					payload["template"] = templateSelection
					payload["generated_files"] = generatedFiles
				}
				if projectCfg.Project != "" {
					payload["project"] = projectCfg.Project
				}
				if projectCfg.Template.Active != "" {
					payload["active_template"] = projectCfg.Template.Active
				}
				if projectConfigPath != "" {
					payload["project_config_path"] = projectConfigPath
				}

				return enc.Encode(payload)
			}

			for _, path := range instructionFiles {
				label := "instruction file"
				if filepath.Base(path) == "AGENTS.md" {
					label = "AGENTS.md"
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated %s at %s\n", label, path); err != nil {
					return err
				}
			}

			for _, path := range generatedFiles {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated template file at %s\n", path); err != nil {
					return err
				}
			}
			if projectConfigPath != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated project config at %s\n", projectConfigPath); err != nil {
					return err
				}
			}

			if status == "already_initialized" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "agentcom already initialized at %s\n", app.cfg.HomeDir)
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "agentcom initialized at %s\n", app.cfg.HomeDir)
			return err
		},
	}

	cmd.Flags().BoolVar(&batch, "batch", false, "Run init without the onboarding wizard")
	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use accessible text prompts for setup wizard")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite project configuration if it already exists")
	cmd.Flags().StringVar(&agentsValue, "agents-md", "", "Generate agent instruction files in the current directory")
	cmd.Flags().StringVar(&templateName, "template", "", "Generate a project scaffold: company|oh-my-opencode|custom")
	if flag := cmd.Flags().Lookup("agents-md"); flag != nil {
		flag.NoOptDefVal = promptInstructionSelection
	}
	if flag := cmd.Flags().Lookup("template"); flag != nil {
		flag.NoOptDefVal = promptTemplateSelection
	}

	return cmd
}

func writeInitReport(cmd *cobra.Command, report onboard.ApplyReport) error {
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")

		payload := map[string]any{
			"path":   report.HomeDir,
			"status": report.Status,
		}
		if len(report.InstructionFiles) > 0 {
			payload["instruction_files"] = report.InstructionFiles
		}
		if report.AgentsMDPath != "" {
			payload["agents_md"] = report.AgentsMDPath
		}
		if report.Template != "" {
			payload["template"] = report.Template
		}
		if report.Project != "" {
			payload["project"] = report.Project
		}
		if report.ProjectConfigPath != "" {
			payload["project_config_path"] = report.ProjectConfigPath
		}
		if len(report.GeneratedFiles) > 0 {
			payload["generated_files"] = report.GeneratedFiles
		}
		if len(report.MemoryFiles) > 0 {
			payload["memory_files"] = report.MemoryFiles
		}
		if report.CustomTemplatePath != "" {
			payload["custom_template_path"] = report.CustomTemplatePath
		}

		return enc.Encode(payload)
	}

	for _, path := range report.InstructionFiles {
		label := "instruction file"
		if filepath.Base(path) == "AGENTS.md" {
			label = "AGENTS.md"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated %s at %s\n", label, path); err != nil {
			return err
		}
	}
	for _, path := range report.MemoryFiles {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated memory file at %s\n", path); err != nil {
			return err
		}
	}
	if report.CustomTemplatePath != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "saved custom template at %s\n", report.CustomTemplatePath); err != nil {
			return err
		}
	}
	if report.ProjectConfigPath != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated project config at %s\n", report.ProjectConfigPath); err != nil {
			return err
		}
	}
	for _, path := range report.GeneratedFiles {
		if isAlreadyListedInitPath(path, report) {
			continue
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated template file at %s\n", path); err != nil {
			return err
		}
	}

	message := "agentcom initialized at %s\n"
	if report.Status == "already_initialized" {
		message = "agentcom already initialized at %s\n"
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), message, report.HomeDir)
	return err
}

func isAlreadyListedInitPath(path string, report onboard.ApplyReport) bool {
	for _, listed := range report.InstructionFiles {
		if listed == path {
			return true
		}
	}
	for _, listed := range report.MemoryFiles {
		if listed == path {
			return true
		}
	}
	return false
}

func consumeInitOptionalValues(agentsValue, templateValue string, args []string) (string, string, []string) {
	remaining := append([]string(nil), args...)
	agentsSelection := agentsValue
	templateSelection := templateValue

	if agentsSelection == promptInstructionSelection && len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
		agentsSelection = remaining[0]
		remaining = remaining[1:]
	}
	if templateSelection == promptTemplateSelection && len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
		templateSelection = remaining[0]
		remaining = remaining[1:]
	}

	return agentsSelection, templateSelection, remaining
}

func defaultInitProject(projectDir string) (string, error) {
	if projectFlag != "" {
		return projectFlag, nil
	}
	project, err := config.ResolveProject("", projectDir)
	if err != nil {
		return "", fmt.Errorf("cli.defaultInitProject: %w", err)
	}
	if project != "" {
		return project, nil
	}

	suggested := strings.ToLower(filepath.Base(projectDir))
	if err := config.ValidateProjectName(suggested); err != nil {
		return "", nil
	}
	return suggested, nil
}

func ensureInitProjectConfig(projectDir string, force bool, templateSelection string) (string, config.ProjectConfig, error) {
	trimmedTemplate := strings.TrimSpace(templateSelection)
	if trimmedTemplate == promptTemplateSelection {
		trimmedTemplate = ""
	}

	path := filepath.Join(projectDir, config.ProjectConfigFileName)
	existingCfg, existingPath, err := config.LoadProjectConfig(projectDir)
	if err != nil {
		return "", config.ProjectConfig{}, fmt.Errorf("cli.ensureInitProjectConfig: %w", err)
	}

	targetCfg := existingCfg
	if strings.TrimSpace(projectFlag) != "" {
		targetCfg.Project = projectFlag
	}
	if trimmedTemplate != "" {
		targetCfg.Template.Active = trimmedTemplate
	}

	shouldWrite := force || existingPath == "" || strings.TrimSpace(projectFlag) != "" || trimmedTemplate != ""
	if !shouldWrite {
		return existingPath, existingCfg, nil
	}

	if force {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", config.ProjectConfig{}, fmt.Errorf("cli.ensureInitProjectConfig: remove existing: %w", err)
		}
	}

	writtenPath, err := config.SaveProjectConfig(projectDir, targetCfg)
	if err != nil {
		return "", config.ProjectConfig{}, fmt.Errorf("cli.ensureInitProjectConfig: %w", err)
	}
	return writtenPath, targetCfg, nil
}
