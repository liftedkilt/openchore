# OpenChore Project Guidelines

## Development Workflow
- **Migrations:** All schema changes must be accompanied by an Up and Down migration in `migrations/`.
- **Seed Data:** `seed.go` is used for development (`make dev` or `make seed`). It is **MANDATORY** to update `seed.go` to match any database schema changes (e.g., new required columns, table renames).
- **Backend:** Go (Standard Library + SQL).
- **Frontend:** React (TypeScript) + Vanilla CSS. Prefer functional components and hooks.

## Technical Rules
- Ensure `CGO_ENABLED=1` for SQLite support in Go.
- Use `internal/model/model.go` as the single source of truth for data structures.
- All points-related changes must record a transaction in `point_transactions`.
- Bonus chore points must not be awarded unless all `required` and `core` chores for the day are complete.
