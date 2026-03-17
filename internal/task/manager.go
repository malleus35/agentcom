package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malleus35/agentcom/internal/db"
	gonanoid "github.com/matoous/go-nanoid/v2"
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
	reviewer string,
	assignedTo string,
	createdBy string,
	blockedBy []string,
	policy *ReviewPolicy,
) (*db.Task, error) {
	id, err := gonanoid.New()
	if err != nil {
		return nil, fmt.Errorf("task.Manager.Create: generate id: %w", err)
	}

	taskPriority := NormalizePriority(priority)
	if err := ValidatePriority(taskPriority); err != nil {
		return nil, fmt.Errorf("task.Manager.Create: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("task.Manager.Create: %w", err)
	}

	taskReviewer := strings.TrimSpace(reviewer)
	if taskReviewer == "" && policy != nil {
		taskReviewer = policy.ResolveReviewer(taskPriority)
	}

	blockedByJSON, err := json.Marshal(blockedBy)
	if err != nil {
		return nil, fmt.Errorf("task.Manager.Create: marshal blocked_by: %w", err)
	}

	t := &db.Task{
		ID:          "tsk_" + id,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		Priority:    taskPriority,
		Reviewer:    taskReviewer,
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

	if err := ValidateTransition(t.Status, StatusAssigned); err != nil {
		return fmt.Errorf("task.Manager.Delegate: validate transition: %w", err)
	}

	t.AssignedTo = targetAgentID
	t.Status = StatusAssigned
	if err := m.db.UpdateTask(ctx, t); err != nil {
		return fmt.Errorf("task.Manager.Delegate: update task assignee: %w", err)
	}

	return nil
}

// UpdateStatus updates task status and result with transition validation.
func (m *Manager) UpdateStatus(ctx context.Context, taskID string, newStatus string, result string) error {
	t, err := m.db.FindTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: find task: %w", err)
	}

	if strings.TrimSpace(t.Reviewer) != "" && t.Status == StatusInProgress && newStatus == StatusCompleted {
		if err := m.db.UpdateTaskStatus(ctx, taskID, StatusBlocked, reviewRequiredResult(t.Reviewer, result)); err != nil {
			return fmt.Errorf("task.Manager.UpdateStatus: block for review: %w", err)
		}
		return nil
	}

	if err := ValidateTransitionWithReviewer(t.Status, newStatus, t.Reviewer); err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: validate transition: %w", err)
	}

	if err := m.db.UpdateTaskStatus(ctx, taskID, newStatus, result); err != nil {
		return fmt.Errorf("task.Manager.UpdateStatus: update task status: %w", err)
	}

	return nil
}

func (m *Manager) ApproveTask(ctx context.Context, taskID string, result string) error {
	return m.completeReviewedTask(ctx, taskID, StatusCompleted, result)
}

func (m *Manager) RejectTask(ctx context.Context, taskID string, result string) error {
	return m.completeReviewedTask(ctx, taskID, StatusFailed, result)
}

func (m *Manager) completeReviewedTask(ctx context.Context, taskID string, finalStatus string, result string) error {
	t, err := m.db.FindTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task.Manager.completeReviewedTask: find task: %w", err)
	}
	if strings.TrimSpace(t.Reviewer) == "" {
		return fmt.Errorf("task.Manager.completeReviewedTask: reviewer is not configured")
	}
	if t.Status != StatusBlocked {
		return fmt.Errorf("task.Manager.completeReviewedTask: %w", ErrInvalidTransition)
	}
	if finalStatus == StatusCompleted {
		if err := ValidateTransition(t.Status, StatusCompleted); err != nil {
			return fmt.Errorf("task.Manager.completeReviewedTask: validate transition: %w", err)
		}
	}
	if err := m.db.UpdateTaskStatus(ctx, taskID, finalStatus, coalesceResult(result, t.Result)); err != nil {
		return fmt.Errorf("task.Manager.completeReviewedTask: update task status: %w", err)
	}
	return nil
}

func reviewRequiredResult(reviewer string, result string) string {
	message := fmt.Sprintf("review required by %s", reviewer)
	if strings.TrimSpace(result) == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, result)
}

func coalesceResult(result string, fallback string) string {
	if strings.TrimSpace(result) != "" {
		return result
	}
	return fallback
}
