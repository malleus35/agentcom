package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/peanut-cc/agentcom/internal/agent"
	"github.com/peanut-cc/agentcom/internal/db"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var aliveOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := agent.NewRegistry(app.db, app.cfg)

			var (
				agents []*db.Agent
				err    error
			)

			if aliveOnly {
				agents, err = registry.ListAlive(cmd.Context())
			} else {
				agents, err = registry.ListAll(cmd.Context())
			}
			if err != nil {
				return fmt.Errorf("cli.newListCmd: list agents: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(agents); err != nil {
					return fmt.Errorf("cli.newListCmd: encode: %w", err)
				}
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tPID\tLAST_HEARTBEAT"); err != nil {
				return fmt.Errorf("cli.newListCmd: write header: %w", err)
			}

			for _, a := range agents {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%v\n", a.Name, a.Type, a.Status, a.PID, a.LastHeartbeat); err != nil {
					return fmt.Errorf("cli.newListCmd: write row: %w", err)
				}
			}

			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newListCmd: flush: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&aliveOnly, "alive", false, "Show only alive agents")

	return cmd
}
