# Farm pets

An optional feature (off by default): each person can adopt one farm animal
(horse, dog, cat, chicken, cow, pig, sheep or goat). All the animals live on
one shared family farm that everyone sees on their Today screen, so whose
animal is sick or hiding is no secret.

Turn it on in **Manage → Settings → Farm pets** (or `PUT /api/admin/pets`,
or the `set_farm_pets` MCP tool). The first time, a starter set of pet-care
rewards is added to the store: Feed, Treat, Play, five accessories, Restyle coat
and New animal.

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

**Today's gain shows straight away.** As soon as every Must-do and Every-day
chore for today is done, the animal is shown with the +15 health that tonight's
check will give it, so a sick animal perks up when the kid finishes rather
than the next morning. It is only a preview: un-ticking a chore or a rejection
takes it away again, and the nightly check is still what records it (nothing
is counted twice). If the food will run out tonight, the preview also takes
off the −10 for that, so it shows +5 until the animal has enough food to last
the night (more than 20). A paused person's animal gets no preview, since
nothing changes for it overnight. A chore that only unlocks later in the day has to be done
too.

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
| `restyle` | New coat colour, picked at redemption (`{"pet": {"color": …}}`) |
| `picture` | The person's own picture is shown in place of the drawn animal, sent at redemption (`{"pet": {"image": …}}`); see [custom-animals.md](custom-animals.md) |
| `gift` | A treat for an animal on a linked friend family's farm, picked at redemption (`{"pet": {"friend": …}}`); see [friends.md](friends.md) |
| `playdate` | The person's animal visits a linked family's farm for the day (`{"pet": {"link": …}}`) and gains Joy +25 |
| `new_animal` | A different animal (and coat), picked at redemption (`{"pet": {"species": …, "color": …}}`). Name, health, food, joy and accessories carry over, so it's never a way out of a sick pet |

Accessories (`pet_item`) are one of 19 built-ins (hats, glasses, neckwear) or a
parent's own emoji accessory written `e:<head|face|neck>:<emoji>`, e.g. `e:head:🎓`.
Each species has four coat colours (picked free when adopting); see
`model.PetColors`. The store and farm sheet preview every pet reward on the
person's own animal, and accessories can be tried on before buying.

Refusals happen before any points move. Pet-care rewards can't be shared or
saved for, and are hidden while farm pets are off. Owned accessories can be
switched on and off for free (`PATCH /api/me/pet`). Undoing a pet-care
redemption refunds the points but leaves the effect.

## Christmas season

From five weeks before Christmas (November 20) through one week after (January 1), the farm scene dresses for winter: snow covers the field and hills, snowy pines line the far hill, the barn gets a snowy roof and strings of lights, and snow falls across the scene (it stays still when the device asks for reduced motion). The season follows the device's own date, needs no setting, and is purely visual. To preview it on any day, add `?season=christmas` to a page URL (`?season=none` forces it off, `?season=auto` returns to the calendar); the choice lasts for that browser tab. The logic lives in `web/src/pages/kid/farm/season.ts`, and the colours come from the `--farm-snow-*` / `--farm-tree*` dials in `web/src/design/themes.css`.

## API

- `GET /api/pets` – the farm: `enabled` and every animal with mood and owner.
- `GET /api/me/pet`, `POST /api/me/pet` (adopt: `name`, `species`),
  `PATCH /api/me/pet` (`name`, `accessory`).
- `PUT /api/admin/pets` (`enabled`), `DELETE /api/admin/users/{id}/pet`
  (also in Settings → Farm pets).
- `POST /api/rewards/{id}/redeem` takes an optional `{"pet": {"species", "color"}}`
  for `new_animal` / `restyle` rewards.

Tunables live in `internal/pet/pet.go`; keep this page in step.
