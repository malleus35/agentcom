package onboard

import (
	"testing"

	"github.com/malleus35/agentcom/internal/task"
)

func TestTemplateDefinitionCarriesReviewPolicyAndRoles(t *testing.T) {
	definition := TemplateDefinition{
		Name:        "company",
		Description: "Company template",
		ReviewPolicy: &task.ReviewPolicy{
			RequireReviewAbove: task.PriorityHigh,
			DefaultReviewer:    "user",
		},
		Roles: []TemplateRole{{
			Name:             "plan",
			AgentName:        "plan",
			AgentType:        "pm",
			CommunicatesWith: []string{"frontend", "backend"},
			Responsibilities: []string{"coordinate"},
		}},
	}

	if definition.ReviewPolicy == nil || definition.ReviewPolicy.DefaultReviewer != "user" {
		t.Fatalf("ReviewPolicy = %+v", definition.ReviewPolicy)
	}
	if len(definition.Roles) != 1 || definition.Roles[0].CommunicatesWith[1] != "backend" {
		t.Fatalf("Roles = %+v", definition.Roles)
	}
}
