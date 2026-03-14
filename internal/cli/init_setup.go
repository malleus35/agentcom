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
)

var newOnboardPrompter = func(accessible bool, input io.Reader, output io.Writer) onboard.Prompter {
	return onboard.NewHuhPrompter(accessible, input, output)
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

type initSetupExecutor struct {
	projectDir string
}

func newInitSetupExecutor(projectDir string) onboard.Applier {
	return &initSetupExecutor{projectDir: projectDir}
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
		Template: result.Template,
	}

	if result.WriteAgentsMD {
		agentsMDPath := filepath.Join(e.projectDir, "AGENTS.md")
		if err := writeProjectAgentsMD(agentsMDPath); err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: write AGENTS.md: %w", err)
		}
		report.AgentsMDPath = agentsMDPath
		report.GeneratedFiles = append(report.GeneratedFiles, agentsMDPath)
	}

	if result.Template != "" {
		generatedFiles, err := writeTemplateScaffold(e.projectDir, result.Template)
		if err != nil {
			return onboard.ApplyReport{}, fmt.Errorf("cli.initSetupExecutor.Apply: write template scaffold: %w", err)
		}
		report.GeneratedFiles = append(report.GeneratedFiles, generatedFiles...)
	}

	sort.Strings(report.GeneratedFiles)
	return report, nil
}
