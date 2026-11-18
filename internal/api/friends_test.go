package api_test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

// yesterdayChores gives a kid n chores yesterday, each worth points, and
// marks the first done of them complete, straight in the database (the API
// only completes today's).
func (e *testEnv) yesterdayChores(t *testing.T, kid, n, done int, category string, points int) {
	t.Helper()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	for i := 0; i < n; i++ {
		resp := e.expectStatus(t, "POST", "/api/chores",
			map[string]any{"title": fmt.Sprintf("Chore %d", i), "category": category, "points_value": points}, adminHeaders(), http.StatusCreated)
		var chore map[string]any
		decodeBody(t, resp, &chore)
		resp = e.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
			map[string]any{"assigned_to": kid, "specific_date": yesterday}, adminHeaders(), http.StatusCreated)
		var sched map[string]any
		decodeBody(t, resp, &sched)
		// Made today, for yesterday: a finished day only counts chores that
		// existed on it, so pretend this one has been there all along.
		if _, err := e.db.Exec(`UPDATE chore_schedules SET created_at = datetime('now', '-10 days') WHERE id = ?`, int(sched["id"].(float64))); err != nil {
			t.Fatal(err)
		}
		if i < done {
			if _, err := e.db.Exec(`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, completion_date)
				VALUES (?, ?, 'approved', ?)`, int(sched["id"].(float64)), kid, yesterday); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLeaderboardScoresByShareOfPointsEarned(t *testing.T) {
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
	env.yesterdayChores(t, ava, 5, 4, "core", 5) // 20 of 25 points: 80%
	// Ben did nine small chores and skipped the big one: 9 of 10 chores, but
	// only 18 of 40 points. The score follows the points.
	env.yesterdayChores(t, ben, 9, 9, "required", 2)
	env.yesterdayChores(t, ben, 1, 0, "required", 22)
	env.yesterdayChores(t, cat, 2, 2, "core", 5)  // 100% but too few chores to rank
	env.yesterdayChores(t, ava, 3, 3, "bonus", 9) // bonus never moves the percentage

	got := env.leaderboard(t)
	want := []struct {
		name                                 string
		rank, done, assigned, bonus, percent float64
	}{{"Ava", 1, 4, 5, 3, 80}, {"Ben", 2, 9, 10, 0, 45}, {"Cat", 0, 2, 2, 0, 100}}
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
	if g := env.leaderboard(t)[0]; g["assigned"] != 5.0 {
		t.Fatalf("today's undone chore already counted: %v", g)
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{}, childHeaders(ava), http.StatusCreated)
	if g := env.leaderboard(t)[0]; g["assigned"] != 6.0 || g["done"] != 5.0 || g["share"] != 8333.0 || g["percent"] != 83.0 || g["done_points"] != nil {
		t.Fatalf("today's done chore not counted: %v", g)
	}
}

func TestFriendFamiliesLinkThroughHub(t *testing.T) {
	hubSrv := startHub(t)
	// The test hub is on this machine; a hub named by an invite normally has
	// to be on a public address (TestFriendInviteCannotPointInward).
	t.Setenv("OPENCHORE_HUB_ALLOW_PRIVATE", "true")
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

	aPath, bPath := fmt.Sprintf("/api/admin/friends/%d", aLink), fmt.Sprintf("/api/admin/friends/%d", bLink)
	put := func(e *testEnv, path string, body map[string]any) map[string]any {
		t.Helper()
		var out map[string]any
		decodeBody(t, e.expectStatus(t, "PUT", path, body, adminHeaders(), http.StatusOK), &out)
		return out
	}
	// names lists a direction of a link as "Name:flag" pairs.
	names := func(l map[string]any, dir, flag string) string {
		var out []string
		for _, v := range l[dir].([]any) {
			m := v.(map[string]any)
			out = append(out, fmt.Sprintf("%v:%v", m["name"], m[flag]))
		}
		return strings.Join(out, " ")
	}
	sync := func(e *testEnv, path string) map[string]any {
		t.Helper()
		var out map[string]any
		decodeBody(t, e.expectStatus(t, "POST", path+"/sync", nil, adminHeaders(), http.StatusOK), &out)
		return out
	}

	// Linked, but nothing is shared in either direction until parents choose.
	link = sync(a, aPath)
	if link["status"] != "active" || link["peer_name"] != "The Bos" || len(link["inbound"].([]any)) != 0 || link["invite"] != "" {
		t.Fatalf("A's link after the first sync: %v", link)
	}
	if lb := b.leaderboard(t); len(lb) != 1 || lb[0]["name"] != "Bo" {
		t.Fatalf("B's leaderboard before anything is shared: %v", lb)
	}

	// A's parent shares Ava (not Secret) with the Bos. B is offered Ava, but
	// she isn't shown, and none of her data is kept, until B's parent picks her.
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{999}}, adminHeaders(), http.StatusBadRequest)
	link = put(a, aPath, map[string]any{"share_user_ids": []int{ava}})
	if got := names(link, "outbound", "shared"); got != "Admin:false Ava:true Secret:false" {
		t.Fatalf("A's outbound: %s", got)
	}
	link = sync(b, bPath)
	if got := names(link, "inbound", "accepted"); got != "Ava:false" {
		t.Fatalf("B's inbound: %s", got)
	}
	if lb := b.leaderboard(t); len(lb) != 1 {
		t.Fatalf("Ava shown at B before B's parent accepted her: %v", lb)
	}
	if farms := b.getJSON(t, "/api/friends/farms", childHeaders(bo))["farms"].([]any); len(farms) != 0 {
		t.Fatalf("A's farm shown at B before anyone was accepted: %v", farms)
	}
	var stored string
	if err := b.db.QueryRow(`SELECT snapshot FROM friend_links`).Scan(&stored); err != nil || strings.Contains(stored, "Biscuit") {
		t.Fatalf("B kept data for a person its parent hasn't accepted: %v %s", err, stored)
	}

	// B's parent accepts Ava (and can't accept someone who wasn't offered).
	b.expectStatus(t, "PUT", bPath, map[string]any{"accept_member_ids": []string{"12345"}}, adminHeaders(), http.StatusBadRequest)
	avaID := fmt.Sprint(ava)
	link = put(b, bPath, map[string]any{"accept_member_ids": []string{avaID}, "share_user_ids": []int{bo}})
	if got := names(link, "inbound", "accepted"); got != "Ava:true" {
		t.Fatalf("B's inbound after accepting: %s", got)
	}
	// A can see that the Bos picked Ava up, and is now offered Bo.
	link = sync(a, aPath)
	if got := names(link, "outbound", "accepted"); got != "Admin:false Ava:true Secret:false" {
		t.Fatalf("A doesn't see that B accepted Ava: %s", got)
	}
	if got := names(link, "inbound", "accepted"); got != "Bo:false" {
		t.Fatalf("A's inbound: %s", got)
	}

	// B's leaderboard has Ava with her real count; Secret never left A.
	var sawAva bool
	for _, e := range b.leaderboard(t) {
		if e["name"] == "Secret" {
			t.Fatal("an unshared person was sent to the other family")
		}
		if e["name"] == "Ava" {
			sawAva = true
			if e["family"] != "The Avas" || e["done"] != 1.0 || e["assigned"] != 1.0 || e["rank"] != 0.0 || e["percent"] != 100.0 {
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

	// What B holds about Ava is only what its screens show: weekly totals
	// and how the animal is doing. No days, point values or health numbers.
	if err := b.db.QueryRow(`SELECT snapshot FROM friend_links`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{`"days"`, `"date"`, `points`, `"health"`, `"fullness"`, `"joy"`, "Make bed"} {
		if strings.Contains(stored, leaked) {
			t.Fatalf("the snapshot carries %s: %s", leaked, stored)
		}
	}
	if !strings.Contains(stored, `"weeks"`) || !strings.Contains(stored, `"share":10000`) {
		t.Fatalf("the snapshot has no weekly totals: %s", stored)
	}

	// The wall display's leaderboard is public and has this household only.
	wall := b.getJSON(t, "/api/leaderboard/household", nil)
	if es := wall["entries"].([]any); len(es) != 1 || es[0].(map[string]any)["name"] != "Bo" || len(wall["families"].([]any)) != 0 {
		t.Fatalf("household leaderboard shows other families: %v", wall)
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

	// When A's parent stops sharing Ava, B loses her at its next sync.
	put(a, aPath, map[string]any{"share_user_ids": []int{}})
	if link = sync(b, bPath); len(link["inbound"].([]any)) != 0 {
		t.Fatalf("Ava still held by B after A stopped sharing her: %v", link)
	}
	for _, e := range b.leaderboard(t) {
		if e["name"] == "Ava" {
			t.Fatal("Ava still on B's leaderboard after A stopped sharing her")
		}
	}

	// A unlinks: gone at A at once, and B finds out at its next sync and
	// drops A's data.
	a.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/friends/%d", aLink), nil, adminHeaders(), http.StatusNoContent)
	if links := a.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any); len(links) != 0 {
		t.Fatalf("A still has links: %v", links)
	}
	if link = sync(b, bPath); link["status"] != "ended" {
		t.Fatalf("B's link after A unlinked: %v", link)
	}
	if farms := b.getJSON(t, "/api/friends/farms", childHeaders(bo))["farms"].([]any); len(farms) != 0 {
		t.Fatalf("B still shows A's farm: %v", farms)
	}
	b.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/friends/%d", bLink), nil, adminHeaders(), http.StatusNoContent)
}

// link pairs two fresh households through hub and returns them with each
// side's link path. Ava (A) is shared with B and accepted there.
func linkedPair(t *testing.T, hubSrv *httptest.Server) (a, b *testEnv, aPath, bPath string, ava int) {
	t.Helper()
	t.Setenv("OPENCHORE_HUB_ALLOW_PRIVATE", "true")
	a, b = setupTest(t), setupTest(t)
	a.createAdmin(t)
	b.createAdmin(t)
	ava = a.createChild(t, "Ava")
	a.enablePets(t)
	a.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog"}, childHeaders(ava), http.StatusCreated)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Avas", "hub_url": hubSrv.URL, "hub_key": "hubkey"}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Bos"}, adminHeaders(), http.StatusOK)
	var link map[string]any
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &link)
	aPath = fmt.Sprintf("/api/admin/friends/%d", int(link["id"].(float64)))
	invite := link["invite"].(string)
	decodeBody(t, b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite}, adminHeaders(), http.StatusCreated), &link)
	bPath = fmt.Sprintf("/api/admin/friends/%d", int(link["id"].(float64)))
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", bPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(ava)}}, adminHeaders(), http.StatusOK)
	return a, b, aPath, bPath, ava
}

