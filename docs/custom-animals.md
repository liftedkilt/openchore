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
  picture", added with the starter pet rewards, 60 points). The person picks
  the file, sees it on the grass, and redeems. A picture that's refused costs
  nothing.
- **Going back:** "Go back to the drawn animal" on the farm sheet is free and
  deletes the picture.
- **Grown-ups** can remove a person's picture in Settings → Farm pets
  (`DELETE /api/admin/users/{id}/pet/picture`, MCP `remove_farm_pet_picture`).
  There is no approval step before a picture appears on the family farm.
- The animal keeps its kind, coat, name, health, food, joy and accessories
  underneath. Changing coat colour has no visible effect while a picture is on.
- The night and bold-outline farm skins don't restyle a picture.

## Friend families

A picture is **not** sent to linked families ([friends.md](friends.md)). They
see the built-in animal of the same kind. Sending pictures is planned, with
the sending parent's approval and a per-family switch on the receiving side.

## API

- `POST /api/rewards/{id}/redeem` with `{"pet": {"image": "data:image/png;base64,…"}}`
  for a `picture` reward.
- `PATCH /api/me/pet` with `{"sprite": ""}` to go back to the drawn animal.
- `GET /api/pets` returns `sprite` (an id) and `sprite_frames` (1 to 6) on an
  animal that has a picture; the file is at `/uploads/pets/<id>.png`.
