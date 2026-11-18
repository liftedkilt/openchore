package api_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/hub"
)

// startHub runs a coordination server that needs key "hubkey" to create links.
func startHub(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	h, err := hub.New(db, []string{"hubkey"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(func() { srv.Close(); db.Close() })
	return srv
}

func (e *testEnv) getJSON(t *testing.T, path string, headers map[string]string) map[string]any {
	t.Helper()
	resp := e.expectStatus(t, "GET", path, nil, headers, http.StatusOK)
	var out map[string]any
	decodeBody(t, resp, &out)
	return out
}

func (e *testEnv) leaderboard(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, v := range e.getJSON(t, "/api/leaderboard", adminHeaders())["entries"].([]any) {
		out = append(out, v.(map[string]any))
	}
	return out
}

// yesterdayChores gives a kid n chores yesterday and marks the first done of
// them complete, straight in the database (the API only completes today's).
func (e *testEnv) yesterdayChores(t *testing.T, kid, n, done int, category string) {
	t.Helper()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	for i := 0; i < n; i++ {
		resp := e.expectStatus(t, "POST", "/api/chores",
			map[string]any{"title": fmt.Sprintf("Chore %d", i), "category": category, "points_value": 5}, adminHeaders(), http.StatusCreated)
		var chore map[string]any
		decodeBody(t, resp, &chore)
		resp = e.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
			map[string]any{"assigned_to": kid, "specific_date": yesterday}, adminHeaders(), http.StatusCreated)
		var sched map[string]any
		decodeBody(t, resp, &sched)
		if i < done {
			if _, err := e.db.Exec(`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, completion_date)
				VALUES (?, ?, 'approved', ?)`, int(sched["id"].(float64)), kid, yesterday); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLeaderboardScoresByShareOfChoresDone(t *testing.T) {
	if time.Now().Weekday() == time.Monday {
		t.Skip("yesterday is in last week on a Monday")
	}
	env := setupTest(t)
	env.createAdmin(t)
	ava := env.createChild(t, "Ava")
	ben := env.createChild(t, "Ben")
	cat := env.createChild(t, "Cat")

	// Off until a parent turns it on.
	if lb := env.getJSON(t, "/api/leaderboard", adminHeaders()); lb["enabled"] != false || len(lb["entries"].([]any)) != 0 {
		t.Fatalf("leaderboard on by default: %v", lb)
	}
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, childHeaders(ava), http.StatusForbidden)
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
	env.expectStatus(t, "GET", "/api/leaderboard", nil, nil, http.StatusUnauthorized)

	// People who joined today have no yesterday; these three have been here a while.
	if _, err := env.db.Exec(`UPDATE users SET created_at = datetime('now', '-30 days')`); err != nil {
		t.Fatal(err)
	}
	env.yesterdayChores(t, ava, 5, 4, "core")      // 80%
	env.yesterdayChores(t, ben, 10, 9, "required") // 90%: more chores, better share
	env.yesterdayChores(t, cat, 2, 2, "core")      // 100% but too few to rank
	env.yesterdayChores(t, ava, 3, 3, "bonus")     // bonus never moves the percentage

	got := env.leaderboard(t)
	want := []struct {
		name                                 string
		rank, done, assigned, bonus, percent float64
	}{{"Ben", 1, 9, 10, 0, 90}, {"Ava", 2, 4, 5, 3, 80}, {"Cat", 0, 2, 2, 0, 100}}
	if len(got) != len(want) {
		t.Fatalf("entries: %v", got)
	}
	for i, w := range want {
		g := got[i]
		if g["name"] != w.name || g["rank"] != w.rank || g["done"] != w.done || g["assigned"] != w.assigned ||
			g["bonus"] != w.bonus || g["percent"] != w.percent {
			t.Errorf("place %d = %v, want %+v", i, g, w)
		}
	}

	// A chore due today doesn't count against anyone until it's done.
	sched := env.createScheduledChore(t, "Make bed", ava, nil)
	if g := env.leaderboard(t)[1]; g["assigned"] != 5.0 {
		t.Fatalf("today's undone chore already counted: %v", g)
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{}, childHeaders(ava), http.StatusCreated)
	if g := env.leaderboard(t)[1]; g["assigned"] != 6.0 || g["done"] != 5.0 {
		t.Fatalf("today's done chore not counted: %v", g)
	}
}

func TestFriendFamiliesLinkThroughHub(t *testing.T) {
	hubSrv := startHub(t)
	a, b := setupTest(t), setupTest(t)
	a.createAdmin(t)
	b.createAdmin(t)
	ava := a.createChild(t, "Ava")
	a.createChild(t, "Secret") // never shared
	bo := b.createChild(t, "Bo")

	a.enablePets(t)
	a.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog"}, childHeaders(ava), http.StatusCreated)
	sched := a.createScheduledChore(t, "Make bed", ava, nil)
	a.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{}, childHeaders(ava), http.StatusCreated)

	// Needs a family name and a hub; the hub needs its key.
	a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Avas"}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": "ftp://nope"}, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": hubSrv.URL, "hub_key": "wrong"}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusBadRequest)
	resp := a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_key": "hubkey"}, adminHeaders(), http.StatusOK)
	var settings map[string]any
	decodeBody(t, resp, &settings)
	if settings["hub_key_set"] != true || strings.Contains(fmt.Sprint(settings), "hubkey") {
		t.Fatalf("hub key leaked or not set: %v", settings)
	}
	a.expectStatus(t, "GET", "/api/admin/settings/friends_hub_key", nil, adminHeaders(), http.StatusForbidden)

	resp = a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated)
	var link map[string]any
	decodeBody(t, resp, &link)
	invite := link["invite"].(string)
	aLink := int(link["id"].(float64))
	if link["status"] != "waiting" || !strings.HasPrefix(invite, "ocf1.") {
		t.Fatalf("invite: %v", link)
	}

	// Share Ava (not Secret) and Bo.
	a.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/share", ava), map[string]any{"shared": true}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/share", bo), map[string]any{"shared": true}, adminHeaders(), http.StatusOK)

	// B joins: no hub settings needed, only a name and the invite.
	b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite}, adminHeaders(), http.StatusBadRequest)
	b.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Bos"}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": "ocf1.garbage"}, adminHeaders(), http.StatusBadRequest)
	resp = b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite[:20] + " \n" + invite[20:]}, adminHeaders(), http.StatusCreated)
	decodeBody(t, resp, &link)
	bLink := int(link["id"].(float64))
	// The invite is single-use.
	c := setupTest(t)
	c.createAdmin(t)
	c.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "Strangers"}, adminHeaders(), http.StatusOK)
	c.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite}, adminHeaders(), http.StatusBadRequest)

	// A syncs: picks up B's people and leaves its own; B syncs and gets A's.
	resp = a.expectStatus(t, "POST", fmt.Sprintf("/api/admin/friends/%d/sync", aLink), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &link)
	if link["status"] != "active" || link["peer_name"] != "The Bos" || fmt.Sprint(link["members"]) != "[Bo]" || link["invite"] != "" {
		t.Fatalf("A's link after sync: %v", link)
	}
	resp = b.expectStatus(t, "POST", fmt.Sprintf("/api/admin/friends/%d/sync", bLink), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &link)
	if link["peer_name"] != "The Avas" || fmt.Sprint(link["members"]) != "[Ava]" {
		t.Fatalf("B's link after sync: %v", link)
	}

	// B's leaderboard has Ava with her real count; Secret never left A.
	var sawAva bool
	for _, e := range b.leaderboard(t) {
		if e["name"] == "Secret" {
			t.Fatal("an unshared person was sent to the other family")
		}
		if e["name"] == "Ava" {
			sawAva = true
			if e["family"] != "The Avas" || e["done"] != 1.0 || e["assigned"] != 1.0 || e["rank"] != 0.0 {
				t.Fatalf("Ava on B's leaderboard: %v", e)
			}
		}
	}
	if !sawAva {
		t.Fatal("Ava isn't on B's leaderboard")
	}

	// B's kids can look at A's farm.
	farms := b.getJSON(t, "/api/friends/farms", childHeaders(bo))["farms"].([]any)
	if len(farms) != 1 {
		t.Fatalf("farms: %v", farms)
	}
	farm := farms[0].(map[string]any)
	pets := farm["pets"].([]any)
	if farm["family"] != "The Avas" || len(pets) != 1 || pets[0].(map[string]any)["name"] != "Biscuit" ||
		pets[0].(map[string]any)["owner_name"] != "Ava" {
		t.Fatalf("farm: %v", farm)
	}

	// The hub only ever held sealed data.
	var blob []byte
	hubDB := hubSrv.Config.Handler.(*hub.Hub)
	if err := hubDB.DB().QueryRow(`SELECT a_data FROM links`).Scan(&blob); err != nil || len(blob) == 0 {
		t.Fatalf("hub blob: %v", err)
	}
	for _, s := range []string{"Ava", "Biscuit", "The Avas"} {
		if strings.Contains(string(blob), s) {
			t.Fatalf("the hub can read %q", s)
		}
	}

	// Switching sharing off empties what the other family holds at the next sync.
	a.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/share", ava), map[string]any{"shared": false}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", fmt.Sprintf("/api/admin/friends/%d/sync", aLink), nil, adminHeaders(), http.StatusOK)
	resp = b.expectStatus(t, "POST", fmt.Sprintf("/api/admin/friends/%d/sync", bLink), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &link)
	if fmt.Sprint(link["members"]) != "[]" {
		t.Fatalf("Ava still held by B after sharing was switched off: %v", link)
	}

	// A unlinks: gone at A at once, and B finds out at its next sync and
	// drops A's data.
	a.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/friends/%d", aLink), nil, adminHeaders(), http.StatusNoContent)
	if links := a.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any); len(links) != 0 {
		t.Fatalf("A still has links: %v", links)
	}
	resp = b.expectStatus(t, "POST", fmt.Sprintf("/api/admin/friends/%d/sync", bLink), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &link)
	if link["status"] != "ended" {
		t.Fatalf("B's link after A unlinked: %v", link)
	}
	if farms := b.getJSON(t, "/api/friends/farms", childHeaders(bo))["farms"].([]any); len(farms) != 0 {
		t.Fatalf("B still shows A's farm: %v", farms)
	}
	b.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/friends/%d", bLink), nil, adminHeaders(), http.StatusNoContent)
}
