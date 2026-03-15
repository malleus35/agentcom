package onboard

type TemplateDefinition struct {
	Name        string
	Description string
	Reference   string
	CommonTitle string
	CommonBody  string
	Roles       []TemplateRole
}

type TemplateRole struct {
	Name             string
	Description      string
	AgentName        string
	AgentType        string
	CommunicatesWith []string
	Responsibilities []string
}
