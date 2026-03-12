package mcp

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
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

	return NewServer(database, cfg), database
}

func TestServerRunJSONRPCRoundTrip(t *testing.T) {
	server, database := setupMCPTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender := &db.Agent{Name: "sender", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, sender); err != nil {
		t.Fatalf("InsertAgent(sender) error = %v", err)
	}
	receiver := &db.Agent{Name: "receiver", Type: "worker", Status: "alive"}
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
