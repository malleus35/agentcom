package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func TestRegisterUsesStructuredErrorForInvalidName(t *testing.T) {
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
	defer database.Close()
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	oldApp := app
	app = &appContext{cfg: cfg, db: database, project: "demo-app"}
	defer func() { app = oldApp }()

	cmd := newRegisterCmd()
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))
	cmd.SetArgs([]string{"--name", "user", "--type", "worker"})

	err = cmd.Execute()
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
