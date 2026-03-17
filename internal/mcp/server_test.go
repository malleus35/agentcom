package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

func setupMCPTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("db.OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("database.Close() error = %v", err)
		}
	})
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	homeDir := t.TempDir()
	cfg := &config.Config{
		HomeDir:     homeDir,
		DBPath:      filepath.Join(homeDir, config.DBFileName),
		SocketsPath: filepath.Join(homeDir, config.SocketsDir),
	}

	return NewServer(database, cfg, "project-a"), database
}

func TestServerRunJSONRPCRoundTrip(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender := &db.Agent{Name: "sender", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, sender); err != nil {
		t.Fatalf("InsertAgent(sender) error = %v", err)
	}
	receiver := &db.Agent{Name: "receiver", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, receiver); err != nil {
		t.Fatalf("InsertAgent(receiver) error = %v", err)
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	runErr := make(chan error, 1)
	go func() {
		runErr <- server.Run(ctx, inReader, outWriter)
	}()

	enc := json.NewEncoder(inWriter)
	dec := json.NewDecoder(outReader)

	if err := enc.Encode(Request{
		JSONRPC: jsonRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("encode initialize error = %v", err)
	}

	var initResp Response
	if err := dec.Decode(&initResp); err != nil {
		t.Fatalf("decode initialize response error = %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize error = %+v", initResp.Error)
	}
	resultMap, ok := initResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("initialize result type = %T, want object", initResp.Result)
	}
	if resultMap["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %v, want %q", resultMap["protocolVersion"], protocolVersion)
	}

	if err := enc.Encode(Request{
		JSONRPC: jsonRPCVersion,
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("encode initialized notification error = %v", err)
	}

	if err := enc.Encode(Request{
		JSONRPC: jsonRPCVersion,
		ID:      2,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("encode tools/list error = %v", err)
	}

	var listResp Response
	if err := dec.Decode(&listResp); err != nil {
		t.Fatalf("decode tools/list response error = %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list error = %+v", listResp.Error)
	}
	listResult, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("tools/list result type = %T, want object", listResp.Result)
	}
	tools, ok := listResult["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list tools = %#v, want non-empty list", listResult["tools"])
	}
	hasSendToUser := false
	hasGetUserMessages := false
	hasUpdateTask := false
	hasApproveTask := false
	hasRejectTask := false
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		switch toolMap["name"] {
		case "send_to_user":
			hasSendToUser = true
		case "get_user_messages":
			hasGetUserMessages = true
		case "update_task":
			hasUpdateTask = true
		case "approve_task":
			hasApproveTask = true
		case "reject_task":
			hasRejectTask = true
		}
	}
	if !hasSendToUser || !hasGetUserMessages || !hasUpdateTask || !hasApproveTask || !hasRejectTask {
		t.Fatalf("tools/list missing expected tools: send_to_user=%v get_user_messages=%v update_task=%v approve_task=%v reject_task=%v", hasSendToUser, hasGetUserMessages, hasUpdateTask, hasApproveTask, hasRejectTask)
	}

	if err := enc.Encode(Request{
		JSONRPC: jsonRPCVersion,
		ID:      3,
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name":"send_message",
			"arguments":{
				"from":"` + sender.ID + `",
				"to":"receiver",
				"type":"notification",
				"topic":"hello",
				"payload":{"ok":true}
			}
		}`),
	}); err != nil {
		t.Fatalf("encode tools/call error = %v", err)
	}

	var callResp Response
	if err := dec.Decode(&callResp); err != nil {
		t.Fatalf("decode tools/call response error = %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("tools/call error = %+v", callResp.Error)
	}
	callResult, ok := callResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("tools/call result type = %T, want object", callResp.Result)
	}
	if callResult["isError"] != false {
		t.Fatalf("tools/call isError = %v, want false", callResult["isError"])
	}

	messages, err := database.ListMessagesForAgent(ctx, receiver.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(ListMessagesForAgent()) = %d, want 1", len(messages))
	}
	if messages[0].Topic != "hello" {
		t.Fatalf("message topic = %q, want hello", messages[0].Topic)
	}

	if err := inWriter.Close(); err != nil {
		t.Fatalf("inWriter.Close() error = %v", err)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("Server.Run() error = %v", err)
	}
}

func TestCreateTaskInvalidPriorityReturnsJSONRPCError(t *testing.T) {
	server, _ := setupMCPTestServer(t)
	responses := runMCPRoundTripRequests(t, server,
		Request{JSONRPC: jsonRPCVersion, ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, ID: 2, Method: "tools/call", Params: json.RawMessage(`{
			"name":"create_task",
			"arguments":{"title":"bad task","priority":"urgent"}
		}`)},
	)
	resp := decodeResponseMap(t, responses[1])
	if resp.Error == nil {
		t.Fatal("resp.Error = nil, want invalid params error")
	}
	if resp.Error.Code != errInvalidParams {
		t.Fatalf("resp.Error.Code = %d, want %d", resp.Error.Code, errInvalidParams)
	}
	if _, ok := responses[1]["result"]; ok {
		t.Fatal("invalid priority response unexpectedly included result")
	}
	if resp.Error.Message == "" {
		t.Fatal("resp.Error.Message = empty, want invalid params detail")
	}
}

func TestMCPHandlerInvalidParamsMatrix(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, database *db.DB)
		request      string
		wantCode     int
		wantMessage  string
		messageMatch string
	}{
		{
			name:     "list_agents rejects wrong alive_only type",
			request:  `{"name":"list_agents","arguments":{"alive_only":"yes"}}`,
			wantCode: errInvalidParams,
		},
		{
			name:        "send_message rejects missing from",
			request:     `{"name":"send_message","arguments":{"from":"","to":"receiver"}}`,
			wantCode:    errInvalidParams,
			wantMessage: "mcp.handleSendMessage: from and to are required",
		},
		{
			name: "send_message rejects unknown recipient",
			setup: func(t *testing.T, database *db.DB) {
				t.Helper()
				ctx := context.Background()
				sender := &db.Agent{Name: "sender", Type: "worker", Project: "project-a", Status: "alive"}
				if err := database.InsertAgent(ctx, sender); err != nil {
					t.Fatalf("InsertAgent(sender) error = %v", err)
				}
			},
			request:      `{"name":"send_message","arguments":{"from":"sender","to":"missing"}}`,
			wantCode:     errInvalidParams,
			messageMatch: "mcp.handleSendMessage:",
		},
		{
			name:        "send_to_user rejects missing text",
			request:     `{"name":"send_to_user","arguments":{"from":"plan","text":""}}`,
			wantCode:    errInvalidParams,
			wantMessage: "mcp.handleSendToUser: from and text are required",
		},
		{
			name: "get_user_messages rejects unknown agent filter",
			setup: func(t *testing.T, database *db.DB) {
				t.Helper()
				ctx := context.Background()
				user := &db.Agent{Name: "user", Type: "human", Project: "project-a", Status: "alive"}
				if err := database.InsertAgent(ctx, user); err != nil {
					t.Fatalf("InsertAgent(user) error = %v", err)
				}
			},
			request:      `{"name":"get_user_messages","arguments":{"agent":"missing"}}`,
			wantCode:     errInvalidParams,
			messageMatch: "mcp.handleGetUserMessages:",
		},
		{
			name:        "broadcast rejects missing from",
			request:     `{"name":"broadcast","arguments":{"from":""}}`,
			wantCode:    errInvalidParams,
			wantMessage: "mcp.handleBroadcast: from is required",
		},
		{
			name: "delegate_task rejects unknown target",
			setup: func(t *testing.T, database *db.DB) {
				t.Helper()
				ctx := context.Background()
				creator := &db.Agent{Name: "creator", Type: "worker", Project: "project-a", Status: "alive"}
				if err := database.InsertAgent(ctx, creator); err != nil {
					t.Fatalf("InsertAgent(creator) error = %v", err)
				}
				created := &db.Task{Title: "task", Status: "pending"}
				if err := database.InsertTask(ctx, created); err != nil {
					t.Fatalf("InsertTask() error = %v", err)
				}
			},
			request:      `{"name":"delegate_task","arguments":{"task_id":"tsk_test","to":"missing"}}`,
			wantCode:     errInvalidParams,
			messageMatch: "mcp.handleDelegateTask:",
		},
		{
			name:         "list_tasks rejects invalid status filter",
			request:      `{"name":"list_tasks","arguments":{"status":"paused"}}`,
			wantCode:     errInvalidParams,
			messageMatch: "mcp.handleListTasks:",
		},
		{
			name:     "get_status rejects wrong project type",
			request:  `{"name":"get_status","arguments":{"project":123}}`,
			wantCode: errInvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, database := setupMCPTestServer(t)
			if tt.setup != nil {
				tt.setup(t, database)
			}

			responses := runMCPRoundTripRequests(t, server,
				Request{JSONRPC: jsonRPCVersion, ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)},
				Request{JSONRPC: jsonRPCVersion, Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
				Request{JSONRPC: jsonRPCVersion, ID: 2, Method: "tools/call", Params: json.RawMessage(tt.request)},
			)

			resp := decodeResponseMap(t, responses[1])
			if resp.Error == nil {
				t.Fatal("resp.Error = nil, want JSON-RPC error")
			}
			if resp.Error.Code != tt.wantCode {
				t.Fatalf("resp.Error.Code = %d, want %d", resp.Error.Code, tt.wantCode)
			}
			if tt.wantMessage != "" && resp.Error.Message != tt.wantMessage {
				t.Fatalf("resp.Error.Message = %q, want %q", resp.Error.Message, tt.wantMessage)
			}
			if tt.messageMatch != "" && !strings.Contains(resp.Error.Message, tt.messageMatch) {
				t.Fatalf("resp.Error.Message = %q, want substring %q", resp.Error.Message, tt.messageMatch)
			}
			if _, ok := responses[1]["result"]; ok {
				t.Fatal("error response unexpectedly included result")
			}
		})
	}
}

func TestMCPRuntimeErrorBoundaryMatrix(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()

	plan := &db.Agent{Name: "plan", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, plan); err != nil {
		t.Fatalf("InsertAgent(plan) error = %v", err)
	}

	responses := runMCPRoundTripRequests(t, server,
		Request{JSONRPC: jsonRPCVersion, ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, ID: 2, Method: "tools/call", Params: json.RawMessage(`{"name":"send_to_user","arguments":{"from":"plan","text":"Proceed?"}}`)},
	)

	resp := decodeResponseMap(t, responses[1])
	if resp.Error == nil {
		t.Fatal("resp.Error = nil, want runtime error")
	}
	if resp.Error.Code != errToolExecution {
		t.Fatalf("resp.Error.Code = %d, want %d", resp.Error.Code, errToolExecution)
	}
	if resp.Error.Message != "mcp.handleSendToUser: no user agent registered; start a session with `agentcom up` first" {
		t.Fatalf("resp.Error.Message = %q", resp.Error.Message)
	}
	if _, ok := responses[1]["result"]; ok {
		t.Fatal("runtime error response unexpectedly included result")
	}
}

func TestUnknownToolReturnsJSONRPCMethodNotFound(t *testing.T) {
	server, _ := setupMCPTestServer(t)

	responses := runMCPRoundTripRequests(t, server,
		Request{JSONRPC: jsonRPCVersion, ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, ID: 2, Method: "tools/call", Params: json.RawMessage(`{"name":"no_such_tool","arguments":{}}`)},
	)

	resp := decodeResponseMap(t, responses[1])
	if resp.Error == nil {
		t.Fatal("resp.Error = nil, want method not found error")
	}
	if resp.Error.Code != errMethodNotFound {
		t.Fatalf("resp.Error.Code = %d, want %d", resp.Error.Code, errMethodNotFound)
	}
	if resp.Error.Message != "unknown tool: no_such_tool" {
		t.Fatalf("resp.Error.Message = %q, want %q", resp.Error.Message, "unknown tool: no_such_tool")
	}
	if _, ok := responses[1]["result"]; ok {
		t.Fatal("unknown tool response unexpectedly included result")
	}
}

func TestToolRuntimeErrorReturnsJSONRPCError(t *testing.T) {
	server, _ := setupMCPTestServer(t)
	server.tools["runtime_failure"] = func(context.Context, json.RawMessage) (interface{}, error) {
		return nil, errors.New("boom")
	}

	responses := runMCPRoundTripRequests(t, server,
		Request{JSONRPC: jsonRPCVersion, ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
		Request{JSONRPC: jsonRPCVersion, ID: 2, Method: "tools/call", Params: json.RawMessage(`{"name":"runtime_failure","arguments":{}}`)},
	)

	resp := decodeResponseMap(t, responses[1])
	if resp.Error == nil {
		t.Fatal("resp.Error = nil, want tool execution error")
	}
	if resp.Error.Code != errToolExecution {
		t.Fatalf("resp.Error.Code = %d, want %d", resp.Error.Code, errToolExecution)
	}
	if resp.Error.Message != "boom" {
		t.Fatalf("resp.Error.Message = %q, want boom", resp.Error.Message)
	}
	if _, ok := responses[1]["result"]; ok {
		t.Fatal("runtime failure response unexpectedly included result")
	}
}

func TestMCPBroadcastExcludesHuman(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()

	sender := &db.Agent{Name: "sender", Type: "worker", Project: "project-a", Status: "alive"}
	receiver := &db.Agent{Name: "receiver", Type: "worker", Project: "project-a", Status: "alive"}
	user := &db.Agent{Name: "user", Type: "human", Project: "project-a", Status: "alive"}
	for _, agent := range []*db.Agent{sender, receiver, user} {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	result, err := server.handleBroadcast(ctx, json.RawMessage(`{
		"from":"sender",
		"topic":"sync",
		"payload":{"ok":true}
	}`))
	if err != nil {
		t.Fatalf("handleBroadcast() error = %v", err)
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("handleBroadcast() result type = %T, want map", result)
	}
	if resultMap["recipients"] != 1 {
		t.Fatalf("recipients = %v, want 1", resultMap["recipients"])
	}

	receiverMessages, err := database.ListMessagesForAgent(ctx, receiver.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(receiver) error = %v", err)
	}
	if len(receiverMessages) != 1 {
		t.Fatalf("len(ListMessagesForAgent(receiver)) = %d, want 1", len(receiverMessages))
	}

	userMessages, err := database.ListMessagesForAgent(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(user) error = %v", err)
	}
	if len(userMessages) != 0 {
		t.Fatalf("len(ListMessagesForAgent(user)) = %d, want 0", len(userMessages))
	}
}

func TestSendToUser(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()

	sender := &db.Agent{Name: "plan", Type: "worker", Project: "project-a", Status: "alive"}
	user := &db.Agent{Name: "user", Type: "human", Project: "project-a", Status: "alive"}
	for _, agent := range []*db.Agent{sender, user} {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	result, err := server.handleSendToUser(ctx, json.RawMessage(`{
		"from":"plan",
		"text":"Proceed?",
		"topic":"approval",
		"priority":"high"
	}`))
	if err != nil {
		t.Fatalf("handleSendToUser() error = %v", err)
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("handleSendToUser() result type = %T, want map", result)
	}
	if resultMap["status"] != "delivered_to_inbox" {
		t.Fatalf("status = %v, want delivered_to_inbox", resultMap["status"])
	}

	messages, err := database.ListMessagesForAgent(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(user) error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(ListMessagesForAgent(user)) = %d, want 1", len(messages))
	}
	if messages[0].Type != "request" {
		t.Fatalf("message type = %q, want request", messages[0].Type)
	}
	if !strings.Contains(messages[0].Payload, `"priority":"high"`) {
		t.Fatalf("payload = %s, want priority field", messages[0].Payload)
	}
}

func TestGetUserMessages(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()

	user := &db.Agent{Name: "user", Type: "human", Project: "project-a", Status: "alive"}
	plan := &db.Agent{Name: "plan", Type: "worker", Project: "project-a", Status: "alive"}
	other := &db.Agent{Name: "other", Type: "worker", Project: "project-a", Status: "alive"}
	for _, agent := range []*db.Agent{user, plan, other} {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}
	for _, msg := range []*db.Message{
		{FromAgent: user.ID, ToAgent: plan.ID, Type: "response", Payload: `{"text":"yes"}`, CreatedAt: "2026-03-17 10:00:00"},
		{FromAgent: user.ID, ToAgent: other.ID, Type: "response", Payload: `{"text":"later"}`, CreatedAt: "2026-03-17 10:01:00"},
		{FromAgent: user.ID, ToAgent: plan.ID, Type: "response", Payload: `{"text":"done"}`, CreatedAt: "2026-03-17 10:02:00", ReadAt: "2026-03-17 10:03:00"},
	} {
		if err := database.InsertMessage(ctx, msg); err != nil {
			t.Fatalf("InsertMessage() error = %v", err)
		}
	}

	result, err := server.handleGetUserMessages(ctx, json.RawMessage(`{
		"agent":"plan"
	}`))
	if err != nil {
		t.Fatalf("handleGetUserMessages() error = %v", err)
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("handleGetUserMessages() result type = %T, want map", result)
	}
	if resultMap["count"] != 1 {
		t.Fatalf("count = %v, want 1", resultMap["count"])
	}

	messages, err := database.ListMessagesFromAgent(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListMessagesFromAgent(user) error = %v", err)
	}
	var unreadPlan int
	for _, msg := range messages {
		if msg.ToAgent == plan.ID && msg.ReadAt == "" {
			unreadPlan++
		}
	}
	if unreadPlan != 0 {
		t.Fatalf("unread plan messages = %d, want 0", unreadPlan)
	}

	_, err = server.handleGetUserMessages(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handleGetUserMessages(all) error = %v", err)
	}
}

func TestSendToUserFailsWithoutUserAgent(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()

	sender := &db.Agent{Name: "plan", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, sender); err != nil {
		t.Fatalf("InsertAgent(sender) error = %v", err)
	}

	_, err := server.handleSendToUser(ctx, json.RawMessage(`{"from":"plan","text":"Proceed?"}`))
	if err == nil {
		t.Fatal("handleSendToUser() error = nil, want missing user agent error")
	}
	if err.Error() != "mcp.handleSendToUser: no user agent registered; start a session with `agentcom up` first" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestTaskReviewLifecycleTools(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx := context.Background()
	projectDir := t.TempDir()
	writeMCPReviewPolicyFixture(t, projectDir)
	withMCPWorkingDir(t, projectDir)

	creator := &db.Agent{Name: "creator", Type: "worker", Project: "project-a", Status: "alive"}
	if err := database.InsertAgent(ctx, creator); err != nil {
		t.Fatalf("InsertAgent(creator) error = %v", err)
	}

	createdRaw, err := server.handleCreateTask(ctx, json.RawMessage(`{"title":"needs review","priority":"high","created_by":"creator"}`))
	if err != nil {
		t.Fatalf("handleCreateTask() error = %v", err)
	}
	created := createdRaw.(map[string]interface{})
	taskID, _ := created["task_id"].(string)
	if created["reviewer"] != "user" {
		t.Fatalf("reviewer = %v, want user", created["reviewer"])
	}

	if _, err := server.handleUpdateTask(ctx, json.RawMessage(`{"task_id":"`+taskID+`","status":"in_progress","result":"started"}`)); err != nil {
		t.Fatalf("handleUpdateTask(in_progress) error = %v", err)
	}
	updatedRaw, err := server.handleUpdateTask(ctx, json.RawMessage(`{"task_id":"`+taskID+`","status":"completed","result":"done"}`))
	if err != nil {
		t.Fatalf("handleUpdateTask(completed) error = %v", err)
	}
	updated := updatedRaw.(map[string]interface{})
	if updated["status"] != "blocked" {
		t.Fatalf("status = %v, want blocked", updated["status"])
	}

	approvedRaw, err := server.handleApproveTask(ctx, json.RawMessage(`{"task_id":"`+taskID+`","result":"approved"}`))
	if err != nil {
		t.Fatalf("handleApproveTask() error = %v", err)
	}
	approved := approvedRaw.(map[string]interface{})
	if approved["status"] != "completed" {
		t.Fatalf("status = %v, want completed", approved["status"])
	}

	rejectedTask, err := server.handleCreateTask(ctx, json.RawMessage(`{"title":"reject me","reviewer":"user"}`))
	if err != nil {
		t.Fatalf("handleCreateTask(rejected) error = %v", err)
	}
	rejectedTaskID := rejectedTask.(map[string]interface{})["task_id"].(string)
	if _, err := server.handleUpdateTask(ctx, json.RawMessage(`{"task_id":"`+rejectedTaskID+`","status":"in_progress"}`)); err != nil {
		t.Fatalf("handleUpdateTask(rejected in_progress) error = %v", err)
	}
	if _, err := server.handleUpdateTask(ctx, json.RawMessage(`{"task_id":"`+rejectedTaskID+`","status":"completed"}`)); err != nil {
		t.Fatalf("handleUpdateTask(rejected completed) error = %v", err)
	}
	rejectedRaw, err := server.handleRejectTask(ctx, json.RawMessage(`{"task_id":"`+rejectedTaskID+`","result":"changes requested"}`))
	if err != nil {
		t.Fatalf("handleRejectTask() error = %v", err)
	}
	rejected := rejectedRaw.(map[string]interface{})
	if rejected["status"] != "failed" {
		t.Fatalf("status = %v, want failed", rejected["status"])
	}
}

func writeMCPReviewPolicyFixture(t *testing.T, projectDir string) {
	t.Helper()
	if _, err := config.SaveProjectConfig(projectDir, config.ProjectConfig{Project: "project-a", Template: config.ProjectTemplateConfig{Active: "test-template"}}); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}
	templateDir := filepath.Join(projectDir, ".agentcom", "templates", "test-template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templateDir) error = %v", err)
	}
	manifest := []byte(`{"name":"test-template","review_policy":{"require_review_above":"high","default_reviewer":"user"}}`)
	if err := os.WriteFile(filepath.Join(templateDir, "template.json"), append(manifest, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile(template.json) error = %v", err)
	}
}

func withMCPWorkingDir(t *testing.T, dir string) {
	t.Helper()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func runMCPRoundTripRequests(t *testing.T, server *Server, requests ...Request) []map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	runErr := make(chan error, 1)
	go func() {
		runErr <- server.Run(ctx, inReader, outWriter)
	}()

	enc := json.NewEncoder(inWriter)
	dec := json.NewDecoder(outReader)
	responses := make([]map[string]interface{}, 0, len(requests))
	for _, req := range requests {
		if err := enc.Encode(req); err != nil {
			t.Fatalf("encode %s error = %v", req.Method, err)
		}
		if req.ID == nil {
			continue
		}
		var raw map[string]interface{}
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("decode %s response error = %v", req.Method, err)
		}
		responses = append(responses, raw)
	}

	if err := inWriter.Close(); err != nil {
		t.Fatalf("inWriter.Close() error = %v", err)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("Server.Run() error = %v", err)
	}

	return responses
}

func decodeResponseMap(t *testing.T, raw map[string]interface{}) Response {
	t.Helper()

	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("json.Marshal(raw) error = %v", err)
	}
	var resp Response
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	return resp
}
