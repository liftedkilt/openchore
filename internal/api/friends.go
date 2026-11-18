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
	score.Total
}

// Leaderboard returns this week's (or ?week=last) standings: everyone in the
// household plus the people friend families share.
func (h *FriendsHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := h.svc.Now()
	if r.URL.Query().Get("week") == "last" {
		now = now.AddDate(0, 0, -7)
	}
	from := score.WeekStart(now)
	to := score.WeekEnd(from)
	out := map[string]any{
		"enabled": h.leaderboardOn(r), "week_start": from, "week_end": to,
		"min_assigned": score.MinAssigned, "entries": []leaderboardEntry{}, "families": []any{},
	}
	if !h.leaderboardOn(r) {
		writeJSON(w, http.StatusOK, out)
		return
	}
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list people")
		return
	}
	days, err := h.svc.Days(ctx, users)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count chores")
		return
	}
	var entries []leaderboardEntry
	for _, u := range users {
		t := score.Sum(days[u.ID], from, to)
		if t.Assigned == 0 && t.Bonus == 0 && u.Role != model.RoleChild {
			continue // a parent with no chores isn't playing
		}
		e := leaderboardEntry{Key: fmt.Sprintf("u%d", u.ID), Name: u.Name, Color: u.Color, UserID: u.ID, Paused: u.Paused, Total: t}
		if st, err := h.store.GetUserStreak(ctx, u.ID); err == nil && st != nil {
			e.Streak = st.CurrentStreak
		}
		entries = append(entries, e)
	}
	links, err := h.store.ListFriendLinks(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list friend families")
		return
	}
	families := []map[string]any{}
	for i := range links {
		l := &links[i]
		snap := friends.PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil {
			continue
		}
		accepts, err := h.store.FriendLinkAccepts(ctx, l.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list friend families")
			return
		}
		shown := friends.Shown(snap, accepts)
		if len(shown) == 0 {
			continue
		}
		name := familyLabel(l.PeerName)
		pets := 0
		for _, m := range shown {
			if m.Pet != nil {
				pets++
			}
			entries = append(entries, leaderboardEntry{
				Key: fmt.Sprintf("f%d:%s", l.ID, m.ID), Name: m.Name, Color: m.Color, Family: name, LinkID: l.ID,
				Streak: m.Streak, Paused: m.Paused, Total: friendWeek(m.Weeks, from),
			})
		}
		families = append(families, map[string]any{"link_id": l.ID, "name": name, "snapshot_at": l.SnapshotAt, "pets": pets})
	}
	score.Rank(len(entries), func(i int) *score.Total { return &entries[i].Total },
		func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	if entries != nil {
		out["entries"] = entries
	}
	out["families"] = families
	writeJSON(w, http.StatusOK, out)
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
				Items: []string{}, OwnerName: m.Name, OwnerColor: m.Color,
			})
		}
		farms = append(farms, map[string]any{"link_id": l.ID, "family": familyLabel(l.PeerName), "snapshot_at": l.SnapshotAt, "pets": pets})
	}
	writeJSON(w, http.StatusOK, map[string]any{"farms": farms})
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
		"id": l.ID, "status": l.Status, "peer_name": l.PeerName, "hub_url": l.HubURL,
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
// them (share_user_ids) and which of the people they offer are shown here
// (accept_member_ids). Each list replaces the previous one; a list left out
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
		Share  *[]int64  `json:"share_user_ids"`
		Accept *[]string `json:"accept_member_ids"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Share == nil && req.Accept == nil) {
		writeError(w, http.StatusBadRequest, "share_user_ids or accept_member_ids is required")
		return
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
