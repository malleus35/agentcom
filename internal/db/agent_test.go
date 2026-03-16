package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInsertAgent(t *testing.T) {
	tests := []struct {
		name    string
		agent   Agent
		wantErr error
		check   func(t *testing.T, got *Agent)
	}{
		{
			name: "generates id when missing",
			agent: Agent{
				Name:         "alpha",
				Type:         "worker",
				SocketPath:   "/tmp/alpha.sock",
				Capabilities: `["send"]`,
				Project:      "project-a",
				Status:       "alive",
			},
			check: func(t *testing.T, got *Agent) {
				t.Helper()
				if !strings.HasPrefix(got.ID, "agt_") {
					t.Fatalf("ID = %q, want agt_ prefix", got.ID)
				}
			},
		},
		{
			name: "preserves preset id",
			agent: Agent{
				ID:           "agt_existing",
				Name:         "beta",
				Type:         "worker",
				SocketPath:   "/tmp/beta.sock",
				Capabilities: `["recv"]`,
				Project:      "project-b",
				Status:       "alive",
			},
			check: func(t *testing.T, got *Agent) {
				t.Helper()
				if got.ID != "agt_existing" {
					t.Fatalf("ID = %q, want agt_existing", got.ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := setupTestDB(t)
			ctx := context.Background()

			agent := tt.agent
			err := database.InsertAgent(ctx, &agent)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("InsertAgent() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			got, err := database.FindAgentByID(ctx, agent.ID)
			if err != nil {
				t.Fatalf("FindAgentByID() error = %v", err)
			}

			if got.Name != agent.Name {
				t.Fatalf("Name = %q, want %q", got.Name, agent.Name)
			}
			if got.SocketPath != agent.SocketPath {
				t.Fatalf("SocketPath = %q, want %q", got.SocketPath, agent.SocketPath)
			}
			if got.Project != agent.Project {
				t.Fatalf("Project = %q, want %q", got.Project, agent.Project)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestAgentCRUD(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	first := &Agent{Name: "alpha", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, first); err != nil {
		t.Fatalf("InsertAgent(first) error = %v", err)
	}

	duplicate := &Agent{Name: "alpha", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, duplicate); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("InsertAgent(duplicate) error = %v, want %v", err, ErrDuplicateName)
	}

	crossProject := &Agent{Name: "alpha", Type: "worker", Project: "project-b", Status: "alive"}
	if err := database.InsertAgent(ctx, crossProject); err != nil {
		t.Fatalf("InsertAgent(cross project) error = %v", err)
	}

	first.WorkDir = "/workspace/project"
	first.Project = "project-a-renamed"
	first.Status = "dead"
	if err := database.UpdateAgent(ctx, first); err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	byName, err := database.FindAgentByNameAndProject(ctx, first.Name, first.Project)
	if err != nil {
		t.Fatalf("FindAgentByNameAndProject() error = %v", err)
	}
	if byName.WorkDir != first.WorkDir {
		t.Fatalf("WorkDir = %q, want %q", byName.WorkDir, first.WorkDir)
	}
	if byName.Project != first.Project {
		t.Fatalf("Project = %q, want %q", byName.Project, first.Project)
	}

	second := &Agent{Name: "beta", Type: "reviewer", Project: "project-a-renamed", Status: "alive"}
	if err := database.InsertAgent(ctx, second); err != nil {
		t.Fatalf("InsertAgent(second) error = %v", err)
	}

	alive, err := database.ListAliveAgentsByProject(ctx, first.Project)
	if err != nil {
		t.Fatalf("ListAliveAgentsByProject() error = %v", err)
	}
	if len(alive) != 1 || alive[0].ID != second.ID {
		t.Fatalf("ListAliveAgentsByProject() = %+v, want only %q", alive, second.ID)
	}

	if err := database.UpdateHeartbeat(ctx, second.ID); err != nil {
		t.Fatalf("UpdateHeartbeat() error = %v", err)
	}

	all, err := database.ListAgentsByProject(ctx, first.Project)
	if err != nil {
		t.Fatalf("ListAgentsByProject() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(ListAgentsByProject()) = %d, want 2", len(all))
	}

	if err := database.DeleteAgent(ctx, first.ID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	if _, err := database.FindAgentByID(ctx, first.ID); !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("FindAgentByID(deleted) error = %v, want %v", err, ErrAgentNotFound)
	}
}

func TestListProjects(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	agents := []*Agent{
		{Name: "alpha", Type: "worker", Project: "project-b", Status: "alive"},
		{Name: "beta", Type: "worker", Project: "project-a", Status: "alive"},
		{Name: "gamma", Type: "worker", Project: "project-b", Status: "alive"},
		{Name: "legacy", Type: "worker", Project: "", Status: "alive"},
	}
	for _, agent := range agents {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	projects, err := database.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}
	if projects[0] != "project-a" || projects[1] != "project-b" {
		t.Fatalf("projects = %#v, want [project-a project-b]", projects)
	}
}
