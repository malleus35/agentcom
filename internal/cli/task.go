package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/peanut-cc/agentcom/internal/agent"
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Task management commands",
	}

	cmd.AddCommand(
		newTaskCreateCmd(),
		newTaskListCmd(),
		newTaskUpdateCmd(),
		newTaskDelegateCmd(),
	)

	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	var desc string
	var assign string
	var priority string
	var blockedBy string
	var creator string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			registry := agent.NewRegistry(app.db, app.cfg)

			assignedID := ""
			if assign != "" {
				resolved, err := resolveAgentID(cmd, registry, assign)
				if err != nil {
					return fmt.Errorf("cli.newTaskCreateCmd: resolve assignee: %w", err)
				}
				assignedID = resolved
			}

			creatorID := ""
			if creator != "" {
				resolved, err := resolveAgentID(cmd, registry, creator)
				if err != nil {
					return fmt.Errorf("cli.newTaskCreateCmd: resolve creator: %w", err)
				}
				creatorID = resolved
			}

			blocked := splitCSV(blockedBy)
			blockedJSON, err := json.Marshal(blocked)
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: marshal blocked_by: %w", err)
			}

			rawID, err := gonanoid.New()
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: generate id: %w", err)
			}
			taskID := "tsk_" + rawID

			if _, err := app.db.ExecContext(cmd.Context(), `
				INSERT INTO tasks (id, title, description, status, priority, assigned_to, created_by, blocked_by)
				VALUES (?, ?, ?, 'pending', ?, ?, ?, ?)
			`, taskID, title, nullIfEmpty(desc), priority, nullIfEmpty(assignedID), nullIfEmpty(creatorID), string(blockedJSON)); err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: insert task: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"id":          taskID,
					"title":       title,
					"description": desc,
					"status":      "pending",
					"priority":    priority,
					"assigned_to": assignedID,
					"created_by":  creatorID,
					"blocked_by":  blocked,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created task %s\n", taskID)
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: write output: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&desc, "desc", "", "Task description")
	f.StringVar(&assign, "assign", "", "Assignee agent name or ID")
	f.StringVar(&priority, "priority", "medium", "Task priority: low|medium|high|critical")
	f.StringVar(&blockedBy, "blocked-by", "", "Comma-separated blocking task IDs")
	f.StringVar(&creator, "creator", "", "Creator agent name or ID")

	return cmd
}

func newTaskListCmd() *cobra.Command {
	var status string
	var assignee string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := `
				SELECT id, title, status, priority, assigned_to, created_at, updated_at
				FROM tasks
				WHERE ( ? = '' OR status = ? )
				  AND ( ? = '' OR assigned_to = ? )
				ORDER BY created_at DESC
			`

			assigneeID := assignee
			if assignee != "" {
				registry := agent.NewRegistry(app.db, app.cfg)
				if resolved, err := resolveAgentID(cmd, registry, assignee); err == nil {
					assigneeID = resolved
				}
			}

			rows, err := app.db.QueryContext(cmd.Context(), query, status, status, assigneeID, assigneeID)
			if err != nil {
				return fmt.Errorf("cli.newTaskListCmd: query tasks: %w", err)
			}
			defer rows.Close()

			type taskRow struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Status     string `json:"status"`
				Priority   string `json:"priority"`
				AssignedTo string `json:"assigned_to"`
				CreatedAt  string `json:"created_at"`
				UpdatedAt  string `json:"updated_at"`
			}

			tasks := make([]taskRow, 0)
			for rows.Next() {
				var t taskRow
				var assigned sql.NullString
				if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &assigned, &t.CreatedAt, &t.UpdatedAt); err != nil {
					return fmt.Errorf("cli.newTaskListCmd: scan: %w", err)
				}
				if assigned.Valid {
					t.AssignedTo = assigned.String
				}
				tasks = append(tasks, t)
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("cli.newTaskListCmd: rows: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(tasks)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "ID\tTITLE\tSTATUS\tPRIORITY\tASSIGNED_TO\tUPDATED_AT"); err != nil {
				return fmt.Errorf("cli.newTaskListCmd: write header: %w", err)
			}
			for _, t := range tasks {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Title, t.Status, t.Priority, t.AssignedTo, t.UpdatedAt); err != nil {
					return fmt.Errorf("cli.newTaskListCmd: write row: %w", err)
				}
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newTaskListCmd: flush: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&status, "status", "", "Filter by task status")
	f.StringVar(&assignee, "assignee", "", "Filter by assignee agent name or ID")

	return cmd
}

func newTaskUpdateCmd() *cobra.Command {
	var status string
	var result string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update task status/result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			if status == "" {
				return fmt.Errorf("cli.newTaskUpdateCmd: --status is required")
			}

			res, err := app.db.ExecContext(cmd.Context(), `
				UPDATE tasks
				SET status = ?, result = NULLIF(?, ''), updated_at = datetime('now')
				WHERE id = ?
			`, status, result, taskID)
			if err != nil {
				return fmt.Errorf("cli.newTaskUpdateCmd: update task: %w", err)
			}

			rows, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("cli.newTaskUpdateCmd: rows affected: %w", err)
			}
			if rows == 0 {
				return fmt.Errorf("cli.newTaskUpdateCmd: task not found: %s", taskID)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"id":     taskID,
					"status": status,
					"result": result,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated task %s status=%s\n", taskID, status)
			if err != nil {
				return fmt.Errorf("cli.newTaskUpdateCmd: write output: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&status, "status", "", "New task status")
	f.StringVar(&result, "result", "", "Task result text")

	return cmd
}

func newTaskDelegateCmd() *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:   "delegate <id>",
		Short: "Delegate task to another agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			registry := agent.NewRegistry(app.db, app.cfg)
			assigneeID, err := resolveAgentID(cmd, registry, to)
			if err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: resolve target: %w", err)
			}

			res, err := app.db.ExecContext(cmd.Context(), `
				UPDATE tasks
				SET assigned_to = ?, updated_at = datetime('now')
				WHERE id = ?
			`, assigneeID, taskID)
			if err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: delegate task: %w", err)
			}

			rows, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: rows affected: %w", err)
			}
			if rows == 0 {
				return fmt.Errorf("cli.newTaskDelegateCmd: task not found: %s", taskID)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"id":           taskID,
					"delegated_to": assigneeID,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "delegated task %s to %s\n", taskID, assigneeID)
			if err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: write output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "Target agent name or ID")
	_ = cmd.MarkFlagRequired("to")

	return cmd
}

func resolveAgentID(cmd *cobra.Command, registry *agent.Registry, nameOrID string) (string, error) {
	agt, err := registry.FindByName(cmd.Context(), nameOrID)
	if err == nil {
		return agt.ID, nil
	}

	agt, err = registry.FindByID(cmd.Context(), nameOrID)
	if err != nil {
		return "", fmt.Errorf("cli.resolveAgentID: %w", err)
	}

	return agt.ID, nil
}

func splitCSV(raw string) []string {
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
