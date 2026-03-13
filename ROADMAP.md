# OpenChore Roadmap

## MVP (Completed)
- [x] Go REST API backend with SQLite
- [x] User management (name, avatar, role: admin/child)
- [x] Chore CRUD with categories (required/core/bonus)
- [x] Recurring schedules (day-of-week), one-off chores, and interval recurrence (every N days)
- [x] Chore assignment to users (multi-child, day presets, quick assign)
- [x] Daily/weekly chore view per user
- [x] Mark chores as completed (with approval status: auto-approved for MVP)
- [x] Time-locked chores (available_at — server-enforced, can't complete before designated time)
- [x] Admin endpoints gated by user role
- [x] Admin dashboard with passcode protection (chore/schedule/user management)
- [x] iPad/iPhone-first touch design (44px min tap targets)

## Phase 2: Points & Economy (Completed)
- [x] Points earned for completing chores (points_value on chores, points_multiplier on schedules)
- [x] Points decay — per-user configurable daily decay if non-bonus chores weren't all completed
- [x] Required chores must be completed before core chore points apply
- [x] Transaction ledger (credit/debit/balance) as single source of truth
- [x] Rewards store — kids spend points on predefined rewards (screen time, pick dinner, etc.)
- [x] Per-kid reward visibility and custom pricing
- [x] Redemption history with admin undo
- [x] Configurable expiry penalties per schedule (block/no_points/penalty)
- [ ] Bonus chore points only count once required + core chores are complete

## Phase 3: Approval & Notifications
- [ ] Parent approval workflow for chore completions (pending/approved/rejected)
- [ ] Push notifications for approval requests
- [ ] Photo proof of completion (photo_url on completions)
- [ ] Weekly report for parents (completion rates, streaks, points earned)

## Phase 4: Smart Scheduling
- [ ] Schedule overrides (temporary changes for a specific week without editing recurring schedule)
- [ ] Proposal system (proposals table: type, payload JSON, status, reviewed_by)
- [ ] LLM-powered chore rebalancing (calendar integration detects absences, generates proposals)
- [ ] Chore rotation — auto-rotate assignments weekly
- [ ] Vacation/sick mode — pause a kid's chores without deleting schedules
- [ ] Fair distribution dashboard for parents

## Phase 5: Social & Gamification (Partially Complete)
- [x] Streaks tracking and display
- [x] Streak rewards — admin-configurable rewards for hitting streak milestones
- [ ] Achievements/badges (milestone-driven, computed server-side)
- [ ] Leaderboard (optional per-family setting)
- [ ] Chore swaps/trading between siblings (parent-approved)
- [ ] Helper mode — partial credit for helping on someone else's chore
- [ ] Family chores (assignment_type: family — anyone can complete, points only if their own chores are done)

## Phase 6: Integrations & Plugins (Partially Complete)
- [x] Webhook system with HMAC signing and delivery logging
- [ ] Plugin architecture for external chore sources (source + external_id on chores)
- [ ] Home Assistant integration (e.g., laundry done notification creates a chore)
- [ ] Calendar integration (Google/Apple Calendar for absence detection)

## Phase 7: UX Enhancements (Partially Complete)
- [x] Per-kid themes (4 themes with custom labels, icons, greetings, sounds, confetti)
- [x] Visual chore icons (emoji/icon on chores)
- [x] Ambient dashboard mode for wall-mounted iPad (family overview when idle)
- [x] Time estimates on chores (estimated_minutes)
- [x] Tooltips on admin UI form fields
- [ ] Text-to-speech for younger kids (age/birth_date on users, simplified TTS descriptions)
- [ ] Morning/afternoon/evening chore grouping (using available_at)
- [ ] Chore templates (age-appropriate packs parents can import)
