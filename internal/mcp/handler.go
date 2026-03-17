package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/message"
	"github.com/malleus35/agentcom/internal/task"
	"github.com/malleus35/agentcom/internal/transport"
)

type invalidParamsError struct {
	message string
}

func (e *invalidParamsError) Error() string {
	return e.message
}

func newInvalidParamsError(format string, args ...any) error {
	return &invalidParamsError{message: fmt.Sprintf(format, args...)}
}

func unmarshalOptionalParams(params json.RawMessage, target any, handler string) error {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, target); err != nil {
		return newInvalidParamsError("%s: %v", handler, err)
	}
	return nil
}

func unmarshalRequiredParams(params json.RawMessage, target any, handler string) error {
	if err := json.Unmarshal(params, target); err != nil {
		return newInvalidParamsError("%s: %v", handler, err)
	}
	return nil
}

func requireTrimmed(value string) string {
	return strings.TrimSpace(value)
}

func requireField(handler string, value string, message string) (string, error) {
	trimmed := requireTrimmed(value)
	if trimmed == "" {
		return "", newInvalidParamsError("%s: %s", handler, message)
	}
	return trimmed, nil
}

func validateTaskStatus(status string) error {
	switch strings.TrimSpace(status) {
	case task.StatusPending, task.StatusAssigned, task.StatusInProgress, task.StatusBlocked, task.StatusCompleted, task.StatusFailed, task.StatusCancelled:
		return nil
	default:
		return fmt.Errorf("invalid status %q", status)
	}
}

func (s *Server) registerTools() {
	s.tools["list_agents"] = s.handleListAgents
	s.tools["send_message"] = s.handleSendMessage
	s.tools["send_to_user"] = s.handleSendToUser
	s.tools["get_user_messages"] = s.handleGetUserMessages
	s.tools["inbox"] = s.handleInbox
	s.tools["broadcast"] = s.handleBroadcast
	s.tools["user_reply"] = s.handleUserReply
	s.tools["create_task"] = s.handleCreateTask
	s.tools["delegate_task"] = s.handleDelegateTask
	s.tools["deregister"] = s.handleDeregister
	s.tools["update_task"] = s.handleUpdateTask
	s.tools["approve_task"] = s.handleApproveTask
	s.tools["reject_task"] = s.handleRejectTask
	s.tools["list_tasks"] = s.handleListTasks
	s.tools["get_status"] = s.handleGetStatus
	s.tools["health"] = s.handleHealth
	s.tools["doctor"] = s.handleDoctor
	s.tools["version"] = s.handleVersion
}

