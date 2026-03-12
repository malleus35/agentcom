// Package config provides path resolution and environment configuration
// for agentcom. It determines the base directory (~/.agentcom/ by default)
// and ensures required subdirectories exist.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultDirName is the default directory name under user's home.
	DefaultDirName = ".agentcom"

	// EnvHome overrides the default base directory.
	EnvHome = "AGENTCOM_HOME"

	// DBFileName is the SQLite database file name.
	DBFileName = "agentcom.db"

	// SocketsDir is the subdirectory for agent Unix Domain Sockets.
	SocketsDir = "sockets"
)

// Config holds resolved paths for agentcom.
type Config struct {
	// HomeDir is the base directory (e.g., ~/.agentcom/).
	HomeDir string

	// DBPath is the full path to the SQLite database file.
	DBPath string

	// SocketsPath is the directory where agent sockets are stored.
	SocketsPath string
}

// Load resolves the agentcom configuration from environment and defaults.
// It does NOT create directories — call EnsureDirs() separately.
func Load() (*Config, error) {
	homeDir, err := resolveHomeDir()
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	return &Config{
		HomeDir:     homeDir,
		DBPath:      filepath.Join(homeDir, DBFileName),
		SocketsPath: filepath.Join(homeDir, SocketsDir),
	}, nil
}

// EnsureDirs creates the base directory and all required subdirectories.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.HomeDir,
		c.SocketsPath,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config.EnsureDirs: %w", err)
		}
	}
	return nil
}

// resolveHomeDir determines the agentcom home directory.
// Priority: AGENTCOM_HOME env > ~/.agentcom/
func resolveHomeDir() (string, error) {
	if env := os.Getenv(EnvHome); env != "" {
		return env, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config.resolveHomeDir: %w", err)
	}

	return filepath.Join(home, DefaultDirName), nil
}
