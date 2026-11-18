package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// --- Leaderboard days ---

// ListScoreDays returns every person's stored days from..to, keyed by user.
func (s *Store) ListScoreDays(ctx context.Context, from, to string) (map[int64][]model.ScoreDay, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, date, assigned, done, bonus, paused FROM score_days
		 WHERE date >= ? AND date <= ? ORDER BY user_id, date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]model.ScoreDay{}
	for rows.Next() {
		var uid int64
		var d model.ScoreDay
		if err := rows.Scan(&uid, &d.Date, &d.Assigned, &d.Done, &d.Bonus, &d.Paused); err != nil {
			return nil, err
		}
		out[uid] = append(out[uid], d)
	}
	return out, rows.Err()
}

// SaveScoreDay stores (or replaces) one person's day.
func (s *Store) SaveScoreDay(ctx context.Context, userID int64, d model.ScoreDay) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO score_days (user_id, date, assigned, done, bonus, paused) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, date) DO UPDATE SET assigned = excluded.assigned, done = excluded.done,
		   bonus = excluded.bonus, paused = excluded.paused`,
		userID, d.Date, d.Assigned, d.Done, d.Bonus, d.Paused)
	return err
}

// --- Sharing ---

// SharedUserIDs returns the people a parent has chosen to share with friend
// families.
func (s *Store) SharedUserIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM users WHERE share_with_friends = 1`)
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

// SetUserShared turns sharing with friend families on or off for a person.
// It reports whether the person exists.
func (s *Store) SetUserShared(ctx context.Context, userID int64, shared bool) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET share_with_friends = ? WHERE id = ?`, shared, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// --- Friend links ---

const friendLinkColumns = `id, status, hub_url, hub_link, token, secret, invite, invite_expires,
	peer_name, snapshot, snapshot_at, last_sync_at, last_error, created_at`

func scanFriendLink(row interface{ Scan(...any) error }) (*model.FriendLink, error) {
	l := &model.FriendLink{}
	var expires, snapAt, syncAt sql.NullTime
	if err := row.Scan(&l.ID, &l.Status, &l.HubURL, &l.HubLink, &l.Token, &l.Secret, &l.Invite, &expires,
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
		`INSERT INTO friend_links (status, hub_url, hub_link, token, secret, invite, invite_expires)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.Status, l.HubURL, l.HubLink, l.Token, l.Secret, l.Invite, l.InviteExpires)
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

// DeleteFriendLink removes a link and the other family's cached data with it.
func (s *Store) DeleteFriendLink(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM friend_links WHERE id = ?`, id)
	return err
}
