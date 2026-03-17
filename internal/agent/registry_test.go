package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func setupRegistryTest(t *testing.T) (*Registry, *db.DB, *config.Config) {
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

	homeDir := t.TempDir()
	cfg := &config.Config{
		HomeDir:     homeDir,
		DBPath:      filepath.Join(homeDir, config.DBFileName),
		SocketsPath: filepath.Join(homeDir, config.SocketsDir),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("cfg.EnsureDirs() error = %v", err)
	}

	return NewRegistry(database, cfg), database, cfg
}

func TestRegistryRegisterAndDeregister(t *testing.T) {
	registry, _, cfg := setupRegistryTest(t)
	ctx := context.Background()

	agent, err := registry.Register(ctx, "alpha", "worker", []string{"send", "recv"}, "/workspace", "project-a")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if agent.ID == "" {
		t.Fatal("Register() returned empty ID")
	}
	wantSocketPath := filepath.Join(cfg.SocketsPath, agent.ID+".sock")
	if agent.SocketPath != wantSocketPath {
		t.Fatalf("SocketPath = %q, want %q", agent.SocketPath, wantSocketPath)
	}

	if _, err := registry.Register(ctx, "alpha", "worker", nil, "", "project-a"); !errors.Is(err, db.ErrDuplicateName) {
		t.Fatalf("Register(duplicate) error = %v, want %v", err, db.ErrDuplicateName)
	}
	if _, err := registry.Register(ctx, "alpha", "worker", nil, "", "project-b"); err != nil {
		t.Fatalf("Register(cross project duplicate) error = %v", err)
	}

	if err := os.WriteFile(agent.SocketPath, []byte("socket"), 0o600); err != nil {
		t.Fatalf("WriteFile(socket) error = %v", err)
	}
	if err := registry.Deregister(ctx, agent.Name, agent.Project); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if _, err := os.Stat(agent.SocketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket file still exists, stat error = %v", err)
	}
	if _, err := registry.FindByID(ctx, agent.ID); !errors.Is(err, db.ErrAgentNotFound) {
		t.Fatalf("FindByID(after deregister) error = %v, want %v", err, db.ErrAgentNotFound)
	}
}

