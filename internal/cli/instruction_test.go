package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrapWithMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "basic", content: "hello", want: "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"},
		{name: "trailing newline", content: "hello\n", want: "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"},
		{name: "multiline", content: "line1\nline2", want: "<!-- AGENTCOM:START -->\nline1\nline2\n<!-- AGENTCOM:END -->\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapWithMarkers(tt.content)
			if got != tt.want {
				t.Fatalf("wrapWithMarkers() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindMarkerBounds(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantFound bool
		wantStart int
		wantEnd   int
	}{
		{name: "no markers", content: "plain text", wantFound: false},
		{name: "markers in middle", content: "before\n<!-- AGENTCOM:START -->\nmiddle\n<!-- AGENTCOM:END -->\nafter", wantFound: true, wantStart: 7, wantEnd: 60},
		{name: "markers at start", content: "<!-- AGENTCOM:START -->\ncontent\n<!-- AGENTCOM:END -->\n", wantFound: true, wantStart: 0, wantEnd: 54},
		{name: "only start marker", content: "<!-- AGENTCOM:START -->\ncontent", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, gotFound := findMarkerBounds(tt.content)
			if gotFound != tt.wantFound {
				t.Fatalf("findMarkerBounds() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotStart != tt.wantStart {
				t.Fatalf("findMarkerBounds() start = %d, want %d", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Fatalf("findMarkerBounds() end = %d, want %d", gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestReplaceMarkerBlock(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		newBlock string
		want     string
	}{
		{
			name:     "replace middle block",
			existing: "before\n<!-- AGENTCOM:START -->\nold\n<!-- AGENTCOM:END -->\nafter",
			newBlock: "<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\n",
			want:     "before\n<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\nafter",
		},
		{
			name:     "replace starting block",
			existing: "<!-- AGENTCOM:START -->\nold\n<!-- AGENTCOM:END -->\nafter",
			newBlock: "<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\n",
			want:     "<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\nafter",
		},
		{
			name:     "replace ending block",
			existing: "before\n<!-- AGENTCOM:START -->\nold\n<!-- AGENTCOM:END -->\n",
			newBlock: "<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\n",
			want:     "before\n<!-- AGENTCOM:START -->\nnew\n<!-- AGENTCOM:END -->\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceMarkerBlock(tt.existing, tt.newBlock)
			if got != tt.want {
				t.Fatalf("replaceMarkerBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendMarkerBlock(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		newBlock string
		want     string
	}{
		{name: "empty existing", existing: "", newBlock: "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n", want: "<!-- AGENTCOM:START -->\nhello\n<!-- AGENTCOM:END -->\n"},
		{name: "no trailing newline", existing: "hello", newBlock: "<!-- AGENTCOM:START -->\nworld\n<!-- AGENTCOM:END -->\n", want: "hello\n\n<!-- AGENTCOM:START -->\nworld\n<!-- AGENTCOM:END -->\n"},
		{name: "multiple trailing newlines", existing: "hello\n\n\n", newBlock: "<!-- AGENTCOM:START -->\nworld\n<!-- AGENTCOM:END -->\n", want: "hello\n\n<!-- AGENTCOM:START -->\nworld\n<!-- AGENTCOM:END -->\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendMarkerBlock(tt.existing, tt.newBlock)
			if got != tt.want {
				t.Fatalf("appendMarkerBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteInstructionFileWithMode(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		mode            writeMode
		newContent      string
		wantErr         bool
		wantContains    []string
		wantNotContain  []string
		runTwice        bool
	}{
		{
			name:         "create new file",
			mode:         writeModeAppend,
			newContent:   "# CLAUDE.md\n## Workflow\n- step 1",
			wantContains: []string{"AGENTCOM:START", "# CLAUDE.md", "AGENTCOM:END"},
		},
		{
			name:            "append to existing without markers",
			existingContent: "# My Project\n\nExisting instructions.\n",
			mode:            writeModeAppend,
			newContent:      "## agentcom Workflow\n- step 1",
			wantContains:    []string{"# My Project", "Existing instructions", "AGENTCOM:START", "agentcom Workflow"},
		},
		{
			name:            "update existing markers",
			existingContent: "# Header\n<!-- AGENTCOM:START -->\nold content\n<!-- AGENTCOM:END -->\n# Footer\n",
			mode:            writeModeAppend,
			newContent:      "new content",
			wantContains:    []string{"# Header", "new content", "# Footer"},
			wantNotContain:  []string{"old content"},
		},
		{
			name:            "force overwrite",
			existingContent: "# My Custom Content\n\nDo not lose this.\n",
			mode:            writeModeOverwrite,
			newContent:      "## agentcom only",
			wantContains:    []string{"AGENTCOM:START", "agentcom only"},
			wantNotContain:  []string{"My Custom Content"},
		},
		{
			name:            "create mode rejects existing",
			existingContent: "something",
			mode:            writeModeCreate,
			newContent:      "new content",
			wantErr:         true,
		},
		{
			name:         "idempotent double append",
			mode:         writeModeAppend,
			newContent:   "## agentcom only",
			wantContains: []string{"AGENTCOM:START", "agentcom only"},
			runTwice:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "nested", "instruction.md")
			if tt.existingContent != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll() error = %v", err)
				}
				if err := os.WriteFile(path, []byte(tt.existingContent), 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}

			if err := writeInstructionFile(path, tt.newContent, tt.mode); (err != nil) != tt.wantErr {
				t.Fatalf("writeInstructionFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			content := string(data)
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Fatalf("content missing %q: %s", want, content)
				}
			}
			for _, unwanted := range tt.wantNotContain {
				if strings.Contains(content, unwanted) {
					t.Fatalf("content contains %q: %s", unwanted, content)
				}
			}

			if tt.runTwice {
				before := content
				if err := writeInstructionFile(path, tt.newContent, tt.mode); err != nil {
					t.Fatalf("second writeInstructionFile() error = %v", err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile() second read error = %v", err)
				}
				if string(data) != before {
					t.Fatalf("writeInstructionFile() second write changed content = %q, want %q", string(data), before)
				}
			}
		})
	}
}

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

	paths, err := writeAgentInstructions(projectDir, []string{"claude", "codex", "cursor"}, writeModeAppend)
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

	paths2, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
	if err != nil {
		t.Fatalf("writeAgentInstructions() second call error = %v, want nil", err)
	}
	if len(paths2) != 1 {
		t.Fatalf("len(paths2) = %d, want 1", len(paths2))
	}
	agentsData2, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) second read error = %v", err)
	}
	if !strings.Contains(string(agentsData2), agentMarkerStart("codex")) {
		t.Fatal("AGENTS.md missing codex start marker after second write")
	}
	if strings.Count(string(agentsData2), agentMarkerStart("codex")) != 1 {
		t.Fatal("AGENTS.md has duplicate codex marker blocks")
	}
}

func TestWriteAgentMemoryFiles(t *testing.T) {
	projectDir := t.TempDir()

	paths, err := writeAgentMemoryFiles(projectDir, []string{"claude", "codex", "cursor"}, writeModeAppend)
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

	paths2, err := writeAgentMemoryFiles(projectDir, []string{"codex"}, writeModeAppend)
	if err != nil {
		t.Fatalf("writeAgentMemoryFiles() second call error = %v, want nil", err)
	}
	if len(paths2) != 1 {
		t.Fatalf("len(paths2) = %d, want 1", len(paths2))
	}
	data2, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) second read error = %v", err)
	}
	if strings.Count(string(data2), agentcomMarkerStart) != 1 {
		t.Fatal("MEMORY.md has duplicate marker blocks")
	}
}

func TestWriteInstructionPreservesUserContent(t *testing.T) {
	projectDir := t.TempDir()
	userContent := "# My Project\n\n## Custom Rules\n\n- Rule 1\n- Rule 2\n"
	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(userContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
	if err != nil {
		t.Fatalf("writeAgentInstructions() error = %v", err)
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# My Project") {
		t.Fatal("user content lost: missing # My Project")
	}
	if !strings.Contains(content, "Rule 1") {
		t.Fatal("user content lost: missing Rule 1")
	}
	if !strings.Contains(content, agentMarkerStart("codex")) {
		t.Fatal("missing codex start marker")
	}
	if !strings.Contains(content, "agentcom Workflow") {
		t.Fatal("missing agentcom workflow content")
	}

	_, err = writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
	if err != nil {
		t.Fatalf("second writeAgentInstructions() error = %v", err)
	}
	data2, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile() second read error = %v", err)
	}
	if string(data) != string(data2) {
		t.Fatal("second run produced different content (not idempotent)")
	}
}

func TestAgentSpecificMarkers(t *testing.T) {
	content := wrapWithAgentMarkers("codex", "hello")
	if !strings.Contains(content, agentMarkerStart("codex")) {
		t.Fatalf("wrapWithAgentMarkers() missing start marker: %s", content)
	}
	if !strings.Contains(content, agentMarkerEnd("codex")) {
		t.Fatalf("wrapWithAgentMarkers() missing end marker: %s", content)
	}

	start, end, found := findMarkerBounds("before\n"+content+"after", "codex")
	if !found {
		t.Fatal("findMarkerBounds() did not find agent-specific markers")
	}
	if start == 0 || end <= start {
		t.Fatalf("findMarkerBounds() returned invalid bounds start=%d end=%d", start, end)
	}
}

func TestMultiAgentSharedPath(t *testing.T) {
	projectDir := t.TempDir()

	paths, err := writeAgentInstructions(projectDir, []string{"codex", "opencode"}, writeModeAppend)
	if err != nil {
		t.Fatalf("writeAgentInstructions() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1 shared path", len(paths))
	}

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, agentMarkerStart("codex")) || !strings.Contains(content, agentMarkerEnd("codex")) {
		t.Fatalf("AGENTS.md missing codex markers: %s", content)
	}
	if !strings.Contains(content, agentMarkerStart("opencode")) || !strings.Contains(content, agentMarkerEnd("opencode")) {
		t.Fatalf("AGENTS.md missing opencode markers: %s", content)
	}

	if err := os.WriteFile(agentsPath, []byte(strings.ReplaceAll(content, "Register each long-running agent session", "CUSTOM-OPENCODE")), 0o644); err != nil {
		t.Fatalf("WriteFile(AGENTS.md) error = %v", err)
	}

	paths, err = writeAgentInstructions(projectDir, []string{"codex"}, writeModeAppend)
	if err != nil {
		t.Fatalf("writeAgentInstructions(codex) error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths second run) = %d, want 1", len(paths))
	}

	data, err = os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) second read error = %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "CUSTOM-OPENCODE") {
		t.Fatalf("opencode block should be preserved when only codex updates: %s", content)
	}
}

func TestMultiAgentSharedPathOverwriteKeepsAllBlocks(t *testing.T) {
	projectDir := t.TempDir()

	paths, err := writeAgentInstructions(projectDir, []string{"codex", "opencode"}, writeModeOverwrite)
	if err != nil {
		t.Fatalf("writeAgentInstructions() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1 shared path", len(paths))
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, agentMarkerStart("codex")) || !strings.Contains(content, agentMarkerEnd("codex")) {
		t.Fatalf("overwrite content missing codex markers: %s", content)
	}
	if !strings.Contains(content, agentMarkerStart("opencode")) || !strings.Contains(content, agentMarkerEnd("opencode")) {
		t.Fatalf("overwrite content missing opencode markers: %s", content)
	}
}
