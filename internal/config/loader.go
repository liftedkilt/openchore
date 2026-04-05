package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// Load reads and parses a YAML config file. Returns nil, nil if the file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	return &cfg, nil
}

// Apply writes the config to the database. It is idempotent: if any users already
// exist in the database, the entire operation is skipped.
func Apply(ctx context.Context, s *store.Store, cfg *Config) error {
	users, err := s.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("checking existing users: %w", err)
	}
	if len(users) > 0 {
		log.Println("config: database already populated, skipping")
		return nil
	}

	log.Println("config: applying configuration to empty database")

	// 1. Create users, build name→ID map
	nameToID := make(map[string]int64, len(cfg.Users))
	for _, u := range cfg.Users {
		user := &model.User{
			Name:      u.Name,
			Role:      u.Role,
			Theme:     u.Theme,
			AvatarURL: u.Avatar,
		}
		if u.Age > 0 {
			age := u.Age
			user.Age = &age
		}
		if err := s.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("creating user %q: %w", u.Name, err)
		}
		nameToID[u.Name] = user.ID
		log.Printf("config: created user %q (id=%d)", u.Name, user.ID)
	}

	// 2. Apply settings
	for key, value := range cfg.Settings {
		v := value
		if key == "admin_passcode" {
			hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hashing admin passcode: %w", err)
			}
			v = string(hash)
		}
		if err := s.SetSetting(ctx, key, v); err != nil {
			return fmt.Errorf("setting %q: %w", key, err)
		}
	}

	// 3. Create chores and their schedules
	for _, c := range cfg.Chores {
		chore := &model.Chore{
			Title:            c.Title,
			Icon:             c.Icon,
			Category:         c.Category,
			PointsValue:      c.Points,
			MissedPenaltyValue: c.MissedPenalty,
			RequiresApproval: c.RequiresApproval,
			RequiresPhoto:    c.RequiresPhoto,
			PhotoSource:      c.PhotoSource,
		}
		// Find admin user for created_by
		for _, u := range cfg.Users {
			if u.Role == "admin" {
				chore.CreatedBy = nameToID[u.Name]
				break
			}
		}
		if err := s.CreateChore(ctx, chore); err != nil {
			return fmt.Errorf("creating chore %q: %w", c.Title, err)
		}
		log.Printf("config: created chore %q (id=%d)", c.Title, chore.ID)

		for _, sc := range c.Schedules {
			userID, ok := nameToID[sc.AssignTo]
			if !ok {
				return fmt.Errorf("chore %q: schedule references unknown user %q", c.Title, sc.AssignTo)
			}

			days := ExpandDays(sc.Days)
			if len(days) == 0 {
				// No days specified — create a single unscheduled entry
				days = []int{-1}
			}

			multiplier := sc.PointsMultiplier
			if multiplier == 0 {
				multiplier = 1.0
			}

			for _, dow := range days {
				schedule := &model.ChoreSchedule{
					ChoreID:            chore.ID,
					AssignedTo:         userID,
					AssignmentType:     "individual",
					PointsMultiplier:   multiplier,
					AvailableAt:        nilStr(sc.AvailableAt),
					DueBy:              nilStr(sc.DueBy),
					ExpiryPenalty:      sc.Expiry,
					ExpiryPenaltyValue: sc.ExpiryPoints,
				}
				if dow >= 0 {
					d := dow
					schedule.DayOfWeek = &d
				}
				if err := s.CreateSchedule(ctx, schedule); err != nil {
					return fmt.Errorf("creating schedule for chore %q, user %q, day %d: %w", c.Title, sc.AssignTo, dow, err)
				}
			}
		}
	}

	// 4. Create rewards
	for _, r := range cfg.Rewards {
		reward := &model.Reward{
			Name:   r.Name,
			Icon:   r.Icon,
			Cost:   r.Cost,
			Active: true,
		}
		if r.Stock != 0 {
			stock := r.Stock
			reward.Stock = &stock
		}
		// Find admin user for created_by
		for _, u := range cfg.Users {
			if u.Role == "admin" {
				reward.CreatedBy = nameToID[u.Name]
				break
			}
		}
		if err := s.CreateReward(ctx, reward); err != nil {
			return fmt.Errorf("creating reward %q: %w", r.Name, err)
		}
		log.Printf("config: created reward %q (id=%d)", r.Name, reward.ID)
	}

	// 5. Create streak rewards
	for _, sr := range cfg.StreakRewards {
		streakReward := &model.StreakReward{
			StreakDays:  sr.Days,
			BonusPoints: sr.BonusPoints,
			Label:       sr.Label,
		}
		if err := s.CreateStreakReward(ctx, streakReward); err != nil {
			return fmt.Errorf("creating streak reward %q: %w", sr.Label, err)
		}
		log.Printf("config: created streak reward %q", sr.Label)
	}

	// 6. Apply AI settings (if provided)
	if cfg.AI != nil {
		aiSettings := map[string]string{
			"ai_enabled": "false",
		}
		if cfg.AI.Enabled {
			aiSettings["ai_enabled"] = "true"
		}
		if cfg.AI.Endpoint != "" {
			aiSettings["ai_endpoint"] = cfg.AI.Endpoint
		}
		if cfg.AI.Model != "" {
			aiSettings["ai_model"] = cfg.AI.Model
		}
		if cfg.AI.AutoApproveThreshold > 0 {
			aiSettings["ai_auto_approve_threshold"] = fmt.Sprintf("%.2f", cfg.AI.AutoApproveThreshold)
		}
		if cfg.AI.TTSEnabled {
			aiSettings["ai_tts_enabled"] = "true"
		}
		if cfg.AI.TTSEndpoint != "" {
			aiSettings["ai_tts_endpoint"] = cfg.AI.TTSEndpoint
		}
		if cfg.AI.TTSVoice != "" {
			aiSettings["ai_tts_voice"] = cfg.AI.TTSVoice
		}
		for k, v := range aiSettings {
			if err := s.SetSetting(ctx, k, v); err != nil {
				return fmt.Errorf("setting AI config %q: %w", k, err)
			}
		}
		log.Println("config: AI settings applied")
	}

	log.Println("config: configuration applied successfully")
	return nil
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
