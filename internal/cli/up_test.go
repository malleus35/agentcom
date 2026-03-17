package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func setupUpTestDB(t *testing.T) *db.DB {
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

func TestEnsureInitProjectConfigSetsActiveTemplate(t *testing.T) {
	projectDir := t.TempDir()

	oldProjectFlag := projectFlag
	projectFlag = ""
	defer func() { projectFlag = oldProjectFlag }()

	path, cfg, err := ensureInitProjectConfig(projectDir, false, "company")
	if err != nil {
		t.Fatalf("ensureInitProjectConfig() error = %v", err)
	}
	if path != filepath.Join(projectDir, config.ProjectConfigFileName) {
		t.Fatalf("path = %q, want %q", path, filepath.Join(projectDir, config.ProjectConfigFileName))
	}
	if cfg.Template.Active != "company" {
		t.Fatalf("Template.Active = %q, want company", cfg.Template.Active)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) == "" {
		t.Fatal("config file is empty")
	}
}

func TestFilterTemplateRolesByOnlySelection(t *testing.T) {
	roles := []templateManifestRole{{Name: "frontend"}, {Name: "backend"}, {Name: "plan"}}

	filtered, err := filterTemplateRoles(roles, "plan,frontend")
	if err != nil {
		t.Fatalf("filterTemplateRoles() error = %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
	if filtered[0].Name != "plan" || filtered[1].Name != "frontend" {
		t.Fatalf("filtered roles = %#v, want [plan frontend]", filtered)
	}
}

func TestRegisterUserPseudoAgent(t *testing.T) {
	database := setupUpTestDB(t)
	ctx := context.Background()

	stale := &db.Agent{Name: "user", Type: "human", Project: "demo-app", Status: "alive", PID: 99}
	if err := database.InsertAgent(ctx, stale); err != nil {
		t.Fatalf("InsertAgent(stale) error = %v", err)
	}

	userAgent, err := registerUserPseudoAgent(ctx, database, 12345, "demo-app")
	if err != nil {
		t.Fatalf("registerUserPseudoAgent() error = %v", err)
	}
	if userAgent.Role != "user" || userAgent.Type != "human" {
		t.Fatalf("userAgent = %+v, want role=user type=human", userAgent)
	}
	if userAgent.PID != 12345 {
		t.Fatalf("userAgent.PID = %d, want 12345", userAgent.PID)
	}
	if userAgent.AgentID == stale.ID {
		t.Fatalf("userAgent.AgentID = %q, want stale agent to be replaced", userAgent.AgentID)
	}

	stored, err := database.FindAgentByID(ctx, userAgent.AgentID)
	if err != nil {
		t.Fatalf("FindAgentByID() error = %v", err)
	}
	if stored.Name != "user" || stored.Type != "human" || stored.PID != 12345 {
		t.Fatalf("stored user = %+v", stored)
	}
	if _, err := database.FindAgentByID(ctx, stale.ID); err != db.ErrAgentNotFound {
		t.Fatalf("FindAgentByID(stale) error = %v, want %v", err, db.ErrAgentNotFound)
	}
}

func TestDeregisterUserPseudoAgent(t *testing.T) {
	database := setupUpTestDB(t)
	ctx := context.Background()

	userAgent, err := registerUserPseudoAgent(ctx, database, 12345, "demo-app")
	if err != nil {
		t.Fatalf("registerUserPseudoAgent() error = %v", err)
	}

	if err := deregisterUserPseudoAgent(ctx, database, userAgent); err != nil {
		t.Fatalf("deregisterUserPseudoAgent() error = %v", err)
	}
	if _, err := database.FindAgentByID(ctx, userAgent.AgentID); err != db.ErrAgentNotFound {
		t.Fatalf("FindAgentByID(after deregister) error = %v, want %v", err, db.ErrAgentNotFound)
	}

	if err := deregisterUserPseudoAgent(ctx, database, userAgent); err != nil {
		t.Fatalf("deregisterUserPseudoAgent(second call) error = %v", err)
	}
}

func TestLoadUpRuntimeStateBackwardsCompatibleWithoutUserAgent(t *testing.T) {
	projectDir := t.TempDir()
	path := upRuntimeStatePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	legacyState := map[string]any{
		"project":        "demo-app",
		"project_dir":    projectDir,
		"template":       "company",
		"started_at":     "2026-03-17T00:00:00Z",
		"supervisor_pid": 12345,
		"agents": []map[string]any{{
			"role":     "frontend",
			"name":     "frontend",
			"type":     "engineer-frontend",
			"pid":      54321,
			"project":  "demo-app",
			"agent_id": "agt_frontend",
		}},
	}
	data, err := json.Marshal(legacyState)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	state, _, err := loadUpRuntimeState(projectDir)
	if err != nil {
		t.Fatalf("loadUpRuntimeState() error = %v", err)
	}
	if state.UserAgent != nil {
		t.Fatalf("state.UserAgent = %+v, want nil", state.UserAgent)
	}
	if len(state.Agents) != 1 {
		t.Fatalf("len(state.Agents) = %d, want 1", len(state.Agents))
	}
}
