// Package mcp serves OpenChore to AI assistants over the Model Context
// Protocol (streamable HTTP transport, stateless JSON responses).
//
// Every tool runs by calling OpenChore's own REST API in-process with the
// caller's credentials, so validation, points bookkeeping, webhooks and
// permissions are exactly what the web app gets. The package does not
// import internal/api; the router hands itself in as an http.Handler.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Version is reported in serverInfo.
const Version = "1.0.0"

// supportedVersions are the MCP protocol revisions this server speaks,
// newest first. Nothing here depends on revision-specific features beyond
// tools, so any of them works.
var supportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const maxBodyBytes = 4 << 20

// Handler is the /api/mcp endpoint.
type Handler struct {
	api    http.Handler
	prefix string
	tools  []*Tool
	byName map[string]*Tool
}

// NewHandler builds the MCP endpoint. api is the full OpenChore router and
// prefix the path its REST API lives under (normally "/api").
func NewHandler(api http.Handler, prefix string) *Handler {
	h := &Handler{api: api, prefix: prefix, byName: map[string]*Tool{}}
	for _, t := range allTools() {
		if _, dup := h.byName[t.Name]; dup {
			panic("mcp: duplicate tool " + t.Name)
		}
		h.tools = append(h.tools, t)
		h.byName[t.Name] = t
	}
	return h
}

// Tools returns the tool catalog (used by tests and docs).
func (h *Handler) Tools() []*Tool { return h.tools }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// Stateless server: no SSE stream to open and no session to end.
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "could not read body"}})
		return
	}
	body = bytes.TrimSpace(body)

	call := &caller{api: h.api, prefix: h.prefix, orig: r}

	// Batches were dropped from the spec in 2025-06-18 but older clients
	// may still send them; answering costs nothing.
	if len(body) > 0 && body[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil || len(batch) == 0 {
			writeJSON(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "invalid JSON"}})
			return
		}
		var out []rpcResponse
		for _, raw := range batch {
			if resp := h.handle(r.Context(), call, raw); resp != nil {
				out = append(out, *resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	resp := h.handle(r.Context(), call, body)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted) // notification
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handle answers one JSON-RPC message; nil means it was a notification.
func (h *Handler) handle(ctx context.Context, call *caller, raw json.RawMessage) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "invalid JSON"}}
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil // notifications/initialized, notifications/cancelled, ...
	}
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if req.JSONRPC != "2.0" || req.Method == "" {
		resp.Error = &rpcError{codeInvalidRequest, "expected a JSON-RPC 2.0 request"}
		return resp
	}

	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := supportedVersions[0]
		for _, v := range supportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		resp.Result = map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "openchore", "title": "OpenChore", "version": Version},
			"instructions":    instructions,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		list := make([]map[string]any, 0, len(h.tools))
		for _, t := range h.tools {
			list = append(list, t.describe())
		}
		resp.Result = map[string]any{"tools": list}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			resp.Error = &rpcError{codeInvalidParams, "invalid params"}
			return resp
		}
		t := h.byName[p.Name]
		if t == nil {
			resp.Error = &rpcError{codeInvalidParams, "unknown tool: " + p.Name}
			return resp
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		resp.Result = h.runTool(ctx, call, t, p.Arguments)
	default:
		resp.Error = &rpcError{codeMethodNotFound, "method not found: " + req.Method}
	}
	return resp
}

// runTool executes a tool and shapes the outcome as a CallToolResult. Tool
// failures (bad arguments, API errors) are results with isError set, so the
// model can read the message and correct itself.
func (h *Handler) runTool(ctx context.Context, call *caller, t *Tool, args map[string]any) (result map[string]any) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("mcp: tool %s panicked: %v", t.Name, rec)
			result = errorResult(fmt.Sprintf("internal error in %s", t.Name))
		}
	}()
	a := Args(args)
	for _, req := range t.required() {
		if !a.Has(req) {
			return errorResult("missing required argument: " + req)
		}
	}
	out, err := t.Run(ctx, call, a)
	if err != nil {
		return errorResult(err.Error())
	}
	text, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return errorResult("could not encode result")
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": false,
	}
}

func errorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": msg}},
		"isError": true,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
