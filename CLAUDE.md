# OpenChore

Family chore-tracking PWA: Go API + React/TypeScript frontend, SQLite storage, optional AI (any OpenAI-compatible model: photo review notes, weekly summaries, description drafts) and read-aloud audio (any OpenAI-compatible speech API, e.g. Kokoro).

## Tech stack
- **Backend:** Go (stdlib + `chi/v5` router). Build with `CGO_ENABLED=0`.
- **DB:** `modernc.org/sqlite` (pure Go), WAL mode, **single-writer pool (`MaxOpenConns=1`)**.
- **Frontend:** React 18 + TypeScript + Vite. Vanilla CSS / CSS Modules. React Router v7. Functional components and hooks.
- **Migrations:** `golang-migrate` with embedded `iofs`, numbered SQL in `migrations/`.

## Directory map
- `cmd/server/` — entry point; runs migrations, seeds config, starts background workers.
- `cmd/hub/` — the separate friend-families coordination server (`internal/hub`).
- `internal/model/model.go` — **single source of truth for data structures**.
- `internal/api/` — chi handlers, middleware, auth; integration tests in `api_test.go`.
- `internal/store/` — SQLite DAO (one method per query, `context.Context`-aware).
- `internal/webhook/` — async dispatcher, HMAC-SHA256 signing, expiry/decay checkers.
- `internal/config/` — YAML loader; **seeds only when DB is empty** (except `auth:`, read every start).
- `internal/llm/` (OpenAI-compatible chat client), `internal/tts/` (speech client + per-chore audio), `internal/discord/` — optional integrations. Wiring lives in `internal/api/ai.go` and `cmd/server/main.go`.
- `migrations/` — numbered `*_up.sql` / `*_down.sql` pairs.
- `config/config.example.yaml` — dev seed data.
- `web/src/` — `pages/`, `components/`, `hooks/`, `api.ts` (typed client), `types.ts`.
- `e2e/` — Playwright suite (auto-starts API + Vite).
- `compose*.yaml`, `Containerfile` — containers; the optional `ai` (llama.cpp) and `tts` (Kokoro) profiles use upstream images.

## Common commands
- `make dev` — wipes DB, copies example config, runs API (`:8080`) + Vite concurrently.
- `make dev-ai` — same plus llama.cpp + Kokoro in Docker (`AI_BASE_URL`/`TTS_BASE_URL` set for you).
- `make api` / `make ui` — run one side.
- `make test` — Go integration tests (httptest, stdlib).
- `make test-e2e` — Playwright (fresh DB).
- `make test-all` — both.
- `make build` — Go binary + Vite bundle.
- `make install` — Go + npm deps.

