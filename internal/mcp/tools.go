package mcp

import (
	"context"
	"errors"
	"net/http"
)

// instructions are sent at initialize. They teach the assistant OpenChore's
// model so a parent can ask in plain language ("give Zoe the dishes on
// school days") without knowing about schedules or categories.
const instructions = `OpenChore is a family chore tracker. You are helping a parent manage it.

How it fits together:
- People are parents (role "admin") or kids (role "child"). Parents can be given chores too.
- A chore is the definition: title, description, emoji icon, category, points, and whether it needs a parent's approval or a photo.
- A schedule assigns a chore to ONE person on a pattern: a weekday, every N days from a start date, or one specific date. A chore done by two kids on weekdays is 10 schedules (2 people x 5 days). Use assign_chore to create them in one step, and household_overview to see them grouped into plain language ("Zoe: Mon-Fri").
- Optional schedule timing: available_at (HH:MM, can't be ticked off before), due_by (HH:MM deadline) with expiry_penalty: "block" (can't be done late), "no_points" (late earns nothing), or "penalty" (late costs expiry_penalty_value points). start_date / end_date limit when a schedule applies.
- Categories (labels the family sees in the app): "required" = Must do; "core" = Every day; "bonus" = Bonus; "optional" = Optional. Core points are held until that day's Must-do chores are done. Bonus points are only paid once all Must-do and Every-day chores for the day are done. Optional chores never count toward the day (no streak, penalty or gate) and pay any points straight away; kids can add their own optional chores (always 0 points). missed_penalty_value is deducted when a chore ends the day undone.
- Triggers assign a chore on demand, e.g. Home Assistant calling the trigger URL when the dryer finishes. assignment_type "individual" goes to the trigger's default person; "fcfs" (first come, first served) offers it to every kid and the first to finish earns the points. Triggered chores are for today only.
- Chores needing approval or a photo wait in list_pending_approvals until a parent approves or rejects them.
- Points live in a ledger (adjust_points for manual changes). Rewards cost points and can be limited or priced per kid. Streak milestones pay bonus points for consecutive complete days. Decay can slowly reduce idle balances.

Working well:
- Start with household_overview to see people, chores and who does what. It also flags chores nobody is assigned and likely duplicates.
- Refer to people and chores by name with the parent, and look up ids yourself.
- Confirm before bulk or destructive changes, and summarize what you changed afterwards.
- To stop or move an assignment, prefer unassign_chore (mode "retire"), which ends the schedule and keeps its history and reports. Deleting a schedule or chore also deletes its completion history.
- Dates are YYYY-MM-DD and times HH:MM in the household's local time.`

