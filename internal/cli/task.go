package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/task"
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
		newTaskApproveCmd(),
		newTaskRejectCmd(),
	)

	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	var desc string
	var assign string
	var priority string
	var blockedBy string
	var creator string
	var reviewer string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			normalizedPriority := task.NormalizePriority(priority)
			if err := task.ValidatePriority(normalizedPriority); err != nil {
				return newUserError(
					"Task priority is invalid",
					"Task priority must be one of low, medium, high, or critical.",
					"Retry with `--priority low`, `--priority medium`, `--priority high`, or `--priority critical`.",
				)
			}

			registry := agent.NewRegistry(app.db, app.cfg)

			assignedID := ""
			if assign != "" {
				resolved, err := resolveAgentID(cmd, registry, assign, currentProjectFilter())
				if err != nil {
					return presentAgentResolutionError(assign, err)
				}
				assignedID = resolved
			}

			creatorID := ""
			if creator != "" {
				resolved, err := resolveAgentID(cmd, registry, creator, currentProjectFilter())
				if err != nil {
					return presentAgentResolutionError(creator, err)
				}
				creatorID = resolved
			}

			blocked := splitCSV(blockedBy)
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: getwd: %w", err)
			}
			policy, err := loadActiveTaskReviewPolicy(cwd)
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: load review policy: %w", err)
			}

			manager := task.NewManager(app.db)
			created, err := manager.Create(cmd.Context(), title, desc, normalizedPriority, reviewer, assignedID, creatorID, blocked, policy)
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: create task: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"id":          created.ID,
					"title":       title,
					"description": desc,
					"status":      created.Status,
					"priority":    created.Priority,
					"reviewer":    created.Reviewer,
					"assigned_to": assignedID,
					"created_by":  creatorID,
					"blocked_by":  blocked,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created task %s\n", created.ID)
			if err != nil {
				return fmt.Errorf("cli.newTaskCreateCmd: write output: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&desc, "desc", "", "Task description")
	f.StringVar(&assign, "assign", "", "Assignee agent name or ID")
	f.StringVar(&priority, "priority", task.PriorityMedium, "Task priority: low|medium|high|critical (default: medium)")
	f.StringVar(&blockedBy, "blocked-by", "", "Comma-separated blocking task IDs")
	f.StringVar(&creator, "creator", "", "Creator agent name or ID")
	f.StringVar(&reviewer, "reviewer", "", "Reviewer agent name, ID, or user")

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
				if resolved, err := resolveAgentID(cmd, registry, assignee, currentProjectFilter()); err == nil {
					assigneeID = resolved
				} else if !errors.Is(err, agent.ErrAgentNotFound) {
					return err
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

			manager := task.NewManager(app.db)
			if err := manager.UpdateStatus(cmd.Context(), taskID, status, result); err != nil {
				return fmt.Errorf("cli.newTaskUpdateCmd: update task: %w", err)
			}

			updated, err := app.db.FindTaskByID(cmd.Context(), taskID)
			if err != nil {
				return fmt.Errorf("cli.newTaskUpdateCmd: find task: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"id":       updated.ID,
					"status":   updated.Status,
					"result":   updated.Result,
					"reviewer": updated.Reviewer,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated task %s status=%s\n", taskID, updated.Status)
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
			assigneeID, err := resolveAgentID(cmd, registry, to, currentProjectFilter())
			if err != nil {
				return presentAgentResolutionError(to, err)
			}

			manager := task.NewManager(app.db)
			if err := manager.Delegate(cmd.Context(), taskID, assigneeID); err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: delegate task: %w", err)
			}

			updated, err := app.db.FindTaskByID(cmd.Context(), taskID)
			if err != nil {
				return fmt.Errorf("cli.newTaskDelegateCmd: find task: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"id":           updated.ID,
					"delegated_to": updated.AssignedTo,
					"status":       updated.Status,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "delegated task %s to %s\n", taskID, updated.AssignedTo)
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

func newTaskApproveCmd() *cobra.Command {
	var result string

	cmd := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve a blocked review task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := task.NewManager(app.db)
			if err := manager.ApproveTask(cmd.Context(), args[0], result); err != nil {
				return fmt.Errorf("cli.newTaskApproveCmd: approve task: %w", err)
			}
			updated, err := app.db.FindTaskByID(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("cli.newTaskApproveCmd: find task: %w", err)
			}
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"id": updated.ID, "status": updated.Status, "result": updated.Result})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "approved task %s\n", updated.ID)
			if err != nil {
				return fmt.Errorf("cli.newTaskApproveCmd: write output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&result, "result", "", "Approval result text")
	return cmd
}

func newTaskRejectCmd() *cobra.Command {
	var result string

	cmd := &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject a blocked review task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := task.NewManager(app.db)
			if err := manager.RejectTask(cmd.Context(), args[0], result); err != nil {
				return fmt.Errorf("cli.newTaskRejectCmd: reject task: %w", err)
			}
			updated, err := app.db.FindTaskByID(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("cli.newTaskRejectCmd: find task: %w", err)
			}
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"id": updated.ID, "status": updated.Status, "result": updated.Result})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "rejected task %s\n", updated.ID)
			if err != nil {
				return fmt.Errorf("cli.newTaskRejectCmd: write output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&result, "result", "", "Rejection result text")
	return cmd
}

func resolveAgentID(cmd *cobra.Command, registry *agent.Registry, nameOrID string, project string) (string, error) {
	agt, err := registry.FindByName(cmd.Context(), nameOrID, project)
	if err == nil {
		return agt.ID, nil
	}

	agt, err = registry.FindByID(cmd.Context(), nameOrID)
	if err != nil {
		return "", fmt.Errorf("cli.resolveAgentID: %w", err)
	}

	return agt.ID, nil
}

func presentAgentResolutionError(nameOrID string, err error) error {
	if errors.Is(err, agent.ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
		return newUserError(
			fmt.Sprintf("Agent %q could not be resolved", nameOrID),
			"The provided agent name or ID does not exist in the current project context.",
			"Check `agentcom list` or `agentcom status` for available agents, then retry with a valid name or ID.",
		)
	}
	return fmt.Errorf("cli.presentAgentResolutionError: %w", err)
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

func loadActiveTaskReviewPolicy(projectDir string) (*task.ReviewPolicy, error) {
	projectCfg, _, err := config.LoadProjectConfig(projectDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	if strings.TrimSpace(projectCfg.Template.Active) == "" {
		return nil, nil
	}

	manifestPath := filepath.Join(projectDir, ".agentcom", "templates", projectCfg.Template.Active, "template.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read template manifest: %w", err)
	}

	var manifest struct {
		ReviewPolicy *task.ReviewPolicy `json:"review_policy,omitempty"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal template manifest: %w", err)
	}
	if manifest.ReviewPolicy != nil {
		if err := manifest.ReviewPolicy.Validate(); err != nil {
			return nil, fmt.Errorf("validate review policy: %w", err)
		}
	}
	return manifest.ReviewPolicy, nil
}
