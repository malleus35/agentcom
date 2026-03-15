package onboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
)

func TestIsFirstRun(t *testing.T) {
	t.Run("nonexistent directory", func(t *testing.T) {
		if !IsFirstRun(filepath.Join(t.TempDir(), "missing")) {
			t.Fatal("IsFirstRun() = false, want true")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		if !IsFirstRun(t.TempDir()) {
			t.Fatal("IsFirstRun() = false, want true")
		}
	})

	t.Run("existing database", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, config.DBFileName)
		if err := os.WriteFile(path, []byte("db"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if IsFirstRun(home) {
			t.Fatal("IsFirstRun() = true, want false")
		}
	})
}

func TestDetectDefaults(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "agentcom-home")
	t.Setenv(config.EnvHome, expected)

	got, err := DetectDefaults()
	if err != nil {
		t.Fatalf("DetectDefaults() error = %v", err)
	}
	if got.HomeDir != expected {
		t.Fatalf("HomeDir = %q, want %q", got.HomeDir, expected)
	}
	if got.Template != "" {
		t.Fatalf("Template = %q, want empty", got.Template)
	}
	if got.WriteAgentsMD {
		t.Fatal("WriteAgentsMD = true, want false")
	}
}
