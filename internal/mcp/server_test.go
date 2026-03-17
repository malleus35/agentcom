package mcp

import (
	"context"
	"encoding/json"
	"io"
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
		}
	}
	if !hasSendToUser || !hasGetUserMessages {
		t.Fatalf("tools/list missing user tools: send_to_user=%v get_user_messages=%v", hasSendToUser, hasGetUserMessages)
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
