package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/onboard"
)

type setupTestPrompter struct {
	result onboard.Result
	err    error
}

func (p setupTestPrompter) Run(_ context.Context, _ onboard.Result) (onboard.Result, error) {
	return p.result, p.err
}

func TestInitCommandRunsWizardByDefault(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")

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

	oldPrompter := newOnboardPrompter
	oldInteractive := isInteractiveInput
	defer func() {
		newOnboardPrompter = oldPrompter
		isInteractiveInput = oldInteractive
	}()

	newOnboardPrompter = func(accessible bool, input io.Reader, output io.Writer) onboard.Prompter {
		return setupTestPrompter{result: onboard.Result{
			HomeDir:           homeDir,
			Template:          "company",
			WriteAgentsMD:     true,
			WriteInstructions: true,
			SelectedAgents:    []string{"codex"},
			Confirmed:         true,
		}}
	}
	isInteractiveInput = func(_ io.Reader) bool { return true }

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(homeDir, "agentcom.db")); err != nil {
		t.Fatalf("Stat(agentcom.db) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, "sockets")); err != nil {
		t.Fatalf("Stat(sockets) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); err != nil {
		t.Fatalf("Stat(AGENTS.md) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom", "templates", "company", "template.json")); err != nil {
		t.Fatalf("Stat(template.json) error = %v", err)
	}
	if !strings.Contains(buf.String(), "agentcom initialized at ") {
		t.Fatalf("output = %q, want init success", buf.String())
	}
}

func TestInitCommandNonInteractiveDefaultsToBatch(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), ".agentcom-home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
	}

	oldInteractive := isInteractiveInput
	oldApp := app
	defer func() {
		isInteractiveInput = oldInteractive
		app = oldApp
	}()
	isInteractiveInput = func(_ io.Reader) bool { return false }
	app = &appContext{cfg: &config.Config{HomeDir: homeDir}}

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "agentcom already initialized at ") {
		t.Fatalf("output = %q, want batch init result", buf.String())
	}
}

func TestShouldRunWizard(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		json        bool
		args        []string
		want        bool
	}{
		{name: "interactive default", interactive: true, want: true},
		{name: "batch flag", interactive: true, args: []string{"--batch"}, want: false},
		{name: "json mode", interactive: true, json: true, want: false},
		{name: "non interactive", interactive: false, want: false},
	}

	oldInteractive := isInteractiveInput
	oldJSON := jsonOutput
	defer func() {
		isInteractiveInput = oldInteractive
		jsonOutput = oldJSON
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInteractiveInput = func(_ io.Reader) bool { return tt.interactive }
			jsonOutput = tt.json

			cmd := newInitCmd()
			cmd.SetArgs(tt.args)
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatalf("Flags().Parse() error = %v", err)
			}

			if got := shouldRunWizard(cmd); got != tt.want {
				t.Fatalf("shouldRunWizard() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitCommandBatchGeneratesInstructionFiles(t *testing.T) {
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
	cmd.SetArgs([]string{"--batch", "--agents-md", "claude,codex"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, "CLAUDE.md")); err != nil {
		t.Fatalf("Stat(CLAUDE.md) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); err != nil {
		t.Fatalf("Stat(AGENTS.md) error = %v", err)
	}
	if !strings.Contains(buf.String(), "instruction_files") {
		t.Fatalf("json output missing instruction_files: %s", buf.String())
	}
}

func TestInitCommandBatchWritesProjectConfig(t *testing.T) {
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
	app = &appContext{cfg: &config.Config{HomeDir: homeDir}, project: "demo-app"}
	defer func() { app = oldApp }()

	oldProjectFlag := projectFlag
	projectFlag = "demo-app"
	defer func() { projectFlag = oldProjectFlag }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom.json")); err != nil {
		t.Fatalf("Stat(.agentcom.json) error = %v", err)
	}
	if !strings.Contains(buf.String(), "\"project\": \"demo-app\"") {
		t.Fatalf("json output missing project: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "project_config_path") {
		t.Fatalf("json output missing project_config_path: %s", buf.String())
	}

	buf.Reset()
	cmd = newInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second cmd.Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "\"project\": \"demo-app\"") {
		t.Fatalf("second json output missing project: %s", buf.String())
	}
}

func TestInitCommandWizardGeneratesInstructionAndMemoryFiles(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")

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

	oldPrompter := newOnboardPrompter
	oldInteractive := isInteractiveInput
	defer func() {
		newOnboardPrompter = oldPrompter
		isInteractiveInput = oldInteractive
	}()

	newOnboardPrompter = func(accessible bool, input io.Reader, output io.Writer) onboard.Prompter {
		return setupTestPrompter{result: onboard.Result{
			HomeDir:           homeDir,
			Project:           "wizard-app",
			WriteInstructions: true,
			WriteMemory:       true,
			SelectedAgents:    []string{"claude", "codex"},
			Confirmed:         true,
		}}
	}
	isInteractiveInput = func(_ io.Reader) bool { return true }

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, "CLAUDE.md")); err != nil {
		t.Fatalf("Stat(CLAUDE.md) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); err != nil {
		t.Fatalf("Stat(AGENTS.md) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agents", "MEMORY.md")); err != nil {
		t.Fatalf("Stat(MEMORY.md) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, ".agentcom.json"))
	if err != nil {
		t.Fatalf("ReadFile(.agentcom.json) error = %v", err)
	}
	if !strings.Contains(string(data), `"project": "wizard-app"`) {
		t.Fatalf("project config = %s, want wizard-app", string(data))
	}

	database, err := db.Open(filepath.Join(homeDir, config.DBFileName))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer database.Close()
	projects, err := database.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0] != "wizard-app" {
		t.Fatalf("projects = %#v, want [wizard-app]", projects)
	}
}

func TestInitSetupReInitPreservesContent(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), ".agentcom-test")
	userContent := "# My Project\n\n## Team Rules\n\n- Always write tests first.\n"
	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(userContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor := &initSetupExecutor{projectDir: projectDir}
	result := onboard.Result{HomeDir: homeDir, Project: "test-project", WriteInstructions: true, SelectedAgents: []string{"codex"}}
	if _, err := executor.Apply(context.Background(), result); err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	data1, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content1 := string(data1)
	if !strings.Contains(content1, "# My Project") {
		t.Fatal("user content lost after first init")
	}
	if !strings.Contains(content1, agentcomMarkerStart) {
		t.Fatal("agentcom markers missing after first init")
	}

	if _, err := executor.Apply(context.Background(), result); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	data2, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile() second read error = %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatal("second init changed content")
	}
}
