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
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/pet"
	"github.com/liftedkilt/openchore/internal/score"
	"github.com/liftedkilt/openchore/internal/sprite"
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

	// A custom animal picture is sent only if it's this small, and a
	// snapshot carries at most this much of them (the hub caps the whole
	// snapshot at 256 KB).
	maxPictureBytes  = 48 << 10
	maxPicturesTotal = 150 << 10

	snapshotVersion = 1
	invitePrefix    = "ocf1."
	syncInterval    = 15 * time.Minute
	maxNameLen      = 40
)

// ErrNoHub is returned when an invite is asked for and no hub is set.
var ErrNoHub = errors.New("no coordination server is set")

// Service does the scoring catch-up and all the talking to hubs.
type Service struct {
	store   *store.Store
	client  *http.Client // for this household's own hub
	guarded *http.Client // for a hub named by someone else's invite
	now     func() time.Time
	// ReadPicture loads a custom animal picture by its upload id (set by
	// the API layer, which owns the uploads folder). Nil = never send one.
	ReadPicture func(id string) ([]byte, error)
	mu          sync.Mutex // one sync at a time
}

func New(s *store.Store) *Service {
	return &Service{store: s, client: hubClient(false), guarded: hubClient(true), now: time.Now}
}

// hubClient makes the HTTP client for talking to hubs. It never follows
// redirects (a hub has no reason to send one). A guarded client also refuses
// to connect to anything but a public address: it is used for hubs named by
// an invite, which is text someone else wrote, so that a crafted invite
// can't point this server at its own network.
func hubClient(guard bool) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second}
	if guard {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !publicIP(ip) {
				return errPrivateHub
			}
			return nil
		}
	}
	return &http.Client{
		Timeout:       20 * time.Second,
		Transport:     &http.Transport{DialContext: d.DialContext, TLSHandshakeTimeout: 10 * time.Second, MaxIdleConns: 4, IdleConnTimeout: time.Minute},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

var errPrivateHub = errors.New("that invite points at a private network address, which isn't allowed")

// notPublic lists address ranges that are not ordinary internet hosts but
// that net.IP's own checks let through: carrier-grade NAT (also used by
// overlay networks), benchmarking and protocol-assignment blocks, reserved
// space and broadcast.
var notPublic = func() []netip.Prefix {
	var out []netip.Prefix
	for _, p := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"::/128", "::1/128", "100::/64", "2001:db8::/32",
	} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}()

