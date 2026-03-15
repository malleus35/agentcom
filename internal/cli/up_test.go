package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
)

func TestEnsureInitProjectConfigSetsActiveTemplate(t *testing.T) {
	projectDir := t.TempDir()

	oldProjectFlag := projectFlag
	projectFlag = ""
	defer func() { projectFlag = oldProjectFlag }()

	path, cfg, err := ensureInitProjectConfig(projectDir, false, "company")
	if err != nil {
		t.Fatalf("ensureInitProjectConfig() error = %v", err)
	}
	if path != filepath.Join(projectDir, config.ProjectConfigFileName) {
		t.Fatalf("path = %q, want %q", path, filepath.Join(projectDir, config.ProjectConfigFileName))
	}
	if cfg.Template.Active != "company" {
		t.Fatalf("Template.Active = %q, want company", cfg.Template.Active)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) == "" {
		t.Fatal("config file is empty")
	}
}

func TestFilterTemplateRolesByOnlySelection(t *testing.T) {
	roles := []templateManifestRole{{Name: "frontend"}, {Name: "backend"}, {Name: "plan"}}

	filtered, err := filterTemplateRoles(roles, "plan,frontend")
	if err != nil {
		t.Fatalf("filterTemplateRoles() error = %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
	if filtered[0].Name != "plan" || filtered[1].Name != "frontend" {
		t.Fatalf("filtered roles = %#v, want [plan frontend]", filtered)
	}
}
