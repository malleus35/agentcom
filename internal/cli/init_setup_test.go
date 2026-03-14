package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/onboard"
)

type setupTestPrompter struct {
	result onboard.Result
	err    error
}

func (p setupTestPrompter) Run(_ context.Context, _ onboard.Result) (onboard.Result, error) {
	return p.result, p.err
}

func TestInitCommandSetupRunsWizard(t *testing.T) {
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
			HomeDir:       homeDir,
			Template:      "company",
			WriteAgentsMD: true,
			Confirmed:     true,
		}}
	}
	isInteractiveInput = func(_ io.Reader) bool { return true }

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--setup"})

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

func TestInitCommandSetupRejectsNonInteractiveInput(t *testing.T) {
	oldInteractive := isInteractiveInput
	defer func() { isInteractiveInput = oldInteractive }()
	isInteractiveInput = func(_ io.Reader) bool { return false }

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--setup"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("cmd.Execute() error = nil, want error")
	}
}
