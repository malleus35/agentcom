package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show system status summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			project := currentProjectFilter()
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: getwd: %w", err)
			}
			projectCfg, _, err := config.LoadProjectConfig(projectDir)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: load project config: %w", err)
			}
			templateName := projectCfg.Template.Active
			totalAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents WHERE (? = '' OR project = ?)`, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total agents: %w", err)
			}
			aliveAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents WHERE status = 'alive' AND (? = '' OR project = ?)`, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: alive agents: %w", err)
			}
			deadAgents, err := scalarInt(cmd, `SELECT COUNT(*) FROM agents WHERE status = 'dead' AND (? = '' OR project = ?)`, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: dead agents: %w", err)
			}
			totalMessages, err := scalarInt(cmd, `
				SELECT COUNT(*)
				FROM messages m
				LEFT JOIN agents sender ON sender.id = m.from_agent
				LEFT JOIN agents recipient ON recipient.id = m.to_agent
				WHERE (? = '' OR sender.project = ? OR recipient.project = ?)
			`, project, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total messages: %w", err)
			}
			unreadMessages, err := scalarInt(cmd, `
				SELECT COUNT(*)
				FROM messages m
				LEFT JOIN agents sender ON sender.id = m.from_agent
				LEFT JOIN agents recipient ON recipient.id = m.to_agent
				WHERE m.read_at IS NULL AND (? = '' OR sender.project = ? OR recipient.project = ?)
			`, project, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: unread messages: %w", err)
			}
			totalTasks, err := scalarInt(cmd, `
				SELECT COUNT(*)
				FROM tasks
				LEFT JOIN agents assigned ON assigned.id = tasks.assigned_to
				LEFT JOIN agents created ON created.id = tasks.created_by
				WHERE (? = '' OR assigned.project = ? OR created.project = ?)
			`, project, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: total tasks: %w", err)
			}

			tasksByStatus := make(map[string]int)
			rows, err := app.db.QueryContext(cmd.Context(), `
				SELECT tasks.status, COUNT(*)
				FROM tasks
				LEFT JOIN agents assigned ON assigned.id = tasks.assigned_to
				LEFT JOIN agents created ON created.id = tasks.created_by
				WHERE (? = '' OR assigned.project = ? OR created.project = ?)
				GROUP BY tasks.status
				ORDER BY tasks.status
			`, project, project, project)
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

			unreadByAgent := make(map[string]int)
			messageRows, err := app.db.QueryContext(cmd.Context(), `
				SELECT recipient.name, COUNT(*)
				FROM messages m
				JOIN agents recipient ON recipient.id = m.to_agent
				WHERE m.read_at IS NULL AND (? = '' OR recipient.project = ?)
				GROUP BY recipient.name
				ORDER BY recipient.name
			`, project, project)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: unread by agent query: %w", err)
			}
			defer messageRows.Close()
			for messageRows.Next() {
				var agentName string
				var count int
				if err := messageRows.Scan(&agentName, &count); err != nil {
					return fmt.Errorf("cli.newStatusCmd: unread by agent scan: %w", err)
				}
				unreadByAgent[agentName] = count
			}
			if err := messageRows.Err(); err != nil {
				return fmt.Errorf("cli.newStatusCmd: unread by agent rows: %w", err)
			}

			roleStatus := make(map[string]string)
			state, _, err := loadUpRuntimeState(projectDir)
			if err != nil {
				return fmt.Errorf("cli.newStatusCmd: load runtime state: %w", err)
			}
			for _, agent := range state.Agents {
				status := "stopped"
				if processAliveCheck(agent.PID) {
					registered, findErr := app.db.FindAgentByNameAndProject(cmd.Context(), agent.Name, project)
					if findErr == nil && registered.Status == "alive" {
						status = "alive"
					}
				}
				roleStatus[agent.Role] = status
			}

			payload := map[string]any{
				"project":         project,
				"template":        templateName,
				"total_agents":    totalAgents,
				"alive_agents":    aliveAgents,
				"dead_agents":     deadAgents,
				"total_messages":  totalMessages,
				"unread_messages": unreadMessages,
				"unread_by_agent": unreadByAgent,
				"total_tasks":     totalTasks,
				"role_status":     roleStatus,
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
			if _, err := fmt.Fprintf(tw, "project\t%s\n", project); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
			}
			if _, err := fmt.Fprintf(tw, "template\t%s\n", templateName); err != nil {
				return fmt.Errorf("cli.newStatusCmd: write metric: %w", err)
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
			for agentName, count := range unreadByAgent {
				if _, err := fmt.Fprintf(tw, "unread_%s\t%d\n", agentName, count); err != nil {
					return fmt.Errorf("cli.newStatusCmd: write unread metric: %w", err)
				}
			}
			for role, status := range roleStatus {
				if _, err := fmt.Fprintf(tw, "role_%s\t%s\n", role, status); err != nil {
					return fmt.Errorf("cli.newStatusCmd: write role metric: %w", err)
				}
			}

			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newStatusCmd: flush: %w", err)
			}

			return nil
		},
	}
}

func scalarInt(cmd *cobra.Command, query string, args ...any) (int, error) {
	var v sql.NullInt64
	if err := app.db.QueryRowContext(cmd.Context(), query, args...).Scan(&v); err != nil {
		return 0, fmt.Errorf("cli.scalarInt: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}
