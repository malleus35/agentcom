package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/task"
)

func (s *Server) registerTools() {
	s.tools["list_agents"] = s.handleListAgents
	s.tools["send_message"] = s.handleSendMessage
	s.tools["broadcast"] = s.handleBroadcast
	s.tools["create_task"] = s.handleCreateTask
	s.tools["delegate_task"] = s.handleDelegateTask
	s.tools["list_tasks"] = s.handleListTasks
	s.tools["get_status"] = s.handleGetStatus
}

func (s *Server) handleListAgents(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type listAgentsParams struct {
		AliveOnly bool `json:"alive_only"`
	}

	var p listAgentsParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("mcp.handleListAgents: %w", err)
		}
	}

	var (
		agents []*db.Agent
		err    error
	)
	if p.AliveOnly {
		agents, err = s.db.ListAliveAgents(ctx)
	} else {
		agents, err = s.db.ListAllAgents(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("mcp.handleListAgents: %w", err)
	}

	return map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}, nil
}

func (s *Server) handleSendMessage(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type sendMessageParams struct {
		From    string          `json:"from"`
		To      string          `json:"to"`
		Type    string          `json:"type"`
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}

	var p sendMessageParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("mcp.handleSendMessage: %w", err)
	}
	if strings.TrimSpace(p.From) == "" || strings.TrimSpace(p.To) == "" {
		return nil, fmt.Errorf("mcp.handleSendMessage: from and to are required")
	}

	msgType := strings.TrimSpace(p.Type)
	if msgType == "" {
		msgType = "notification"
	}
	payload := p.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	targetAgent, err := s.resolveAgentByNameOrID(ctx, p.To)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleSendMessage: %w", err)
	}

	msg := &db.Message{
		FromAgent: p.From,
		ToAgent:   targetAgent.ID,
		Type:      msgType,
		Topic:     p.Topic,
		Payload:   string(payload),
	}
	if err := s.db.InsertMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("mcp.handleSendMessage: %w", err)
	}

	return map[string]interface{}{
		"message_id": msg.ID,
		"to":         targetAgent.ID,
	}, nil
}

func (s *Server) handleBroadcast(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type broadcastParams struct {
		From    string          `json:"from"`
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}

	var p broadcastParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("mcp.handleBroadcast: %w", err)
	}
	if strings.TrimSpace(p.From) == "" {
		return nil, fmt.Errorf("mcp.handleBroadcast: from is required")
	}

	payload := p.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	agents, err := s.db.ListAliveAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleBroadcast: %w", err)
	}

	messageIDs := make([]string, 0, len(agents))
	recipients := 0
	for _, a := range agents {
		if a.ID == p.From || a.Name == p.From {
			continue
		}

		msg := &db.Message{
			FromAgent: p.From,
			ToAgent:   a.ID,
			Type:      "broadcast",
			Topic:     p.Topic,
			Payload:   string(payload),
		}
		if err := s.db.InsertMessage(ctx, msg); err != nil {
			return nil, fmt.Errorf("mcp.handleBroadcast: %w", err)
		}

		messageIDs = append(messageIDs, msg.ID)
		recipients++
	}

	return map[string]interface{}{
		"recipients":  recipients,
		"message_ids": messageIDs,
	}, nil
}

func (s *Server) handleCreateTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type createTaskParams struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		AssignedTo  string   `json:"assigned_to"`
		CreatedBy   string   `json:"created_by"`
		BlockedBy   []string `json:"blocked_by"`
	}

	var p createTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("mcp.handleCreateTask: %w", err)
	}
	if strings.TrimSpace(p.Title) == "" {
		return nil, fmt.Errorf("mcp.handleCreateTask: title is required")
	}

	priority := strings.TrimSpace(p.Priority)
	if priority == "" {
		priority = "medium"
	}
	blockedByJSON, err := json.Marshal(p.BlockedBy)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleCreateTask: %w", err)
	}

	t := &db.Task{
		Title:       p.Title,
		Description: p.Description,
		Status:      "pending",
		Priority:    priority,
		AssignedTo:  p.AssignedTo,
		CreatedBy:   p.CreatedBy,
		BlockedBy:   string(blockedByJSON),
	}
	if err := s.db.InsertTask(ctx, t); err != nil {
		return nil, fmt.Errorf("mcp.handleCreateTask: %w", err)
	}

	return map[string]interface{}{
		"task_id": t.ID,
		"status":  t.Status,
	}, nil
}

