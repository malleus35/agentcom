package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/task"
)

func TestTaskCreateRejectsInvalidPriority(t *testing.T) {
	projectDir, cleanup := setupTaskCommandContext(t, nil)
	defer cleanup()
	withTaskCommandDir(t, projectDir)

	cmd := newTaskCreateCmd()
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetArgs([]string{"priority test", "--priority", "urgent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("cmd.Execute() error = nil, want invalid priority error")
	}
}

func TestTaskCreateAppliesTemplateReviewPolicy(t *testing.T) {
	projectDir, cleanup := setupTaskCommandContext(t, &task.ReviewPolicy{
		RequireReviewAbove: task.PriorityHigh,
		DefaultReviewer:    "user",
	})
	defer cleanup()
	withTaskCommandDir(t, projectDir)

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newTaskCreateCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"policy task", "--priority", "HIGH"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if payload["priority"] != task.PriorityHigh {
		t.Fatalf("priority = %v, want high", payload["priority"])
	}
	if payload["reviewer"] != "user" {
		t.Fatalf("reviewer = %v, want user", payload["reviewer"])
	}
}

func TestTaskUpdateAndApproveCommands(t *testing.T) {
	projectDir, cleanup := setupTaskCommandContext(t, nil)
	defer cleanup()
	withTaskCommandDir(t, projectDir)

	reviewTask := &db.Task{Title: "review", Status: task.StatusInProgress, Priority: task.PriorityHigh, Reviewer: "user", BlockedBy: "[]"}
	if err := app.db.InsertTask(context.Background(), reviewTask); err != nil {
		t.Fatalf("InsertTask(reviewTask) error = %v", err)
	}

	updateCmd := newTaskUpdateCmd()
	updateCmd.SetOut(bytes.NewBuffer(nil))
	updateCmd.SetArgs([]string{reviewTask.ID, "--status", task.StatusCompleted, "--result", "done"})
	if err := updateCmd.Execute(); err != nil {
		t.Fatalf("updateCmd.Execute() error = %v", err)
	}

	blocked, err := app.db.FindTaskByID(context.Background(), reviewTask.ID)
	if err != nil {
		t.Fatalf("FindTaskByID(blocked) error = %v", err)
	}
	if blocked.Status != task.StatusBlocked {
		t.Fatalf("Status = %q, want %q", blocked.Status, task.StatusBlocked)
	}

	approveCmd := newTaskApproveCmd()
	approveCmd.SetOut(bytes.NewBuffer(nil))
	approveCmd.SetArgs([]string{reviewTask.ID, "--result", "approved"})
	if err := approveCmd.Execute(); err != nil {
		t.Fatalf("approveCmd.Execute() error = %v", err)
	}

	approved, err := app.db.FindTaskByID(context.Background(), reviewTask.ID)
	if err != nil {
		t.Fatalf("FindTaskByID(approved) error = %v", err)
	}
	if approved.Status != task.StatusCompleted || approved.Result != "approved" {
		t.Fatalf("approved task = %+v", approved)
	}
}

func TestTaskCreateUsesStructuredErrorForUnknownAssignee(t *testing.T) {
	projectDir, cleanup := setupTaskCommandContext(t, nil)
	defer cleanup()
	withTaskCommandDir(t, projectDir)

	cmd := newTaskCreateCmd()
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetArgs([]string{"task with missing assignee", "--assign", "missing-agent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("cmd.Execute() error = nil, want structured error")
	}
	message := err.Error()
	for _, want := range []string{"Error:", "Reason:", "Hint:"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
}

func setupTaskCommandContext(t *testing.T, policy *task.ReviewPolicy) (string, func()) {
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
		t.Fatalf("database.Migrate() error = %v", err)
	}

	if _, err := config.SaveProjectConfig(projectDir, config.ProjectConfig{Project: "demo-app", Template: config.ProjectTemplateConfig{Active: "test-template"}}); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}
	if policy != nil {
		templateDir := filepath.Join(projectDir, ".agentcom", "templates", "test-template")
		if err := os.MkdirAll(templateDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(templateDir) error = %v", err)
		}
		manifest, err := json.Marshal(map[string]any{"name": "test-template", "review_policy": policy})
		if err != nil {
			t.Fatalf("json.Marshal(manifest) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(templateDir, "template.json"), append(manifest, '\n'), 0o644); err != nil {
			t.Fatalf("WriteFile(template.json) error = %v", err)
		}
	}

	oldApp := app
	app = &appContext{cfg: cfg, db: database, project: "demo-app"}
	return projectDir, func() {
		app = oldApp
		if err := database.Close(); err != nil {
			t.Fatalf("database.Close() error = %v", err)
		}
	}
}

func withTaskCommandDir(t *testing.T, dir string) {
	t.Helper()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
