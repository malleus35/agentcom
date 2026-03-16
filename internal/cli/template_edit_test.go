package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateAddRole(t *testing.T) {
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

	if _, err := saveCustomTemplate(projectDir, templateDefinition{
		Name:        "custom-team",
		Description: "Custom team template",
		Reference:   "local",
		CommonTitle: "Custom Team Common Instructions",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{
			{Name: "frontend", Description: "desc", AgentName: "frontend", AgentType: "engineer", CommunicatesWith: []string{"backend"}, Responsibilities: []string{"ship ui"}},
			{Name: "backend", Description: "desc", AgentName: "backend", AgentType: "engineer", CommunicatesWith: []string{"frontend"}, Responsibilities: []string{"ship api"}},
		},
	}); err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}

	cmd := newTemplateEditCmd()
	cmd.SetArgs([]string{"custom-team", "add-role", "devops"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	definition, _, err := loadCustomTemplate(projectDir, "custom-team")
	if err != nil {
		t.Fatalf("loadCustomTemplate() error = %v", err)
	}
	if len(definition.Roles) != 3 {
		t.Fatalf("len(definition.Roles) = %d, want 3", len(definition.Roles))
	}
	if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
		t.Fatalf("validateCommunicationGraph() issues = %v, want no errors", issues)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agents", "skills", "agentcom", "custom-team-devops", "SKILL.md")); err != nil {
		t.Fatalf("Stat(devops skill) error = %v", err)
	}
}

func TestTemplateRemoveRole(t *testing.T) {
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

	definition := templateDefinition{
		Name:        "custom-team",
		Description: "Custom team template",
		Reference:   "local",
		CommonTitle: "Custom Team Common Instructions",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{
			{Name: "frontend", Description: "desc", AgentName: "frontend", AgentType: "engineer", CommunicatesWith: []string{"backend", "design"}, Responsibilities: []string{"ship ui"}},
			{Name: "backend", Description: "desc", AgentName: "backend", AgentType: "engineer", CommunicatesWith: []string{"frontend", "design"}, Responsibilities: []string{"ship api"}},
			{Name: "design", Description: "desc", AgentName: "design", AgentType: "designer", CommunicatesWith: []string{"frontend", "backend"}, Responsibilities: []string{"ship ux"}},
		},
	}
	if _, err := saveCustomTemplate(projectDir, definition); err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}
	if _, err := writeTemplateScaffold(projectDir, "custom-team", writeModeAppend); err != nil {
		t.Fatalf("writeTemplateScaffold() error = %v", err)
	}

	cmd := newTemplateEditCmd()
	cmd.SetArgs([]string{"custom-team", "remove-role", "design"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	updated, _, err := loadCustomTemplate(projectDir, "custom-team")
	if err != nil {
		t.Fatalf("loadCustomTemplate() error = %v", err)
	}
	if len(updated.Roles) != 2 {
		t.Fatalf("len(updated.Roles) = %d, want 2", len(updated.Roles))
	}
	for _, role := range updated.Roles {
		if containsString(role.CommunicatesWith, "design") {
			t.Fatalf("role %s still references removed design role", role.Name)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agents", "skills", "agentcom", "custom-team-design", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("Stat(removed skill) error = %v, want not exist", err)
	}
}

func TestTemplateEditBuiltInRejected(t *testing.T) {
	cmd := newTemplateEditCmd()
	cmd.SetArgs([]string{"company", "add-role", "devops"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("cmd.Execute() error = nil, want built-in rejection")
	}
}

func TestTemplateEditGraphStaysValid(t *testing.T) {
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

	if _, err := saveCustomTemplate(projectDir, templateDefinition{
		Name:        "custom-team",
		Description: "Custom team template",
		Reference:   "local",
		CommonTitle: "Custom Team Common Instructions",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{
			{Name: "plan", Description: "desc", AgentName: "plan", AgentType: "planner", CommunicatesWith: []string{"review"}, Responsibilities: []string{"plan work"}},
			{Name: "review", Description: "desc", AgentName: "review", AgentType: "reviewer", CommunicatesWith: []string{"plan"}, Responsibilities: []string{"review work"}},
		},
	}); err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}

	if _, err := addRoleToTemplate("custom-team", "security"); err != nil {
		t.Fatalf("addRoleToTemplate() error = %v", err)
	}
	definition, _, err := loadCustomTemplate(projectDir, "custom-team")
	if err != nil {
		t.Fatalf("loadCustomTemplate() error = %v", err)
	}
	if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
		t.Fatalf("validateCommunicationGraph() issues = %v, want no errors", issues)
	}

	if _, err := removeRoleFromTemplate("custom-team", "security"); err != nil {
		t.Fatalf("removeRoleFromTemplate() error = %v", err)
	}
	definition, _, err = loadCustomTemplate(projectDir, "custom-team")
	if err != nil {
		t.Fatalf("loadCustomTemplate() second error = %v", err)
	}
	if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
		t.Fatalf("validateCommunicationGraph() after remove issues = %v, want no errors", issues)
	}
}
