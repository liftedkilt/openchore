# OpenChore

A family chore management system designed for a wall-mounted iPad. Gamifies household tasks with a points economy, rewards store, streaks, and per-kid theming.

## Features

### Chore Management
- **Chore library** with categories: Required, Core, and Bonus
- **Flexible scheduling**: weekly (pick days), every-N-days interval, or one-off dates
- **Time locks**: chores hidden until a set time (`available_at`)
- **Deadlines**: chores expire after a set time (`due_by`) with configurable penalties
- **Multi-child assignment**: assign the same schedule to multiple kids at once

### Points Economy
- Points earned on chore completion (configurable per chore, with schedule-level multiplier)
- **Required-first rule**: core chore points are held pending until all required chores are done
- **Expiry penalties**: when a deadline passes, configurable per schedule:
  - **Block**: cannot complete at all (breaks streak)
  - **No points**: can complete to keep streak, but earns 0 points
  - **Penalty**: can complete, but deducts a set number of points
- **Points decay**: per-user configurable daily decay if non-bonus chores weren't all completed the previous day
- Full transaction ledger (credit/debit history with reasons)

### Rewards Store
- Admin-created rewards with point costs and optional stock limits
- Per-kid reward visibility and custom pricing
- Redemption history with admin undo capability
- Optimistic UI with toast notifications and haptic feedback

### Streaks & Milestones
- Daily streak tracking (consecutive days with all non-bonus chores completed)
- Admin-configurable streak milestones with bonus point rewards
- Streak display with next-milestone progress

### Webhooks
- Outbound webhooks for key events: `chore.completed`, `chore.uncompleted`, `chore.expired`, `reward.redeemed`, `daily.complete`, `streak.milestone`, `points.decayed`
- HMAC-SHA256 request signing
- Delivery logging with response tracking

### Theming
- 4 built-in themes: Default, Quest, Galaxy, Forest
- Per-kid theme selection with custom category labels, icons, greetings, sounds, and confetti colors
- Synthesized sound effects on completion

### Additional
- **Ambient dashboard**: wall-mounted family overview mode with auto-rotation between kids
- **Admin passcode**: PIN-protected admin panel (no user accounts required). The default passcode is `0000`.
- **Auto-migrations**: database schema managed via embedded SQL migrations

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+, chi/v5 router |
| Database | SQLite3 (WAL mode, foreign keys) |
| Migrations | golang-migrate with embedded SQL |
| Frontend | React 18, TypeScript, Vite |
| Styling | CSS Modules |
| Icons | lucide-react |
| Testing | Go standard library + httptest (69 integration tests) |
| Containers | Multi-stage Alpine builds, Podman/Docker Compose |

## Getting Started

### Prerequisites
- Go 1.22+ with `gcc` (for CGO/sqlite3)
- Node.js 18+ and npm

### Install Dependencies
```bash
make install
```

### Development
```bash
make dev    # Runs API (port 8080) and Vite dev server concurrently
```

Or run them separately:
```bash
make api    # Go API server only
make ui     # Vite dev server only
```

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `DB_PATH` | `openchore.db` | SQLite database file path |
| `TZ` | system | Timezone (important for deadline/time-lock accuracy) |

### Seed Data
```bash
make seed   # Populate database with sample users, chores, and schedules
```

### Running Tests
```bash
make test   # Runs all Go integration tests
```

### Production Build
```bash
make build  # Builds Go binary + Vite production bundle
```

### Container Deployment
```bash
podman-compose up -d    # or docker compose up -d
```

The compose setup runs:
- **api**: Go server with SQLite volume mount
- **web**: Nginx serving the Vite build, proxying `/api` to the API container

## Project Structure

```
.
├── cmd/server/main.go        # Entry point: migrations, background workers, HTTP server
├── internal/
│   ├── api/                  # HTTP handlers and router
│   │   ├── router.go         # Route definitions and middleware
│   │   ├── chores.go         # Chore CRUD, schedules, completions
│   │   ├── points.go         # Points balance, adjustments, decay config
│   │   ├── rewards.go        # Rewards store, redemptions
│   │   └── api_test.go       # Integration test suite (69 tests)
│   ├── model/model.go        # Data types (User, Chore, Schedule, etc.)
│   ├── store/store.go        # SQLite data access layer
│   └── webhook/              # Async webhook dispatcher + background checkers
│       ├── dispatcher.go     # Event firing, HMAC signing, delivery logging
│       ├── expiry.go         # Background expiry checker (1-min interval)
│       └── decay.go          # Background decay checker (15-min interval)
├── migrations/               # Embedded SQL migrations (001-008)
├── web/                      # React frontend
│   └── src/
│       ├── api.ts            # Typed API client
│       ├── types.ts          # TypeScript interfaces
│       ├── pages/
│       │   ├── Dashboard.tsx          # Kid view (daily/weekly/rewards)
│       │   ├── AdminDashboard.tsx     # Admin panel (chores/rewards/points/settings)
│       │   ├── AmbientDashboard.tsx   # Wall-mounted family overview
│       │   └── ProfileSelection.tsx   # User picker
│       └── hooks/            # useIdleRedirect, useThemeSound
├── compose.yaml              # Container orchestration
├── Containerfile             # Multi-stage Go build
├── Makefile                  # Dev/build/test commands
└── seed.go                   # Sample data seeder
```

## API Reference

### Public (no auth)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users` | List all users |
| GET | `/api/users/{id}` | Get user details |
| POST | `/api/admin/verify` | Verify admin passcode |

### Authenticated (`X-User-ID` header)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users/{id}/chores?view=daily&date=YYYY-MM-DD` | Get scheduled chores |
| POST | `/api/schedules/{id}/complete` | Complete a chore |
| DELETE | `/api/schedules/{id}/complete?date=YYYY-MM-DD` | Undo completion |
| GET | `/api/users/{id}/points` | Get point balance + history |
| GET | `/api/users/{id}/streak` | Get streak data |
| GET | `/api/rewards` | List available rewards |
| POST | `/api/rewards/{id}/redeem` | Redeem a reward |
| PUT | `/api/users/{id}/theme` | Update theme preference |
| PUT | `/api/users/{id}/avatar` | Update avatar URL |

### Admin (requires admin role)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Create user |
| PUT | `/api/users/{id}` | Update user |
| DELETE | `/api/users/{id}` | Delete user |
| POST/PUT/DELETE | `/api/chores[/{id}]` | Chore CRUD |
| POST/DELETE | `/api/chores/{id}/schedules[/{sid}]` | Schedule management |
| POST/PUT/DELETE | `/api/rewards[/{id}]` | Reward CRUD |
| PUT | `/api/rewards/{id}/assignments` | Per-kid reward visibility |
| DELETE | `/api/redemptions/{id}` | Undo redemption |
| POST | `/api/points/adjust` | Manual point adjustment |
| GET | `/api/points/balances` | All user balances |
| GET/PUT | `/api/admin/users/{id}/decay` | Decay config per user |
| POST/PUT/DELETE | `/api/admin/webhooks[/{id}]` | Webhook management |
| PUT | `/api/admin/passcode` | Update admin PIN |
| POST/DELETE | `/api/admin/streak-rewards[/{id}]` | Streak milestone management |

## Chore Categories

| Category | Purpose |
|----------|---------|
| **Required** | Must-do chores. Core chore points are held pending until all required chores are completed. |
| **Core** | Standard daily chores. Points earned only after required chores are done. |
| **Bonus** | Optional extra chores for additional points. |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