func (e *testEnv) storedSnapshot(t *testing.T) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(`SELECT snapshot FROM friend_links`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// An invite is text someone else wrote. The hub it names can't be on this
// server's own network, can't carry a query, and failures don't describe
// the network to the person who pasted it.
func TestFriendInviteCannotPointInward(t *testing.T) {
	var hits int
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"token":"t"}`))
	}))
	defer inner.Close()
	env := setupTest(t)
	env.createAdmin(t)
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "Us"}, adminHeaders(), http.StatusOK)
	invite := func(hub string) string {
		raw := fmt.Sprintf(`{"h":%q,"l":"L","c":"C","k":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, hub)
		return "ocf1." + base64.RawURLEncoding.EncodeToString([]byte(raw))
	}
	join := func(hub string) (int, string) {
		resp := env.request(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite(hub)}, adminHeaders())
		var out map[string]any
		decodeBody(t, resp, &out)
		return resp.StatusCode, fmt.Sprint(out["error"])
	}
	if code, msg := join(inner.URL); code != http.StatusBadRequest || !strings.Contains(msg, "private network") || hits != 0 {
		t.Fatalf("invite to a loopback hub: %d %q, %d request(s) reached it", code, msg, hits)
	}
	if code, _ := join(inner.URL + "/admin?x="); code != http.StatusBadRequest || hits != 0 {
		t.Fatalf("invite with a query in the hub address: %d, %d request(s)", code, hits)
	}
	if code, _ := join("http://example.com"); code != http.StatusBadRequest {
		t.Fatalf("plain http to a public host: %d", code)
	}
	// Where it is allowed, a dead hub is reported without addresses or ports.
	t.Setenv("OPENCHORE_HUB_ALLOW_PRIVATE", "true")
	if code, msg := join("http://127.0.0.1:1"); code != http.StatusBadGateway || strings.Contains(msg, "127.0.0.1") || strings.Contains(msg, "refused") {
		t.Fatalf("dead hub: %d %q", code, msg)
	}
	if links := env.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any); len(links) != 0 {
		t.Fatalf("a failed join left a link: %v", links)
	}
}

