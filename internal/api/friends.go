package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/liftedkilt/openchore/internal/friends"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/score"
	"github.com/liftedkilt/openchore/internal/store"
)

// The leaderboard and friend families (docs/friends.md). The leaderboard
// ranks people by chores done out of chores assigned this week; linking to
// another family's OpenChore (through a hub, internal/friends) adds their
// shared people to it and lets kids look at their farm.

const maxFamilyNameLen = 40

type FriendsHandler struct {
	store *store.Store
	svc   *friends.Service
}

func NewFriendsHandler(s *store.Store, svc *friends.Service) *FriendsHandler {
	svc.ReadPicture = readPetImage // approved custom animal pictures can go to friend families
	return &FriendsHandler{store: s, svc: svc}
}

func (h *FriendsHandler) leaderboardOn(r *http.Request) bool {
	v, _ := h.store.GetSetting(r.Context(), friends.SettingLeaderboard)
	return v == "true"
}

type leaderboardEntry struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Color  string `json:"color,omitempty"`
	Family string `json:"family,omitempty"`  // set for people from a friend family
	LinkID int64  `json:"link_id,omitempty"` // "
	UserID int64  `json:"user_id,omitempty"` // set for this household's people
	Streak int    `json:"streak"`
	Paused bool   `json:"paused,omitempty"`
	// Winner: came first last week, so wears the ribbon this week.
	Winner bool `json:"winner,omitempty"`
	score.Total
}

// Leaderboard returns this week's (or ?week=last) standings: everyone in the
// household plus the people friend families share. Signed-in only.
func (h *FriendsHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	h.leaderboard(w, r, true)
}

// HouseholdLeaderboard is the same for this household alone, with nobody
// from a friend family. It is public, like chores and points, so the wall
// display (which isn't signed in) can show it.
func (h *FriendsHandler) HouseholdLeaderboard(w http.ResponseWriter, r *http.Request) {
	h.leaderboard(w, r, false)
}

func (h *FriendsHandler) leaderboard(w http.ResponseWriter, r *http.Request, withFriends bool) {
	ctx := r.Context()
	now := h.svc.Now()
	last := r.URL.Query().Get("week") == "last"
	if last {
		now = now.AddDate(0, 0, -7)
	}
	from := score.WeekStart(now)
	out := map[string]any{
		"enabled": h.leaderboardOn(r), "week_start": from, "week_end": score.WeekEnd(from),
		"min_assigned": score.MinAssigned, "entries": []leaderboardEntry{}, "families": []any{}, "teams": []any{},
	}
	if !h.leaderboardOn(r) {
		writeJSON(w, http.StatusOK, out)
		return
	}
	st, err := h.standings(ctx, from, withFriends)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to work out the leaderboard")
		return
	}
	// Whoever came first last week wears the winner's ribbon this week.
	if !last {
		prev, err := h.standings(ctx, score.WeekStart(now.AddDate(0, 0, -7)), withFriends)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to work out the leaderboard")
			return
		}
		won := winners(prev.entries)
		for i := range st.entries {
			st.entries[i].Winner = won[st.entries[i].Key]
		}
	}
	if st.entries != nil {
		out["entries"] = st.entries
	}
	out["families"] = st.families
	if withFriends {
		out["teams"] = st.teams
	}
	writeJSON(w, http.StatusOK, out)
}

// standing is one week's leaderboard.
type standing struct {
	entries  []leaderboardEntry
	families []map[string]any
	teams    []teamView
}

// teamView is this household and one linked family as two teams for a week,
// with their team goal if they have one.
type teamView struct {
	LinkID int64    `json:"link_id"`
	Family string   `json:"family"`
	Ours   teamSide `json:"ours"`
	Theirs teamSide `json:"theirs"`
	// Together is both families' people averaged as one team.
	Together teamSide  `json:"together"`
	Goal     *goalView `json:"goal,omitempty"`
}

type teamSide struct {
	People int `json:"people"`
	Score  int `json:"score"` // 0 to 100, rounded down like a person's
}

func side(people, shareSum int) teamSide {
	if people <= 0 {
		return teamSide{}
	}
	return teamSide{People: people, Score: shareSum / people / 100}
}

