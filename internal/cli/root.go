// Package cli defines all cobra commands for the agentcom CLI.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/transport"
	"github.com/spf13/cobra"
)

// Build-time variables set via ldflags.
var (
	Version   = "dev"
	BuildDate = "unknown"
	GoVersion = "unknown"
)

// appContext holds shared application state initialized in PersistentPreRunE.
type appContext struct {
	cfg         *config.Config
	db          *db.DB
	project     string
	allProjects bool
}

var (
	// Global flags
	jsonOutput  bool
	verbose     bool
	projectFlag string
	allProjects bool

	// Shared state
	app *appContext
)

// NewRootCmd creates the root cobra command for agentcom.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "agentcom",
		Short:         "Parallel AI agent communication tool",
		Long:          `agentcom enables real-time communication between parallel AI coding agent sessions via SQLite and Unix Domain Sockets.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if shouldSkipAppInit(cmd) {
				configureLogging()
				return nil
			}
			return initApp(cmd.Context())
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return shutdownApp()
		},
	}

	pf := root.PersistentFlags()
	pf.BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	pf.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	pf.StringVar(&projectFlag, "project", "", "Project name override")
	pf.BoolVar(&allProjects, "all-projects", false, "Show resources from all projects")

	// Register subcommands
	root.AddCommand(
		newInitCmd(),
		newAgentsCmd(),
		newSkillCmd(),
		newVersionCmd(),
		newUpCmd(),
		newDownCmd(),
		newUpSupervisorCmd(),
		newRegisterCmd(),
		newDeregisterCmd(),
		newListCmd(),
		newSendCmd(),
		newBroadcastCmd(),
		newInboxCmd(),
		newUserCmd(),
		newTaskCmd(),
		newStatusCmd(),
		newHealthCmd(),
		newDoctorCmd(),
		newMCPServerCmd(),
	)

	return root
}

func shouldSkipAppInit(cmd *cobra.Command) bool {
	if cmd != nil && cmd.Name() == "init" {
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err == nil && dryRun {
			configureLogging()
			return true
		}
	}
	return shouldRunWizard(cmd)
}

func configureLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// initApp initializes shared application state.
func initApp(ctx context.Context) error {
	configureLogging()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cli.initApp: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cli.initApp: getwd: %w", err)
	}
	project, err := config.ResolveProject(projectFlag, cwd)
	if err != nil {
		return fmt.Errorf("cli.initApp: resolve project: %w", err)
	}

	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("cli.initApp: %w", err)
	}
	transport.ApplyRuntimeConfig(cfg.Runtime)
	transport.ApplyPollerRuntimeConfig(cfg.Runtime)
	agent.ApplyHeartbeatRuntimeConfig(cfg.Runtime)
	agent.ApplyRegistryRuntimeConfig(cfg.Runtime)
	applyCLIRuntimeConfig(cfg.Runtime)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("cli.initApp: %w", err)
	}

	if err := database.Migrate(ctx); err != nil {
		database.Close()
		return fmt.Errorf("cli.initApp: migrate: %w", err)
	}

	app = &appContext{
		cfg:         cfg,
		db:          database,
		project:     project,
		allProjects: allProjects,
	}

	slog.Debug("agentcom initialized", "db", cfg.DBPath, "sockets", cfg.SocketsPath)
	return nil
}

func applyCLIRuntimeConfig(runtime config.RuntimeConfig) {
	upSupervisorHealthCheckInterval = runtime.SupervisorHealthCheckInterval
}

func currentProjectFilter() string {
	if app == nil || app.allProjects {
		return ""
	}
	return app.project
}

// shutdownApp cleans up shared application state.
func shutdownApp() error {
	if app != nil && app.db != nil {
		return app.db.Close()
	}
	return nil
}
