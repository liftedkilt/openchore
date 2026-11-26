package api_test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/friends"
	"github.com/liftedkilt/openchore/internal/hub"
	"github.com/liftedkilt/openchore/internal/model"
)

// startHub runs a coordination server that needs key "hubkey" to create links.
func startHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newTestHub(t))
	t.Cleanup(srv.Close)
	return srv
}

// startFlakyHub is startHub with a switch: while down is set, every request
// gets a 503 as if the hub were unreachable.
func startFlakyHub(t *testing.T) (*httptest.Server, *atomic.Bool) {
	t.Helper()
	srv, down, _ := startWatchedHub(t)
	return srv, down
}

// startWatchedHub is startFlakyHub that also counts finished exchanges, so a
// test can wait for a background sync.
func startWatchedHub(t *testing.T) (*httptest.Server, *atomic.Bool, *atomic.Int64) {
	t.Helper()
	h, down, exchanges := newTestHub(t), &atomic.Bool{}, &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		h.ServeHTTP(w, r)
		if strings.HasSuffix(r.URL.Path, "/exchange") {
			exchanges.Add(1)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, down, exchanges
}

// waitFor waits up to five seconds for n to reach want.
func waitFor(t *testing.T, n *atomic.Int64, want int64, what string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); n.Load() < want; {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s (%d of %d)", what, n.Load(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A gift goes to the hub as soon as it's bought. Undoing the purchase leaves
// a new snapshot without it straight away, so the other family's next sync
// doesn't pick it up.
// giftSetup links A and B, with B's Bo (100 points, his own animal) shared
// with A and accepted there, ready to send Ava's animal a treat.
type giftSetup struct {
	a, b         *testEnv
	aPath, bPath string
	ava, bo      int
	gift         int    // B's gift reward
	avaKey       string // the friend key for Ava's animal, as B sees it
	exchanges    *atomic.Int64
}

func newGiftSetup(t *testing.T) *giftSetup {
	t.Helper()
	hubSrv, _, exchanges := startWatchedHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)
	bo := b.createChild(t, "Bo")
	b.enablePets(t)
	b.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(bo), http.StatusCreated)
	b.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": bo, "amount": 100, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo)}}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	return &giftSetup{a: a, b: b, aPath: aPath, bPath: bPath, ava: ava, bo: bo, gift: b.petRewardID(t, bo, "gift"),
		avaKey: fmt.Sprintf("f%s:%d", bPath[strings.LastIndex(bPath, "/")+1:], ava), exchanges: exchanges}
}

// A gift is marked paid in the transaction that charges for it: if its
// reservation is gone, nothing is charged. An unpaid reservation left behind
// by a server that stopped mid-purchase stops holding the day's limit.
func TestFriendSendIsPaidInOneStep(t *testing.T) {
	g := newGiftSetup(t)
	b, bo := g.b, g.bo
	today := time.Now().Format("2006-01-02")
	reserve := func(id, created string) {
		t.Helper()
		if _, err := b.db.Exec(`INSERT INTO friend_sends (id, link_id, kind, from_user, to_member, day, created_at) VALUES (?, 1, 'gift', ?, ?, ?, ?)`,
			id, bo, fmt.Sprint(g.ava), today, created); err != nil {
			t.Fatal(err)
		}
	}
	before := b.balance(t, bo)

	reserve("r1", time.Now().UTC().Format("2006-01-02 15:04:05"))
	red, err := b.store.RedeemRewardFor(t.Context(), int64(bo), int64(g.gift), "r1")
	if err != nil {
		t.Fatal(err)
	}
	var paidBy int64
	if err := b.db.QueryRow(`SELECT redemption_id FROM friend_sends WHERE id = 'r1'`).Scan(&paidBy); err != nil || paidBy != red.ID {
		t.Fatalf("reservation not marked paid by redemption %d: %d %v", red.ID, paidBy, err)
	}
	if err := b.store.UndoRedemption(t.Context(), red.ID); err != nil {
		t.Fatal(err)
	}

	// The reservation vanished before paying: no charge, no redemption.
	if _, err := b.store.RedeemRewardFor(t.Context(), int64(bo), int64(g.gift), "gone"); err == nil {
		t.Fatal("paid for a gift whose reservation is gone")
	}
	if got := b.balance(t, bo); got != before {
		t.Fatalf("balance %d -> %d after a refused payment", before, got)
	}

	// An unpaid reservation from 20 minutes ago no longer holds today's treat.
	reserve("orphan", time.Now().UTC().Add(-20*time.Minute).Format("2006-01-02 15:04:05"))
	b.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", g.gift), map[string]any{"pet": map[string]any{"friend": g.avaKey}}, childHeaders(bo), http.StatusCreated)
	var left int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM friend_sends WHERE id = 'orphan'`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("orphaned reservation still there: %d %v", left, err)
	}
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM friend_sends WHERE redemption_id = 0`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("a bought gift was left unpaid: %d %v", left, err)
	}
}

