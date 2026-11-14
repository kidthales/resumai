package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kidthales/resumai/internal/version"
)

const (
	jsonRPCVersion     = "2.0"
	mcpProtocolVersion = "2024-11-05"
)

// Standard JSON-RPC 2.0 error codes.
const (
	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternalError  = -32603
)

// Request represents an incoming JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an outgoing JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error payload.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool describes an MCP tool and its JSON Schema input specification.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// CallToolParams holds the decoded parameters for an MCP tools/call request.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ContentItem represents a single MCP content block in a tool response.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult represents the MCP tools/call response payload.
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ServeStdio runs the newline-delimited JSON-RPC 2.0 MCP loop over r and w until EOF or ctx cancellation.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	if r == nil || w == nil {
		return errors.New("stdio reader and writer are required")
	}

	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			respBytes, hasResp, handleErr := s.HandleLine(ctx, line)
			if handleErr != nil {
				return handleErr
			}
			if hasResp {
				if _, werr := w.Write(append(respBytes, '\n')); werr != nil {
					return fmt.Errorf("write mcp response: %w", werr)
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read mcp stdin: %w", err)
		}
	}
}

// HandleLine processes a single JSON-RPC 2.0 message line and returns the serialized
// JSON response (if the message was a request with an id rather than a notification).
func (s *Server) HandleLine(ctx context.Context, line []byte) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false, nil
	}

	var req Request
	if err := json.Unmarshal(trimmed, &req); err != nil {
		resp := Response{
			JSONRPC: jsonRPCVersion,
			ID:      json.RawMessage("null"),
			Error: &RPCError{
				Code:    errCodeParseError,
				Message: "invalid JSON-RPC payload",
			},
		}
		b, merr := json.Marshal(resp)
		return b, true, merr
	}

	if req.JSONRPC != jsonRPCVersion || strings.TrimSpace(req.Method) == "" {
		if isNotificationID(req.ID) {
			return nil, false, nil
		}
		resp := Response{
			JSONRPC: jsonRPCVersion,
			ID:      normalizeID(req.ID),
			Error: &RPCError{
				Code:    errCodeInvalidRequest,
				Message: "invalid JSON-RPC 2.0 request",
			},
		}
		b, merr := json.Marshal(resp)
		return b, true, merr
	}

	// JSON-RPC notifications omit id and must not receive a response.
	if isNotificationID(req.ID) {
		return nil, false, nil
	}

	resp := s.dispatchRequest(ctx, req)
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, false, fmt.Errorf("marshal mcp response: %w", err)
	}
	return b, true, nil
}

func (s *Server) dispatchRequest(ctx context.Context, req Request) Response {
	id := normalizeID(req.ID)

	switch req.Method {
	case "initialize":
		var initParams struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &initParams)
		}
		proto := mcpProtocolVersion
		if initParams.ProtocolVersion == "2024-11-05" || initParams.ProtocolVersion == "2025-03-26" {
			proto = initParams.ProtocolVersion
		}
		return Response{
			JSONRPC: jsonRPCVersion,
			ID:      id,
			Result: map[string]any{
				"protocolVersion": proto,
				"capabilities": map[string]any{
					"tools": map[string]any{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]any{
					"name":    "resumai",
					"version": version.Version,
				},
			},
		}

	case "ping":
		return Response{
			JSONRPC: jsonRPCVersion,
			ID:      id,
			Result:  map[string]any{},
		}

	case "tools/list":
		return Response{
			JSONRPC: jsonRPCVersion,
			ID:      id,
			Result: map[string]any{
				"tools": s.ListTools(),
			},
		}

	case "tools/call":
		var params CallToolParams
		if len(req.Params) == 0 || json.Unmarshal(req.Params, &params) != nil || strings.TrimSpace(params.Name) == "" {
			return Response{
				JSONRPC: jsonRPCVersion,
				ID:      id,
				Error: &RPCError{
					Code:    errCodeInvalidParams,
					Message: "tools/call requires a non-empty tool name",
				},
			}
		}
		if !s.hasTool(params.Name) {
			return Response{
				JSONRPC: jsonRPCVersion,
				ID:      id,
				Error: &RPCError{
					Code:    errCodeInvalidParams,
					Message: fmt.Sprintf("unknown tool %q", params.Name),
				},
			}
		}
		res, err := s.CallTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return Response{
				JSONRPC: jsonRPCVersion,
				ID:      id,
				Error: &RPCError{
					Code:    errCodeInternalError,
					Message: err.Error(),
				},
			}
		}
		return Response{
			JSONRPC: jsonRPCVersion,
			ID:      id,
			Result:  res,
		}

	default:
		return Response{
			JSONRPC: jsonRPCVersion,
			ID:      id,
			Error: &RPCError{
				Code:    errCodeMethodNotFound,
				Message: fmt.Sprintf("method not found: %q", req.Method),
			},
		}
	}
}

func isNotificationID(id json.RawMessage) bool {
	return len(bytes.TrimSpace(id)) == 0
}

func normalizeID(id json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(id)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	return trimmed
}