## Conventions
- **Models:** add/extend types only in `internal/model/model.go`.
- **Migrations:** every schema change ships an `up` *and* `down` SQL file in `migrations/`.
- **Seed data:** edit `config/config.example.yaml`; the seeder only runs on an empty DB, so `make dev` (which wipes) is the way to re-seed.
- **Points:** every points change must write a row to `point_transactions`. Undo in the history (`POST /api/points/transactions/{id}/undo`, `internal/api/undo.go`) never adds "Undo:" rows: it unchecks the chore, refunds the redemption, or writes a reversal linked by `reverses_id`, and `ListVisiblePointTransactions` hides undone pairs. Never delete a penalty row: the daily checkers look up its idempotency key.
- **Bonus chores:** do not award bonus points unless all `required` and `core` chores for the day are complete.
- **Optional chores:** never count toward the day (streaks, missed penalties, decay, bonus gate) — use `model.CountsTowardDay`; points, if any, pay immediately. Kids add their own via `/api/me/chores` (`internal/api/ownchores.go`): always `optional`, 0 points, `source = "self"`, and only editable by the owner while still 0 points.
- **Farm pets** (optional, `docs/pets.md`): rules in `internal/pet`, daily checker `internal/webhook/pets.go`, API `internal/api/pets.go`. Points never buy health. A person can show their own picture in place of the drawn animal (`pet_action` `picture`, `pets.sprite`, `docs/custom-animals.md`); it goes to a friend family only if a parent approved it (`sprite_approved`) and the receiving parent turned on `show_pictures` for that link, and it is re-checked on arrival (`internal/sprite`). Pet care is an ordinary reward with `pet_action`; check pet rules in `checkPetReward` *before* `RedeemReward` so a refusal costs nothing.
- **Leaderboard & friend families** (optional, `docs/friends.md`): score rules in `internal/score` (the share of assigned chore points earned; never a points total), sync and snapshots in `internal/friends`, API `internal/api/friends.go`. Linked families are also teams with an optional weekly goal (a combined score), last week's winner wears a ribbon, and kids can spend points on a `gift` or `playdate` for a friend's animal (sent in the snapshot, applied once on arrival, between people both parents picked). Instances never call each other: both call the hub (`internal/hub`, `cmd/hub`), which only holds encrypted snapshots. Sharing is per linked family and two-way (`friend_link_shares` = who we send, `friend_link_accepts` = who we show); a person is visible only when both sides picked them, and data for anyone not accepted is dropped on arrival. A snapshot carries only what the other family's screens show (weekly score as a share plus chore counts, streak, the animal's look and mood): never chore names, photos, point values or balances, rewards, daily breakdowns or pet meters; keep it that way.
- **Auth:** server-issued HMAC-signed sessions (`openchore_session` cookie or `Bearer ocs1.…`) from `POST /api/auth/login` (tap/PIN), OIDC (`internal/api/oidc.go`) or setup; API tokens (`Bearer <hex>`) act as admin. `X-User-ID` is **not** trusted. Admin is a role on a profile; every admin needs a PIN or linked identity. Middleware: `RequireSession`, `RequireUserOrToken`, `RequireAdmin`. OIDC providers come from `auth.oidc`/`OIDC_*` (read-only) plus the `oidc_providers` table (Settings UI); `OIDCService.Reload` applies changes without a restart. See `docs/authentication.md`.
- **Parents take part:** don't filter by `role = 'child'` for chores/points/rewards/streaks; role only gates management.
- **Errors:** respond with JSON `{"error": "..."}` via `writeError(w, status, msg)`. Log with stdlib `log.Printf`.
- **AI is advisory:** it never rejects a completion. Photo review only annotates *pending* completions and may auto-approve (through the same `approveCompletion` path as a parent). AI features are off unless a base URL is set, via `AI_BASE_URL`/`AI_MODEL` (or `TTS_BASE_URL`) or Manage → Settings (env wins). Clients are swappable at runtime: fetch them per use from `AIServices` (`internal/api/aiconfig.go`) and handle nil; see `docs/ai.md`.
- **Background work:** long-running goroutines are started from `cmd/server/main.go` and must accept a `context.Context` for shutdown.

## Testing
- Go: real DB + `httptest`, no mocks. Add cases alongside `internal/api/api_test.go`; auth/OIDC cases (with a fake IdP) live in `auth_test.go`. Use `sessionHeaders(id)` / `adminHeaders()` to mint sessions.
- E2E API calls: `authHeaders(userId)` from `e2e/tests/helpers/setup.ts` (signs in via the API); parents sign in with `loginAsAdmin(page)`.
- E2E: Playwright (Chromium). Two projects — keep the `admin-pin-change` project's dependency ordering intact.
- Frontend unit: Vitest (see `web/src/**/*.test.ts`).

## Gotchas
- Never rebuild `chores` / `chore_schedules` with DROP TABLE in a migration: foreign keys are on, so the drop cascades and deletes schedules/completions. To widen a CHECK, edit `sqlite_schema` in place (see `022_optional_category.up.sql`).
- SQLite is single-writer — long transactions block everything; keep store methods short.
- `make dev` **wipes the database**. Don't run it against data you want to keep.
- No `golangci-lint`/ESLint config in-repo; rely on `go vet`, `tsc`, and tests.
- CI (`.github/workflows/build.yml`) runs Go + e2e tests before building images to `ghcr.io`.
