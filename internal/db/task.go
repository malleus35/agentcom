package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

var ErrTaskNotFound = errors.New("task not found")

// Task represents a persisted task row.
type Task struct {
	ID          string
	Title       string
	Description string
	Status      string
	Priority    string
	AssignedTo  string
	CreatedBy   string
	BlockedBy   string
	Result      string
	CreatedAt   string
	UpdatedAt   string
}

// InsertTask inserts a new task row and generates a task ID.
func (d *DB) InsertTask(ctx context.Context, task *Task) error {
	id, err := gonanoid.New()
	if err != nil {
		return fmt.Errorf("db.InsertTask: generate id: %w", err)
	}
	task.ID = "tsk_" + id

	stmt, err := d.PrepareContext(ctx, `
		INSERT INTO tasks (
			id, title, description, status, priority, assigned_to, created_by, blocked_by, result
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("db.InsertTask: prepare: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx,
		task.ID,
		task.Title,
		nullableString(task.Description),
		task.Status,
		task.Priority,
		nullableString(task.AssignedTo),
		nullableString(task.CreatedBy),
		task.BlockedBy,
		nullableString(task.Result),
	); err != nil {
		return fmt.Errorf("db.InsertTask: exec: %w", err)
	}

	return nil
}

// UpdateTask updates an existing task and refreshes updated_at.
func (d *DB) UpdateTask(ctx context.Context, task *Task) error {
	stmt, err := d.PrepareContext(ctx, `
		UPDATE tasks
		SET title = ?, description = ?, status = ?, priority = ?, assigned_to = ?, created_by = ?, blocked_by = ?, result = ?, updated_at = datetime('now')
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.UpdateTask: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx,
		task.Title,
		nullableString(task.Description),
		task.Status,
		task.Priority,
		nullableString(task.AssignedTo),
		nullableString(task.CreatedBy),
		task.BlockedBy,
		nullableString(task.Result),
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("db.UpdateTask: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.UpdateTask: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// FindTaskByID finds a task by ID.
func (d *DB) FindTaskByID(ctx context.Context, id string) (*Task, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, title, description, status, priority, assigned_to, created_by, blocked_by, result, created_at, updated_at
		FROM tasks
		WHERE id = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("db.FindTaskByID: prepare: %w", err)
	}
	defer stmt.Close()

	task, err := scanTask(stmt.QueryRowContext(ctx, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("db.FindTaskByID: %w", err)
	}

	return task, nil
}

// ListAllTasks lists all tasks by created time.
func (d *DB) ListAllTasks(ctx context.Context) ([]*Task, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, title, description, status, priority, assigned_to, created_by, blocked_by, result, created_at, updated_at
		FROM tasks
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListAllTasks: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListAllTasks: query: %w", err)
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListAllTasks: scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListAllTasks: rows: %w", err)
	}

	return tasks, nil
}

// ListTasksByStatus lists tasks filtered by status.
func (d *DB) ListTasksByStatus(ctx context.Context, status string) ([]*Task, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, title, description, status, priority, assigned_to, created_by, blocked_by, result, created_at, updated_at
		FROM tasks
		WHERE status = ?
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListTasksByStatus: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("db.ListTasksByStatus: query: %w", err)
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListTasksByStatus: scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListTasksByStatus: rows: %w", err)
	}

	return tasks, nil
}

// ListTasksByAssignee lists tasks assigned to a specific agent.
func (d *DB) ListTasksByAssignee(ctx context.Context, agentID string) ([]*Task, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, title, description, status, priority, assigned_to, created_by, blocked_by, result, created_at, updated_at
		FROM tasks
		WHERE assigned_to = ?
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListTasksByAssignee: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("db.ListTasksByAssignee: query: %w", err)
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListTasksByAssignee: scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListTasksByAssignee: rows: %w", err)
	}

	return tasks, nil
}

// UpdateTaskStatus updates status/result and refreshes updated_at.
func (d *DB) UpdateTaskStatus(ctx context.Context, id, status, result string) error {
	stmt, err := d.PrepareContext(ctx, `
		UPDATE tasks
		SET status = ?, result = ?, updated_at = datetime('now')
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.UpdateTaskStatus: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, status, nullableString(result), id)
	if err != nil {
		return fmt.Errorf("db.UpdateTaskStatus: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.UpdateTaskStatus: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func scanTask(scanner rowScanner) (*Task, error) {
	task := &Task{}
	var description sql.NullString
	var assignedTo sql.NullString
	var createdBy sql.NullString
	var result sql.NullString

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&description,
		&task.Status,
		&task.Priority,
		&assignedTo,
		&createdBy,
		&task.BlockedBy,
		&result,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if description.Valid {
		task.Description = description.String
	}
	if assignedTo.Valid {
		task.AssignedTo = assignedTo.String
	}
	if createdBy.Valid {
		task.CreatedBy = createdBy.String
	}
	if result.Valid {
		task.Result = result.String
	}

	return task, nil
}
