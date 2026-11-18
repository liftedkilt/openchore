#!/usr/bin/env python3
"""Make the custom-animal starter kit in web/public/sprites (needs Pillow).

  blank-64.png        an empty, transparent 64x64 canvas to draw on
  guide-layer-64.png  the same size with faint guide marks: put it on a layer
                      under your drawing and hide it before you export
  guide.png           the guide enlarged and labelled, for reading
  sample-duck.png     a finished example

and the same four for a walking animal, a strip of six 64x64 frames (stand,
walk 1, walk 2, sick, lying down, happy):

  blank-strip.png  guide-layer-strip.png  guide-strip.png  sample-duck-strip.png

The layout the app expects (docs/custom-animals.md): the animal faces right,
its feet stand on the bottom edge, and hats, glasses and scarves are placed
at fixed spots, so the head belongs in the upper right.

Run from the repo root:  python3 web/scripts/gen-sprite-kit.py
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).resolve().parent.parent / "public" / "sprites"
SIZE = 64
# Where accessories go, in 64x64 pixels. Keep in step with SPRITE_ANCHORS in
# web/src/pages/kid/farm/FarmAnimal.tsx.
HAT = (46, 8)      # bottom-centre of a hat: the top of the head
EYES = (46, 19)    # centre of glasses
NECK = (39, 32)    # centre of a scarf or bandana
HEAD = (46, 20, 11)  # suggested head: centre x, centre y, radius


def blank():
    return Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))


def guide_layer():
    im = blank()
    d = ImageDraw.Draw(im)
    faint, mark = (80, 120, 200, 70), (220, 80, 60, 150)
    d.line([(0, SIZE - 1), (SIZE - 1, SIZE - 1)], fill=(60, 140, 70, 200))  # ground
    x, y, r = HEAD
    d.ellipse([x - r, y - r, x + r, y + r], outline=faint)                  # head
    d.ellipse([8, 28, 46, 54], outline=faint)                               # body
    for px, py in (HAT, EYES, NECK):
        d.line([(px - 2, py), (px + 2, py)], fill=mark)
        d.line([(px, py - 2), (px, py + 2)], fill=mark)
    return im


def guide_big(layer, scale=10):
    pad_top, pad_side = 70, 190
    w, h = SIZE * scale + pad_side * 2, SIZE * scale + pad_top + 110
    im = Image.new("RGBA", (w, h), (250, 246, 238, 255))
    d = ImageDraw.Draw(im)
    try:
        font = ImageFont.truetype("DejaVuSans.ttf", 18)
        bold = ImageFont.truetype("DejaVuSans-Bold.ttf", 22)
    except OSError:
        font = bold = ImageFont.load_default()
    ox, oy = pad_side, pad_top
    # Checkerboard = transparent.
    for gy in range(0, SIZE, 4):
        for gx in range(0, SIZE, 4):
            c = (255, 255, 255, 255) if (gx // 4 + gy // 4) % 2 == 0 else (236, 236, 236, 255)
            d.rectangle([ox + gx * scale, oy + gy * scale, ox + (gx + 4) * scale - 1, oy + (gy + 4) * scale - 1], fill=c)
    big = layer.resize((SIZE * scale, SIZE * scale), Image.NEAREST)
    im.alpha_composite(big, (ox, oy))
    d = ImageDraw.Draw(im)
    d.rectangle([ox, oy, ox + SIZE * scale, oy + SIZE * scale], outline=(60, 60, 60, 255), width=2)
    ink, red, green = (40, 40, 40, 255), (200, 60, 40, 255), (40, 120, 60, 255)
    d.text((ox, 22), "Custom animal: 64 x 64 pixels, transparent background", font=bold, fill=ink)

    def label(pt, text, side):
        px, py = ox + pt[0] * scale + scale // 2, oy + pt[1] * scale + scale // 2
        tx = ox + SIZE * scale + 14 if side == "right" else 12
        d.line([(px, py), (tx if side == "right" else tx + 160, py)], fill=red, width=2)
        d.text((tx, py - 22 if side == "left" else py - 11), text, font=font, fill=red)

    label(HAT, "hat sits here", "right")
    label(EYES, "glasses here", "right")
    label(NECK, "scarf here", "left")
    gy = oy + SIZE * scale
    d.text((ox, gy + 10), "Feet stand on the bottom edge (the ground).", font=font, fill=green)
    d.text((ox, gy + 38), "Face the animal to the right. The app flips it when it walks left.", font=font, fill=ink)
    d.text((ox, gy + 66), "The circles are only a suggestion for where the head and body go.", font=font, fill=ink)
    # Facing arrow.
    ax, ay = ox + SIZE * scale - 150, oy + 36
    d.line([(ax, ay), (ax + 100, ay)], fill=ink, width=4)
    d.polygon([(ax + 100, ay - 10), (ax + 122, ay), (ax + 100, ay + 10)], fill=ink)
    return im.convert("RGB")


FRAMES = ["stand", "walk 1", "walk 2", "sick", "lying down", "happy"]


def duck(pose="stand"):
    """A duck drawn at 32x32 and doubled, so its pixels are crisp 2x2 blocks.

    pose is one of FRAMES: the same duck with its legs, head, eye and colour
    changed, which is all a frame strip needs.
    """
    im = Image.new("RGBA", (32, 32), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    line = (74, 52, 30, 255)
    body, shade = (255, 221, 102, 255), (240, 186, 64, 255)  # a yellow duck: body, shaded parts
    if pose in ("sick", "lying down"):
        body, shade = (214, 222, 122, 255), (176, 190, 92, 255)  # off-colour
    orange, dark_orange = (244, 150, 40, 255), (206, 112, 24, 255)
    lying = pose == "lying down"
    by = 4 if lying else 0                                     # body drops onto the ground
    hy = {"sick": 2, "lying down": 7}.get(pose, 0)             # head hangs when unwell
    hx = 1 if lying else 0
    # Legs and feet, behind the body: (x of the leg, how far the foot is lifted).
    legs = {"walk 1": [(11, 1), (20, 0)], "walk 2": [(15, 0), (17, 1)], "lying down": []}.get(pose, [(13, 0), (18, 0)])
    for x, lift in legs:
        d.line([(x, 25), (x, 29 - lift)], fill=dark_orange)
        d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=orange)
        d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=dark_orange)
    # Body with a tail flick.
    d.ellipse([4, 14 + by, 23, 27 + by], fill=body, outline=line)
    tail_up = -2 if pose == "happy" else 0
    d.polygon([(4, 19 + by), (1, 14 + by + tail_up), (7, 16 + by)], fill=body, outline=line)
    d.line([(8, 25 + by), (19, 25 + by)], fill=shade)
    d.line([(10, 26 + by), (17, 26 + by)], fill=shade)
    # Wing (lifted when happy).
    wy = -2 if pose == "happy" else 0
    d.ellipse([8, 17 + by + wy, 17, 23 + by + wy], fill=shade, outline=line)
    d.line([(10, 21 + by + wy), (15, 21 + by + wy)], fill=line)
    # Neck and head.
    if not lying:
        d.rectangle([19, 11 + hy, 23, 18], fill=body)
        d.line([(18, 12 + hy), (18, 17)], fill=line)
        d.line([(24, 14 + hy), (24, 17)], fill=line)
    d.ellipse([17 + hx, 4 + hy, 27 + hx, 14 + hy], fill=body, outline=line)
    # Bill (open when happy).
    if pose == "happy":
        d.rectangle([27, 8, 31, 9], fill=orange, outline=dark_orange)
        d.rectangle([27, 11, 30, 12], fill=orange, outline=dark_orange)
    else:
        d.rectangle([27 + hx, 9 + hy, 31, 11 + hy], fill=orange, outline=dark_orange)
        d.point((27 + hx, 10 + hy), fill=orange)
    # Eye: open, half shut when sick, shut when lying down, a smile when happy.
    ex, ey = 23 + hx, 8 + hy
    if pose == "happy":
        d.point((ex - 1, ey + 1), fill=line)
        d.point((ex, ey), fill=line)
        d.point((ex + 1, ey + 1), fill=line)
    elif pose in ("sick", "lying down"):
        d.line([(ex - 1, ey + 1), (ex + 1, ey + 1)], fill=line)
        if pose == "sick":
            d.point((ex + 1, ey), fill=line)
    else:
        d.rectangle([ex, ey, ex + 1, ey + 1], fill=line)
        d.point((ex, ey), fill=(255, 255, 255, 255))
    # Cheek (flushed when well).
    if pose not in ("sick", "lying down"):
        d.point((22, 11), fill=(246, 150, 140, 255))
        d.point((23, 11), fill=(246, 150, 140, 255))
    # A little heart when happy.
    if pose == "happy":
        heart = (236, 86, 110, 255)
        for px, py in [(12, 4), (14, 4), (11, 5), (12, 5), (13, 5), (14, 5), (15, 5), (12, 6), (13, 6), (14, 6), (13, 7)]:
            d.point((px, py), fill=heart)
    return im.resize((SIZE, SIZE), Image.NEAREST)


def strip(frames):
    im = Image.new("RGBA", (SIZE * len(frames), SIZE), (0, 0, 0, 0))
    for i, fr in enumerate(frames):
        im.alpha_composite(fr, (i * SIZE, 0))
    return im


def guide_strip_layer(layer):
    """The single-picture guide repeated six times."""
    return strip([layer] * len(FRAMES))


def guide_strip_big(layer, scale=4):
    n = len(FRAMES)
    pad, top = 24, 70
    w, h = n * SIZE * scale + pad * 2, SIZE * scale + top + 120
    im = Image.new("RGBA", (w, h), (250, 246, 238, 255))
    d = ImageDraw.Draw(im)
    try:
        font = ImageFont.truetype("DejaVuSans.ttf", 17)
        bold = ImageFont.truetype("DejaVuSans-Bold.ttf", 21)
    except OSError:
        font = bold = ImageFont.load_default()
    ink, green = (40, 40, 40, 255), (40, 120, 60, 255)
    d.text((pad, 22), "Walking animal: six 64 x 64 pictures side by side (384 x 64), transparent background", font=bold, fill=ink)
    big = layer.resize((SIZE * scale, SIZE * scale), Image.NEAREST)
    for i, name in enumerate(FRAMES):
        ox = pad + i * SIZE * scale
        for gy in range(0, SIZE, 8):
            for gx in range(0, SIZE, 8):
                c = (255, 255, 255, 255) if (gx // 8 + gy // 8) % 2 == 0 else (236, 236, 236, 255)
                d.rectangle([ox + gx * scale, top + gy * scale, ox + (gx + 8) * scale - 1, top + (gy + 8) * scale - 1], fill=c)
        im.alpha_composite(big, (ox, top))
        d = ImageDraw.Draw(im)
        d.rectangle([ox, top, ox + SIZE * scale, top + SIZE * scale], outline=(60, 60, 60, 255), width=2)
        d.text((ox + 8, top + SIZE * scale + 8), f"{i + 1}. {name}", font=bold, fill=ink)
    y = top + SIZE * scale + 44
    d.text((pad, y), "Every picture: animal facing right, feet on the bottom edge, head in the same place.", font=font, fill=green)
    d.text((pad, y + 26), "You can stop early (after picture 3, 4 or 5), but don't leave a gap in the middle.", font=font, fill=ink)
    d.text((pad, y + 52), "Without a sick or lying-down picture the app tints picture 1 green and squashes it.", font=font, fill=ink)
    return im.convert("RGB")


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    blank().save(OUT / "blank-64.png")
    layer = guide_layer()
    layer.save(OUT / "guide-layer-64.png")
    guide_big(layer).save(OUT / "guide.png", optimize=True)
    duck().save(OUT / "sample-duck.png")
    # The same again as a strip of six frames, for a walking animal.
    Image.new("RGBA", (SIZE * len(FRAMES), SIZE), (0, 0, 0, 0)).save(OUT / "blank-strip.png")
    guide_strip_layer(layer).save(OUT / "guide-layer-strip.png")
    guide_strip_big(layer).save(OUT / "guide-strip.png", optimize=True)
    strip([duck(f) for f in FRAMES]).save(OUT / "sample-duck-strip.png")
    for p in sorted(OUT.glob("*.png")):
        print(p.name, p.stat().st_size, "bytes")
