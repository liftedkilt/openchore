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