func TestFriendUndoneGiftDoesNotArrive(t *testing.T) {
	g := newGiftSetup(t)
	a, b, aPath, bPath, ava, bo, gift, avaKey, exchanges := g.a, g.b, g.aPath, g.bPath, g.ava, g.bo, g.gift, g.avaKey, g.exchanges

	// A reserved gift that isn't paid for yet never goes out.
	if _, err := b.db.Exec(`INSERT INTO friend_sends (id, link_id, kind, from_user, to_member, day) VALUES ('unpaid', 1, 'gift', ?, ?, ?)`,
		bo, fmt.Sprint(ava), time.Now().Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	var got int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_received`).Scan(&got); err != nil || got != 0 {
		t.Fatalf("a gift nobody paid for arrived: %d %v", got, err)
	}
	b.db.Exec(`DELETE FROM friend_sends WHERE id = 'unpaid'`)

	for _, undo := range []string{"points history", "redemption"} {
		base := exchanges.Load()
		resp := b.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", gift), map[string]any{"pet": map[string]any{"friend": avaKey}}, childHeaders(bo), http.StatusCreated)
		var redemption map[string]any
		decodeBody(t, resp, &redemption)
		id := int64(redemption["id"].(float64))
		waitFor(t, exchanges, base+1, "the sync after buying the gift")
		if undo == "redemption" {
			b.expectStatus(t, "DELETE", fmt.Sprintf("/api/redemptions/%d", id), nil, adminHeaders(), http.StatusNoContent)
		} else {
			var txn int64
			if err := b.db.QueryRow(`SELECT id FROM point_transactions WHERE reason = 'reward_redeem' AND reference_id = ?`, id).Scan(&txn); err != nil {
				t.Fatal(err)
			}
			b.expectStatus(t, "POST", fmt.Sprintf("/api/points/transactions/%d/undo", txn), nil, adminHeaders(), http.StatusNoContent)
		}
		waitFor(t, exchanges, base+2, "the sync after undoing it ("+undo+")")
		a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_received`).Scan(&got); err != nil || got != 0 {
			t.Fatalf("a gift undone from the %s still arrived: %d %v", undo, got, err)
		}
	}

	// The control: the same gift, not undone, does arrive.
	base := exchanges.Load()
	b.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", gift), map[string]any{"pet": map[string]any{"friend": avaKey}}, childHeaders(bo), http.StatusCreated)
	waitFor(t, exchanges, base+1, "the sync after buying the gift")
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_received`).Scan(&got); err != nil || got != 1 {
		t.Fatalf("a gift that wasn't undone didn't arrive: %d %v", got, err)
	}
}

func newTestHub(t *testing.T) *hub.Hub {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h, err := hub.New(db, []string{"hubkey"})
	if err != nil {
		t.Fatal(err)
	}
	return h
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

	// After B joins, A sees pending_confirm with B's family name and spent invite cleared,
	// cannot configure sharing yet, and nothing is shared until A's parent confirms.
	link = sync(a, aPath)
	if link["status"] != "pending_confirm" || link["peer_name"] != "The Bos" || len(link["inbound"].([]any)) != 0 || link["invite"] != "" {
		t.Fatalf("A's link after B joins (before confirm): %v", link)
	}
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusBadRequest)
	if lb := b.leaderboard(t); len(lb) != 1 || lb[0]["name"] != "Bo" {
		t.Fatalf("B's leaderboard before A confirms: %v", lb)
	}
	// A's parent confirms The Bos; the link becomes active.
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/confirm", nil, adminHeaders(), http.StatusOK), &link)
	if link["status"] != "active" || link["peer_name"] != "The Bos" || len(link["inbound"].([]any)) != 0 || link["invite"] != "" {
		t.Fatalf("A's link after confirm: %v", link)
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
	// Nor does A send it: until B's parent picks Ava, she leaves A as a name
	// and a colour only.
	sent, err := friends.New(a.store).Snapshot(t.Context(), int64(aLink))
	if err != nil || len(sent.Members) != 1 || sent.Members[0].Name != "Ava" || sent.Members[0].Pet != nil || len(sent.Members[0].Weeks) != 0 {
		t.Fatalf("A sent more than a name for someone B hasn't accepted: %v %+v", err, sent)
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

	// A now sends Ava in full, and B has her at its next sync.
	sync(b, bPath)
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
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", aPath+"/confirm", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", bPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(ava)}}, adminHeaders(), http.StatusOK)
	// A learns that Ava was accepted and sends her in full; B picks that up.
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	return a, b, aPath, bPath, ava
}

// Unlinking while the hub is down still removes the link here at once, and
// the hub is told once it's back, so the other family stops getting this
// family's data instead of keeping the last snapshot for good.
func TestFriendUnlinkRetriesWhenHubIsDown(t *testing.T) {
	hubSrv, down := startFlakyHub(t)
	a, b, aPath, bPath, _ := linkedPair(t, hubSrv)
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)

	down.Store(true)
	a.expectStatus(t, "DELETE", aPath, nil, adminHeaders(), http.StatusNoContent)
	if links := a.getJSON(t, "/api/admin/friends", adminHeaders())["links"].([]any); len(links) != 0 {
		t.Fatalf("A still has links: %v", links)
	}
	var pending int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_unlinks`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("unconfirmed unlinks: %d %v", pending, err)
	}

	// Back up: the hub still holds A's snapshot until it's told.
	down.Store(false)
	var link map[string]any
	decodeBody(t, b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK), &link)
	if link["status"] != "active" {
		t.Fatalf("B's link before the retry: %v", link)
	}

	// The next round of syncing at A tells the hub, and B finds out.
	friends.New(a.store).SyncAll(t.Context())
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_unlinks`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("unlink still pending after the hub came back: %d %v", pending, err)
	}
	decodeBody(t, b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK), &link)
	if link["status"] != "ended" {
		t.Fatalf("B's link after the retry: %v", link)
	}
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
	// Path-traversal link IDs are rejected before any request leaves the server.
	traversal := "ocf1." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"h":%q,"l":"..","c":"C","k":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, inner.URL)))
	resp := env.request(t, "POST", "/api/admin/friends/join", map[string]any{"invite": traversal}, adminHeaders())
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || hits != 0 {
		t.Fatalf("path-traversal invite: status=%d hits=%d", resp.StatusCode, hits)
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
	// With farm pets off at A, the treat doesn't touch Biscuit.
	a.expectStatus(t, "PUT", "/api/admin/pets", map[string]any{"enabled": false}, adminHeaders(), http.StatusOK)
	bothSync()
	var petJoy int
	if err := a.db.QueryRow(`SELECT joy FROM pets WHERE user_id = ?`, ava).Scan(&petJoy); err != nil || petJoy != 40 {
		t.Fatalf("a gift changed an animal with farm pets off: joy %d %v", petJoy, err)
	}
	a.enablePets(t)
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
	// The same goes for Undo in the points history.
	resp = b.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", gift), map[string]any{"pet": map[string]any{"friend": avaKey}}, childHeaders(theo), http.StatusCreated)
	decodeBody(t, resp, &redemption)
	var txnID int64
	if err := b.db.QueryRow(`SELECT id FROM point_transactions WHERE reason = 'reward_redeem' AND reference_id = ?`, int64(redemption["id"].(float64))).Scan(&txnID); err != nil {
		t.Fatalf("gift's points entry: %v", err)
	}
	b.expectStatus(t, "POST", fmt.Sprintf("/api/points/transactions/%d/undo", txnID), nil, adminHeaders(), http.StatusNoContent)
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM friend_sends WHERE from_user = ?`, theo).Scan(&left); err != nil || left != 0 {
		t.Fatalf("a gift undone from the points history is still queued to send: %d %v", left, err)
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
	// These columns hold the server's local date, so age them in local time
	// (SQLite's date('now') is UTC and can be a day ahead in the evening).
	eightDaysAgo := time.Now().AddDate(0, 0, -8).Format(model.DateFormat)
	a.db.Exec(`UPDATE friend_received SET received_on = ?`, eightDaysAgo)
	b.db.Exec(`UPDATE friend_sends SET day = ?`, eightDaysAgo)
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
		if _, err := b.db.Exec(`INSERT INTO friend_sends (id, link_id, kind, from_user, to_member, day, redemption_id) VALUES (?, 1, 'gift', ?, ?, ?, ?)`,
			fmt.Sprintf("flood%d", i), bo, fmt.Sprint(ava), time.Now().Format("2006-01-02"), 9000+i); err != nil {
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

// Reaching an agreed weekly team goal shows a celebratory decoration on both
// families' farms (own farm via /api/friends/visitors, visited friend farm via
// /api/friends/farms) while preserving privacy, feature switches, rounding, and
// agreement rules.
func TestTeamGoalFarmCelebration(t *testing.T) {
	hubSrv := startHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)

	// Pin both clocks to next week's Wednesday so GeneratedAt stays monotonic
	// after linkedPair's initial sync and Tuesday of the same week is always a
	// finished day inside "this" week, regardless of when the test runs.
	base := time.Now().AddDate(0, 0, 7)
	monday := time.Date(base.Year(), base.Month(), base.Day()-((int(base.Weekday())+6)%7), 12, 0, 0, 0, time.Local)
	wed := monday.AddDate(0, 0, 2)
	weekStart := monday.Format("2006-01-02")
	tue := monday.AddDate(0, 0, 1).Format("2006-01-02")
	a.friends.SetClock(func() time.Time { return wed })
	b.friends.SetClock(func() time.Time { return wed })

	bo := b.createChild(t, "Bo")
	b.enablePets(t)
	b.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(bo), http.StatusCreated)
	for _, e := range []*testEnv{a, b} {
		e.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
		if _, err := e.db.Exec(`UPDATE users SET created_at = '2020-01-01 00:00:00'`); err != nil {
			t.Fatal(err)
		}
	}

	bothSync := func() {
		t.Helper()
		for i := 0; i < 2; i++ {
			a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
			b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
		}
	}
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)
	bothSync()
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo)}}, adminHeaders(), http.StatusOK)
	bothSync()

	setScore := func(e *testEnv, userID, assigned, done int) {
		t.Helper()
		if _, err := e.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, ?, ?)`,
			userID, tue, assigned, done); err != nil {
			t.Fatal(err)
		}
	}
	ownCelebrations := func(e *testEnv, hdr map[string]string) []any {
		t.Helper()
		out := e.getJSON(t, "/api/friends/visitors", hdr)
		raw, ok := out["goal_celebrations"]
		if !ok || raw == nil {
			t.Fatalf("goal_celebrations missing or null: %v", out)
		}
		return raw.([]any)
	}
	friendFarmGoal := func(e *testEnv, hdr map[string]string) map[string]any {
		t.Helper()
		farms := e.getJSON(t, "/api/friends/farms", hdr)["farms"].([]any)
		if len(farms) != 1 {
			t.Fatalf("expected 1 friend farm, got %v", farms)
		}
		gc, _ := farms[0].(map[string]any)["goal_celebration"].(map[string]any)
		return gc
	}

	// 1. No goal yet -> hidden on both farms even with 100% scores.
	setScore(a, ava, 10000, 10000)
	setScore(b, bo, 10000, 10000)
	bothSync()
	if got := ownCelebrations(a, childHeaders(ava)); len(got) != 0 {
		t.Fatalf("expected no own celebration before goal: %v", got)
	}
	if gc := friendFarmGoal(a, childHeaders(ava)); gc != nil {
		t.Fatalf("expected no friend farm celebration before goal: %v", gc)
	}

	// 2. Unagreed goals ("ours", "theirs", "differs") stay hidden.
	a.expectStatus(t, "PUT", aPath, map[string]any{"goal_target": 80}, adminHeaders(), http.StatusOK)
	bothSync()
	if len(ownCelebrations(a, childHeaders(ava))) != 0 || len(ownCelebrations(b, childHeaders(bo))) != 0 {
		t.Fatalf("one-sided goal should not celebrate")
	}
	if friendFarmGoal(a, childHeaders(ava)) != nil || friendFarmGoal(b, childHeaders(bo)) != nil {
		t.Fatalf("one-sided goal should not celebrate on visited farm")
	}
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 75}, adminHeaders(), http.StatusOK)
	bothSync()
	if len(ownCelebrations(a, childHeaders(ava))) != 0 || len(ownCelebrations(b, childHeaders(bo))) != 0 {
		t.Fatalf("differing goals should not celebrate")
	}

	// 3. Agreed goal 80, floor-rounding boundary: (5998 + 10000) / 2 / 100 = 7999 / 100 = 79 < 80 -> hidden.
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 80}, adminHeaders(), http.StatusOK)
	setScore(a, ava, 10000, 5998)
	setScore(b, bo, 10000, 10000)
	bothSync()
	if len(ownCelebrations(a, childHeaders(ava))) != 0 || len(ownCelebrations(b, childHeaders(bo))) != 0 {
		t.Fatalf("79.99%% floored to 79 should stay below target 80")
	}
	if friendFarmGoal(a, childHeaders(ava)) != nil || friendFarmGoal(b, childHeaders(bo)) != nil {
		t.Fatalf("79.99%% floored to 79 should stay hidden on friend farms")
	}

	// 4. Exact boundary: (6000 + 10000) / 2 / 100 = 80 >= 80 -> celebrates on BOTH farms for BOTH households!
	setScore(a, ava, 10000, 6000)
	bothSync()
	checkGoal := func(label string, m map[string]any, wantFamily string, wantScore float64) {
		t.Helper()
		if m == nil {
			t.Fatalf("%s: expected goal celebration, got nil", label)
		}
		if m["family"] != wantFamily || m["week"] != "this" || m["week_start"] != weekStart || m["target"] != 80.0 || m["score"] != wantScore {
			t.Fatalf("%s: unexpected celebration %v", label, m)
		}
	}
	aOwn := ownCelebrations(a, childHeaders(ava))
	bOwn := ownCelebrations(b, childHeaders(bo))
	if len(aOwn) != 1 || len(bOwn) != 1 {
		t.Fatalf("expected 1 own celebration on each side: A=%v B=%v", aOwn, bOwn)
	}
	checkGoal("A own farm", aOwn[0].(map[string]any), "The Bos", 80)
	checkGoal("B own farm", bOwn[0].(map[string]any), "The Avas", 80)
	checkGoal("A visited farm", friendFarmGoal(a, childHeaders(ava)), "The Bos", 80)
	checkGoal("B visited farm", friendFarmGoal(b, childHeaders(bo)), "The Avas", 80)

	// 5. Public privacy invariant: /api/pets leaks nothing; unauthenticated friend endpoints return 401.
	pubPets := a.getJSON(t, "/api/pets", nil)
	if _, has := pubPets["goal_celebrations"]; has {
		t.Fatalf("public /api/pets leaked goal_celebrations: %v", pubPets)
	}
	if _, has := pubPets["goal_celebration"]; has {
		t.Fatalf("public /api/pets leaked goal_celebration: %v", pubPets)
	}
	a.expectStatus(t, "GET", "/api/friends/visitors", nil, nil, http.StatusUnauthorized)
	a.expectStatus(t, "GET", "/api/friends/farms", nil, nil, http.StatusUnauthorized)

	// 6. allow_visits: false MUST NOT suppress team goal celebrations (gifts/playdates only).
	a.expectStatus(t, "PUT", aPath, map[string]any{"allow_visits": false}, adminHeaders(), http.StatusOK)
	bothSync()
	if len(ownCelebrations(a, childHeaders(ava))) != 1 || friendFarmGoal(a, childHeaders(ava)) == nil {
		t.Fatalf("allow_visits=false suppressed team goal celebration on A")
	}
	if len(ownCelebrations(b, childHeaders(bo))) != 1 || friendFarmGoal(b, childHeaders(bo)) == nil {
		t.Fatalf("allow_visits=false suppressed team goal celebration on B")
	}
	a.expectStatus(t, "PUT", aPath, map[string]any{"allow_visits": true}, adminHeaders(), http.StatusOK)
	bothSync()

	// 7. Farm pets disabled on A hides A's own celebration and hides A's celebration on B's /api/friends/farms (pets: []).
	a.expectStatus(t, "PUT", "/api/admin/pets", map[string]any{"enabled": false}, adminHeaders(), http.StatusOK)
	bothSync()
	if got := ownCelebrations(a, childHeaders(ava)); len(got) != 0 {
		t.Fatalf("expected empty own celebrations when pets disabled on A: %v", got)
	}
	if gc := friendFarmGoal(b, childHeaders(bo)); gc != nil {
		t.Fatalf("expected no friend farm celebration on B when A turned pets off: %v", gc)
	}
	a.enablePets(t)
	bothSync()

	// 8. Leaderboard disabled on A hides both own-farm and visited-farm celebrations on A.
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": false}, adminHeaders(), http.StatusOK)
	if got := ownCelebrations(a, childHeaders(ava)); len(got) != 0 {
		t.Fatalf("expected empty own celebrations when leaderboard disabled: %v", got)
	}
	if gc := friendFarmGoal(a, childHeaders(ava)); gc != nil {
		t.Fatalf("expected no friend farm celebration when leaderboard disabled: %v", gc)
	}
	a.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)

	// 9. Dropping the goal (goal_target: 0) hides the celebration on both sides.
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 0}, adminHeaders(), http.StatusOK)
	bothSync()
	if len(ownCelebrations(a, childHeaders(ava))) != 0 || len(ownCelebrations(b, childHeaders(bo))) != 0 {
		t.Fatalf("dropped goal should clear own celebrations")
	}
	if friendFarmGoal(a, childHeaders(ava)) != nil || friendFarmGoal(b, childHeaders(bo)) != nil {
		t.Fatalf("dropped goal should clear friend farm celebrations")
	}
}

// Deterministic weekly rollover (this -> last -> cleared after 2 weeks or new
// proposal), unlinking, and multiple linked friend families.
func TestTeamGoalFarmCelebrationRolloverAndMultipleLinks(t *testing.T) {
	hubSrv := startHub(t)
	a, b, aPath, bPath, ava := linkedPair(t, hubSrv)

	base := time.Now().AddDate(0, 0, 7)
	mondayW := time.Date(base.Year(), base.Month(), base.Day()-((int(base.Weekday())+6)%7), 12, 0, 0, 0, time.Local)
	now := mondayW.AddDate(0, 0, 2) // Wednesday week W
	weekW := mondayW.Format("2006-01-02")
	tueW := mondayW.AddDate(0, 0, 1).Format("2006-01-02")
	tueW1 := mondayW.AddDate(0, 0, 8).Format("2006-01-02")
	a.friends.SetClock(func() time.Time { return now })
	b.friends.SetClock(func() time.Time { return now })

	bo := b.createChild(t, "Bo")
	b.enablePets(t)
	b.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(bo), http.StatusCreated)
	for _, e := range []*testEnv{a, b} {
		e.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
		if _, err := e.db.Exec(`UPDATE users SET created_at = '2020-01-01 00:00:00'`); err != nil {
			t.Fatal(err)
		}
	}

	syncPair := func(x *testEnv, xPath string, y *testEnv, yPath string) {
		t.Helper()
		for i := 0; i < 2; i++ {
			x.expectStatus(t, "POST", xPath+"/sync", nil, adminHeaders(), http.StatusOK)
			y.expectStatus(t, "POST", yPath+"/sync", nil, adminHeaders(), http.StatusOK)
		}
	}

	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)
	syncPair(a, aPath, b, bPath)
	a.expectStatus(t, "PUT", aPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(bo)}}, adminHeaders(), http.StatusOK)
	syncPair(a, aPath, b, bPath)

	// Link a third household C ("The Cas") to A.
	c := setupTest(t)
	c.friends.SetClock(func() time.Time { return now })
	c.createAdmin(t)
	cal := c.createChild(t, "Cal")
	c.enablePets(t)
	c.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Pip", "species": "cat"}, childHeaders(cal), http.StatusCreated)
	c.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": "The Cas", "leaderboard_enabled": true}, adminHeaders(), http.StatusOK)
	if _, err := c.db.Exec(`UPDATE users SET created_at = '2020-01-01 00:00:00'`); err != nil {
		t.Fatal(err)
	}
	var inv struct {
		ID     int64  `json:"id"`
		Invite string `json:"invite"`
	}
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &inv)
	acPath := fmt.Sprintf("/api/admin/friends/%d", inv.ID)
	var joined struct {
		ID int64 `json:"id"`
	}
	decodeBody(t, c.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": inv.Invite}, adminHeaders(), http.StatusCreated), &joined)
	caPath := fmt.Sprintf("/api/admin/friends/%d", joined.ID)

	a.expectStatus(t, "POST", acPath+"/sync", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", acPath+"/confirm", nil, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "PUT", acPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusOK)
	c.expectStatus(t, "PUT", caPath, map[string]any{"share_user_ids": []int{cal}}, adminHeaders(), http.StatusOK)
	syncPair(a, acPath, c, caPath)
	a.expectStatus(t, "PUT", acPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(cal)}}, adminHeaders(), http.StatusOK)
	c.expectStatus(t, "PUT", caPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(ava)}}, adminHeaders(), http.StatusOK)
	syncPair(a, acPath, c, caPath)

	// Seed scores: Ava 90%, Bo 90%, Cal 50%.
	for _, s := range []struct {
		env  *testEnv
		uid  int
		done int
	}{{a, ava, 90}, {b, bo, 90}, {c, cal, 50}} {
		if _, err := s.env.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, 100, ?)`,
			s.uid, tueW, s.done); err != nil {
			t.Fatal(err)
		}
	}

	// Agree goal 85 on A<->B (reached: 90 >= 85) and goal 85 on A<->C (not reached: (90+50)/2 = 70 < 85).
	a.expectStatus(t, "PUT", aPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", bPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "PUT", acPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	c.expectStatus(t, "PUT", caPath, map[string]any{"goal_target": 85}, adminHeaders(), http.StatusOK)
	syncPair(a, aPath, b, bPath)
	syncPair(a, acPath, c, caPath)

	// A's own farm celebrates only B; A's visited farms decorate B only, never C.
	aOwn := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any)
	if len(aOwn) != 1 || aOwn[0].(map[string]any)["family"] != "The Bos" {
		t.Fatalf("expected only The Bos in A own celebrations, got %v", aOwn)
	}
	farmsByFamily := func(e *testEnv, hdr map[string]string) map[string]map[string]any {
		t.Helper()
		out := map[string]map[string]any{}
		for _, f := range e.getJSON(t, "/api/friends/farms", hdr)["farms"].([]any) {
			fm := f.(map[string]any)
			out[fm["family"].(string)] = fm
		}
		return out
	}
	af := farmsByFamily(a, childHeaders(ava))
	if af["The Bos"]["goal_celebration"] == nil || af["The Cas"]["goal_celebration"] != nil {
		t.Fatalf("expected goal_celebration on The Bos only: %v", af)
	}

	// Now Cal finishes more chores so A<->C reaches (90+90)/2 = 90 >= 85 too.
	if _, err := c.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, 100, 90)`, cal, tueW); err != nil {
		t.Fatal(err)
	}
	syncPair(a, acPath, c, caPath)
	aOwn = a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any)
	if len(aOwn) != 2 {
		t.Fatalf("expected 2 own celebrations on A (The Bos & The Cas), got %v", aOwn)
	}
	af = farmsByFamily(a, childHeaders(ava))
	if af["The Bos"]["goal_celebration"] == nil || af["The Cas"]["goal_celebration"] == nil {
		t.Fatalf("expected goal_celebration on both visited farms: %v", af)
	}

	// Monday rollover (week W+1): both celebrations persist with week="last" and week_start=weekW!
	now = mondayW.AddDate(0, 0, 7)
	syncPair(a, aPath, b, bPath)
	syncPair(a, acPath, c, caPath)
	aOwn = a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any)
	if len(aOwn) != 2 {
		t.Fatalf("expected celebrations to persist into week W+1, got %v", aOwn)
	}
	for _, raw := range aOwn {
		m := raw.(map[string]any)
		if m["week"] != "last" || m["week_start"] != weekW {
			t.Fatalf("expected week=last week_start=%s after Monday rollover, got %v", weekW, m)
		}
	}
	if gc := farmsByFamily(b, childHeaders(bo))["The Avas"]["goal_celebration"].(map[string]any); gc["week"] != "last" || gc["week_start"] != weekW {
		t.Fatalf("expected B visited farm to carry last week's celebration, got %v", gc)
	}

	// A proposes a new unagreed goal for week W+1 on A<->C -> clears A<->C celebration while A<->B stays.
	a.expectStatus(t, "PUT", acPath, map[string]any{"goal_target": 80}, adminHeaders(), http.StatusOK)
	syncPair(a, acPath, c, caPath)
	aOwn = a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any)
	if len(aOwn) != 1 || aOwn[0].(map[string]any)["family"] != "The Bos" {
		t.Fatalf("new unagreed proposal on C should clear C celebration only, got %v", aOwn)
	}

	// Unlinking B removes B's celebration immediately on A and on B after sync (status=ended).
	a.expectStatus(t, "DELETE", aPath, nil, adminHeaders(), http.StatusNoContent)
	if got := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any); len(got) != 0 {
		t.Fatalf("unlink should clear celebration immediately on A: %v", got)
	}
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	if got := b.getJSON(t, "/api/friends/visitors", childHeaders(bo))["goal_celebrations"].([]any); len(got) != 0 {
		t.Fatalf("ended link should clear celebration on B after sync: %v", got)
	}

	// Two-week rollover: agree and reach A<->C in week W+1, then advance to week W+3 -> cleared.
	c.expectStatus(t, "PUT", caPath, map[string]any{"goal_target": 80}, adminHeaders(), http.StatusOK)
	if _, err := a.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, 100, 90)`, ava, tueW1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.db.Exec(`INSERT OR REPLACE INTO score_days (user_id, date, assigned, done) VALUES (?, ?, 100, 90)`, cal, tueW1); err != nil {
		t.Fatal(err)
	}
	now = mondayW.AddDate(0, 0, 9) // Wednesday week W+1
	syncPair(a, acPath, c, caPath)
	if got := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any); len(got) != 1 {
		t.Fatalf("expected active celebration in week W+1, got %v", got)
	}
	now = mondayW.AddDate(0, 0, 21) // Monday week W+3 (2 weeks later)
	if got := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["goal_celebrations"].([]any); len(got) != 0 {
		t.Fatalf("expected celebration to expire after 2 weeks, got %v", got)
	}
	if af := farmsByFamily(a, childHeaders(ava)); af["The Cas"]["goal_celebration"] != nil {
		t.Fatalf("expected visited farm celebration to expire after 2 weeks, got %v", af["The Cas"])
	}
}

