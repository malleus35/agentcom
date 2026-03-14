package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/malleus35/agentcom/internal/onboard"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var writeAgentsMD bool
	var templateName string
	var setup bool
	var accessible bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize agentcom home and optionally run setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setup {
				if jsonOutput {
					return fmt.Errorf("cli.newInitCmd: --json is not supported with --setup")
				}
				if !isInteractiveInput(cmd.InOrStdin()) {
					return fmt.Errorf("cli.newInitCmd: --setup requires an interactive terminal")
				}

				defaults, err := onboard.DetectDefaults()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: detect onboarding defaults: %w", err)
				}
				if writeAgentsMD {
					defaults.WriteAgentsMD = true
				}
				if templateName != "" {
					defaults.Template = templateName
				}

				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd for setup: %w", err)
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

				for _, path := range report.GeneratedFiles {
					if path == report.AgentsMDPath {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated AGENTS.md at %s\n", path); err != nil {
							return err
						}
						continue
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated template file at %s\n", path); err != nil {
						return err
					}
				}

				if report.Status == "already_initialized" {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "agentcom already initialized at %s\n", report.HomeDir)
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "agentcom initialized at %s\n", report.HomeDir)
				return err
			}

			info, err := os.Stat(app.cfg.HomeDir)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("cli.newInitCmd: stat home: %w", err)
			}

			status := "initialized"
			if err == nil && info.IsDir() {
				status = "already_initialized"
			}

			agentsMDPath := ""
			generatedFiles := []string{}
			if writeAgentsMD {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd: %w", err)
				}
				agentsMDPath = filepath.Join(cwd, "AGENTS.md")
				if err := writeProjectAgentsMD(agentsMDPath); err != nil {
					return fmt.Errorf("cli.newInitCmd: write AGENTS.md: %w", err)
				}
			}

			if templateName != "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: getwd for template scaffold: %w", err)
				}
				generatedFiles, err = writeTemplateScaffold(cwd, templateName)
				if err != nil {
					return fmt.Errorf("cli.newInitCmd: write template scaffold: %w", err)
				}
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				payload := map[string]any{
					"path":   app.cfg.HomeDir,
					"status": status,
				}
				if agentsMDPath != "" {
					payload["agents_md"] = agentsMDPath
				}
				if templateName != "" {
					payload["template"] = templateName
					payload["generated_files"] = generatedFiles
				}

				return enc.Encode(payload)
			}

			if agentsMDPath != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated AGENTS.md at %s\n", agentsMDPath); err != nil {
					return err
				}
			}

			for _, path := range generatedFiles {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "generated template file at %s\n", path); err != nil {
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

	cmd.Flags().BoolVar(&setup, "setup", false, "Run interactive setup wizard for init")
	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use accessible text prompts for setup wizard")
	cmd.Flags().BoolVar(&writeAgentsMD, "agents-md", false, "Generate project AGENTS.md in current directory")
	cmd.Flags().StringVar(&templateName, "template", "", "Generate built-in project scaffold: company|oh-my-opencode")

	return cmd
}

func writeProjectAgentsMD(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("AGENTS.md already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat AGENTS.md: %w", err)
	}

	content := `# AGENTS.md

## agentcom Workflow

- Run ` + "`agentcom init`" + ` once per machine to create the local SQLite database and socket directories.
- Register each long-running agent session with ` + "`agentcom register --name <name> --type <type>`" + `.
- Send direct messages with ` + "`agentcom send --from <sender> <target> <message-or-json>`" + `.
- Broadcast updates with ` + "`agentcom broadcast --from <sender> <message-or-json>`" + `.
- Create and delegate tasks with ` + "`agentcom task create`" + ` and ` + "`agentcom task delegate`" + `.
- Check inbox/status with ` + "`agentcom inbox`" + ` and ` + "`agentcom status`" + `.
- Start MCP mode with ` + "`agentcom mcp-server`" + ` for tool-based integrations.

## Recommended Conventions

- Use stable agent names per worktree or terminal session.
- Keep one registered process per agent name.
- Prefer JSON payloads for structured messages between agents.
- Deregister agents cleanly on shutdown, or let ` + "`register`" + ` auto-clean up on signal.
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}

	return nil
}
