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
- **Optional AI, your choice of model.** A vision model can pre-check photo
  proof for you and write up each week, and chores can be read aloud in a
  recorded voice. Point it at a model on your own machine or a hosted one — or
  leave it off entirely.

## Quick start

```bash
git clone https://github.com/liftedkilt/openchore.git
cd openchore
cp config/config.example.yaml config/config.yaml   # your family, chores, rewards
docker compose up -d
```

Open **http://localhost:8080** and pick a profile.

> [!IMPORTANT]
> Parents sign in to their own profile with a PIN (the example config uses
> `1234` for Alex and `5678` for Jamie) and then tap **Manage**. Change those
> PINs with the key icon on the dashboard before putting this on your network.
> Want Pocket ID, Authelia, Google or another OpenID Connect sign-in? See
> [Signing in](docs/authentication.md).

`config.yaml` is applied **only when the database is empty**. After first boot,
manage everything from the admin panel — or wipe and re-seed with
`./redeploy.sh --wipe`. Starting with no config at all drops you into a guided
setup wizard instead.

Want the AI extras? They are off by default. Point OpenChore at any
OpenAI-compatible model — a local one from the compose profiles below, Ollama,
or a hosted API — see [AI features](docs/ai.md):

```bash
# in .env: AI_BASE_URL=http://llama:8080/v1  AI_MODEL=gemma-4-e4b
#          TTS_BASE_URL=http://kokoro:8880/v1
docker compose --profile ai --profile tts up -d   # Gemma 4 E4B (~6 GB RAM) + Kokoro voices (~2 GB)
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
  Points already committed to a savings goal are not a safe harbour: if the
  spendable balance can't cover the debit, decay reclaims the rest from the
  kid's goals (personal first, then their share of a family pool).
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
<td><img src="docs/screenshots/admin-kids.png" alt="Manage screen showing a child's chores with tappable status circles"></td>
</tr>
<tr>
<td align="center"><b>Week</b> — what's done, what's locked, what's coming</td>
<td align="center"><b>Manage</b> — everyone's day at a glance; tap a circle to tick a chore off for them</td>
</tr>
<tr>
<td><img src="docs/screenshots/parent-sign-in.png" alt="A parent's PIN pad with a Continue with Pocket ID button" width="60%"></td>
<td><img src="docs/screenshots/linked-accounts.png" alt="The linked accounts sheet" width="60%"></td>
</tr>
<tr>
<td align="center"><b>Sign in</b> — tap your profile, then PIN or single sign-on</td>
<td align="center"><b>Linked accounts</b> — Pocket ID, Authelia, Google, any OIDC provider</td>
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
- Parents can take part too
- PINs and OIDC single sign-on
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

**Optional AI** (local or hosted)
- Photo pre-checks for approvals
- Weekly summaries
- Chore description drafting
- Recorded read-aloud voices

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
| `AI_BASE_URL`, `AI_MODEL`, `AI_API_KEY` | — | OpenAI-compatible model for AI features; off when unset. See [AI features](docs/ai.md) |
| `TTS_BASE_URL`, `TTS_MODEL`, `TTS_API_KEY` | — | OpenAI-compatible speech service for read-aloud audio; the browser's voice is used when unset |
| `POINTS_DECAY_INTERVAL` | `15m` | How often the decay worker checks (the e2e suite shortens it) |
| `OPENCHORE_PUBLIC_URL` | request host | External URL used for OIDC redirect URIs |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, … | — | One OIDC provider without editing config; see [Signing in](docs/authentication.md) |
| `OPENCHORE_SESSION_SECRET` | generated | Session signing key (≥32 chars); otherwise generated once and stored in the database |

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

- [Signing in](docs/authentication.md) — PINs, parents, sessions, OIDC providers, upgrading
- [AI features](docs/ai.md) — photo review, summaries, read-aloud voices, choosing a model, upgrading
- [API reference](docs/api.md) — endpoints, auth, and webhook events
- [Roadmap](ROADMAP.md) — shipped and planned
- [CLAUDE.md](CLAUDE.md) — architecture notes and conventions

## License

[MIT](LICENSE)