// TestFriendInviteConfirmAndDecline covers the full invite-join parent
// confirmation lifecycle, auth & precondition errors, outgoing/incoming privacy
// invariants, side-b waiting UX, online & offline Confirm/Decline, corrupt
// ciphertext handling, and FriendMember.Streak preservation.
func TestFriendInviteConfirmAndDecline(t *testing.T) {
	t.Setenv("OPENCHORE_HUB_ALLOW_PRIVATE", "true")
	hubSrv, down := startFlakyHub(t)

	setupHousehold := func(family, kidName, petName string) (*testEnv, int) {
		e := setupTest(t)
		e.createAdmin(t)
		kid := e.createChild(t, kidName)
		e.enablePets(t)
		e.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": petName, "species": "dog"}, childHeaders(kid), http.StatusCreated)
		e.yesterdayChores(t, kid, 1, 1, "core", 10)
		// Give the kid a streak so we can verify FriendMember.Streak is preserved.
		if _, err := e.db.Exec(`INSERT INTO user_streaks (user_id, current_streak, longest_streak, last_completed_date) VALUES (?, 4, 4, ?)`, kid, time.Now().Format(model.DateFormat)); err != nil {
			t.Fatal(err)
		}
		e.expectStatus(t, "PUT", "/api/admin/friends", map[string]any{"family_name": family, "hub_url": hubSrv.URL, "hub_key": "hubkey"}, adminHeaders(), http.StatusOK)
		return e, kid
	}

	a, ava := setupHousehold("The Avas", "Ava", "Biscuit")
	b, bo := setupHousehold("The Bos", "Bo", "Clover")

	// Auth & missing ID checks on confirm endpoint.
	a.expectStatus(t, "POST", "/api/admin/friends/1/confirm", nil, nil, http.StatusUnauthorized)
	a.expectStatus(t, "POST", "/api/admin/friends/1/confirm", nil, childHeaders(ava), http.StatusForbidden)
	a.expectStatus(t, "POST", "/api/admin/friends/nope/confirm", nil, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "POST", "/api/admin/friends/999/confirm", nil, adminHeaders(), http.StatusNotFound)

	// Make invite on A: starts in waiting with initial minimal handshake uploaded.
	var aLink map[string]any
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &aLink)
	aID := int64(aLink["id"].(float64))
	aPath := fmt.Sprintf("/api/admin/friends/%d", aID)
	invite1 := aLink["invite"].(string)
	if aLink["status"] != "waiting" || aLink["side"] != "a" || aLink["peer_pending"] != false {
		t.Fatalf("new invite link: %v", aLink)
	}
	// Preconditions on waiting link: confirm -> 400, update -> 400.
	a.expectStatus(t, "POST", aPath+"/confirm", nil, adminHeaders(), http.StatusBadRequest)
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusBadRequest)

	// Even if a share row existed in SQLite before confirmation, Snapshot MUST
	// publish ONLY the minimal pending handshake (zero members/scores/streaks/pets).
	if err := a.store.SetFriendLinkShares(t.Context(), aID, []int64{int64(ava)}); err != nil {
		t.Fatal(err)
	}
	snapWaiting, err := friends.New(a.store).Snapshot(t.Context(), aID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapWaiting.Pending || !snapWaiting.NoVisits || snapWaiting.Family != "The Avas" ||
		snapWaiting.Today != "" || len(snapWaiting.Members) != 0 || len(snapWaiting.Accepted) != 0 ||
		len(snapWaiting.Team) != 0 || snapWaiting.Goal != nil || len(snapWaiting.Sends) != 0 {
		t.Fatalf("waiting snapshot leaked data: %+v", snapWaiting)
	}

	// B joins: immediately sees A's family name, peer_pending=true, last_error="" (no error banner!).
	var bLink map[string]any
	decodeBody(t, b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": invite1}, adminHeaders(), http.StatusCreated), &bLink)
	bPath := fmt.Sprintf("/api/admin/friends/%d", int(bLink["id"].(float64)))
	if bLink["status"] != "active" || bLink["side"] != "b" || bLink["peer_name"] != "The Avas" ||
		bLink["peer_pending"] != true || bLink["last_error"] != "" {
		t.Fatalf("B after join while A unconfirmed: %v", bLink)
	}
	// B offers Bo (with pet Clover) before A confirms.
	b.expectStatus(t, "PUT", bPath, map[string]any{"share_user_ids": []int{bo}}, adminHeaders(), http.StatusOK)

	// A syncs: moves waiting -> pending_confirm, clears invite, stores peer_name ONLY (snapshot == "").
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "pending_confirm" || aLink["peer_name"] != "The Bos" || aLink["invite"] != "" || len(aLink["inbound"].([]any)) != 0 {
		t.Fatalf("A in pending_confirm: %v", aLink)
	}
	var rawSnap string
	if err := a.db.QueryRow(`SELECT snapshot FROM friend_links WHERE id = ?`, aID).Scan(&rawSnap); err != nil || rawSnap != "" {
		t.Fatalf("A persisted peer snapshot before confirmation: err=%v snap=%q", err, rawSnap)
	}
	// Second sync while still pending_confirm MUST NOT auto-activate.
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "pending_confirm" {
		t.Fatalf("second sync auto-activated link: %v", aLink)
	}
	// Outgoing privacy while pending_confirm (even with share row in DB).
	snapPending, err := friends.New(a.store).Snapshot(t.Context(), aID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapPending.Pending || !snapPending.NoVisits || len(snapPending.Members) != 0 || len(snapPending.Team) != 0 {
		t.Fatalf("pending_confirm snapshot leaked data: %+v", snapPending)
	}
	// Incoming privacy while pending_confirm across all kid & wall-display endpoints.
	if lb := a.leaderboard(t); len(lb) != 1 || lb[0]["name"] != "Ava" {
		t.Fatalf("A leaderboard leaked unconfirmed peer: %v", lb)
	}
	if wall := a.getJSON(t, "/api/leaderboard/household", nil)["entries"].([]any); len(wall) != 1 {
		t.Fatalf("A wall leaderboard leaked unconfirmed peer: %v", wall)
	}
	if farms := a.getJSON(t, "/api/friends/farms", childHeaders(ava))["farms"].([]any); len(farms) != 0 {
		t.Fatalf("A farms leaked unconfirmed peer: %v", farms)
	}
	if vis := a.getJSON(t, "/api/friends/visitors", childHeaders(ava))["visitors"].([]any); len(vis) != 0 {
		t.Fatalf("A visitors leaked unconfirmed peer: %v", vis)
	}
	// UpdateLink rejected with 400 while pending_confirm.
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusBadRequest)
	// Clean up injected pre-share before testing normal flow.
	if err := a.store.SetFriendLinkShares(t.Context(), aID, nil); err != nil {
		t.Fatal(err)
	}

	// Online Decline on pending_confirm deletes link on A and ends it on B.
	a.expectStatus(t, "DELETE", aPath, nil, adminHeaders(), http.StatusNoContent)
	decodeBody(t, b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK), &bLink)
	if bLink["status"] != "ended" {
		t.Fatalf("B link after A declined online: %v", bLink)
	}
	b.expectStatus(t, "DELETE", bPath, nil, adminHeaders(), http.StatusNoContent)

	// Offline Decline while hub is down: deletes locally, queues friend_unlinks, RetryUnlinks tells hub once back.
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &aLink)
	aPath = fmt.Sprintf("/api/admin/friends/%d", int(aLink["id"].(float64)))
	decodeBody(t, b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": aLink["invite"].(string)}, adminHeaders(), http.StatusCreated), &bLink)
	bPath = fmt.Sprintf("/api/admin/friends/%d", int(bLink["id"].(float64)))
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "pending_confirm" {
		t.Fatalf("offline-decline setup: %v", aLink)
	}
	down.Store(true)
	a.expectStatus(t, "DELETE", aPath, nil, adminHeaders(), http.StatusNoContent)
	var queued int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM friend_unlinks`).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("offline decline queued unlinks = %d, err=%v", queued, err)
	}
	down.Store(false)
	friends.New(a.store).RetryUnlinks(t.Context())
	decodeBody(t, b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK), &bLink)
	if bLink["status"] != "ended" {
		t.Fatalf("B link after A offline decline retried: %v", bLink)
	}
	b.expectStatus(t, "DELETE", bPath, nil, adminHeaders(), http.StatusNoContent)

	// Confirm while hub is temporarily offline: flips to active with last_error set, then recovers when hub is back.
	decodeBody(t, a.expectStatus(t, "POST", "/api/admin/friends/invites", nil, adminHeaders(), http.StatusCreated), &aLink)
	aPath = fmt.Sprintf("/api/admin/friends/%d", int(aLink["id"].(float64)))
	decodeBody(t, b.expectStatus(t, "POST", "/api/admin/friends/join", map[string]any{"invite": aLink["invite"].(string)}, adminHeaders(), http.StatusCreated), &bLink)
	bPath = fmt.Sprintf("/api/admin/friends/%d", int(bLink["id"].(float64)))
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "pending_confirm" {
		t.Fatalf("offline-confirm setup: %v", aLink)
	}
	down.Store(true)
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/confirm", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "active" || aLink["last_error"] == "" {
		t.Fatalf("confirm while hub down: %v", aLink)
	}
	// Double confirm on now-active link returns 400.
	a.expectStatus(t, "POST", aPath+"/confirm", nil, adminHeaders(), http.StatusBadRequest)
	down.Store(false)
	decodeBody(t, a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK), &aLink)
	if aLink["status"] != "active" || aLink["last_error"] != "" {
		t.Fatalf("sync after hub recovered: %v", aLink)
	}
	// B syncs: peer_pending clears to false once A confirmed and synced.
	decodeBody(t, b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK), &bLink)
	if bLink["peer_pending"] != false {
		t.Fatalf("B peer_pending should clear after A confirms: %v", bLink)
	}

	// Verify FriendMember.Streak still round-trips after confirmation.
	a.expectStatus(t, "PUT", aPath, map[string]any{"share_user_ids": []int{ava}}, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "PUT", bPath, map[string]any{"accept_member_ids": []string{fmt.Sprint(ava)}}, adminHeaders(), http.StatusOK)
	a.expectStatus(t, "POST", aPath+"/sync", nil, adminHeaders(), http.StatusOK)
	b.expectStatus(t, "POST", bPath+"/sync", nil, adminHeaders(), http.StatusOK)
	var sawStreak bool
	for _, e := range b.leaderboard(t) {
		if e["name"] == "Ava" {
			if e["streak"] != 4.0 {
				t.Fatalf("Ava's streak on B's leaderboard = %v, want 4", e["streak"])
			}
			sawStreak = true
		}
	}
	if !sawStreak {
		t.Fatal("Ava not found on B's leaderboard for streak check")
	}
}