// A stray 404 from something in front of the hub must not end the link, and
// un-ticking a person removes what's stored about them even when the hub
// can't be reached.
func TestFriendLinkSurvivesHubTrouble(t *testing.T) {
	hubSrv := startHub(t)
	_, b, _, bPath, _ := linkedPair(t, hubSrv)
	if !strings.Contains(b.storedSnapshot(t), "Biscuit") {
		t.Fatal("setup: B doesn't hold Ava's animal")
	}
	// Put a broken proxy in front of the hub, as B sees it.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer broken.Close()
	if _, err := b.db.Exec(`UPDATE friend_links SET hub_url = ?`, broken.URL); err != nil {
		t.Fatal(err)
	}
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusBadGateway)
	link := b.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any)[0].(map[string]any)
	if link["status"] != "active" || link["last_error"] == "" || !strings.Contains(b.storedSnapshot(t), "Biscuit") {
		t.Fatalf("a 404 ended the link or dropped its data: %v", link)
	}
	// Un-tick Ava while the hub is still unreachable.
	b.expectStatus(t, "PUT", bPath, map[string]any{"accept_member_ids": []string{}}, adminHeaders(), http.StatusOK)
	if stored := b.storedSnapshot(t); strings.Contains(stored, "Biscuit") || strings.Contains(stored, `"share"`) || !strings.Contains(stored, "Ava") {
		t.Fatalf("after un-ticking with the hub down, B still holds Ava's data (or lost her name): %s", stored)
	}
	// The hub comes back: the link carries on.
	if _, err := b.db.Exec(`UPDATE friend_links SET hub_url = ?`, hubSrv.URL); err != nil {
		t.Fatal(err)
	}
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
}

