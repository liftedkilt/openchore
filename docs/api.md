# API Reference

The API is served under `/api` and speaks JSON in both directions. Errors come
back as `{"error": "..."}` with a matching HTTP status.

## Authentication

There are no sessions. A request identifies itself in one of three ways:

| Method | Header | Used by |
|--------|--------|---------|
| User | `X-User-ID: <id>` | The web app, after profile selection |
| Token | `Authorization: Bearer <token>` | Integrations (Home Assistant, scripts) |
| None | — | Public endpoints only |

Admin endpoints additionally require the caller to have the `admin` role. The
admin panel itself is gated behind a passcode (`POST /api/admin/verify`), which
is a UI lock rather than a transport-level credential.

Uploaded photos are served from `/uploads/*` and generated TTS audio from
`/tts/*`.

---

## Public

No authentication required.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users` | List users (drives the profile selection screen) |
| `GET` | `/api/users/{id}` | Get one user |
| `POST` | `/api/users/{id}/verify-pin` | Verify a profile PIN from the login screen |
| `POST` | `/api/admin/verify` | Verify the admin passcode |
| `POST` | `/api/setup` | First-run setup — only succeeds while no users exist |
| `POST` | `/api/hooks/trigger/{uuid}` | Fire a chore trigger; the UUID is the credential |

---

## Authenticated

Requires `X-User-ID` or a Bearer token.

### Chores

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users/{id}/chores?view=daily&date=YYYY-MM-DD` | Scheduled chores (`view=daily` or `weekly`) |
| `POST` | `/api/schedules/{id}/complete` | Complete a chore; body may set `completion_date` |
| `DELETE` | `/api/schedules/{id}/complete?date=YYYY-MM-DD` | Undo a completion |
| `POST` | `/api/upload` | Upload photo proof (multipart) |
| `PUT` | `/api/completions/{id}/photo` | Attach an uploaded photo to a completion |

### Points, streaks, rewards

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users/{id}/points` | Balance plus the transaction ledger |
| `GET` | `/api/users/{id}/streak` | Current streak and next milestone |
| `GET` | `/api/rewards` | Rewards visible to this user |
| `POST` | `/api/rewards/{id}/redeem` | Redeem a reward |
| `GET` | `/api/users/{id}/redemptions` | Redemption history |

### Reward commitments

Kids can pledge points toward a specific reward and watch progress accrue,
instead of spending balance the moment it lands.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users/{id}/commitments` | List this user's commitments |
| `POST` | `/api/rewards/{id}/commit` | Commit to saving for a reward |
| `POST` | `/api/commitments/{id}/contribute` | Put points toward a commitment |
| `PUT` | `/api/commitments/{id}/auto-contribute` | Toggle automatic contribution |
| `DELETE` | `/api/commitments/{id}` | Break a commitment |
| `GET` | `/api/pools/{id}` | Read a shared savings pool |

### Profile preferences

| Method | Path | Description |
|--------|------|-------------|
| `PUT` | `/api/users/{id}/theme` | Set theme |
| `PUT` | `/api/users/{id}/avatar` | Set avatar URL |
| `PUT` | `/api/users/{id}/line-color` | Set the ambient graph line color |
| `PUT` | `/api/users/{id}/pin` | Set a profile PIN |
| `DELETE` | `/api/users/{id}/pin` | Clear the profile PIN |

---

## Admin

Requires an authenticated caller with the `admin` role.

### People

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/users` | Create a user |
| `PUT` | `/api/users/{id}` | Update a user |
| `DELETE` | `/api/users/{id}` | Delete a user |
| `PUT` | `/api/users/{id}/pause` | Pause chores (vacation mode) |
| `PUT` | `/api/users/{id}/unpause` | Resume chores |

### Chores and schedules

| Method | Path | Description |
|--------|------|-------------|
| `GET` `POST` | `/api/chores` | List / create chores |
| `GET` `PUT` `DELETE` | `/api/chores/{id}` | Read / update / delete a chore |
| `GET` `POST` | `/api/chores/{id}/schedules` | List / create schedules |
| `DELETE` | `/api/chores/{id}/schedules/{scheduleID}` | Delete a schedule |
| `POST` | `/api/schedules/{scheduleID}/excuse` | Excuse a chore and waive its penalty |

### Approvals

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/completions/pending` | Completions awaiting review |
| `POST` | `/api/completions/{id}/approve` | Approve, releasing points |
| `POST` | `/api/completions/{id}/reject` | Reject |

