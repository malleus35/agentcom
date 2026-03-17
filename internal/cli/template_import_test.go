package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
)

func TestLoadTemplateDefinitionFromFileMinimalYAML(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "template.yaml")
	content := []byte("name: my-team\nroles: [frontend, backend, plan, review]\n")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	definition, err := loadTemplateDefinitionFromFile(filePath)
	if err != nil {
		t.Fatalf("loadTemplateDefinitionFromFile() error = %v", err)
	}
	if definition.Name != "my-team" {
		t.Fatalf("definition.Name = %q, want my-team", definition.Name)
	}
	if len(definition.Roles) != 4 {
		t.Fatalf("len(definition.Roles) = %d, want 4", len(definition.Roles))
	}
	if definition.Roles[0].AgentName == "" || definition.Roles[0].AgentType == "" {
		t.Fatalf("minimal import should generate role defaults: %+v", definition.Roles[0])
	}
}

func TestLoadTemplateDefinitionFromFileWithReviewPolicy(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "template.yaml")
	content := []byte("name: my-team\nreview_policy:\n  require_review_above: high\n  default_reviewer: user\nroles: [frontend, backend]\n")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	definition, err := loadTemplateDefinitionFromFile(filePath)
	if err != nil {
		t.Fatalf("loadTemplateDefinitionFromFile() error = %v", err)
	}
	if definition.ReviewPolicy == nil {
		t.Fatal("ReviewPolicy = nil, want parsed policy")
	}
	if definition.ReviewPolicy.DefaultReviewer != "user" {
		t.Fatalf("DefaultReviewer = %q, want user", definition.ReviewPolicy.DefaultReviewer)
	}
}

func TestInitCommandFromFileCreatesCustomTemplate(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), ".agentcom-home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
	}
	filePath := filepath.Join(projectDir, "template.yaml")
	content := []byte("name: my-team\nroles: [frontend, backend, plan, review]\n")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	oldApp := app
	app = &appContext{cfg: &config.Config{HomeDir: homeDir}}
	defer func() { app = oldApp }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch", "--from-file", filePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if payload["template"] != "my-team" {
		t.Fatalf("payload template = %v, want my-team", payload["template"])
	}
	if _, ok := payload["custom_template_path"]; !ok {
		t.Fatalf("payload missing custom_template_path: %s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom", "templates", "my-team", "template.json")); err != nil {
		t.Fatalf("Stat(template.json) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agents", "skills", "agentcom", "my-team-frontend", "SKILL.md")); err != nil {
		t.Fatalf("Stat(frontend skill) error = %v", err)
	}
}

func TestInitCommandFromFileCreatesExactFullTemplate(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), ".agentcom-home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
	}
	filePath := filepath.Join(projectDir, "template.yaml")
	content := []byte("name: my-team\ndescription: Custom team for project X\nreference: internal\nroles:\n  - name: frontend\n    description: Custom frontend description\n    agent_name: fe-agent\n    agent_type: engineer\n    responsibilities:\n      - Build UI components\n    communicates_with: [backend]\n  - name: backend\n    description: Custom backend description\n    agent_name: be-agent\n    agent_type: engineer\n    responsibilities:\n      - Build APIs\n    communicates_with: [frontend]\n")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	oldApp := app
	app = &appContext{cfg: &config.Config{HomeDir: homeDir}}
	defer func() { app = oldApp }()

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--batch", "--from-file", filePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	definition, _, err := loadCustomTemplate(projectDir, "my-team")
	if err != nil {
		t.Fatalf("loadCustomTemplate() error = %v", err)
	}
	if definition.Description != "Custom team for project X" {
		t.Fatalf("definition.Description = %q, want exact imported description", definition.Description)
	}
	if definition.Reference != "internal" {
		t.Fatalf("definition.Reference = %q, want internal", definition.Reference)
	}
	if definition.Roles[0].AgentName != "fe-agent" {
		t.Fatalf("definition.Roles[0].AgentName = %q, want fe-agent", definition.Roles[0].AgentName)
	}
	if len(definition.Roles[0].Responsibilities) != 1 || definition.Roles[0].Responsibilities[0] != "Build UI components" {
		t.Fatalf("definition.Roles[0].Responsibilities = %v, want imported responsibilities", definition.Roles[0].Responsibilities)
	}
}

func TestLoadTemplateDefinitionFromFileInvalidYAML(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "template.yaml")
	if err := os.WriteFile(filePath, []byte("name: broken\nroles: [frontend\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := loadTemplateDefinitionFromFile(filePath); err == nil {
		t.Fatal("loadTemplateDefinitionFromFile() error = nil, want parse error")
	}
}

func TestInitCommandFromFileMissingFile(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), ".agentcom-home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
	}
	oldApp := app
	app = &appContext{cfg: &config.Config{HomeDir: homeDir}}
	defer func() { app = oldApp }()

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--batch", "--from-file", filepath.Join(t.TempDir(), "missing.yaml")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("cmd.Execute() error = nil, want missing file error")
	}
}
