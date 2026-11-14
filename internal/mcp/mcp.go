package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
	"github.com/kidthales/resumai/internal/vault"
)

// RenderBundleRequest carries non-PII parameters for materializing an exported application packet.
type RenderBundleRequest struct {
	Application *domain.JobApplication
	Theme       string
	OutputDir   string
}

// RenderBundleResult returns non-PII export metadata and layout telemetry produced by local rendering.
type RenderBundleResult struct {
	Export    domain.ExportRecord     `json:"export"`
	Telemetry *render.LayoutTelemetry `json:"telemetry,omitempty"`
}

// SubmissionRequest carries parameters for locking a submitted application packet.
type SubmissionRequest struct {
	Application *domain.JobApplication
	Notes       string
}

// SubmissionResult returns the immutable SHA-256 hash and generated interview brief path.
type SubmissionResult struct {
	BundleSHA256       string `json:"bundle_sha256"`
	InterviewBriefPath string `json:"interview_brief_path"`
}

// PacketMaterializer renders local PII-injected resume packets and finalizes submission locks
// without ever exposing candidate PII across the MCP boundary.
type PacketMaterializer interface {
	RenderAndMarkExported(ctx context.Context, req RenderBundleRequest) (*RenderBundleResult, error)
	FinalizeSubmission(ctx context.Context, req SubmissionRequest) (*SubmissionResult, error)
}

type toolHandler func(ctx context.Context, args json.RawMessage) (any, error)

type registeredTool struct {
	def     Tool
	handler toolHandler
}

// Server coordinates MCP tool calls against domain logic, air-gapped vault storage,
// and an optional local packet materializer.
type Server struct {
	vault        vault.Vault
	materializer PacketMaterializer
	tools        map[string]registeredTool
	toolOrder    []string
}

// ServerOption configures optional dependencies on an MCP Server.
type ServerOption func(*Server)

// WithPacketMaterializer attaches a local packet materializer for render/submit tools.
func WithPacketMaterializer(m PacketMaterializer) ServerOption {
	return func(s *Server) {
		s.materializer = m
	}
}

// NewServer initializes a new air-gapped MCP Server instance with all tools registered.
func NewServer(v vault.Vault, opts ...ServerOption) *Server {
	s := &Server{
		vault: v,
		tools: make(map[string]registeredTool),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	s.registerVaultTools()
	s.registerApplicationTools()
	return s
}

func (s *Server) registerTool(def Tool, handler toolHandler) {
	if _, exists := s.tools[def.Name]; !exists {
		s.toolOrder = append(s.toolOrder, def.Name)
	}
	s.tools[def.Name] = registeredTool{
		def:     def,
		handler: handler,
	}
}

func (s *Server) hasTool(name string) bool {
	_, ok := s.tools[name]
	return ok
}

// ListTools returns all registered MCP tool definitions in deterministic registration order.
func (s *Server) ListTools() []Tool {
	out := make([]Tool, 0, len(s.toolOrder))
	for _, name := range s.toolOrder {
		out = append(out, s.tools[name].def)
	}
	return out
}

// CallTool executes a registered MCP tool by name and returns an MCP CallToolResult.
// Domain/validation failures are returned as IsError=true payloads with secrets redacted.
func (s *Server) CallTool(ctx context.Context, name string, args json.RawMessage) (*CallToolResult, error) {
	entry, ok := s.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	if s.vault == nil {
		return toolErrorResult(errors.New("vault storage is not initialized")), nil
	}

	payload, err := entry.handler(ctx, args)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(payload)
}

func decodeArgs[T any](raw json.RawMessage) (T, error) {
	var out T
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return out, nil
	}
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return out, fmt.Errorf("invalid tool arguments: %w", err)
	}
	return out, nil
}

func toolJSONResult(v any) (*CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return &CallToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: string(b),
			},
		},
	}, nil
}

func toolErrorResult(err error) *CallToolResult {
	msg := render.RedactSecrets(err.Error())
	return &CallToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}

func schemaObject(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func strProp(desc string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": desc,
	}
}

func boolProp(desc string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": desc,
	}
}

func intProp(desc string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": desc,
	}
}

func strArrayProp(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items": map[string]any{
			"type": "string",
		},
	}
}
