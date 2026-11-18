package model

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// DateFormat is the standard YYYY-MM-DD date layout used throughout the application.
const DateFormat = "2006-01-02"

// Chore categories
const (
	CategoryRequired = "required"
	CategoryCore     = "core"
	CategoryBonus    = "bonus"
	// CategoryOptional chores are never expected: they don't count toward
	// finishing the day (streaks, missed-chore penalties, decay, the bonus
	// gate) and their points, if any, are paid straight away. Kids can add
	// their own, always worth 0 points.
	CategoryOptional = "optional"
)

// ValidCategory reports whether c is a known chore category.
func ValidCategory(c string) bool {
	switch c {
	case CategoryRequired, CategoryCore, CategoryBonus, CategoryOptional:
		return true
	}
	return false
}

// CountsTowardDay reports whether a chore of this category has to be done
// for the day to count as complete (streaks, penalties, decay, bonus gate).
func CountsTowardDay(c string) bool {
	return c == CategoryRequired || c == CategoryCore
}

// ChoreSourceSelf marks a chore a person added for themselves (kid
// self-service). Such chores are always optional and worth 0 points.
const ChoreSourceSelf = "self"

// Completion statuses
const (
	StatusPending    = "pending"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusExcused    = "excused"
)

// Point transaction reasons
const (
	ReasonChoreComplete   = "chore_complete"
	ReasonChoreUncomplete = "chore_uncomplete"
	ReasonAdminAdjust     = "admin_adjust"
	ReasonRewardRedeem    = "reward_redeem"
	ReasonExpiryPenalty   = "expiry_penalty"
	ReasonPointsDecay     = "points_decay"
	ReasonMissedChore     = "missed_chore"
	ReasonCommitToGoal    = "commit_to_goal"
	ReasonGoalBreak       = "goal_break"
)

// Commitment statuses
const (
	CommitmentActive    = "active"
	CommitmentRedeemed  = "redeemed"
	CommitmentCancelled = "cancelled"
)

// Photo source modes
const (
	PhotoSourceChild    = "child"
	PhotoSourceExternal = "external"
	PhotoSourceBoth     = "both"
)

// Assignment types
const (
	AssignmentIndividual = "individual"
	AssignmentFamily     = "family"
	AssignmentFCFS       = "fcfs"
)

// Expiry penalty modes
const (
	ExpiryBlock    = "block"
	ExpiryNoPoints = "no_points"
	ExpiryPenalty  = "penalty"
)

// User is a household member. AuthProviders lists the IDs of the OIDC
// providers linked to the profile so the login screen can offer "Continue
// with ..." after the profile is tapped; subjects and emails are never
// exposed on the public listing.
type User struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	AvatarURL      string    `json:"avatar_url"`
	Role           string    `json:"role"`
	Age            *int      `json:"age,omitempty"`
	Theme          string    `json:"theme,omitempty"`
	LineColor      string    `json:"line_color,omitempty"`
	Color          string    `json:"color,omitempty"`
	Paused         bool      `json:"paused"`
	HasPin         bool      `json:"has_pin"`
	PinHash        string    `json:"-"`
	PinLength      int       `json:"pin_length,omitempty"` // digits in the PIN; 0 = no PIN or not known yet
	AuthProviders  []string  `json:"auth_providers"`
	SessionVersion int64     `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

// User roles. Admins manage the household *and* take part like everyone else
// (chores, points, rewards, streaks); the role only gates management actions.
const (
	RoleAdmin = "admin"
	RoleChild = "child"
)

// Skins, stored in users.theme. An empty theme means "not chosen"; the
// client resolves it by age.
const (
	ThemeSunroom = "sunroom"
	ThemeBlocks  = "blocks"
	ThemeTint    = "tint"
)

// ValidTheme reports whether t is one of the skins the API accepts.
func ValidTheme(t string) bool {
	return t == ThemeSunroom || t == ThemeBlocks || t == ThemeTint
}

// PersonColors are the keys stored in users.color, in assignment order.
// They are keys, not hex values: each theme defines its own shade.
var PersonColors = []string{"coral", "mint", "butter", "sky", "rose", "leaf", "lilac", "sand"}

// ValidPersonColor reports whether c is one of PersonColors.
func ValidPersonColor(c string) bool {
	for _, k := range PersonColors {
		if k == c {
			return true
		}
	}
	return false
}

// NextPersonColor picks a colour for a new person given the colours already
// in use: the first unused one in palette order, or once all are taken the
// least-used one (so assignment keeps cycling round-robin).
func NextPersonColor(used []string) string {
	counts := make(map[string]int, len(PersonColors))
	for _, c := range used {
		counts[c]++
	}
	best := PersonColors[0]
	for _, c := range PersonColors[1:] {
		if counts[c] < counts[best] {
			best = c
		}
	}
	return best
}

// UserIdentity is an external OIDC identity linked to a profile.
type UserIdentity struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Provider    string     `json:"provider"`
	Subject     string     `json:"-"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"display_name,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// OIDCProvider is a single sign-on provider added from the admin UI.
