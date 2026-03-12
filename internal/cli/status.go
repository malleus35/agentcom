package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show system status summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			totalAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total agents: %w", err)
			}
			aliveAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents WHERE status = 'alive'`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: alive agents: %w", err)
			}
			deadAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents WHERE status = 'dead'`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: dead agents: %w", err)
			}
			totalMessages, err := scalarInt(cmd, `SELECT COUNT(*) FROM messages`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total messages: %w", err)
			}
			unreadMessages, err := scalarInt(cmd, `SELECT COUNT(*) FROM messages WHERE read_at IS NULL`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: unread messages: %w", err)
			}
			totalTasks, err := scalarInt(cmd, `SELECT COUNT(*) FROM tasks`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total tasks: %w", err)
			}

			tasksByStatus := make(map[string]int)
			rows, err := app.db.QueryContext(cmd.Context(), `SELECT status, COUNT(*) FROM tasks GROUP BY status ORDER BY status`)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: tasks by status query: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var status string
				var count int
				if err := rows.Scan(&status, &count); err != nil {
					return fmt.Errorf("cli.newStatusCmd: tasks by status scan: %w", err)
				}
				tasksByStatus[status] = count
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("cli.newStatusCmd: tasks by status rows: %w", err)
			}

			payload := map[string]any{
				"total_agents":    totalAgents,
				"alive_agents":    aliveAgents,
				"dead_agents":     deadAgents,
				"total_messages":  totalMessages,
				"unread_messages": unreadMessages,
				"total_tasks":     totalTasks,
				"tasks_by_status": tasksByStatus,
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(payload)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "METRIC\tVALUE"); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write header: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "total_agents\t%d\n", totalAgents); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "alive_agents\t%d\n", aliveAgents); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "dead_agents\t%d\n", deadAgents); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "total_messages\t%d\n", totalMessages); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "unread_messages\t%d\n", unreadMessages); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "total_tasks\t%d\n", totalTasks); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			for status, count := range tasksByStatus {
				if _, err := fmt.Fprintf(tw, "tasks_%s\t%d\n", status, count); err != nil {
					return fmt.Errorf("cli.newStatusCmd: write status metric: %w", err)
				}
			}

			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newStatusCmd: flush: %w", err)
			}

			return nil
		},
	}
}

func scalarInt(cmd *cobra.Command, query string) (int, error) {
	var v sql.NullInt64
	if err := app.db.QueryRowContext(cmd.Context(), query).Scan(&v); err != nil {
		return 0, fmt.Errorf("cli.scalarInt: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}
