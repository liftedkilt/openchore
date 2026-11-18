package store

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// --- Leaderboard days ---

// ListScoreDays returns every person's stored days from..to, keyed by user.
func (s *Store) ListScoreDays(ctx context.Context, from, to string) (map[int64][]model.ScoreDay, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, date, assigned, done, bonus, assigned_points, done_points, paused FROM score_days
		 WHERE date >= ? AND date <= ? ORDER BY user_id, date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]model.ScoreDay{}
	for rows.Next() {
		var uid int64
		var d model.ScoreDay
		if err := rows.Scan(&uid, &d.Date, &d.Assigned, &d.Done, &d.Bonus, &d.AssignedPoints, &d.DonePoints, &d.Paused); err != nil {
			return nil, err
		}
		out[uid] = append(out[uid], d)
	}
	return out, rows.Err()
}

// SaveScoreDay stores (or replaces) one person's day.
func (s *Store) SaveScoreDay(ctx context.Context, userID int64, d model.ScoreDay) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO score_days (user_id, date, assigned, done, bonus, assigned_points, done_points, paused)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, date) DO UPDATE SET assigned = excluded.assigned, done = excluded.done,
		   bonus = excluded.bonus, assigned_points = excluded.assigned_points, done_points = excluded.done_points,
		   paused = excluded.paused`,
		userID, d.Date, d.Assigned, d.Done, d.Bonus, d.AssignedPoints, d.DonePoints, d.Paused)
	return err
}

// --- Sharing, per linked family ---

// FriendLinkShares returns the people a parent chose to share with one
// linked family.
func (s *Store) FriendLinkShares(ctx context.Context, linkID int64) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM friend_link_shares WHERE link_id = ?`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// FriendLinkAccepts returns the other family's people (by snapshot member
// id) a parent chose to show from one linked family.
func (s *Store) FriendLinkAccepts(ctx context.Context, linkID int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT member_id FROM friend_link_accepts WHERE link_id = ?`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SetFriendLinkShares replaces who is shared with one linked family.
func (s *Store) SetFriendLinkShares(ctx context.Context, linkID int64, userIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM friend_link_shares WHERE link_id = ?`, linkID); err != nil {
		return err
	}
	for _, id := range userIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO friend_link_shares (link_id, user_id) VALUES (?, ?)`, linkID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetFriendLinkAccepts replaces which of the other family's people are shown.
func (s *Store) SetFriendLinkAccepts(ctx context.Context, linkID int64, memberIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM friend_link_accepts WHERE link_id = ?`, linkID); err != nil {
		return err
	}
	for _, id := range memberIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO friend_link_accepts (link_id, member_id) VALUES (?, ?)`, linkID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Friend links ---

const friendLinkColumns = `id, status, side, show_pictures, allow_visits, goal_week, goal_target, hub_url, hub_link, token, secret, invite, invite_expires,
	peer_name, snapshot, snapshot_at, last_sync_at, last_error, created_at`

func scanFriendLink(row interface{ Scan(...any) error }) (*model.FriendLink, error) {
	l := &model.FriendLink{}
	var expires, snapAt, syncAt sql.NullTime
	if err := row.Scan(&l.ID, &l.Status, &l.Side, &l.ShowPictures, &l.AllowVisits, &l.GoalWeek, &l.GoalTarget, &l.HubURL, &l.HubLink, &l.Token, &l.Secret, &l.Invite, &expires,
		&l.PeerName, &l.Snapshot, &snapAt, &syncAt, &l.LastError, &l.CreatedAt); err != nil {
		return nil, err
	}
	for _, f := range []struct {
		src sql.NullTime
		dst **time.Time
	}{{expires, &l.InviteExpires}, {snapAt, &l.SnapshotAt}, {syncAt, &l.LastSyncAt}} {
		if f.src.Valid {
			t := f.src.Time
			*f.dst = &t
		}
	}
	return l, nil
}

func (s *Store) ListFriendLinks(ctx context.Context) ([]model.FriendLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+friendLinkColumns+` FROM friend_links ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FriendLink
	for rows.Next() {
		l, err := scanFriendLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

func (s *Store) GetFriendLink(ctx context.Context, id int64) (*model.FriendLink, error) {
	l, err := scanFriendLink(s.db.QueryRowContext(ctx, `SELECT `+friendLinkColumns+` FROM friend_links WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

func (s *Store) CreateFriendLink(ctx context.Context, l *model.FriendLink) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO friend_links (status, side, hub_url, hub_link, token, secret, invite, invite_expires)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Status, l.Side, l.HubURL, l.HubLink, l.Token, l.Secret, l.Invite, l.InviteExpires)
	if err != nil {
		return err
	}
	l.ID, _ = res.LastInsertId()
	return nil
}

// SaveFriendSync records the outcome of one sync with the hub. A non-empty
// snapshot replaces the stored one; status "ended" also wipes the other
// family's data.
func (s *Store) SaveFriendSync(ctx context.Context, id int64, status, peerName, snapshot string, snapshotAt *time.Time, syncErr string, now time.Time) error {
	if status == model.FriendStatusEnded {
		_, err := s.db.ExecContext(ctx,
			`UPDATE friend_links SET status = ?, snapshot = '', snapshot_at = NULL, invite = '', last_error = ? WHERE id = ?`,
			status, syncErr, id)
		return err
	}
	if syncErr != "" {
		_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET last_error = ? WHERE id = ?`, syncErr, id)
		return err
	}
	if status == model.FriendStatusActive {
		if _, err := s.db.ExecContext(ctx, `UPDATE friend_links SET status = ?, invite = '' WHERE id = ?`, status, id); err != nil {
			return err
		}
	}
	if snapshot != "" {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE friend_links SET peer_name = ?, snapshot = ?, snapshot_at = ? WHERE id = ?`,
			peerName, snapshot, snapshotAt, id); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET last_sync_at = ?, last_error = '' WHERE id = ?`, now, id)
	return err
}

// SetFriendShowPictures records whether this family's custom animal
// pictures are shown here.
func (s *Store) SetFriendShowPictures(ctx context.Context, id int64, show bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET show_pictures = ? WHERE id = ?`, show, id)
	return err
}

// SetFriendAllowVisits records whether gifts and playdates from this family
// are taken.
func (s *Store) SetFriendAllowVisits(ctx context.Context, id int64, allow bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET allow_visits = ? WHERE id = ?`, allow, id)
	return err
}