// Scopes is space-separated; empty means "openid profile email".
type OIDCProvider struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Issuer       string    `json:"issuer"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"`
	Scopes       string    `json:"scopes"`
	Prompt       string    `json:"prompt"`
	CreatedAt    time.Time `json:"created_at"`
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
	TTSAudioURL        string    `json:"tts_audio_url,omitempty"`
	CreatedBy          int64     `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}

type ChoreSchedule struct {
	ID               int64    `json:"id"`
	ChoreID          int64    `json:"chore_id"`
	AssignedTo       int64    `json:"assigned_to"`
	AssignmentType   string   `json:"assignment_type"`
	FcfsGroupID      *string  `json:"fcfs_group_id,omitempty"`
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

// OwnChore is a chore a person added for themselves (Source "self"), with
// its schedules, as returned by GET /api/me/chores.
type OwnChore struct {
	Chore
	Schedules []ChoreSchedule `json:"schedules"`
	// Editable is false once a grown-up has changed it into something the
	// owner may not manage (points added, or a different category).
	Editable bool `json:"editable"`
}

type ChoreCompletion struct {
	ID              int64      `json:"id"`
	ChoreScheduleID int64     `json:"chore_schedule_id"`
	CompletedBy     int64      `json:"completed_by"`
	Status          string     `json:"status"` // approved, pending, rejected, excused
	PhotoURL        string     `json:"photo_url,omitempty"`
	ApprovedBy      *int64     `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	CompletedAt     time.Time  `json:"completed_at"`
	CompletionDate  string     `json:"completion_date"`
	// AIFeedback is the AI photo reviewer's note for the approving parent
	// (or, on an excused completion, the excuse reason).
	AIFeedback      string     `json:"ai_feedback,omitempty"`
	AIConfidence    float64    `json:"ai_confidence,omitempty"`
	AIComplete      *bool      `json:"ai_complete,omitempty"`
	// UncompletedAt, when non-nil, marks a soft-deleted completion. The row
	// is preserved (photo + AI metadata + approval) so a kid can un-check and
	// re-check a chore without losing the approved state. Reader queries
	// exposing "is this done?" must treat non-nil UncompletedAt as not done.
	UncompletedAt *time.Time `json:"uncompleted_at,omitempty"`
}

// --- Points & Rewards ---

type PointTransaction struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Amount         int       `json:"amount"`
	Reason         string    `json:"reason"`
	ReferenceID    *int64    `json:"reference_id,omitempty"`
	Note           string    `json:"note,omitempty"`
	IdempotencyKey *string   `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
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
	Shareable     bool               `json:"shareable"`
	PetAction     string             `json:"pet_action,omitempty"` // "" or a pet-care action (feed, treat, play, accessory)
	PetItem       string             `json:"pet_item,omitempty"`   // the accessory an "accessory" reward gives
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

// RewardCommitment represents a kid earmarking points toward a chosen reward.
// AmountSaved is derived from point_transactions referencing this commitment
// and is populated by the store layer (not stored on the row itself).
// SharedPoolID, when non-nil, marks this row as a kid's share of a pooled
// family goal — Pool is populated for caller convenience.
type RewardCommitment struct {
	ID                    int64                 `json:"id"`
	UserID                int64                 `json:"user_id"`
	RewardID              int64                 `json:"reward_id"`
	RewardName            string                `json:"reward_name,omitempty"`
	RewardIcon            string                `json:"reward_icon,omitempty"`
	TargetCost            int                   `json:"target_cost"`
	AmountSaved           int                   `json:"amount_saved"`
	AutoContributePercent int                   `json:"auto_contribute_percent"`
	Status                string                `json:"status"`
	SharedPoolID          *int64                `json:"shared_pool_id,omitempty"`
	Pool                  *SharedCommitmentPool `json:"pool,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
	RedeemedAt            *time.Time            `json:"redeemed_at,omitempty"`
	CancelledAt           *time.Time            `json:"cancelled_at,omitempty"`
}

