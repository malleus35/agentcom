package onboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/malleus35/agentcom/internal/config"
)

// DetectDefaults resolves the default onboarding selections from the current environment.
func DetectDefaults() (Result, error) {
	cfg, err := config.Load()
	if err != nil {
		return Result{}, fmt.Errorf("onboard.DetectDefaults: %w", err)
	}
	return Result{
		HomeDir:       cfg.HomeDir,
		Template:      "",
		WriteAgentsMD: false,
		Confirmed:     false,
	}, nil
}

// IsFirstRun reports whether the given agentcom home has not been initialized yet.
func IsFirstRun(homeDir string) bool {
	if homeDir == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(homeDir, config.DBFileName))
	return os.IsNotExist(err)
}
