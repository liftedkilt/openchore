package config

// Config represents the top-level YAML configuration file.
type Config struct {
	Users         []UserConfig         `yaml:"users,omitempty"`
	Chores        []ChoreConfig        `yaml:"chores,omitempty"`
	Rewards       []RewardConfig       `yaml:"rewards,omitempty"`
	StreakRewards []StreakRewardConfig `yaml:"streak_rewards,omitempty"`
	Settings      map[string]string    `yaml:"settings,omitempty"`
	AI            *AIConfig            `yaml:"ai,omitempty"`
	Webhooks      *WebhooksConfig      `yaml:"webhooks,omitempty"`
}

// WebhooksConfig holds runtime tunables for the webhook subsystem.
// Currently only governs retention/cleanup of webhook_deliveries rows.
type WebhooksConfig struct {
	// DeliveryRetentionDays is the number of days of webhook delivery history to keep.
	// Rows with created_at older than now - retention are purged. Default: 30.
	// A value <= 0 disables cleanup (retain forever).
	DeliveryRetentionDays int `yaml:"delivery_retention_days,omitempty"`

	// DeliveryCleanupIntervalHours is how often the cleanup goroutine runs.
	// Default: 24 (once per day). Must be > 0 to schedule cleanup.
	DeliveryCleanupIntervalHours int `yaml:"delivery_cleanup_interval_hours,omitempty"`
}

// Retention defaults for webhook_deliveries cleanup.
const (
	DefaultWebhookDeliveryRetentionDays        = 30
	DefaultWebhookDeliveryCleanupIntervalHours = 24
)

// WebhookRetention returns the effective retention duration in days for
// webhook_deliveries rows, falling back to the default when unset.
func (c *Config) WebhookRetentionDays() int {
	if c == nil || c.Webhooks == nil || c.Webhooks.DeliveryRetentionDays == 0 {
		return DefaultWebhookDeliveryRetentionDays
	}
	return c.Webhooks.DeliveryRetentionDays
}

// WebhookCleanupIntervalHours returns the effective interval between cleanup
// runs in hours, falling back to the default when unset.
func (c *Config) WebhookCleanupIntervalHours() int {
	if c == nil || c.Webhooks == nil || c.Webhooks.DeliveryCleanupIntervalHours <= 0 {
		return DefaultWebhookDeliveryCleanupIntervalHours
	}
	return c.Webhooks.DeliveryCleanupIntervalHours
}

// AIConfig holds settings for AI-powered features (LiteRT or Ollama + Kokoro TTS).
type AIConfig struct {
	Enabled              bool    `yaml:"enabled"`
	Endpoint             string  `yaml:"endpoint"`
	Model                string  `yaml:"model"`
	AutoApproveThreshold float64 `yaml:"auto_approve_threshold"`
	TTSEnabled           bool    `yaml:"tts_enabled"`
	TTSEndpoint          string  `yaml:"tts_endpoint"`
	TTSVoice             string  `yaml:"tts_voice"`
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
