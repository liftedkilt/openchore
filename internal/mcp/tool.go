package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Tool is one MCP tool.
type Tool struct {
	Name        string
	Title       string
	Description string
	Schema      map[string]any
	// Hints for the client (MCP tool annotations).
	ReadOnly    bool
	Destructive bool
	// Covers lists the REST routes the tool uses, as "METHOD /path" relative
	// to the API prefix with parameters written {}. A test checks that every
	// route in the router is covered by some tool or deliberately excluded.
	Covers []string
	Run    func(ctx context.Context, c *caller, a Args) (any, error)
}

func (t *Tool) describe() map[string]any {
	ann := map[string]any{
		"title":         t.Title,
		"readOnlyHint":  t.ReadOnly,
		"openWorldHint": false,
	}
	if !t.ReadOnly {
		ann["destructiveHint"] = t.Destructive
	}
	return map[string]any{
		"name":        t.Name,
		"title":       t.Title,
		"description": t.Description,
		"inputSchema": t.Schema,
		"annotations": ann,
	}
}

func (t *Tool) required() []string {
	req, _ := t.Schema["required"].([]string)
	return req
}

// --- arguments ---

// Args are a tool call's arguments as decoded from JSON.
type Args map[string]any

func (a Args) Has(k string) bool { _, ok := a[k]; return ok }

func (a Args) Int(k string) (int64, bool) {
	switch v := a[k].(type) {
	case float64:
		return int64(v), v == float64(int64(v))
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	case string:
		var n int64
		_, err := fmt.Sscan(v, &n)
		return n, err == nil
	}
	return 0, false
}

func (a Args) Str(k string) string {
	s, _ := a[k].(string)
	return s
}

func (a Args) Bool(k string) bool {
	b, _ := a[k].(bool)
	return b
}

func (a Args) Ints(k string) ([]int64, error) {
	raw, ok := a[k].([]any)
	if !ok {
		if n, ok := a.Int(k); ok { // tolerate a bare number
			return []int64{n}, nil
		}
		return nil, fmt.Errorf("%s must be a list of ids", k)
	}
	out := make([]int64, 0, len(raw))
	for i := range raw {
		n, ok := Args{"v": raw[i]}.Int("v")
		if !ok {
			return nil, fmt.Errorf("%s must contain whole numbers", k)
		}
		out = append(out, n)
	}
	return out, nil
}

// pick copies the named arguments that are present into a request body.
func (a Args) pick(keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := a[k]; ok {
			out[k] = v
		}
	}
	return out
}

// --- in-process API calls ---

// caller sends requests through the OpenChore router as the MCP client,
// reusing its Authorization header (an API token or a parent's session).
type caller struct {
	api    http.Handler
	prefix string
	orig   *http.Request
}

// APIError is a non-2xx answer from the REST API.
type APIError struct {
	Status int
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("OpenChore API error (%d): %s", e.Status, e.Msg)
}

// do calls the REST API. path is relative to the API prefix ("/chores/3").
// The decoded JSON body is returned; non-JSON bodies come back as a string.
func (c *caller) do(ctx context.Context, method, path string, query url.Values, body any) (any, error) {
	target := c.prefix + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	// Drop the outer request's chi routing state, or the router would keep
	// matching against /api/mcp instead of routing this request afresh.
	ctx = context.WithValue(ctx, chi.RouteCtxKey, (*chi.Context)(nil))
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	req.Host = c.orig.Host
	req.RemoteAddr = c.orig.RemoteAddr
	for _, h := range []string{"Authorization", "Cookie", "X-Forwarded-For", "X-Real-IP", "X-Forwarded-Host", "X-Forwarded-Proto"} {
		if v := c.orig.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	c.api.ServeHTTP(rec, req)

	raw := bytes.TrimSpace(rec.Body.Bytes())
	var decoded any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			decoded = string(raw)
		}
	}
	if rec.Code >= 400 {
		msg := http.StatusText(rec.Code)
		if m, ok := decoded.(map[string]any); ok {
			if e, ok := m["error"].(string); ok && e != "" {
				msg = e
			}
		} else if s, ok := decoded.(string); ok && s != "" {
			msg = s
		}
		return nil, &APIError{Status: rec.Code, Msg: msg}
	}
	if decoded == nil {
		return map[string]any{"ok": true, "status": rec.Code}, nil
	}
	return decoded, nil
}

// getInto calls GET and decodes into v.
func (c *caller) getInto(ctx context.Context, path string, query url.Values, v any) error {
	out, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(out)
	return json.Unmarshal(b, v)
}

// --- passthrough tools ---

var pathParamRe = regexp.MustCompile(`\{([a-z_]+)\}`)

// endpoint describes a tool that maps one-to-one onto a REST endpoint.
// Path parameters ({id}) and query keys are taken from the arguments; every
// other argument is sent as the JSON body (for non-GET requests).
type endpoint struct {
	name, title, desc string
	method, path      string
	query             []string
	props             map[string]any
	required          []string
	readOnly          bool
	destructive       bool
}

func (e endpoint) tool() *Tool {
	params := pathParamRe.FindAllStringSubmatch(e.path, -1)
	required := append([]string{}, e.required...)
	for _, p := range params {
		if !contains(required, p[1]) {
			required = append(required, p[1])
		}
	}
	props := e.props
	if props == nil {
		props = map[string]any{}
	}
	return &Tool{
		Name:        e.name,
		Title:       e.title,
		Description: e.desc,
		Schema:      obj(props, required...),
		ReadOnly:    e.readOnly,
		Destructive: e.destructive,
		Covers:      []string{e.method + " " + pathParamRe.ReplaceAllString(e.path, "{}")},
		Run: func(ctx context.Context, c *caller, a Args) (any, error) {
			path := e.path
			used := map[string]bool{}
			for _, p := range params {
				v, ok := a[p[1]]
				if !ok {
					return nil, fmt.Errorf("missing required argument: %s", p[1])
				}
				path = strings.Replace(path, p[0], url.PathEscape(fmt.Sprint(v)), 1)
				used[p[1]] = true
			}
			q := url.Values{}
			for _, k := range e.query {
				if v, ok := a[k]; ok && v != nil && fmt.Sprint(v) != "" {
					q.Set(k, scalar(v))
				}
				used[k] = true
			}
			var body any
			if e.method != http.MethodGet {
				b := map[string]any{}
				for k, v := range a {
					if !used[k] {
						b[k] = v
					}
				}
				body = b
			}
			return c.do(ctx, e.method, path, q, body)
		},
	}
}

// scalar renders a JSON value for a query string (whole numbers without ".0").
func scalar(v any) string {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return fmt.Sprint(int64(f))
	}
	return fmt.Sprint(v)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- JSON Schema helpers ---

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		sort.Strings(required)
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func number(desc string) map[string]any {
	return map[string]any{"type": "number", "description": desc}
}
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "enum": vals, "description": desc}
}
func ids(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": desc}
}

// nullable allows null as well (to clear a value).
func nullable(s map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range s {
		out[k] = v
	}
	out["type"] = []string{fmt.Sprint(s["type"]), "null"}
	return out
}

// merge combines property maps.
func merge(maps ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
