package onboard

import (
	"fmt"
	"path/filepath"

	"github.com/malleus35/agentcom/internal/config"
)

// Result captures the selections made during onboarding.
type Result struct {
	HomeDir           string
	Project           string
	Template          string
	WriteAgentsMD     bool
	SelectedAgents    []string
	WriteMemory       bool
	WriteInstructions bool
	CustomTemplate    *TemplateDefinition
	Confirmed         bool
}

// ApplyReport describes the filesystem changes produced by onboarding.
type ApplyReport struct {
	HomeDir            string   `json:"home_dir"`
	DBPath             string   `json:"db_path"`
	Status             string   `json:"status"`
	Project            string   `json:"project,omitempty"`
	ProjectConfigPath  string   `json:"project_config_path,omitempty"`
	Template           string   `json:"template,omitempty"`
	AgentsMDPath       string   `json:"agents_md,omitempty"`
	InstructionFiles   []string `json:"instruction_files,omitempty"`
	MemoryFiles        []string `json:"memory_files,omitempty"`
	CustomTemplatePath string   `json:"custom_template_path,omitempty"`
	GeneratedFiles     []string `json:"generated_files,omitempty"`
}

// Validate checks whether the onboarding result can be applied safely.
func (r Result) Validate() error {
	if r.HomeDir == "" {
		return fmt.Errorf("onboard.Result.Validate: home directory is required")
	}
	if !filepath.IsAbs(r.HomeDir) {
		return fmt.Errorf("onboard.Result.Validate: home directory must be an absolute path")
	}
	if err := config.ValidateProjectName(r.Project); err != nil {
		return fmt.Errorf("onboard.Result.Validate: %w", err)
	}
	if r.WriteInstructions && len(r.SelectedAgents) == 0 {
		return fmt.Errorf("onboard.Result.Validate: at least one selected agent is required when writing instructions")
	}
	if !r.Confirmed {
		return fmt.Errorf("onboard.Result.Validate: onboarding not confirmed")
	}
	return nil
}
