# Custom animals

A person can swap the drawn farm animal ([pets.md](pets.md)) for a picture of
their own: something they drew, pixel art, anything, made in any art program.
It can be **one picture**, or a **strip of up to six** for an animal that
really walks and has its own sick, lying-down and happy pictures.

## The picture

| | |
| --- | --- |
| File | PNG |
| Size | 64 × 64 pixels is the standard. Anything from 8 to 128 pixels a side is kept as it is; bigger pictures are scaled down to fit 128 |
| Background | See-through (transparent). A picture with no see-through pixels is refused, because the animal would be a square |
| Facing | Right. The app flips it when the animal walks left |
| Feet | On the bottom edge. The bottom of the picture is the ground |
| Head | Upper right, so hats, glasses and scarves land in the right place (below) |

The picture is shown with hard pixel edges, never smoothed, so pixel art stays
crisp. Scaling a big picture down picks pixels without blending for the same
reason: a 640 × 640 export of 64 × 64 pixel art comes back exactly as drawn.

With one picture, the app makes it bob as it walks, tints it green when the
animal is sick (more strongly when it's very sick), and squashes it flat when
it's too sick to stand. A hiding animal is in its shelter, so it needs no
picture.

## A walking animal: the strip

Put the pictures side by side in one PNG, each the same square size, in this
order:

| # | Picture | Shown when |
| --- | --- | --- |
| 1 | Standing | The animal is standing still (and whenever a later picture is missing) |
| 2 | Walk 1 | Walking: pictures 2 and 3 swap back and forth |
| 3 | Walk 2 | |
| 4 | Sick | The animal is sick |
| 5 | Lying down | The animal is very sick |
| 6 | Happy | The animal is happy, or being fed, played with or dressed |

- The standard strip is 384 × 64: six 64 × 64 pictures.
- **How the app tells:** a file whose width is exactly 2 to 6 times its height
  is a strip of that many square pictures. Anything else is one picture. So a
  single picture should be square; pad a wide animal with see-through space.
- **You can stop early.** Three pictures give a standing and walking animal;
  with only two, walking swaps picture 2 with the standing one. Blank pictures
  at the end are dropped. A blank picture in the middle is refused: draw every
  picture up to the last one you use (copy the standing one if you have
  nothing better for a slot).
- **Missing mood pictures fall back.** Without picture 4 a sick animal is the
  standing picture tinted green; without picture 5 a very sick one is tinted
  and squashed; without picture 6 a happy one just stands.
- A sick animal shuffles about without the walk pictures.
- Each picture can be up to 128 pixels a side; a bigger strip is scaled down
  by its height.
- Accessories stay at the same spots on every picture, so keep the head in
  about the same place. They are hidden on the lying-down picture.

### Where accessories go

Accessories are placed at fixed spots, measured in a 64 × 64 picture from the
top-left corner:

| Accessory | Spot (x, y) | Meaning |
| --- | --- | --- |
| Hat | 46, 8 | The top of the head: a hat's brim sits here |
| Glasses | 46, 19 | The middle of the eyes |
| Scarf, bandana, bell | 39, 32 | The neck |

A picture of another size is scaled to the same square first, so the spots
stay in proportion. If your animal's head is somewhere else, accessories will
float; either move the head or go without them.

## Starter pack

OpenChore ships a ready-made pack of eight original pixel-art farm animals in
`web/public/sprites/pack/` (served at `/sprites/pack/<id>-strip.png`). Each is
a full 6-frame `384 × 64` strip (`standing`, `walk 1`, `walk 2`, `sick`,
`lying down`, `happy`), aligned to the accessory spots above and well under the
48 KB friend-sharing limit:

| Animal | Strip file |
| --- | --- |
| Donkey | [`donkey-strip.png`](../web/public/sprites/pack/donkey-strip.png) |
| Duck | [`duck-strip.png`](../web/public/sprites/pack/duck-strip.png) |
| Goose | [`goose-strip.png`](../web/public/sprites/pack/goose-strip.png) |
| Turkey | [`turkey-strip.png`](../web/public/sprites/pack/turkey-strip.png) |
| Rabbit | [`rabbit-strip.png`](../web/public/sprites/pack/rabbit-strip.png) |
| Alpaca | [`alpaca-strip.png`](../web/public/sprites/pack/alpaca-strip.png) |
| Rooster | [`rooster-strip.png`](../web/public/sprites/pack/rooster-strip.png) |
| Mouse | [`mouse-strip.png`](../web/public/sprites/pack/mouse-strip.png) |

Kids can tap any animal directly inside **Use my own picture** to try it on the
grass and redeem with no file download or upload required — or download a strip
to customize it in a pixel-art editor. The strips are generated reproducibly by
`web/scripts/gen-starter-pack.py` (needs Pillow), and all artwork is original
to OpenChore (MIT-licensed).

## Fantasy characters

Seven more ready-made characters are sold separately from the starter pack. Each one is its own pet-care reward, so a kid gets one character per redemption and parents set each price on its own (100 points by default):

| Character | Reward | Picture |
| --- | --- | --- |
| Dragon | 🐉 Dragon | [`dragon.png`](../internal/sprite/characters/dragon.png) |
| Christmas gnome | 🎄 Christmas gnome | [`gnome.png`](../internal/sprite/characters/gnome.png) |
| Hippogriff | 🪶 Hippogriff | [`hippogriff.png`](../internal/sprite/characters/hippogriff.png) |
| Unicorn | 🦄 Unicorn | [`unicorn.png`](../internal/sprite/characters/unicorn.png) |
| Yeti | ❄️ Yeti | [`yeti.png`](../internal/sprite/characters/yeti.png) |
| Phoenix | 🔥 Phoenix | [`phoenix.png`](../internal/sprite/characters/phoenix.png) |
| Griffin | 🦁 Griffin | [`griffin.png`](../internal/sprite/characters/griffin.png) |

- **The reward:** `pet_action` `character` with `pet_item` naming the character (`dragon`, `gnome`, `hippogriff`, `unicorn`, `yeti`, `phoenix` or `griffin`). Its thumbnail shows the character, and redeeming needs no pick. A parent can add more rewards for the same character (a birthday dragon at a lower price, say) in Manage → Rewards.
- **Already that character:** the reward is hidden from that kid, and redeeming it again is refused before any points move, even when two taps arrive at once. Another character, an upload or going back to the drawn animal all replace it.
- **Undo:** undoing a character purchase (from the points history or the redemption list) refunds it and puts back the drawn animal, as long as the animal is still that character. If the kid has bought another picture or character since, the refund goes ahead and the animal is left as it is.
- **Where the pictures live:** in the server, not the web app (`internal/sprite/characters/`, built into the binary and served at `/api/pets/characters/<id>.png`). Redeeming copies the picture into the animal's own upload, so it then works like any other picture. Because it is the app's own art, it goes to friend families without a parent approving it first (a parent can still untick it in Settings → Farm pets).
- **No back door:** "Use my own picture" refuses a picture that copies a character: the whole strip or any one frame, flipped, enlarged, shifted a few pixels, slightly recoloured or with a few pixels changed. Each frame is compared on the 32 × 32 drawing grid, and it counts as a copy when 85% of its solid cells line up with a character's in a close colour (`sprite.Copies`). A kid who draws their own dragon, or repaints one in new colours, is fine. None of the free starter pictures comes closer than about 75%.
- **Existing households:** turning farm pets on adds these rewards with the starter set. A household that already had pets on gets them from migration `031_pet_characters`.
- The strips are generated by `web/scripts/gen-fantasy-pack.py` (needs Pillow), which reuses the starter pack's drawing helpers and checks. The list of characters is `PetCharacters` in `internal/model/model.go`; `PET_CHARACTERS` in `web/src/types.ts` must match.

## Starter kit

The app serves these (and links them from the upload screen); they are in
`web/public/sprites/`:

| File | What it is |
| --- | --- |
| [`blank-64.png`](../web/public/sprites/blank-64.png) | An empty, see-through 64 × 64 canvas to draw on |
| [`guide-layer-64.png`](../web/public/sprites/guide-layer-64.png) | The same size with faint marks for the ground, head, body and the three accessory spots. Put it on a layer under your drawing and hide it before you export |
| [`guide.png`](../web/public/sprites/guide.png) | The guide enlarged and labelled, for reading |
| [`sample-duck.png`](../web/public/sprites/sample-duck.png) | A finished example |
| [`blank-strip.png`](../web/public/sprites/blank-strip.png) | An empty 384 × 64 canvas: six pictures in a row |
| [`guide-layer-strip.png`](../web/public/sprites/guide-layer-strip.png) | The guide marks repeated for all six pictures |
| [`guide-strip.png`](../web/public/sprites/guide-strip.png) | The six slots enlarged and labelled |
| [`sample-duck-strip.png`](../web/public/sprites/sample-duck-strip.png) | The example duck as a full strip: standing, two steps, sick, lying down, happy |

They are made by `web/scripts/gen-sprite-kit.py` (needs Pillow). The spots in
that script and `SPRITE_ANCHORS` in `web/src/pages/kid/farm/FarmAnimal.tsx`
must match.

### Making one

1. Open `blank-64.png` in any art program that keeps transparency (Aseprite,
   Piskel, Procreate, Krita, GIMP, Photoshop…).
2. Add `guide-layer-64.png` as a layer underneath.
3. Draw the animal facing right, feet on the bottom edge, head in the upper
   right. For pixel art, turn off smoothing (anti-aliasing) on your brush.
4. Hide the guide layer and export as PNG with a transparent background.

For a walking animal, do the same on `blank-strip.png` with
`guide-layer-strip.png`: draw the standing picture first, copy it into the
next slots, and change the legs for the two steps. Many pixel-art programs can
export an animation's frames as one row, which is the same thing.

## In the app

- **Getting one:** a reward with `pet_action` `picture` ("Use my own
  picture", added with the starter pet rewards, 60 points). The person either
  picks one of the ready-made starter-pack animals or uploads a PNG file, sees
  it on the grass, and redeems. A picture that's refused costs nothing.
- **Going back:** "Go back to the drawn animal" on the farm sheet is free and
  deletes the picture.
- **Grown-ups** can remove a person's picture in Settings → Farm pets
  (`DELETE /api/admin/users/{id}/pet/picture`, MCP `remove_farm_pet_picture`).
  There is no approval step before a picture appears on the family's own
  farm; approval is for sending it to other families (below).
- The animal keeps its kind, coat, name, health, food, joy and accessories
  underneath. Changing coat colour has no visible effect while a picture is on.
- The night and bold-outline farm skins don't restyle a picture.

## Friend families

A picture goes to a linked family ([friends.md](friends.md)) only when **both
parents** have said so:

1. **The sending parent approves it.** In Settings → Farm pets, tick "OK to
   send to friend families" next to the animal, after looking at the picture.
   A new picture needs approving again, and a tick only counts for the
   picture that was on screen when you ticked it. This never affects your own farm,
   where the picture shows either way.
2. **The receiving parent opts in.** Under the linked family in Settings,
   "Show custom animal pictures from …" is off until they tick it. While it
   is off, pictures from that family aren't kept at all, and their animals
   show as the built-in kind.

A picture that arrives is treated like an upload from a stranger: it is
checked and re-encoded by the same rules before it is kept or shown, and one
that fails is dropped. Only pictures up to 48 KB are sent (pixel art is a
few kilobytes), with at most 150 KB of them per family.

## API

- `POST /api/rewards/{id}/redeem` with `{"pet": {"image": "data:image/png;base64,…"}}`
  for a `picture` reward.
- `POST /api/rewards/{id}/redeem` with no body for a `character` reward.
- `GET /api/pets/characters/<id>.png` is a character's picture strip (public).
- `PATCH /api/me/pet` with `{"sprite": ""}` to go back to the drawn animal.
- `PUT /api/admin/users/{id}/pet/picture` with `{"approved": true}` to allow
  (or stop) sending it to friend families; `DELETE` removes the picture.
- `PUT /api/admin/friends/{id}` with `{"show_pictures": true}` to show a
  linked family's pictures.
- `GET /api/pets` returns `sprite` (an id) and `sprite_frames` (1 to 6) on an
  animal that has a picture; the file is at `/uploads/pets/<id>.png`.
- `GET /api/me/pet` and `GET /api/pets` also return `character` when the picture is a ready-made character.