// publicIP reports whether ip is an ordinary address on the public internet.
// IPv6 forms that carry an IPv4 address (mapped, compatible, NAT64, 6to4) are
// judged by the address inside them.
func publicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if addr.Is6() {
		b := addr.As16()
		var inner []byte
		switch {
		case bytes.Equal(b[:12], make([]byte, 12)): // ::a.b.c.d
			inner = b[12:]
		case bytes.Equal(b[:12], []byte{0, 0x64, 0xff, 0x9b, 0, 0, 0, 0, 0, 0, 0, 0}): // 64:ff9b::/96
			inner = b[12:]
		case b[0] == 0x20 && b[1] == 0x02: // 2002::/16
			inner = b[2:6]
		}
		if inner != nil {
			v4, _ := netip.AddrFromSlice(inner)
			return publicIP(v4.AsSlice())
		}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() || addr.IsInterfaceLocalMulticast() {
		return false
	}
	for _, p := range notPublic {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// trusts reports whether a hub may be on a private address: this
// household's own configured hub (it may well be on the home network), or
// any hub when OPENCHORE_HUB_ALLOW_PRIVATE=true says private hubs are
// expected. Every other hub came from someone else's invite and is guarded.
func (f *Service) trusts(ctx context.Context, hubURL string) bool {
	own, _, _ := f.Hub(ctx)
	return (own != "" && own == hubURL) || os.Getenv("OPENCHORE_HUB_ALLOW_PRIVATE") == "true"
}

// trustsLink is trusts for an existing link. A link this household made the
// invite for (side "a") was made on its own hub, and stays trusted even if
// the configured hub has since been changed.
func (f *Service) trustsLink(ctx context.Context, l *model.FriendLink) bool {
	return l.Side == "a" || f.trusts(ctx, l.HubURL)
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
// A finished day is counted against the chores that existed on that day: a
// schedule created afterwards is left out, so adding a chore today can't
// reach back and mark earlier days as missed.
//
// Paused is a current flag with no history. A day is marked paused when it
// is first counted while the person is paused and they did nothing that day;
// a paused row is never recounted. (A day they did do chores on is counted
// even if they have been paused since.)
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
			chores, err := f.choresOn(ctx, u.ID, date, now)
			if err != nil {
				return err
			}
			day := score.SummarizeDay(date, chores, true)
			if u.Paused && day.Done == 0 && day.Bonus == 0 {
				day = model.ScoreDay{Date: date, Paused: true}
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

// choresOn returns a person's chores for a finished day, without any
// schedule that was only created after that day ended.
func (f *Service) choresOn(ctx context.Context, userID int64, date string, now time.Time) ([]model.ScheduledChore, error) {
	chores, err := f.store.GetScheduledChoresForUser(ctx, userID, []string{date}, now)
	if err != nil {
		return nil, err
	}
	day, err := time.ParseInLocation(model.DateFormat, date, now.Location())
	if err != nil {
		return nil, err
	}
	later, err := f.store.SchedulesCreatedSince(ctx, userID, day.AddDate(0, 0, 1))
	if err != nil || len(later) == 0 {
		return chores, err
	}
	kept := chores[:0]
	for _, c := range chores {
		if !later[c.ScheduleID] {
			kept = append(kept, c)
		}
	}
	return kept, nil
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
	link, err := f.store.GetFriendLink(ctx, linkID)
	if err != nil {
		return nil, err
	}
	if link != nil && link.GoalTarget > 0 && link.GoalWeek != "" {
		snap.Goal = &model.FriendGoal{Start: link.GoalWeek, Target: link.GoalTarget}
	}
	if snap.Sends, err = f.store.FriendSendsSince(ctx, linkID, now.AddDate(0, 0, -1).Format(model.DateFormat)); err != nil {
		return nil, err
	}
	if link != nil {
		snap.NoVisits = !link.AllowVisits
	}
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
	// The team: the people shared with this family that it has chosen to
	// show, who had chores that week. A total never covers someone the
	// other parent hasn't accepted (it would give their score away).
	for _, start := range []string{score.WeekStart(now.AddDate(0, 0, -7)), score.WeekStart(now)} {
		snap.Team = append(snap.Team, teamWeek(users, days, teamOf(shared, link), start))
	}
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
	pictureBytes := 0
	accepted := teamOf(shared, link) // shared, and picked by the other family's parent
	for _, u := range users {
		if !shared[u.ID] {
			continue
		}
		if !accepted[u.ID] {
			// Not picked over there yet: only what it takes to offer them,
			// so nothing more leaves this household until both parents agree.
			snap.Members = append(snap.Members, model.FriendMember{ID: strconv.FormatInt(u.ID, 10), Name: u.Name, Color: u.Color, Weeks: []model.FriendWeek{}})
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
			// The animal's own picture goes along only once a parent here
			// has approved it for sharing, and only while there's room.
			if p.Sprite != "" && p.SpriteApproved && f.ReadPicture != nil {
				if raw, err := f.ReadPicture(p.Sprite); err == nil && len(raw) <= maxPictureBytes && pictureBytes+len(raw) <= maxPicturesTotal {
					m.Pet.Picture, m.Pet.Frames = raw, max(p.SpriteFrames, 1)
					pictureBytes += len(raw)
				}
			}
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

// teamOf is who plays for this household against a linked family: the people
// shared with that family whom the family has chosen to show.
func teamOf(shared map[int64]bool, l *model.FriendLink) map[int64]bool {
	team := map[int64]bool{}
	if l == nil {
		return team
	}
	if snap := PeerSnapshot(l); snap != nil {
		for _, id := range snap.Accepted {
			if uid, err := strconv.ParseInt(id, 10, 64); err == nil && shared[uid] {
				team[uid] = true
			}
		}
	}
	return team
}

func teamWeek(users []model.User, days map[int64][]model.ScoreDay, team map[int64]bool, start string) model.FriendTeamWeek {
	tw := model.FriendTeamWeek{Start: start}
	for _, u := range users {
		if !team[u.ID] {
			continue
		}
		if t := score.Sum(days[u.ID], start, score.WeekEnd(start)); t.Assigned > 0 {
			tw.People++
			tw.ShareSum += t.Share
		}
	}
	return tw
}

// TeamWeek is how this household's team against a linked family did in the
// week starting start: the same numbers its snapshot sends, so both families
// work out the same team scores.
func (f *Service) TeamWeek(ctx context.Context, l *model.FriendLink, users []model.User, days map[int64][]model.ScoreDay, start string) (model.FriendTeamWeek, error) {
	shared, err := f.store.FriendLinkShares(ctx, l.ID)
	if err != nil {
		return model.FriendTeamWeek{Start: start}, err
	}
	return teamWeek(users, days, teamOf(shared, l), start), nil
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

// aad binds sealed data to the link and to who sealed it, so a blob can't
// be handed back to its own sender as if it came from the other family.
func aad(l *model.FriendLink, side string) []byte { return []byte(l.HubLink + ":" + side) }

func otherSide(side string) string {
	if side == "a" {
		return "b"
	}
	return "a"
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
	return a.Seal(nonce, nonce, plain, aad(l, l.Side)), nil
}

func open(l *model.FriendLink, sealed []byte) ([]byte, error) {
	a, err := aead(l.Secret)
	if err != nil {
		return nil, err
	}
	if len(sealed) < a.NonceSize() {
		return nil, errors.New("snapshot too short")
	}
	return a.Open(nil, sealed[:a.NonceSize()], sealed[a.NonceSize():], aad(l, otherSide(l.Side)))
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
	// Keep the tidied form, so the same hub always compares equal.
	if inv.Hub, err = cleanHubURL(inv.Hub); err != nil {
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
	// Just an origin, optionally with a path prefix: nothing that could
	// smuggle a different request (query, fragment) or credentials.
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil || strings.ContainsAny(raw, "?# \t\r\n") {
		return "", errors.New("the coordination server address can't contain ?, # or a user name")
	}
	// Keys and tokens travel in the request, so plain http is only for a
	// hub on this machine or network.
	if u.Scheme == "http" {
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || publicIP(ip)) {
			return "", errors.New("the coordination server address must start with https://")
		}
	}
	// One spelling per hub: lower-case scheme and host, no trailing dot or slash.
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(strings.TrimSuffix(u.Host, ".")) + strings.TrimRight(u.EscapedPath(), "/"), nil
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

func (f *Service) call(ctx context.Context, trusted bool, method, hubURL, path, token string, body, out any) error {
	target := hubURL + path
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
	client := f.guarded
	if trusted {
		client = f.client
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, errPrivateHub) {
			return errInput{errPrivateHub}
		}
		// The details (addresses, ports) go to the log, not to the screen.
		log.Printf("friends: calling %s: %v", hubURL, err)
		return errors.New("couldn't reach the coordination server")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		// The hub's own words are shown to the parent, so keep them short
		// and plain whatever the hub sends.
		e.Error = strings.Map(func(r rune) rune {
			if r < ' ' || r == 0x7f {
				return ' '
			}
			return r
		}, clip(e.Error, 200))
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
	if err := f.call(ctx, true, http.MethodPost, hubURL, "/v1/links", key, nil, &res); err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	l := &model.FriendLink{Status: model.FriendStatusWaiting, Side: "a", HubURL: hubURL, HubLink: res.Link, Token: res.Token,
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
	if err := f.call(ctx, f.trusts(ctx, inv.Hub), http.MethodPost, inv.Hub, "/v1/links/"+url.PathEscape(inv.Link)+"/join", "",
		map[string]string{"code": inv.Code}, &res); err != nil {
		return nil, err
	}
	l := &model.FriendLink{Status: model.FriendStatusActive, Side: "b", HubURL: inv.Hub, HubLink: inv.Link, Token: res.Token, Secret: inv.Key}
	if err := f.store.CreateFriendLink(ctx, l); err != nil {
		return nil, err
	}
	_ = f.Sync(ctx, l.ID)
	return f.store.GetFriendLink(ctx, l.ID)
}

// Sync leaves this family's snapshot at the hub and picks up the other's.
// Failures are recorded on the link (and returned).
//
// A snapshot only carries full details for people the other family has
// accepted, and which those are comes back in the other family's snapshot.
// So when it shows someone newly accepted, the exchange is done once more
// straight away rather than leaving them a blank until the next round.
func (f *Service) Sync(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	again, err := f.syncOnce(ctx, id)
	if err == nil && again {
		_, err = f.syncOnce(ctx, id)
	}
	return err
}

// sentFull is who a link's snapshot currently carries full details for.
func (f *Service) sentFull(ctx context.Context, id int64) (map[int64]bool, error) {
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil {
		return nil, err
	}
	shared, err := f.store.FriendLinkShares(ctx, id)
	if err != nil {
		return nil, err
	}
	return teamOf(shared, l), nil
}

func (f *Service) syncOnce(ctx context.Context, id int64) (again bool, err error) {
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil || l.Status == model.FriendStatusEnded {
		return false, err
	}
	before, err := f.sentFull(ctx, id)
	if err != nil {
		return false, err
	}
	if err := f.exchange(ctx, l); err != nil {
		return false, err
	}
	after, err := f.sentFull(ctx, id)
	if err != nil {
		return false, err
	}
	for uid := range after {
		if !before[uid] {
			return true, nil
		}
	}
	return false, nil
}

func (f *Service) exchange(ctx context.Context, l *model.FriendLink) error {
	id := l.ID
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
	err = f.call(ctx, f.trustsLink(ctx, l), http.MethodPost, l.HubURL, "/v1/links/"+url.PathEscape(l.HubLink)+"/exchange", l.Token,
		map[string]any{"data": sealed}, &res)
	var he *hubError
	// Only the hub's own "gone" ends a link. A 404 can come from anything in
	// front of the hub (a proxy mid-restart), and ending is not undoable.
	if errors.As(err, &he) && he.status == http.StatusGone {
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
		// The peer's clock isn't trusted: a time in the future would block
		// every later snapshot from replacing this one.
		if ps.GeneratedAt.After(now.Add(10 * time.Minute)) {
			ps.GeneratedAt = now.UTC()
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
			strip(&ps, accepts)
			keepPictures(&ps, l.ShowPictures)
			if err := f.receive(ctx, l, &ps, accepts, now); err != nil {
				return fail(err)
			}
			ps.Sends = nil // applied (or refused) above; not kept with the snapshot
			clean, _ := json.Marshal(ps)
			peerName, peerJSON = ps.Family, string(clean)
			at := ps.GeneratedAt
			peerAt = &at
		}
	}
	return f.store.SaveFriendSync(ctx, id, status, peerName, peerJSON, peerAt, "", now)
}

// strip cuts every member a parent here hasn't accepted down to what's
// needed to offer them: id, name and colour.
func strip(s *model.FriendSnapshot, accepts map[string]bool) {
	for i, m := range s.Members {
		if !accepts[m.ID] {
			s.Members[i] = model.FriendMember{ID: m.ID, Name: m.Name, Color: m.Color, Weeks: []model.FriendWeek{}}
		}
	}
}

// What a gift does to the animal that gets it: the same as a treat.
const (
	giftFullness = 15
	giftJoy      = 20
	// maxGiftsPerDay is how many gifts one animal takes from one family in a
	// day, whatever that family's server says.
	maxGiftsPerDay = 3
)

// receive applies the gifts and playdates in a peer's snapshot, each once.
// A gift only lands on an animal whose owner this household shares with that
// family; a playdate only counts from a person a parent here has accepted.
func (f *Service) receive(ctx context.Context, l *model.FriendLink, s *model.FriendSnapshot, accepts map[string]bool, now time.Time) error {
	if !l.AllowVisits || len(s.Sends) == 0 {
		return nil
	}
	today := now.Format(model.DateFormat)
	shares, err := f.store.FriendLinkShares(ctx, l.ID)
	if err != nil {
		return err
	}
	// With farm pets off here there's no animal to feed, so a gift isn't
	// taken. It lands if pets are turned back on while it's still being sent.
	petsOn, _ := f.store.GetSetting(ctx, "pets_enabled")
	gifts, err := f.store.ListFriendReceived(ctx, model.PetActionGift, "", today)
	if err != nil {
		return err
	}
	got := map[int64]int{}
	for _, g := range gifts {
		if g.LinkID == l.ID {
			got[g.ToUser]++
		}
	}
	dates, err := f.store.ListFriendReceived(ctx, model.PetActionPlaydate, "", "")
	if err != nil {
		return err
	}
	visited := map[string]bool{} // from member + day
	for _, d := range dates {
		if d.LinkID == l.ID {
			visited[d.FromMember+"|"+d.Day] = true
		}
	}
	for _, sd := range s.Sends {
		if !accepts[sd.From] {
			continue
		}
		// Only something sent in the last couple of days counts. What has
		// arrived is remembered for a week, so a stale snapshot (the other
		// family's server has gone quiet) or a replayed id can never be
		// applied a second time.
		if !recent(sd.Day, now) {
			continue
		}
		if sd.Kind == model.PetActionPlaydate {
			if visited[sd.From+"|"+sd.Day] {
				continue // one visit per animal per day
			}
			visited[sd.From+"|"+sd.Day] = true
		}
		var to int64
		if sd.Kind == model.PetActionGift {
			to, _ = strconv.ParseInt(sd.To, 10, 64)
			if petsOn != "true" || !shares[to] || got[to] >= maxGiftsPerDay {
				continue
			}
			if p, err := f.store.GetPetForUser(ctx, to); err != nil || p == nil {
				continue
			}
		}
		first, err := f.store.ReceiveFriendSend(ctx, l.ID, sd, to, today)
		if err != nil {
			return err
		}
		if first && sd.Kind == model.PetActionGift {
			got[to]++
			if err := f.store.AdjustPetNeeds(ctx, to, giftFullness, giftJoy); err != nil {
				return err
			}
		}
	}
	return f.store.PruneFriendSends(ctx, now.AddDate(0, 0, -7).Format(model.DateFormat))
}

// recent reports whether day (YYYY-MM-DD, someone else's calendar) is within
// two days either side of now: time zones can put a friend a day ahead or
// behind, and a send travels for up to two days.
func recent(day string, now time.Time) bool {
	d, err := time.ParseInLocation(model.DateFormat, day, now.Location())
	if err != nil {
		return false
	}
	gap := now.Sub(d)
	return gap > -48*time.Hour && gap < 72*time.Hour
}

// SendTarget is a checked gift or playdate, ready to record.
type SendTarget struct {
	LinkID int64
	Send   model.FriendSend
	// Name is who it's for, for the message shown to the kid: the animal
	// for a gift, the family for a playdate.
	Name string
}

// CheckSend says whether a person may send a gift (to the animal of the
// friend with key friendKey, "f<link>:<member>") or a playdate (to the family
// of link linkID). msg is a refusal to show; "" means go ahead with target.
// Nothing is recorded and no points move.
func (f *Service) CheckSend(ctx context.Context, userID int64, kind, friendKey string, linkID int64) (target *SendTarget, msg string, err error) {
	now := f.now()
	today := now.Format(model.DateFormat)
	memberID := ""
	if kind == model.PetActionGift {
		var ok bool
		rest, _ := strings.CutPrefix(friendKey, "f")
		link, member, ok := strings.Cut(rest, ":")
		if linkID, _ = strconv.ParseInt(link, 10, 64); !ok || linkID == 0 || member == "" {
			return nil, "choose a friend's animal", nil
		}
		memberID = member
	}
	l, err := f.store.GetFriendLink(ctx, linkID)
	if err != nil {
		return nil, "", err
	}
	snap := (*model.FriendSnapshot)(nil)
	if l != nil {
		snap = PeerSnapshot(l)
	}
	if l == nil || l.Status != model.FriendStatusActive || snap == nil {
		return nil, "choose a friend family", nil
	}
	name := l.PeerName
	if name == "" {
		name = "your friends"
	}
	send := model.FriendSend{Kind: kind, From: strconv.FormatInt(userID, 10), Day: today}
	// A gift or a visit only arrives between people both parents picked:
	// the other family has to be shown this person.
	theyShow := false
	for _, id := range snap.Accepted {
		theyShow = theyShow || id == send.From
	}
	shares, err := f.store.FriendLinkShares(ctx, l.ID)
	if err != nil {
		return nil, "", err
	}
	if !shares[userID] || !theyShow {
		return nil, name + " can't see you yet. A grown-up in each family has to tick you first.", nil
	}
	if snap.NoVisits {
		return nil, name + " aren't taking treats or playdates right now.", nil
	}
	switch kind {
	case model.PetActionGift:
		accepts, err := f.store.FriendLinkAccepts(ctx, l.ID)
		if err != nil {
			return nil, "", err
		}
		var pet *model.FriendPet
		for _, m := range Shown(snap, accepts) {
			if m.ID == memberID {
				pet = m.Pet
			}
		}
		if pet == nil {
			return nil, "that friend doesn't have an animal to give a treat to", nil
		}
		n, err := f.store.CountFriendSends(ctx, userID, kind, today, l.ID, memberID)
		if err != nil {
			return nil, "", err
		}
		if n > 0 {
			return nil, "you've already sent " + pet.Name + " a treat today", nil
		}
		send.To, name = memberID, pet.Name
	case model.PetActionPlaydate:
		n, err := f.store.CountFriendSends(ctx, userID, kind, today, 0, "")
		if err != nil {
			return nil, "", err
		}
		if n > 0 {
			return nil, "your animal has already gone on a playdate today", nil
		}
	default:
		return nil, "unknown kind", nil
	}
	return &SendTarget{LinkID: l.ID, Send: send, Name: name}, "", nil
}

// ReserveSend records a checked gift or playdate before any points move. It
// returns a refusal to show ("" = recorded) when today's limit for it has
// been used in the meantime: the table decides, so two taps at once can't
// both get through. If paying then fails, call CancelSend.
func (f *Service) ReserveSend(ctx context.Context, userID int64, t *SendTarget) (msg string, err error) {
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	t.Send.ID = base64.RawURLEncoding.EncodeToString(id)
	ok, err := f.store.AddFriendSend(ctx, t.LinkID, t.Send, userID)
	if err != nil {
		return "", err
	}
	if !ok {
		t.Send.ID = ""
		if t.Send.Kind == model.PetActionGift {
			return "you've already sent " + t.Name + " a treat today", nil
		}
		return "your animal has already gone on a playdate today", nil
	}
	return "", nil
}

// CancelSend takes back a reserved send (paying for it failed).
func (f *Service) CancelSend(ctx context.Context, t *SendTarget) {
	if t != nil && t.Send.ID != "" {
		_ = f.store.DeleteFriendSend(ctx, t.Send.ID)
	}
}

// ConfirmSend notes which redemption paid for a reserved send (so undoing
// the redemption can take it back) and starts a sync so it arrives soon.
func (f *Service) ConfirmSend(ctx context.Context, t *SendTarget, redemptionID int64) {
	_ = f.store.SetFriendSendRedemption(ctx, t.Send.ID, redemptionID)
	f.SyncSoon(ctx, t.LinkID)
}

// SyncSoon syncs a link in the background, without holding up the request
// that asked for it.
func (f *Service) SyncSoon(ctx context.Context, linkID int64) {
	go func() { _ = f.Sync(context.WithoutCancel(ctx), linkID) }()
}

// Visitors returns the friend families' animals that are here on a playdate
// today (their today), and the gifts that arrived today for this
// household's animals.
func (f *Service) Visitors(ctx context.Context) ([]model.Pet, []model.FriendGift, error) {
	visitors, gifts := []model.Pet{}, []model.FriendGift{}
	links, err := f.store.ListFriendLinks(ctx)
	if err != nil {
		return nil, nil, err
	}
	today := f.now().Format(model.DateFormat)
	for i := range links {
		l := &links[i]
		snap := PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil || !l.AllowVisits {
			continue
		}
		accepts, err := f.store.FriendLinkAccepts(ctx, l.ID)
		if err != nil {
			return nil, nil, err
		}
		byID := map[string]model.FriendMember{}
		for _, m := range Shown(snap, accepts) {
			byID[m.ID] = m
		}
		family := l.PeerName
		if family == "" {
			family = "Friend family"
		}
		// A playdate lasts for the sender's day. Their "today" comes from
		// their server, so it also has to be about now by our own clock: a
		// family whose server has gone quiet doesn't leave an animal here.
		if snap.Today != "" && recent(snap.Today, f.now()) && snap.GeneratedAt.After(f.now().Add(-36*time.Hour)) {
			dates, err := f.store.ListFriendReceived(ctx, model.PetActionPlaydate, snap.Today, "")
			if err != nil {
				return nil, nil, err
			}
			seen := map[string]bool{}
			for _, d := range dates {
				m, ok := byID[d.FromMember]
				if d.LinkID != l.ID || !ok || m.Pet == nil || seen[m.ID] {
					continue
				}
				seen[m.ID] = true
				p := m.Pet
				mood := p.Mood
				if p.Hiding {
					mood = "content" // it came out to play; it isn't hiding here
				}
				visitors = append(visitors, model.Pet{
					ID: -(l.ID*1000 + int64(len(visitors)) + 1), Name: p.Name, Species: p.Species, Color: p.Color, Accessory: p.Accessory,
					Mood: mood, Sick: p.Sick, Items: []string{}, OwnerName: m.Name, OwnerColor: m.Color,
					SpriteData: pictureURL(p.Picture), SpriteFrames: p.Frames, Visiting: true, Family: family,
				})
			}
		}
		got, err := f.store.ListFriendReceived(ctx, model.PetActionGift, "", today)
		if err != nil {
			return nil, nil, err
		}
		for _, g := range got {
			if m, ok := byID[g.FromMember]; ok && g.LinkID == l.ID {
				gifts = append(gifts, model.FriendGift{ToUserID: g.ToUser, From: m.Name, Family: family})
			}
		}
	}
	return visitors, gifts, nil
}

// pictureURL turns a friend's animal picture (checked and re-encoded when it
// arrived) into a data: URL a page can show.
func pictureURL(png []byte) string {
	if len(png) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// PictureURL is pictureURL for other packages.
func PictureURL(png []byte) string { return pictureURL(png) }

// HasFriendAnimals reports whether there is any linked family animal a kid
// could send a gift to (gift and playdate rewards are hidden otherwise).
func (f *Service) HasFriendAnimals(ctx context.Context) bool {
	links, err := f.store.ListFriendLinks(ctx)
	if err != nil {
		return false
	}
	for i := range links {
		l := &links[i]
		snap := PeerSnapshot(l)
		if l.Status != model.FriendStatusActive || snap == nil {
			continue
		}
		accepts, err := f.store.FriendLinkAccepts(ctx, l.ID)
		if err != nil {
			continue
		}
		for _, m := range Shown(snap, accepts) {
			if m.Pet != nil {
				return true
			}
		}
	}
	return false
}

// SetAllowVisits records whether gifts and playdates from a linked family
// are taken.
func (f *Service) SetAllowVisits(ctx context.Context, id int64, allow bool) error {
	return f.store.SetFriendAllowVisits(ctx, id, allow)
}

// keepPictures deals with the custom animal pictures in a peer's snapshot.
// Unless a parent here chose to show that family's pictures they are
// dropped. Otherwise each one is checked and re-encoded like an upload
// (it's a file from someone else's server); one that fails is dropped and
// the animal shows as its built-in kind.
func keepPictures(s *model.FriendSnapshot, show bool) {
	for i := range s.Members {
		p := s.Members[i].Pet
		if p == nil || len(p.Picture) == 0 {
			if p != nil {
				p.Picture, p.Frames = nil, 0
			}
			continue
		}
		p.Frames = 0
		if !show || len(p.Picture) > maxPictureBytes {
			p.Picture = nil
			continue
		}
		clean, frames, err := sprite.Decode(p.Picture)
		if err != nil {
			p.Picture = nil
			continue
		}
		p.Picture, p.Frames = clean, frames
	}
}

// SetShowPictures records whether a linked family's custom animal pictures
// are shown here. Turning it off deletes the stored ones at once; turning
// it on brings them in at the next sync.
func (f *Service) SetShowPictures(ctx context.Context, id int64, show bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.store.SetFriendShowPictures(ctx, id, show); err != nil || show {
		return err
	}
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil {
		return err
	}
	snap := PeerSnapshot(l)
	if snap == nil {
		return nil
	}
	keepPictures(snap, false)
	clean, _ := json.Marshal(snap)
	return f.store.SetFriendSnapshot(ctx, id, string(clean))
}

// SetAccepts replaces which of a linked family's people are shown here, and
// straight away cuts the stored copy of anyone no longer accepted down to
// their name. That doesn't wait for the next sync, which might fail: a
// person a parent has un-ticked isn't kept.
func (f *Service) SetAccepts(ctx context.Context, id int64, memberIDs []string) error {
	f.mu.Lock() // not while a sync is reading the accepts and writing the snapshot
	defer f.mu.Unlock()
	if err := f.store.SetFriendLinkAccepts(ctx, id, memberIDs); err != nil {
		return err
	}
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil {
		return err
	}
	snap := PeerSnapshot(l)
	if snap == nil {
		return nil
	}
	accepts := map[string]bool{}
	for _, m := range memberIDs {
		accepts[m] = true
	}
	strip(snap, accepts)
	clean, _ := json.Marshal(snap)
	return f.store.SetFriendSnapshot(ctx, id, string(clean))
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
	if !validDate(s.Today) {
		s.Today = ""
	}
	team := s.Team[:0]
	for _, tw := range s.Team {
		if validDate(tw.Start) {
			tw.People = max(0, min(tw.People, 30))
			tw.ShareSum = max(0, min(tw.ShareSum, tw.People*score.ShareScale))
			team = append(team, tw)
		}
	}
	if s.Team = team; len(s.Team) > 2 {
		s.Team = s.Team[len(s.Team)-2:]
	}
	if g := s.Goal; g != nil && (!validDate(g.Start) || g.Target < 1 || g.Target > 100) {
		s.Goal = nil
	}
	sends := s.Sends[:0]
	for _, sd := range s.Sends {
		sd.ID, sd.From, sd.To = clip(sd.ID, 40), clip(sd.From, 20), clip(sd.To, 20)
		if sd.ID != "" && sd.From != "" && validDate(sd.Day) && (sd.Kind == model.PetActionGift || sd.Kind == model.PetActionPlaydate) && len(sends) < 50 {
			sends = append(sends, sd)
		}
	}
	s.Sends = sends
	// A member needs an id of its own: accepting is by id, so an empty or
	// repeated one would let a second person ride in on the first.
	seen := map[string]bool{}
	members := s.Members[:0]
	for _, m := range s.Members {
		m.ID = clip(m.ID, 20)
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		members = append(members, m)
	}
	s.Members = members
	for i := range s.Members {
		m := &s.Members[i]
		m.Name = clip(m.Name, maxNameLen)
		// The colour ends up in CSS (var(--person-<colour>)), so only one
		// of our own colour names is kept: anything else could carry a
		// url() that makes every screen here fetch the other family's server.
		if !model.ValidPersonColor(m.Color) {
			m.Color = ""
		}
		m.Streak = max(0, min(m.Streak, 9999))
		weeks := m.Weeks[:0]
		for _, w := range m.Weeks {
			if validDate(w.Start) {
				weeks = append(weeks, w)
			}
		}
		m.Weeks = weeks
		if len(m.Weeks) > 2 {
			m.Weeks = m.Weeks[len(m.Weeks)-2:]
		}
		for j := range m.Weeks {
			w := &m.Weeks[j]
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

func validDate(d string) bool {
	_, err := time.Parse(model.DateFormat, d)
	return err == nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// SyncAll syncs every live link, logging failures, and retries any unlink
// the hub hasn't confirmed.
func (f *Service) SyncAll(ctx context.Context) {
	f.RetryUnlinks(ctx)
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
// can't be reached the link is still removed here at once, and the hub is
// told later (RetryUnlinks): until then it would go on handing this
// family's last snapshot to the other family.
func (f *Service) Unlink(ctx context.Context, id int64) error {
	l, err := f.store.GetFriendLink(ctx, id)
	if err != nil || l == nil {
		return err
	}
	if l.Status != model.FriendStatusEnded {
		if err := f.hubDelete(ctx, l); err != nil {
			log.Printf("friends: couldn't tell the hub about unlinking %q, will retry: %v", l.PeerName, err)
			return f.store.DeleteFriendLinkLater(ctx, id)
		}
	}
	return f.store.DeleteFriendLink(ctx, id)
}

func (f *Service) hubDelete(ctx context.Context, l *model.FriendLink) error {
	return f.call(ctx, f.trustsLink(ctx, l), http.MethodDelete, l.HubURL, "/v1/links/"+url.PathEscape(l.HubLink), l.Token, nil, nil)
}

// unlinkRetryFor is how long an unconfirmed unlink is retried: the hub drops
// a link nobody syncs after 90 days anyway.
const unlinkRetryFor = 90 * 24 * time.Hour

// RetryUnlinks tells the hub about unlinks it missed (see Unlink). The hub
// answers a delete of a link that's already gone with success too.
func (f *Service) RetryUnlinks(ctx context.Context) {
	pending, err := f.store.ListFriendUnlinks(ctx, f.now().Add(-unlinkRetryFor))
	if err != nil {
		log.Printf("friends: listing unconfirmed unlinks: %v", err)
		return
	}
	for id, l := range pending {
		if err := f.hubDelete(ctx, &l); err != nil {
			continue // still unreachable; next sync tries again
		}
		if err := f.store.DeleteFriendUnlink(ctx, id); err != nil {
			log.Printf("friends: forgetting a confirmed unlink: %v", err)
		}
	}
}