### Points and rewards

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/points/balances` | Every user's balance |
| `POST` | `/api/points/adjust` | Manual credit or debit |
| `GET` `PUT` | `/api/admin/users/{id}/decay` | Per-user decay configuration |
| `GET` | `/api/rewards/all` | All rewards, including hidden ones |
| `POST` | `/api/rewards` | Create a reward |
| `PUT` `DELETE` | `/api/rewards/{id}` | Update / delete a reward |
| `PUT` | `/api/rewards/{id}/assignments` | Per-kid visibility and pricing |
| `DELETE` | `/api/redemptions/{redemptionID}` | Undo a redemption |
| `GET` `POST` | `/api/admin/streak-rewards` | List / create streak milestones |
| `DELETE` | `/api/admin/streak-rewards/{id}` | Delete a milestone |

### Integrations

| Method | Path | Description |
|--------|------|-------------|
| `GET` `POST` | `/api/chores/{id}/triggers` | List / create per-chore trigger URLs |
| `PUT` `DELETE` | `/api/triggers/{id}` | Update / delete a trigger |
| `GET` | `/api/chores/triggerable` | Discovery: chores with triggers, plus the user list |
| `GET` `POST` | `/api/admin/webhooks` | List / create outbound webhooks |
| `PUT` `DELETE` | `/api/admin/webhooks/{id}` | Update / delete a webhook |
| `GET` | `/api/admin/webhooks/{id}/deliveries` | Delivery log with response bodies |
| `GET` `POST` | `/api/admin/tokens` | List / create API tokens |
| `DELETE` | `/api/admin/tokens/{id}` | Revoke a token |

### Reports, settings, AI

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/admin/reports` | Analytics data behind the reports page |
| `GET` | `/api/admin/reports/ai-summary` | Narrative summary of a kid's week |
| `GET` `PUT` | `/api/admin/settings/{key}` | Read / write a setting |
| `PUT` | `/api/admin/passcode` | Change the admin passcode |
| `GET` | `/api/admin/export-config` | Export current configuration as YAML |
| `POST` | `/api/admin/ai/test` | Test photo review against an image |
| `POST` | `/api/admin/ai/tts` | Synthesize speech for arbitrary text |
| `POST` | `/api/admin/ai/tts-sync` | Kick off a TTS sync pass |
| `POST` | `/api/admin/ai/generate-description` | Draft a chore description |
| `POST` | `/api/admin/ai/suggest-points` | Suggest a point value for a chore |
| `POST` | `/api/chores/{id}/tts/regenerate` | Regenerate one chore's audio |
| `POST` | `/api/chores/{id}/tts/generate-description` | Regenerate one chore's spoken description |

---

## Outbound webhooks

OpenChore posts JSON to registered URLs when something happens. Each request
carries an HMAC-SHA256 signature derived from the webhook's secret, and every
attempt is recorded in the delivery log.

| Event | Fires when |
|-------|-----------|
| `chore.completed` | A chore is marked done |
| `chore.uncompleted` | A completion is undone |
| `chore.expired` | A deadline passes with the chore unfinished |
| `reward.redeemed` | A reward is redeemed |
| `daily.complete` | A kid finishes everything non-bonus for the day |
| `streak.milestone` | A streak milestone is reached |
| `points.decayed` | Daily decay debits a balance |
| `chore.missed` | A chore ends the day unfinished |
| `chore.fcfs_completed` | A first-come-first-served chore is claimed |
| `auth.admin_passcode.verified` `auth.admin_passcode.failed` `auth.admin_passcode.changed` | Admin passcode activity |
| `auth.profile_pin.verified` `auth.profile_pin.failed` `auth.profile_pin.changed` `auth.profile_pin.cleared` | Profile PIN activity |
