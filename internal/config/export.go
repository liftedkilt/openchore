package config

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/liftedkilt/openchore/internal/store"
)

// Export reads the current database state and builds a Config for the requested sections.
// Valid sections: "users", "chores", "rewards", "streak_rewards", "settings".
func Export(ctx context.Context, s *store.Store, sections []string) (*Config, error) {
	wanted := make(map[string]bool, len(sections))
	for _, sec := range sections {
		wanted[sec] = true
	}

	cfg := &Config{}

	// Build ID→name map for user lookups in schedules
	idToName := make(map[int64]string)

	if wanted["users"] {
		users, err := s.ListUsers(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing users: %w", err)
		}
		for _, u := range users {
			uc := UserConfig{
				Name:   u.Name,
				Role:   u.Role,
				Theme:  u.Theme,
				Avatar: u.AvatarURL,
			}
			if u.Age != nil {
				uc.Age = *u.Age
			}
			cfg.Users = append(cfg.Users, uc)
			idToName[u.ID] = u.Name
		}
	}

	if wanted["chores"] {
		// Ensure we have the name map even if users weren't exported
		if len(idToName) == 0 {
			users, err := s.ListUsers(ctx)
			if err != nil {
				return nil, fmt.Errorf("listing users for name map: %w", err)
			}
			for _, u := range users {
				idToName[u.ID] = u.Name
			}
		}

		chores, err := s.ListChores(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing chores: %w", err)
		}
		for _, c := range chores {
			cc := ChoreConfig{
				Title:            c.Title,
				Icon:             c.Icon,
				Category:         c.Category,
				Points:           c.PointsValue,
				MissedPenalty:    c.MissedPenaltyValue,
				RequiresApproval: c.RequiresApproval,
				RequiresPhoto:    c.RequiresPhoto,
			}

			schedules, err := s.ListSchedulesForChore(ctx, c.ID)
			if err != nil {
				return nil, fmt.Errorf("listing schedules for chore %q: %w", c.Title, err)
			}

			// Group schedules by (user, time settings) to collapse days
			type schedKey struct {
				AssignedTo         int64
				AvailableAt        string
				DueBy              string
				ExpiryPenalty      string
				ExpiryPenaltyValue int
				PointsMultiplier   float64
			}
			grouped := make(map[schedKey][]int)
			keyOrder := []schedKey{}

			for _, sc := range schedules {
				key := schedKey{
					AssignedTo:         sc.AssignedTo,
					ExpiryPenalty:      sc.ExpiryPenalty,
					ExpiryPenaltyValue: sc.ExpiryPenaltyValue,
					PointsMultiplier:   sc.PointsMultiplier,
				}
				if sc.AvailableAt != nil {
					key.AvailableAt = *sc.AvailableAt
				}
				if sc.DueBy != nil {
					key.DueBy = *sc.DueBy
				}
				if _, exists := grouped[key]; !exists {
					keyOrder = append(keyOrder, key)
				}
				if sc.DayOfWeek != nil {
					grouped[key] = append(grouped[key], *sc.DayOfWeek)
				}
			}

			for _, key := range keyOrder {
				sc := ScheduleConfig{
					AssignTo:    idToName[key.AssignedTo],
					AvailableAt: key.AvailableAt,
					DueBy:       key.DueBy,
					Expiry:      key.ExpiryPenalty,
					ExpiryPoints: key.ExpiryPenaltyValue,
				}
				if key.PointsMultiplier != 1.0 && key.PointsMultiplier != 0 {
					sc.PointsMultiplier = key.PointsMultiplier
				}
				if days, ok := grouped[key]; ok && len(days) > 0 {
					sc.Days = CollapseDays(days)
				}
				cc.Schedules = append(cc.Schedules, sc)
			}

			cfg.Chores = append(cfg.Chores, cc)
		}
	}

	if wanted["rewards"] {
		rewards, err := s.ListRewards(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("listing rewards: %w", err)
		}
		for _, r := range rewards {
			rc := RewardConfig{
				Name: r.Name,
				Icon: r.Icon,
				Cost: r.Cost,
			}
			if r.Stock != nil {
				rc.Stock = *r.Stock
			}
			cfg.Rewards = append(cfg.Rewards, rc)
		}
	}

	if wanted["streak_rewards"] {
		streakRewards, err := s.ListStreakRewards(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing streak rewards: %w", err)
		}
		for _, sr := range streakRewards {
			cfg.StreakRewards = append(cfg.StreakRewards, StreakRewardConfig{
				Days:        sr.StreakDays,
				BonusPoints: sr.BonusPoints,
				Label:       sr.Label,
			})
		}
	}

	if wanted["settings"] {
		settings, err := s.ListSettings(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing settings: %w", err)
		}
		// Exclude admin_passcode from export for security
		delete(settings, "admin_passcode")
		if len(settings) > 0 {
			cfg.Settings = settings
		}
	}

	return cfg, nil
}

// Marshal serializes a Config to YAML.
func Marshal(cfg *Config) ([]byte, error) {
	return yaml.Marshal(cfg)
}