// Adding a chore today must not reach back and mark earlier days as missed.
func TestLeaderboardPastDaysIgnoreNewChores(t *testing.T) {
	if time.Now().Weekday() == time.Monday {
		t.Skip("yesterday is in last week on a Monday")
	}
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.createChild(t, "Sib")
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
	if _, err := env.db.Exec(`UPDATE users SET created_at = datetime('now', '-30 days')`); err != nil {
		t.Fatal(err)
	}
	env.yesterdayChores(t, kid, 5, 5, "core", 5)
	mine := func() map[string]any {
		for _, e := range env.leaderboard(t) {
			if e["name"] == "Kid" {
				return e
			}
		}
		t.Fatal("Kid not on the leaderboard")
		return nil
	}
	if e := mine(); e["assigned"] != 5.0 || e["percent"] != 100.0 {
		t.Fatalf("before: %v", e)
	}
	// A new every-day chore, created now: it applies from today, not to yesterday.
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "New daily", "category": "required", "points_value": 50}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	for dow := 0; dow < 7; dow++ {
		env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
			map[string]any{"assigned_to": kid, "day_of_week": dow}, adminHeaders(), http.StatusCreated)
	}
	if _, err := env.db.Exec(`DELETE FROM score_days`); err != nil { // force a recount of the finished days
		t.Fatal(err)
	}
	if e := mine(); e["assigned"] != 5.0 || e["percent"] != 100.0 {
		t.Fatalf("a chore added today changed yesterday's score: %v", e)
	}
}

// A hub key belongs to one hub.
func TestFriendHubKeyDoesNotFollowANewAddress(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	put := func(body map[string]any) map[string]any {
		var out map[string]any
		decodeBody(t, env.expectStatus(t, "PUT", "/api/admin/friends", body, adminHeaders(), http.StatusOK), &out)
		return out
	}
	if out := put(map[string]any{"hub_url": "https://one.example", "hub_key": "k1"}); out["hub_key_set"] != true {
		t.Fatalf("set: %v", out)
	}
	if out := put(map[string]any{"hub_url": "https://one.example"}); out["hub_key_set"] != true {
		t.Fatalf("same address dropped the key: %v", out)
	}
	if out := put(map[string]any{"hub_url": "https://two.example"}); out["hub_key_set"] != false {
		t.Fatalf("the key followed a new address: %v", out)
	}
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": "https://two.example/?x=1"}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": "http://two.example"}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": "http://192.168.1.50:8090"}, adminHeaders(), http.StatusOK)
}

// A link this household made the invite for stays usable when its hub
// address in Settings changes later, even though that hub is on a private
// address; and whatever a hub says in an error is kept short.
func TestFriendLinkKeepsItsOwnHub(t *testing.T) {
	hubSrv := startHub(t)
	a := setupTest(t)
	a.createAdmin(t)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Avas", "hub_url": hubSrv.URL, "hub_key": "hubkey"}, adminHeaders(), http.StatusOK)
	var link map[string]any
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &link)
	path := fmt.Sprintf("/api/admin/friends/%d", int(link["id"].(float64)))
	a.expectStatus(t, "POST", path+"/sync", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"hub_url": "https://elsewhere.example"}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", path+"/sync", nil, adminHeaders(), http.StatusOK)

	// A hub that answers with a wall of text.
	loud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":%q}`, strings.Repeat("x", 100000)+"\nsecond line")
	}))
	defer loud.Close()
	if _, err := a.db.Exec(`UPDATE friend_links SET hub_url = ?`, loud.URL); err != nil {
		t.Fatal(err)
	}
	resp := a.request(t, "POST", path+"/sync", nil, adminHeaders())
	var out map[string]any
	decodeBody(t, resp, &out)
	if msg := fmt.Sprint(out["error"]); len(msg) > 250 || strings.Contains(msg, "\n") {
		t.Fatalf("hub error not kept short: %d characters", len(msg))
	}
}