// goalView is a team goal as one household sees it. State is "active" (both
// families have the same goal for this week), "ours" (we put one forward and
// they haven't agreed yet), "theirs" (they did, and we haven't) or "differs"
// (each put forward a different number).
type goalView struct {
	Target int    `json:"target"`
	State  string `json:"state"`
	// TheirTarget is set with state "differs": both families have put a
	// number forward and they aren't the same. Target is ours.
	TheirTarget int `json:"their_target,omitempty"`
	// Met: the two families together are at or above the target. For a
	// finished week that is the result; for this week, the score so far.
	Met bool `json:"met"`
}

// goalFor works out the goal between this household's link and the other
// family's snapshot for the week starting week.
func goalFor(l *model.FriendLink, snap *model.FriendSnapshot, week string, together teamSide) *goalView {
	ours, theirs := 0, 0
	if l.GoalWeek == week {
		ours = l.GoalTarget
	}
	if snap != nil && snap.Goal != nil && snap.Goal.Start == week {
		theirs = snap.Goal.Target
	}
	g := &goalView{}
	switch {
	case ours > 0 && ours == theirs:
		g.Target, g.State = ours, "active"
	case ours > 0 && theirs > 0:
		g.Target, g.State, g.TheirTarget = ours, "differs", theirs
	case ours > 0:
		g.Target, g.State = ours, "ours"
	case theirs > 0:
		g.Target, g.State = theirs, "theirs"
	default:
		return nil
	}
	g.Met = g.State == "active" && together.People > 0 && together.Score >= g.Target
	return g
}

// goalCelebration describes an agreed weekly team goal that two linked
// families have reached together, shown as a celebratory decoration on both
// farms for the rest of that week and the week after.
type goalCelebration struct {
	LinkID    int64  `json:"link_id"`
	Family    string `json:"family"`
	Week      string `json:"week"`       // "this" or "last"
	WeekStart string `json:"week_start"` // Monday YYYY-MM-DD
	Target    int    `json:"target"`
	Score     int    `json:"score"`
}

// goalCelebrations returns every active friend link whose agreed team goal for
// this week or last week has been reached.
func (h *FriendsHandler) goalCelebrations(ctx context.Context) ([]goalCelebration, error) {
	links, err := h.store.ListFriendLinks(ctx)
	if err != nil {
		return nil, err
	}
	return h.goalCelebrationsForLinks(ctx, links)
}

func (h *FriendsHandler) goalCelebrationsForLinks(ctx context.Context, links []model.FriendLink) ([]goalCelebration, error) {
	now := h.svc.Now()
	weeks := []struct {
		key   string
		start string
	}{
		{key: "this", start: score.WeekStart(now)},
		{key: "last", start: score.WeekStart(now.AddDate(0, 0, -7))},
	}

	type candidate struct {
		link *model.FriendLink
		snap *model.FriendSnapshot
		week string
		key  string
	}
	var cands []candidate
	for i := range links {
		l := &links[i]
		snap := friends.PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil || l.GoalTarget <= 0 || snap.Goal == nil {
			continue
		}
		for _, w := range weeks {
			if l.GoalWeek == w.start && snap.Goal.Start == w.start && l.GoalTarget == snap.Goal.Target {
				cands = append(cands, candidate{link: l, snap: snap, week: w.start, key: w.key})
				break
			}
		}
	}
	out := []goalCelebration{}
	if len(cands) == 0 {
		return out, nil
	}

	users, err := h.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	days, err := h.svc.Days(ctx, users)
	if err != nil {
		return nil, err
	}

	for _, c := range cands {
		ours, err := h.svc.TeamWeek(ctx, c.link, users, days, c.week)
		if err != nil {
			return nil, err
		}
		var theirs model.FriendTeamWeek
		for _, tw := range c.snap.Team {
			if tw.Start == c.week {
				theirs = tw
				break
			}
		}
		together := side(ours.People+theirs.People, ours.ShareSum+theirs.ShareSum)
		g := goalFor(c.link, c.snap, c.week, together)
		if g != nil && g.Met {
			out = append(out, goalCelebration{
				LinkID:    c.link.ID,
				Family:    familyLabel(c.link.PeerName),
				Week:      c.key,
				WeekStart: c.week,
				Target:    g.Target,
				Score:     together.Score,
			})
		}
	}
	return out, nil
}

// winners are the keys of whoever is ranked first, if there was anyone to
// beat: with a single ranked person there's no contest and no ribbon.
func winners(entries []leaderboardEntry) map[string]bool {
	ranked := 0
	for _, e := range entries {
		if e.Rank > 0 {
			ranked++
		}
	}
	out := map[string]bool{}
	if ranked < 2 {
		return out
	}
	for _, e := range entries {
		if e.Rank == 1 && e.Share > 0 { // a week where nobody did anything has no winner
			out[e.Key] = true
		}
	}
	return out
}

