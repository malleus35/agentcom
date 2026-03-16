package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewUserErrorFormat(t *testing.T) {
	err := newUserError("Cannot init", "AGENTS.md already exists", "Use `agentcom init --force` to overwrite it.")

	var asError error = err
	message := asError.Error()
	for _, want := range []string{
		"Error: Cannot init",
		"Reason: AGENTS.md already exists",
		"Hint: Use `agentcom init --force` to overwrite it.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("error message %q missing %q", message, want)
		}
	}
}

func TestUserFacingErrorsUseStructuredFormat(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		hintParts []string
	}{
		{
			name: "instruction file exists",
			err: func() error {
				path := filepath.Join(t.TempDir(), "AGENTS.md")
				if err := writeInstructionFile(path, "first", writeModeCreate); err != nil {
					return err
				}
				return writeInstructionFile(path, "second", writeModeCreate)
			}(),
			hintParts: []string{"--force"},
		},
		{
			name: "unknown template name",
			err: func() error {
				_, err := resolveTemplateDefinition("missing-template")
				return err
			}(),
			hintParts: []string{"agentcom agents template"},
		},
		{
			name:      "invalid skill name",
			err:       validateSkillName("Invalid_Skill"),
			hintParts: []string{"lowercase", "hyphens"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error = nil, want structured error")
			}
			message := tt.err.Error()
			for _, section := range []string{"Error:", "Reason:", "Hint:"} {
				if !strings.Contains(message, section) {
					t.Fatalf("error %q missing %q", message, section)
				}
			}
			for _, hintPart := range tt.hintParts {
				if !strings.Contains(message, hintPart) {
					t.Fatalf("error %q missing hint %q", message, hintPart)
				}
			}
		})
	}
}
