package task

import (
	"context"
	"errors"
	"strings"
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

func TestManagerLifecycle(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")
	assignee := insertTaskTestAgent(t, database, "assignee")

	manager := NewManager(database)
	query := NewQuery(database)

	created, err := manager.Create(ctx, "ship it", "complete tests", "medium", "", "", creator.ID, []string{"P1-09"}, nil)
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
	created, err := manager.Create(ctx, "ship it", "complete tests", "high", "", "", creator.ID, nil, nil)
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

func TestManagerCreateRejectsInvalidPriority(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	_, err := manager.Create(ctx, "ship it", "complete tests", "urgent", "", "", creator.ID, nil, nil)
	if err == nil {
		t.Fatal("Create() error = nil, want invalid priority error")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Fatalf("Create() error = %v, want invalid priority context", err)
	}
}

func TestManagerCreateAppliesReviewerAndPolicy(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	policy := &ReviewPolicy{
		RequireReviewAbove: PriorityHigh,
		DefaultReviewer:    "user",
		Rules:              []ReviewPolicyRule{{Priority: PriorityCritical, Reviewer: "review"}},
	}

	created, err := manager.Create(ctx, "ship it", "complete tests", "HIGH", "", "", creator.ID, nil, policy)
	if err != nil {
		t.Fatalf("Create(policy) error = %v", err)
	}
	if created.Priority != PriorityHigh {
		t.Fatalf("Priority = %q, want %q", created.Priority, PriorityHigh)
	}
	if created.Reviewer != "user" {
		t.Fatalf("Reviewer = %q, want user", created.Reviewer)
	}

	explicit, err := manager.Create(ctx, "ship it", "complete tests", "critical", "alice", "", creator.ID, nil, policy)
	if err != nil {
		t.Fatalf("Create(explicit reviewer) error = %v", err)
	}
	if explicit.Reviewer != "alice" {
		t.Fatalf("Reviewer = %q, want alice", explicit.Reviewer)
	}
}

func TestManagerReviewLifecycle(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	query := NewQuery(database)

	created, err := manager.Create(ctx, "ship it", "complete tests", "high", "user", "", creator.ID, nil, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := manager.UpdateStatus(ctx, created.ID, StatusInProgress, "started"); err != nil {
		t.Fatalf("UpdateStatus(in_progress) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusCompleted, "done"); err != nil {
		t.Fatalf("UpdateStatus(completed with reviewer) error = %v", err)
	}

	blocked, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(blocked) error = %v", err)
	}
	if blocked.Status != StatusBlocked {
		t.Fatalf("Status = %q, want %q", blocked.Status, StatusBlocked)
	}
	if !strings.Contains(blocked.Result, "review required") {
		t.Fatalf("Result = %q, want review required message", blocked.Result)
	}

	if err := manager.ApproveTask(ctx, created.ID, "approved"); err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	approved, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(approved) error = %v", err)
	}
	if approved.Status != StatusCompleted || approved.Result != "approved" {
		t.Fatalf("approved task = %+v", approved)
	}

	rejected, err := manager.Create(ctx, "ship it 2", "complete tests", "high", "user", "", creator.ID, nil, nil)
	if err != nil {
		t.Fatalf("Create(reject) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, rejected.ID, StatusInProgress, "started"); err != nil {
		t.Fatalf("UpdateStatus(rejected in_progress) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, rejected.ID, StatusCompleted, "done"); err != nil {
		t.Fatalf("UpdateStatus(rejected completed) error = %v", err)
	}
	if err := manager.RejectTask(ctx, rejected.ID, "changes requested"); err != nil {
		t.Fatalf("RejectTask() error = %v", err)
	}
	failed, err := query.FindByID(ctx, rejected.ID)
	if err != nil {
		t.Fatalf("FindByID(failed) error = %v", err)
	}
	if failed.Status != StatusFailed || failed.Result != "changes requested" {
		t.Fatalf("failed task = %+v", failed)
	}
}

func TestManagerReopensTerminalTasks(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	query := NewQuery(database)

	tests := []struct {
		name         string
		activate     bool
		terminal     string
		reopened     string
		reopenResult string
		nextStatus   string
	}{
		{name: "completed reopens to pending", activate: true, terminal: StatusCompleted, reopened: StatusPending, reopenResult: "reopened from done", nextStatus: StatusInProgress},
		{name: "failed retries to pending", activate: true, terminal: StatusFailed, reopened: StatusPending, reopenResult: "retry requested", nextStatus: StatusInProgress},
		{name: "cancelled resurrects to pending", terminal: StatusCancelled, reopened: StatusPending, reopenResult: "resurrected", nextStatus: StatusInProgress},
		{name: "completed moves to cancelled", activate: true, terminal: StatusCompleted, reopened: StatusCancelled, reopenResult: "closed after completion"},
		{name: "failed moves to cancelled", activate: true, terminal: StatusFailed, reopened: StatusCancelled, reopenResult: "closed after failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := manager.Create(ctx, tt.name, "terminal reopen flow", "medium", "", "", creator.ID, nil, nil)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			if tt.activate {
				if err := manager.UpdateStatus(ctx, created.ID, StatusInProgress, "started"); err != nil {
					t.Fatalf("UpdateStatus(in_progress) error = %v", err)
				}
			}
			if err := manager.UpdateStatus(ctx, created.ID, tt.terminal, "terminal result"); err != nil {
				t.Fatalf("UpdateStatus(%s) error = %v", tt.terminal, err)
			}
			if err := manager.UpdateStatus(ctx, created.ID, tt.reopened, tt.reopenResult); err != nil {
				t.Fatalf("UpdateStatus(%s) error = %v", tt.reopened, err)
			}

			reopened, err := query.FindByID(ctx, created.ID)
			if err != nil {
				t.Fatalf("FindByID(reopened) error = %v", err)
			}
			if reopened.Status != tt.reopened {
				t.Fatalf("Status = %q, want %q", reopened.Status, tt.reopened)
			}
			if reopened.Result != tt.reopenResult {
				t.Fatalf("Result = %q, want %q", reopened.Result, tt.reopenResult)
			}

			if tt.nextStatus == "" {
				return
			}
			if err := manager.UpdateStatus(ctx, created.ID, tt.nextStatus, "resumed"); err != nil {
				t.Fatalf("UpdateStatus(%s after reopen) error = %v", tt.nextStatus, err)
			}

			resumed, err := query.FindByID(ctx, created.ID)
			if err != nil {
				t.Fatalf("FindByID(resumed) error = %v", err)
			}
			if resumed.Status != tt.nextStatus {
				t.Fatalf("Status after resume = %q, want %q", resumed.Status, tt.nextStatus)
			}
		})
	}
}

func TestManagerReopenedTaskStillRequiresReview(t *testing.T) {
	database := setupTaskTestDB(t)
	ctx := context.Background()
	creator := insertTaskTestAgent(t, database, "creator")

	manager := NewManager(database)
	query := NewQuery(database)

	created, err := manager.Create(ctx, "reviewed reopen", "ensure review survives reopen", "high", "user", "", creator.ID, nil, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := manager.UpdateStatus(ctx, created.ID, StatusInProgress, "started"); err != nil {
		t.Fatalf("UpdateStatus(in_progress) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusCompleted, "done"); err != nil {
		t.Fatalf("UpdateStatus(completed) error = %v", err)
	}
	if err := manager.ApproveTask(ctx, created.ID, "approved"); err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusPending, "reopened"); err != nil {
		t.Fatalf("UpdateStatus(pending) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusInProgress, "started again"); err != nil {
		t.Fatalf("UpdateStatus(in_progress again) error = %v", err)
	}
	if err := manager.UpdateStatus(ctx, created.ID, StatusCompleted, "done again"); err != nil {
		t.Fatalf("UpdateStatus(completed again) error = %v", err)
	}

	blocked, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(blocked) error = %v", err)
	}
	if blocked.Status != StatusBlocked {
		t.Fatalf("Status = %q, want %q", blocked.Status, StatusBlocked)
	}
	if !strings.Contains(blocked.Result, "review required by user") {
		t.Fatalf("Result = %q, want review required message", blocked.Result)
	}

	if err := manager.RejectTask(ctx, created.ID, "changes requested again"); err != nil {
		t.Fatalf("RejectTask() error = %v", err)
	}

	failed, err := query.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID(failed) error = %v", err)
	}
	if failed.Status != StatusFailed || failed.Result != "changes requested again" {
		t.Fatalf("failed task = %+v", failed)
	}
}