// standings works out the leaderboard for the week starting from.
func (h *FriendsHandler) standings(ctx context.Context, from string, withFriends bool) (*standing, error) {
	to := score.WeekEnd(from)
	st := &standing{families: []map[string]any{}, teams: []teamView{}}
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	days, err := h.svc.Days(ctx, users)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		t := score.Sum(days[u.ID], from, to)
		if t.Assigned == 0 && t.Bonus == 0 && u.Role != model.RoleChild {
			continue // a parent with no chores isn't playing
		}
		e := leaderboardEntry{Key: fmt.Sprintf("u%d", u.ID), Name: u.Name, Color: u.Color, UserID: u.ID, Paused: u.Paused, Total: t}
		if s, err := h.store.GetUserStreak(ctx, u.ID); err == nil && s != nil {
			e.Streak = s.CurrentStreak
		}
		st.entries = append(st.entries, e)
	}
	var links []model.FriendLink
	if withFriends {
		if links, err = h.store.ListFriendLinks(ctx); err != nil {
			return nil, err
		}
	}
	for i := range links {
		l := &links[i]
		snap := friends.PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil {
			continue
		}
		name := familyLabel(l.PeerName)
		// The two families as teams: each side's numbers cover the people it
		// shares with the other, so both households work out the same thing.
		ours, err := h.svc.TeamWeek(ctx, l, users, days, from)
		if err != nil {
			return nil, err
		}
		var theirs model.FriendTeamWeek
		for _, tw := range snap.Team {
			if tw.Start == from {
				theirs = tw
			}
		}
		if ours.People > 0 || theirs.People > 0 || l.GoalWeek == from || (snap.Goal != nil && snap.Goal.Start == from) {
			tv := teamView{LinkID: l.ID, Family: name, Ours: side(ours.People, ours.ShareSum), Theirs: side(theirs.People, theirs.ShareSum),
				Together: side(ours.People+theirs.People, ours.ShareSum+theirs.ShareSum)}
			tv.Goal = goalFor(l, snap, from, tv.Together)
			st.teams = append(st.teams, tv)
		}

		accepts, err := h.store.FriendLinkAccepts(ctx, l.ID)
		if err != nil {
			return nil, err
		}
		shown := friends.Shown(snap, accepts)
		if len(shown) == 0 {
			continue
		}
		pets := 0
		for _, m := range shown {
			if m.Pet != nil {
				pets++
			}
			st.entries = append(st.entries, leaderboardEntry{
				Key: fmt.Sprintf("f%d:%s", l.ID, m.ID), Name: m.Name, Color: m.Color, Family: name, LinkID: l.ID,
				Streak: m.Streak, Paused: m.Paused, Total: friendWeek(m.Weeks, from),
			})
		}
		st.families = append(st.families, map[string]any{"link_id": l.ID, "name": name, "snapshot_at": l.SnapshotAt, "pets": pets})
	}
	score.Rank(len(st.entries), func(i int) *score.Total { return &st.entries[i].Total },
		func(i, j int) { st.entries[i], st.entries[j] = st.entries[j], st.entries[i] })
	return st, nil
}

// Ribbons returns who in this household wears the winner's ribbon on the
// public farm (and the wall display) this week: whoever came first among the
// household last week. It deliberately leaves friend families out, so the
// public farm says nothing about them; signed-in screens take the ribbon
// from the full leaderboard instead. Empty when the leaderboard is off.
func (h *FriendsHandler) Ribbons(ctx context.Context) map[int64]bool {
	out := map[int64]bool{}
	if v, _ := h.store.GetSetting(ctx, friends.SettingLeaderboard); v != "true" {
		return out
	}
	prev, err := h.standings(ctx, score.WeekStart(h.svc.Now().AddDate(0, 0, -7)), false)
	if err != nil {
		return out
	}
	won := winners(prev.entries)
	for _, e := range prev.entries {
		if e.UserID != 0 && won[e.Key] {
			out[e.UserID] = true
		}
	}
	return out
}

// friendWeek is a friend's total for the week starting on start. Their weeks
// are their own (their time zone); one that doesn't line up with ours yet
// reads as nothing done so far.
func friendWeek(weeks []model.FriendWeek, start string) score.Total {
	for _, w := range weeks {
		if w.Start == start {
			return score.FromShare(w.Assigned, w.Done, w.Bonus, w.Share)
		}
	}
	return score.Total{}
}

