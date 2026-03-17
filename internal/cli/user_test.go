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

func TestUserInboxJSONMarksMessagesRead(t *testing.T) {
	projectDir, cleanup := setupUserTestContext(t)
	defer cleanup()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newUserCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"inbox"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	var messages []userMessageView
	if err := json.Unmarshal(buf.Bytes(), &messages); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0].FromAgentName != "plan" {
		t.Fatalf("messages[0].FromAgentName = %q, want plan", messages[0].FromAgentName)
	}

	unread, err := app.db.ListUnreadMessages(context.Background(), testUserAgentID(t))
	if err != nil {
		t.Fatalf("ListUnreadMessages() error = %v", err)
	}
	if len(unread) != 0 {
		t.Fatalf("len(unread) = %d, want 0", len(unread))
	}
}

func TestUserReplyCreatesResponseMessage(t *testing.T) {
	projectDir, cleanup := setupUserTestContext(t)
	defer cleanup()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newUserCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"reply", "plan", `{"text":"Yes"}`})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	planAgent, err := app.db.FindAgentByNameAndProject(context.Background(), "plan", "demo-app")
	if err != nil {
		t.Fatalf("FindAgentByNameAndProject(plan) error = %v", err)
	}
	messages, err := app.db.ListMessagesForAgent(context.Background(), planAgent.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(plan) error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Type != "response" {
		t.Fatalf("message type = %q, want response", messages[0].Type)
	}
	if messages[0].FromAgent != testUserAgentID(t) {
		t.Fatalf("message from = %q, want %q", messages[0].FromAgent, testUserAgentID(t))
	}
}

func TestUserPendingFiltersUnreadRequests(t *testing.T) {
	projectDir, cleanup := setupUserTestContext(t)
	defer cleanup()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newUserCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pending"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	var messages []userMessageView
	if err := json.Unmarshal(buf.Bytes(), &messages); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Type != "request" {
		t.Fatalf("messages[0].Type = %q, want request", messages[0].Type)
	}
}

func TestUserInboxWithoutUserAgentFailsClearly(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	projectDir := t.TempDir()
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
	defer database.Close()
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if _, err := config.SaveProjectConfig(projectDir, config.ProjectConfig{Project: "demo-app"}); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	oldApp := app
	app = &appContext{cfg: cfg, db: database, project: "demo-app"}
	defer func() { app = oldApp }()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cmd := newUserCmd()
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetArgs([]string{"inbox"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("cmd.Execute() error = nil, want user agent error")
	}
	if err.Error() != "cli.newUserInboxCmd: no user agent registered; run `agentcom up` first" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRootCommandContainsUserSubcommand(t *testing.T) {
	root := NewRootCmd()
	if _, _, err := root.Find([]string{"user"}); err != nil {
		t.Fatalf("root.Find(user) error = %v", err)
	}
}

func setupUserTestContext(t *testing.T) (string, func()) {
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
	if _, err := config.SaveProjectConfig(projectDir, config.ProjectConfig{Project: "demo-app"}); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	plan := &db.Agent{Name: "plan", Type: "worker", Project: "demo-app", Status: "alive"}
	user := &db.Agent{Name: "user", Type: "human", Project: "demo-app", Status: "alive"}
	other := &db.Agent{Name: "other", Type: "worker", Project: "demo-app", Status: "alive"}
	for _, agent := range []*db.Agent{plan, user, other} {
		if err := database.InsertAgent(context.Background(), agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	if err := database.InsertMessage(context.Background(), &db.Message{FromAgent: plan.ID, ToAgent: user.ID, Type: "request", Topic: "approval", Payload: `{"text":"Proceed?"}`, CreatedAt: "2026-03-17 10:00:00"}); err != nil {
		t.Fatalf("InsertMessage(request) error = %v", err)
	}
	if err := database.InsertMessage(context.Background(), &db.Message{FromAgent: plan.ID, ToAgent: user.ID, Type: "notification", Topic: "note", Payload: `{"text":"FYI"}`, CreatedAt: "2026-03-17 10:01:00"}); err != nil {
		t.Fatalf("InsertMessage(notification) error = %v", err)
	}
	if err := database.InsertMessage(context.Background(), &db.Message{FromAgent: other.ID, ToAgent: user.ID, Type: "request", Topic: "read", Payload: `{"text":"Already read"}`, CreatedAt: "2026-03-17 10:02:00", ReadAt: "2026-03-17 10:03:00"}); err != nil {
		t.Fatalf("InsertMessage(read request) error = %v", err)
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

func testUserAgentID(t *testing.T) string {
	t.Helper()
	userAgent, err := app.db.FindAgentByNameAndProject(context.Background(), "user", "demo-app")
	if err != nil {
		t.Fatalf("FindAgentByNameAndProject(user) error = %v", err)
	}
	return userAgent.ID
}
