package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

			frontend := findTemplateRole(t, definition, "frontend")
			plan := findTemplateRole(t, definition, "plan")
			switch name {
			case "company":
				for _, target := range []string{"architect", "review", "plan"} {
					if !containsString(frontend.CommunicatesWith, target) {
						t.Fatalf("company frontend missing %q in communicates_with: %v", target, frontend.CommunicatesWith)
					}
				}
			case "oh-my-opencode":
				for _, target := range []string{"design", "backend", "plan"} {
					if !containsString(frontend.CommunicatesWith, target) {
						t.Fatalf("oh-my-opencode frontend missing %q in communicates_with: %v", target, frontend.CommunicatesWith)
					}
				}
				for _, target := range []string{"architect", "review"} {
					if containsString(frontend.CommunicatesWith, target) {
						t.Fatalf("oh-my-opencode frontend unexpectedly contains %q in communicates_with: %v", target, frontend.CommunicatesWith)
					}
				}
				for _, target := range []string{"architect", "frontend", "backend", "design", "review"} {
					if !containsString(plan.CommunicatesWith, target) {
						t.Fatalf("oh-my-opencode plan missing %q in communicates_with: %v", target, plan.CommunicatesWith)
					}
				}
			}
		})
	}

	if _, err := resolveTemplateDefinition("missing"); err == nil {
		t.Fatal("resolveTemplateDefinition() error = nil, want error")
	}
}

func findTemplateRole(t *testing.T, definition templateDefinition, roleName string) templateRole {
	t.Helper()
	for _, role := range definition.Roles {
		if role.Name == roleName {
			return role
		}
	}
	t.Fatalf("role %q not found in template %q", roleName, definition.Name)
	return templateRole{}
}

