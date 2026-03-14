package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/spf13/cobra"
)

func TestResolveTemplateDefinition(t *testing.T) {
	templates := []string{"company", "oh-my-opencode"}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			definition, err := resolveTemplateDefinition(name)
			if err != nil {
				t.Fatalf("resolveTemplateDefinition() error = %v", err)
			}
			if definition.Name != name {
				t.Fatalf("definition.Name = %q, want %q", definition.Name, name)
			}
			if len(definition.Roles) != 6 {
				t.Fatalf("len(definition.Roles) = %d, want 6", len(definition.Roles))
			}
		})
	}

	if _, err := resolveTemplateDefinition("missing"); err == nil {
		t.Fatal("resolveTemplateDefinition() error = nil, want error")
	}
}

func TestWriteTemplateScaffold(t *testing.T) {
	projectDir := t.TempDir()
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

	paths, err := writeTemplateScaffold(projectDir, "company")
	if err != nil {
		t.Fatalf("writeTemplateScaffold() error = %v", err)
	}
	if len(paths) != 30 {
		t.Fatalf("len(paths) = %d, want 30", len(paths))
	}

	commonPath := filepath.Join(projectDir, ".agentcom", "templates", "company", "COMMON.md")
	commonData, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatalf("ReadFile(common) error = %v", err)
	}
	if !strings.Contains(string(commonData), "frontend, backend, plan, review, architect, and design") {
		t.Fatalf("common markdown missing expected roles: %s", string(commonData))
	}

	manifestPath := filepath.Join(projectDir, ".agentcom", "templates", "company", "template.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	var manifest templateDefinition
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(manifest) error = %v", err)
	}
	if manifest.Name != "company" {
		t.Fatalf("manifest.Name = %q, want company", manifest.Name)
	}

	sharedSkillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "SKILL.md")
	sharedSkillData, err := os.ReadFile(sharedSkillPath)
	if err != nil {
		t.Fatalf("ReadFile(shared skill) error = %v", err)
	}
	if !strings.Contains(string(sharedSkillData), "Shared agentcom skill instructions") {
		t.Fatalf("shared skill missing expected content: %s", string(sharedSkillData))
	}

	skillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-frontend", "SKILL.md")
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(skill) error = %v", err)
	}
	content := string(skillData)
	if !strings.Contains(content, "Read shared agentcom instructions first: `../SKILL.md`") {
		t.Fatalf("skill missing shared skill reference: %s", content)
	}
	if !strings.Contains(content, "Read common instructions first: `.agentcom/templates/company/COMMON.md`") {
		t.Fatalf("skill missing common path: %s", content)
	}
	if !strings.Contains(content, "Primary contacts: design, backend, review, architect") {
		t.Fatalf("skill missing communication map: %s", content)
	}

	if _, err := writeTemplateScaffold(projectDir, "company"); err == nil {
		t.Fatal("writeTemplateScaffold() second call error = nil, want error")
	}
}

func TestAgentsTemplateCommandOutputsJSON(t *testing.T) {
	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newAgentsTemplateCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"oh-my-opencode"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var got templateDefinition
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if got.Name != "oh-my-opencode" {
		t.Fatalf("got.Name = %q, want oh-my-opencode", got.Name)
	}
	if len(got.Roles) != 6 {
		t.Fatalf("len(got.Roles) = %d, want 6", len(got.Roles))
	}
}

func TestAgentsTemplateCommandInteractiveSelection(t *testing.T) {
	oldSelector := templateSelectionEnabled
	templateSelectionEnabled = func(cmd *cobra.Command) bool { return true }
	defer func() { templateSelectionEnabled = oldSelector }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	input := bytes.NewBufferString("open\n1\n")
	output := &bytes.Buffer{}
	cmd := newAgentsTemplateCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "Search templates") {
		t.Fatalf("interactive output missing search prompt: %s", text)
	}
	if !strings.Contains(text, "oh-my-opencode") {
		t.Fatalf("interactive output missing selected template details: %s", text)
	}
	if !strings.Contains(text, "reference: oh-my-opencode") {
		t.Fatalf("interactive output missing template reference: %s", text)
	}
	if strings.Contains(text, "company-style") {
		t.Fatalf("interactive output should filter non-matching template: %s", text)
	}
}

func TestInitCommandGeneratesTemplateScaffold(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), ".agentcom-home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
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
	cmd.SetArgs([]string{"--template", "company"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if got["template"] != "company" {
		t.Fatalf("template = %v, want company", got["template"])
	}

	generatedFiles, ok := got["generated_files"].([]any)
	if !ok || len(generatedFiles) == 0 {
		t.Fatalf("generated_files = %#v, want non-empty array", got["generated_files"])
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".opencode", "skills", "agentcom", "company-plan", "SKILL.md")); err != nil {
		t.Fatalf("Stat(plan skill) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".opencode", "skills", "agentcom", "SKILL.md")); err != nil {
		t.Fatalf("Stat(shared skill) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom", "templates", "company", "template.json")); err != nil {
		t.Fatalf("Stat(template.json) error = %v", err)
	}
}