func (s *Server) handleDelegateTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type delegateTaskParams struct {
		TaskID string `json:"task_id"`
		To     string `json:"to"`
	}

	var p delegateTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}
	if strings.TrimSpace(p.TaskID) == "" || strings.TrimSpace(p.To) == "" {
		return nil, fmt.Errorf("mcp.handleDelegateTask: task_id and to are required")
	}

	t, err := s.db.FindTaskByID(ctx, p.TaskID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}
	if err := task.ValidateTransition(t.Status, task.StatusAssigned); err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}

	agentRecord, err := s.resolveAgentByNameOrID(ctx, p.To)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}

	t.AssignedTo = agentRecord.ID
	t.Status = task.StatusAssigned
	if err := s.db.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}

	return map[string]interface{}{
		"task_id":     t.ID,
		"assigned_to": t.AssignedTo,
		"status":      t.Status,
	}, nil
}

func (s *Server) handleListTasks(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type listTasksParams struct {
		Status   string `json:"status"`
		Assignee string `json:"assignee"`
	}

	var p listTasksParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("mcp.handleListTasks: %w", err)
		}
	}

	status := strings.TrimSpace(p.Status)
	assignee := strings.TrimSpace(p.Assignee)

	var (
		tasks []*db.Task
		err   error
	)
	switch {
	case status != "":
		tasks, err = s.db.ListTasksByStatus(ctx, status)
	case assignee != "":
		resolvedAssignee, resolveErr := s.resolveAssigneeID(ctx, assignee)
		if resolveErr != nil {
			return nil, fmt.Errorf("mcp.handleListTasks: %w", resolveErr)
		}
		tasks, err = s.db.ListTasksByAssignee(ctx, resolvedAssignee)
	default:
		tasks, err = s.db.ListAllTasks(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("mcp.handleListTasks: %w", err)
	}

	if status != "" && assignee != "" {
		resolvedAssignee, resolveErr := s.resolveAssigneeID(ctx, assignee)
		if resolveErr != nil {
			return nil, fmt.Errorf("mcp.handleListTasks: %w", resolveErr)
		}
		filtered := make([]*db.Task, 0, len(tasks))
		for _, t := range tasks {
			if t.AssignedTo == resolvedAssignee {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	return map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	}, nil
}

func (s *Server) handleGetStatus(ctx context.Context, _ json.RawMessage) (interface{}, error) {
	var agentCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents`).Scan(&agentCount); err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}

	var messageCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&messageCount); err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}
	defer rows.Close()

	tasksByStatus := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
		}
		tasksByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}

	return map[string]interface{}{
		"agents": map[string]int{
			"total": agentCount,
		},
		"messages": map[string]int{
			"total": messageCount,
		},
		"tasks": tasksByStatus,
	}, nil
}

func (s *Server) resolveAgentByNameOrID(ctx context.Context, nameOrID string) (*db.Agent, error) {
	agentRecord, err := s.db.FindAgentByName(ctx, nameOrID)
	if err == nil {
		return agentRecord, nil
	}

	agentRecord, err = s.db.FindAgentByID(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("mcp.resolveAgentByNameOrID: %w", err)
	}

	return agentRecord, nil
}

func (s *Server) resolveAssigneeID(ctx context.Context, assignee string) (string, error) {
	agentRecord, err := s.resolveAgentByNameOrID(ctx, assignee)
	if err == nil {
		return agentRecord.ID, nil
	}

	return assignee, nil
}
