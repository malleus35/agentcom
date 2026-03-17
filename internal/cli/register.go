package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/transport"
	"github.com/spf13/cobra"
)

func newRegisterCmd() *cobra.Command {
	var name string
	var agentType string
	var capabilitiesRaw string
	var workdir string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register current process as an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workdir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("cli.newRegisterCmd: getwd: %w", err)
				}
				workdir = cwd
			}

			caps := parseCapabilities(capabilitiesRaw)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			registry := agent.NewRegistry(app.db, app.cfg)
			agt, err := registry.Register(ctx, name, agentType, caps, workdir, app.project)
			if err != nil {
				if errors.Is(err, agent.ErrInvalidAgentName) {
					return newUserError(
						fmt.Sprintf("Agent name %q is invalid", name),
						"Agent names must start with a letter or number, may contain only letters, numbers, underscores, or hyphens, and cannot use the reserved name `user`.",
						"Retry with a name like `plan`, `frontend-1`, or `worker_alpha`.",
					)
				}
				return fmt.Errorf("cli.newRegisterCmd: register: %w", err)
			}

			heartbeat := agent.NewHeartbeat(app.db, agt.ID)
			heartbeat.Start(ctx)

			listener := transport.NewListener()
			server := transport.NewServer(agt.SocketPath, listener.Handle)
			poller := transport.NewPoller(app.db, agt.ID, listener.Handle)

			server.Start(ctx)
			poller.Start(ctx)

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(map[string]any{
					"id":           agt.ID,
					"name":         agt.Name,
					"type":         agt.Type,
					"pid":          agt.PID,
					"socket_path":  agt.SocketPath,
					"capabilities": caps,
					"workdir":      agt.WorkDir,
					"project":      agt.Project,
					"status":       "registered",
				}); err != nil {
					return fmt.Errorf("cli.newRegisterCmd: encode: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"registered agent %s (%s) id=%s pid=%d socket=%s project=%s\n",
					agt.Name,
					agt.Type,
					agt.ID,
					agt.PID,
					agt.SocketPath,
					agt.Project,
				); err != nil {
					return fmt.Errorf("cli.newRegisterCmd: write output: %w", err)
				}
			}

			<-ctx.Done()

			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := registry.Deregister(cleanupCtx, agt.ID, agt.Project); err != nil {
				return fmt.Errorf("cli.newRegisterCmd: deregister: %w", err)
			}

			if !jsonOutput {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "deregistered agent %s (%s)\n", agt.Name, agt.ID); err != nil {
					return fmt.Errorf("cli.newRegisterCmd: write shutdown output: %w", err)
				}
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Agent name")
	f.StringVar(&agentType, "type", "", "Agent type")
	f.StringVar(&capabilitiesRaw, "cap", "", "Comma-separated capabilities")
	f.StringVar(&workdir, "workdir", "", "Agent working directory (default: current working directory)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func parseCapabilities(raw string) []string {
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		capability := strings.TrimSpace(p)
		if capability == "" {
			continue
		}
		out = append(out, capability)
	}

	return out
}
