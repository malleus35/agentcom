package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/mcp"
	"github.com/spf13/cobra"
)

// newMCPServerCmd creates the mcp-server command.
func newMCPServerCmd() *cobra.Command {
	var registerName string
	var registerType string

	cmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Run MCP JSON-RPC server over STDIO",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var (
				registry     *agent.Registry
				registered   bool
				registeredID string
			)

			if registerName != "" {
				if registerType == "" {
					return fmt.Errorf("cli.newMCPServerCmd: --type is required when --register is set")
				}

				registry = agent.NewRegistry(app.db, app.cfg)
				agentRecord, err := registry.Register(ctx, registerName, registerType, []string{"mcp", "tools"}, "")
				if err != nil {
					return fmt.Errorf("cli.newMCPServerCmd: register: %w", err)
				}

				registered = true
				registeredID = agentRecord.ID

				heartbeat := agent.NewHeartbeat(app.db, agentRecord.ID)
				heartbeat.Start(ctx)

				defer func() {
					deregisterCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					if err := registry.Deregister(deregisterCtx, registeredID); err != nil {
						slog.Error("failed to deregister mcp server agent", "agent_id", registeredID, "error", err)
					}
				}()
			}

			server := mcp.NewServer(app.db, app.cfg)
			if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
				if registered {
					slog.Debug("mcp server stopped with registered agent", "agent_id", registeredID, "error", err)
				}
				return fmt.Errorf("cli.newMCPServerCmd: run: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&registerName, "register", "", "Register this MCP server as agent with given name")
	cmd.Flags().StringVar(&registerType, "type", "", "Agent type used with --register")

	return cmd
}
