package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	t.Setenv("HOME", homeDir)

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
		{name: "user claude", scope: "user", agent: "claude", want: filepath.Join(homeDir, ".claude", "skills", "my-skill", "SKILL.md")},
		{name: "user codex", scope: "user", agent: "codex", want: filepath.Join(homeDir, ".agents", "skills", "my-skill", "SKILL.md")},
		{name: "user gemini", scope: "user", agent: "gemini", want: filepath.Join(homeDir, ".gemini", "skills", "my-skill", "SKILL.md")},
		{name: "user opencode", scope: "user", agent: "opencode", want: filepath.Join(homeDir, ".config", "opencode", "skills", "my-skill", "SKILL.md")},
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

func TestWriteSkillFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "skills", "my-skill", "SKILL.md")
	content := renderSkillContent("my-skill", defaultSkillDescription)

	if err := writeSkillFile(path, content); err != nil {
		t.Fatalf("writeSkillFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != content {
		t.Fatalf("file content = %q, want %q", string(data), content)
	}

	if err := writeSkillFile(path, content); err == nil {
		t.Fatal("writeSkillFile() second call error = nil, want error")
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
	cmd.SetArgs([]string{"demo-skill", "--agent", "claude", "--scope", "project"})

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

	content, err := os.ReadFile(filepath.Join(projectDir, ".claude", "skills", "demo-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "name: demo-skill") {
		t.Fatalf("content missing skill name: %s", string(content))
	}
}
