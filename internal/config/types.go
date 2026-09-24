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
	Auth          *AuthConfig          `yaml:"auth,omitempty"`
}

// AuthConfig holds runtime authentication settings. Unlike the seed sections
// above it is read on every start, so providers can be added to an existing
// install. String values support ${ENV_VAR} expansion so secrets can stay out
// of the file.
type AuthConfig struct {
	// PublicURL is the externally reachable base URL (e.g.
	// https://chores.example.com), used to build OIDC redirect URIs. When
	// empty, the base_url setting or the request's Host is used.
	PublicURL string `yaml:"public_url,omitempty"`
	// KioskSessionTTL bounds tap/PIN sessions (default 12h).
	KioskSessionTTL string `yaml:"kiosk_session_ttl,omitempty"`
	// PersonalSessionTTL bounds OIDC sessions on personal devices (default 720h).
	PersonalSessionTTL string               `yaml:"personal_session_ttl,omitempty"`
	OIDC               []OIDCProviderConfig `yaml:"oidc,omitempty"`
}

// OIDCProviderConfig describes one OpenID Connect provider (Pocket ID,
// Authelia, Authentik, Keycloak, Zitadel, Google, Microsoft, ...).
type OIDCProviderConfig struct {
	// ID is the stable identifier stored with linked identities and used in
	// the callback URL: /api/auth/oidc/<id>/callback. Do not change it once
	// accounts are linked.
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes,omitempty"`
	// Prompt is passed through as the OIDC prompt parameter. "login" forces
	// the provider to re-authenticate every time, which is recommended when
	// parents sign in on a shared wall tablet.
	Prompt string `yaml:"prompt,omitempty"`
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
	Name string `yaml:"name"`
	Role string `yaml:"role"`
	// Pin is a 4-8 digit profile PIN, hashed on seed. Admin profiles need
	// a PIN (or a linked account) to sign in.
	Pin    string `yaml:"pin,omitempty"`
	Age    int    `yaml:"age,omitempty"`
	// Theme is the person's skin: sunroom, blocks or tint. The legacy
	// names default, quest, galaxy and forest are accepted and mapped.
	Theme string `yaml:"theme,omitempty"`
	// Color is a person colour key (coral, mint, butter, sky, rose, leaf,
	// lilac, sand). Empty assigns the next free one.
	Color  string `yaml:"color,omitempty"`
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
