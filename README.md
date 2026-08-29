<div align="center">

# OpenChore

**Household chores, turned into a game your kids actually check.**

A self-hosted family chore tracker with a points economy, a rewards store,
streaks, and an always-on wall display. One Go binary, one SQLite file, no cloud
account.

[![Build](https://github.com/liftedkilt/openchore/actions/workflows/build.yml/badge.svg)](https://github.com/liftedkilt/openchore/actions/workflows/build.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![PWA](https://img.shields.io/badge/PWA-installable-5A0FC8)](https://web.dev/progressive-web-apps/)

<img src="docs/screenshots/ambient-dashboard.png" alt="OpenChore wall display showing three children's daily progress" width="100%">

</div>

---

## Why OpenChore

Chore charts fall apart because nobody looks at them. OpenChore is built for a
tablet mounted where the family already stands — the kitchen wall — so the chart
looks back.

- **It runs on your hardware.** A single static binary and a SQLite file. No
  subscription, no account, no telemetry.
- **Points are a real economy.** Every credit and debit lands in a transaction
  ledger. Kids spend on rewards you define, at prices you set.
- **The rules do the nagging.** Deadlines, time locks, expiry penalties, and
  daily decay are enforced by the server, not by you at 9pm.
- **Built to be wired up.** Signed outbound webhooks, per-chore trigger URLs,
  API tokens, and a Home Assistant integration.
- **Optional local AI.** Photo proof can be checked by a vision model, and chores
  can be read aloud — both running on your own machine, if you want them at all.

## Quick start

```bash
git clone https://github.com/liftedkilt/openchore.git
cd openchore
cp config/config.example.yaml config/config.yaml   # your family, chores, rewards
docker compose up -d
```

Open **http://localhost:8080** and pick a profile.

> [!IMPORTANT]
> The admin passcode is `0000` on a fresh database, or whatever
> `settings.admin_passcode` says in your config (the example file uses `1234`).
> Change it under **Admin → Settings** before putting this on your network.

`config.yaml` is applied **only when the database is empty**. After first boot,
manage everything from the admin panel — or wipe and re-seed with
`./redeploy.sh --wipe`. Starting with no config at all drops you into a guided
setup wizard instead.

Want the AI extras? They sit behind a compose profile and are off by default:

```bash
docker compose --profile ai up -d    # adds LiteRT (~3.1 GB) + Kokoro TTS (~2 GB)
```

## How the points work

The scheduling model is what makes the economy hold together. Chores fall into
three tiers, and the tiers gate each other:

| Tier | Behavior |
|------|----------|
| **Required** | Non-negotiable. Nothing else pays out until these are done. |
| **Core** | The daily routine. Points are held **pending** until every required chore is complete. |
| **Bonus** | Optional extras. Only awarded once required *and* core are finished. |

That single rule stops the obvious exploit: cherry-picking the fun 15-point
bonus chore and skipping the ones that matter.

Around it sit the other levers:

- **Time locks** — a chore stays hidden until `available_at`, and groups itself
  into morning / afternoon / evening.
- **Deadlines** — past `due_by`, a schedule either **blocks** completion, awards
  **no points**, or applies a **penalty**, your choice per schedule.
- **Decay** — an optional daily debit when the previous day was left unfinished.
- **Streaks** — consecutive days with everything non-bonus done, with milestone
  bonuses you configure.
- **Approval** — chores can require a parent to sign off, with photo proof, before
  points are released.

## A look around

<table>
<tr>
<td width="50%"><img src="docs/screenshots/kid-dashboard.png" alt="A child's daily chore list"></td>
<td width="50%"><img src="docs/screenshots/rewards-store.png" alt="The rewards store"></td>
</tr>
<tr>
<td align="center"><b>Today</b> — grouped by time of day, with pending points and streak progress</td>
<td align="center"><b>Rewards</b> — spend the balance on things you actually control</td>
</tr>
<tr>
<td><img src="docs/screenshots/kid-week.png" alt="Weekly chore view"></td>
<td><img src="docs/screenshots/admin-kids.png" alt="Admin dashboard showing each child's status"></td>
</tr>
<tr>
<td align="center"><b>Week</b> — what's done, what's locked, what's coming</td>
<td align="center"><b>Admin</b> — every kid's day at a glance</td>
</tr>
</table>

## Features

<table>
<tr><td valign="top" width="33%">

**Scheduling**
- Weekly, every-N-days, or one-off
- Time locks and deadlines
- Multi-child assignment
- Family and first-come chores
- Quick-assign for ad-hoc tasks
- Vacation mode

</td><td valign="top" width="33%">

**Economy**
- Transaction ledger
- Rewards store with stock limits
- Per-kid pricing and visibility
- Savings commitments and pools
- Streak milestones
- Configurable decay and penalties

</td><td valign="top" width="33%">

**Household**
- Parent approval queue
- Photo proof via QR handoff
- Discord notifications
- Reports: scorecards, trends, misses
- 4 themes, per-kid personalization
- English and German

</td></tr>
<tr><td valign="top">

**Integrations**
- Outbound webhooks, HMAC-signed
- Delivery log with responses
- Per-chore trigger URLs
- Bearer API tokens
- Home Assistant integration

</td><td valign="top">

**Optional AI** (local)
- Photo verification via Gemma 4 / LiteRT
- Text-to-speech via Kokoro
- Chore description drafting
- Point-value suggestions

</td><td valign="top">

**Accessibility**
- Read-aloud chore cards
- Swipe-to-complete
- 44px minimum tap targets
- Installable PWA, fullscreen
- Ambient wall display mode

</td></tr>
</table>

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | `8080` | API listen port |
| `DB_PATH` | `openchore.db` | SQLite file location |
| `CONFIG_PATH` | `config/config.yaml` | Seed configuration |
| `TZ` | system | **Set this** — deadlines and time locks depend on it |
| `WEB_PORT` | `8080` | Host port for the web container |
| `AI_ENDPOINT` | `http://litert:8080` | Vision backend (LiteRT or Ollama) |
| `TTS_ENDPOINT` | `http://kokoro:8880` | Kokoro TTS service |

## Development

Requires Go 1.25+ and Node 22+.

```bash
make install    # Go modules + npm packages
make dev        # wipes the DB, seeds from config, runs API :8080 + Vite :5173
make test       # Go integration tests against a real SQLite DB
make test-e2e   # Playwright suite, fresh database
make build      # static binary + production bundle
```

`make dev` **deletes the database** on every run — that's how re-seeding works.
Point `DB_PATH` elsewhere if you care about the data.

The stack is Go with `chi` and pure-Go SQLite (`CGO_ENABLED=0`, WAL, single
writer), React 18 + TypeScript + Vite on the front, and `golang-migrate` with
embedded SQL for schema changes. Tests are integration-first: a real database
and `httptest`, no mocks.

## Documentation

- [API reference](docs/api.md) — endpoints, auth, and webhook events
- [Roadmap](ROADMAP.md) — shipped and planned
- [CLAUDE.md](CLAUDE.md) — architecture notes and conventions

## License

[MIT](LICENSE)
