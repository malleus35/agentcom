package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindInstructionDefinition(t *testing.T) {
	tests := []struct {
		name          string
		agent         string
		wantID        string
		wantPath      string
		wantFormat    string
		wantHasMemory bool
	}{
		{name: "claude", agent: "claude", wantID: "claude", wantPath: "CLAUDE.md", wantFormat: "markdown"},
		{name: "claude alias", agent: "claude-code", wantID: "claude", wantPath: "CLAUDE.md", wantFormat: "markdown"},
		{name: "codex", agent: "codex", wantID: "codex", wantPath: "AGENTS.md", wantFormat: "markdown", wantHasMemory: true},
		{name: "gemini alias", agent: "gemini-cli", wantID: "gemini", wantPath: "GEMINI.md", wantFormat: "markdown"},
		{name: "cursor", agent: "cursor", wantID: "cursor", wantPath: filepath.Join(".cursor", "rules", "agentcom.mdc"), wantFormat: "mdc"},
		{name: "github copilot", agent: "github-copilot", wantID: "github-copilot", wantPath: filepath.Join(".github", "copilot-instructions.md"), wantFormat: "markdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition, ok := findInstructionDefinition(tt.agent)
			if !ok {
				t.Fatalf("findInstructionDefinition(%q) = false, want true", tt.agent)
			}
			if definition.AgentID != tt.wantID {
				t.Fatalf("definition.AgentID = %q, want %q", definition.AgentID, tt.wantID)
			}
			if definition.RelativePath != tt.wantPath {
				t.Fatalf("definition.RelativePath = %q, want %q", definition.RelativePath, tt.wantPath)
			}
			if definition.Format != tt.wantFormat {
				t.Fatalf("definition.Format = %q, want %q", definition.Format, tt.wantFormat)
			}
			if definition.SupportsMemory != tt.wantHasMemory {
				t.Fatalf("definition.SupportsMemory = %v, want %v", definition.SupportsMemory, tt.wantHasMemory)
			}
		})
	}

	if _, ok := findInstructionDefinition("missing"); ok {
		t.Fatal("findInstructionDefinition(missing) = true, want false")
	}
}

func TestWriteAgentInstructions(t *testing.T) {
	projectDir := t.TempDir()

	paths, err := writeAgentInstructions(projectDir, []string{"claude", "codex", "cursor"})
	if err != nil {
		t.Fatalf("writeAgentInstructions() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("len(paths) = %d, want 3", len(paths))
	}

	claudePath := filepath.Join(projectDir, "CLAUDE.md")
	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) error = %v", err)
	}
	claudeContent := string(claudeData)
	if !strings.Contains(claudeContent, "# CLAUDE.md") {
		t.Fatalf("CLAUDE.md missing title: %s", claudeContent)
	}
	if !strings.Contains(claudeContent, "agentcom Workflow") {
		t.Fatalf("CLAUDE.md missing workflow: %s", claudeContent)
	}

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}
	agentsContent := string(agentsData)
	if !strings.Contains(agentsContent, "# AGENTS.md") {
		t.Fatalf("AGENTS.md missing title: %s", agentsContent)
	}
	if !strings.Contains(agentsContent, "Register each long-running agent session") {
		t.Fatalf("AGENTS.md missing expected body: %s", agentsContent)
	}

	cursorPath := filepath.Join(projectDir, ".cursor", "rules", "agentcom.mdc")
	cursorData, err := os.ReadFile(cursorPath)
	if err != nil {
		t.Fatalf("ReadFile(cursor rule) error = %v", err)
	}
	cursorContent := string(cursorData)
	if !strings.Contains(cursorContent, "alwaysApply: true") {
		t.Fatalf("cursor rule missing alwaysApply: %s", cursorContent)
	}
	if !strings.Contains(cursorContent, "agentcom Workflow") {
		t.Fatalf("cursor rule missing workflow: %s", cursorContent)
	}

	if _, err := writeAgentInstructions(projectDir, []string{"codex"}); err == nil {
		t.Fatal("writeAgentInstructions() second call error = nil, want error")
	}
}

func TestWriteAgentMemoryFiles(t *testing.T) {
	projectDir := t.TempDir()

	paths, err := writeAgentMemoryFiles(projectDir, []string{"claude", "codex", "cursor"})
	if err != nil {
		t.Fatalf("writeAgentMemoryFiles() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}

	memoryPath := filepath.Join(projectDir, ".agents", "MEMORY.md")
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# MEMORY.md") {
		t.Fatalf("memory content missing title: %s", content)
	}
	if !strings.Contains(content, "decision") {
		t.Fatalf("memory content missing expected sections: %s", content)
	}

	if _, err := writeAgentMemoryFiles(projectDir, []string{"codex"}); err == nil {
		t.Fatal("writeAgentMemoryFiles() second call error = nil, want error")
	}
}