func allTools() []*Tool {
	var ts []*Tool
	add := func(e endpoint) { ts = append(ts, e.tool()) }

	// ---- Overview and chore helpers (household.go) ----
	ts = append(ts, householdTools()...)

	// ---- People ----
	add(endpoint{name: "list_people", title: "List people", readOnly: true,
		desc:   "List everyone in the household (parents and kids) with ids, roles, ages and whether they're paused.",
		method: http.MethodGet, path: "/users"})
	add(endpoint{name: "get_person", title: "Get a person", readOnly: true,
		desc:   "Get one person's profile.",
		method: http.MethodGet, path: "/users/{id}", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "create_person", title: "Add a person",
		desc:   "Add a parent or kid. A parent (role admin) must be given a 4-8 digit PIN.",
		method: http.MethodPost, path: "/users", required: []string{"name"},
		props: map[string]any{
			"name":       str("Display name."),
			"role":       enum("admin for a parent, child for a kid (default child).", "admin", "child"),
			"age":        integer("Age in years (picks a default look for kids)."),
			"pin":        str("Profile PIN, 4-8 digits. Required for parents."),
			"avatar_url": str("Avatar image URL."),
			"theme":      enum("App look.", "sunroom", "blocks", "tint"),
			"color":      enum("Person colour.", "coral", "mint", "butter", "sky", "rose", "leaf", "lilac", "sand"),
		}})
	ts = append(ts, updatePersonTool())
	add(endpoint{name: "delete_person", title: "Remove a person",
		desc:   "Delete a person and all of their schedules, completions and points. The last parent can't be removed. Prefer pause_person for a temporary break.",
		method: http.MethodDelete, path: "/users/{id}", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "pause_person", title: "Pause a person",
		desc:   "Vacation mode: pause someone's chores (no chores, misses or penalties) until unpaused.",
		method: http.MethodPut, path: "/users/{id}/pause", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "unpause_person", title: "Unpause a person",
		desc:   "Resume a paused person's chores.",
		method: http.MethodPut, path: "/users/{id}/unpause", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "set_person_pin", title: "Set a PIN",
		desc:   "Set or reset someone's profile PIN (4-8 digits). Parents can reset a kid's PIN without the old one.",
		method: http.MethodPut, path: "/users/{id}/pin", required: []string{"new_pin"},
		props: map[string]any{
			"id":          integer("Person id."),
			"new_pin":     str("New PIN, 4-8 digits."),
			"current_pin": str("Current PIN, only needed when changing your own."),
		}})
	add(endpoint{name: "clear_person_pin", title: "Remove a PIN",
		desc:   "Remove someone's PIN so their profile opens with a tap. Parents always need a PIN or linked account.",
		method: http.MethodDelete, path: "/users/{id}/pin",
		props: map[string]any{"id": integer("Person id."), "current_pin": str("Current PIN, only needed for your own.")}})
	add(endpoint{name: "list_linked_accounts", title: "List linked accounts", readOnly: true,
		desc:   "List the single sign-on accounts linked to a profile.",
		method: http.MethodGet, path: "/users/{id}/identities", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "unlink_account", title: "Unlink an account",
		desc:   "Unlink a single sign-on account from a profile (signs that person out everywhere).",
		method: http.MethodDelete, path: "/users/{id}/identities/{identity_id}",
		props: map[string]any{"id": integer("Person id."), "identity_id": integer("Linked account id from list_linked_accounts.")}})
	ts = append(ts, &Tool{
		Name: "whoami", Title: "Who am I", ReadOnly: true,
		Description: "Show which parent profile or API token this connection is acting as.",
		Schema:      obj(map[string]any{}),
		Covers:      []string{"GET /auth/me"},
		Run: func(ctx context.Context, c *caller, a Args) (any, error) {
			out, err := c.do(ctx, http.MethodGet, "/auth/me", nil, nil)
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
				// /auth/me only describes sessions; reaching this tool at
				// all means an API token was accepted.
				return map[string]any{"acting_as": "API token", "access": "parent (admin)"}, nil
			}
			return out, err
		},
	})

	// ---- Chores and schedules ----
	add(endpoint{name: "list_chores", title: "List chores", readOnly: true,
		desc:   "List every chore definition (without who it's assigned to; household_overview has that).",
		method: http.MethodGet, path: "/chores"})
	add(endpoint{name: "get_chore", title: "Get a chore", readOnly: true,
		desc:   "Get one chore's full definition.",
		method: http.MethodGet, path: "/chores/{id}", props: map[string]any{"id": integer("Chore id.")}})
	add(endpoint{name: "delete_chore", title: "Delete a chore",
		desc:   "Delete a chore with all its schedules, triggers and completion history (points already earned stay). To stop it without losing history, use unassign_chore instead.",
		method: http.MethodDelete, path: "/chores/{id}", props: map[string]any{"id": integer("Chore id.")}})
	add(endpoint{name: "list_schedules", title: "List a chore's schedules", readOnly: true,
		desc:   "Raw schedule rows for a chore (one per person per weekday). household_overview shows the same thing grouped.",
		method: http.MethodGet, path: "/chores/{chore_id}/schedules", props: map[string]any{"chore_id": integer("Chore id.")}})
	add(endpoint{name: "update_schedule", title: "Edit a schedule",
		desc: "Change one schedule's timing or scoring in place. Omitted fields are kept; null clears. " +
			"The person and days can't change here: use unassign_chore then assign_chore.",
		method: http.MethodPut, path: "/schedules/{schedule_id}",
		props: map[string]any{
			"schedule_id":          integer("Schedule id."),
			"available_at":         nullable(str("Earliest time it can be done, HH:MM.")),
			"due_by":               nullable(str("Deadline, HH:MM.")),
			"expiry_penalty":       enum("What happens after due_by.", "block", "no_points", "penalty"),
			"expiry_penalty_value": integer("Points deducted when late, for penalty mode."),
			"points_multiplier":    number("Multiplier on the chore's points (1 = normal)."),
			"start_date":           nullable(str("First day it applies, YYYY-MM-DD.")),
			"end_date":             nullable(str("Last day it applies, YYYY-MM-DD.")),
		}})
	add(endpoint{name: "delete_schedule", title: "Delete a schedule",
		desc:   "Delete one schedule row and its completion history. Prefer unassign_chore with mode retire.",
		method: http.MethodDelete, path: "/chores/{chore_id}/schedules/{schedule_id}",
		props: map[string]any{"chore_id": integer("Chore id."), "schedule_id": integer("Schedule id.")}})
	add(endpoint{name: "get_person_chores", title: "A person's chores", readOnly: true,
		desc:   "What a person has on a day (or that week, Monday-Sunday): each scheduled chore with its schedule_id and whether it's done, pending approval, late or locked.",
		method: http.MethodGet, path: "/users/{id}/chores", query: []string{"view", "date"},
		props: map[string]any{
			"id":   integer("Person id."),
			"view": enum("daily (default) or weekly.", "daily", "weekly"),
			"date": str("YYYY-MM-DD, default today."),
		}})
	add(endpoint{name: "complete_chore", title: "Mark a chore done", safe: true,
		desc:   "Tick off a chore for its assignee (by schedule_id from get_person_chores). Points go to the assignee; a parent doesn't need a photo.",
		method: http.MethodPost, path: "/schedules/{schedule_id}/complete",
		props: map[string]any{
			"schedule_id":     integer("Schedule id."),
			"completion_date": str("YYYY-MM-DD, default today."),
			"skip_photo":      boolean("Finish a photo chore without a photo (it then waits for approval)."),
		}})
	add(endpoint{name: "uncomplete_chore", title: "Undo a completion",
		desc:   "Un-tick a chore for a date and take back its points.",
		method: http.MethodDelete, path: "/schedules/{schedule_id}/complete", query: []string{"date"},
		props: map[string]any{"schedule_id": integer("Schedule id."), "date": str("YYYY-MM-DD, default today.")}})
	add(endpoint{name: "excuse_chore", title: "Excuse a chore",
		desc:   "Excuse a chore for a day (sick, away...): it no longer counts as missed and any penalty is refunded.",
		method: http.MethodPost, path: "/schedules/{schedule_id}/excuse",
		props: map[string]any{
			"schedule_id": integer("Schedule id."),
			"date":        str("YYYY-MM-DD, default today."),
			"reason":      str("Why (shown in history)."),
		}})

	// ---- Approvals ----
	add(endpoint{name: "list_pending_approvals", title: "Pending approvals", readOnly: true,
		desc:   "Completions waiting for a parent: chore, who did it, photo URL and any AI photo-review note.",
		method: http.MethodGet, path: "/completions/pending"})
	add(endpoint{name: "approve_completion", title: "Approve",
		desc:   "Approve a pending completion and release its points.",
		method: http.MethodPost, path: "/completions/{completion_id}/approve", props: map[string]any{"completion_id": integer("Completion id.")}})
	add(endpoint{name: "reject_completion", title: "Reject",
		desc:   "Reject a pending completion (no points).",
		method: http.MethodPost, path: "/completions/{completion_id}/reject", props: map[string]any{"completion_id": integer("Completion id.")}})

	// ---- Points, decay, streaks ----
	add(endpoint{name: "get_point_balances", title: "Point balances", readOnly: true,
		desc:   "Everyone's current point balance.",
		method: http.MethodGet, path: "/points/balances"})
	add(endpoint{name: "get_person_points", title: "A person's points", readOnly: true,
		desc:   "One person's balance and full points history.",
		method: http.MethodGet, path: "/users/{id}/points", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "adjust_points", title: "Adjust points",
		desc:   "Give (positive) or take away (negative) points by hand, with a note for the history.",
		method: http.MethodPost, path: "/points/adjust", required: []string{"user_id", "amount"},
		props: map[string]any{
			"user_id": integer("Person id."),
			"amount":  integer("Points to add (negative to remove). Not zero."),
			"note":    str("Reason shown in the history."),
		}})
	add(endpoint{name: "get_decay_settings", title: "Point decay settings", readOnly: true,
		desc:   "A person's point decay settings (balances slowly shrinking).",
		method: http.MethodGet, path: "/admin/users/{id}/decay", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "set_decay_settings", title: "Set point decay",
		desc:   "Turn point decay on or off for a person: lose decay_rate points every decay_interval_hours.",
		method: http.MethodPut, path: "/admin/users/{id}/decay", required: []string{"enabled"},
		props: map[string]any{
			"id":                   integer("Person id."),
			"enabled":              boolean("On or off."),
			"decay_rate":           integer("Points removed each interval."),
			"decay_interval_hours": integer("Hours between decays (default 24)."),
		}})
	add(endpoint{name: "get_person_streak", title: "A person's streak", readOnly: true,
		desc:   "Current and longest streak of complete days, and the next milestone.",
		method: http.MethodGet, path: "/users/{id}/streak", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "list_streak_milestones", title: "Streak milestones", readOnly: true,
		desc:   "Bonus points paid at streak lengths.",
		method: http.MethodGet, path: "/admin/streak-rewards"})
	add(endpoint{name: "create_streak_milestone", title: "Add a streak milestone",
		desc:   "Pay bonus_points when someone reaches streak_days complete days in a row.",
		method: http.MethodPost, path: "/admin/streak-rewards", required: []string{"streak_days", "bonus_points"},
		props: map[string]any{
			"streak_days":  integer("Days in a row."),
			"bonus_points": integer("Points paid."),
			"label":        str("Name, e.g. \"One week!\"."),
		}})
	add(endpoint{name: "delete_streak_milestone", title: "Remove a streak milestone",
		desc:   "Delete a streak milestone.",
		method: http.MethodDelete, path: "/admin/streak-rewards/{id}", props: map[string]any{"id": integer("Milestone id.")}})

	// ---- Rewards ----
	add(endpoint{name: "list_rewards", title: "List rewards", readOnly: true,
		desc:   "Every reward, including hidden ones, with per-kid visibility and prices.",
		method: http.MethodGet, path: "/rewards/all"})
	add(endpoint{name: "create_reward", title: "Add a reward",
		desc:   "Add a reward kids can spend points on.",
		method: http.MethodPost, path: "/rewards", required: []string{"name", "cost"},
		props: map[string]any{
			"name":        str("Reward name."),
			"description": str("Details."),
			"icon":        str("An emoji."),
			"cost":        integer("Price in points (positive)."),
			"stock":       integer("How many are available (omit for unlimited)."),
			"shareable":   boolean("Kids can pool points toward it together."),
		}})
	add(endpoint{name: "update_reward", title: "Edit a reward",
		desc:   "Change a reward; omitted fields are kept. active=false hides it.",
		method: http.MethodPut, path: "/rewards/{id}",
		props: map[string]any{
			"id":          integer("Reward id."),
			"name":        str("Reward name."),
			"description": str("Details."),
			"icon":        str("An emoji."),
			"cost":        integer("Price in points."),
			"stock":       integer("How many are available."),
			"active":      boolean("Shown to kids."),
			"shareable":   boolean("Kids can pool points toward it."),
		}})
	add(endpoint{name: "delete_reward", title: "Delete a reward",
		desc:   "Delete a reward. Setting active=false with update_reward hides it instead.",
		method: http.MethodDelete, path: "/rewards/{id}", props: map[string]any{"id": integer("Reward id.")}})
	add(endpoint{name: "set_reward_visibility", title: "Who sees a reward",
		desc: "Choose which people can see a reward and optionally give each a custom price. " +
			"This replaces the whole list; an empty list makes it visible to everyone at its normal cost.",
		method: http.MethodPut, path: "/rewards/{id}/assignments", required: []string{"assignments"},
		props: map[string]any{
			"id": integer("Reward id."),
			"assignments": map[string]any{
				"type":        "array",
				"description": "People who can see it.",
				"items": obj(map[string]any{
					"user_id":     integer("Person id."),
					"custom_cost": integer("Price for this person (omit for the normal cost)."),
				}, "user_id"),
			},
		}})
	add(endpoint{name: "list_redemptions", title: "A person's redemptions", readOnly: true,
		desc:   "Rewards a person has bought.",
		method: http.MethodGet, path: "/users/{id}/redemptions", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "undo_redemption", title: "Undo a redemption",
		desc:   "Cancel a reward purchase and refund the points.",
		method: http.MethodDelete, path: "/redemptions/{redemption_id}", props: map[string]any{"redemption_id": integer("Redemption id.")}})
	add(endpoint{name: "list_savings_goals", title: "A person's savings goals", readOnly: true,
		desc:   "Rewards a kid is saving points toward, with progress.",
		method: http.MethodGet, path: "/users/{id}/commitments", props: map[string]any{"id": integer("Person id.")}})
	add(endpoint{name: "get_shared_goal", title: "A shared savings goal", readOnly: true,
		desc:   "A reward several kids are saving toward together, with each one's share.",
		method: http.MethodGet, path: "/pools/{id}", props: map[string]any{"id": integer("Shared pool id.")}})

	// ---- Triggers ----
	triggerProps := map[string]any{
		"default_assigned_to":  integer("Person who gets it (individual mode)."),
		"default_due_by":       str("Deadline, HH:MM."),
		"default_available_at": str("Earliest time, HH:MM."),
		"enabled":              boolean("Whether the trigger URL works."),
		"cooldown_minutes":     integer("Ignore repeat fires within this many minutes."),
		"assignment_type":      enum("individual: the default person; fcfs: every kid, first to finish earns it.", "individual", "fcfs"),
	}
	add(endpoint{name: "list_triggers", title: "A chore's triggers", readOnly: true,
		desc:   "A chore's trigger URLs (uuid) and settings.",
		method: http.MethodGet, path: "/chores/{chore_id}/triggers", props: map[string]any{"chore_id": integer("Chore id.")}})
	add(endpoint{name: "create_trigger", title: "Add a trigger",
		desc: "Let something like Home Assistant assign a chore on demand (e.g. when the dryer finishes). " +
			"Returns a uuid; the automation POSTs to /api/hooks/trigger/{uuid}. Each fire assigns the chore for today.",
		method: http.MethodPost, path: "/chores/{chore_id}/triggers",
		props: merge(map[string]any{"chore_id": integer("Chore id.")}, triggerProps)})
	add(endpoint{name: "update_trigger", title: "Edit a trigger",
		desc:   "Change a trigger's settings; omitted fields are kept.",
		method: http.MethodPut, path: "/triggers/{id}",
		props: merge(map[string]any{"id": integer("Trigger id (not the uuid).")}, triggerProps)})
	add(endpoint{name: "delete_trigger", title: "Delete a trigger",
		desc:   "Delete a trigger; its URL stops working.",
		method: http.MethodDelete, path: "/triggers/{id}", props: map[string]any{"id": integer("Trigger id.")}})
	add(endpoint{name: "list_triggerable_chores", title: "Chores with triggers", readOnly: true,
		desc:   "Every chore that has triggers, with their settings and uuids.",
		method: http.MethodGet, path: "/chores/triggerable"})
	add(endpoint{name: "fire_trigger", title: "Fire a trigger now",
		desc:   "Fire a trigger as Home Assistant would, assigning its chore for today.",
		method: http.MethodPost, path: "/hooks/trigger/{uuid}", query: []string{"due_by", "available_at", "assign_to"},
		props: map[string]any{
			"uuid":         str("Trigger uuid."),
			"due_by":       str("Override the deadline, HH:MM."),
			"available_at": str("Override the earliest time, HH:MM."),
			"assign_to":    str("Override who gets it, by name (individual mode)."),
		}})

	// ---- Reports ----
	add(endpoint{name: "get_report", title: "Reports", readOnly: true,
		desc:   "Completion stats behind the Reports page for a week, month or year: per person, per chore, per day.",
		method: http.MethodGet, path: "/admin/reports", query: []string{"period", "date"},
		props: map[string]any{
			"period": enum("Default week.", "week", "month", "year"),
			"date":   str("Any day in the period, YYYY-MM-DD (default today)."),
		}})
	add(endpoint{name: "get_ai_summary", title: "AI summary", readOnly: true,
		desc:   "A written summary of someone's week/month/year (needs AI configured).",
		method: http.MethodGet, path: "/admin/reports/ai-summary", query: []string{"user_id", "period", "date"}, required: []string{"user_id"},
		props: map[string]any{
			"user_id": integer("Person id."),
			"period":  enum("Default week.", "week", "month", "year"),
			"date":    str("Any day in the period, YYYY-MM-DD."),
		}})

	// ---- Settings ----
	add(endpoint{name: "get_setting", title: "Read a setting", readOnly: true,
		desc:   "Read a setting. Keys: base_url, discord_webhook_url, ai_photo_review, ai_auto_approve, ai_auto_approve_threshold, ai_weekly_summary, tts_voice.",
		method: http.MethodGet, path: "/admin/settings/{key}", props: map[string]any{"key": str("Setting key.")}})
	add(endpoint{name: "set_setting", title: "Change a setting",
		desc:   "Change a setting (values are strings, e.g. \"true\"). Keys: base_url, discord_webhook_url, ai_photo_review, ai_auto_approve, ai_auto_approve_threshold, ai_weekly_summary, tts_voice.",
		method: http.MethodPut, path: "/admin/settings/{key}", required: []string{"value"},
		props: map[string]any{"key": str("Setting key."), "value": str("New value.")}})
	add(endpoint{name: "export_config", title: "Export configuration", readOnly: true,
		desc:   "The household's setup as config.yaml text: a readable backup of people, chores, schedules, rewards and settings.",
		method: http.MethodGet, path: "/admin/export-config", query: []string{"sections"},
		props: map[string]any{"sections": str("Comma-separated subset of users,chores,rewards,streak_rewards,settings.")}})

	// ---- Integrations: webhooks and API tokens ----
	add(endpoint{name: "list_webhooks", title: "List webhooks", readOnly: true,
		desc:   "Outbound webhooks OpenChore posts events to.",
		method: http.MethodGet, path: "/admin/webhooks"})
	add(endpoint{name: "create_webhook", title: "Add a webhook",
		desc: "Post events to a URL. events is comma-separated or * for all: chore.completed, chore.uncompleted, chore.expired, chore.missed, " +
			"chore.fcfs_completed, reward.redeemed, daily.complete, streak.milestone, points.decayed, report.weekly_summary.",
		method: http.MethodPost, path: "/admin/webhooks", required: []string{"url"},
		props: map[string]any{
			"url":    str("Destination URL."),
			"secret": str("HMAC signing secret."),
			"events": str("Events, comma-separated, or *."),
		}})
	add(endpoint{name: "update_webhook", title: "Edit a webhook",
		desc:   "Change a webhook; omitted fields are kept.",
		method: http.MethodPut, path: "/admin/webhooks/{id}",
		props: map[string]any{
			"id":     integer("Webhook id."),
			"url":    str("Destination URL."),
			"secret": str("HMAC signing secret."),
			"events": str("Events, comma-separated, or *."),
			"active": boolean("Enabled."),
		}})
	add(endpoint{name: "delete_webhook", title: "Delete a webhook",
		desc:   "Delete a webhook.",
		method: http.MethodDelete, path: "/admin/webhooks/{id}", props: map[string]any{"id": integer("Webhook id.")}})
	add(endpoint{name: "list_webhook_deliveries", title: "Webhook delivery log", readOnly: true,
		desc:   "Recent delivery attempts for a webhook, with response codes and bodies.",
		method: http.MethodGet, path: "/admin/webhooks/{id}/deliveries", props: map[string]any{"id": integer("Webhook id.")}})
	add(endpoint{name: "list_api_tokens", title: "List API tokens", readOnly: true,
		desc:   "API tokens (names and last use; the secrets aren't shown).",
		method: http.MethodGet, path: "/admin/tokens"})
	add(endpoint{name: "create_api_token", title: "Create an API token",
		desc:   "Create an API token for an integration. It has full parent access and is shown only once.",
		method: http.MethodPost, path: "/admin/tokens", required: []string{"name"},
		props: map[string]any{"name": str("What it's for.")}})
	add(endpoint{name: "revoke_api_token", title: "Revoke an API token",
		desc:   "Revoke an API token. Revoking the one this connection uses disconnects you.",
		method: http.MethodDelete, path: "/admin/tokens/{id}", props: map[string]any{"id": integer("Token id.")}})

	// ---- Sign-in ----
	add(endpoint{name: "get_sign_in_config", title: "Sign-in settings", readOnly: true,
		desc:   "Single sign-on providers, session lengths and sign-in options.",
		method: http.MethodGet, path: "/admin/auth/config"})
	add(endpoint{name: "set_session_lengths", title: "Session lengths",
		desc:   "How long sign-ins last, in hours, on shared (kiosk) and personal devices. null restores the default.",
		method: http.MethodPut, path: "/admin/auth/sessions",
		props: map[string]any{
			"kiosk_session_hours":    nullable(integer("Shared device session hours.")),
			"personal_session_hours": nullable(integer("Personal device session hours.")),
		}})
	add(endpoint{name: "set_sign_in_options", title: "Sign-in options",
		desc:   "Whether people with linked accounts may also use a PIN, and where signed-out visitors land.",
		method: http.MethodPut, path: "/admin/auth/options",
		props: map[string]any{
			"pin_sign_in": boolean("Allow PIN sign-in for people with a linked account."),
			"start_page":  str("picker, wall, or provider:<id>."),
		}})
	providerProps := map[string]any{
		"name":          str("Button label, e.g. \"Pocket ID\"."),
		"issuer":        str("OIDC issuer URL."),
		"client_id":     str("OAuth client id."),
		"client_secret": str("OAuth client secret (omit on edit to keep)."),
		"scopes":        str("Space-separated scopes (default \"openid profile email\")."),
		"prompt":        str("OIDC prompt parameter, e.g. select_account."),
	}
	add(endpoint{name: "add_sign_in_provider", title: "Add a sign-in provider",
		desc:   "Add an OpenID Connect single sign-on provider.",
		method: http.MethodPost, path: "/admin/auth/providers", required: []string{"id", "name", "issuer", "client_id"},
		props: merge(map[string]any{"id": str("Short id used in URLs, e.g. pocket.")}, providerProps)})
	add(endpoint{name: "update_sign_in_provider", title: "Edit a sign-in provider",
		desc:   "Edit a provider added in the app (not ones from config.yaml).",
		method: http.MethodPut, path: "/admin/auth/providers/{id}",
		props: merge(map[string]any{"id": str("Provider id.")}, providerProps)})
	add(endpoint{name: "remove_sign_in_provider", title: "Remove a sign-in provider",
		desc:   "Remove a provider added in the app. Refused if it would lock someone out.",
		method: http.MethodDelete, path: "/admin/auth/providers/{id}", props: map[string]any{"id": str("Provider id.")}})
	add(endpoint{name: "test_sign_in_provider", title: "Test a sign-in provider", readOnly: true,
		desc:   "Check that a provider's issuer can be reached and discovered.",
		method: http.MethodPost, path: "/admin/auth/providers/{id}/test", props: map[string]any{"id": str("Provider id.")}})

	// ---- AI and read-aloud audio ----
	add(endpoint{name: "get_ai_status", title: "AI status", readOnly: true,
		desc:   "Whether the optional AI model and read-aloud voice are configured.",
		method: http.MethodGet, path: "/admin/ai/status"})
	add(endpoint{name: "get_ai_config", title: "AI configuration", readOnly: true,
		desc:   "Where the AI model and speech service live (keys are not shown).",
		method: http.MethodGet, path: "/admin/ai/config"})
	aiService := obj(map[string]any{
		"base_url": str("OpenAI-compatible base URL."),
		"model":    str("Model or voice model name."),
		"api_key":  str("API key; omit to keep, empty string clears."),
	})
	add(endpoint{name: "update_ai_config", title: "Configure AI",
		desc:   "Point OpenChore at an OpenAI-compatible model (ai) and/or speech service (tts). Sections set by environment variables can't be changed.",
		method: http.MethodPut, path: "/admin/ai/config",
		props: map[string]any{"ai": aiService, "tts": aiService}})
	add(endpoint{name: "draft_chore_description", title: "Draft a description", readOnly: true,
		desc:   "Have the AI draft a kid-friendly chore description (needs AI configured). Nothing is saved.",
		method: http.MethodPost, path: "/admin/ai/generate-description", required: []string{"title"},
		props: map[string]any{"title": str("Chore title."), "category": enum("Chore category.", "required", "core", "bonus", "optional")}})
	add(endpoint{name: "regenerate_all_audio", title: "Re-record all audio",
		desc:   "Re-record every chore's read-aloud audio (e.g. after changing the voice).",
		method: http.MethodPost, path: "/admin/tts/regenerate"})
	add(endpoint{name: "regenerate_chore_audio", title: "Re-record a chore's audio",
		desc:   "Re-record one chore's read-aloud audio.",
		method: http.MethodPost, path: "/chores/{id}/tts/regenerate", props: map[string]any{"id": integer("Chore id.")}})

	return ts
}
