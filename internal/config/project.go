package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ProjectConfigFileName = ".agentcom.json"
	maxProjectSearchDepth = 10
	maxProjectNameLength  = 64
)

var projectNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type ProjectConfig struct {
	Project  string                `json:"project"`
	Template ProjectTemplateConfig `json:"template,omitempty"`
}

type ProjectTemplateConfig struct {
	Active string `json:"active,omitempty"`
}

func SaveProjectConfig(dir string, cfg ProjectConfig) (string, error) {
	if err := ValidateProjectName(cfg.Project); err != nil {
		return "", fmt.Errorf("config.SaveProjectConfig: %w", err)
	}

	path := filepath.Join(dir, ProjectConfigFileName)
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("config.SaveProjectConfig: marshal: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("config.SaveProjectConfig: write: %w", err)
	}

	return path, nil
}

func WriteProjectConfig(dir string, project string) (string, error) {
	if err := ValidateProjectName(project); err != nil {
		return "", fmt.Errorf("config.WriteProjectConfig: %w", err)
	}

	path := filepath.Join(dir, ProjectConfigFileName)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("config.WriteProjectConfig: %s already exists", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("config.WriteProjectConfig: stat: %w", err)
	}

	writtenPath, err := SaveProjectConfig(dir, ProjectConfig{Project: project})
	if err != nil {
		return "", fmt.Errorf("config.WriteProjectConfig: %w", err)
	}

	return writtenPath, nil
}

func LoadProjectConfig(startDir string) (ProjectConfig, string, error) {
	current := startDir
	for depth := 0; depth <= maxProjectSearchDepth; depth++ {
		path := filepath.Join(current, ProjectConfigFileName)
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg ProjectConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return ProjectConfig{}, "", fmt.Errorf("config.LoadProjectConfig: decode %s: %w", path, err)
			}
			if err := ValidateProjectName(cfg.Project); err != nil {
				return ProjectConfig{}, "", fmt.Errorf("config.LoadProjectConfig: %w", err)
			}
			return cfg, path, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, "", fmt.Errorf("config.LoadProjectConfig: read %s: %w", path, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return ProjectConfig{}, "", nil
}

func ResolveProject(explicit string, startDir string) (string, error) {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		if err := ValidateProjectName(explicit); err != nil {
			return "", fmt.Errorf("config.ResolveProject: %w", err)
		}
		return explicit, nil
	}

	cfg, _, err := LoadProjectConfig(startDir)
	if err != nil {
		return "", fmt.Errorf("config.ResolveProject: %w", err)
	}
	return cfg.Project, nil
}

func ValidateProjectName(project string) error {
	if project == "" {
		return nil
	}
	if len(project) > maxProjectNameLength {
		return fmt.Errorf("invalid project %q: must be %d characters or fewer", project, maxProjectNameLength)
	}
	if !projectNamePattern.MatchString(project) {
		return fmt.Errorf("invalid project %q: must contain only lowercase letters, numbers, hyphens, or underscores", project)
	}
	return nil
}
