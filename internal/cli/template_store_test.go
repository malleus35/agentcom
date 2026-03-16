package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadCustomTemplates(t *testing.T) {
	projectDir := t.TempDir()
	definition := templateDefinition{
		Name:        "custom-team",
		Description: "Custom delivery template",
		Reference:   "local",
		CommonTitle: "Custom Team Common Instructions",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{{
			Name:             "planner",
			Description:      "Planning role",
			AgentName:        "planner",
			AgentType:        "planner",
			CommunicatesWith: []string{"review"},
			Responsibilities: []string{"Break work down."},
		}, {
			Name:             "review",
			Description:      "Review role",
			AgentName:        "review",
			AgentType:        "reviewer",
			CommunicatesWith: []string{"planner"},
			Responsibilities: []string{"Review delivered work."},
		}},
	}

	basePath, err := saveCustomTemplate(projectDir, definition)
	if err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(basePath, "template.json")); err != nil {
		t.Fatalf("Stat(template.json) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(basePath, "COMMON.md")); err != nil {
		t.Fatalf("Stat(COMMON.md) error = %v", err)
	}

	loaded, err := loadCustomTemplates(projectDir)
	if err != nil {
		t.Fatalf("loadCustomTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Name != definition.Name {
		t.Fatalf("loaded[0].Name = %q, want %q", loaded[0].Name, definition.Name)
	}
	if loaded[0].CommonBody != definition.CommonBody {
		t.Fatalf("loaded[0].CommonBody = %q, want %q", loaded[0].CommonBody, definition.CommonBody)
	}
}

func TestMergeTemplateDefinitions(t *testing.T) {
	builtIn := []templateDefinition{{Name: "company"}, {Name: "oh-my-opencode"}}
	custom := []templateDefinition{{Name: "custom-team"}}

	merged := mergeTemplateDefinitions(builtIn, custom)
	if len(merged) != 3 {
		t.Fatalf("len(merged) = %d, want 3", len(merged))
	}
	if merged[2].Name != "custom-team" {
		t.Fatalf("merged[2].Name = %q, want custom-team", merged[2].Name)
	}
}

func TestSaveCustomTemplateRejectsBuiltInName(t *testing.T) {
	projectDir := t.TempDir()
	_, err := saveCustomTemplate(projectDir, templateDefinition{Name: "company", Description: "desc", CommonTitle: "Title", CommonBody: "Body", Roles: []templateRole{{Name: "planner", Description: "desc", AgentName: "planner", AgentType: "planner"}}})
	if err == nil {
		t.Fatal("saveCustomTemplate() error = nil, want error")
	}
}