func TestRegistryMarkInactive(t *testing.T) {
	registry, database, _ := setupRegistryTest(t)
	ctx := context.Background()

	stale, err := registry.Register(ctx, "stale", "worker", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register(stale) error = %v", err)
	}
	alive, err := registry.Register(ctx, "alive", "worker", nil, "", "project-b")
	if err != nil {
		t.Fatalf("Register(alive) error = %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE agents SET pid = ?, last_heartbeat = datetime('now', '-31 seconds') WHERE id = ?`, -1, stale.ID); err != nil {
		t.Fatalf("ExecContext(stale update) error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE agents SET pid = ?, last_heartbeat = datetime('now') WHERE id = ?`, os.Getpid(), alive.ID); err != nil {
		t.Fatalf("ExecContext(alive update) error = %v", err)
	}

	if err := registry.MarkInactive(ctx); err != nil {
		t.Fatalf("MarkInactive() error = %v", err)
	}

	staleAgent, err := registry.FindByID(ctx, stale.ID)
	if err != nil {
		t.Fatalf("FindByID(stale) error = %v", err)
	}
	if staleAgent.Status != "dead" {
		t.Fatalf("stale agent status = %q, want dead", staleAgent.Status)
	}

	aliveAgent, err := registry.FindByID(ctx, alive.ID)
	if err != nil {
		t.Fatalf("FindByID(alive) error = %v", err)
	}
	if aliveAgent.Status != "alive" {
		t.Fatalf("alive agent status = %q, want alive", aliveAgent.Status)
	}
}

func TestRegistryMarkInactiveSkipsHuman(t *testing.T) {
	registry, database, _ := setupRegistryTest(t)
	ctx := context.Background()

	human, err := registry.Register(ctx, "user", "human", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register(user) error = %v", err)
	}
	worker, err := registry.Register(ctx, "worker", "worker", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register(worker) error = %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE agents SET pid = ?, last_heartbeat = datetime('now', '-31 seconds') WHERE id = ?`, 0, human.ID); err != nil {
		t.Fatalf("ExecContext(human update) error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE agents SET pid = ?, last_heartbeat = datetime('now', '-31 seconds') WHERE id = ?`, -1, worker.ID); err != nil {
		t.Fatalf("ExecContext(worker update) error = %v", err)
	}

	if err := registry.MarkInactive(ctx); err != nil {
		t.Fatalf("MarkInactive() error = %v", err)
	}

	humanAgent, err := registry.FindByID(ctx, human.ID)
	if err != nil {
		t.Fatalf("FindByID(human) error = %v", err)
	}
	if humanAgent.Status != "alive" {
		t.Fatalf("human agent status = %q, want alive", humanAgent.Status)
	}

	workerAgent, err := registry.FindByID(ctx, worker.ID)
	if err != nil {
		t.Fatalf("FindByID(worker) error = %v", err)
	}
	if workerAgent.Status != "dead" {
		t.Fatalf("worker agent status = %q, want dead", workerAgent.Status)
	}
}

func TestHeartbeatStart(t *testing.T) {
	registry, database, _ := setupRegistryTest(t)
	ctx := context.Background()

	agent, err := registry.Register(ctx, "heartbeat", "worker", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE agents SET last_heartbeat = datetime('now', '-31 seconds') WHERE id = ?`, agent.ID); err != nil {
		t.Fatalf("ExecContext(set stale heartbeat) error = %v", err)
	}

	before, err := database.FindAgentByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("FindAgentByID(before) error = %v", err)
	}

	hb := NewHeartbeat(database, agent.ID)
	hb.interval = 10 * time.Millisecond

	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	hb.Start(hbCtx)

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		current, err := database.FindAgentByID(ctx, agent.ID)
		if err != nil {
			t.Fatalf("FindAgentByID(poll) error = %v", err)
		}
		if current.LastHeartbeat.After(before.LastHeartbeat) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("heartbeat did not update last_heartbeat")
}

func TestRegistryProjectFiltering(t *testing.T) {
	registry, _, _ := setupRegistryTest(t)
	ctx := context.Background()

	alpha, err := registry.Register(ctx, "alpha", "worker", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register(alpha) error = %v", err)
	}
	beta, err := registry.Register(ctx, "beta", "worker", nil, "", "project-b")
	if err != nil {
		t.Fatalf("Register(beta) error = %v", err)
	}

	gotAlpha, err := registry.FindByName(ctx, "alpha", "project-a")
	if err != nil {
		t.Fatalf("FindByName(project-a) error = %v", err)
	}
	if gotAlpha.ID != alpha.ID {
		t.Fatalf("FindByName(project-a) id = %q, want %q", gotAlpha.ID, alpha.ID)
	}
	if _, err := registry.FindByName(ctx, "alpha", "project-b"); !errors.Is(err, db.ErrAgentNotFound) {
		t.Fatalf("FindByName(wrong project) error = %v, want %v", err, db.ErrAgentNotFound)
	}

	projectA, err := registry.ListAll(ctx, "project-a")
	if err != nil {
		t.Fatalf("ListAll(project-a) error = %v", err)
	}
	if len(projectA) != 1 || projectA[0].ID != alpha.ID {
		t.Fatalf("ListAll(project-a) = %+v, want only %q", projectA, alpha.ID)
	}

	projectBAlive, err := registry.ListAlive(ctx, "project-b")
	if err != nil {
		t.Fatalf("ListAlive(project-b) error = %v", err)
	}
	if len(projectBAlive) != 1 || projectBAlive[0].ID != beta.ID {
		t.Fatalf("ListAlive(project-b) = %+v, want only %q", projectBAlive, beta.ID)
	}
}
