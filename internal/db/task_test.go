package db

import (
	"context"
	"errors"
	"testing"
)

func TestTaskCRUD(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	creator := &Agent{Name: "creator", Type: "planner", Status: "alive"}
	if err := database.InsertAgent(ctx, creator); err != nil {
		t.Fatalf("InsertAgent(creator) error = %v", err)
	}
	assignee := &Agent{Name: "assignee", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, assignee); err != nil {
		t.Fatalf("InsertAgent(assignee) error = %v", err)
	}

	task := &Task{
		ID:          "tsk_custom",
		Title:       "ship feature",
		Description: "implement wave 8",
		Status:      "pending",
		Priority:    "high",
		Reviewer:    "user",
		AssignedTo:  assignee.ID,
		CreatedBy:   creator.ID,
		BlockedBy:   `["P1-09"]`,
	}
	if err := database.InsertTask(ctx, task); err != nil {
		t.Fatalf("InsertTask() error = %v", err)
	}

	got, err := database.FindTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("FindTaskByID() error = %v", err)
	}
	if got.Title != task.Title {
		t.Fatalf("Title = %q, want %q", got.Title, task.Title)
	}
	if got.Reviewer != task.Reviewer {
		t.Fatalf("Reviewer = %q, want %q", got.Reviewer, task.Reviewer)
	}
	if task.ID != "tsk_custom" {
		t.Fatalf("Task ID = %q, want preserved custom id", task.ID)
	}

	task.Status = "assigned"
	task.Result = "delegated"
	task.Reviewer = "review"
	if err := database.UpdateTask(ctx, task); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	if err := database.UpdateTaskStatus(ctx, task.ID, "in_progress", "started"); err != nil {
		t.Fatalf("UpdateTaskStatus() error = %v", err)
	}

	got, err = database.FindTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("FindTaskByID(after update) error = %v", err)
	}
	if got.Status != "in_progress" || got.Result != "started" {
		t.Fatalf("task after status update = %+v", got)
	}
	if got.Reviewer != "review" {
		t.Fatalf("Reviewer(after update) = %q, want review", got.Reviewer)
	}

	all, err := database.ListAllTasks(ctx)
	if err != nil {
		t.Fatalf("ListAllTasks() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len(ListAllTasks()) = %d, want 1", len(all))
	}

	byStatus, err := database.ListTasksByStatus(ctx, "in_progress")
	if err != nil {
		t.Fatalf("ListTasksByStatus() error = %v", err)
	}
	if len(byStatus) != 1 {
		t.Fatalf("len(ListTasksByStatus()) = %d, want 1", len(byStatus))
	}

	byAssignee, err := database.ListTasksByAssignee(ctx, assignee.ID)
	if err != nil {
		t.Fatalf("ListTasksByAssignee() error = %v", err)
	}
	if len(byAssignee) != 1 {
		t.Fatalf("len(ListTasksByAssignee()) = %d, want 1", len(byAssignee))
	}

	if err := database.UpdateTaskStatus(ctx, "missing", "failed", "oops"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("UpdateTaskStatus(missing) error = %v, want %v", err, ErrTaskNotFound)
	}
}
