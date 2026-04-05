# OpenChore

A family chore management system designed for a wall-mounted iPad. Gamifies household tasks with a points economy, rewards store, streaks, approval workflows, and per-kid theming. Installable as a fullscreen PWA on iOS and Android.

## Features

### Chore Management
- **Chore library** with categories: Required, Core, and Bonus
- **Flexible scheduling**: weekly (pick days), every-N-days interval, or one-off dates
- **Time locks**: chores hidden until a set time (`available_at`), grouped by morning/afternoon/evening
- **Deadlines**: chores expire after a set time (`due_by`) with configurable penalties
- **Multi-child assignment**: assign the same schedule to multiple kids at once
- **Family chores**: `family` assignment type — anyone can complete
- **Multi-step creation wizard**: guided chore + schedule setup in one flow
- **Photo proof**: chores can require photo evidence via QR code scan from a second device
- **Chore triggers**: per-chore webhook URLs for external systems (Home Assistant, etc.) with cooldown and default assignee
- **AI photo review**: optional LLM-powered verification of photo proof submissions (Gemma 4 via Ollama)
- **Text-to-speech**: AI-generated audio descriptions of chores via Kokoro TTS

### Approval Workflow
- Chores can require parent approval before points are awarded
- Admin dashboard shows pending completions for review (approve/reject)
- Discord notifications for approval requests and chore events (via webhook URL in settings)

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

### Admin
- **Passcode-protected panel** (PIN-based, no user accounts required; default passcode is `0000`)
- **Reports dashboard**: weekly/monthly/yearly views with charts — kid scorecards, most-missed chores, completion trends, category breakdown, points flow, day-of-week analysis
- **Vacation mode**: pause a kid's chores without deleting schedules
- **Config export**: export current configuration as YAML

### Theming & Personalization
- 4 built-in themes: Default, Quest, Galaxy, Forest
- Per-kid theme selection with custom category labels, icons, greetings, sounds, and confetti colors
- **Line color picker**: kids choose their color on the ambient dashboard race graph
- **Avatar picker**: 12 DiceBear styles with shuffle and pastel backgrounds
- Synthesized sound effects on completion (Web Audio API)

### Quick Assign
- **Floating action button** on admin dashboard for instant one-off chore assignment
- Pick an existing chore or create a new one, select kids, pick today/tomorrow/custom date
- No need to navigate through the full chore wizard for ad-hoc tasks

### PWA Support
- Web app manifest with `standalone` display for fullscreen home screen apps
- Apple mobile web app meta tags for iOS Safari

### AI Features (Optional)
- **AI photo review**: submitted photo proofs are automatically verified by a local LLM (Gemma 4 via Ollama) — checks whether the photo matches the chore description before approval
- **Text-to-speech**: AI-generated chore descriptions read aloud via Kokoro-FastAPI, replacing browser SpeechSynthesis for higher-quality audio

AI services run as optional containers:
```bash
docker compose --profile ai up        # Start with AI services
```

First-time setup — pull the vision model:
```bash
docker exec openchore-ollama ollama pull gemma4:e2b
```

**Resource requirements:** Ollama (gemma4:e2b) needs ~7.3 GB RAM; Kokoro TTS adds ~2 GB. Total AI stack is ~10 GB additional. A VM with 12-14 GB total RAM is recommended when running the full AI stack alongside the OS and OpenChore.

Configure from **Admin --> Settings --> AI Photo Review**.

### Accessibility
- **Text-to-speech**: speaker button on chore cards reads title and description aloud. When Kokoro TTS is enabled, uses AI-generated audio; otherwise falls back to browser SpeechSynthesis API. Defaults on for kids age 7 and under; any user can toggle via header button. Preference persisted per-user.
- **Swipe-to-complete**: swipe right on a chore card to mark it done (touch devices). Visual green "Done!" hint with 100px threshold.
- **Ambient dashboard**: wall-mounted family overview mode with auto-rotation between kids

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25+, chi/v5 router |
| Database | SQLite3 (WAL mode, foreign keys) |
| Migrations | golang-migrate with embedded SQL |
| Frontend | React 18, TypeScript, Vite |
| Styling | CSS Modules |
| Icons | lucide-react |
| Charts | Custom SVG (BarChart, LineChart components) |
| Testing | Go standard library + httptest (267 integration tests) |
| Containers | Multi-stage Alpine builds, Podman/Docker Compose |