// A custom animal picture reaches a friend family only when a parent on the
// sending side approved it and a parent on the receiving side chose to show
// that family's pictures. Otherwise it isn't sent, or isn't kept.
func TestFriendPicturesNeedBothParents(t *testing.T) {
	hubSrv := startHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)
	bo := b.createChild(t, "Bo")
	a.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": ava, "amount": 500, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	reward := a.petRewardID(t, ava, "picture")
	upload := func() string {
		a.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", reward),
			map[string]any{"pet": map[string]any{"image": spritePNG(t, 3*32, 32, false, 3)}}, childHeaders(ava), http.StatusCreated).Body.Close()
		id := a.myPet(t, ava)["sprite"].(string)
		t.Cleanup(func() { os.Remove("data/uploads/pets/" + id + ".png") })
		return id
	}
	first := upload()
	approve := fmt.Sprintf("/api/admin/users/%d/pet/picture", ava)
	bothSync := func() {
		t.Helper()
		a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
		b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	}
	theirPet := func() map[string]any {
		t.Helper()
		farms := b.getJSON(t, "/api/friends/farms", childHeaders(bo))["farms"].([]any)
		return farms[0].(map[string]any)["pets"].([]any)[0].(map[string]any)
	}

	// Not approved: not sent at all, even though B would show pictures.
	b.expectStatus(t, "PUT", bPath, map[string]any{"show_pictures": true}, adminHeaders(), http.StatusOK)
	bothSync()
	if p := theirPet(); p["sprite_data"] != nil || strings.Contains(b.storedSnapshot(t), `"picture"`) {
		t.Fatalf("an unapproved picture was sent: %v", p)
	}
	// A kid can't approve their own; a parent can.
	a.expectStatus(t, "PUT", approve, map[string]any{"approved": true, "sprite": first}, childHeaders(ava), http.StatusForbidden)
	a.expectStatus(t, "PUT", approve, map[string]any{"approved": true}, adminHeaders(), http.StatusBadRequest) // which picture?
	a.expectStatus(t, "PUT", approve, map[string]any{"approved": true, "sprite": first}, adminHeaders(), http.StatusOK)
	bothSync()
	p := theirPet()
	if data, _ := p["sprite_data"].(string); !strings.HasPrefix(data, "data:image/png;base64,") || p["sprite_frames"] != 3.0 || p["species"] != "dog" {
		t.Fatalf("approved picture not shown at B: %v", p)
	}
	// B's parent turns pictures off: gone from B at once, hub or no hub.
	b.expectStatus(t, "PUT", bPath, map[string]any{"show_pictures": false}, adminHeaders(), http.StatusOK)
	if p := theirPet(); p["sprite_data"] != nil || strings.Contains(b.storedSnapshot(t), `"picture"`) {
		t.Fatalf("picture kept after B turned pictures off: %v", p)
	}
	b.expectStatus(t, "PUT", bPath, map[string]any{"show_pictures": true}, adminHeaders(), http.StatusOK)
	if p := theirPet(); p["sprite_data"] == nil {
		t.Fatalf("picture didn't come back when B turned pictures on again: %v", p)
	}
	// A new picture needs approving again, and a tick meant for the old
	// one (a settings page left open) doesn't approve it unseen.
	upload()
	if a.myPet(t, ava)["sprite_approved"] != nil {
		t.Fatal("a new picture kept the old one's approval")
	}
	a.expectStatus(t, "PUT", approve, map[string]any{"approved": true, "sprite": first}, adminHeaders(), http.StatusConflict)
	if a.myPet(t, ava)["sprite_approved"] != nil {
		t.Fatal("a tick for the old picture approved the new one")
	}
	bothSync()
	if p := theirPet(); p["sprite_data"] != nil {
		t.Fatalf("a new, unapproved picture was sent: %v", p)
	}
	// Approving when there's no picture makes no sense.
	a.expectStatus(t, "PATCH", "/api/me/pet", map[string]any{"sprite": ""}, childHeaders(ava), http.StatusOK)
	a.expectStatus(t, "PUT", approve, map[string]any{"approved": true, "sprite": first}, adminHeaders(), http.StatusNotFound)
}

