# Leaderboard and friend families

Two optional features (both off by default), set up in **Manage → Settings →
Leaderboard & friend families**:

- a **weekly leaderboard** for the household, and
- **friend families**: a link to another family's OpenChore, so the kids
  share one leaderboard and can look at each other's farm ([pets.md](pets.md)).

## The score

One number per person per week (Monday to Sunday, in the server's time zone):

**points earned ÷ points assigned × 100**, a score out of 100 shown as
"Score 88 (12 of 13 chores)". No percentages are shown.

Each assigned chore is worth its own point value, so a big chore counts for
more than a small one. It is a share, never a points total: every family sets
its own point values, so totals can't be compared between families, but the
share of your own chores' points can.

| Rule | Detail |
| --- | --- |
| Assigned | Must-do and Every-day chores (`required`, `core`) scheduled for that person |
| Worth | The chore's point value. A schedule's points multiplier isn't applied |
| No points | If none of a person's chores carry points that week, the score is chores done ÷ chores assigned |
| Excused | Left out of both sides of the fraction |
| Paused days | Left out entirely |
| Done | Completed and approved, or waiting for a grown-up. A rejected try isn't done |
| Today | A chore due today joins the fraction when it's done, when it expires, or when the day ends. Nobody is marked down in the morning for chores that aren't due yet |
| Bonus | Bonus chores, and family / first-come chores the person did, are counted on the side ("+3 bonus"). They break ties and never change the score |
| Optional | Optional chores and kids' own tasks count for nothing |
| Penalties and decay | Not part of the score: it looks only at which chores were done |
| Ranking | Share earned, then bonus count; ties share a place. A person needs 5 assigned chores in the week to be ranked |

A kid with one chore a day reaches 100% more easily than a kid with eight.
The chore count is always shown next to the score so that stays visible.
Because chores are weighted, the two can differ: 9 of 10 chores is well under
a score of 90 if the one missed was the big one.

Finished days are stored in `score_days` (so a paused day stays out of the
score after the person is unpaused) and recounted for a week, which picks up
late approvals and rejections. Rules live in `internal/score`.

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

| Shared | Never shared |
| --- | --- |
| First name and colour | Chore names and descriptions |
| Chores done / assigned / bonus per day, and the point value of those chores, last 14 days | Photos |
| Streak length | Point balances, rewards, redemptions |
| Farm animal: name, kind, coat, accessory, mood, health, food, joy | Ages, PINs, sign-in details |

A friend family sees the farm animal as it really is, sick and hiding
included. Drawn (uploaded) accessories aren't sent. Kids can't send each other
anything.

The leaderboard and friends' farms need a signed-in person; unlike the
household's own chores and farm they aren't shown to a device nobody has
signed in on.

### The hub

`cmd/hub` (in the same container image as `openchore-hub`) is a small
separate server with its own SQLite file. It is the only part that has to be
reachable from both homes.

```
PORT       listen port (default 8090)
HUB_DB     SQLite file (default hub.db)
HUB_KEYS   comma-separated keys allowed to create links
HUB_OPEN   "true" to let anyone create links
```

What it knows: that a link exists, two token hashes, and two encrypted blobs
with their sizes and times. It can't read names, scores or animals. Creating
a link needs one of `HUB_KEYS`; joining needs only the invite. Invites nobody
joins are deleted after 7 days, and links nobody syncs after 90.

An instance takes its hub from Settings, or from `OPENCHORE_HUB_URL` /
`OPENCHORE_HUB_KEY` (which win).

## API

- `GET /api/leaderboard?week=this|last` – the standings, friends included.
  `percent` is the score out of 100.
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
