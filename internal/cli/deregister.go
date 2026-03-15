package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/spf13/cobra"
)

func newDeregisterCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "deregister <name-or-id>",
		Short: "Deregister an agent by name or id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]
			registry := agent.NewRegistry(app.db, app.cfg)

			agt, err := registry.FindByName(cmd.Context(), nameOrID, currentProjectFilter())
			if err != nil {
				agt, err = registry.FindByID(cmd.Context(), nameOrID)
				if err != nil {
					return fmt.Errorf("cli.newDeregisterCmd: resolve target: %w", err)
				}
			}

			if !force {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "agent: %s (%s) type=%s status=%s\n", agt.Name, agt.ID, agt.Type, agt.Status); err != nil {
					return fmt.Errorf("cli.newDeregisterCmd: write output: %w", err)
				}
				if _, err := fmt.Fprint(cmd.OutOrStdout(), "are you sure? [y/N] "); err != nil {
					return fmt.Errorf("cli.newDeregisterCmd: write prompt: %w", err)
				}

				reader := bufio.NewReader(cmd.InOrStdin())
				answer, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("cli.newDeregisterCmd: read confirmation: %w", err)
				}

				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "y" && answer != "yes" {
					if !jsonOutput {
						_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
					}
					return nil
				}
			}

			if err := registry.Deregister(cmd.Context(), agt.ID, agt.Project); err != nil {
				return fmt.Errorf("cli.newDeregisterCmd: deregister: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"id":     agt.ID,
					"name":   agt.Name,
					"status": "deregistered",
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "deregistered %s (%s)\n", agt.Name, agt.ID)
			if err != nil {
				return fmt.Errorf("cli.newDeregisterCmd: write success output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}
