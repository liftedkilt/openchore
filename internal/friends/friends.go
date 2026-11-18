// Package friends links this household to other families' OpenChore
// instances ("friend families") for a shared leaderboard and farm visits.
//
// The instances never talk to each other. Each one calls a hub
// (internal/hub) every few minutes, leaves an encrypted snapshot of the
// people a parent chose to share, and picks up the other family's. The key
// is made with the invite and only the two families hold it. See
// docs/friends.md.
package friends

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/pet"
	"github.com/liftedkilt/openchore/internal/score"
	"github.com/liftedkilt/openchore/internal/store"
)

const (
	// Settings (app_settings keys).
	SettingFamilyName  = "friends_family_name"
	SettingHubURL      = "friends_hub_url"
	SettingHubKey      = "friends_hub_key" // secret
	SettingLeaderboard = "leaderboard_enabled"

	// SnapshotDays is how many days of chore counts are kept to score from:
	// enough for this week and last.
	SnapshotDays = 14
	// recomputeDays is how far back finished days are recounted, so a late
	// approval or rejection still lands in the score.
	recomputeDays = 7

	snapshotVersion = 1
	invitePrefix    = "ocf1."
	syncInterval    = 15 * time.Minute
	maxNameLen      = 40
)

// ErrNoHub is returned when an invite is asked for and no hub is set.
var ErrNoHub = errors.New("no coordination server is set")

// Service does the scoring catch-up and all the talking to hubs.
type Service struct {
	store  *store.Store
	client *http.Client
	now    func() time.Time
	mu     sync.Mutex // one sync at a time
}

func New(s *store.Store) *Service {
	return &Service{store: s, client: &http.Client{Timeout: 20 * time.Second}, now: time.Now}
}

// SetClock overrides the clock (tests).
func (f *Service) SetClock(now func() time.Time) { f.now = now }

// Now is the service's clock.
func (f *Service) Now() time.Time { return f.now() }

