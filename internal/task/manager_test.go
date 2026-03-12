package task

import (
	"context"
	"errors"
	"testing"

	"github.com/malleus35/agentcom/internal/db"
)

func setupTaskTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("db.OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("database.Close() error = %v", err)
		}
	})
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	return database
}

func insertTaskTestAgent(t *testing.T, database *db.DB, name string) *db.Agent {
	t.Helper()

	agent := &db.Agent{Name: name, Type: "worker", Status: "alive"}
	if err := database.InsertAgent(context.Background(), agent); err != nil {
		t.Fatalf("InsertAgent(%s) error = %v", name, err)
	}

	return agent
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{name: "pending to assigned", from: StatusPending, to: StatusAssigned},
		{name: "assigned to in_progress", from: StatusAssigned, to: StatusInProgress},
		{name: "in_progress to completed", from: StatusInProgress, to: StatusCompleted},
		{name: "completed to pending invalid", from: StatusCompleted, to: StatusPending, wantErr: ErrInvalidTransition},
		{name: "pending to failed invalid", from: StatusPending, to: StatusFailed, wantErr: ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateTransition() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: StatusCompleted, want: true},
		{status: StatusFailed, want: true},
		{status: StatusCancelled, want: true},
		{status: StatusPending, want: false},
		{status: StatusAssigned, want: false},
	}

	for _, tt := range tests {
		if got := IsTerminal(tt.status); got != tt.want {
			t.Fatalf("IsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestManagerLifecycle(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")
	assignee := insertTaskTestAgent(t, database, "assignee")

	manager := NewManager(database)
	query := NewQuery(database)

	created, err := manager.Create(ctx, "ship it", "complete tests", "", "", creator.ID, []string{"P1-09"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Priority != "medium" {
		t.Fatalf("Priority = %q, want medium", created.Priority)
	}
	if created.Status != StatusPending {
		t.Fatalf("Status = %q, want %q", created.Status, StatusPending)
	}

	if err := manager.Delegate(ctx, created.ID, assignee.ID); err != nil {
		t.Fatalf("Delegate() error = %v", err)
	}

	afterDelegate, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(after delegate) error = %v", err)
	}
	if afterDelegate.Status != StatusAssigned {
		t.Fatalf("delegated status = %q, want %q", afterDelegate.Status, StatusAssigned)
	}
	if afterDelegate.AssignedTo != assignee.ID {
		t.Fatalf("AssignedTo = %q, want %q", afterDelegate.AssignedTo, assignee.ID)
	}

	if err := manager.UpdateStatus(ctx, created.ID, StatusInProgress, "started"); err != nil {
		t.Fatalf("UpdateStatus(in_progress) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusCompleted, "done"); err != nil {
		t.Fatalf("UpdateStatus(completed) error = %v", err)
	}

	completed, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(completed) error = %v", err)
	}
	if completed.Status != StatusCompleted || completed.Result != "done" {
		t.Fatalf("completed task = %+v", completed)
	}

	byAssignee, err := query.ListByAssignee(ctx, assignee.ID)
	if err != nil {
		t.Fatalf("ListByAssignee() error = %v", err)
	}
	if len(byAssignee) != 1 {
		t.Fatalf("len(ListByAssignee()) = %d, want 1", len(byAssignee))
	}

	byStatus, err := query.ListByStatus(ctx, StatusCompleted)
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(byStatus) != 1 {
		t.Fatalf("len(ListByStatus()) = %d, want 1", len(byStatus))
	}
}

func TestManagerRejectsInvalidTransitions(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	created, err := manager.Create(ctx, "ship it", "complete tests", "high", "", creator.ID, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := manager.UpdateStatus(ctx, created.ID, StatusCompleted, "done"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("UpdateStatus(invalid) error = %v, want %v", err, ErrInvalidTransition)
	}

	if err := manager.Delegate(ctx, created.ID, creator.ID); err != nil {
		t.Fatalf("Delegate() error = %v", err)
	}
	if err := manager.Delegate(ctx, created.ID, creator.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Delegate(invalid) error = %v, want %v", err, ErrInvalidTransition)
	}
}
