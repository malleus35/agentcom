package onboard

import "github.com/malleus35/agentcom/internal/task"

type TemplateDefinition struct {
	Name         string
	Description  string
	Reference    string
	CommonTitle  string
	CommonBody   string
	ReviewPolicy *task.ReviewPolicy
	Roles        []TemplateRole
}

type TemplateRole struct {
	Name             string
	Description      string
	AgentName        string
	AgentType        string
	CommunicatesWith []string
	Responsibilities []string
}