func familyLabel(name string) string {
	if name == "" {
		return "Friend family"
	}
	return name
}

// Farms returns each friend family's farm as their last snapshot showed it,
// in the same shape as GET /api/pets so the same farm view can draw it.
func (h *FriendsHandler) Farms(w http.ResponseWriter, r *http.Request) {
	links, err := h.store.ListFriendLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list friend families")
		return
	}
	// A friend who came first on last week's leaderboard wears the ribbon here too.
	won := map[string]bool{}
	celebrationsByLink := map[int64]goalCelebration{}
	if h.leaderboardOn(r) {
		if prev, err := h.standings(r.Context(), score.WeekStart(h.svc.Now().AddDate(0, 0, -7)), true); err == nil {
			won = winners(prev.entries)
		}
		if goals, err := h.goalCelebrationsForLinks(r.Context(), links); err == nil {
			for _, gc := range goals {
				celebrationsByLink[gc.LinkID] = gc
			}
		}
	}
	farms := []map[string]any{}
	for i := range links {
		l := &links[i]
		snap := friends.PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil {
			continue
		}
		accepts, err := h.store.FriendLinkAccepts(r.Context(), l.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list friend families")
			return
		}
		shown := friends.Shown(snap, accepts)
		if len(shown) == 0 {
			continue
		}
		pets := []model.Pet{}
		for j, m := range shown {
			p := m.Pet
			if p == nil {
				continue
			}
			pets = append(pets, model.Pet{
				ID: int64(j + 1), Name: p.Name, Species: p.Species, Color: p.Color, Accessory: p.Accessory,
				Mood: p.Mood, Hiding: p.Hiding, Sick: p.Sick,
				SpriteData: friends.PictureURL(p.Picture), SpriteFrames: p.Frames,
				Ribbon: won[fmt.Sprintf("f%d:%s", l.ID, m.ID)], FriendKey: fmt.Sprintf("f%d:%s", l.ID, m.ID),
				Items: []string{}, OwnerName: m.Name, OwnerColor: m.Color,
			})
		}
		farm := map[string]any{"link_id": l.ID, "family": familyLabel(l.PeerName), "snapshot_at": l.SnapshotAt, "pets": pets}
		if gc, ok := celebrationsByLink[l.ID]; ok && len(pets) > 0 {
			farm["goal_celebration"] = gc
		}
		farms = append(farms, farm)
	}
	writeJSON(w, http.StatusOK, map[string]any{"farms": farms})
}

// Visitors returns the friend families' animals that are on this farm for a
// playdate today, the treats friends sent this household's animals today,
// and any agreed weekly team goals reached with linked families.
func (h *FriendsHandler) Visitors(w http.ResponseWriter, r *http.Request) {
	visitors, gifts, err := h.svc.Visitors(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list visitors")
		return
	}
	celebrations := []goalCelebration{}
	if petsEnabled(r.Context(), h.store) && h.leaderboardOn(r) {
		if got, err := h.goalCelebrations(r.Context()); err == nil {
			celebrations = got
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"visitors":          visitors,
		"gifts":             gifts,
		"goal_celebrations": celebrations,
	})
}

// --- Parents ---

// AdminGet returns everything Settings → Friend families shows.
func (h *FriendsHandler) AdminGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list people")
		return
	}
	links, err := h.store.ListFriendLinks(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list friend families")
		return
	}
	outLinks := []map[string]any{}
	for i := range links {
		v, err := h.linkView(ctx, &links[i], users)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list friend families")
			return
		}
		outLinks = append(outLinks, v)
	}
	hubURL, hubKey, fromEnv := h.svc.Hub(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"leaderboard_enabled": h.leaderboardOn(r),
		"family_name":         h.svc.FamilyName(ctx),
		"hub_url":             hubURL,
		"hub_key_set":         hubKey != "",
		"hub_from_env":        fromEnv,
		"links":               outLinks,
	})
}

