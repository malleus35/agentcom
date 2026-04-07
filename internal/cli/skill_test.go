package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type skillValidationResult struct {
	Path   string   `json:"path"`
	Status string   `json:"status"`
	Checks []string `json:"checks,omitempty"`
	Issues []string `json:"issues,omitempty"`
}

// PH11 T1.2.5 — schema_version + sections + parseable JSON examples.

func TestSkillSchemaVersionInRenderedContent(t *testing.T) {
	content := renderSkillContent("my-skill", "demo description")
	matched, version := extractSkillSchemaVersion(content)
	if !matched {
		t.Fatalf("rendered SKILL.md is missing schema_version frontmatter:\n%s", content)
	}
	if version != skillSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", version, skillSchemaVersion)
	}
}

func TestSkillSectionsInRenderedContent(t *testing.T) {
	content := renderSkillContent("my-skill", "demo description")
	required := []string{"## Communication", "## Workflow", "## Examples"}
	for _, section := range required {
		if !strings.Contains(content, section) {
			t.Errorf("rendered SKILL.md is missing section %q", section)
		}
	}
	if got := len(strings.Split(content, "\n")); got < 50 {
		t.Errorf("rendered SKILL.md has %d lines, want >= 50 (PRD E5 minimum)", got)
	}
	// Validator must accept the rendered content end-to-end (non-shared path).
	report := validateSkillFile("project/.claude/skills/my-skill/SKILL.md", content)
	if report.Status != "pass" {
		t.Errorf("validateSkillFile rejected freshly rendered SKILL.md: %+v", report)
	}
}

func TestSkillExamplesParse(t *testing.T) {
	content := renderSkillContent("my-skill", "demo description")
	// Extract every fenced ```json ... ``` block and ensure each parses.
	var blocks []string
	rest := content
	for {
		start := strings.Index(rest, "```json")
		if start < 0 {
			break
		}
		rest = rest[start+len("```json"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			t.Fatalf("unterminated ```json block in rendered SKILL.md")
		}
		blocks = append(blocks, strings.TrimSpace(rest[:end]))
		rest = rest[end+3:]
	}
	if len(blocks) < 3 {
		t.Fatalf("expected at least 3 JSON example blocks, got %d", len(blocks))
	}
	for i, block := range blocks {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(block), &parsed); err != nil {
			t.Errorf("JSON example #%d does not parse: %v\nblock=%s", i+1, err, block)
		}
	}
}

func TestExtractSkillSchemaVersion(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantMatched bool
		wantVersion int
	}{
		{name: "missing", content: "---\nname: x\n---\n", wantMatched: false},
		{name: "v1", content: "---\nname: x\nschema_version: 1\n---\n", wantMatched: true, wantVersion: 1},
		{name: "v2", content: "---\nschema_version: 2\nname: x\n---\n", wantMatched: true, wantVersion: 2},
		{name: "no_frontmatter", content: "# Title\n", wantMatched: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, v := extractSkillSchemaVersion(tc.content)
			if matched != tc.wantMatched {
				t.Fatalf("matched = %v, want %v", matched, tc.wantMatched)
			}
			if matched && v != tc.wantVersion {
				t.Fatalf("version = %d, want %d", v, tc.wantVersion)
			}
		})
	}
}

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "my-skill"},
		{name: "valid numeric", input: "skill-2"},
		{name: "uppercase", input: "My-skill", wantErr: true},
		{name: "underscore", input: "my_skill", wantErr: true},
		{name: "double hyphen", input: "my--skill", wantErr: true},
		{name: "leading hyphen", input: "-skill", wantErr: true},
		{name: "trailing hyphen", input: "skill-", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkillName(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("validateSkillName() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateSkillName() error = %v", err)
			}
		})
	}
}

