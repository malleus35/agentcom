package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/peanut-cc/agentcom/internal/db"
)

// Manager manages task creation, delegation, and status changes.
type Manager struct {
	db *db.DB
}

// NewManager creates a task manager.
func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

// Create creates a new task with default status and priority handling.
func (m *Manager) Create(
	ctx context.Context,
	title string,
	description string,
	priority string,
	assignedTo string,
	createdBy string,
	blockedBy []string,
) (*db.Task, error) {
	id, err := gonanoid.New()
	if err != nil {
		return nil, fmt.Errorf("task.Manager.Create: generate id: %w", err)
	}

	taskPriority := strings.TrimSpace(priority)
	if taskPriority == "" {
		taskPriority = "medium"
	}

	blockedByJSON, err := json.Marshal(blockedBy)
	if err != nil {
		return nil, fmt.Errorf("task.Manager.Create: marshal blocked_by: %w", err)
	}

	t := &db.Task{
		ID:          "tsk_" + id,
		Title:       title,
		Description: description,
		Status:      "pending",
		Priority:    taskPriority,
		AssignedTo:  assignedTo,
		CreatedBy:   createdBy,
		BlockedBy:   string(blockedByJSON),
	}

	if err := m.db.InsertTask(ctx, t); err != nil {
		return nil, fmt.Errorf("task.Manager.Create: insert task: %w", err)
	}

	return t, nil
}

// Delegate delegates a task to a target agent and sets it assigned.
func (m *Manager) Delegate(ctx context.Context, taskID string, targetAgentID string) error {
	t, err := m.db.FindTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task.Manager.Delegate: find task: %w", err)
	}

	if err := ValidateTransition(t.Status, "assigned"); err != nil {
		return fmt.Errorf("task.Manager.Delegate: validate transition: %w", err)
	}

	t.AssignedTo = targetAgentID
	t.Status = "assigned"
	if err := m.db.UpdateTask(ctx, t); err != nil {
		return fmt.Errorf("task.Manager.Delegate: update task assignee: %w", err)
	}

	if err := m.db.UpdateTaskStatus(ctx, taskID, "assigned", t.Result); err != nil {
		return fmt.Errorf("task.Manager.Delegate: update task status: %w", err)
	}

	return nil
}

// UpdateStatus updates task status and result with transition validation.
func (m *Manager) UpdateStatus(ctx context.Context, taskID string, newStatus string, result string) error {
	t, err := m.db.FindTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: find task: %w", err)
	}

	if err := ValidateTransition(t.Status, newStatus); err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: validate transition: %w", err)
	}

	if err := m.db.UpdateTaskStatus(ctx, taskID, newStatus, result); err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: update task status: %w", err)
	}

	return nil
}
