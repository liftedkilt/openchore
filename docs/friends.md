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

rounded **down** to a whole number, so 100 always means nothing was missed.
(Any work at all shows as at least 1.)

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
100 × 57 ÷ 59 = 96.6, shown as **96**.

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
| Zero-point chores | In a week that does have points, a chore worth 0 points barely moves the score: missing one takes a full 100 down to 99, and it breaks ties |
| Bonus | Bonus chores, and family / first-come chores the person did, are counted on the side ("+3 bonus") |
| Optional | Optional chores and kids' own tasks count for nothing |
| Penalties and decay | Not part of the score: it looks only at which chores were done |
| Nothing assigned | No score yet; shown as "–" |

### Ranking

1. People with at least **5 assigned chores** that week are ranked; the rest
   are listed under "Getting started". With fewer chores, a single one swings
   the score too far.
2. Higher share first, compared to a hundredth of a point, finer than the
   score shown: 133 ÷ 200 (66.50) ranks just below 2 ÷ 3 (66.66) even though
   both show 66.
3. Then the higher fraction of chores done.
4. Then more bonus chores.
5. Anyone still level shares a place.

### The winner's ribbon

Whoever is ranked first when a week ends wears a ribbon for the whole of the
next week: a rosette by their name on the leaderboard and on their animal's
name tag on the farm. Ties share it. There has to be someone to beat: with
only one ranked person there is no ribbon. "First" means first on your own
leaderboard, friends included; a friend who wins wears the ribbon when you
visit their farm. A week in which nobody scored has no winner. The wall
display and the public farm aren't signed in, so there the ribbon goes to the
best of this household alone.

### What it doesn't fix

- A kid with one chore a day reaches 100 more easily than a kid with eight.
  The chore count next to the score keeps that visible.
- Changing a chore's point value mid-week changes that week's score, because
  the last 7 finished days are recounted at the new value.
- Each server reports its own numbers. There is no cheat detection; the
  parents are the referees.

### Where it lives

Rules: `internal/score`. Finished days are stored in `score_days` and
recounted for a week, which picks up late approvals and rejections.

- **A finished day only counts the chores that existed on it.** A schedule
  created later is left out, so adding a chore on Friday doesn't mark Monday
  to Thursday as missed. (Deleting a chore does remove it from the days still
  being recounted.)
- **Paused days** are stored as paused, so they stay out of the score after
  the person is unpaused. Pausing has no history: a day is treated as paused
  if the person is paused when it's first counted and did nothing that day.

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
out at its next sync, and both sides delete the other family's data. Only the
hub saying the link is gone ends it: a hub that is down, or a proxy in front
of it answering with an error, is just a failed sync, shown on the link and
tried again.

If the hub can't be reached when you unlink, the link and the other family's data are still removed here straight away. The hub address and this side's token are kept (in `friend_unlinks`) and the delete is tried again at every sync, for up to 90 days, so the hub stops handing your last snapshot to the other family once it's back.

### What is shared

Sharing is chosen **per linked family and in both directions**, under that
family in Settings, and both lists start empty:

- **Sent to them:** which of your people you send to that family.
- **Offered by them:** which of the people they send you are shown here.

A person appears on the other family's leaderboard and farm only when both parents have ticked them. Each side can see whether the other has ticked the people it sends. Until the other parent ticks someone you send, only that person's name and colour leave your server: their chore counts, streak and animal are sent only once they're shown there. The receiving side checks too: until you tick someone you're offered, only their name and colour are kept on your server, and anything more is dropped as it arrives. Un-ticking someone removes what was stored about them at once, whether or not the hub can be reached.

A linked family is sent only what its screens show:

| Shared | Never shared |
| --- | --- |
| First name and colour | Chore names and descriptions |
| For this week and last: chores done, chores assigned, bonus count, and the score | A day-by-day breakdown, and the point values the score was worked out from |
| Streak length | Photos |
| Its team's weekly total, and a team goal if it has put one forward | |
| Gifts and playdates its kids sent yours in the last two days | |
| Farm animal: name, kind, coat, accessory, and how it's doing (mood, sick, hiding) | The animal's health, food and joy numbers |
| A custom animal picture, only if you approved it and they chose to show pictures ([custom-animals.md](custom-animals.md)) | A custom picture you haven't approved |
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
signed in on. The wall display shows a one-line leaderboard of **this
household only** (`GET /api/leaderboard/household`, which is public like
chores and points).

### Families as teams, and a goal together

Two linked families are also two teams. The leaderboard shows each family's
score for the week and the two together. A family's team is the people it
shares with the other family *that the other family has chosen to show*, among
those who had chores: a team total never covers someone the other parent
hasn't ticked, since it would give that person's score away. After a tick it
takes a couple of syncs for both sides to count the same people.

Either parent can put forward a **team goal** for the week: the score both
families aim to average *together*, from 1 to 100. It is set under the linked
family in Settings. It counts once the other parent agrees to the same number;
until then each side sees "waiting for them" or "they put forward 85". The
leaderboard then shows progress ("Both families together: 78 of 85") and, when
the week is over, whether it was reached. A goal is for one week; either side
can drop it.

If each parent puts forward a different number, both see the other's and
either can agree to it.

