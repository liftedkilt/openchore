# API Reference

The API is served under `/api` and speaks JSON in both directions. Errors come
back as `{"error": "..."}` with a matching HTTP status.

## Authentication

A request identifies itself in one of three ways:

| Method | How | Used by |
|--------|-----|---------|
| Session | `openchore_session` cookie, or `Authorization: Bearer ocs1.…` | The web app, after signing in to a profile |
| API token | `Authorization: Bearer <token>` | Integrations (Home Assistant, scripts); acts as an admin |
| None | — | Public endpoints only |

Sessions are HMAC-signed and issued by the server: `POST /api/auth/login`
(tap or PIN), an OIDC sign-in, or `POST /api/setup`. The legacy `X-User-ID`
header is **ignored**. Admin endpoints need a session for a profile with the
`admin` role, or an API token. See [authentication.md](authentication.md)
for the full model.

`POST /api/auth/login` takes `{"user_id": 3, "pin": "1234"}` (omit `pin` for
profiles without one) and returns `{"user", "session", "token"}`. On failure
the body includes a `code`:

| Code | Status | Meaning |
|------|--------|---------|
| `incorrect_pin` | 401 | Wrong PIN |
| `locked_out` | 429 | Too many wrong PINs; see `retry_after_seconds` |
| `oidc_required` | 403 | The profile signs in with a linked account; see `providers` |
| `admin_setup_required` | 403 | Parent profile without a credential; resend with `new_pin` (plus `legacy_passcode` when `legacy_passcode: true`) |
| `incorrect_passcode` | 401 | Wrong legacy household passcode |

Uploaded photos are served from `/uploads/*` and generated TTS audio from
`/tts/*`.

---

## Public

No authentication required.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users` | List users (drives the profile selection screen) |
| `GET` | `/api/users/{id}` | Get one user |
| `GET` | `/api/users/{id}/chores?view=daily&date=YYYY-MM-DD` | Scheduled chores, read-only (used by the wall display) |
| `GET` | `/api/users/{id}/points` | Balance plus the transaction ledger |
| `GET` | `/api/users/{id}/streak` | Current streak and next milestone |
| `POST` | `/api/auth/login` | Sign in to a profile (see above) |
| `POST` | `/api/auth/logout` | Clear the session cookie |
| `GET` | `/api/auth/providers` | Configured OIDC providers (`id`, `name`) |
| `GET` | `/api/auth/oidc/{provider}/start?mode=login&user_id=…&return=/path` | Browser redirect to the provider; `user_id` is the tapped profile |
| `GET` | `/api/auth/oidc/{provider}/start?mode=link&return=/path` | Link the provider to the signed-in profile |
| `GET` | `/api/auth/oidc/{provider}/callback` | Provider redirect target; errors come back as `?auth_error=<code>` |
| `POST` | `/api/setup` | First-run setup: `{"parent": {"name", "pin"}, "children", "chores"}`. Only succeeds while no users exist, and signs the parent in |
| `POST` | `/api/hooks/trigger/{uuid}` | Fire a chore trigger; the UUID is the credential |

---

## Authenticated

Requires a session or an API token.

### Session

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/auth/me` | The signed-in user and session (`method`, `expires_at`, `persistent`, `provider`) |
| `POST` | `/api/auth/logout-everywhere` | Revoke every session for the caller |
| `POST` | `/api/auth/upload-link` | `{"schedule_id"}` → a 30-minute token that can only upload a photo and complete that chore (used by the QR code) |
| `GET` | `/api/users/{id}/identities` | Linked accounts (self or admin) |
| `DELETE` | `/api/users/{id}/identities/{identityID}` | Unlink an account (self or admin); revokes that person's sessions |

### Chores

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/schedules/{id}/complete` | Complete a chore; body may set `completion_date`. Only the assignee or an admin may call it. Points go to the assignee, and an admin completing on someone's behalf doesn't need a photo |
| `DELETE` | `/api/schedules/{id}/complete?date=YYYY-MM-DD` | Undo a completion (assignee or admin) |
| `POST` | `/api/upload` | Upload photo proof (multipart) |
| `PUT` | `/api/completions/{id}/photo` | Attach an uploaded photo to a completion |

### Points, streaks, rewards

| Method | Path | Description |
|--------|------|-------------|
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
| `PUT` | `/api/users/{id}/theme` | Set your skin: `sunroom`, `blocks` or `tint` |
| `PUT` | `/api/users/{id}/avatar` | Set avatar URL |
| `PUT` | `/api/users/{id}/line-color` | Set the ambient graph line color |
| `PUT` | `/api/users/{id}/color` | Set your person colour: `coral`, `mint`, `butter`, `sky`, `rose`, `leaf`, `lilac` or `sand` |
| `PUT` | `/api/users/{id}/pin` | Set a profile PIN |
| `DELETE` | `/api/users/{id}/pin` | Clear the profile PIN |

---

## Admin

Requires an authenticated caller with the `admin` role.

### People

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/users` | Create a user; `pin` is required for `role: admin` |
| `PUT` | `/api/users/{id}` | Update a user. Promoting to admin requires an existing credential; the last admin can't be demoted |
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
| `GET` `PUT` | `/api/admin/settings/{key}` | Read / write a setting (secrets such as `session_secret` are not readable) |
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
| `points.decayed` | Daily decay debits a balance (payload reports any points reclaimed from savings goals) |
| `chore.missed` | A chore ends the day unfinished |
| `chore.fcfs_completed` | A first-come-first-served chore is claimed |
| `auth.profile_pin.verified` `auth.profile_pin.failed` `auth.profile_pin.changed` `auth.profile_pin.cleared` | Profile PIN activity, including PIN sign-ins |
| `auth.oidc.login` | Someone signed in with a linked account |
| `auth.identity.linked` `auth.identity.unlinked` | A linked account was added or removed |
| `auth.admin_passcode.verified` `auth.admin_passcode.failed` | The legacy household passcode was used during the one-time upgrade step |