func (s *Server) handleListAgents(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type listAgentsParams struct {
		AliveOnly bool   `json:"alive_only"`
		Project   string `json:"project"`
	}

	var p listAgentsParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleListAgents"); err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	var (
		agents []*db.Agent
		err    error
	)
	if p.AliveOnly {
		agents, err = s.db.ListAliveAgentsByProject(ctx, project)
	} else {
		agents, err = s.db.ListAgentsByProject(ctx, project)
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
		Project string          `json:"project"`
		Type    string          `json:"type"`
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}

	var p sendMessageParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleSendMessage"); err != nil {
		return nil, err
	}
	from, err := requireField("mcp.handleSendMessage", p.From, "from and to are required")
	if err != nil {
		return nil, err
	}
	to, err := requireField("mcp.handleSendMessage", p.To, "from and to are required")
	if err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	msgType := strings.TrimSpace(p.Type)
	if msgType == "" {
		msgType = "notification"
	}
	payload := p.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	senderAgent, err := s.resolveAgentByNameOrID(ctx, from, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleSendMessage: %v", err)
	}

	targetAgent, err := s.resolveAgentByNameOrID(ctx, to, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleSendMessage: %v", err)
	}

	msg := &db.Message{
		FromAgent: senderAgent.ID,
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
		Project string          `json:"project"`
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}

	var p broadcastParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleBroadcast"); err != nil {
		return nil, err
	}
	from, err := requireField("mcp.handleBroadcast", p.From, "from is required")
	if err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	payload := p.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	senderAgent, err := s.resolveAgentByNameOrID(ctx, from, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleBroadcast: %v", err)
	}

	agents, err := s.db.ListAliveAgentsByProject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleBroadcast: %w", err)
	}

	messageIDs := make([]string, 0, len(agents))
	recipients := 0
	for _, a := range agents {
		if a.ID == senderAgent.ID {
			continue
		}
		if a.Type == "human" {
			continue
		}

		msg := &db.Message{
			FromAgent: senderAgent.ID,
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

func (s *Server) handleSendToUser(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type sendToUserParams struct {
		From     string `json:"from"`
		Text     string `json:"text"`
		Topic    string `json:"topic"`
		Priority string `json:"priority"`
		Project  string `json:"project"`
	}

	var p sendToUserParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleSendToUser"); err != nil {
		return nil, err
	}
	from, err := requireField("mcp.handleSendToUser", p.From, "from and text are required")
	if err != nil {
		return nil, err
	}
	if _, err := requireField("mcp.handleSendToUser", p.Text, "from and text are required"); err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	senderAgent, err := s.resolveAgentByNameOrID(ctx, from, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleSendToUser: %v", err)
	}
	userAgent, err := s.resolveUserAgent(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleSendToUser: %w", err)
	}

	payload, err := json.Marshal(map[string]string{
		"text":     p.Text,
		"priority": strings.TrimSpace(p.Priority),
	})
	if err != nil {
		return nil, fmt.Errorf("mcp.handleSendToUser: marshal payload: %w", err)
	}

	msg := &db.Message{
		FromAgent: senderAgent.ID,
		ToAgent:   userAgent.ID,
		Type:      "request",
		Topic:     p.Topic,
		Payload:   string(payload),
	}
	if err := s.db.InsertMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("mcp.handleSendToUser: %w", err)
	}

	return map[string]interface{}{
		"message_id": msg.ID,
		"status":     "delivered_to_inbox",
		"to":         userAgent.ID,
	}, nil
}

func (s *Server) handleGetUserMessages(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type getUserMessagesParams struct {
		Agent      string `json:"agent"`
		UnreadOnly *bool  `json:"unread_only"`
		Project    string `json:"project"`
	}

	var p getUserMessagesParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleGetUserMessages"); err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	userAgent, err := s.resolveUserAgent(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleGetUserMessages: %w", err)
	}

	messages, err := s.db.ListMessagesFromAgent(ctx, userAgent.ID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleGetUserMessages: %w", err)
	}

	targetAgentID := ""
	if strings.TrimSpace(p.Agent) != "" {
		agentRecord, err := s.resolveAgentByNameOrID(ctx, p.Agent, project)
		if err != nil {
			return nil, newInvalidParamsError("mcp.handleGetUserMessages: %v", err)
		}
		targetAgentID = agentRecord.ID
	}

	unreadOnly := true
	if p.UnreadOnly != nil {
		unreadOnly = *p.UnreadOnly
	}

	filtered := make([]*db.Message, 0, len(messages))
	for _, msg := range messages {
		if targetAgentID != "" && msg.ToAgent != targetAgentID {
			continue
		}
		if unreadOnly && msg.ReadAt != "" {
			continue
		}
		filtered = append(filtered, msg)
	}
	for _, msg := range filtered {
		if msg.ReadAt != "" {
			continue
		}
		if err := s.db.MarkRead(ctx, msg.ID); err != nil {
			return nil, fmt.Errorf("mcp.handleGetUserMessages: mark read: %w", err)
		}
	}

	return map[string]interface{}{
		"messages": filtered,
		"count":    len(filtered),
	}, nil
}

func (s *Server) handleInbox(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type inboxParams struct {
		Agent   string `json:"agent"`
		Unread  bool   `json:"unread"`
		From    string `json:"from"`
		Project string `json:"project"`
	}

	var p inboxParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleInbox"); err != nil {
		return nil, err
	}
	agentRef, err := requireField("mcp.handleInbox", p.Agent, "agent is required")
	if err != nil {
		return nil, err
	}
	project := s.requestedProject(p.Project)
	target, err := s.resolveAgentByNameOrID(ctx, agentRef, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleInbox: %v", err)
	}
	inbox := message.NewInbox(s.db)
	var messagesList []*db.Message
	if p.Unread {
		messagesList, err = inbox.ListUnread(ctx, target.ID)
	} else {
		messagesList, err = inbox.ListMessages(ctx, target.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("mcp.handleInbox: %w", err)
	}
	filtered := make([]*db.Message, 0, len(messagesList))
	fromID := ""
	if strings.TrimSpace(p.From) != "" {
		sender, err := s.resolveAgentByNameOrID(ctx, p.From, project)
		if err != nil {
			return nil, newInvalidParamsError("mcp.handleInbox: %v", err)
		}
		fromID = sender.ID
	}
	for _, msg := range messagesList {
		if fromID != "" && msg.FromAgent != fromID {
			continue
		}
		filtered = append(filtered, msg)
	}
	return map[string]interface{}{"messages": filtered, "count": len(filtered)}, nil
}