// SharedCommitmentPool is the pooled save target multiple kids are
// contributing to. AmountSaved is the sum across active contributors.
type SharedCommitmentPool struct {
	ID           int64             `json:"id"`
	RewardID     int64             `json:"reward_id"`
	RewardName   string            `json:"reward_name,omitempty"`
	RewardIcon   string            `json:"reward_icon,omitempty"`
	TargetCost   int               `json:"target_cost"`
	AmountSaved  int               `json:"amount_saved"`
	Status       string            `json:"status"`
	Contributors []PoolContributor `json:"contributors,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	RedeemedAt   *time.Time        `json:"redeemed_at,omitempty"`
}

// PoolContributor is one kid's slice of a shared pool — populated for the UI
// leaderboard ("You: 75, Alice: 120, Bob: 50").
type PoolContributor struct {
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	AmountSaved int    `json:"amount_saved"`
}

// --- Pets ---

// Pet actions a reward can carry.
const (
	PetActionFeed      = "feed"
	PetActionTreat     = "treat"
	PetActionPlay      = "play"
	PetActionAccessory = "accessory"
	PetActionNewAnimal = "new_animal" // swap species (and coat); health, food and joy carry over
	PetActionRestyle   = "restyle"    // change coat colour
	PetActionDesign    = "design"     // draw or upload your own accessory
	PetActionPicture   = "picture"    // use your own picture as the animal (docs/custom-animals.md)
)

// PetSpriteFrames is the order of the frames in a custom animal's picture
// strip. A strip may stop early: a missing frame falls back to the first.
var PetSpriteFrames = []string{"stand", "walk1", "walk2", "sick", "lying", "happy"}

// ValidPetAction reports whether a is a pet action or empty (ordinary reward).
func ValidPetAction(a string) bool {
	switch a {
	case "", PetActionFeed, PetActionTreat, PetActionPlay, PetActionAccessory, PetActionNewAnimal, PetActionRestyle, PetActionDesign, PetActionPicture:
		return true
	}
	return false
}

// PetItems are the built-in accessories a pet can wear (drawn by the web app).
var PetItems = []string{
	"straw_hat", "cowboy_hat", "flower_crown", "crown", "party_hat", "top_hat", "beanie", "wizard_hat", "pirate_hat", "flower", "bow",
	"sunglasses", "round_glasses", "heart_glasses",
	"bandana", "scarf", "bell", "bowtie", "medal",
}

// PetItemSlots are where a custom emoji accessory ("e:<slot>:<emoji>") sits.
var PetItemSlots = []string{"head", "face", "neck"}

// ValidPetItem reports whether it is a built-in accessory, a custom emoji
// accessory "e:<slot>:<emoji>" (1-8 characters, no spaces), or a drawn one
// "img:<slot>:<hex id>".
func ValidPetItem(it string) bool {
	for _, v := range PetItems {
		if v == it {
			return true
		}
	}
	if rest, ok := strings.CutPrefix(it, "img:"); ok {
		slot, id, ok := strings.Cut(rest, ":")
		return ok && slices.Contains(PetItemSlots, slot) && validHexID(id)
	}
	rest, ok := strings.CutPrefix(it, "e:")
	if !ok {
		return false
	}
	slot, emoji, ok := strings.Cut(rest, ":")
	if !ok || !slices.Contains(PetItemSlots, slot) {
		return false
	}
	n := utf8.RuneCountInString(emoji)
	return n >= 1 && n <= 8 && !strings.ContainsAny(emoji, " \t\n:")
}

func validHexID(id string) bool {
	if len(id) < 16 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// PetColors are each species' coat colours; the first is the default.
var PetColors = map[string][]string{
	"horse":   {"chestnut", "black", "grey", "palomino"},
	"dog":     {"golden", "chocolate", "black", "white"},
	"cat":     {"ginger", "grey", "black", "white"},
	"chicken": {"white", "brown", "black", "golden"},
	"cow":     {"holstein", "jersey", "black", "red"},
	"pig":     {"pink", "black", "ginger", "spotted"},
	"sheep":   {"white", "cream", "brown", "black"},
	"goat":    {"white", "tan", "brown", "black"},
}

// ValidPetColor reports whether c is one of species' coat colours.
func ValidPetColor(species, c string) bool {
	return slices.Contains(PetColors[species], c)
}

// PetSpecies are the farm animals a person can adopt. Every pet lives on
// one shared family farm, so everyone sees how everyone's animal is doing.
var PetSpecies = []string{"horse", "dog", "cat", "chicken", "cow", "pig", "sheep", "goat"}

// ValidPetSpecies reports whether sp is one of PetSpecies.
func ValidPetSpecies(sp string) bool {
	for _, v := range PetSpecies {
		if v == sp {
			return true
		}
	}
	return false
}

// Pet is a person's optional companion. Chores keep it healthy; points buy
// food and play (pet-care rewards). Health can't be bought.
type Pet struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`
	Species        string    `json:"species"`
	Health         int       `json:"health"`
	Fullness       int       `json:"fullness"`
	Joy            int       `json:"joy"`
	Color          string    `json:"color,omitempty"`     // coat colour (PetColors); "" = default
	Accessory      string    `json:"accessory,omitempty"` // worn item, from Items
	// Sprite, when set, is the id of the person's own picture, shown in
	// place of the drawn animal (uploads/pets/<id>.png). Species and Color
	// are kept underneath: they are what friend families see.
	Sprite string `json:"sprite,omitempty"`
	// SpriteFrames is how many frames the picture holds side by side
	// (PetSpriteFrames order); 1 for a single picture.
	SpriteFrames int `json:"sprite_frames,omitempty"`
	Items          []string  `json:"items"`               // owned accessories
	LastMissedDate *string   `json:"last_missed_date,omitempty"`
	LastDayChecked string    `json:"last_day_checked"`
	CreatedAt      time.Time `json:"created_at"`

	// Computed on read.
	Mood    string `json:"mood"`   // hiding, very_sick, sick, hungry, bored, happy, content
	// HealedToday: all of today's chores are done, so Health already shows
	// the gain tonight's check will make real (pet.ShowToday).
	HealedToday bool `json:"healed_today,omitempty"`
	Hiding  bool   `json:"hiding"` // missed chores yesterday and today's aren't done yet
	Sick    bool   `json:"sick"`
	CanPlay bool   `json:"can_play"`

	// Owner details for the shared farm view.
	OwnerName  string `json:"owner_name,omitempty"`
	OwnerColor string `json:"owner_color,omitempty"`
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
	ID                 int64   `json:"id"`
	UUID               string  `json:"uuid"`
	ChoreID            int64   `json:"chore_id"`
	DefaultAssignedTo  *int64  `json:"default_assigned_to,omitempty"`
	DefaultDueBy       *string `json:"default_due_by,omitempty"`
	DefaultAvailableAt *string `json:"default_available_at,omitempty"`
	Enabled            bool    `json:"enabled"`
	CooldownMinutes    int     `json:"cooldown_minutes"`
	AssignmentType     string  `json:"assignment_type"`
	AllowRepeat        bool    `json:"allow_repeat"`
	LastTriggeredAt    *string `json:"last_triggered_at,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

// --- Triggerable Chore (HA integration discovery) ---

type TriggerableChoreInfo struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon,omitempty"`
	PointsValue int    `json:"points_value"`
}

