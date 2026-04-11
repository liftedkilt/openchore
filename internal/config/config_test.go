package config

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/migrations"
)

func setupStore(t *testing.T) *store.Store {
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
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrations: %v", err)
	}
	return store.New(db)
}

// --- Day helpers ---

func TestExpandDays(t *testing.T) {
	tests := []struct {
		input    []string
		expected []int
	}{
		{[]string{"daily"}, []int{0, 1, 2, 3, 4, 5, 6}},
		{[]string{"weekdays"}, []int{1, 2, 3, 4, 5}},
		{[]string{"weekends"}, []int{0, 6}},
		{[]string{"mon", "wed", "fri"}, []int{1, 3, 5}},
		{[]string{"sun"}, []int{0}},
		{[]string{}, nil},
	}
	for _, tt := range tests {
		got := ExpandDays(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ExpandDays(%v) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("ExpandDays(%v)[%d] = %d, want %d", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestCollapseDays(t *testing.T) {
	tests := []struct {
		input    []int
		expected []string
	}{
		{[]int{0, 1, 2, 3, 4, 5, 6}, []string{"daily"}},
		{[]int{1, 2, 3, 4, 5}, []string{"weekdays"}},
		{[]int{0, 6}, []string{"weekends"}},
		{[]int{1, 3, 5}, []string{"mon", "wed", "fri"}},
		{[]int{0}, []string{"sun"}},
	}
	for _, tt := range tests {
		got := CollapseDays(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("CollapseDays(%v) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("CollapseDays(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

// --- Load ---

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file")
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
users:
  - name: TestAdmin
    role: admin
chores:
  - title: Test Chore
    category: required
    points: 5
    schedules:
      - assign_to: TestAdmin
        days: [weekdays]
rewards:
  - name: Test Reward
    cost: 10
streak_rewards:
  - days: 3
    bonus_points: 5
    label: Three days
settings:
  admin_passcode: "9999"
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(cfg.Users))
	}
	if cfg.Users[0].Name != "TestAdmin" {
		t.Errorf("expected user name TestAdmin, got %q", cfg.Users[0].Name)
	}
	if len(cfg.Chores) != 1 {
		t.Errorf("expected 1 chore, got %d", len(cfg.Chores))
	}
	if len(cfg.Chores[0].Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(cfg.Chores[0].Schedules))
	}
	if len(cfg.Rewards) != 1 {
		t.Errorf("expected 1 reward, got %d", len(cfg.Rewards))
	}
	if len(cfg.StreakRewards) != 1 {
		t.Errorf("expected 1 streak reward, got %d", len(cfg.StreakRewards))
	}
	if cfg.Settings["admin_passcode"] != "9999" {
		t.Errorf("expected passcode 9999, got %q", cfg.Settings["admin_passcode"])
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(":::invalid"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// --- Apply ---

func TestApplyCreatesData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	cfg := &Config{
		Users: []UserConfig{
			{Name: "Admin", Role: "admin", Theme: "default"},
			{Name: "Kid", Role: "child", Age: 8, Theme: "galaxy"},
		},
		Chores: []ChoreConfig{
			{
				Title:    "Test Chore",
				Category: "required",
				Points:   5,
				Schedules: []ScheduleConfig{
					{AssignTo: "Kid", Days: []string{"weekdays"}, Expiry: "block"},
				},
			},
		},
		Rewards: []RewardConfig{
			{Name: "Prize", Cost: 25, Stock: -1},
		},
		StreakRewards: []StreakRewardConfig{
			{Days: 7, BonusPoints: 10, Label: "Week!"},
		},
		Settings: map[string]string{
			"admin_passcode": "1234",
		},
	}

	if err := Apply(ctx, s, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify users
	users, _ := s.ListUsers(ctx)
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}

	// Verify chores
	chores, _ := s.ListChores(ctx)
	if len(chores) != 1 {
		t.Errorf("expected 1 chore, got %d", len(chores))
	}

	// Verify schedules (5 weekdays)
	schedules, _ := s.ListSchedulesForChore(ctx, chores[0].ID)
	if len(schedules) != 5 {
		t.Errorf("expected 5 schedules (weekdays), got %d", len(schedules))
	}

	// Verify rewards
	rewards, _ := s.ListRewards(ctx, false)
	if len(rewards) != 1 {
		t.Errorf("expected 1 reward, got %d", len(rewards))
	}

	// Verify streak rewards
	streakRewards, _ := s.ListStreakRewards(ctx)
	if len(streakRewards) != 1 {
		t.Errorf("expected 1 streak reward, got %d", len(streakRewards))
	}

	// Verify passcode was hashed (not stored as plaintext)
	passcode, _ := s.GetSetting(ctx, "admin_passcode")
	if passcode == "1234" {
		t.Error("passcode should be hashed, not stored as plaintext")
	}
	if len(passcode) < 20 {
		t.Error("passcode hash seems too short")
	}
}

func TestApplySkipsPopulatedDB(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	// First apply
	cfg := &Config{
		Users: []UserConfig{{Name: "Admin", Role: "admin"}},
	}
	if err := Apply(ctx, s, cfg); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	// Second apply should skip
	cfg2 := &Config{
		Users: []UserConfig{{Name: "Admin2", Role: "admin"}},
	}
	if err := Apply(ctx, s, cfg2); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	// Should still have only 1 user
	users, _ := s.ListUsers(ctx)
	if len(users) != 1 {
		t.Errorf("expected 1 user (skipped), got %d", len(users))
	}
	if users[0].Name != "Admin" {
		t.Errorf("expected user Admin, got %q", users[0].Name)
	}
}

func TestApplyUnknownUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	cfg := &Config{
		Users: []UserConfig{{Name: "Admin", Role: "admin"}},
		Chores: []ChoreConfig{
			{
				Title:    "Test",
				Category: "required",
				Points:   1,
				Schedules: []ScheduleConfig{
					{AssignTo: "NonExistent", Days: []string{"mon"}},
				},
			},
		},
	}
	err := Apply(ctx, s, cfg)
	if err == nil {
		t.Fatal("expected error for unknown user in schedule")
	}
}

// --- Export ---

func TestExportRoundTrip(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	// Apply a config
	original := &Config{
		Users: []UserConfig{
			{Name: "Admin", Role: "admin"},
			{Name: "Kid", Role: "child", Age: 10},
		},
		Chores: []ChoreConfig{
			{
				Title:    "Daily Chore",
				Category: "required",
				Points:   5,
				Icon:     "🧹",
				Schedules: []ScheduleConfig{
					{AssignTo: "Kid", Days: []string{"daily"}, Expiry: "block"},
				},
			},
		},
		Rewards: []RewardConfig{
			{Name: "Prize", Cost: 50, Icon: "🎁"},
		},
		StreakRewards: []StreakRewardConfig{
			{Days: 7, BonusPoints: 10, Label: "Week!"},
		},
	}
	if err := Apply(ctx, s, original); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Export all sections
	exported, err := Export(ctx, s, []string{"users", "chores", "rewards", "streak_rewards"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if len(exported.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(exported.Users))
	}
	if len(exported.Chores) != 1 {
		t.Errorf("expected 1 chore, got %d", len(exported.Chores))
	}
	if exported.Chores[0].Title != "Daily Chore" {
		t.Errorf("expected chore title 'Daily Chore', got %q", exported.Chores[0].Title)
	}

	// Should have collapsed 7 individual schedules to "daily"
	if len(exported.Chores[0].Schedules) != 1 {
		t.Errorf("expected 1 collapsed schedule, got %d", len(exported.Chores[0].Schedules))
	}
	if len(exported.Chores[0].Schedules) > 0 {
		days := exported.Chores[0].Schedules[0].Days
		if len(days) != 1 || days[0] != "daily" {
			t.Errorf("expected days=[daily], got %v", days)
		}
	}

	if len(exported.Rewards) != 1 {
		t.Errorf("expected 1 reward, got %d", len(exported.Rewards))
	}
	if len(exported.StreakRewards) != 1 {
		t.Errorf("expected 1 streak reward, got %d", len(exported.StreakRewards))
	}
}

func TestExportSelectiveSections(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	cfg := &Config{
		Users:   []UserConfig{{Name: "Admin", Role: "admin"}},
		Rewards: []RewardConfig{{Name: "Prize", Cost: 50}},
	}
	if err := Apply(ctx, s, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Export only rewards
	exported, err := Export(ctx, s, []string{"rewards"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if len(exported.Users) != 0 {
		t.Errorf("expected 0 users (not requested), got %d", len(exported.Users))
	}
	if len(exported.Rewards) != 1 {
		t.Errorf("expected 1 reward, got %d", len(exported.Rewards))
	}
}

func TestExportExcludesPasscode(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	cfg := &Config{
		Users:    []UserConfig{{Name: "Admin", Role: "admin"}},
		Settings: map[string]string{"admin_passcode": "1234", "base_url": "https://example.com"},
	}
	if err := Apply(ctx, s, cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	exported, err := Export(ctx, s, []string{"settings"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, ok := exported.Settings["admin_passcode"]; ok {
		t.Error("exported settings should not include admin_passcode")
	}
	if exported.Settings["base_url"] != "https://example.com" {
		t.Errorf("expected base_url, got %v", exported.Settings)
	}
}

func TestMarshal(t *testing.T) {
	cfg := &Config{
		Users: []UserConfig{{Name: "Test", Role: "admin"}},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty YAML output")
	}
}

// ===== Webhook retention helpers (issue #18) =====

// TestWebhookRetentionDays covers the fallback ladder: nil Config, nil
// Webhooks section, zero value, and explicit override. This guards against
// a regression where unset config causes cleanup to misbehave (e.g. purge
// everything because retention read as 0, or panic on nil).
func TestWebhookRetentionDays(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want int
	}{
		{"nil config", nil, DefaultWebhookDeliveryRetentionDays},
		{"nil webhooks section", &Config{}, DefaultWebhookDeliveryRetentionDays},
		{"zero value falls through to default", &Config{Webhooks: &WebhooksConfig{}}, DefaultWebhookDeliveryRetentionDays},
		{"explicit override", &Config{Webhooks: &WebhooksConfig{DeliveryRetentionDays: 7}}, 7},
		{"negative value passes through (disables cleanup)", &Config{Webhooks: &WebhooksConfig{DeliveryRetentionDays: -1}}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.WebhookRetentionDays()
			if got != tc.want {
				t.Errorf("WebhookRetentionDays() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestWebhookCleanupIntervalHours mirrors WebhookRetentionDays, but note
// the interval helper clamps non-positive values to the default because
// a zero or negative tick interval would break the cleanup goroutine's
// ticker.
func TestWebhookCleanupIntervalHours(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want int
	}{
		{"nil config", nil, DefaultWebhookDeliveryCleanupIntervalHours},
		{"nil webhooks section", &Config{}, DefaultWebhookDeliveryCleanupIntervalHours},
		{"zero value falls through", &Config{Webhooks: &WebhooksConfig{}}, DefaultWebhookDeliveryCleanupIntervalHours},
		{"negative value clamped to default", &Config{Webhooks: &WebhooksConfig{DeliveryCleanupIntervalHours: -5}}, DefaultWebhookDeliveryCleanupIntervalHours},
		{"explicit override", &Config{Webhooks: &WebhooksConfig{DeliveryCleanupIntervalHours: 6}}, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.WebhookCleanupIntervalHours()
			if got != tc.want {
				t.Errorf("WebhookCleanupIntervalHours() = %d, want %d", got, tc.want)
			}
		})
	}
}