// --- Gifts and playdates ---

// AddFriendSend records a gift or playdate a person here is sending. It
// reports false, without recording, when today's limit for it is already
// used (the table's unique indexes decide, so two requests at once can't
// both get through).
func (s *Store) AddFriendSend(ctx context.Context, linkID int64, send model.FriendSend, fromUser int64) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO friend_sends (id, link_id, kind, from_user, to_member, day) VALUES (?, ?, ?, ?, ?, ?)`,
		send.ID, linkID, send.Kind, fromUser, send.To, send.Day)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetFriendSendRedemption notes which redemption paid for a send.
func (s *Store) SetFriendSendRedemption(ctx context.Context, sendID string, redemptionID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE friend_sends SET redemption_id = ? WHERE id = ?`, redemptionID, sendID)
	return err
}

// DeleteFriendSend takes back a send by its id, or (id "") the one a
// redemption paid for.
func (s *Store) DeleteFriendSend(ctx context.Context, sendID string, redemptionID int64) error {
	if sendID != "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM friend_sends WHERE id = ?`, sendID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM friend_sends WHERE redemption_id = ? AND redemption_id != 0`, redemptionID)
	return err
}

// CountFriendSends counts what a person sent on a day of one kind: to one
// animal if toMember is set, and to one linked family unless linkID is 0.
func (s *Store) CountFriendSends(ctx context.Context, fromUser int64, kind, day string, linkID int64, toMember string) (int, error) {
	q := `SELECT COUNT(*) FROM friend_sends WHERE from_user = ? AND kind = ? AND day = ?`
	args := []any{fromUser, kind, day}
	if linkID != 0 {
		q += ` AND link_id = ?`
		args = append(args, linkID)
	}
	if toMember != "" {
		q += ` AND to_member = ?`
		args = append(args, toMember)
	}
	var n int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// FriendSendsSince returns what was sent to one linked family on or after day.
func (s *Store) FriendSendsSince(ctx context.Context, linkID int64, day string) ([]model.FriendSend, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind, from_user, to_member, day FROM friend_sends WHERE link_id = ? AND day >= ? ORDER BY created_at, id`, linkID, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FriendSend
	for rows.Next() {
		var f model.FriendSend
		var from int64
		if err := rows.Scan(&f.ID, &f.Kind, &from, &f.To, &f.Day); err != nil {
			return nil, err
		}
		f.From = strconv.FormatInt(from, 10)
		out = append(out, f)
	}
	return out, rows.Err()
}

// ReceiveFriendSend records a gift or playdate that arrived, once. It
// reports whether this is the first time it has been seen.
func (s *Store) ReceiveFriendSend(ctx context.Context, linkID int64, send model.FriendSend, toUser int64, today string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO friend_received (link_id, send_id, kind, from_member, to_user, day, received_on) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		linkID, send.ID, send.Kind, send.From, toUser, send.Day, today)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FriendReceived is one gift or playdate that arrived.
type FriendReceived struct {
	LinkID     int64
	Kind       string
	FromMember string
	ToUser     int64
	Day        string
}

// ListFriendReceived returns what arrived of one kind: sent on the sender's
// day (if day is set) and/or received here on receivedOn (if set).
func (s *Store) ListFriendReceived(ctx context.Context, kind, day, receivedOn string) ([]FriendReceived, error) {
	q, args := `SELECT link_id, kind, from_member, to_user, day FROM friend_received WHERE kind = ?`, []any{kind}
	if day != "" {
		q += ` AND day = ?`
		args = append(args, day)
	}
	if receivedOn != "" {
		q += ` AND received_on = ?`
		args = append(args, receivedOn)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY rowid`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FriendReceived
	for rows.Next() {
		var r FriendReceived
		if err := rows.Scan(&r.LinkID, &r.Kind, &r.FromMember, &r.ToUser, &r.Day); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneFriendSends forgets gifts and playdates older than day, sent and received.
func (s *Store) PruneFriendSends(ctx context.Context, day string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM friend_sends WHERE day < ?`, day); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM friend_received WHERE received_on < ?`, day)
	return err
}

// SetFriendGoal records this household's team goal with a linked family
// (week "" and target 0 clear it).
func (s *Store) SetFriendGoal(ctx context.Context, id int64, week string, target int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET goal_week = ?, goal_target = ? WHERE id = ?`, week, target, id)
	return err
}

// SetFriendSnapshot replaces the stored copy of the other family's snapshot.
func (s *Store) SetFriendSnapshot(ctx context.Context, id int64, snapshot string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE friend_links SET snapshot = ? WHERE id = ?`, snapshot, id)
	return err
}

// SchedulesCreatedSince returns the ids of a person's schedules that were
// created at or after cutoff.
func (s *Store) SchedulesCreatedSince(ctx context.Context, userID int64, cutoff time.Time) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM chore_schedules WHERE assigned_to = ? AND datetime(created_at) >= datetime(?)`,
		userID, cutoff.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// DeleteFriendLink removes a link and the other family's cached data with it.
func (s *Store) DeleteFriendLink(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM friend_links WHERE id = ?`, id)
	return err
}