// lastWeekDay stores one finished day in last week for a person.
func (e *testEnv) lastWeekDay(t *testing.T, userID, assigned, done int) {
	t.Helper()
	now := time.Now()
	monday := now.AddDate(0, 0, -((int(now.Weekday())+6)%7)-7)
	if _, err := e.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, ?, ?)`,
		userID, monday.AddDate(0, 0, 2).Format("2006-01-02"), assigned, done); err != nil {
		t.Fatal(err)
	}
}

// Whoever came first last week wears the winner's ribbon this week, on the
// leaderboard and on the farm. With nobody to beat there is no ribbon.
func TestLeaderboardWinnerWearsARibbon(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	ava := env.createChild(t, "Ava")
	ben := env.createChild(t, "Ben")
	env.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog"}, childHeaders(ava), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(ben), http.StatusCreated)
	if _, err := env.db.Exec(`UPDATE users SET created_at = datetime('now', '-30 days')`); err != nil {
		t.Fatal(err)
	}
	ribbons := func() string {
		var out []string
		for _, e := range env.leaderboard(t) {
			if e["winner"] == true {
				out = append(out, "board:"+e["name"].(string))
			}
		}
		for _, p := range env.getJSON(t, "/api/pets", nil)["pets"].([]any) {
			if pet := p.(map[string]any); pet["ribbon"] == true {
				out = append(out, "farm:"+pet["name"].(string))
			}
		}
		return strings.Join(out, " ")
	}
	// Only Ava had enough chores to be ranked last week: nobody to beat.
	env.lastWeekDay(t, ava, 6, 6)
	if got := ribbons(); got != "" {
		t.Fatalf("a ribbon with nobody to beat: %s", got)
	}
	env.lastWeekDay(t, ben, 6, 4)
	if got := ribbons(); got != "board:Ava farm:Biscuit" {
		t.Fatalf("ribbons = %q, want Ava's on the board and on her animal", got)
	}
}

// Two linked families are also two teams, and can agree a goal to reach
// together. Both households work out the same numbers.
func TestFriendTeamsAndGoal(t *testing.T) {
	hubSrv := startHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)
	bo := b.createChild(t, "Bo")
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)
	for _, e := range []*testEnv{a, b} {
		e.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
		if _, err := e.db.Exec(`UPDATE users SET created_at = datetime('now', '-30 days')`); err != nil {
			t.Fatal(err)
		}
	}
	a.lastWeekDay(t, ava, 5, 4) // 80
	b.lastWeekDay(t, bo, 5, 5)  // 100
	b.createChild(t, "Unshared")
	bothSync := func() {
		t.Helper()
		for i := 0; i < 2; i++ {
			a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
			b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
		}
	}
	bothSync()
	team := func(e *testEnv, week string) map[string]any {
		t.Helper()
		teams := e.getJSON(t, "/api/leaderboard?week="+week, adminHeaders())["teams"].([]any)
		if len(teams) != 1 {
			t.Fatalf("teams (%s): %v", week, teams)
		}
		return teams[0].(map[string]any)
	}
	score := func(m map[string]any, k string) string {
		s := m[k].(map[string]any)
		return fmt.Sprintf("%v/%v", s["people"], s["score"])
	}
	// A hasn't ticked Bo yet: B's team total would be exactly Bo's score, so
	// it isn't sent, and A's own team only counts people B shows (Ava).
	if at := team(a, "last"); score(at, "theirs") != "0/0" || score(at, "ours") != "1/80" {
		t.Fatalf("before A accepted Bo: %v", at)
	}
	if stored := a.storedSnapshot(t); strings.Contains(stored, `"share_sum":10000`) {
		t.Fatalf("A holds a team total that gives away an unaccepted person's score: %s", stored)
	}
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo)}}, adminHeaders(), http.StatusOK)
	bothSync() // B learns that A shows Bo...
	bothSync() // ...and its next snapshot counts him in its team
	at, bt := team(a, "last"), team(b, "last")
	if score(at, "ours") != "1/80" || score(at, "theirs") != "1/100" || score(at, "together") != "2/90" {
		t.Fatalf("A's view of last week's teams: %v", at)
	}
	if score(bt, "ours") != "1/100" || score(bt, "theirs") != "1/80" || score(bt, "together") != "2/90" {
		t.Fatalf("B's view of last week's teams: %v", bt)
	}
	// The public, household-only leaderboard has no teams.
	if teams := a.getJSON(t, "/api/leaderboard/household", nil)["teams"].([]any); len(teams) != 0 {
		t.Fatalf("teams on the household leaderboard: %v", teams)
	}

	// A puts a goal forward; it counts once B agrees to the same one.
	goal := func(e *testEnv) string {
		g, _ := e.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any)[0].(map[string]any)["goal"].(map[string]any)
		if g == nil {
			return "none"
		}
		return fmt.Sprintf("%v %v", g["state"], g["target"])
	}
	a.expectStatus(t, "PUT", aPath, map[string]any{"goal_target": 101}, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "PUT", aPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	bothSync()
	if goal(a) != "ours 85" || goal(b) != "theirs 85" {
		t.Fatalf("after A proposed: A=%q B=%q", goal(a), goal(b))
	}
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	bothSync()
	if goal(a) != "active 85" || goal(b) != "active 85" {
		t.Fatalf("after B agreed: A=%q B=%q", goal(a), goal(b))
	}
	if g := team(a, "this")["goal"].(map[string]any); g["state"] != "active" || g["met"] != false {
		t.Fatalf("this week's goal on the leaderboard: %v", g)
	}
	// A different number from each side: each sees the other's, so one can agree.
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 70}, adminHeaders(), http.StatusOK)
	bothSync()
	their := func(e *testEnv) any {
		return e.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any)[0].(map[string]any)["goal"].(map[string]any)["their_target"]
	}
	if goal(a) != "differs 85" || their(a) != 70.0 || goal(b) != "differs 70" || their(b) != 85.0 {
		t.Fatalf("differing goals: A=%q (%v) B=%q (%v)", goal(a), their(a), goal(b), their(b))
	}
	// Either side can drop it.
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 0}, adminHeaders(), http.StatusOK)
	bothSync()
	if goal(a) != "ours 85" || goal(b) != "theirs 85" {
		t.Fatalf("after B dropped it: A=%q B=%q", goal(a), goal(b))
	}
}

// Kids can spend points to send a friend's animal a treat or send their own
// animal over for a playdate. Each arrives once, only between people both
// parents picked, and never more than the receiving side allows.
func TestFriendGiftsAndPlaydates(t *testing.T) {
	hubSrv := startHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)
	bo := b.createChild(t, "Bo")
	b.enablePets(t)
	b.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(bo), http.StatusCreated)
	b.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": bo, "amount": 500, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	bothSync := func() {
		t.Helper()
		for i := 0; i < 2; i++ {
			a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
			b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
		}
	}
	bothSync()
	gift, playdate := b.petRewardID(t, bo, "gift"), b.petRewardID(t, bo, "playdate")
	redeem := func(reward int, pet map[string]any, want int) string {
		t.Helper()
		resp := b.request(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", reward), map[string]any{"pet": pet}, childHeaders(bo))
		var out map[string]any
		decodeBody(t, resp, &out)
		if resp.StatusCode != want {
			t.Fatalf("redeem: %d %v, want %d", resp.StatusCode, out, want)
		}
		return fmt.Sprint(out["error"])
	}
	avaKey := fmt.Sprintf("f%s:%d", bPath[strings.LastIndex(bPath, "/")+1:], ava)
	joy := func() float64 { return a.myPet(t, ava)["joy"].(float64) }
	a.db.Exec(`UPDATE pets SET joy = 40, fullness = 40`)

	// Bo isn't shared with A yet, so nothing he sends would be shown there.
	link := map[string]any{"link": 1}
	if msg := redeem(gift, map[string]any{"friend": avaKey}, http.StatusUnprocessableEntity); !strings.Contains(msg, "can't see you yet") {
		t.Fatalf("gift before sharing: %q", msg)
	}
	if msg := redeem(playdate, link, http.StatusUnprocessableEntity); !strings.Contains(msg, "can't see you yet") {
		t.Fatalf("playdate before sharing: %q", msg)
	}
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)
	bothSync()
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo)}}, adminHeaders(), http.StatusOK)
	bothSync()

	// A treat for Ava's animal: costs Bo points, lands on Biscuit once.
	before := b.balance(t, bo)
	redeem(gift, map[string]any{"friend": "f9:9"}, http.StatusUnprocessableEntity)
	redeem(gift, map[string]any{"friend": avaKey}, http.StatusCreated)
	if b.balance(t, bo) != before-10 {
		t.Fatalf("gift cost: %d -> %d", before, b.balance(t, bo))
	}
	if msg := redeem(gift, map[string]any{"friend": avaKey}, http.StatusUnprocessableEntity); !strings.Contains(msg, "already sent Biscuit") {
		t.Fatalf("second gift the same day: %q", msg)
	}
	bothSync()
	bothSync() // arriving again changes nothing
	if joy() != 60 {
		t.Fatalf("Biscuit's joy after one treat = %v, want 60", joy())
	}
	got := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))
	if gifts := got["gifts"].([]any); len(gifts) != 1 || gifts[0].(map[string]any)["from"] != "Bo" || gifts[0].(map[string]any)["to_user_id"] != float64(ava) {
		t.Fatalf("gifts at A: %v", got)
	}

	// A playdate: Clover shows up on A's farm for the day.
	redeem(playdate, link, http.StatusCreated)
	if msg := redeem(playdate, link, http.StatusUnprocessableEntity); !strings.Contains(msg, "already gone on a playdate") {
		t.Fatalf("second playdate the same day: %q", msg)
	}
	bothSync()
	visitors := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["visitors"].([]any)
	if len(visitors) != 1 {
		t.Fatalf("visitors at A: %v", visitors)
	}
	if v := visitors[0].(map[string]any); v["name"] != "Clover" || v["owner_name"] != "Bo" || v["visiting"] != true || v["family"] != "The Bos" {
		t.Fatalf("visitor: %v", v)
	}
	// Visitors are friends' kids' animals: signed-in only, and not on the public farm.
	a.expectStatus(t, "GET", "/api/friends/visitors", nil, nil, http.StatusUnauthorized)
	if pets := a.getJSON(t, "/api/pets", nil)["pets"].([]any); len(pets) != 1 {
		t.Fatalf("the public farm shows visitors: %v", pets)
	}

	// Undoing the redemption takes an unsent gift back with the points.
	theo := b.createChild(t, "Theo")
	b.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Socks", "species": "cat"}, childHeaders(theo), http.StatusCreated)
	b.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": theo, "amount": 100, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo, theo}}, adminHeaders(), http.StatusOK)
	bothSync()
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo), fmt.Sprint(theo)}}, adminHeaders(), http.StatusOK)
	bothSync()
	resp := b.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", gift), map[string]any{"pet": map[string]any{"friend": avaKey}}, childHeaders(theo), http.StatusCreated)
	var redemption map[string]any
	decodeBody(t, resp, &redemption)
	b.expectStatus(t, "DELETE", fmt.Sprintf("/api/redemptions/%d", int(redemption["id"].(float64))), nil, adminHeaders(), http.StatusNoContent)
	var left int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM friend_sends WHERE from_user = ?`, theo).Scan(&left); err != nil || left != 0 {
		t.Fatalf("an undone gift is still queued to send: %d %v", left, err)
	}

	// Two taps at once: the limit holds and only one is paid for.
	b.db.Exec(`DELETE FROM friend_sends WHERE kind = 'playdate'`)
	before = b.balance(t, theo)
	codes := make(chan int, 6)
	for i := 0; i < 6; i++ {
		go func() {
			r := b.request(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", playdate), map[string]any{"pet": link}, childHeaders(theo))
			r.Body.Close()
			codes <- r.StatusCode
		}()
	}
	created := 0
	for i := 0; i < 6; i++ {
		if <-codes == http.StatusCreated {
			created++
		}
	}
	if created != 1 || b.balance(t, theo) != before-15 {
		t.Fatalf("six playdates at once: %d went through, balance %d -> %d", created, before, b.balance(t, theo))
	}

	// Something that arrived isn't applied again later: not after it has
	// been forgotten here, and not from a snapshot that has gone stale.
	bothSync()
	a.db.Exec(`UPDATE pets SET joy = 10`)
	a.db.Exec(`UPDATE friend_received SET received_on = date('now', '-8 days')`)
	b.db.Exec(`UPDATE friend_sends SET day = date('now', '-8 days')`)
	bothSync()
	if joy() != 10 {
		t.Fatalf("an old gift was applied again: joy %v", joy())
	}
	if v := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["visitors"].([]any); len(v) != 0 {
		t.Fatalf("an old playdate still has a visitor here: %v", v)
	}
	b.db.Exec(`DELETE FROM friend_sends`)

	// A's parent turns visits off: B's kids are told, and aren't charged.
	a.expectStatus(t, "PUT", aPath, map[string]any{"allow_visits": false}, adminHeaders(), http.StatusOK)
	bothSync()
	before = b.balance(t, bo)
	if msg := redeem(gift, map[string]any{"friend": avaKey}, http.StatusUnprocessableEntity); !strings.Contains(msg, "aren't taking treats") || b.balance(t, bo) != before {
		t.Fatalf("gift while A isn't taking them: %q, balance %d -> %d", msg, before, b.balance(t, bo))
	}
	if v := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["visitors"].([]any); len(v) != 0 {
		t.Fatalf("visitors after A turned visits off: %v", v)
	}
	// However many gifts the other server claims, an animal takes three a day from one family.
	a.expectStatus(t, "PUT", aPath, map[string]any{"allow_visits": true}, adminHeaders(), http.StatusOK)
	bothSync()
	if _, err := b.db.Exec(`DROP INDEX idx_friend_sends_gift`); err != nil { // B's server stops playing by the rules
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := b.db.Exec(`INSERT INTO friend_sends (id, link_id, kind, from_user, to_member, day) VALUES (?, 1, 'gift', ?, ?, ?)`,
			fmt.Sprintf("flood%d", i), bo, fmt.Sprint(ava), time.Now().Format("2006-01-02")); err != nil {
			t.Fatal(err)
		}
	}
	a.db.Exec(`UPDATE pets SET joy = 0`)
	a.db.Exec(`DELETE FROM friend_received`)
	bothSync()
	if joy() != 60 { // three, however many were claimed
		t.Fatalf("joy after a flood of gifts = %v, want 60 (three gifts a day at most)", joy())
	}
}
