package cli

import (
	"strings"
	"testing"
)

func TestValidateAgentToolsSelection(t *testing.T) {
	tests := []struct {
		name    string
		value   []string
		wantErr bool
	}{
		{name: "empty selection", wantErr: true},
		{name: "single selection", value: []string{"codex"}},
		{name: "multiple selections", value: []string{"claude", "codex"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentToolsSelection(tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("validateAgentToolsSelection() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateAgentToolsSelection() error = %v", err)
			}
			if tt.wantErr && err != nil && err.Error() != agentToolsError {
				t.Fatalf("validateAgentToolsSelection() error = %q, want %q", err.Error(), agentToolsError)
			}
		})
	}
}

func TestAgentToolsDescriptionIncludesSelectionGuidance(t *testing.T) {
	if !strings.Contains(agentToolsDescription, "Space to select") {
		t.Fatalf("agentToolsDescription = %q, want Space guidance", agentToolsDescription)
	}
	if !strings.Contains(agentToolsDescription, "Enter to continue") {
		t.Fatalf("agentToolsDescription = %q, want Enter guidance", agentToolsDescription)
	}
}
