package mcp

// ToolDef describes an MCP tool and its input schema.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// AllTools returns all MCP tools supported by agentcom.
func AllTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "list_agents",
			Description: "List registered agents, optionally only alive agents.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"alive_only": map[string]interface{}{"type": "boolean"},
					"project":    map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "send_message",
			Description: "Send a message to a target agent and persist it.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from":    map[string]interface{}{"type": "string"},
					"to":      map[string]interface{}{"type": "string"},
					"project": map[string]interface{}{"type": "string"},
					"type":    map[string]interface{}{"type": "string"},
					"topic":   map[string]interface{}{"type": "string"},
					"payload": map[string]interface{}{"type": "object"},
				},
				"required":             []string{"from", "to"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "broadcast",
			Description: "Broadcast a message to all alive agents.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from":    map[string]interface{}{"type": "string"},
					"project": map[string]interface{}{"type": "string"},
					"topic":   map[string]interface{}{"type": "string"},
					"payload": map[string]interface{}{"type": "object"},
				},
				"required":             []string{"from"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "create_task",
			Description: "Create a new task.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":       map[string]interface{}{"type": "string"},
					"description": map[string]interface{}{"type": "string"},
					"project":     map[string]interface{}{"type": "string"},
					"priority":    map[string]interface{}{"type": "string"},
					"assigned_to": map[string]interface{}{"type": "string"},
					"created_by":  map[string]interface{}{"type": "string"},
					"blocked_by": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required":             []string{"title"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "delegate_task",
			Description: "Delegate an existing task to an agent.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string"},
					"to":      map[string]interface{}{"type": "string"},
					"project": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"task_id", "to"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "list_tasks",
			Description: "List tasks with optional status and assignee filters.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status":   map[string]interface{}{"type": "string"},
					"assignee": map[string]interface{}{"type": "string"},
					"project":  map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "get_status",
			Description: "Get system status summary counts.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
	}
}