## Getting Started

### Prerequisites
- Go 1.25+ with `gcc` (for CGO/sqlite3)
- Node.js 18+ and npm

### Install Dependencies
```bash
make install
```

### Development
```bash
make dev    # Copies config.example.yaml if needed, wipes DB, runs API (port 8080) and Vite dev server
```

Or run them separately:
```bash
make api    # Go API server only
make ui     # Vite dev server only
```

### Seed Data

There is no separate seed command. The server auto-applies `config/config.yaml` on first boot when the database is empty. `make dev` copies `config/config.example.yaml` to `config/config.yaml` if it doesn't exist, wipes the DB, and starts both servers — so seed data is applied automatically.

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `DB_PATH` | `openchore.db` | SQLite database file path |
| `CONFIG_PATH` | `config/config.yaml` | Path to seed/config YAML file |
| `TZ` | system | Timezone (important for deadline/time-lock accuracy) |

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
├── cmd/server/main.go        # Entry point: migrations, config seeding, background workers, HTTP server
├── config/
│   └── config.example.yaml   # Development seed data (users, chores, schedules, rewards, webhooks)
├── internal/
│   ├── api/                  # HTTP handlers and router
│   │   ├── router.go         # Route definitions and middleware
│   │   ├── chores.go         # Chore CRUD, schedules, completions, approvals, photo upload
│   │   ├── triggers.go       # Per-chore trigger webhooks
│   │   ├── points.go         # Points balance, adjustments, decay config
│   │   ├── rewards.go        # Rewards store, redemptions
│   │   ├── reports.go        # Admin analytics/reports
│   │   ├── api_test.go       # Integration test suite
│   │   ├── reports_test.go   # Reports endpoint tests
│   │   └── triggers_test.go  # Trigger endpoint tests
│   ├── ai/                   # AI photo review + TTS description generation
│   ├── discord/              # Discord webhook notifications
│   │   └── notifier.go       # Sends approval requests, chore events to Discord
│   ├── model/model.go        # Data types (User, Chore, Schedule, Trigger, etc.)
│   ├── ollama/               # Ollama API client (Gemma 4 vision model)
│   ├── store/store.go        # SQLite data access layer
│   ├── tts/                  # Kokoro TTS client (text-to-speech via Kokoro-FastAPI)
│   └── webhook/              # Async webhook dispatcher + background checkers
│       ├── dispatcher.go     # Event firing, HMAC signing, delivery logging
│       ├── expiry.go         # Background expiry checker (1-min interval)
│       └── decay.go          # Background decay checker (15-min interval)
├── migrations/               # Embedded SQL migrations (001–004)
├── web/                      # React frontend
│   └── src/
│       ├── api.ts            # Typed API client
│       ├── types.ts          # TypeScript interfaces + theme config
│       ├── pages/
│       │   ├── Dashboard.tsx          # Kid view (daily/weekly/rewards)
│       │   ├── AdminDashboard.tsx     # Admin panel (chores/users/rewards/points/settings)
│       │   ├── AdminPasscode.tsx      # PIN entry screen
│       │   ├── AmbientDashboard.tsx   # Wall-mounted family overview
│       │   ├── PhotoUpload.tsx        # QR-scanned photo upload page
│       │   ├── ProfileSelection.tsx   # User picker
│       │   ├── Reports.tsx            # Admin analytics with charts
│       │   └── SetupWizard.tsx        # First-boot setup flow
│       ├── components/
│       │   ├── Modal/                 # Reusable modal component
│       │   ├── CreateChoreWizard/     # Multi-step chore creation
│       │   ├── EditChoreModal/        # Chore editing with schedules + triggers
│       │   ├── QuickAssign/           # One-off chore assignment FAB + modal
│       │   └── charts/               # BarChart and LineChart (SVG)
│       └── hooks/
│           ├── useIdleRedirect.ts     # Auto-redirect after inactivity
│           ├── useTextToSpeech.ts     # Browser SpeechSynthesis wrapper
│           └── useThemeSound.ts       # Web Audio API completion sounds
├── compose.yaml              # Container orchestration
├── Containerfile             # Multi-stage Go build
└── Makefile                  # Dev/build/test commands
```

## API Reference

### Public (no auth)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users` | List all users |
| GET | `/api/users/{id}` | Get user details |
| POST | `/api/admin/verify` | Verify admin passcode |
| POST | `/api/setup` | Initial setup (only when no users exist) |
| POST | `/api/hooks/trigger/{uuid}` | Fire a chore trigger (UUID is auth) |

