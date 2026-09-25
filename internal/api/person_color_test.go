package api_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/migrations"
)

// --- Skins (users.theme) ---

func TestThemeAcceptsSkinsOnly(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	path := fmt.Sprintf("/api/users/%d/theme", kid)

	for _, theme := range []string{"sunroom", "blocks", "tint"} {
		resp := env.expectStatus(t, "PUT", path, map[string]any{"theme": theme}, childHeaders(kid), http.StatusOK)
		var user map[string]any
		decodeBody(t, resp, &user)
		if user["theme"] != theme {
			t.Fatalf("expected theme %q, got %v", theme, user["theme"])
		}
	}
	for _, legacy := range []string{"default", "quest", "galaxy", "forest", "house", ""} {
		env.expectStatus(t, "PUT", path, map[string]any{"theme": legacy}, childHeaders(kid), http.StatusBadRequest)
	}
}

func TestAdminCreateAndUpdateValidateTheme(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/users", map[string]any{"name": "Kid", "role": "child", "theme": "quest"},
		adminHeaders(), http.StatusBadRequest)
	resp := env.expectStatus(t, "POST", "/api/users", map[string]any{"name": "Kid", "role": "child", "theme": "blocks"},
		adminHeaders(), http.StatusCreated)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["theme"] != "blocks" {
		t.Fatalf("expected blocks, got %v", user["theme"])
	}
	id := int(user["id"].(float64))
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", id), map[string]any{"theme": "galaxy"},
		adminHeaders(), http.StatusBadRequest)
}

// --- Person colours (users.color) ---

func TestColorEndpoint(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")

	// Self: OK.
	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/color", kid1), map[string]any{"color": "lilac"},
		childHeaders(kid1), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["color"] != "lilac" {
		t.Fatalf("expected lilac, got %v", user["color"])
	}

	// Someone else, even an admin: forbidden (admins use PUT /users/{id}).
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/color", kid1), map[string]any{"color": "sky"},
		childHeaders(kid2), http.StatusForbidden)
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/color", kid1), map[string]any{"color": "sky"},
		adminHeaders(), http.StatusForbidden)

	// Invalid keys, including hex values: bad request.
	for _, bad := range []string{"", "red", "#ff9b80", "Coral"} {
		env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/color", kid1), map[string]any{"color": bad},
			childHeaders(kid1), http.StatusBadRequest)
	}

	// Unauthenticated: rejected.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/color", kid1), map[string]any{"color": "sky"},
		nil, http.StatusUnauthorized)

	// Persisted.
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d", kid1), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &user)
	if user["color"] != "lilac" {
		t.Fatalf("expected lilac to persist, got %v", user["color"])
	}
}

func TestAdminUpdateColor(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	path := fmt.Sprintf("/api/users/%d", kid)

	resp := env.expectStatus(t, "PUT", path, map[string]any{"color": "sand"}, adminHeaders(), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["color"] != "sand" {
		t.Fatalf("expected sand, got %v", user["color"])
	}

	// Omitting color leaves it unchanged.
	resp = env.expectStatus(t, "PUT", path, map[string]any{"name": "Kiddo"}, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &user)
	if user["color"] != "sand" || user["name"] != "Kiddo" {
		t.Fatalf("expected name change to keep sand, got %v / %v", user["name"], user["color"])
	}

	env.expectStatus(t, "PUT", path, map[string]any{"color": "teal"}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/users", map[string]any{"name": "Other", "role": "child", "color": "teal"},
		adminHeaders(), http.StatusBadRequest)

	// Only admins may update another profile.
	other := env.createChild(t, "Other")
	env.expectStatus(t, "PUT", path, map[string]any{"color": "mint"}, childHeaders(other), http.StatusForbidden)
}

func TestNewUsersGetFirstFreeColor(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t) // inserted directly, no colour

	colorOf := func(body map[string]any) string {
		resp := env.expectStatus(t, "POST", "/api/users", body, adminHeaders(), http.StatusCreated)
		var user map[string]any
		decodeBody(t, resp, &user)
		c, _ := user["color"].(string)
		return c
	}

	if got := colorOf(map[string]any{"name": "A", "role": "child"}); got != "coral" {
		t.Fatalf("first user: expected coral, got %q", got)
	}
	// An explicit colour is kept and counts as used.
	if got := colorOf(map[string]any{"name": "B", "role": "child", "color": "butter"}); got != "butter" {
		t.Fatalf("explicit colour: expected butter, got %q", got)
	}
	if got := colorOf(map[string]any{"name": "C", "role": "child"}); got != "mint" {
		t.Fatalf("expected mint (first unused), got %q", got)
	}
	want := []string{"sky", "rose", "leaf", "lilac", "sand"}
	for i, w := range want {
		if got := colorOf(map[string]any{"name": fmt.Sprintf("D%d", i), "role": "child"}); got != w {
			t.Fatalf("user D%d: expected %s, got %q", i, w, got)
		}
	}
	// All eight taken: cycle round-robin from the start of the palette.
	if got := colorOf(map[string]any{"name": "E", "role": "child"}); got != "coral" {
		t.Fatalf("after all colours used: expected coral, got %q", got)
	}
	if got := colorOf(map[string]any{"name": "F", "role": "child"}); got != "mint" {
		t.Fatalf("round-robin: expected mint, got %q", got)
	}
}

