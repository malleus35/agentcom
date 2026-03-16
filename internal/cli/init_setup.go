package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

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
}

func newInitSetupExecutor(projectDir string, force bool) onboard.Applier {
	return &initSetupExecutor{projectDir: projectDir, force: force}
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

	report := onboard.ApplyReport{
		HomeDir:  cfg.HomeDir,
		DBPath:   cfg.DBPath,
		Status:   status,
		Project:  result.Project,
		Template: result.Template,
	}

	mode := writeModeAppend
	if e.force {
		mode = writeModeOverwrite
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