### Authenticated (`X-User-ID` header)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users/{id}/chores?view=daily&date=YYYY-MM-DD` | Get scheduled chores |
| POST | `/api/schedules/{id}/complete` | Complete a chore |
| DELETE | `/api/schedules/{id}/complete?date=YYYY-MM-DD` | Undo completion |
| POST | `/api/upload` | Upload photo proof |
| GET | `/api/users/{id}/points` | Get point balance + history |
| GET | `/api/users/{id}/streak` | Get streak data |
| GET | `/api/rewards` | List available rewards |
| POST | `/api/rewards/{id}/redeem` | Redeem a reward |
| GET | `/api/users/{id}/redemptions` | Redemption history |
| PUT | `/api/users/{id}/theme` | Update theme preference |
| PUT | `/api/users/{id}/avatar` | Update avatar URL |
| PUT | `/api/users/{id}/line-color` | Update ambient graph line color |

### Admin (requires admin role)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Create user |
| PUT | `/api/users/{id}` | Update user |
| DELETE | `/api/users/{id}` | Delete user |
| PUT | `/api/users/{id}/pause` | Pause user (vacation mode) |
| PUT | `/api/users/{id}/unpause` | Unpause user |
| GET/POST | `/api/chores[/{id}]` | Chore list/create |
| GET/PUT/DELETE | `/api/chores/{id}` | Chore read/update/delete |
| GET/POST | `/api/chores/{id}/schedules` | Schedule list/create |
| DELETE | `/api/chores/{id}/schedules/{sid}` | Delete schedule |
| GET/POST | `/api/chores/{id}/triggers` | Trigger list/create |
| PUT/DELETE | `/api/triggers/{id}` | Update/delete trigger |
| GET | `/api/completions/pending` | List pending approvals |
| POST | `/api/completions/{id}/approve` | Approve completion |
| POST | `/api/completions/{id}/reject` | Reject completion |
| GET/PUT | `/api/admin/settings/{key}` | Read/write settings |
| PUT | `/api/admin/passcode` | Update admin PIN |
| GET | `/api/points/balances` | All user balances |
| POST | `/api/points/adjust` | Manual point adjustment |
| GET/PUT | `/api/admin/users/{id}/decay` | Decay config per user |
| GET/POST | `/api/rewards[/all]` | Reward list/create |
| PUT/DELETE | `/api/rewards/{id}` | Update/delete reward |
| PUT | `/api/rewards/{id}/assignments` | Per-kid reward visibility |
| DELETE | `/api/redemptions/{id}` | Undo redemption |
| GET/POST/DELETE | `/api/admin/streak-rewards[/{id}]` | Streak milestone CRUD |
| GET | `/api/admin/reports` | Analytics/reports data |
| GET | `/api/admin/export-config` | Export config as YAML |
| GET/POST/PUT/DELETE | `/api/admin/webhooks[/{id}]` | Webhook CRUD |
| GET | `/api/admin/webhooks/{id}/deliveries` | Webhook delivery log |

## Chore Categories

| Category | Purpose |
|----------|---------|
| **Required** | Must-do chores. Core chore points are held pending until all required chores are completed. |
| **Core** | Standard daily chores. Points earned only after required chores are done. |
| **Bonus** | Optional extra chores for additional points. |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
