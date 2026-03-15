// Package cli defines all cobra commands for the agentcom CLI.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
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
	cfg *config.Config
	db  *db.DB
}

var (
	// Global flags
	jsonOutput bool
	verbose    bool

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

	// Register subcommands
	root.AddCommand(
		newInitCmd(),
		newAgentsCmd(),
		newSkillCmd(),
		newVersionCmd(),
		newRegisterCmd(),
		newDeregisterCmd(),
		newListCmd(),
		newSendCmd(),
		newBroadcastCmd(),
		newInboxCmd(),
		newTaskCmd(),
		newStatusCmd(),
		newHealthCmd(),
		newMCPServerCmd(),
	)

	return root
}

func shouldSkipAppInit(cmd *cobra.Command) bool {
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

	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("cli.initApp: %w", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("cli.initApp: %w", err)
	}

	if err := database.Migrate(ctx); err != nil {
		database.Close()
		return fmt.Errorf("cli.initApp: migrate: %w", err)
	}

	app = &appContext{
		cfg: cfg,
		db:  database,
	}

	slog.Debug("agentcom initialized", "db", cfg.DBPath, "sockets", cfg.SocketsPath)
	return nil
}

// shutdownApp cleans up shared application state.
func shutdownApp() error {
	if app != nil && app.db != nil {
		return app.db.Close()
	}
	return nil
}