type TriggerInfo struct {
	ID                  int64   `json:"id"`
	UUID                string  `json:"uuid"`
	DefaultAssignedTo   *int64  `json:"default_assigned_to,omitempty"`
	DefaultAssignedName string  `json:"default_assigned_name,omitempty"`
	DefaultDueBy        *string `json:"default_due_by,omitempty"`
	DefaultAvailableAt  *string `json:"default_available_at,omitempty"`
	Enabled             bool    `json:"enabled"`
	CooldownMinutes     int     `json:"cooldown_minutes"`
}

type TriggerableChore struct {
	TriggerableChoreInfo
	Triggers []TriggerInfo `json:"triggers"`
}

// --- API Tokens ---

type APIToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
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
	Completed          bool       `json:"completed"`
	CompletionID       *int64     `json:"completion_id,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	PhotoURL           *string    `json:"photo_url,omitempty"`
	CompletionStatus   *string    `json:"completion_status,omitempty"`
	AIFeedback         *string    `json:"ai_feedback,omitempty"`
	CompletedByName    string     `json:"completed_by_name,omitempty"`
	CompletedBySibling bool       `json:"completed_by_sibling,omitempty"`
	TTSAudioURL        string     `json:"tts_audio_url,omitempty"`
	Date               string     `json:"date"`
}

// AIReviewResult is an AI photo review: advice for a parent, never a verdict.
type AIReviewResult struct {
	Complete   bool    `json:"complete"`
	Confidence float64 `json:"confidence"`
	Feedback   string  `json:"feedback"`
}

// --- Leaderboard & friend families (docs/friends.md) ---

// ScoreDay is one person's chores on one day, counted for the leaderboard:
// Done out of Assigned chores, the same in the chores' point values
// (DonePoints out of AssignedPoints, which is what the percentage is made
// from), and Bonus chores counted on the side. A Paused day is left out of
// the score.
type ScoreDay struct {
	Date           string `json:"date"`
	Assigned       int    `json:"assigned"`
	Done           int    `json:"done"`
	Bonus          int    `json:"bonus"`
	AssignedPoints int    `json:"assigned_points"`
	DonePoints     int    `json:"done_points"`
	Paused         bool   `json:"paused,omitempty"`
}

// Friend link statuses.
const (
	FriendStatusWaiting = "waiting" // invite made, the other family hasn't joined yet
	FriendStatusActive  = "active"
	FriendStatusEnded   = "ended" // the other family unlinked
)

// FriendLink is a link to another family's OpenChore, kept through a hub.
type FriendLink struct {
	ID            int64      `json:"id"`
	Status        string     `json:"status"`
	HubURL        string     `json:"hub_url"`
	HubLink       string     `json:"-"`
	Token         string     `json:"-"`
	Secret        string     `json:"-"`
	Invite        string     `json:"invite,omitempty"`
	InviteExpires *time.Time `json:"invite_expires,omitempty"`
	PeerName      string     `json:"peer_name"`
	Snapshot      string     `json:"-"`
	SnapshotAt    *time.Time `json:"snapshot_at,omitempty"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// FriendSnapshot is what one family sends another: the people a parent chose
