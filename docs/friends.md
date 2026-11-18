# Leaderboard and friend families

Two optional features (both off by default), set up in **Manage → Settings →
Leaderboard & friend families**:

- a **weekly leaderboard** for the household, and
- **friend families**: a link to another family's OpenChore, so the kids
  share one leaderboard and can look at each other's farm ([pets.md](pets.md)).

## The score

Everyone gets one number per week (Monday to Sunday, in the server's time
zone): a score from 0 to 100, shown as "Score 88 (12 of 13 chores)".

### The formula

```
score = 100 × (points of the assigned chores you did)
              ÷ (points of all the chores you were assigned)
```

rounded to the nearest whole number.

- **"Points" here is each chore's own point value**, the number a parent set
  on the chore. It is not the points balance, and nothing is added to or
  taken from anyone's balance by the leaderboard.
- **"Assigned" means Must-do and Every-day chores** (`required`, `core`)
  scheduled for that person that week.
- The whole week is added up first and divided once. It is not an average of
  daily scores, so a day with many chores counts for more than a day with one.

### A worked example

Emma's week has 13 assigned chores:

| Chores | Each worth | Assigned | Done | Points assigned | Points earned |
| --- | --- | --- | --- | --- | --- |
| Make bed | 2 | 7 | 7 | 14 | 14 |
| Feed the cats | 5 | 5 | 5 | 25 | 25 |
| Mow the lawn | 20 | 1 | 0 | 20 | 0 |
| **Total** | | **13** | **12** | **59** | **39** |

Score = 100 × 39 ÷ 59 = 66.1, shown as **Score 66 (12 of 13 chores)**.

She did 12 of 13 chores, but the one she skipped was the biggest, so her score
is 66 and not 92. If she had skipped a "Make bed" instead, it would be
100 × 57 ÷ 59 = **97**.

### Why it works this way

- **A share, not a total, so families can be compared.** Every family sets
  its own point values. One family's chores are worth 2 to 20 points, another's
  20 to 200, and there is no fair exchange rate between them. Dividing by what
  *you* were assigned cancels the scale out: 39 of 59 and 390 of 590 are the
  same score.
- **It also evens out workload.** A kid with 6 chores and a kid with 30 are
  both measured against what their own parents asked of them.
- **Weighted by points, so effort counts.** A plain count would treat making
  the bed and mowing the lawn as equal, and skipping the hard chore would cost
  no more than skipping the easy one. Point values are the parents' own
  statement of how much each chore matters, so they are the weights.
- **Out of 100, with no percent sign.** It reads as a score, and the chore
  count sits next to it so kids can see what it's made from. It's labelled
  "Score" because "points" already means the balance kids spend on rewards.
- **Bonus chores don't raise it.** They're optional, so counting them would
  favour kids whose parents offer more of them. They only break ties.

### The rules in full

| Rule | Detail |
| --- | --- |
| Assigned | Must-do and Every-day chores (`required`, `core`) scheduled for that person individually |
| Worth | The chore's point value. A schedule's points multiplier isn't applied |
| Done | Completed and approved, or waiting for a grown-up. A rejected try isn't done |
| Excused | Left out of both sides of the division: neither assigned nor done |
| Paused days | Left out entirely |
| Today | A chore due today joins both sides when it's done, or joins the assigned side when it expires or the day ends. Nobody is marked down in the morning for chores that aren't due yet |
| No points | If none of a person's assigned chores carry points that week, the score is 100 × chores done ÷ chores assigned |
| Zero-point chores | In a week that does have points, a chore worth 0 points doesn't move the score (it still shows in the chore count) |
| Bonus | Bonus chores, and family / first-come chores the person did, are counted on the side ("+3 bonus") |
| Optional | Optional chores and kids' own tasks count for nothing |
| Penalties and decay | Not part of the score: it looks only at which chores were done |
| Nothing assigned | No score yet; shown as "–" |

### Ranking

1. People with at least **5 assigned chores** that week are ranked; the rest
   are listed under "Getting started". With fewer chores, a single one swings
   the score too far.
2. Higher share first, compared to a hundredth of a point, finer than the
   score shown: 2 ÷ 3 (66.67) ranks just below 67 ÷ 100 (67.00) even though
   both show 67.
3. Then more bonus chores.
4. Anyone still level shares a place.

### What it doesn't fix

- A kid with one chore a day reaches 100 more easily than a kid with eight.
  The chore count next to the score keeps that visible.
- Changing a chore's point value mid-week changes that week's score, because
  the last 7 finished days are recounted at the new value.
- Each server reports its own numbers. There is no cheat detection; the
  parents are the referees.

### Where it lives

Rules: `internal/score`. Finished days are stored in `score_days` (so a paused
day stays out of the score after the person is unpaused) and recounted for a
week, which picks up late approvals and rejections.

## Friend families

### How two homes are linked

The two OpenChore instances never connect to each other, and neither has to
be reachable from the internet. Each one calls a **coordination server** (the
hub) every 15 minutes, leaves an encrypted snapshot there and picks up the
other family's.

1. A parent in one family sets a hub address and key (once), gives the family
   a name, and makes an **invite**.
2. They send the invite to the other parent (text, email, anything).
3. The other parent gives their family a name and pastes the invite under
   **Join with an invite**. They don't need a hub key.
4. Each parent then ticks who to send and who to show (below). Nothing is
   shared just by linking.

The invite works once and runs out after 7 days. It carries the hub address,
the link, a one-time join code, and the key the two families use to encrypt
their snapshots. That key never goes to the hub.

Either parent can unlink. The hub deletes the link; the other family finds
out at its next sync, and both sides delete the other family's data.

