#!/usr/bin/env python3
"""Make the custom-animal starter kit in web/public/sprites (needs Pillow).

  blank-64.png        an empty, transparent 64x64 canvas to draw on
  guide-layer-64.png  the same size with faint guide marks: put it on a layer
                      under your drawing and hide it before you export
  guide.png           the guide enlarged and labelled, for reading
  sample-duck.png     a finished example

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


def duck():
    """A duck drawn at 32x32 and doubled, so its pixels are crisp 2x2 blocks."""
    im = Image.new("RGBA", (32, 32), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    line, white, shade = (74, 52, 30, 255), (255, 221, 102, 255), (240, 186, 64, 255)  # a yellow duck: body, shaded parts
    orange, dark_orange = (244, 150, 40, 255), (206, 112, 24, 255)
    # Feet and legs (drawn first, behind the body).
    for x in (13, 18):
        d.line([(x, 25), (x, 29)], fill=dark_orange)
        d.line([(x - 1, 30), (x + 3, 30)], fill=orange)
        d.line([(x - 1, 31), (x + 3, 31)], fill=dark_orange)
    # Body with a tail flick.
    d.ellipse([4, 14, 23, 27], fill=white, outline=line)
    d.polygon([(4, 19), (1, 14), (7, 16)], fill=white, outline=line)
    d.line([(8, 25), (19, 25)], fill=shade)
    d.line([(10, 26), (17, 26)], fill=shade)
    # Wing.
    d.ellipse([8, 17, 17, 23], fill=shade, outline=line)
    d.line([(10, 21), (15, 21)], fill=line)
    # Neck and head.
    d.rectangle([19, 11, 23, 18], fill=white)
    d.line([(18, 12), (18, 17)], fill=line)
    d.line([(24, 14), (24, 17)], fill=line)
    d.ellipse([17, 4, 27, 14], fill=white, outline=line)
    # Bill.
    d.rectangle([27, 9, 31, 11], fill=orange, outline=dark_orange)
    d.point((27, 10), fill=orange)
    # Eye and cheek.
    d.rectangle([23, 8, 24, 9], fill=line)
    d.point((23, 8), fill=(255, 255, 255, 255))
    d.point((22, 11), fill=(246, 178, 170, 255))
    d.point((23, 11), fill=(246, 178, 170, 255))
    return im.resize((SIZE, SIZE), Image.NEAREST)


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    blank().save(OUT / "blank-64.png")
    layer = guide_layer()
    layer.save(OUT / "guide-layer-64.png")
    guide_big(layer).save(OUT / "guide.png", optimize=True)
    duck().save(OUT / "sample-duck.png")
    for p in sorted(OUT.glob("*.png")):
        print(p.name, p.stat().st_size, "bytes")
