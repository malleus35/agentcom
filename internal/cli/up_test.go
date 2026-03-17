package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestHandleExistingRuntimeStateRemovesStaleState(t *testing.T) {
	projectDir := t.TempDir()
	staleSocketPath := filepath.Join(projectDir, "stale.sock")
	if err := os.WriteFile(staleSocketPath, []byte("socket"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale socket) error = %v", err)
	}

	if err := writeUpRuntimeState(projectDir, upRuntimeState{
		ProjectDir:    projectDir,
		Template:      "company",
		StartedAt:     time.Now().UTC(),
		SupervisorPID: -1,
		Agents: []upRuntimeStateAgent{{
			Role:       "frontend",
			Name:       "frontend",
			Type:       "worker",
			PID:        -1,
			SocketPath: staleSocketPath,
		}},
	}); err != nil {
		t.Fatalf("writeUpRuntimeState() error = %v", err)
	}

	if err := handleExistingRuntimeState(projectDir, false); err != nil {
		t.Fatalf("handleExistingRuntimeState() error = %v", err)
	}

	state, _, err := loadUpRuntimeState(projectDir)
	if err != nil {
		t.Fatalf("loadUpRuntimeState() error = %v", err)
	}
	if state.SupervisorPID != 0 {
		t.Fatalf("SupervisorPID = %d, want stale state removed", state.SupervisorPID)
	}
	if _, err := os.Stat(staleSocketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale socket stat error = %v, want not exist", err)
	}
}

func TestCollectStaleRuntimeAgents(t *testing.T) {
	database := setupUpTestDB(t)
	ctx := context.Background()

	staleWorker := &db.Agent{Name: "stale", Type: "worker", Status: "alive", PID: -1, Project: "demo-app"}
	if err := database.InsertAgent(ctx, staleWorker); err != nil {
		t.Fatalf("InsertAgent(staleWorker) error = %v", err)
	}
	aliveWorker := &db.Agent{Name: "alive", Type: "worker", Status: "alive", PID: os.Getpid(), Project: "demo-app"}
	if err := database.InsertAgent(ctx, aliveWorker); err != nil {
		t.Fatalf("InsertAgent(aliveWorker) error = %v", err)
	}
	human := &db.Agent{Name: "user", Type: "human", Status: "alive", PID: -1, Project: "demo-app"}
	if err := database.InsertAgent(ctx, human); err != nil {
		t.Fatalf("InsertAgent(human) error = %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE agents SET last_heartbeat = datetime('now', '-31 seconds') WHERE id IN (?, ?)`, staleWorker.ID, human.ID); err != nil {
		t.Fatalf("ExecContext(stale heartbeat) error = %v", err)
	}

	staleAgents, err := collectStaleRuntimeAgents(ctx, database, []upRuntimeStateAgent{
		{Role: "stale", AgentID: staleWorker.ID, PID: staleWorker.PID, Type: staleWorker.Type},
		{Role: "alive", AgentID: aliveWorker.ID, PID: aliveWorker.PID, Type: aliveWorker.Type},
		{Role: "user", AgentID: human.ID, PID: human.PID, Type: human.Type},
	}, 30*time.Second, time.Now().UTC())
	if err != nil {
		t.Fatalf("collectStaleRuntimeAgents() error = %v", err)
	}
	if len(staleAgents) != 1 {
		t.Fatalf("len(staleAgents) = %d, want 1", len(staleAgents))
	}
	if staleAgents[0].AgentID != staleWorker.ID {
		t.Fatalf("stale agent = %+v, want %q", staleAgents[0], staleWorker.ID)
	}
}

func TestRunWithCleanupTimeoutProvidesDeadline(t *testing.T) {
	const timeout = 250 * time.Millisecond

	var (
		hasDeadline bool
		remaining   time.Duration
	)

	err := runWithCleanupTimeout(timeout, func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("missing deadline")
		}
		hasDeadline = true
		remaining = time.Until(deadline)
		return nil
	})
	if err != nil {
		t.Fatalf("runWithCleanupTimeout() error = %v", err)
	}
	if !hasDeadline {
		t.Fatal("cleanup callback did not observe a deadline")
	}
	if remaining <= 0 || remaining > timeout {
		t.Fatalf("deadline remaining = %v, want > 0 and <= %v", remaining, timeout)
	}
}

func TestRunWithCleanupTimeoutReturnsCallbackError(t *testing.T) {
	wantErr := errors.New("boom")

	err := runWithCleanupTimeout(100*time.Millisecond, func(ctx context.Context) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWithCleanupTimeout() error = %v, want %v", err, wantErr)
	}
}