### What is shared

Sharing is chosen **per linked family and in both directions**, under that
family in Settings, and both lists start empty:

- **Sent to them:** which of your people you send to that family.
- **Offered by them:** which of the people they send you are shown here.

A person appears on the other family's leaderboard and farm only when both
parents have ticked them. Each side can see whether the other has ticked the
people it sends. Until you tick someone you're offered, only their name and
colour are kept on your server; their chore counts and animal are dropped as
they arrive.

A linked family is sent only what its screens show:

| Shared | Never shared |
| --- | --- |
| First name and colour | Chore names and descriptions |
| For this week and last: chores done, chores assigned, bonus count, and the score | A day-by-day breakdown, and the point values the score was worked out from |
| Streak length | Photos |
| Farm animal: name, kind, coat, accessory, and how it's doing (mood, sick, hiding) | The animal's health, food and joy numbers, and a custom picture |
| | Point balances, rewards, redemptions, ages, PINs, sign-in details |

The score travels as a share out of 10,000 (the score to two more decimal
places), so a family's point scale never leaves the house. Everyone on a
leaderboard, local or from a friend family, is ranked on that same number.
A friend's week is their own Monday to Sunday; near midnight on a Sunday,
families in different time zones can briefly be a week apart, and a friend
whose week doesn't line up yet shows as having nothing counted.

A friend family sees the farm animal as it really is, sick and hiding
included. Drawn (uploaded) accessories aren't sent. Kids can't send each other
anything.

The leaderboard and friends' farms need a signed-in person; unlike the
household's own chores and farm they aren't shown to a device nobody has
signed in on.

### The hub

`cmd/hub` (in the API container image as `openchore-hub`) is a small separate
server with its own SQLite file. It is the only part that has to be reachable
from both homes, so it is the only part you put on the internet.

What it knows: that a link exists, two token hashes, and two encrypted blobs
with their sizes and times. It can't read names, scores or animals, and it has
no web interface or sign-in.

One hub can serve any number of families. Only the family that *makes* an
invite needs the hub's address and key; the family that joins needs nothing
but the invite.

#### Running one

```yaml
services:
  hub:
    image: ghcr.io/liftedkilt/openchore/api:latest
    entrypoint: ["openchore-hub"]
    restart: unless-stopped
    environment:
      HUB_DB: /data/hub.db
      HUB_TRUST_PROXY: "true"   # only behind exactly one reverse proxy
    volumes:
      - hub-data:/data
    ports:
      - "8090:8090"
volumes:
  hub-data:
```

Put it behind HTTPS (any reverse proxy) with **no** sign-in wall in front:
OpenChore servers call it, not people, and they can't log in. Then:

1. On first start the hub makes a key and writes it to `hub.key` next to the
   database. Read it from there (`docker compose exec hub cat /data/hub.key`);
   it is never printed in the log.
2. In OpenChore, under **Settings → Leaderboard & friend families → Invite a
   family**, enter the hub's address and that key.

| Setting | Meaning |
| --- | --- |
| `PORT` | Listen port (default 8090) |
| `HUB_DB` | SQLite file (default `hub.db`) |
| `HUB_KEYS` | Comma-separated keys allowed to create links. If set, no key file is made |
| `HUB_KEY_FILE` | Where the made-on-first-start key lives (default `hub.key` next to `HUB_DB`) |
| `HUB_OPEN` | `true` lets anyone create links without a key |
| `HUB_TRUST_PROXY` | `true` behind one reverse proxy, so rate limits use the caller's real address |

An OpenChore server takes its hub from Settings, or from `OPENCHORE_HUB_URL` /
`OPENCHORE_HUB_KEY` (which win).

#### How it protects itself

- **Keys and tokens.** Creating a link needs a hub key. Joining needs the
  one-time code in the invite. Everything after that needs the token that
  member was given; a wrong token gets the same answer as a link that doesn't
  exist.
- **Rate limits** per address: 120 requests a minute, and 10 a minute for
  creating or joining links. An honest server makes one request per link every
  15 minutes.
- **Size limits.** A snapshot can be at most 256 KB, and the hub holds at most
  10,000 links.
- **Clean-up.** Invites nobody joins are deleted after 7 days, and links
  nobody syncs after 90.
- **Logging.** One line per request: address, method, path and status. Never
  bodies, keys or tokens.

Back up the SQLite file and `hub.key` together. If the hub's data is lost,
linked families see the link as ended and need a new invite.

## API

- `GET /api/leaderboard?week=this|last` – the standings, friends included.
  `percent` is the score out of 100; `share` is the same to two more places
  (0 to 10000) and is what people are ranked on.
- `GET /api/friends/farms` – each friend family's farm, shaped like `GET /api/pets`.
- `GET` / `PUT /api/admin/friends` – leaderboard switch, family name, hub, and
  each linked family with its `outbound` (your people: `shared`, and `accepted`
  once they show them) and `inbound` (their people: `accepted` = shown here).
- `PUT /api/admin/friends/{id}` (`share_user_ids`, `accept_member_ids`; each
  replaces its list) – who to send to and show from one family.
- `POST /api/admin/friends/invites`, `POST /api/admin/friends/join` (`invite`),
  `POST /api/admin/friends/{id}/sync`, `DELETE /api/admin/friends/{id}`.

Hub: `POST /v1/links`, `POST /v1/links/{id}/join`, `POST /v1/links/{id}/exchange`,
`DELETE /v1/links/{id}`.

## Not built yet

Playdates (sending an animal to a friend's farm), gifts, trophies,
family-vs-family and co-op goals, and the leaderboard on the wall display.
