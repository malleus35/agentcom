package task

import (
	"context"
	"fmt"

	"github.com/peanut-cc/agentcom/internal/db"
)

// Query provides task query operations.
type Query struct {
	db *db.DB
}

// NewQuery creates a task query service.
func NewQuery(database *db.DB) *Query {
	return &Query{db: database}
}

// ListAll returns all tasks.
func (q *Query) ListAll(ctx context.Context) ([]*db.Task, error) {
	tasks, err := q.db.ListAllTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("task.Query.ListAll: %w", err)
	}
	return tasks, nil
}

// ListByStatus returns tasks filtered by status.
func (q *Query) ListByStatus(ctx context.Context, status string) ([]*db.Task, error) {
	tasks, err := q.db.ListTasksByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("task.Query.ListByStatus: %w", err)
	}
	return tasks, nil
}

// ListByAssignee returns tasks assigned to a given agent.
func (q *Query) ListByAssignee(ctx context.Context, agentID string) ([]*db.Task, error) {
	tasks, err := q.db.ListTasksByAssignee(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("task.Query.ListByAssignee: %w", err)
	}
	return tasks, nil
}

// FindByID returns a task by its unique ID.
func (q *Query) FindByID(ctx context.Context, id string) (*db.Task, error) {
	t, err := q.db.FindTaskByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("task.Query.FindByID: %w", err)
	}
	return t, nil
}
