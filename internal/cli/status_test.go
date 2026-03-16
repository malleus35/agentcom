package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func TestStatusCommandJSONIncludesProjectTemplateAndRoleDetails(t *testing.T) {
	projectDir, cleanup := setupStatusTestContext(t)
	defer cleanup()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newStatusCmd()
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}

	if payload["project"] != "demo-app" {
		t.Fatalf("project = %v, want demo-app", payload["project"])
	}
	if payload["template"] != "company" {
		t.Fatalf("template = %v, want company", payload["template"])
	}
	if _, ok := payload["role_status"]; !ok {
		t.Fatalf("payload missing role_status: %#v", payload)
	}
	if _, ok := payload["unread_by_agent"]; !ok {
		t.Fatalf("payload missing unread_by_agent: %#v", payload)
	}
}

func TestStatusCommandTextIncludesTemplateAndRoleStatus(t *testing.T) {
	projectDir, cleanup := setupStatusTestContext(t)
	defer cleanup()
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newStatusCmd()
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := buf.String()
	for _, want := range []string{"template", "company", "role_frontend", "role_backend", "unread_frontend"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("status output missing %q: %s", want, output)
		}
	}
}

func setupStatusTestContext(t *testing.T) (string, func()) {
	t.Helper()
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	cfg := &config.Config{
		HomeDir:     homeDir,
		DBPath:      filepath.Join(homeDir, config.DBFileName),
		SocketsPath: filepath.Join(homeDir, config.SocketsDir),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if _, err := config.SaveProjectConfig(projectDir, config.ProjectConfig{Project: "demo-app", Template: config.ProjectTemplateConfig{Active: "company"}}); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	frontend := &db.Agent{Name: "frontend", Type: "engineer-frontend", Project: "demo-app", Status: "alive", PID: os.Getpid()}
	backend := &db.Agent{Name: "backend", Type: "engineer-backend", Project: "demo-app", Status: "alive", PID: os.Getpid()}
	for _, agent := range []*db.Agent{frontend, backend} {
		if err := database.InsertAgent(context.Background(), agent); err != nil {
			t.Fatalf("InsertAgent() error = %v", err)
		}
	}

	if err := database.InsertMessage(context.Background(), &db.Message{FromAgent: backend.ID, ToAgent: frontend.ID, Type: "notification", Payload: `{"text":"hello"}`}); err != nil {
		t.Fatalf("InsertMessage(frontend) error = %v", err)
	}
	if err := database.InsertTask(context.Background(), &db.Task{Title: "Review", Status: "pending", Priority: "high", AssignedTo: frontend.ID, CreatedBy: backend.ID}); err != nil {
		t.Fatalf("InsertTask() error = %v", err)
	}
	if err := writeUpRuntimeState(projectDir, upRuntimeState{
		Project:       "demo-app",
		ProjectDir:    projectDir,
		Template:      "company",
		SupervisorPID: os.Getpid(),
		Agents: []upRuntimeStateAgent{
			{Role: "frontend", Name: "frontend", Type: "engineer-frontend", PID: os.Getpid(), Project: "demo-app"},
			{Role: "backend", Name: "backend", Type: "engineer-backend", PID: os.Getpid(), Project: "demo-app"},
		},
	}); err != nil {
		t.Fatalf("writeUpRuntimeState() error = %v", err)
	}

	oldApp := app
	app = &appContext{cfg: cfg, db: database, project: "demo-app"}
	return projectDir, func() {
		app = oldApp
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
}
