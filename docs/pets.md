# Farm pets

An optional feature (off by default): each person can adopt one farm animal
(horse, dog, cat, chicken, cow, pig, sheep or goat). All the animals live on
one shared family farm that everyone sees on their Today screen, so whose
animal is sick or hiding is no secret.

Turn it on in **Manage → Settings → Farm pets** (or `PUT /api/admin/pets`,
or the `set_farm_pets` MCP tool). The first time, a starter set of pet-care
rewards is added to the store: Feed, Treat, Play, and three accessories.

## The one rule

**Chores keep the animal healthy; points only buy food, play and
accessories.** Health can't be bought, so the pet is never a way to pay your
way out of chores.

## How it changes

Each night (the checker runs every 15 minutes and handles each finished day
once, catching up at most 7 days after downtime), for every animal:

| What happened that day | Effect |
| --- | --- |
| Every day | Food −20, Joy −15 |
| All Must-do / Every-day chores done | Health +15 |
| No chores that count that day | Health +5 |
| Chores missed | Health −10 per missed chore (at most −30), and it **hides** |
| Food ran out | Health −10 |
| Person paused | Nothing changes |

"Done" means completed (approved or waiting for a grown-up), excused, or a
first-come-first-served chore a sibling got to first. A rejected try doesn't
count. Bonus and Optional chores never count, same as streaks and penalties.

Mood, shown on the farm:

- **Hiding** – chores were missed yesterday and today's chores that are
  available so far aren't all done. It hides in its shelter by the fence and
  won't come out to play. Catching up brings it out.
- **Sick** (health below 40) / **very sick** (below 15, lies down, too sick to
  play) – recovers only through chores.
- **Hungry** (food below 30), **bored** (joy below 30), **happy**, **content**.

There's no death: at 0 health it's very sick until chores bring it back.

## Pet-care rewards

Ordinary rewards with a `pet_action`, redeemed through the normal reward
store (ledger row, stock, per-person visibility and pricing all apply):

| `pet_action` | Effect |
| --- | --- |
| `feed` | Food +40, Joy +5 |
| `treat` | Food +15, Joy +20 |
| `play` | Joy +35, Food −10 (refused while hiding or very sick) |
| `accessory` | Gives `pet_item` and puts it on (refused if already owned) |

Refusals happen before any points move. Pet-care rewards can't be shared or
saved for, and are hidden while farm pets are off. Owned accessories can be
switched on and off for free (`PATCH /api/me/pet`). Undoing a pet-care
redemption refunds the points but leaves the effect.

## API

- `GET /api/pets` – the farm: `enabled` and every animal with mood and owner.
- `GET /api/me/pet`, `POST /api/me/pet` (adopt: `name`, `species`),
  `PATCH /api/me/pet` (`name`, `accessory`).
- `PUT /api/admin/pets` (`enabled`), `DELETE /api/admin/users/{id}/pet`.

Tunables live in `internal/pet/pet.go`; keep this page in step.