// to share with that family, with their weekly score and chore counts,
// streak and farm animal. It carries only what the other family's screens
// show: never chore names, photos, point values, point balances, rewards, a
// day-by-day breakdown or the animal's health numbers.
type FriendSnapshot struct {
	Version     int            `json:"version"`
	Family      string         `json:"family"`
	GeneratedAt time.Time      `json:"generated_at"`
	Today       string         `json:"today"` // the sender's date, in its own time zone
	Members     []FriendMember `json:"members"`
	// Accepted lists the receiver's people (by their FriendMember.ID) that
	// the sender has chosen to show, so each parent can see whether the
	// other side has picked a kid up yet.
	Accepted []string `json:"accepted"`
}

type FriendMember struct {
	ID     string     `json:"id"` // stable within the sending family
	Name   string     `json:"name"`
	Color  string     `json:"color,omitempty"`
	Streak int        `json:"streak"`
	Paused bool       `json:"paused,omitempty"`
	// Weeks holds this week and last week (the sender's own weeks).
	Weeks []FriendWeek `json:"weeks"`
	Pet   *FriendPet   `json:"pet,omitempty"`
}

// FriendWeek is one person's week as sent to a friend family: how many
// chores, and the score as a share (0 to 10000, score.ShareScale). The point
// values the share was worked out from stay at home.
type FriendWeek struct {
	Start    string `json:"start"` // the Monday, YYYY-MM-DD
	Assigned int    `json:"assigned"`
	Done     int    `json:"done"`
	Bonus    int    `json:"bonus"`
	Share    int    `json:"share"`
}

// FriendPet is a farm animal as shown to a friend family, in its real state:
// how it looks and how it's doing, without the health, food and joy numbers.
type FriendPet struct {
	Name      string `json:"name"`
	Species   string `json:"species"`
	Color     string `json:"color,omitempty"`
	Accessory string `json:"accessory,omitempty"`
	Mood      string `json:"mood"`
	Hiding    bool   `json:"hiding"`
	Sick      bool   `json:"sick"`
}
