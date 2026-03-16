package cli

import (
	"reflect"
	"testing"
)

func TestGenerateDefaultRole(t *testing.T) {
	tests := []struct {
		name            string
		roleName        string
		allRoles        []string
		wantType        string
		wantAgentName   string
		wantDescription string
		wantComm        []string
	}{
		{
			name:            "known frontend",
			roleName:        "frontend",
			allRoles:        []string{"frontend", "backend"},
			wantType:        "engineer-frontend",
			wantAgentName:   "frontend",
			wantDescription: "Frontend implementation specialist for UI delivery and design handoff.",
			wantComm:        []string{"backend"},
		},
		{
			name:            "known backend",
			roleName:        "backend",
			allRoles:        []string{"frontend", "backend", "plan"},
			wantType:        "engineer-backend",
			wantAgentName:   "backend",
			wantDescription: "Backend implementation specialist for APIs, services, and data flows.",
			wantComm:        []string{"frontend", "plan"},
		},
		{
			name:            "unknown custom",
			roleName:        "ops",
			allRoles:        []string{"ops", "dev"},
			wantType:        "specialist",
			wantAgentName:   "ops",
			wantDescription: "Specialist role for ops tasks.",
			wantComm:        []string{"dev"},
		},
		{
			name:            "single role",
			roleName:        "solo",
			allRoles:        []string{"solo"},
			wantType:        "specialist",
			wantAgentName:   "solo",
			wantDescription: "Specialist role for solo tasks.",
			wantComm:        []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateDefaultRole(tt.roleName, tt.allRoles)
			if got.AgentType != tt.wantType {
				t.Fatalf("AgentType = %q, want %q", got.AgentType, tt.wantType)
			}
			if got.AgentName != tt.wantAgentName {
				t.Fatalf("AgentName = %q, want %q", got.AgentName, tt.wantAgentName)
			}
			if got.Description != tt.wantDescription {
				t.Fatalf("Description = %q, want %q", got.Description, tt.wantDescription)
			}
			if !reflect.DeepEqual(got.CommunicatesWith, tt.wantComm) {
				t.Fatalf("CommunicatesWith = %v, want %v", got.CommunicatesWith, tt.wantComm)
			}
		})
	}
}

func TestIsKnownRole(t *testing.T) {
	if !isKnownRole("frontend") {
		t.Fatal("isKnownRole(frontend) = false, want true")
	}
	if isKnownRole("unknown-role") {
		t.Fatal("isKnownRole(unknown-role) = true, want false")
	}
}