// linkView is a linked family as a parent sees it, with both directions of
// sharing. A person is only visible to the other side when both parents
// picked them:
//
//	outbound: this household's people; shared = we send them to this
//	  family, accepted = that family has chosen to show them.
//	inbound: the people that family offers us; accepted = we show them.
func (h *FriendsHandler) linkView(ctx context.Context, l *model.FriendLink, users []model.User) (map[string]any, error) {
	if users == nil {
		var err error
		if users, err = h.store.ListUsers(ctx); err != nil {
			return nil, err
		}
	}
	shares, err := h.store.FriendLinkShares(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	accepts, err := h.store.FriendLinkAccepts(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	snap := friends.PeerSnapshot(l)
	theirs := map[string]bool{}
	inbound := []map[string]any{}
	if snap != nil {
		for _, id := range snap.Accepted {
			theirs[id] = true
		}
		for _, m := range snap.Members {
			inbound = append(inbound, map[string]any{"id": m.ID, "name": m.Name, "color": m.Color, "accepted": accepts[m.ID]})
		}
	}
	outbound := []map[string]any{}
	for _, u := range users {
		outbound = append(outbound, map[string]any{
			"id": u.ID, "name": u.Name, "color": u.Color, "role": u.Role,
			"shared": shares[u.ID], "accepted": shares[u.ID] && theirs[fmt.Sprint(u.ID)],
		})
	}
	return map[string]any{
		"id": l.ID, "status": l.Status, "peer_name": l.PeerName, "hub_url": l.HubURL, "show_pictures": l.ShowPictures, "allow_visits": l.AllowVisits,
		"goal":   goalFor(l, snap, score.WeekStart(h.svc.Now()), teamSide{}),
		"invite": l.Invite, "invite_expires": l.InviteExpires, "outbound": outbound, "inbound": inbound,
		"snapshot_at": l.SnapshotAt, "last_sync_at": l.LastSyncAt, "last_error": l.LastError, "created_at": l.CreatedAt,
	}, nil
}

// writeLink answers with one link's view.
func (h *FriendsHandler) writeLink(w http.ResponseWriter, r *http.Request, status int, id int64) {
	l, err := h.store.GetFriendLink(r.Context(), id)
	if err != nil || l == nil {
		writeError(w, http.StatusInternalServerError, "failed to load friend family")
		return
	}
	v, err := h.linkView(r.Context(), l, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load friend family")
		return
	}
	writeJSON(w, status, v)
}

// AdminUpdate changes the leaderboard switch, the family name and the hub.
func (h *FriendsHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LeaderboardEnabled *bool   `json:"leaderboard_enabled"`
		FamilyName         *string `json:"family_name"`
		HubURL             *string `json:"hub_url"`
		HubKey             *string `json:"hub_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx := r.Context()
	set := map[string]string{}
	if req.LeaderboardEnabled != nil {
		set[friends.SettingLeaderboard] = fmt.Sprint(*req.LeaderboardEnabled)
	}
	if req.FamilyName != nil {
		name := strings.TrimSpace(*req.FamilyName)
		if utf8.RuneCountInString(name) > maxFamilyNameLen {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("family name must be %d characters or fewer", maxFamilyNameLen))
			return
		}
		set[friends.SettingFamilyName] = name
	}
	if req.HubURL != nil || req.HubKey != nil {
		if _, _, fromEnv := h.svc.Hub(ctx); fromEnv {
			writeError(w, http.StatusConflict, "the coordination server is set by OPENCHORE_HUB_URL and can't be changed here")
			return
		}
	}
	if req.HubURL != nil {
		u := strings.TrimSpace(*req.HubURL)
		if u != "" {
			var err error
			if u, err = friends.CleanHubURL(u); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		set[friends.SettingHubURL] = u
	}
	if req.HubKey != nil {
		set[friends.SettingHubKey] = strings.TrimSpace(*req.HubKey)
	} else if u, changing := set[friends.SettingHubURL]; changing {
		// A key belongs to one hub: never carry it over to a different address.
		if old, _, _ := h.svc.Hub(ctx); old != u {
			set[friends.SettingHubKey] = ""
		}
	}
	for k, v := range set {
		if err := h.store.SetSetting(ctx, k, v); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	h.AdminGet(w, r)
}

// UpdateLink sets, for one linked family, who this household shares with
// them (share_user_ids), which of the people they offer are shown here
// (accept_member_ids), and whether their custom animal pictures are shown
// (show_pictures). Each list replaces the previous one; a list left out
// isn't changed. It then syncs, so the other family isn't left with a stale
// copy for 15 minutes.
func (h *FriendsHandler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	l, _ := h.store.GetFriendLink(ctx, id)
	if l == nil {
		writeError(w, http.StatusNotFound, "friend family not found")
		return
	}
	var req struct {
		Share        *[]int64  `json:"share_user_ids"`
		Accept       *[]string `json:"accept_member_ids"`
		ShowPictures *bool     `json:"show_pictures"`
		// GoalTarget puts forward (or agrees to) a team goal for this week:
		// the score, 1 to 100, the two families aim to average together.
		// 0 drops it.
		GoalTarget *int `json:"goal_target"`
		// AllowVisits: take gifts and playdates from this family.
		AllowVisits *bool `json:"allow_visits"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Share == nil && req.Accept == nil && req.ShowPictures == nil && req.GoalTarget == nil && req.AllowVisits == nil) {
		writeError(w, http.StatusBadRequest, "share_user_ids, accept_member_ids, show_pictures, allow_visits or goal_target is required")
		return
	}
	if req.AllowVisits != nil {
		if err := h.svc.SetAllowVisits(ctx, id, *req.AllowVisits); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	if req.GoalTarget != nil {
		week, target := score.WeekStart(h.svc.Now()), *req.GoalTarget
		if target < 0 || target > 100 {
			writeError(w, http.StatusBadRequest, "goal_target must be a score from 1 to 100, or 0 for no goal")
			return
		}
		if target == 0 {
			week = ""
		}
		if err := h.store.SetFriendGoal(ctx, id, week, target); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	if req.ShowPictures != nil {
		if err := h.svc.SetShowPictures(ctx, id, *req.ShowPictures); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	if req.Share != nil {
		users, err := h.store.ListUsers(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list people")
			return
		}
		known := map[int64]bool{}
		for _, u := range users {
			known[u.ID] = true
		}
		for _, uid := range *req.Share {
			if !known[uid] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("person %d not found", uid))
				return
			}
		}
		if err := h.store.SetFriendLinkShares(ctx, id, *req.Share); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	if req.Accept != nil {
		// Only people the other family actually offers can be accepted.
		offered := map[string]bool{}
		if snap := friends.PeerSnapshot(l); snap != nil {
			for _, m := range snap.Members {
				offered[m.ID] = true
			}
		}
		for _, mid := range *req.Accept {
			if !offered[mid] {
				writeError(w, http.StatusBadRequest, "that person isn't being shared by the other family")
				return
			}
		}
		if err := h.svc.SetAccepts(ctx, id, *req.Accept); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save")
			return
		}
	}
	if l.Status != model.FriendStatusEnded {
		_ = h.svc.Sync(ctx, id) // a failure is recorded on the link
	}
	h.writeLink(w, r, http.StatusOK, id)
}

func (h *FriendsHandler) needName(w http.ResponseWriter, r *http.Request) bool {
	if h.svc.FamilyName(r.Context()) == "" {
		writeError(w, http.StatusBadRequest, "give your family a name first, so the other family knows who you are")
		return false
	}
	return true
}

func (h *FriendsHandler) hubFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, friends.ErrNoHub) {
		writeError(w, http.StatusBadRequest, "set a coordination server first")
		return
	}
	if friends.Refused(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusBadGateway, err.Error())
}

