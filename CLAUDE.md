# OpenChore

Family chore-tracking PWA: Go API + React/TypeScript frontend, SQLite storage, optional AI photo verification (LiteRT/Gemma) and TTS (Kokoro).

## Tech stack
- **Backend:** Go (stdlib + `chi/v5` router). Build with `CGO_ENABLED=0`.
- **DB:** `modernc.org/sqlite` (pure Go), WAL mode, **single-writer pool (`MaxOpenConns=1`)**.
- **Frontend:** React 18 + TypeScript + Vite. Vanilla CSS / CSS Modules. React Router v7. Functional components and hooks.
- **Migrations:** `golang-migrate` with embedded `iofs`, numbered SQL in `migrations/`.

## Directory map
- `cmd/server/` — entry point; runs migrations, seeds config, starts background workers.
- `internal/model/model.go` — **single source of truth for data structures**.
- `internal/api/` — chi handlers, middleware, auth; integration tests in `api_test.go`.
- `internal/store/` — SQLite DAO (one method per query, `context.Context`-aware).
- `internal/webhook/` — async dispatcher, HMAC-SHA256 signing, expiry/decay checkers.
- `internal/config/` — YAML loader; **seeds only when DB is empty**.
- `internal/ai/`, `internal/aibackend/`, `internal/tts/`, `internal/discord/` — optional integrations.
- `migrations/` — numbered `*_up.sql` / `*_down.sql` pairs.
- `config/config.example.yaml` — dev seed data.
- `web/src/` — `pages/`, `components/`, `hooks/`, `api.ts` (typed client), `types.ts`.
- `e2e/` — Playwright suite (auto-starts API + Vite).
- `litert/`, `compose*.yaml`, `Containerfile` — container/AI sidecars.

## Common commands
- `make dev` — wipes DB, copies example config, runs API (`:8080`) + Vite concurrently.
- `make dev-ai` — same plus LiteRT + Kokoro sidecars.
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
- **Points:** every points change must write a row to `point_transactions`.
- **Bonus chores:** do not award bonus points unless all `required` and `core` chores for the day are complete.
- **Auth:** `X-User-ID` header (and optional `Bearer` token); admin gated by PIN. No sessions. Middleware: `RequireUser`, `RequireUserOrToken`, `RequireAdmin`.
- **Errors:** respond with JSON `{"error": "..."}` via `writeError(w, status, msg)`. Log with stdlib `log.Printf`.
- **Background work:** long-running goroutines are started from `cmd/server/main.go` and must accept a `context.Context` for shutdown.

## Testing
- Go: real DB + `httptest`, no mocks. Add cases alongside `internal/api/api_test.go`.
- E2E: Playwright (Chromium). Two projects — keep the `admin-pin-change` project's dependency ordering intact.
- Frontend unit: Vitest (see `web/src/**/*.test.ts`).

## Gotchas
- SQLite is single-writer — long transactions block everything; keep store methods short.
- `make dev` **wipes the database**. Don't run it against data you want to keep.
- No `golangci-lint`/ESLint config in-repo; rely on `go vet`, `tsc`, and tests.
- CI (`.github/workflows/build.yml`) runs Go + e2e tests before building images to `ghcr.io`.