func TestComputeEscalationTargets(t *testing.T) {
	tests := []struct {
		name             string
		roleName         string
		communicatesWith []string
		want             []string
	}{
		{name: "architect excludes self keeps plan", roleName: "architect", communicatesWith: []string{"plan", "frontend", "backend", "design", "review"}, want: []string{"plan"}},
		{name: "plan excludes self keeps architect", roleName: "plan", communicatesWith: []string{"architect", "frontend", "backend", "design", "review"}, want: []string{"architect"}},
		{name: "frontend gets architect only", roleName: "frontend", communicatesWith: []string{"design", "backend", "review", "architect"}, want: []string{"architect"}},
		{name: "backend gets both preferred targets", roleName: "backend", communicatesWith: []string{"frontend", "architect", "review", "plan"}, want: []string{"plan", "architect"}},
		{name: "fallback contacts", roleName: "worker", communicatesWith: []string{"helper", "monitor"}, want: []string{"helper", "monitor"}},
		{name: "empty contacts", roleName: "solo", communicatesWith: []string{}, want: []string{}},
		{name: "only self in contacts", roleName: "recursive", communicatesWith: []string{"recursive"}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEscalationTargets(tt.roleName, tt.communicatesWith)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("computeEscalationTargets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderEscalationLine(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		want    string
	}{
		{name: "empty", targets: []string{}, want: ""},
		{name: "single", targets: []string{"plan"}, want: "- Escalate blockers to `plan` when requirements or system boundaries change.\n"},
		{name: "double", targets: []string{"plan", "architect"}, want: "- Escalate blockers to `plan` and `architect` when requirements or system boundaries change.\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderEscalationLine(tt.targets)
			if got != tt.want {
				t.Fatalf("renderEscalationLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateCommunicationGraph(t *testing.T) {
	tests := []struct {
		name       string
		roles      []templateRole
		wantErrors int
		wantWarns  int
	}{
		{
			name: "symmetric graph passes",
			roles: []templateRole{
				{Name: "a", CommunicatesWith: []string{"b"}},
				{Name: "b", CommunicatesWith: []string{"a"}},
			},
		},
		{
			name: "self reference detected",
			roles: []templateRole{
				{Name: "a", CommunicatesWith: []string{"a", "b"}},
				{Name: "b", CommunicatesWith: []string{"a"}},
			},
			wantErrors: 1,
		},
		{
			name: "asymmetric edge warned",
			roles: []templateRole{
				{Name: "a", CommunicatesWith: []string{"b"}},
				{Name: "b"},
			},
			wantWarns: 1,
		},
		{
			name: "unknown role reference",
			roles: []templateRole{
				{Name: "a", CommunicatesWith: []string{"missing"}},
			},
			wantErrors: 1,
		},
		{
			name: "isolated role warned",
			roles: []templateRole{
				{Name: "a", CommunicatesWith: []string{"b"}},
				{Name: "b", CommunicatesWith: []string{"a"}},
				{Name: "c"},
			},
			wantWarns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validateCommunicationGraph(tt.roles)
			gotErrors := 0
			gotWarns := 0
			for _, issue := range issues {
				switch issue.Severity {
				case "error":
					gotErrors++
				case "warning":
					gotWarns++
				}
			}
			if gotErrors != tt.wantErrors {
				t.Fatalf("error count = %d, want %d (issues=%v)", gotErrors, tt.wantErrors, issues)
			}
			if gotWarns != tt.wantWarns {
				t.Fatalf("warning count = %d, want %d (issues=%v)", gotWarns, tt.wantWarns, issues)
			}
			if hasGraphErrors(issues) != (tt.wantErrors > 0) {
				t.Fatalf("hasGraphErrors() = %v, want %v", hasGraphErrors(issues), tt.wantErrors > 0)
			}
		})
	}

	for _, definition := range builtInTemplateDefinitions() {
		t.Run("built-in "+definition.Name+" passes", func(t *testing.T) {
			issues := validateCommunicationGraph(definition.Roles)
			if len(issues) != 0 {
				t.Fatalf("validateCommunicationGraph(%s) issues = %v, want none", definition.Name, issues)
			}
		})
	}
}

func TestValidateCustomTemplateDefinitionRejectsGraphErrors(t *testing.T) {
	definition := templateDefinition{
		Name:        "custom-team",
		Description: "Custom team",
		Reference:   "local",
		CommonTitle: "Custom Team",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{
			{Name: "a", Description: "role a", AgentName: "a", AgentType: "worker", CommunicatesWith: []string{"missing"}},
			{Name: "b", Description: "role b", AgentName: "b", AgentType: "worker"},
		},
	}

	err := validateCustomTemplateDefinition(definition)
	if err == nil {
		t.Fatal("validateCustomTemplateDefinition() error = nil, want graph error")
	}
	if !strings.Contains(err.Error(), "communication graph") {
		t.Fatalf("validateCustomTemplateDefinition() error = %q, want communication graph message", err.Error())
	}
}

func TestValidateCustomTemplateDefinitionAllowsGraphWarnings(t *testing.T) {
	definition := templateDefinition{
		Name:        "custom-warn-team",
		Description: "Custom team",
		Reference:   "local",
		CommonTitle: "Custom Team",
		CommonBody:  "Coordinate through agentcom.",
		Roles: []templateRole{
			{Name: "a", Description: "role a", AgentName: "a", AgentType: "worker", CommunicatesWith: []string{"b"}},
			{Name: "b", Description: "role b", AgentName: "b", AgentType: "worker"},
		},
	}

	if err := validateCustomTemplateDefinition(definition); err != nil {
		t.Fatalf("validateCustomTemplateDefinition() error = %v, want nil for warnings", err)
	}
}

func TestRenderContactDetails(t *testing.T) {
	definition, err := resolveTemplateDefinition("company")
	if err != nil {
		t.Fatalf("resolveTemplateDefinition() error = %v", err)
	}

	var frontend templateRole
	for _, role := range definition.Roles {
		if role.Name == "frontend" {
			frontend = role
			break
		}
	}

	details := renderContactDetails(frontend, definition.Roles)
	if !strings.Contains(details, "- **design** (design): Design specialist for UX direction") {
		t.Fatalf("renderContactDetails() missing design details: %s", details)
	}
	if !strings.Contains(details, "- **plan** (plan): Planning specialist") {
		t.Fatalf("renderContactDetails() missing plan details: %s", details)
	}
	if strings.Contains(details, "architect") && !strings.Contains(details, "- **architect** (architect):") {
		t.Fatalf("renderContactDetails() contains unexpected architect formatting: %s", details)
	}
}

func TestRenderCollaborationProtocol(t *testing.T) {
	role := templateRole{
		Name:             "plan",
		AgentName:        "prometheus",
		CommunicatesWith: []string{"architect", "frontend", "backend"},
	}

	protocol := renderCollaborationProtocol(role)
	for _, section := range []string{"### Request", "### Response", "### Escalation", "### Report"} {
		if !strings.Contains(protocol, section) {
			t.Fatalf("renderCollaborationProtocol() missing %q: %s", section, protocol)
		}
	}
	if !strings.Contains(protocol, "agentcom task create \"Coordinate dependency\" --creator prometheus") {
		t.Fatalf("renderCollaborationProtocol() missing concrete request command: %s", protocol)
	}
	if !strings.Contains(protocol, "agentcom send --from prometheus architect") {
		t.Fatalf("renderCollaborationProtocol() missing concrete escalation command: %s", protocol)
	}
	if strings.Contains(protocol, "--from <sender>") {
		t.Fatalf("renderCollaborationProtocol() still contains sender placeholder: %s", protocol)
	}
}

func TestSharedSkillContentQuality(t *testing.T) {
	content := renderAgentcomSharedSkillContent()
	lines := strings.Split(content, "\n")
	if len(lines) < 60 {
		t.Fatalf("shared SKILL.md has %d lines, want at least 60", len(lines))
	}
	for _, section := range []string{"## Overview", "## Lifecycle", "## Message Format", "## Task Lifecycle", "## Decision Guide", "## Quick Reference"} {
		if !strings.Contains(content, section) {
			t.Fatalf("shared SKILL.md missing section: %s", section)
		}
	}
	for _, placeholder := range []string{"<sender>", "<target>", "<task-id>"} {
		if strings.Contains(content, placeholder) {
			t.Fatalf("shared SKILL.md contains placeholder: %s", placeholder)
		}
	}
}

func TestRoleSkillContentQuality(t *testing.T) {
	definitions := builtInTemplateDefinitions()
	for _, def := range definitions {
		for _, role := range def.Roles {
			role := role
			t.Run(def.Name+"/"+role.Name, func(t *testing.T) {
				content := renderRoleSkillContent(def, role, templateRoleSkillName(def.Name, role.Name), ".agentcom/templates/"+def.Name+"/COMMON.md")
				lines := strings.Split(content, "\n")
				if len(lines) < 50 {
					t.Fatalf("role %s/%s SKILL.md has %d lines, want at least 50", def.Name, role.Name, len(lines))
				}
				for _, section := range []string{"## Workflow", "## Examples", "## Anti-patterns", "## Communication", "## Handoff Protocol"} {
					if !strings.Contains(content, section) {
						t.Fatalf("role %s/%s SKILL.md missing section: %s", def.Name, role.Name, section)
					}
				}
				for _, placeholder := range []string{"<sender>", "<target>", "<task-id>"} {
					if strings.Contains(content, placeholder) {
						t.Fatalf("role %s/%s SKILL.md contains placeholder: %s", def.Name, role.Name, placeholder)
					}
				}
			})
		}
	}
}

func TestTemplateCommonContentQuality(t *testing.T) {
	definitions := builtInTemplateDefinitions()
	contents := make(map[string]string, len(definitions))
	for _, def := range definitions {
		content := renderTemplateCommonContent(def)
		contents[def.Name] = content
		lines := strings.Split(content, "\n")
		if len(lines) < 25 {
			t.Fatalf("COMMON.md for %s has %d lines, want at least 25", def.Name, len(lines))
		}
	}
	if contents["company"] == contents["oh-my-opencode"] {
		t.Fatal("COMMON.md content should differ by template")
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

	paths, err := writeTemplateScaffold(projectDir, "company", writeModeAppend)
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
	if !strings.Contains(string(commonData), "`agentcom up`") || !strings.Contains(string(commonData), "`agentcom down`") {
		t.Fatalf("common markdown missing managed lifecycle guidance: %s", string(commonData))
	}
	if !strings.Contains(string(commonData), "low-level manual runs of a single standalone role") {
		t.Fatalf("common markdown missing standalone register guidance: %s", string(commonData))
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
	if !strings.Contains(string(sharedSkillData), "Default template lifecycle") {
		t.Fatalf("shared skill missing default lifecycle guidance: %s", string(sharedSkillData))
	}
	if !strings.Contains(string(sharedSkillData), "low-level path for manually running one standalone agent session") {
		t.Fatalf("shared skill missing register guidance: %s", string(sharedSkillData))
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
	if strings.Contains(content, "<sender>") || strings.Contains(content, "<target>") {
		t.Fatalf("skill still contains placeholders: %s", content)
	}
	if !strings.Contains(content, "### Primary Contacts") {
		t.Fatalf("skill missing primary contacts section: %s", content)
	}
	if !strings.Contains(content, "### Coordination Commands") {
		t.Fatalf("skill missing coordination commands section: %s", content)
	}
	for _, section := range []string{"### Request", "### Response", "### Escalation", "### Report"} {
		if !strings.Contains(content, section) {
			t.Fatalf("skill missing protocol section %q: %s", section, content)
		}
	}
	if !strings.Contains(content, "- **design** (design):") {
		t.Fatalf("skill missing detailed contact entry: %s", content)
	}
	if !strings.Contains(content, "send --from frontend") {
		t.Fatalf("skill missing concrete sender name: %s", content)
	}
	if !strings.Contains(content, "Escalate blockers to `plan` and `architect`") {
		t.Fatalf("frontend skill missing expected escalation targets: %s", content)
	}

	architectSkillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-architect", "SKILL.md")
	architectSkillData, err := os.ReadFile(architectSkillPath)
	if err != nil {
		t.Fatalf("ReadFile(architect skill) error = %v", err)
	}
	architectContent := string(architectSkillData)
	if strings.Contains(architectContent, "Escalate blockers to `plan` and `architect`") {
		t.Fatal("architect skill has self-referential escalation")
	}
	if !strings.Contains(architectContent, "Escalate blockers to `plan`") {
		t.Fatalf("architect skill missing escalation to plan: %s", architectContent)
	}

	planSkillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-plan", "SKILL.md")
	planSkillData, err := os.ReadFile(planSkillPath)
	if err != nil {
		t.Fatalf("ReadFile(plan skill) error = %v", err)
	}
	planContent := string(planSkillData)
	if strings.Contains(planContent, "Escalate blockers to `plan`") {
		t.Fatal("plan skill has self-referential escalation")
	}
	if !strings.Contains(planContent, "Escalate blockers to `architect`") {
		t.Fatalf("plan skill missing escalation to architect: %s", planContent)
	}

	paths2, err := writeTemplateScaffold(projectDir, "company", writeModeAppend)
	if err != nil {
		t.Fatalf("writeTemplateScaffold() second call error = %v, want nil", err)
	}
	if len(paths2) != 30 {
		t.Fatalf("second call len(paths) = %d, want 30", len(paths2))
	}
	commonData2, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatalf("ReadFile(common) second read error = %v", err)
	}
	if string(commonData) != string(commonData2) {
		t.Fatal("COMMON.md changed on second scaffold write")
	}
	skillData2, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(skill) second read error = %v", err)
	}
	if !strings.Contains(string(skillData2), agentcomMarkerStart) {
		t.Fatal("skill file missing marker after second scaffold write")
	}
	if strings.Count(string(skillData2), agentcomMarkerStart) != 1 {
		t.Fatal("skill file has duplicate marker blocks")
	}
}

func TestTemplateScaffoldReInit(t *testing.T) {
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

	paths1, err := writeTemplateScaffold(projectDir, "company", writeModeAppend)
	if err != nil {
		t.Fatalf("first scaffold error = %v", err)
	}
	skillPath := filepath.Join(projectDir, ".agents", "skills", "agentcom", "company-frontend", "SKILL.md")
	data1, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(skill) error = %v", err)
	}
	commonPath := filepath.Join(projectDir, ".agentcom", "templates", "company", "COMMON.md")
	common1, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatalf("ReadFile(common) error = %v", err)
	}

	paths2, err := writeTemplateScaffold(projectDir, "company", writeModeAppend)
	if err != nil {
		t.Fatalf("second scaffold error = %v", err)
	}
	if len(paths1) != len(paths2) {
		t.Fatalf("path count mismatch: %d vs %d", len(paths1), len(paths2))
	}
	data2, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(skill) second read error = %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatal("skill file content changed on re-scaffold")
	}
	common2, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatalf("ReadFile(common) second read error = %v", err)
	}
	if string(common1) != string(common2) {
		t.Fatal("COMMON.md changed on re-scaffold")
	}
}

func TestTemplateExportCommandOutputsYAML(t *testing.T) {
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

	buf := &bytes.Buffer{}
	cmd := newAgentsCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", "export", "company"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	output := buf.String()
	for _, want := range []string{"name: company", "roles:", "common_title:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("export output missing %q: %s", want, output)
		}
	}
}

func TestTemplateExportImportRoundtrip(t *testing.T) {
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

	buf := &bytes.Buffer{}
	cmd := newAgentsCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", "export", "company"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	exportPath := filepath.Join(projectDir, "company.yaml")
	if err := os.WriteFile(exportPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	imported, err := loadTemplateDefinitionFromFile(exportPath)
	if err != nil {
		t.Fatalf("loadTemplateDefinitionFromFile() error = %v", err)
	}
	definition, err := resolveTemplateDefinition("company")
	if err != nil {
		t.Fatalf("resolveTemplateDefinition() error = %v", err)
	}

	if imported.Name != definition.Name {
		t.Fatalf("imported.Name = %q, want %q", imported.Name, definition.Name)
	}
	if !reflect.DeepEqual(imported.Roles, definition.Roles) {
		t.Fatalf("imported roles = %#v, want %#v", imported.Roles, definition.Roles)
	}
	if imported.CommonTitle != definition.CommonTitle {
		t.Fatalf("imported.CommonTitle = %q, want %q", imported.CommonTitle, definition.CommonTitle)
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

func TestAgentsTemplateCommandListIncludesCustomTemplates(t *testing.T) {
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
		Roles:       []templateRole{{Name: "planner", Description: "desc", AgentName: "planner", AgentType: "planner"}},
	}); err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newAgentsTemplateCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "custom-team") {
		t.Fatalf("output missing custom template: %s", buf.String())
	}
}

func TestAgentsTemplateCommandDeleteCustomTemplate(t *testing.T) {
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

	basePath, err := saveCustomTemplate(projectDir, templateDefinition{
		Name:        "custom-team",
		Description: "Custom team template",
		Reference:   "local",
		CommonTitle: "Custom Team Common Instructions",
		CommonBody:  "Coordinate through agentcom.",
		Roles:       []templateRole{{Name: "planner", Description: "desc", AgentName: "planner", AgentType: "planner"}},
	})
	if err != nil {
		t.Fatalf("saveCustomTemplate() error = %v", err)
	}

	cmd := newAgentsTemplateCmd()
	cmd.SetIn(bytes.NewBufferString("y\n"))
	cmd.SetArgs([]string{"--delete", "custom-team"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		t.Fatalf("Stat(custom template path) error = %v, want not exist", err)
	}
}