func (s *Server) handleUserReply(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type userReplyParams struct {
		To      string          `json:"to"`
		Text    string          `json:"text"`
		Payload json.RawMessage `json:"payload"`
		Project string          `json:"project"`
	}
	var p userReplyParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleUserReply"); err != nil {
		return nil, err
	}
	to, err := requireField("mcp.handleUserReply", p.To, "to is required")
	if err != nil {
		return nil, err
	}
	project := s.requestedProject(p.Project)
	userAgent, err := s.resolveUserAgent(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleUserReply: %w", err)
	}
	payload := p.Payload
	if len(payload) == 0 {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			return nil, newInvalidParamsError("mcp.handleUserReply: text or payload is required")
		}
		payload, err = json.Marshal(map[string]string{"text": text})
		if err != nil {
			return nil, fmt.Errorf("mcp.handleUserReply: marshal payload: %w", err)
		}
	}
	router := message.NewRouter(s.db, mcpDBFinder{db: s.db}, transport.NewClient(), project)
	env, err := router.Send(ctx, userAgent.ID, to, "response", "", payload)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleUserReply: %w", err)
	}
	return env, nil
}

func (s *Server) handleCreateTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type createTaskParams struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Project     string   `json:"project"`
		Priority    string   `json:"priority"`
		Reviewer    string   `json:"reviewer"`
		AssignedTo  string   `json:"assigned_to"`
		CreatedBy   string   `json:"created_by"`
		BlockedBy   []string `json:"blocked_by"`
	}

	var p createTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, newInvalidParamsError("mcp.handleCreateTask: %v", err)
	}
	if strings.TrimSpace(p.Title) == "" {
		return nil, newInvalidParamsError("mcp.handleCreateTask: title is required")
	}

	project := s.requestedProject(p.Project)
	priority := task.NormalizePriority(p.Priority)
	if priority == "" {
		priority = task.PriorityMedium
	}
	if err := task.ValidatePriority(priority); err != nil {
		return nil, newInvalidParamsError("mcp.handleCreateTask: %v", err)
	}

	assignedTo := strings.TrimSpace(p.AssignedTo)
	if assignedTo != "" {
		agentRecord, err := s.resolveAgentByNameOrID(ctx, assignedTo, project)
		if err == nil {
			assignedTo = agentRecord.ID
		}
	}
	createdBy := strings.TrimSpace(p.CreatedBy)
	if createdBy != "" {
		agentRecord, err := s.resolveAgentByNameOrID(ctx, createdBy, project)
		if err == nil {
			createdBy = agentRecord.ID
		}
	}
	policy, err := s.loadActiveTaskReviewPolicy()
	if err != nil {
		return nil, fmt.Errorf("mcp.handleCreateTask: %w", err)
	}

	manager := task.NewManager(s.db)
	t, err := manager.Create(ctx, p.Title, p.Description, priority, strings.TrimSpace(p.Reviewer), assignedTo, createdBy, p.BlockedBy, policy)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleCreateTask: %w", err)
	}

	return map[string]interface{}{
		"task_id":  t.ID,
		"status":   t.Status,
		"priority": t.Priority,
		"reviewer": t.Reviewer,
	}, nil
}