// Start runs the catch-up and sync loop until ctx is done.
func (f *Service) Start(ctx context.Context) {
	tick := time.NewTicker(syncInterval)
	defer tick.Stop()
	for {
		if err := f.CatchUp(ctx, true); err != nil {
			log.Printf("friends: scoring: %v", err)
		}
		f.SyncAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// --- Scores ---

// CatchUp stores a score row for every person's finished days in the
// snapshot window. With recompute, recent days are counted again; without,
// only missing days are filled in (cheap enough to call before a read).
//
// Paused is a current flag with no history, so a day is marked paused if the
// person is paused when the day is first counted, and a paused row is never
// recounted.
func (f *Service) CatchUp(ctx context.Context, recompute bool) error {
	now := f.now()
	users, err := f.store.ListUsers(ctx)
	if err != nil {
		return err
	}
	from := now.AddDate(0, 0, -(SnapshotDays - 1)).Format(model.DateFormat)
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
	recountFrom := now.AddDate(0, 0, -recomputeDays).Format(model.DateFormat)
	stored, err := f.store.ListScoreDays(ctx, from, yesterday)
	if err != nil {
		return err
	}
	for _, u := range users {
		have := map[string]model.ScoreDay{}
		for _, d := range stored[u.ID] {
			have[d.Date] = d
		}
		created := u.CreatedAt.In(now.Location()).Format(model.DateFormat)
		for i := SnapshotDays - 1; i >= 1; i-- {
			date := now.AddDate(0, 0, -i).Format(model.DateFormat)
			if date < created {
				continue
			}
			old, exists := have[date]
			if exists && (old.Paused || u.Paused || !recompute || date < recountFrom) {
				continue
			}
			day := model.ScoreDay{Date: date, Paused: true}
			if !u.Paused {
				chores, err := f.store.GetScheduledChoresForUser(ctx, u.ID, []string{date}, now)
				if err != nil {
					return err
				}
				day = score.SummarizeDay(date, chores, true)
			}
			if exists && old == day {
				continue
			}
			if err := f.store.SaveScoreDay(ctx, u.ID, day); err != nil {
				return err
			}
		}
	}
	return nil
}

// Days returns every person's days for the snapshot window: stored rows for
// finished days plus today, counted live.
func (f *Service) Days(ctx context.Context, users []model.User) (map[int64][]model.ScoreDay, error) {
	if err := f.CatchUp(ctx, false); err != nil {
		return nil, err
	}
	now := f.now()
	today := now.Format(model.DateFormat)
	from := now.AddDate(0, 0, -(SnapshotDays - 1)).Format(model.DateFormat)
	days, err := f.store.ListScoreDays(ctx, from, today)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		d := model.ScoreDay{Date: today, Paused: true}
		if !u.Paused {
			chores, err := f.store.GetScheduledChoresForUser(ctx, u.ID, []string{today}, now)
			if err != nil {
				return nil, err
			}
			d = score.SummarizeDay(today, chores, false)
		}
		days[u.ID] = append(days[u.ID], d)
	}
	return days, nil
}

// --- Snapshots ---

// FamilyName is the name other families see.
func (f *Service) FamilyName(ctx context.Context) string {
	v, _ := f.store.GetSetting(ctx, SettingFamilyName)
	return v
}

// Snapshot builds what this family shares with one linked family: only the
// people a parent picked for that family, and nothing but names, colours,
// weekly scores and chore counts, streaks and farm animals.
func (f *Service) Snapshot(ctx context.Context, linkID int64) (*model.FriendSnapshot, error) {
	now := f.now()
	snap := &model.FriendSnapshot{
		Version:     snapshotVersion,
		Family:      f.FamilyName(ctx),
		GeneratedAt: now.UTC(),
		Today:       now.Format(model.DateFormat),
		Members:     []model.FriendMember{},
		Accepted:    []string{},
	}
	accepts, err := f.store.FriendLinkAccepts(ctx, linkID)
	if err != nil {
		return nil, err
	}
	for id := range accepts {
		snap.Accepted = append(snap.Accepted, id)
	}
	sort.Strings(snap.Accepted)
	shared, err := f.store.FriendLinkShares(ctx, linkID)
	if err != nil || len(shared) == 0 {
		return snap, err
	}
	users, err := f.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	days, err := f.Days(ctx, users)
	if err != nil {
		return nil, err
	}
	pets := map[int64]*model.Pet{}
	if v, _ := f.store.GetSetting(ctx, "pets_enabled"); v == "true" {
		all, err := f.store.ListPets(ctx)
		if err != nil {
			return nil, err
		}
		for i := range all {
			pets[all[i].UserID] = &all[i]
		}
	}
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
	for _, u := range users {
		if !shared[u.ID] {
			continue
		}
		m := model.FriendMember{ID: strconv.FormatInt(u.ID, 10), Name: u.Name, Color: u.Color, Paused: u.Paused}
		// Two weekly totals, not the days behind them, and the share
		// without the point values it came from.
		for _, start := range []string{score.WeekStart(now.AddDate(0, 0, -7)), score.WeekStart(now)} {
			t := score.Sum(days[u.ID], start, score.WeekEnd(start))
			m.Weeks = append(m.Weeks, model.FriendWeek{Start: start, Assigned: t.Assigned, Done: t.Done, Bonus: t.Bonus, Share: t.Share})
		}
		if st, err := f.store.GetUserStreak(ctx, u.ID); err == nil && st != nil {
			m.Streak = st.CurrentStreak
		}
		if p := pets[u.ID]; p != nil {
			chores, err := f.store.GetScheduledChoresForUser(ctx, u.ID, []string{snap.Today}, now)
			if err != nil {
				return nil, err
			}
			pet.ShowToday(p, chores)
			pet.Describe(p, yesterday, pet.CaughtUp(chores))
			acc := p.Accessory
			if strings.HasPrefix(acc, "img:") {
				acc = "" // drawn accessories are pictures on this server; not sent
			}
			m.Pet = &model.FriendPet{Name: p.Name, Species: p.Species, Color: p.Color, Accessory: acc,
				Mood: p.Mood, Hiding: p.Hiding, Sick: p.Sick}
		}
		snap.Members = append(snap.Members, m)
	}
	return snap, nil
}

// Shown returns the members of a peer snapshot that a parent here chose to
// show. Only these appear on the leaderboard and in farm visits.
func Shown(snap *model.FriendSnapshot, accepts map[string]bool) []model.FriendMember {
	var out []model.FriendMember
	for _, m := range snap.Members {
		if accepts[m.ID] {
			out = append(out, m)
		}
	}
	return out
}

// PeerSnapshot decodes a link's stored snapshot of the other family (nil if
// there isn't one yet).
func PeerSnapshot(l *model.FriendLink) *model.FriendSnapshot {
	if l.Snapshot == "" {
		return nil
	}
	var s model.FriendSnapshot
	if json.Unmarshal([]byte(l.Snapshot), &s) != nil {
		return nil
	}
	return &s
}

// --- Encryption (the hub only ever sees sealed data) ---

func aead(secret string) (cipher.AEAD, error) {
	key, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(key) != 32 {
		return nil, errors.New("bad link key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(l *model.FriendLink, plain []byte) ([]byte, error) {
	a, err := aead(l.Secret)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return a.Seal(nonce, nonce, plain, []byte(l.HubLink)), nil
}

func open(l *model.FriendLink, sealed []byte) ([]byte, error) {
	a, err := aead(l.Secret)
	if err != nil {
		return nil, err
	}
	if len(sealed) < a.NonceSize() {
		return nil, errors.New("snapshot too short")
	}
	return a.Open(nil, sealed[:a.NonceSize()], sealed[a.NonceSize():], []byte(l.HubLink))
}

// --- Invites ---

// invite is what one parent hands the other: where the hub is, which link,
// the one-time code that lets them join it, and the key for the snapshots.
type invite struct {
	Hub  string `json:"h"`
	Link string `json:"l"`
	Code string `json:"c"`
	Key  string `json:"k"`
}

func (i invite) String() string {
	b, _ := json.Marshal(i)
	return invitePrefix + base64.RawURLEncoding.EncodeToString(b)
}

func parseInvite(s string) (*invite, error) {
	bad := errInput{errors.New("that doesn't look like an OpenChore invite")}
	s = strings.Join(strings.Fields(s), "") // pasted codes pick up spaces and line breaks
	rest, ok := strings.CutPrefix(s, invitePrefix)
	if !ok {
		return nil, bad
	}
	raw, err := base64.RawURLEncoding.DecodeString(rest)
	if err != nil {
		return nil, bad
	}
	var inv invite
	if json.Unmarshal(raw, &inv) != nil || inv.Link == "" || inv.Code == "" || inv.Key == "" {
		return nil, bad
	}
	if _, err := cleanHubURL(inv.Hub); err != nil {
		return nil, bad
	}
	return &inv, nil
}

func cleanHubURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", errors.New("the coordination server address must start with https://")
	}
	return raw, nil
}

// CleanHubURL validates a hub address typed by a parent.
func CleanHubURL(raw string) (string, error) { return cleanHubURL(raw) }

// Hub returns the hub this household creates invites on, and its key.
// OPENCHORE_HUB_URL / OPENCHORE_HUB_KEY win over Settings.
func (f *Service) Hub(ctx context.Context) (hubURL, key string, fromEnv bool) {
	if v := os.Getenv("OPENCHORE_HUB_URL"); v != "" {
		return strings.TrimRight(v, "/"), os.Getenv("OPENCHORE_HUB_KEY"), true
	}
	hubURL, _ = f.store.GetSetting(ctx, SettingHubURL)
	key, _ = f.store.GetSetting(ctx, SettingHubKey)
	return hubURL, key, false
}

// --- Hub calls ---

type hubError struct {
	status int
	msg    string
}

func (e *hubError) Error() string { return e.msg }

// errInput marks a mistake in what the parent typed or pasted.
type errInput struct{ error }

// Refused reports whether err is the parent's to fix (a bad or used invite,
// a wrong hub key) rather than the hub being unreachable or broken.
func Refused(err error) bool {
	var in errInput
	var he *hubError
	return errors.As(err, &in) || (errors.As(err, &he) && he.status >= 400 && he.status < 500)
}

func (f *Service) call(ctx context.Context, method, target, token string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach the coordination server: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("the coordination server answered %d", resp.StatusCode)
		}
		return &hubError{resp.StatusCode, e.Error}
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// CreateInvite makes a new link on this household's hub and returns it,
// waiting, with the invite to hand to the other family.
func (f *Service) CreateInvite(ctx context.Context) (*model.FriendLink, error) {
	hubURL, key, _ := f.Hub(ctx)
	if hubURL == "" {
		return nil, ErrNoHub
	}
	var res struct {
		Link      string    `json:"link"`
		Token     string    `json:"token"`
		JoinCode  string    `json:"join_code"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := f.call(ctx, http.MethodPost, hubURL+"/v1/links", key, nil, &res); err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	l := &model.FriendLink{Status: model.FriendStatusWaiting, HubURL: hubURL, HubLink: res.Link, Token: res.Token,
		Secret: base64.RawURLEncoding.EncodeToString(secret), InviteExpires: &res.ExpiresAt}
	l.Invite = invite{Hub: hubURL, Link: res.Link, Code: res.JoinCode, Key: l.Secret}.String()
	if err := f.store.CreateFriendLink(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Join uses another family's invite, then syncs once so their names show up
// straight away.
func (f *Service) Join(ctx context.Context, code string) (*model.FriendLink, error) {
	inv, err := parseInvite(code)
	if err != nil {
		return nil, err
	}
	links, err := f.store.ListFriendLinks(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		if l.HubLink == inv.Link && l.HubURL == inv.Hub {
			return nil, errInput{errors.New("that invite is this family's own, or has already been used here")}
		}
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := f.call(ctx, http.MethodPost, inv.Hub+"/v1/links/"+url.PathEscape(inv.Link)+"/join", "",
		map[string]string{"code": inv.Code}, &res); err != nil {
		return nil, err
	}
	l := &model.FriendLink{Status: model.FriendStatusActive, HubURL: inv.Hub, HubLink: inv.Link, Token: res.Token, Secret: inv.Key}
	if err := f.store.CreateFriendLink(ctx, l); err != nil {
		return nil, err
	}
	_ = f.Sync(ctx, l.ID)
	return f.store.GetFriendLink(ctx, l.ID)
}

// Sync leaves this family's snapshot at the hub and picks up the other's.
// Failures are recorded on the link (and returned).
func (f *Service) Sync(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil || l.Status == model.FriendStatusEnded {
		return err
	}
	now := f.now()
	fail := func(err error) error {
		_ = f.store.SaveFriendSync(ctx, id, l.Status, "", "", nil, err.Error(), now)
		return err
	}
	snap, err := f.Snapshot(ctx, id)
	if err != nil {
		return fail(err)
	}
	plain, _ := json.Marshal(snap)
	sealed, err := seal(l, plain)
	if err != nil {
		return fail(err)
	}
	var res struct {
		Joined bool `json:"joined"`
		Peer   *struct {
			Data []byte `json:"data"`
		} `json:"peer"`
	}
	err = f.call(ctx, http.MethodPost, l.HubURL+"/v1/links/"+url.PathEscape(l.HubLink)+"/exchange", l.Token,
		map[string]any{"data": sealed}, &res)
	var he *hubError
	if errors.As(err, &he) && (he.status == http.StatusGone || he.status == http.StatusNotFound) {
		// The other family unlinked (or the invite ran out): drop their data.
		return f.store.SaveFriendSync(ctx, id, model.FriendStatusEnded, "", "", nil, "", now)
	}
	if err != nil {
		return fail(err)
	}
	status := l.Status
	if res.Joined {
		status = model.FriendStatusActive
	}
	peerName, peerJSON := "", ""
	var peerAt *time.Time
	if res.Peer != nil {
		raw, err := open(l, res.Peer.Data)
		if err != nil {
			return fail(errors.New("couldn't read the other family's data (wrong key?)"))
		}
		var ps model.FriendSnapshot
		if err := json.Unmarshal(raw, &ps); err != nil || ps.Version != snapshotVersion {
			return fail(errors.New("the other family runs a version of OpenChore this one can't read"))
		}
		// Never go back to an older snapshot than the one already held.
		if old := PeerSnapshot(l); old == nil || !ps.GeneratedAt.Before(old.GeneratedAt) {
			sanitize(&ps)
			// Keep nothing but the name of anyone a parent here hasn't
			// picked: enough to offer them, no more.
			accepts, err := f.store.FriendLinkAccepts(ctx, id)
			if err != nil {
				return fail(err)
			}
			for i, m := range ps.Members {
				if !accepts[m.ID] {
					ps.Members[i] = model.FriendMember{ID: m.ID, Name: m.Name, Color: m.Color, Weeks: []model.FriendWeek{}}
				}
			}
			clean, _ := json.Marshal(ps)
			peerName, peerJSON = ps.Family, string(clean)
			at := ps.GeneratedAt
			peerAt = &at
		}
	}
	return f.store.SaveFriendSync(ctx, id, status, peerName, peerJSON, peerAt, "", now)
}

// sanitize bounds what another family's snapshot can put on our screens.
func sanitize(s *model.FriendSnapshot) {
	s.Family = clip(s.Family, maxNameLen)
	if len(s.Members) > 30 {
		s.Members = s.Members[:30]
	}
	if len(s.Accepted) > 100 {
		s.Accepted = s.Accepted[:100]
	}
	for i := range s.Accepted {
		s.Accepted[i] = clip(s.Accepted[i], 20)
	}
	for i := range s.Members {
		m := &s.Members[i]
		m.ID, m.Name, m.Color = clip(m.ID, 20), clip(m.Name, maxNameLen), clip(m.Color, 20)
		if len(m.Weeks) > 2 {
			m.Weeks = m.Weeks[len(m.Weeks)-2:]
		}
		for j := range m.Weeks {
			w := &m.Weeks[j]
			w.Start = clip(w.Start, 10)
			w.Assigned, w.Bonus = max(0, min(w.Assigned, 2000)), max(0, min(w.Bonus, 2000))
			w.Done = max(0, min(w.Done, w.Assigned))
			w.Share = max(0, min(w.Share, score.ShareScale))
		}
		if p := m.Pet; p != nil {
			// Only animals, coats and moods this version can draw.
			if !model.ValidPetSpecies(p.Species) {
				m.Pet = nil
				continue
			}
			p.Name = clip(p.Name, maxNameLen)
			if !model.ValidPetColor(p.Species, p.Color) {
				p.Color = ""
			}
			if !model.ValidPetItem(p.Accessory) || strings.HasPrefix(p.Accessory, "img:") {
				p.Accessory = ""
			}
			if !knownMoods[p.Mood] {
				p.Mood = "content"
			}
		}
	}
}

var knownMoods = map[string]bool{
	"hiding": true, "very_sick": true, "sick": true, "hungry": true, "bored": true, "happy": true, "content": true,
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// SyncAll syncs every live link, logging failures.
func (f *Service) SyncAll(ctx context.Context) {
	links, err := f.store.ListFriendLinks(ctx)
	if err != nil {
		log.Printf("friends: listing links: %v", err)
		return
	}
	for _, l := range links {
		if l.Status == model.FriendStatusEnded {
			continue
		}
		if err := f.Sync(ctx, l.ID); err != nil {
			log.Printf("friends: sync with %q failed: %v", l.PeerName, err)
		}
	}
}

// Unlink ends a link: the hub deletes it (so the other family is told at
// their next sync) and the other family's data is removed here. If the hub
// can't be reached the link is still removed locally.
func (f *Service) Unlink(ctx context.Context, id int64) error {
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil {
		return err
	}
	if l.Status != model.FriendStatusEnded {
		if err := f.call(ctx, http.MethodDelete, l.HubURL+"/v1/links/"+url.PathEscape(l.HubLink), l.Token, nil, nil); err != nil {
			log.Printf("friends: couldn't tell the hub about unlinking %q: %v", l.PeerName, err)
		}
	}
	return f.store.DeleteFriendLink(ctx, id)
}
