package onboard

import (
	"fmt"
	"path/filepath"
)

var validTemplates = map[string]struct{}{
	"":               {},
	"company":        {},
	"oh-my-opencode": {},
}

// Result captures the selections made during onboarding.
type Result struct {
	HomeDir       string
	Template      string
	WriteAgentsMD bool
	Confirmed     bool
}

// ApplyReport describes the filesystem changes produced by onboarding.
type ApplyReport struct {
	HomeDir        string   `json:"home_dir"`
	DBPath         string   `json:"db_path"`
	Status         string   `json:"status"`
	Template       string   `json:"template,omitempty"`
	AgentsMDPath   string   `json:"agents_md,omitempty"`
	GeneratedFiles []string `json:"generated_files,omitempty"`
}

// Validate checks whether the onboarding result can be applied safely.
func (r Result) Validate() error {
	if r.HomeDir == "" {
		return fmt.Errorf("onboard.Result.Validate: home directory is required")
	}
	if !filepath.IsAbs(r.HomeDir) {
		return fmt.Errorf("onboard.Result.Validate: home directory must be an absolute path")
	}
	if _, ok := validTemplates[r.Template]; !ok {
		return fmt.Errorf("onboard.Result.Validate: unsupported template %q", r.Template)
	}
	if !r.Confirmed {
		return fmt.Errorf("onboard.Result.Validate: onboarding not confirmed")
	}
	return nil
}
