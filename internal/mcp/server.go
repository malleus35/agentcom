package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

const protocolVersion = "2024-11-05"

const (
	jsonRPCVersion    = "2.0"
	errMethodNotFound = -32601
	errInvalidRequest = -32600
	errInvalidParams  = -32602
	errInternalError  = -32603
)

// ToolHandler handles one MCP tool call.
type ToolHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// Server serves MCP JSON-RPC 2.0 over STDIO.
type Server struct {
	db          *db.DB
	cfg         *config.Config
	tools       map[string]ToolHandler
	initialized bool
}

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewServer creates a new MCP server instance.
func NewServer(database *db.DB, cfg *config.Config) *Server {
	s := &Server{
		db:    database,
		cfg:   cfg,
		tools: make(map[string]ToolHandler),
	}
	s.registerTools()
	return s
}

// Run starts the JSON-RPC loop over the given streams.
func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var req Request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("mcp.Server.Run: decode request: %w", err)
		}

		resp := s.routeRequest(ctx, &req)
		if resp == nil {
			continue
		}

		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("mcp.Server.Run: encode response: %w", err)
		}
	}
}

func (s *Server) routeRequest(ctx context.Context, req *Request) *Response {
	if req.JSONRPC != jsonRPCVersion {
		if req.ID == nil {
			return nil
		}
		return newErrorResponse(req.ID, errInvalidRequest, "Invalid Request")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		slog.Debug("mcp initialized notification received")
		return nil
	case "tools/list":
		if !s.initialized {
			return newErrorResponse(req.ID, errInvalidRequest, "Server not initialized")
		}
		return s.handleToolsList(req)
	case "tools/call":
		if !s.initialized {
			return newErrorResponse(req.ID, errInvalidRequest, "Server not initialized")
		}
		return s.handleToolCall(ctx, req)
	default:
		if req.ID == nil {
			return nil
		}
		return newErrorResponse(req.ID, errMethodNotFound, "Method not found")
	}
}

func (s *Server) handleInitialize(req *Request) *Response {
	s.initialized = true

	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "agentcom",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req *Request) *Response {
	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": AllTools(),
		},
	}
}

func (s *Server) handleToolCall(ctx context.Context, req *Request) *Response {
	type toolCallParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	var params toolCallParams
	if len(req.Params) == 0 {
		return newErrorResponse(req.ID, errInvalidParams, "Invalid params")
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return newErrorResponse(req.ID, errInvalidParams, "Invalid params")
	}
	if params.Name == "" {
		return newErrorResponse(req.ID, errInvalidParams, "Invalid params")
	}

	handler, ok := s.tools[params.Name]
	if !ok {
		return &Response{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Result:  newToolResult(fmt.Sprintf("unknown tool: %s", params.Name), true),
		}
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		slog.Debug("mcp tool call failed", "tool", params.Name, "error", err)
		return &Response{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Result:  newToolResult(err.Error(), true),
		}
	}

	b, err := json.Marshal(result)
	if err != nil {
		return newErrorResponse(req.ID, errInternalError, "Internal error")
	}

	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      req.ID,
		Result:  newToolResult(string(b), false),
	}
}

func newErrorResponse(id interface{}, code int, message string) *Response {
	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

func newToolResult(text string, isError bool) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]string{
			{
				"type": "text",
				"text": text,
			},
		},
		"isError": isError,
	}
}