func (s *Server) handleDelegateTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type delegateTaskParams struct {
		TaskID  string `json:"task_id"`
		To      string `json:"to"`
		Project string `json:"project"`
	}

	var p delegateTaskParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleDelegateTask"); err != nil {
		return nil, err
	}
	if _, err := requireField("mcp.handleDelegateTask", p.TaskID, "task_id and to are required"); err != nil {
		return nil, err
	}
	to, err := requireField("mcp.handleDelegateTask", p.To, "task_id and to are required")
	if err != nil {
		return nil, err
	}

	project := s.requestedProject(p.Project)
	agentRecord, err := s.resolveAgentByNameOrID(ctx, to, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleDelegateTask: %v", err)
	}

	manager := task.NewManager(s.db)
	if err := manager.Delegate(ctx, p.TaskID, agentRecord.ID); err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}
	t, err := s.db.FindTaskByID(ctx, p.TaskID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleDelegateTask: %w", err)
	}

	return map[string]interface{}{
		"task_id":     t.ID,
		"assigned_to": t.AssignedTo,
		"status":      t.Status,
	}, nil
}

func (s *Server) handleUpdateTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type updateTaskParams struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		Result  string `json:"result"`
		Project string `json:"project"`
	}

	var p updateTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, newInvalidParamsError("mcp.handleUpdateTask: %v", err)
	}
	if strings.TrimSpace(p.TaskID) == "" || strings.TrimSpace(p.Status) == "" {
		return nil, newInvalidParamsError("mcp.handleUpdateTask: task_id and status are required")
	}
	manager := task.NewManager(s.db)
	if err := manager.UpdateStatus(ctx, p.TaskID, strings.TrimSpace(p.Status), p.Result); err != nil {
		return nil, fmt.Errorf("mcp.handleUpdateTask: %w", err)
	}
	updated, err := s.db.FindTaskByID(ctx, p.TaskID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleUpdateTask: %w", err)
	}
	return map[string]interface{}{"task_id": updated.ID, "status": updated.Status, "result": updated.Result, "reviewer": updated.Reviewer}, nil
}

func (s *Server) handleApproveTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type approveTaskParams struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
	}
	var p approveTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, newInvalidParamsError("mcp.handleApproveTask: %v", err)
	}
	if strings.TrimSpace(p.TaskID) == "" {
		return nil, newInvalidParamsError("mcp.handleApproveTask: task_id is required")
	}
	manager := task.NewManager(s.db)
	if err := manager.ApproveTask(ctx, p.TaskID, p.Result); err != nil {
		return nil, fmt.Errorf("mcp.handleApproveTask: %w", err)
	}
	updated, err := s.db.FindTaskByID(ctx, p.TaskID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleApproveTask: %w", err)
	}
	return map[string]interface{}{"task_id": updated.ID, "status": updated.Status, "result": updated.Result}, nil
}

func (s *Server) handleRejectTask(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type rejectTaskParams struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
	}
	var p rejectTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, newInvalidParamsError("mcp.handleRejectTask: %v", err)
	}
	if strings.TrimSpace(p.TaskID) == "" {
		return nil, newInvalidParamsError("mcp.handleRejectTask: task_id is required")
	}
	manager := task.NewManager(s.db)
	if err := manager.RejectTask(ctx, p.TaskID, p.Result); err != nil {
		return nil, fmt.Errorf("mcp.handleRejectTask: %w", err)
	}
	updated, err := s.db.FindTaskByID(ctx, p.TaskID)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleRejectTask: %w", err)
	}
	return map[string]interface{}{"task_id": updated.ID, "status": updated.Status, "result": updated.Result}, nil
}