// CreateInvite makes an invite for another family to join.
func (h *FriendsHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	if !h.needName(w, r) {
		return
	}
	l, err := h.svc.CreateInvite(r.Context())
	if err != nil {
		h.hubFailure(w, err)
		return
	}
	_ = h.store.SetSetting(r.Context(), friends.SettingLeaderboard, "true")
	h.writeLink(w, r, http.StatusCreated, l.ID)
}

// Join links to another family with the invite they sent.
func (h *FriendsHandler) Join(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Invite string `json:"invite"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Invite) == "" {
		writeError(w, http.StatusBadRequest, "invite is required")
		return
	}
	if !h.needName(w, r) {
		return
	}
	l, err := h.svc.Join(r.Context(), req.Invite)
	if err != nil {
		h.hubFailure(w, err)
		return
	}
	_ = h.store.SetSetting(r.Context(), friends.SettingLeaderboard, "true")
	h.writeLink(w, r, http.StatusCreated, l.ID)
}

// SyncNow syncs one link straight away.
func (h *FriendsHandler) SyncNow(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if l, _ := h.store.GetFriendLink(r.Context(), id); l == nil {
		writeError(w, http.StatusNotFound, "friend family not found")
		return
	}
	if err := h.svc.Sync(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.writeLink(w, r, http.StatusOK, id)
}

// Unlink ends a link and removes the other family's data.
func (h *FriendsHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if l, _ := h.store.GetFriendLink(r.Context(), id); l == nil {
		writeError(w, http.StatusNotFound, "friend family not found")
		return
	}
	if err := h.svc.Unlink(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