func TestSetupAssignsAndValidatesColor(t *testing.T) {
	env := setupTest(t)

	env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"parent":   map[string]any{"name": "Robin", "pin": "2468"},
		"children": []map[string]any{{"name": "Alice", "color": "purple"}},
	}, nil, http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"parent":   map[string]any{"name": "Robin", "pin": "2468"},
		"children": []map[string]any{{"name": "Alice", "theme": "default"}},
	}, nil, http.StatusBadRequest)

	resp := env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"parent": map[string]any{"name": "Robin", "pin": "2468", "color": "sky"},
		"children": []map[string]any{
			{"name": "Alice", "theme": "tint", "color": "mint"},
			{"name": "Bob", "theme": "blocks"},
		},
	}, nil, http.StatusCreated)
	var result struct {
		Admin    map[string]any   `json:"admin"`
		Children []map[string]any `json:"children"`
	}
	decodeBody(t, resp, &result)
	if result.Admin["color"] != "sky" {
		t.Fatalf("expected parent sky, got %v", result.Admin["color"])
	}
	if result.Children[0]["color"] != "mint" || result.Children[0]["theme"] != "tint" {
		t.Fatalf("unexpected Alice: %v", result.Children[0])
	}
	// Bob gets the first colour not already taken (coral).
	if result.Children[1]["color"] != "coral" {
		t.Fatalf("expected Bob coral, got %v", result.Children[1]["color"])
	}
}

// --- Migration 017 ---

func TestMigration017MapsThemesAndAssignsColors(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		t.Fatalf("migration driver: %v", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if err := m.Migrate(16); err != nil {
		t.Fatalf("migrate to 16: %v", err)
	}

	// Legacy data, inserted in id order.
	legacy := []struct{ name, theme, line string }{
		{"Mia", "default", "#34D399"}, // mint-ish (upper case)
		{"Otto", "quest", "#fb923c"},  // orange -> coral
		{"Gus", "galaxy", ""},         // no line colour -> first free
		{"Fern", "forest", "red"},     // not a hex -> first free
		{"Wes", "weird", "#38bdf8"},   // sky blue; unknown theme -> ''
		{"Pat", "", "#ffffff"},        // white -> nearest (lilac)
	}
	for _, u := range legacy {
		if _, err := db.Exec(`INSERT INTO users (name, role, theme, line_color) VALUES (?, 'child', ?, ?)`, u.name, u.theme, u.line); err != nil {
			t.Fatalf("insert %s: %v", u.name, err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}

	want := map[string][2]string{ // name -> theme, color
		"Mia":  {"sunroom", "mint"},
		"Otto": {"blocks", "coral"},
		"Wes":  {"", "sky"},
		"Pat":  {"", "lilac"},
		// Round-robin over colours nobody has yet (butter, rose, ...) by id.
		"Gus":  {"tint", "butter"},
		"Fern": {"sunroom", "rose"},
	}
	rows, err := db.Query(`SELECT name, theme, color FROM users`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for rows.Next() {
		var name, theme, color string
		if err := rows.Scan(&name, &theme, &color); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if got := [2]string{theme, color}; got != want[name] {
			t.Errorf("%s: got theme/color %v, want %v", name, got, want[name])
		}
	}
	rows.Close()

	// Down restores the legacy theme names and drops the column.
	if err := m.Migrate(16); err != nil {
		t.Fatalf("migrate down to 16: %v", err)
	}
	var theme string
	if err := db.QueryRow(`SELECT theme FROM users WHERE name = 'Otto'`).Scan(&theme); err != nil {
		t.Fatalf("query: %v", err)
	}
	if theme != "quest" {
		t.Fatalf("expected quest after down, got %q", theme)
	}
	if err := db.QueryRow(`SELECT theme FROM users WHERE name = 'Gus'`).Scan(&theme); err != nil {
		t.Fatalf("query: %v", err)
	}
	if theme != "galaxy" {
		t.Fatalf("expected galaxy after down, got %q", theme)
	}
	if _, err := db.Exec(`SELECT color FROM users`); err == nil {
		t.Fatal("expected users.color to be dropped")
	}
}
