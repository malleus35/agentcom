package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func TestValidateAgentToolsSelection(t *testing.T) {
	tests := []struct {
		name    string
		value   []string
		wantErr bool
	}{
		{name: "empty selection", wantErr: true},
		{name: "single selection", value: []string{"codex"}},
		{name: "multiple selections", value: []string{"claude", "codex"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentToolsSelection(tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("validateAgentToolsSelection() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateAgentToolsSelection() error = %v", err)
			}
			if tt.wantErr && err != nil && err.Error() != agentToolsError {
				t.Fatalf("validateAgentToolsSelection() error = %q, want %q", err.Error(), agentToolsError)
			}
		})
	}
}

func TestAgentToolsDescriptionIncludesSelectionGuidance(t *testing.T) {
	if !strings.Contains(agentToolsDescription, "Space to select") {
		t.Fatalf("agentToolsDescription = %q, want Space guidance", agentToolsDescription)
	}
	if !strings.Contains(agentToolsDescription, "Enter to continue") {
		t.Fatalf("agentToolsDescription = %q, want Enter guidance", agentToolsDescription)
	}
}

func TestDefaultWizardProjectName(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "Demo-App")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if got := defaultWizardProjectName(projectDir, "configured-project"); got != "configured-project" {
		t.Fatalf("defaultWizardProjectName(configured) = %q, want configured-project", got)
	}
	if got := defaultWizardProjectName(projectDir, ""); got != "demo-app" {
		t.Fatalf("defaultWizardProjectName(folder) = %q, want demo-app", got)
	}

	invalidDir := filepath.Join(t.TempDir(), "Demo App")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(invalidDir) error = %v", err)
	}
	if got := defaultWizardProjectName(invalidDir, ""); got != "" {
		t.Fatalf("defaultWizardProjectName(invalidDir) = %q, want empty", got)
	}
}

func TestNormalizeWizardProjectName(t *testing.T) {
	if got := normalizeWizardProjectName("demo-app", ""); got != "demo-app" {
		t.Fatalf("normalizeWizardProjectName(default) = %q, want demo-app", got)
	}
	if got := normalizeWizardProjectName("demo-app", "custom-app"); got != "custom-app" {
		t.Fatalf("normalizeWizardProjectName(explicit) = %q, want custom-app", got)
	}
	if got := normalizeWizardProjectName("", ""); got != "" {
		t.Fatalf("normalizeWizardProjectName(empty) = %q, want empty", got)
	}
}

func TestValidateWizardProjectName(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "current-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectDir) error = %v", err)
	}

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
	if err := database.InsertAgent(context.Background(), &db.Agent{Name: "alpha", Type: "worker", Project: "existing-project", Status: "alive"}); err != nil {
		t.Fatalf("InsertAgent() error = %v", err)
	}

	if err := validateWizardProjectName(projectDir, homeDir, ""); err == nil {
		t.Fatal("validateWizardProjectName(empty) error = nil, want error")
	}
	if err := validateWizardProjectName(projectDir, homeDir, "Demo App"); err == nil {
		t.Fatal("validateWizardProjectName(invalid) error = nil, want error")
	}
	if err := validateWizardProjectName(projectDir, homeDir, "existing-project"); err == nil {
		t.Fatal("validateWizardProjectName(duplicate) error = nil, want error")
	}
	if err := validateWizardProjectName(projectDir, homeDir, "new-project"); err != nil {
		t.Fatalf("validateWizardProjectName(unique) error = %v", err)
	}

	configPath := filepath.Join(projectDir, config.ProjectConfigFileName)
	if err := os.WriteFile(configPath, []byte("{\n  \"project\": \"existing-project\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(configPath) error = %v", err)
	}
	if err := validateWizardProjectName(projectDir, homeDir, "existing-project"); err != nil {
		t.Fatalf("validateWizardProjectName(current project) error = %v", err)
	}
}

func TestSimplifiedWizardGeneratesValidTemplate(t *testing.T) {
	definition, err := buildSimplifiedCustomTemplateDefinition("test-team", []string{"frontend", "backend", "plan"}, nil)
	if err != nil {
		t.Fatalf("buildSimplifiedCustomTemplateDefinition() error = %v", err)
	}
	if definition.Name != "test-team" {
		t.Fatalf("definition.Name = %q, want test-team", definition.Name)
	}
	if len(definition.Roles) != 3 {
		t.Fatalf("len(definition.Roles) = %d, want 3", len(definition.Roles))
	}
	if definition.Roles[0].AgentType != "engineer-frontend" {
		t.Fatalf("frontend AgentType = %q, want engineer-frontend", definition.Roles[0].AgentType)
	}
	if err := validateCustomTemplateDefinition(templateDefinitionFromOnboard(*definition)); err != nil {
		t.Fatalf("validateCustomTemplateDefinition() error = %v", err)
	}
	if issues := validateCommunicationGraph(templateDefinitionFromOnboard(*definition).Roles); len(issues) != 0 {
		t.Fatalf("validateCommunicationGraph() issues = %v, want none", issues)
	}
}
