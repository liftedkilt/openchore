package model

import "time"

// DateFormat is the standard YYYY-MM-DD date layout used throughout the application.
const DateFormat = "2006-01-02"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	Age       *int      `json:"age,omitempty"`
	Theme     string    `json:"theme,omitempty"`
	Paused    bool      `json:"paused"`
	CreatedAt time.Time `json:"created_at"`
}

type Chore struct {
	ID                 int64     `json:"id"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	Category           string    `json:"category"`
	Icon               string    `json:"icon,omitempty"`
	PointsValue        int       `json:"points_value"`
	MissedPenaltyValue int       `json:"missed_penalty_value"`
	EstimatedMinutes   *int      `json:"estimated_minutes,omitempty"`
	RequiresApproval   bool      `json:"requires_approval"`
	RequiresPhoto      bool      `json:"requires_photo"`
	PhotoSource        string    `json:"photo_source"`
	Source             string    `json:"source"`
	ExternalID         string    `json:"external_id,omitempty"`
	CreatedBy          int64     `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}

type ChoreSchedule struct {
	ID               int64    `json:"id"`
	ChoreID          int64    `json:"chore_id"`
	AssignedTo       int64    `json:"assigned_to"`
	AssignmentType   string   `json:"assignment_type"`
	DayOfWeek        *int     `json:"day_of_week,omitempty"`
	SpecificDate     *string  `json:"specific_date,omitempty"`
	AvailableAt      *string  `json:"available_at,omitempty"`
	PointsMultiplier    float64  `json:"points_multiplier"`
	StartDate           *string  `json:"start_date,omitempty"`
	EndDate             *string  `json:"end_date,omitempty"`
	RecurrenceInterval  *int     `json:"recurrence_interval,omitempty"`
	RecurrenceStart     *string  `json:"recurrence_start,omitempty"`
	DueBy               *string  `json:"due_by,omitempty"`
	ExpiryPenalty       string   `json:"expiry_penalty"`
	ExpiryPenaltyValue  int      `json:"expiry_penalty_value"`
	CreatedAt           string   `json:"created_at"`
}

type ChoreCompletion struct {
	ID              int64      `json:"id"`
	ChoreScheduleID int64     `json:"chore_schedule_id"`
	CompletedBy     int64      `json:"completed_by"`
	Status          string     `json:"status"`
	PhotoURL        string     `json:"photo_url,omitempty"`
	ApprovedBy      *int64     `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	CompletedAt     time.Time  `json:"completed_at"`
	CompletionDate  string     `json:"completion_date"`
}

// --- Points & Rewards ---

type PointTransaction struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Amount      int       `json:"amount"`
	Reason      string    `json:"reason"`
	ReferenceID *int64    `json:"reference_id,omitempty"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Reward struct {
	ID            int64              `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Icon          string             `json:"icon,omitempty"`
	Cost          int                `json:"cost"`
	EffectiveCost int                `json:"effective_cost"` // per-user cost (may differ from base cost)
	Stock         *int               `json:"stock,omitempty"`
	Active        bool               `json:"active"`
	CreatedBy     int64              `json:"created_by"`
	CreatedAt     time.Time          `json:"created_at"`
	Assignments   []RewardAssignment `json:"assignments,omitempty"`
}

type RewardAssignment struct {
	ID         int64 `json:"id"`
	RewardID   int64 `json:"reward_id"`
	UserID     int64 `json:"user_id"`
	CustomCost *int  `json:"custom_cost,omitempty"`
}

type RewardRedemption struct {
	ID          int64     `json:"id"`
	RewardID    int64     `json:"reward_id"`
	UserID      int64     `json:"user_id"`
	PointsSpent int       `json:"points_spent"`
	CreatedAt   time.Time `json:"created_at"`
}

// --- Streaks ---

type UserStreak struct {
	UserID            int64   `json:"user_id"`
	CurrentStreak     int     `json:"current_streak"`
	LongestStreak     int     `json:"longest_streak"`
	StreakStartDate   *string `json:"streak_start_date,omitempty"`
	LastCompletedDate *string `json:"last_completed_date,omitempty"`
}

type StreakReward struct {
	ID          int64  `json:"id"`
	StreakDays  int    `json:"streak_days"`
	BonusPoints int    `json:"bonus_points"`
	Label       string `json:"label"`
}

// --- Decay ---

type UserDecayConfig struct {
	UserID             int64      `json:"user_id"`
	Enabled            bool       `json:"enabled"`
	DecayRate          int        `json:"decay_rate"`
	DecayIntervalHours int        `json:"decay_interval_hours"`
	LastDecayAt        *time.Time `json:"last_decay_at,omitempty"`
}

// --- Chore Triggers ---

type ChoreTrigger struct {
	ID                int64   `json:"id"`
	UUID              string  `json:"uuid"`
	ChoreID           int64   `json:"chore_id"`
	DefaultAssignedTo *int64  `json:"default_assigned_to,omitempty"`
	DefaultDueBy      *string `json:"default_due_by,omitempty"`
	DefaultAvailableAt *string `json:"default_available_at,omitempty"`
	Enabled           bool    `json:"enabled"`
	CooldownMinutes   int     `json:"cooldown_minutes"`
	LastTriggeredAt   *string `json:"last_triggered_at,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

// --- Webhooks ---

type Webhook struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    string    `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type WebhookDelivery struct {
	ID           int64     `json:"id"`
	WebhookID    int64     `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      string    `json:"payload"`
	StatusCode   *int      `json:"status_code,omitempty"`
	ResponseBody string    `json:"response_body,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScheduledChore is a denormalized view returned by the chores-for-user endpoint.
type ScheduledChore struct {
	ScheduleID       int64   `json:"schedule_id"`
	ChoreID          int64   `json:"chore_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Category         string  `json:"category"`
	Icon             string  `json:"icon,omitempty"`
	PointsValue      int     `json:"points_value"`
	MissedPenaltyValue int    `json:"missed_penalty_value"`
	EstimatedMinutes *int    `json:"estimated_minutes,omitempty"`
	RequiresApproval bool    `json:"requires_approval"`
	RequiresPhoto    bool    `json:"requires_photo"`
	PhotoSource      string  `json:"photo_source"`
	AssignmentType   string  `json:"assignment_type"`
	AvailableAt      *string `json:"available_at,omitempty"`
	DueBy              *string `json:"due_by,omitempty"`
	ExpiryPenalty      string  `json:"expiry_penalty"`
	ExpiryPenaltyValue int     `json:"expiry_penalty_value"`
	Available          bool    `json:"available"`
	Expired            bool    `json:"expired"`
	Completed        bool       `json:"completed"`
	CompletionID     *int64     `json:"completion_id,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	Date             string     `json:"date"`
}
