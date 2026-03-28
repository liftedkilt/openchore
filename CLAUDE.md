# OpenChore Project Guidelines

## Development Workflow
- **Migrations:** All schema changes must be accompanied by an Up and Down migration in `migrations/`.
- **Seed Data:** `config/config.example.yaml` defines the development seed data. The server auto-applies `config/config.yaml` on first boot when the DB is empty. `make dev` copies the example if needed, wipes the DB, and starts both servers.
- **Backend:** Go (Standard Library + SQL).
- **Frontend:** React (TypeScript) + Vanilla CSS. Prefer functional components and hooks.

## Technical Rules
- Ensure `CGO_ENABLED=1` for SQLite support in Go.
- Use `internal/model/model.go` as the single source of truth for data structures.
- All points-related changes must record a transaction in `point_transactions`.
- Bonus chore points must not be awarded unless all `required` and `core` chores for the day are complete.
