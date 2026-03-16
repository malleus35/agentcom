package cli

import "fmt"

type roleMetadata struct {
	Description      string
	AgentNameSuffix  string
	AgentType        string
	Responsibilities []string
}

var knownRoleDefaults = map[string]roleMetadata{
	"frontend": {
		Description:     "Frontend implementation specialist for UI delivery and design handoff.",
		AgentNameSuffix: "frontend",
		AgentType:       "engineer-frontend",
		Responsibilities: []string{
			"Implement UI components and pages from design direction.",
			"Coordinate API contracts with backend.",
			"Send review-ready updates with file and state summaries.",
		},
	},
	"backend": {
		Description:     "Backend implementation specialist for APIs, services, and data flows.",
		AgentNameSuffix: "backend",
		AgentType:       "engineer-backend",
		Responsibilities: []string{
			"Implement services, schemas, and API endpoints.",
			"Confirm payload contracts with frontend.",
			"Escalate system risks and migration needs.",
		},
	},
	"plan": {
		Description:     "Planning specialist for task breakdown, sequencing, and coordination.",
		AgentNameSuffix: "plan",
		AgentType:       "pm",
		Responsibilities: []string{
			"Turn requests into deliverable task breakdowns.",
			"Coordinate handoffs between execution roles.",
			"Track blockers and completion signals across the team.",
		},
	},
	"review": {
		Description:     "Review specialist for QA, regression checks, and feedback loops.",
		AgentNameSuffix: "review",
		AgentType:       "qa",
		Responsibilities: []string{
			"Review delivered changes for correctness and risk.",
			"Request missing context from implementation roles.",
			"Report approval status and follow-up tasks.",
		},
	},
	"architect": {
		Description:     "Architecture specialist for system boundaries and design reviews.",
		AgentNameSuffix: "architect",
		AgentType:       "cto",
		Responsibilities: []string{
			"Define system-level constraints and interfaces.",
			"Review cross-cutting tradeoffs before implementation.",
			"Advise on architectural risk and migration paths.",
		},
	},
	"design": {
		Description:     "Design specialist for UX direction and visual handoff quality.",
		AgentNameSuffix: "design",
		AgentType:       "designer",
		Responsibilities: []string{
			"Produce UI intent, states, and interaction direction.",
			"Resolve ambiguities with frontend and architect.",
			"Support review with expected behavior and acceptance notes.",
		},
	},
	"qa": {
		Description:     "Quality assurance specialist for testing and verification.",
		AgentNameSuffix: "qa",
		AgentType:       "qa",
		Responsibilities: []string{
			"Write and maintain test suites.",
			"Verify implementation against acceptance criteria.",
			"Report defects with reproduction steps.",
		},
	},
	"devops": {
		Description:     "Infrastructure and deployment specialist.",
		AgentNameSuffix: "devops",
		AgentType:       "devops",
		Responsibilities: []string{
			"Manage CI/CD pipelines and deployment processes.",
			"Monitor system health and respond to incidents.",
			"Maintain infrastructure as code.",
		},
	},
	"security": {
		Description:     "Security specialist for auditing and threat modeling.",
		AgentNameSuffix: "security",
		AgentType:       "security",
		Responsibilities: []string{
			"Perform security reviews on code changes.",
			"Define and enforce security policies.",
			"Assess and mitigate security risks.",
		},
	},
}

func generateDefaultRole(roleName string, allRoleNames []string) templateRole {
	meta, ok := knownRoleDefaults[roleName]
	if !ok {
		meta = roleMetadata{
			Description:      fmt.Sprintf("Specialist role for %s tasks.", roleName),
			AgentNameSuffix:  roleName,
			AgentType:        "specialist",
			Responsibilities: []string{fmt.Sprintf("Handle %s-related tasks as assigned.", roleName)},
		}
	}

	communicatesWith := make([]string, 0, len(allRoleNames))
	for _, other := range allRoleNames {
		if other == roleName {
			continue
		}
		communicatesWith = append(communicatesWith, other)
	}

	return templateRole{
		Name:             roleName,
		Description:      meta.Description,
		AgentName:        meta.AgentNameSuffix,
		AgentType:        meta.AgentType,
		CommunicatesWith: communicatesWith,
		Responsibilities: append([]string(nil), meta.Responsibilities...),
	}
}

func isKnownRole(name string) bool {
	_, ok := knownRoleDefaults[name]
	return ok
}
