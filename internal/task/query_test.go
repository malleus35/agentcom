package task

import (
	"context"
	"errors"
	"testing"

	"github.com/malleus35/agentcom/internal/db"
)

func TestQueryOperations(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()

	assignee := insertTaskTestAgent(t, database, "assignee")
	other := insertTaskTestAgent(t, database, "other")
	query := NewQuery(database)
	if query.db != database {
		t.Fatal("NewQuery() did not retain database reference")
	}

	tasks := []*db.Task{
		{Title: "pending task", Status: StatusPending, AssignedTo: assignee.ID, BlockedBy: "[]"},
		{Title: "in progress task", Status: StatusInProgress, AssignedTo: assignee.ID, BlockedBy: "[]"},
		{Title: "other task", Status: StatusCompleted, AssignedTo: other.ID, BlockedBy: "[]"},
	}
	for _, taskRecord := range tasks {
		if err := database.InsertTask(ctx, taskRecord); err != nil {
			t.Fatalf("InsertTask(%s) error = %v", taskRecord.Title, err)
		}
	}

	all, err := query.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len(ListAll()) = %d, want 3", len(all))
	}

	inProgress, err := query.ListByStatus(ctx, StatusInProgress)
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(inProgress) != 1 || inProgress[0].Title != "in progress task" {
		t.Fatalf("ListByStatus() = %+v", inProgress)
	}

	assigned, err := query.ListByAssignee(ctx, assignee.ID)
	if err != nil {
		t.Fatalf("ListByAssignee() error = %v", err)
	}
	if len(assigned) != 2 {
		t.Fatalf("len(ListByAssignee()) = %d, want 2", len(assigned))
	}

	found, err := query.FindByID(ctx, tasks[1].ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.Title != tasks[1].Title {
		t.Fatalf("FindByID().Title = %q, want %q", found.Title, tasks[1].Title)
	}

	_, err = query.FindByID(ctx, "tsk_missing")
	if !errors.Is(err, db.ErrTaskNotFound) {
		t.Fatalf("FindByID(missing) error = %v, want %v", err, db.ErrTaskNotFound)
	}
}