It is a combined score, not a count of chores, so a family with more chores
doesn't carry more of it. Each family works its side out from the people it
shares, and sends only the total, so both homes see the same numbers.

When an agreed team goal is reached, both families' farms (your own farm and
the other family's farm when you visit it) hang celebratory bunting and a
team-goal badge. The decoration stays up for the rest of that week and through
the following week (until a new goal is put forward or another week rolls over)
whenever farm pets and the leaderboard are on. Like the rest of friend-family
data, it is signed-in only and never appears on the public wall display.

### Gifts and playdates

Two pet-care rewards ([pets.md](pets.md)) reach across to a linked family.
Both cost the kid points at the parents' price, and both show up only when
there is a friend's animal to send them to.

| Reward | What happens |
| --- | --- |
| **Treat for a friend's animal** (`gift`) | The kid picks one friend's animal. It gets a treat (a little food, a lot of joy) at the next sync, and that kid sees "Mia sent Biscuit a treat today!". One per friend's animal per day |
| **Playdate** (`playdate`) | The kid picks a linked family. Their animal shows up on that family's farm for the rest of the day, tagged "visiting from …", and comes home happier. One per day; not while it is hiding or too sick to play |

- **Both parents' ticks apply here too.** A gift or a visit only goes between
  people each family has picked: the sender must be shared with that family
  and shown by them.
- **Each arrives once**, however often the families sync. Only something sent
  in the last couple of days is taken, so a family whose server has gone quiet
  can't have an old treat applied again or leave an animal visiting for good.
- **Limits hold on both sides.** One treat per friend's animal per day and one
  playdate per day for the sender (two taps at once can't both get through),
  and an animal takes at most three treats a day from one family, whatever
  the other server says.
- **A parent can turn them off** per family ("Take gifts and playdates from
  …", on by default). The other family's kids are told and aren't charged.
- **Undoing the redemption** (from the points history or the redemption list) takes back a treat or visit that hasn't been picked up yet, along with the points. A gift goes to the hub as soon as it's bought, so undoing it syncs straight away to replace that snapshot. If the other family synced in between, the gift has already arrived and stays.
- **Visiting animals are for signed-in family members.** They aren't on the
  wall display or the public farm.

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
| `HUB_OPEN` | `true` lets anyone create links without a key. Each address (each /64 for IPv6) can then have at most 20 invites waiting to be joined |
| `HUB_TRUST_PROXY` | `true` behind one reverse proxy, so rate limits use the caller's real address. Without it every caller counts as the proxy and shares one limit, which on an open hub lets 20 requests block all new invites for a week; the hub logs a warning when it sees `X-Forwarded-For` without this set |

An OpenChore server takes its hub from Settings, or from `OPENCHORE_HUB_URL` /
`OPENCHORE_HUB_KEY` (which win).

A hub address must be `https://`, with no query or user name in it. Plain
`http://` is only accepted for a hub on this machine or home network. Changing
the address in Settings forgets the saved key, since a key belongs to one hub.

**A hub named by someone else's invite must be on the public internet.** An
invite is text another person wrote, so the server refuses to connect to a
private or local address on its say-so, never follows redirects from a hub,
and reports a failure without network details. The exceptions: the hub this
household has configured itself is always trusted, and
`OPENCHORE_HUB_ALLOW_PRIVATE=true` allows private hubs from invites (for two
households sharing a hub on one home network, or when the hub's public name
resolves to a private address from inside your network and you are the one
joining).

#### How it protects itself

- **Sealed both ways.** What each family stores at the hub is encrypted with
  the invite's key and tied to which side wrote it, so the hub can neither
  read it nor pass a family its own data back as the other family's.
- **Keys and tokens.** Creating a link needs a hub key. Joining needs the
  one-time code in the invite. Everything after that needs the token that
  member was given; a wrong token gets the same answer as a link that doesn't
  exist.
- **Rate limits** per address: 120 requests a minute, and 10 a minute for
  creating or joining links. Behind a proxy (`HUB_TRUST_PROXY`), the address
  is the last one in `X-Forwarded-For`, and only if it is an IP address. An honest server makes one request per link every
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
- `GET /api/friends/farms` – each friend family's farm, shaped like `GET /api/pets`,
  with `goal_celebration` when an agreed team goal with that family was reached.
- `GET /api/friends/visitors` – friends' animals here on a playdate today,
  the treats friends sent today, and `goal_celebrations` for any linked
  families whose agreed team goal was reached.
- `GET` / `PUT /api/admin/friends` – leaderboard switch, family name, hub, and
  each linked family with its `outbound` (your people: `shared`, and `accepted`
  once they show them) and `inbound` (their people: `accepted` = shown here).
- `PUT /api/admin/friends/{id}` (`share_user_ids`, `accept_member_ids`; each
  replaces its list) – who to send to and show from one family. Also
  `show_pictures`, `allow_visits` and `goal_target` (1 to 100; 0 drops it).
- `POST /api/admin/friends/invites`, `POST /api/admin/friends/join` (`invite`),
  `POST /api/admin/friends/{id}/sync`, `DELETE /api/admin/friends/{id}`.

Hub: `POST /v1/links`, `POST /v1/links/{id}/join`, `POST /v1/links/{id}/exchange`,
`DELETE /v1/links/{id}`.

## Not built yet

A history of past winners, and goals that repeat every week.
