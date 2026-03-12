package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvHome(t *testing.T) {
	t.Setenv(EnvHome, filepath.Join(t.TempDir(), "custom-home"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HomeDir != os.Getenv(EnvHome) {
		t.Fatalf("HomeDir = %q, want %q", cfg.HomeDir, os.Getenv(EnvHome))
	}
	if cfg.DBPath != filepath.Join(cfg.HomeDir, DBFileName) {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, filepath.Join(cfg.HomeDir, DBFileName))
	}
	if cfg.SocketsPath != filepath.Join(cfg.HomeDir, SocketsDir) {
		t.Fatalf("SocketsPath = %q, want %q", cfg.SocketsPath, filepath.Join(cfg.HomeDir, SocketsDir))
	}
}

func TestLoadUsesDefaultHomeWhenEnvMissing(t *testing.T) {
	t.Setenv(EnvHome, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if filepath.Base(cfg.HomeDir) != DefaultDirName {
		t.Fatalf("filepath.Base(HomeDir) = %q, want %q", filepath.Base(cfg.HomeDir), DefaultDirName)
	}
}

func TestEnsureDirsCreatesDirectories(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	cfg := &Config{
		HomeDir:     homeDir,
		DBPath:      filepath.Join(homeDir, DBFileName),
		SocketsPath: filepath.Join(homeDir, SocketsDir),
	}

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}

	for _, path := range []string{cfg.HomeDir, cfg.SocketsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", path)
		}
	}
}
