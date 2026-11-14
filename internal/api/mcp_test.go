package api_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/liftedkilt/openchore/internal/mcp"
)

// --- MCP helpers ---

func (e *testEnv) mcpRPC(t *testing.T, headers map[string]string, method string, params any) map[string]any {
	t.Helper()
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	resp := e.request(t, http.MethodPost, "/api/mcp", body, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", method, resp.StatusCode)
	}
	var out map[string]any
	decodeBody(t, resp, &out)
	if out["error"] != nil {
		t.Fatalf("%s: rpc error %v", method, out["error"])
	}
	return out["result"].(map[string]any)
}

// callTool runs a tool and returns its decoded JSON output and isError.
func (e *testEnv) callTool(t *testing.T, headers map[string]string, name string, args map[string]any) (any, bool) {
	t.Helper()
	res := e.mcpRPC(t, headers, "tools/call", map[string]any{"name": name, "arguments": args})
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	isErr, _ := res["isError"].(bool)
	if isErr {
		return text, true
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("%s: result is not JSON: %s", name, text)
	}
	return v, false
}

func (e *testEnv) mustTool(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	v, isErr := e.callTool(t, adminHeaders(), name, args)
	if isErr {
		t.Fatalf("%s failed: %v", name, v)
	}
	m, _ := v.(map[string]any)
	return m
}

func setupMCP(t *testing.T) *testEnv {
	t.Helper()
	env := setupTest(t)
	env.createAdmin(t)
	return env
}

// --- protocol ---

func TestMCPInitializeAndList(t *testing.T) {
	env := setupMCP(t)

	init := env.mcpRPC(t, adminHeaders(), "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	})
	if init["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want the client's", init["protocolVersion"])
	}
	if _, ok := init["capabilities"].(map[string]any)["tools"]; !ok {
		t.Error("tools capability not advertised")
	}
	if s, _ := init["instructions"].(string); !strings.Contains(s, "household_overview") {
		t.Error("instructions missing")
	}

	unknown := env.mcpRPC(t, adminHeaders(), "initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if unknown["protocolVersion"] != "2025-11-25" {
		t.Errorf("unsupported version should fall back to newest, got %v", unknown["protocolVersion"])
	}

	list := env.mcpRPC(t, adminHeaders(), "tools/list", nil)
	tools := list["tools"].([]any)
	if len(tools) < 60 {
		t.Fatalf("only %d tools", len(tools))
	}
	nameRe := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	for _, raw := range tools {
		tool := raw.(map[string]any)
		name, _ := tool["name"].(string)
		if !nameRe.MatchString(name) {
			t.Errorf("bad tool name %q", name)
		}
		if d, _ := tool["description"].(string); d == "" {
			t.Errorf("%s has no description", name)
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("%s schema type = %v", name, schema["type"])
		}
		props, _ := schema["properties"].(map[string]any)
		req, _ := schema["required"].([]any)
		for _, r := range req {
			if _, ok := props[r.(string)]; !ok {
				t.Errorf("%s requires undeclared %v", name, r)
			}
		}
	}

	if r := env.mcpRPC(t, adminHeaders(), "ping", nil); len(r) != 0 {
		t.Errorf("ping = %v", r)
	}
}

func TestMCPTransport(t *testing.T) {
	env := setupMCP(t)

	// Notifications get 202 and no body.
	resp := env.request(t, http.MethodPost, "/api/mcp",
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}, adminHeaders())
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("notification status = %d, want 202", resp.StatusCode)
	}
	resp.Body.Close()

	// No SSE stream: GET is refused.
	resp = env.request(t, http.MethodGet, "/api/mcp", nil, adminHeaders())
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", resp.StatusCode)
	}
	resp.Body.Close()

	// Unknown methods are JSON-RPC errors.
	resp = env.request(t, http.MethodPost, "/api/mcp",
		map[string]any{"jsonrpc": "2.0", "id": 7, "method": "resources/list"}, adminHeaders())
	var out map[string]any
	decodeBody(t, resp, &out)
	if e, _ := out["error"].(map[string]any); e == nil || e["code"].(float64) != -32601 {
		t.Errorf("unknown method: %v", out)
	}
	if out["id"].(float64) != 7 {
		t.Errorf("id not echoed: %v", out["id"])
	}

	// Batches are answered in order.
	resp = env.request(t, http.MethodPost, "/api/mcp", []any{
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"},
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "ping"},
	}, adminHeaders())
	var batch []map[string]any
	decodeBody(t, resp, &batch)
	if len(batch) != 2 {
		t.Errorf("batch answered %d, want 2", len(batch))
	}
}

