package config

// Config represents the top-level YAML configuration file.
type Config struct {
	Users        []UserConfig         `yaml:"users,omitempty"`
	Chores       []ChoreConfig        `yaml:"chores,omitempty"`
	Rewards      []RewardConfig       `yaml:"rewards,omitempty"`
	StreakRewards []StreakRewardConfig `yaml:"streak_rewards,omitempty"`
	Settings     map[string]string    `yaml:"settings,omitempty"`
}

type UserConfig struct {
	Name   string `yaml:"name"`
	Role   string `yaml:"role"`
	Age    int    `yaml:"age,omitempty"`
	Theme  string `yaml:"theme,omitempty"`
	Avatar string `yaml:"avatar,omitempty"`
}

type ChoreConfig struct {
	Title            string           `yaml:"title"`
	Icon             string           `yaml:"icon,omitempty"`
	Category         string           `yaml:"category"`
	Points           int              `yaml:"points"`
	MissedPenalty    int              `yaml:"missed_penalty,omitempty"`
	RequiresApproval bool             `yaml:"requires_approval,omitempty"`
	RequiresPhoto    bool             `yaml:"requires_photo,omitempty"`
	PhotoSource      string           `yaml:"photo_source,omitempty"`
	Schedules        []ScheduleConfig `yaml:"schedules,omitempty"`
}

type ScheduleConfig struct {
	AssignTo         string   `yaml:"assign_to"`
	Days             []string `yaml:"days,omitempty"`
	AvailableAt      string   `yaml:"available_at,omitempty"`
	DueBy            string   `yaml:"due_by,omitempty"`
	Expiry           string   `yaml:"expiry,omitempty"`
	ExpiryPoints     int      `yaml:"expiry_points,omitempty"`
	PointsMultiplier float64  `yaml:"points_multiplier,omitempty"`
}

type RewardConfig struct {
	Name  string `yaml:"name"`
	Icon  string `yaml:"icon,omitempty"`
	Cost  int    `yaml:"cost"`
	Stock int    `yaml:"stock,omitempty"`
}

type StreakRewardConfig struct {
	Days        int    `yaml:"days"`
	BonusPoints int    `yaml:"bonus_points"`
	Label       string `yaml:"label"`
}