func TestSkillTargetPath(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
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
	resolvedProjectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after chdir error = %v", err)
	}
	// os.UserHomeDir() reads HOME on unix but USERPROFILE on windows, so
	// override both to keep this test hermetic on every runner.
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	tests := []struct {
		name  string
		scope string
		agent string
		want  string
	}{
		{name: "project claude", scope: "project", agent: "claude", want: filepath.Join(resolvedProjectDir, ".claude", "skills", "my-skill", "SKILL.md")},
		{name: "project codex", scope: "project", agent: "codex", want: filepath.Join(resolvedProjectDir, ".agents", "skills", "my-skill", "SKILL.md")},
		{name: "project gemini", scope: "project", agent: "gemini", want: filepath.Join(resolvedProjectDir, ".gemini", "skills", "my-skill", "SKILL.md")},
		{name: "project opencode", scope: "project", agent: "opencode", want: filepath.Join(resolvedProjectDir, ".opencode", "skills", "my-skill", "SKILL.md")},
		{name: "project claude alias", scope: "project", agent: "claude-code", want: filepath.Join(resolvedProjectDir, ".claude", "skills", "my-skill", "SKILL.md")},
		{name: "project cursor", scope: "project", agent: "cursor", want: filepath.Join(resolvedProjectDir, ".cursor", "skills", "my-skill.mdc")},
		{name: "project github copilot", scope: "project", agent: "github-copilot", want: filepath.Join(resolvedProjectDir, ".github", "skills", "my-skill.md")},
		{name: "project universal", scope: "project", agent: "universal", want: filepath.Join(resolvedProjectDir, "skills", "my-skill", "SKILL.md")},
		{name: "user claude", scope: "user", agent: "claude", want: filepath.Join(homeDir, ".claude", "skills", "my-skill", "SKILL.md")},
		{name: "user codex", scope: "user", agent: "codex", want: filepath.Join(homeDir, ".agents", "skills", "my-skill", "SKILL.md")},
		{name: "user gemini", scope: "user", agent: "gemini", want: filepath.Join(homeDir, ".gemini", "skills", "my-skill", "SKILL.md")},
		{name: "user opencode", scope: "user", agent: "opencode", want: filepath.Join(homeDir, ".config", "opencode", "skills", "my-skill", "SKILL.md")},
		{name: "user devin", scope: "user", agent: "devin", want: filepath.Join(homeDir, ".devin", "skills", "my-skill.md")},
		{name: "user factory alias", scope: "user", agent: "droid", want: filepath.Join(homeDir, ".factory", "skills", "my-skill", "SKILL.md")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := skillTargetPath(tt.scope, tt.agent, "my-skill")
			if err != nil {
				t.Fatalf("skillTargetPath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("skillTargetPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSkillAgents(t *testing.T) {
	t.Run("all", func(t *testing.T) {
		agents, err := resolveSkillAgents("all")
		if err != nil {
			t.Fatalf("resolveSkillAgents(all) error = %v", err)
		}
		if len(agents) != len(skillAgentDefinitions) {
			t.Fatalf("len(agents) = %d, want %d", len(agents), len(skillAgentDefinitions))
		}
	})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "claude alias", input: "claude-code", want: "claude"},
		{name: "gemini alias", input: "gemini-cli", want: "gemini"},
		{name: "droid alias", input: "droid", want: "factory"},
		{name: "regular", input: "cursor", want: "cursor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents, err := resolveSkillAgents(tt.input)
			if err != nil {
				t.Fatalf("resolveSkillAgents(%q) error = %v", tt.input, err)
			}
			if len(agents) != 1 || agents[0] != tt.want {
				t.Fatalf("resolveSkillAgents(%q) = %v, want [%s]", tt.input, agents, tt.want)
			}
		})
	}

	if _, err := resolveSkillAgents("missing-agent"); err == nil {
		t.Fatal("resolveSkillAgents(missing-agent) error = nil, want error")
	}
}

func TestResolveTemplateSkillTargetsSelectedAgents(t *testing.T) {
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

	targets, err := resolveTemplateSkillTargetsForSelectedAgents("project", "agentcom", []string{"claude", "opencode"})
	if err != nil {
		t.Fatalf("resolveTemplateSkillTargets() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
	if targets[0].Agent != "claude" || targets[1].Agent != "opencode" {
		t.Fatalf("targets = %#v, want claude/opencode", targets)
	}
}

func TestWriteSkillFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "skills", "my-skill", "SKILL.md")
	content := renderSkillContent("my-skill", defaultSkillDescription)

	if err := writeSkillFile(path, content, writeModeCreate); err != nil {
		t.Fatalf("writeSkillFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := wrapWithMarkers(content)
	if string(data) != want {
		t.Fatalf("file content = %q, want %q", string(data), want)
	}

	if err := writeSkillFile(path, content, writeModeCreate); err == nil {
		t.Fatal("writeSkillFile() second call error = nil, want error")
	}
}

func TestWriteSkillFileCreateModeRejectsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test", "SKILL.md")
	if err := writeSkillFile(path, "content", writeModeCreate); err != nil {
		t.Fatalf("first write error = %v", err)
	}
	if err := writeSkillFile(path, "content", writeModeCreate); err == nil {
		t.Fatal("second write error = nil, want error")
	}
}

func TestWriteSkillFileAppendMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("# My Custom Skill\n\nCustom instructions.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := writeSkillFile(path, "generated content", writeModeAppend); err != nil {
		t.Fatalf("append write error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "My Custom Skill") {
		t.Fatal("user content lost")
	}
	if !strings.Contains(content, agentcomMarkerStart) {
		t.Fatal("marker not added")
	}
}

func TestSkillCreateCommandOutputsJSON(t *testing.T) {
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

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newSkillCreateCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"demo-skill", "--agent", "cursor", "--scope", "project"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}

	if got["name"] != "demo-skill" {
		t.Fatalf("name = %v, want demo-skill", got["name"])
	}
	if got["scope"] != "project" {
		t.Fatalf("scope = %v, want project", got["scope"])
	}
	if got["description"] != defaultSkillDescription {
		t.Fatalf("description = %v, want %q", got["description"], defaultSkillDescription)
	}

	content, err := os.ReadFile(filepath.Join(projectDir, ".cursor", "skills", "demo-skill.mdc"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "name: demo-skill") {
		t.Fatalf("content missing skill name: %s", string(content))
	}
}

func TestSkillValidatePassesAfterPhase3(t *testing.T) {
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
	if _, err := writeTemplateScaffold(projectDir, "company", writeModeAppend, nil); err != nil {
		t.Fatalf("writeTemplateScaffold() error = %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newSkillCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	var results []skillValidationResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if len(results) == 0 {
		t.Fatal("skill validate returned no results")
	}
	for _, result := range results {
		if result.Status != "pass" {
			t.Fatalf("validation result = %#v, want pass", result)
		}
	}
}

func TestSkillValidateDetectsLowQuality(t *testing.T) {
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

	badPath := filepath.Join(projectDir, ".agents", "skills", "broken", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(badPath, []byte("# Broken\n\n<sender>\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newSkillCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v output=%s", err, buf.String())
	}

	var results []skillValidationResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1 (%#v)", len(results), results)
	}
	if results[0].Status != "fail" {
		t.Fatalf("validation result = %#v, want fail", results[0])
	}
	if len(results[0].Issues) == 0 {
		t.Fatalf("validation result = %#v, want issues", results[0])
	}
}