func (s *Server) handleListTasks(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type listTasksParams struct {
		Status   string `json:"status"`
		Assignee string `json:"assignee"`
		Project  string `json:"project"`
	}

	var p listTasksParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleListTasks"); err != nil {
		return nil, err
	}

	status := strings.TrimSpace(p.Status)
	assignee := strings.TrimSpace(p.Assignee)
	project := s.requestedProject(p.Project)
	if status != "" {
		if err := validateTaskStatus(status); err != nil {
			return nil, newInvalidParamsError("mcp.handleListTasks: %v", err)
		}
	}

	var (
		tasks []*db.Task
		err   error
	)
	switch {
	case status != "":
		tasks, err = s.db.ListTasksByStatus(ctx, status)
	case assignee != "":
		resolvedAssignee, resolveErr := s.resolveAssigneeID(ctx, assignee, project)
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
		resolvedAssignee, resolveErr := s.resolveAssigneeID(ctx, assignee, project)
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

func (s *Server) handleGetStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type getStatusParams struct {
		Project string `json:"project"`
	}

	var p getStatusParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleGetStatus"); err != nil {
		return nil, err
	}
	project := s.requestedProject(p.Project)

	var agentCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE (? = '' OR project = ?)`, project, project).Scan(&agentCount); err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}

	var messageCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN agents sender ON sender.id = m.from_agent
		LEFT JOIN agents recipient ON recipient.id = m.to_agent
		WHERE (? = '' OR sender.project = ? OR recipient.project = ?)
	`, project, project, project).Scan(&messageCount); err != nil {
		return nil, fmt.Errorf("mcp.handleGetStatus: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tasks.status, COUNT(*)
		FROM tasks
		LEFT JOIN agents assigned ON assigned.id = tasks.assigned_to
		LEFT JOIN agents created ON created.id = tasks.created_by
		WHERE (? = '' OR assigned.project = ? OR created.project = ?)
		GROUP BY tasks.status
	`, project, project, project)
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

func (s *Server) handleDeregister(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type deregisterParams struct {
		NameOrID string `json:"name_or_id"`
		Project  string `json:"project"`
	}
	var p deregisterParams
	if err := unmarshalRequiredParams(params, &p, "mcp.handleDeregister"); err != nil {
		return nil, err
	}
	nameOrID, err := requireField("mcp.handleDeregister", p.NameOrID, "name_or_id is required")
	if err != nil {
		return nil, err
	}
	project := s.requestedProject(p.Project)
	target, err := s.resolveAgentByNameOrID(ctx, nameOrID, project)
	if err != nil {
		return nil, newInvalidParamsError("mcp.handleDeregister: %v", err)
	}
	registry := agent.NewRegistry(s.db, s.cfg)
	if err := registry.Deregister(ctx, target.ID, target.Project); err != nil {
		return nil, fmt.Errorf("mcp.handleDeregister: %w", err)
	}
	return map[string]interface{}{"id": target.ID, "name": target.Name, "status": "deregistered"}, nil
}

func (s *Server) handleHealth(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type healthParams struct {
		Project string `json:"project"`
	}
	var p healthParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleHealth"); err != nil {
		return nil, err
	}
	project := s.requestedProject(p.Project)
	registry := agent.NewRegistry(s.db, s.cfg)
	agents, err := registry.ListAll(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("mcp.handleHealth: %w", err)
	}
	type healthEntry struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		Project      string `json:"project"`
		PID          int    `json:"pid"`
		HeartbeatAge string `json:"heartbeat_age"`
		Socket       bool   `json:"socket"`
		Verdict      string `json:"verdict"`
	}
	entries := make([]healthEntry, 0, len(agents))
	now := time.Now().UTC()
	for _, agt := range agents {
		age := now.Sub(agt.LastHeartbeat)
		socketOK := false
		if agt.SocketPath != "" {
			if _, err := os.Stat(agt.SocketPath); err == nil {
				socketOK = true
			}
		}
		verdict := "OK"
		if age > s.cfg.Runtime.HeartbeatStaleThreshold {
			verdict = "STALE"
		}
		entries = append(entries, healthEntry{Name: agt.Name, Type: agt.Type, Project: agt.Project, PID: agt.PID, HeartbeatAge: age.Round(time.Second).String(), Socket: socketOK, Verdict: verdict})
	}
	return map[string]interface{}{"checks": entries, "count": len(entries)}, nil
}

func (s *Server) handleDoctor(ctx context.Context, params json.RawMessage) (interface{}, error) {
	type doctorParams struct {
		Project string `json:"project"`
	}
	var p doctorParams
	if err := unmarshalOptionalParams(params, &p, "mcp.handleDoctor"); err != nil {
		return nil, err
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("mcp.handleDoctor: getwd: %w", err)
	}
	checks := make([]map[string]interface{}, 0, 4)
	checks = append(checks, map[string]interface{}{"category": "environment", "name": "home_dir", "status": pathStatus(s.cfg.HomeDir), "message": s.cfg.HomeDir})
	checks = append(checks, map[string]interface{}{"category": "environment", "name": "sockets_dir", "status": pathStatus(s.cfg.SocketsPath), "message": s.cfg.SocketsPath})
	projectCfg, configPath, err := config.LoadProjectConfig(projectDir)
	if err != nil || configPath == "" {
		checks = append(checks, map[string]interface{}{"category": "project", "name": "project_config", "status": "fail", "message": fmt.Sprintf("%s is missing", config.ProjectConfigFileName)})
	} else {
		checks = append(checks, map[string]interface{}{"category": "project", "name": "project_config", "status": "pass", "message": configPath})
		activeTemplate := strings.TrimSpace(projectCfg.Template.Active)
		status := "fail"
		if activeTemplate != "" {
			status = "pass"
		}
		checks = append(checks, map[string]interface{}{"category": "project", "name": "active_template", "status": status, "message": activeTemplate})
	}
	return map[string]interface{}{"checks": checks, "count": len(checks)}, nil
}

func (s *Server) handleVersion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"version":   "dev",
		"buildDate": "unknown",
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}, nil
}

func (s *Server) resolveAgentByNameOrID(ctx context.Context, nameOrID string, project string) (*db.Agent, error) {
	agentRecord, err := s.db.FindAgentByNameAndProject(ctx, nameOrID, project)
	if err == nil {
		return agentRecord, nil
	}
	if err != nil && !errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("mcp.resolveAgentByNameOrID: %w", err)
	}

	agentRecord, err = s.db.FindAgentByID(ctx, nameOrID)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			return nil, fmt.Errorf("unknown agent %q", nameOrID)
		}
		return nil, fmt.Errorf("mcp.resolveAgentByNameOrID: %w", err)
	}

	return agentRecord, nil
}

func (s *Server) resolveUserAgent(ctx context.Context, project string) (*db.Agent, error) {
	userAgent, err := s.db.FindAgentByNameAndProject(ctx, "user", project)
	if err == nil {
		return userAgent, nil
	}
	if !errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("mcp.resolveUserAgent: %w", err)
	}

	userAgent, err = s.db.FindAgentByTypeAndProject(ctx, "human", project)
	if err == nil {
		return userAgent, nil
	}
	if errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("no user agent registered; start a session with `agentcom up` first")
	}
	return nil, fmt.Errorf("mcp.resolveUserAgent: %w", err)
}

func (s *Server) resolveAssigneeID(ctx context.Context, assignee string, project string) (string, error) {
	agentRecord, err := s.resolveAgentByNameOrID(ctx, assignee, project)
	if err == nil {
		return agentRecord.ID, nil
	}

	return assignee, nil
}

func (s *Server) requestedProject(project string) string {
	project = strings.TrimSpace(project)
	if project != "" {
		return project
	}
	return s.project
}

type mcpDBFinder struct {
	db *db.DB
}

func (f mcpDBFinder) FindByName(ctx context.Context, name string, project string) (*db.Agent, error) {
	return f.db.FindAgentByNameAndProject(ctx, name, project)
}

func (f mcpDBFinder) FindByID(ctx context.Context, id string) (*db.Agent, error) {
	return f.db.FindAgentByID(ctx, id)
}

func (f mcpDBFinder) ListAlive(ctx context.Context, project string) ([]*db.Agent, error) {
	return f.db.ListAliveAgentsByProject(ctx, project)
}

func pathStatus(path string) string {
	if _, err := os.Stat(path); err == nil {
		return "pass"
	}
	return "fail"
}

func (s *Server) loadActiveTaskReviewPolicy() (*task.ReviewPolicy, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	projectCfg, _, err := config.LoadProjectConfig(cwd)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	if strings.TrimSpace(projectCfg.Template.Active) == "" {
		return nil, nil
	}
	manifestPath := filepath.Join(cwd, ".agentcom", "templates", projectCfg.Template.Active, "template.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read template manifest: %w", err)
	}
	var manifest struct {
		ReviewPolicy *task.ReviewPolicy `json:"review_policy,omitempty"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal template manifest: %w", err)
	}
	if manifest.ReviewPolicy != nil {
		if err := manifest.ReviewPolicy.Validate(); err != nil {
			return nil, fmt.Errorf("validate review policy: %w", err)
		}
	}
	return manifest.ReviewPolicy, nil
}
