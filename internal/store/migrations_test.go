package store_test

import (
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/migrations"
)

// migrator opens an empty in-memory database with the embedded migrations.
func migrator(t *testing.T) (*migrate.Migrate, *sql.DB) {
	t.Helper()
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
	return m, db
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// Every migration applies on a fresh database, the whole chain rolls back,
// and applies again.
func TestMigrationsUpDownUp(t *testing.T) {
	m, db := migrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if v, _, _ := m.Version(); v < 18 {
		t.Fatalf("expected at least version 18, got %d", v)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up again: %v", err)
	}
	mustExec(t, db, `SELECT user_id, week_start, summary FROM weekly_summaries`)
	mustExec(t, db, `SELECT color, ai_complete FROM users, chore_completions`)
}

// 018 on a database with data from before it (017 applied, as on the
// release after the redesign): AI rejections go, the review columns and
// settings move, and the spoken-description column is dropped. Down puts
// the shape back.
func TestMigration018SimplifyAI(t *testing.T) {
	m, db := migrator(t)
	if err := m.Migrate(17); err != nil {
		t.Fatalf("migrate to 17: %v", err)
	}

	mustExec(t, db, `INSERT INTO users (id, name, role, theme, color) VALUES (1, 'Mia', 'child', 'sunroom', 'mint')`)
	mustExec(t, db, `INSERT INTO chores (id, title, created_by, tts_description, tts_audio_url) VALUES (1, 'Make bed', 1, 'Make your bed, Mia!', '/tts/1.wav')`)
	mustExec(t, db, `INSERT INTO chore_schedules (id, chore_id, assigned_to, day_of_week) VALUES (1, 1, 1, 1)`)
	mustExec(t, db, `INSERT INTO chore_completions (id, chore_schedule_id, completed_by, status, completion_date, ai_feedback, ai_confidence)
		VALUES (1, 1, 1, 'approved', '2026-09-21', 'Looks great', 0.9),
		       (2, 1, 1, 'ai_rejected', '2026-09-22', 'Still messy', 0.8),
		       (3, 1, 1, 'pending', '2026-09-23', '', 0)`)
	mustExec(t, db, `INSERT INTO app_settings (key, value) VALUES
		('ai_enabled', 'true'), ('ai_endpoint', 'http://litert:8000'), ('ai_model', 'gemma'),
		('ai_tts_voice', 'af_bella'), ('ai_tts_enabled', 'true'), ('ai_auto_approve_threshold', '0.9')`)

	if err := m.Migrate(18); err != nil {
		t.Fatalf("migrate to 18: %v", err)
	}

	var n int
	db.QueryRow(`SELECT COUNT(*) FROM chore_completions`).Scan(&n)
	if n != 2 {
		t.Fatalf("expected the ai_rejected completion removed (2 left), got %d", n)
	}
	var feedback string
	var aiComplete sql.NullBool
	if err := db.QueryRow(`SELECT ai_feedback, ai_complete FROM chore_completions WHERE id = 1`).Scan(&feedback, &aiComplete); err != nil {
		t.Fatalf("query completion: %v", err)
	}
	if feedback != "Looks great" || aiComplete.Valid {
		t.Fatalf("expected feedback kept and ai_complete NULL, got %q %v", feedback, aiComplete)
	}
	if _, err := db.Exec(`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, completion_date) VALUES (1, 1, 'ai_rejected', '2026-09-24')`); err == nil {
		t.Fatal("expected the ai_rejected status to be refused after 018")
	}
	var audio string
	db.QueryRow(`SELECT tts_audio_url FROM chores WHERE id = 1`).Scan(&audio)
	if audio != "" {
		t.Fatalf("expected audio cleared for re-recording, got %q", audio)
	}
	if _, err := db.Exec(`SELECT tts_description FROM chores`); err == nil {
		t.Fatal("expected chores.tts_description dropped")
	}
	settings := map[string]string{}
	rows, err := db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		t.Fatalf("query settings: %v", err)
	}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		settings[k] = v
	}
	rows.Close()
	want := map[string]string{"ai_photo_review": "true", "tts_voice": "af_bella", "ai_auto_approve_threshold": "0.9"}
	for k, v := range want {
		if settings[k] != v {
			t.Errorf("setting %s: got %q, want %q", k, settings[k], v)
		}
	}
	for _, gone := range []string{"ai_enabled", "ai_endpoint", "ai_model", "ai_tts_voice", "ai_tts_enabled"} {
		if _, ok := settings[gone]; ok {
			t.Errorf("expected setting %s removed", gone)
		}
	}
	mustExec(t, db, `INSERT INTO weekly_summaries (user_id, week_start, summary) VALUES (1, '2026-09-14', 'A good week')`)
	// 017's data is untouched.
	var color string
	db.QueryRow(`SELECT color FROM users WHERE id = 1`).Scan(&color)
	if color != "mint" {
		t.Fatalf("expected 017's colour kept, got %q", color)
	}

	// Down to 17 and back up.
	if err := m.Migrate(17); err != nil {
		t.Fatalf("migrate down to 17: %v", err)
	}
	mustExec(t, db, `SELECT tts_description FROM chores`)
	mustExec(t, db, `INSERT INTO chore_completions (chore_schedule_id, completed_by, status, completion_date) VALUES (1, 1, 'ai_rejected', '2026-09-24')`)
	db.QueryRow(`SELECT value FROM app_settings WHERE key = 'ai_enabled'`).Scan(&feedback)
	if feedback != "true" {
		t.Fatalf("expected ai_enabled restored, got %q", feedback)
	}
	if _, err := db.Exec(`SELECT 1 FROM weekly_summaries`); err == nil {
		t.Fatal("expected weekly_summaries dropped")
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// Older versions deleted schedules and users without cascading, so a
// database can hold completions pointing at rows that no longer exist. 018
// rebuilds chore_completions with foreign keys enforced; the orphans must
// not make it fail.
func TestMigration018OrphanedCompletions(t *testing.T) {
	m, db := migrator(t)
	if err := m.Migrate(17); err != nil {
		t.Fatalf("migrate to 17: %v", err)
	}

	mustExec(t, db, `INSERT INTO users (id, name, role, theme, color) VALUES (1, 'Mia', 'child', 'sunroom', 'mint')`)
	mustExec(t, db, `INSERT INTO chores (id, title, created_by) VALUES (1, 'Make bed', 1)`)
	mustExec(t, db, `INSERT INTO chore_schedules (id, chore_id, assigned_to, day_of_week) VALUES (1, 1, 1, 1)`)
	mustExec(t, db, `PRAGMA foreign_keys = OFF`)
	mustExec(t, db, `INSERT INTO chore_completions (id, chore_schedule_id, completed_by, status, approved_by, completion_date)
		VALUES (1, 1, 1, 'approved', 1, '2026-09-21'),
		       (2, 99, 1, 'approved', NULL, '2026-09-21'),
		       (3, 1, 99, 'approved', NULL, '2026-09-21'),
		       (4, 1, 1, 'approved', 99, '2026-09-22')`)
	mustExec(t, db, `PRAGMA foreign_keys = ON`)

	if err := m.Migrate(18); err != nil {
		t.Fatalf("migrate to 18: %v", err)
	}

	rows, err := db.Query(`SELECT id, approved_by FROM chore_completions ORDER BY id`)
	if err != nil {
		t.Fatalf("query completions: %v", err)
	}
	got := map[int64]sql.NullInt64{}
	for rows.Next() {
		var id int64
		var approvedBy sql.NullInt64
		if err := rows.Scan(&id, &approvedBy); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = approvedBy
	}
	rows.Close()
	if len(got) != 2 {
		t.Fatalf("expected completions 1 and 4 kept, got %v", got)
	}
	if a, ok := got[1]; !ok || !a.Valid || a.Int64 != 1 {
		t.Fatalf("expected completion 1 kept with its approver, got %v", got)
	}
	if a, ok := got[4]; !ok || a.Valid {
		t.Fatalf("expected completion 4 kept with approved_by cleared, got %v", got)
	}
}

// 022 widens the chores category CHECK in place. Rebuilding the table would
// cascade-delete schedules and completions, so they must all survive, and
// down turns optional chores into bonus ones.
func TestMigration022OptionalCategoryKeepsHistory(t *testing.T) {
	m, db := migrator(t)
	if err := m.Migrate(21); err != nil {
		t.Fatalf("migrate to 21: %v", err)
	}
	mustExec(t, db, `INSERT INTO users (id, name, role, theme, color) VALUES (1, 'Mia', 'child', 'sunroom', 'mint')`)
	mustExec(t, db, `INSERT INTO chores (id, title, created_by) VALUES (1, 'Make bed', 1)`)
	mustExec(t, db, `INSERT INTO chore_schedules (id, chore_id, assigned_to, day_of_week) VALUES (1, 1, 1, 1)`)
	mustExec(t, db, `INSERT INTO chore_completions (id, chore_schedule_id, completed_by, completion_date) VALUES (1, 1, 1, '2026-09-21')`)
	if _, err := db.Exec(`INSERT INTO chores (title, category, created_by) VALUES ('x', 'optional', 1)`); err == nil {
		t.Fatal("optional accepted before 022")
	}

	if err := m.Migrate(22); err != nil {
		t.Fatalf("migrate to 22: %v", err)
	}
	var n int
	db.QueryRow(`SELECT (SELECT count(*) FROM chore_schedules) + (SELECT count(*) FROM chore_completions)`).Scan(&n)
	if n != 2 {
		t.Fatalf("schedules/completions lost: %d rows left", n)
	}
	mustExec(t, db, `INSERT INTO chores (id, title, category, created_by) VALUES (2, 'Piano', 'optional', 1)`)
	if _, err := db.Exec(`INSERT INTO chores (title, category, created_by) VALUES ('x', 'nope', 1)`); err == nil {
		t.Fatal("CHECK no longer enforced")
	}
	var ok string
	db.QueryRow(`PRAGMA integrity_check`).Scan(&ok)
	if ok != "ok" {
		t.Fatalf("integrity_check: %s", ok)
	}

	if err := m.Migrate(21); err != nil {
		t.Fatalf("down to 21: %v", err)
	}
	var cat string
	db.QueryRow(`SELECT category FROM chores WHERE id = 2`).Scan(&cat)
	if cat != "bonus" {
		t.Fatalf("down: want bonus, got %s", cat)
	}
	if _, err := db.Exec(`INSERT INTO chores (title, category, created_by) VALUES ('x', 'optional', 1)`); err == nil {
		t.Fatal("optional still accepted after down")
	}
	db.QueryRow(`SELECT count(*) FROM chore_completions`).Scan(&n)
	if n != 1 {
		t.Fatalf("down lost completions: %d", n)
	}
}

// 030 widens the friend_links status CHECK in place to allow pending_confirm.
// Rebuilding friend_links with DROP TABLE would cascade-delete all 4 child
// tables (friend_link_shares, friend_sends, friend_received,
// friend_link_accepts), so every child row and existing link must survive both
// up and down.
func TestMigration030FriendConfirmKeepsChildRows(t *testing.T) {
	m, db := migrator(t)
	if err := m.Migrate(29); err != nil {
		t.Fatalf("migrate to 29: %v", err)
	}
	mustExec(t, db, `INSERT INTO users (id, name, role, theme, color) VALUES (1, 'Ava', 'child', 'sunroom', 'mint')`)
	mustExec(t, db, `INSERT INTO friend_links (id, status, side, hub_url, hub_link, token, secret, peer_name, snapshot)
		VALUES (1, 'active', 'a', 'https://hub.example', 'L1', 't1', 's1', 'Smiths', '{"version":1}')`)
	mustExec(t, db, `INSERT INTO friend_links (id, status, side, hub_url, hub_link, token, secret, invite)
		VALUES (2, 'waiting', 'a', 'https://hub.example', 'L2', 't2', 's2', 'code123')`)
	mustExec(t, db, `INSERT INTO friend_links (id, status, side, hub_url, hub_link, token, secret)
		VALUES (3, 'ended', 'b', 'https://hub.example', 'L3', 't3', 's3')`)
	mustExec(t, db, `INSERT INTO friend_link_shares (link_id, user_id) VALUES (1, 1)`)
	mustExec(t, db, `INSERT INTO friend_sends (id, link_id, kind, from_user, to_member, day, redemption_id)
		VALUES ('s1', 1, 'gift', 1, 'm1', '2026-11-24', 1)`)
	mustExec(t, db, `INSERT INTO friend_received (link_id, send_id, kind, from_member, to_user, day, received_on)
		VALUES (1, 'r1', 'playdate', 'm1', 0, '2026-11-24', '2026-11-24')`)
	mustExec(t, db, `INSERT INTO friend_link_accepts (link_id, member_id) VALUES (1, 'm1')`)

	if _, err := db.Exec(`INSERT INTO friend_links (id, status, side, hub_url, hub_link, token, secret)
		VALUES (4, 'pending_confirm', 'a', 'https://hub.example', 'L4', 't4', 's4')`); err == nil {
		t.Fatal("pending_confirm accepted before 030")
	}

	assertChildren := func(stage string) {
		t.Helper()
		for _, tbl := range []string{"friend_link_shares", "friend_sends", "friend_received", "friend_link_accepts"} {
			var n int
			if err := db.QueryRow(`SELECT count(*) FROM ` + tbl).Scan(&n); err != nil || n != 1 {
				t.Fatalf("%s: %s count = %d (err=%v), want 1", stage, tbl, n, err)
			}
		}
		var ok string
		if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
			t.Fatalf("%s: integrity_check = %q (err=%v)", stage, ok, err)
		}
		rows, err := db.Query(`PRAGMA foreign_key_check`)
		if err != nil {
			t.Fatalf("%s: foreign_key_check: %v", stage, err)
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatalf("%s: foreign_key_check returned violations", stage)
		}
	}

	if err := m.Migrate(30); err != nil {
		t.Fatalf("migrate to 30: %v", err)
	}
	var st1, peer1, snap1, st2, st3 string
	if err := db.QueryRow(`SELECT status, peer_name, snapshot FROM friend_links WHERE id = 1`).Scan(&st1, &peer1, &snap1); err != nil {
		t.Fatal(err)
	}
	if st1 != "active" || peer1 != "Smiths" || snap1 != `{"version":1}` {
		t.Fatalf("existing active link altered: %q %q %q", st1, peer1, snap1)
	}
	db.QueryRow(`SELECT status FROM friend_links WHERE id = 2`).Scan(&st2)
	db.QueryRow(`SELECT status FROM friend_links WHERE id = 3`).Scan(&st3)
	if st2 != "waiting" || st3 != "ended" {
		t.Fatalf("existing waiting/ended links altered: %q %q", st2, st3)
	}
	mustExec(t, db, `INSERT INTO friend_links (id, status, side, hub_url, hub_link, token, secret, peer_name, snapshot)
		VALUES (4, 'pending_confirm', 'a', 'https://hub.example', 'L4', 't4', 's4', 'Garcias', '{"version":1}')`)
	if _, err := db.Exec(`INSERT INTO friend_links (status, side, hub_url, hub_link, token, secret)
		VALUES ('bogus', 'a', 'https://hub.example', 'L5', 't5', 's5')`); err == nil {
		t.Fatal("CHECK no longer enforced after 030")
	}
	assertChildren("up 030")

	if err := m.Migrate(29); err != nil {
		t.Fatalf("down to 29: %v", err)
	}
	var st4, snap4 string
	if err := db.QueryRow(`SELECT status, snapshot FROM friend_links WHERE id = 4`).Scan(&st4, &snap4); err != nil {
		t.Fatal(err)
	}
	if st4 != "waiting" || snap4 != "" {
		t.Fatalf("down: want status=waiting snapshot='', got %q %q", st4, snap4)
	}
	db.QueryRow(`SELECT status FROM friend_links WHERE id = 1`).Scan(&st1)
	if st1 != "active" {
		t.Fatalf("down altered active link: %q", st1)
	}
	if _, err := db.Exec(`INSERT INTO friend_links (status, side, hub_url, hub_link, token, secret)
		VALUES ('pending_confirm', 'a', 'https://hub.example', 'L6', 't6', 's6')`); err == nil {
		t.Fatal("pending_confirm still accepted after down")
	}
	assertChildren("down 029")

	if err := m.Up(); err != nil {
		t.Fatalf("re-upgrade to latest: %v", err)
	}
	assertChildren("re-up")
}