func TestMCPRequiresParent(t *testing.T) {
	env := setupMCP(t)
	ping := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}

	resp := env.request(t, http.MethodPost, "/api/mcp", ping, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	kid := env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	resp = env.request(t, http.MethodPost, "/api/mcp", ping, childHeaders(int(kid["id"].(float64))))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("child status = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// An API token (how the agent gateway connects) works, and tools act as it.
	resp = env.request(t, http.MethodPost, "/api/admin/tokens", map[string]any{"name": "claude"}, adminHeaders())
	var tok map[string]any
	decodeBody(t, resp, &tok)
	tokenHeaders := map[string]string{"Authorization": "Bearer " + tok["token"].(string)}
	out, isErr := env.callTool(t, tokenHeaders, "list_people", nil)
	if isErr || len(out.([]any)) != 2 {
		t.Errorf("list_people via token = %v (error %v)", out, isErr)
	}
	me, _ := env.callTool(t, tokenHeaders, "whoami", nil)
	if !strings.Contains(toJSON(me), "API token") {
		t.Errorf("whoami via token = %v", me)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// --- tools ---

func TestMCPToolErrorsAreResults(t *testing.T) {
	env := setupMCP(t)

	msg, isErr := env.callTool(t, adminHeaders(), "create_chore", map[string]any{"title": "X", "category": "sometimes"})
	if !isErr || !strings.Contains(msg.(string), "category must be") {
		t.Errorf("bad category: isError=%v %v", isErr, msg)
	}
	msg, isErr = env.callTool(t, adminHeaders(), "get_chore", map[string]any{})
	if !isErr || !strings.Contains(msg.(string), "missing required argument: id") {
		t.Errorf("missing arg: %v", msg)
	}
	msg, isErr = env.callTool(t, adminHeaders(), "get_chore", map[string]any{"id": 999})
	if !isErr || !strings.Contains(msg.(string), "404") {
		t.Errorf("not found: %v", msg)
	}

	// An unknown tool is a protocol error, not a result.
	resp := env.request(t, http.MethodPost, "/api/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "nope"},
	}, adminHeaders())
	var out map[string]any
	decodeBody(t, resp, &out)
	if out["error"] == nil {
		t.Errorf("unknown tool should be an rpc error: %v", out)
	}
}

func TestMCPChoreLifecycle(t *testing.T) {
	env := setupMCP(t)
	zoe := env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child", "age": 8})
	nat := env.mustTool(t, "create_person", map[string]any{"name": "Natalie", "role": "child", "age": 13})
	zoeID, natID := zoe["id"].(float64), nat["id"].(float64)

	// Create and assign in one call: two kids x weekdays = 10 schedules.
	out := env.mustTool(t, "create_chore", map[string]any{
		"title": "Unload dishwasher", "category": "core", "icon": "🍽️", "points_value": 5,
		"requires_approval": true,
		"user_ids":          []any{zoeID, natID}, "days": []any{"weekdays"}, "due_by": "09:00",
	})
	chore := out["chore"].(map[string]any)
	choreID := chore["id"].(float64)
	if n := out["assignment"].(map[string]any)["created"].(float64); n != 10 {
		t.Fatalf("created %v schedules, want 10", n)
	}

	// The overview groups the ten rows into one line per kid.
	ov := env.mustTool(t, "household_overview", nil)
	chores := ov["chores"].([]any)
	if len(chores) != 1 {
		t.Fatalf("overview chores = %d", len(chores))
	}
	assigned := chores[0].(map[string]any)["assigned"].([]any)
	if len(assigned) != 2 {
		t.Fatalf("assigned groups = %v", assigned)
	}
	for _, g := range assigned {
		g := g.(map[string]any)
		if g["when"] != "weekdays" || g["due_by"] != "09:00" || g["if_late"] != "can't be done late" {
			t.Errorf("group = %v", g)
		}
		if len(g["schedule_ids"].([]any)) != 5 {
			t.Errorf("group schedule ids = %v", g["schedule_ids"])
		}
	}
	if cat := chores[0].(map[string]any)["category"]; cat != "Every day (core)" {
		t.Errorf("category label = %v", cat)
	}

	// A partial update keeps the settings it didn't mention.
	upd := env.mustTool(t, "update_chore", map[string]any{"id": choreID, "points_value": 8})
	if upd["points_value"].(float64) != 8 || upd["requires_approval"] != true || upd["title"] != "Unload dishwasher" {
		t.Errorf("update_chore lost fields: %v", upd)
	}

	// Retire Natalie's assignment: it ends yesterday, history is kept and
	// it disappears from today's view and the default overview.
	res := env.mustTool(t, "unassign_chore", map[string]any{"chore_id": choreID, "user_ids": []any{natID}})
	if len(res["retired"].([]any)) != 5 || len(res["deleted"].([]any)) != 0 {
		t.Fatalf("unassign = %v", res)
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	scheds, _ := env.callTool(t, adminHeaders(), "list_schedules", map[string]any{"chore_id": choreID})
	ended := 0
	for _, s := range scheds.([]any) {
		s := s.(map[string]any)
		if s["assigned_to"].(float64) == natID {
			if d, _ := s["end_date"].(string); strings.HasPrefix(d, yesterday) {
				ended++
			}
		}
	}
	if ended != 5 {
		t.Errorf("%d of Natalie's schedules end yesterday, want 5", ended)
	}
	ov = env.mustTool(t, "household_overview", nil)
	if a := ov["chores"].([]any)[0].(map[string]any)["assigned"].([]any); len(a) != 1 {
		t.Errorf("after retire, assigned = %v", a)
	}
	ov = env.mustTool(t, "household_overview", map[string]any{"include_ended": true})
	if a := ov["chores"].([]any)[0].(map[string]any)["assigned"].([]any); len(a) != 2 {
		t.Errorf("include_ended assigned = %v", a)
	}
	todays, _ := env.callTool(t, adminHeaders(), "get_person_chores", map[string]any{"id": natID})
	if len(todays.([]any)) != 0 {
		t.Errorf("Natalie still has today's chores: %v", todays)
	}

	// Natalie takes it on every 3 days instead, from tomorrow; retiring that
	// removes it outright because it never came up.
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	asg := env.mustTool(t, "assign_chore", map[string]any{
		"chore_id": choreID, "user_ids": []any{natID}, "recurrence_interval": 3, "recurrence_start": tomorrow, "start_date": tomorrow,
	})
	if asg["created"].(float64) != 1 {
		t.Fatalf("assign interval = %v", asg)
	}
	res = env.mustTool(t, "unassign_chore", map[string]any{"chore_id": choreID, "user_ids": []any{natID}})
	if len(res["deleted"].([]any)) != 1 {
		t.Errorf("future schedule should be deleted: %v", res)
	}

	// Patterns are validated before anything is created.
	msg, isErr := env.callTool(t, adminHeaders(), "assign_chore", map[string]any{"chore_id": choreID, "user_ids": []any{zoeID}})
	if !isErr || !strings.Contains(msg.(string), "exactly one of") {
		t.Errorf("no pattern: %v", msg)
	}
	msg, isErr = env.callTool(t, adminHeaders(), "assign_chore", map[string]any{"chore_id": choreID, "user_ids": []any{zoeID}, "days": []any{"funday"}})
	if !isErr || !strings.Contains(msg.(string), "unknown day") {
		t.Errorf("bad day: %v", msg)
	}

	// A kid completes through the tool and it waits for approval.
	day := time.Now().Weekday()
	if day != time.Saturday && day != time.Sunday {
		todays, _ = env.callTool(t, adminHeaders(), "get_person_chores", map[string]any{"id": zoeID})
		sid := todays.([]any)[0].(map[string]any)["schedule_id"]
		env.mustTool(t, "complete_chore", map[string]any{"schedule_id": sid})
		pending, _ := env.callTool(t, adminHeaders(), "list_pending_approvals", nil)
		if len(pending.([]any)) != 1 {
			t.Errorf("pending = %v", pending)
		}
	}
}

func TestMCPOverviewCleanupHints(t *testing.T) {
	env := setupMCP(t)
	kid := env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	env.mustTool(t, "create_chore", map[string]any{"title": "Feed the cats", "user_ids": []any{kid["id"]}, "days": []any{"daily"}})
	env.mustTool(t, "create_chore", map[string]any{"title": "feed the cat!"})
	env.mustTool(t, "create_chore", map[string]any{"title": "Water plants", "user_ids": []any{kid["id"]}, "days": []any{"mon", "thu"}})

	ov := env.mustTool(t, "household_overview", nil)
	cleanup := ov["cleanup"].(map[string]any)
	if u := cleanup["not_assigned_to_anyone"].([]any); len(u) != 1 || !strings.HasPrefix(u[0].(string), "feed the cat!") {
		t.Errorf("unassigned = %v", u)
	}
	if d := cleanup["possible_duplicates"].([]any); len(d) != 1 || len(d[0].([]any)) != 2 {
		t.Errorf("duplicates = %v", d)
	}
	for _, c := range ov["chores"].([]any) {
		c := c.(map[string]any)
		if c["title"] == "Feed the cats" && c["assigned"].([]any)[0].(map[string]any)["when"] != "every day" {
			t.Errorf("daily = %v", c["assigned"])
		}
		if c["title"] == "Water plants" && c["assigned"].([]any)[0].(map[string]any)["when"] != "Mon, Thu" {
			t.Errorf("mon/thu = %v", c["assigned"])
		}
	}

	// person_id narrows it to one person's chores.
	ov = env.mustTool(t, "household_overview", map[string]any{"person_id": 1})
	if c := ov["chores"].([]any); len(c) != 0 {
		t.Errorf("admin has no chores, got %v", c)
	}
}

func TestMCPTriggerFCFS(t *testing.T) {
	env := setupMCP(t)
	env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	env.mustTool(t, "create_person", map[string]any{"name": "Natalie", "role": "child"})
	ch := env.mustTool(t, "create_chore", map[string]any{"title": "Fold laundry", "icon": "🧺", "points_value": 10})["chore"].(map[string]any)

	trig := env.mustTool(t, "create_trigger", map[string]any{"chore_id": ch["id"], "assignment_type": "fcfs", "default_due_by": "20:00"})
	if _, isErr := env.callTool(t, adminHeaders(), "fire_trigger", map[string]any{"uuid": trig["uuid"]}); isErr {
		t.Fatal("fire_trigger failed")
	}
	ov := env.mustTool(t, "household_overview", nil)
	c := ov["chores"].([]any)[0].(map[string]any)
	if len(c["assigned"].([]any)) != 2 || len(c["triggers"].([]any)) != 1 {
		t.Errorf("triggered chore = %v", c)
	}
	if src := c["assigned"].([]any)[0].(map[string]any)["source"]; src != "trigger, first come first served" {
		t.Errorf("source = %v", src)
	}
}

func TestMCPUpdatePerson(t *testing.T) {
	env := setupMCP(t)
	kid := env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	env.mustTool(t, "update_person", map[string]any{"id": kid["id"], "name": "Zoë", "age": 8, "color": "sky"})
	got := env.mustTool(t, "get_person", map[string]any{"id": kid["id"]})
	if got["name"] != "Zoë" || got["age"].(float64) != 8 || got["color"] != "sky" || got["role"] != "child" {
		t.Errorf("person = %v", got)
	}
}

func TestMCPToolAnnotations(t *testing.T) {
	env := setupMCP(t)
	list := env.mcpRPC(t, adminHeaders(), "tools/list", nil)
	ann := map[string]map[string]any{}
	for _, raw := range list["tools"].([]any) {
		tool := raw.(map[string]any)
		ann[tool["name"].(string)], _ = tool["annotations"].(map[string]any)
	}

	// Writes are destructive unless explicitly marked safe; these in
	// particular must never look auto-approvable to a client or gateway.
	sensitive := []string{
		"create_api_token", "create_webhook", "update_webhook", "set_setting",
		"update_ai_config", "add_sign_in_provider", "update_sign_in_provider",
		"set_sign_in_options", "set_session_lengths", "adjust_points", "set_decay_settings",
	}
	for name := range ann {
		if strings.HasPrefix(name, "delete_") {
			sensitive = append(sensitive, name)
		}
	}
	for _, name := range sensitive {
		a, ok := ann[name]
		if !ok {
			t.Errorf("%s: no such tool", name)
			continue
		}
		if a["readOnlyHint"] != false || a["destructiveHint"] != true {
			t.Errorf("%s annotations = %v, want readOnlyHint false, destructiveHint true", name, a)
		}
	}

	for name, a := range ann {
		switch {
		case a["readOnlyHint"] == true:
			if _, ok := a["destructiveHint"]; ok {
				t.Errorf("%s is read-only but has destructiveHint", name)
			}
		case a["destructiveHint"] == false:
			if name != "complete_chore" {
				t.Errorf("%s is marked non-destructive; only complete_chore opts in", name)
			}
		case a["destructiveHint"] != true:
			t.Errorf("%s: write tool without destructiveHint: %v", name, a)
		}
	}
	for _, name := range []string{"list_people", "household_overview", "export_config", "get_setting"} {
		if ann[name]["readOnlyHint"] != true {
			t.Errorf("%s should be read-only: %v", name, ann[name])
		}
	}
}

// TestMCPCoversAPI keeps the MCP server complete: every REST route must be
// used by some tool or listed here with the reason it isn't.
func TestMCPCoversAPI(t *testing.T) {
	notTools := map[string]string{
		// Signing in is a browser flow; the MCP connection uses an API token.
		"POST /auth/login": "sign-in", "POST /auth/logout": "sign-in", "POST /auth/logout-everywhere": "sign-in",
		"GET /auth/providers": "sign-in", "GET /auth/options": "sign-in",
		"GET /auth/oidc/{}/start": "sign-in", "GET /auth/oidc/{}/callback": "sign-in",
		"POST /auth/upload-link": "QR photo upload for a kid's device",
		"POST /setup":            "first-run setup",
		// Binary uploads can't travel through tool arguments.
		"POST /upload": "photo upload", "PUT /completions/{}/photo": "photo upload", "POST /admin/ai/test": "photo upload",
		// These act as the signed-in kid; an API token has no person behind it.
		"POST /rewards/{}/redeem": "kid action", "POST /rewards/{}/commit": "kid action",
		"POST /commitments/{}/contribute": "kid action", "PUT /commitments/{}/auto-contribute": "kid action",
		"DELETE /commitments/{}": "kid action", "GET /rewards": "kid's reward list; list_rewards shows all",
		// Self-service copies of fields update_person sets.
		"PUT /users/{}/theme": "update_person", "PUT /users/{}/avatar": "update_person", "PUT /users/{}/color": "update_person",
		"PUT /users/{}/line-color": "only settable by that person",
		// The MCP endpoint itself.
		"POST /mcp": "self", "GET /mcp": "self", "PUT /mcp": "self", "DELETE /mcp": "self", "PATCH /mcp": "self",
		"HEAD /mcp": "self", "OPTIONS /mcp": "self", "CONNECT /mcp": "self", "TRACE /mcp": "self", "QUERY /mcp": "self",
	}

	covered := map[string]bool{}
	for _, tool := range mcp.NewHandler(nil, "/api").Tools() {
		if len(tool.Covers) == 0 {
			t.Errorf("tool %s declares no routes", tool.Name)
		}
		for _, c := range tool.Covers {
			covered[c] = true
		}
	}

	env := setupTest(t)
	routes := env.server.Config.Handler.(chi.Routes)
	param := regexp.MustCompile(`\{[^}]+\}`)
	var missing, stale []string
	seen := map[string]bool{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/") {
			return nil // /uploads/*, /tts/* static files
		}
		key := method + " " + param.ReplaceAllString(strings.TrimPrefix(route, "/api"), "{}")
		seen[key] = true
		if !covered[key] && notTools[key] == "" {
			missing = append(missing, key)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for c := range covered {
		if !seen[c] {
			stale = append(stale, c)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	for _, m := range missing {
		t.Errorf("route %s has no MCP tool (add one, or list it in notTools with a reason)", m)
	}
	for _, s := range stale {
		t.Errorf("a tool covers %s, which isn't a route", s)
	}
}

// --- schedule update endpoint ---

func TestUpdateSchedule(t *testing.T) {
	env := setupMCP(t)
	kid := env.mustTool(t, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	out := env.mustTool(t, "create_chore", map[string]any{
		"title": "Read", "user_ids": []any{kid["id"]}, "days": []any{"mon"}, "available_at": "15:00", "due_by": "19:00",
	})
	sched := out["assignment"].(map[string]any)["schedules"].([]any)[0].(map[string]any)
	path := "/api/schedules/" + toJSON(sched["id"])

	put := func(body map[string]any) (int, map[string]any) {
		resp := env.request(t, http.MethodPut, path, body, adminHeaders())
		var m map[string]any
		decodeBody(t, resp, &m)
		return resp.StatusCode, m
	}

	// Omitted fields are kept, null clears.
	code, got := put(map[string]any{"due_by": "20:30", "available_at": nil, "expiry_penalty": "penalty", "expiry_penalty_value": 3})
	if code != 200 || got["due_by"] != "20:30" || got["available_at"] != nil || got["expiry_penalty"] != "penalty" {
		t.Errorf("update = %d %v", code, got)
	}
	code, got = put(map[string]any{"end_date": "2030-01-31"})
	if code != 200 || got["due_by"] != "20:30" || got["end_date"] != "2030-01-31" {
		t.Errorf("end_date = %d %v", code, got)
	}

	for name, body := range map[string]map[string]any{
		"assignee is fixed":   {"assigned_to": 1},
		"days are fixed":      {"day_of_week": 2},
		"bad time":            {"due_by": "7pm"},
		"bad date":            {"end_date": "31/01/2030"},
		"end before start":    {"start_date": "2030-02-01"},
		"penalty needs value": {"expiry_penalty": "penalty", "expiry_penalty_value": 0},
		"bad mode":            {"expiry_penalty": "sometimes"},
		"bad multiplier":      {"points_multiplier": 0},
	} {
		if code, _ := put(body); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, code)
		}
	}

	resp := env.request(t, http.MethodPut, "/api/schedules/9999", map[string]any{"due_by": "10:00"}, adminHeaders())
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing schedule: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = env.request(t, http.MethodPut, path, map[string]any{"due_by": "10:00"}, childHeaders(int(kid["id"].(float64))))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("child can edit schedules: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// API tokens have no profile behind them (user id 0), but chores.created_by,
// rewards.created_by and completions.approved_by reference users. With
// foreign keys enforced, token writes must be recorded against a real parent.
func TestAPITokenWritesRecordAParent(t *testing.T) {
	env := setupMCP(t)
	resp := env.request(t, http.MethodPost, "/api/admin/tokens", map[string]any{"name": "gateway"}, adminHeaders())
	var tok map[string]any
	decodeBody(t, resp, &tok)
	token := map[string]string{"Authorization": "Bearer " + tok["token"].(string)}

	var fk int
	env.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	if fk != 1 {
		t.Fatalf("test DB should enforce foreign keys like production (got %d)", fk)
	}

	kid, isErr := env.callTool(t, token, "create_person", map[string]any{"name": "Zoe", "role": "child"})
	if isErr {
		t.Fatalf("create_person: %v", kid)
	}
	kidID := kid.(map[string]any)["id"]

	out, isErr := env.callTool(t, token, "create_chore", map[string]any{
		"title": "Tidy", "requires_approval": true, "user_ids": []any{kidID}, "specific_date": time.Now().Format("2006-01-02"),
	})
	if isErr {
		t.Fatalf("create_chore via token: %v", out)
	}
	chore := out.(map[string]any)["chore"].(map[string]any)
	if chore["created_by"].(float64) != 1 {
		t.Errorf("created_by = %v, want the parent (1)", chore["created_by"])
	}
	sid := out.(map[string]any)["assignment"].(map[string]any)["schedules"].([]any)[0].(map[string]any)["id"]

	if r, isErr := env.callTool(t, token, "create_reward", map[string]any{"name": "Ice cream", "cost": 20}); isErr {
		t.Errorf("create_reward via token: %v", r)
	}

	// The kid ticks it off, so it waits for a parent.
	resp = env.request(t, http.MethodPost, "/api/schedules/"+toJSON(sid)+"/complete", map[string]any{}, childHeaders(int(kidID.(float64))))
	if resp.StatusCode >= 300 {
		t.Fatalf("kid complete: %d", resp.StatusCode)
	}
	resp.Body.Close()
	pending, _ := env.callTool(t, token, "list_pending_approvals", nil)
	cid := pending.([]any)[0].(map[string]any)["id"]
	if r, isErr := env.callTool(t, token, "approve_completion", map[string]any{"completion_id": cid}); isErr {
		t.Errorf("approve via token: %v", r)
	}
	var approvedBy int64
	env.db.QueryRow(`SELECT approved_by FROM chore_completions WHERE id = ?`, cid).Scan(&approvedBy)
	if approvedBy != 1 {
		t.Errorf("approved_by = %d, want 1", approvedBy)
	}

	// A parent completing through the token is approved on the spot.
	other, _ := env.callTool(t, token, "create_chore", map[string]any{
		"title": "Sweep", "user_ids": []any{kidID}, "specific_date": time.Now().Format("2006-01-02"),
	})
	sid2 := other.(map[string]any)["assignment"].(map[string]any)["schedules"].([]any)[0].(map[string]any)["id"]
	if r, isErr := env.callTool(t, token, "complete_chore", map[string]any{"schedule_id": sid2}); isErr {
		t.Errorf("complete via token: %v", r)
	}
	// ...and undoing then redoing revives that approval.
	if r, isErr := env.callTool(t, token, "uncomplete_chore", map[string]any{"schedule_id": sid2}); isErr {
		t.Errorf("uncomplete via token: %v", r)
	}
	if r, isErr := env.callTool(t, token, "complete_chore", map[string]any{"schedule_id": sid2}); isErr {
		t.Errorf("re-complete via token: %v", r)
	}

	if r, isErr := env.callTool(t, token, "excuse_chore", map[string]any{"schedule_id": sid, "date": time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "reason": "sick"}); isErr {
		t.Errorf("excuse via token: %v", r)
	}
}
