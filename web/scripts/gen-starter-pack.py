#!/usr/bin/env python3
"""Generate the original pixel-art farm animal starter pack in web/public/sprites/pack/.

Creates eight 6-frame transparent 384x64 strips (donkey, duck, goose, turkey,
rabbit, alpaca, rooster, mouse). Each frame is drawn at 32x32 RGBA and scaled
2x with Image.NEAREST for crisp 2x2 pixel-art blocks and hard pixel edges
(alpha is strictly 0 or 255).

Frame order (left to right):
  0: stand   1: walk 1   2: walk 2   3: sick   4: lying down   5: happy

Keep SIZE, FRAMES, and accessory anchors (HAT, EYES, NECK) in step with
web/scripts/gen-sprite-kit.py and SPRITE_ANCHORS in
web/src/pages/kid/farm/FarmAnimal.tsx.

All artwork is 100% original to OpenChore (MIT licensed).

Run from the repo root:  python3 web/scripts/gen-starter-pack.py
"""
from pathlib import Path
from PIL import Image, ImageDraw

OUT = Path(__file__).resolve().parent.parent / "public" / "sprites" / "pack"
SIZE = 64
GRID = 32
FRAMES = ["stand", "walk 1", "walk 2", "sick", "lying down", "happy"]

# Accessory anchors in 64x64 pixels (half these values on the 32x32 drawing grid).
HAT = (46, 8)    # crown of head: (23, 4) on 32x32
EYES = (46, 19)  # eye center:    (23, 9.5) on 32x32
NECK = (39, 32)  # neck center:   (19.5, 16) on 32x32

WHITE = (255, 255, 255, 255)
CHEEK = (246, 150, 140, 255)
HEART = (236, 86, 110, 255)
HEART_HI = (255, 148, 164, 255)


def sick_tint(c, strength=0.42):
    """Shift a healthy coat colour toward a pale olive unwell tone."""
    r, g, b, a = c
    if a == 0:
        return c
    nr = round(r * (1.0 - strength) + 186 * strength)
    ng = round(g * (1.0 - strength) + 204 * strength)
    nb = round(b * (1.0 - strength) + 124 * strength)
    return (nr, ng, nb, a)


def canvas():
    im = Image.new("RGBA", (GRID, GRID), (0, 0, 0, 0))
    return im, ImageDraw.Draw(im)


def px(d, coords, fill):
    for x, y in coords:
        if 1 <= x <= 30 and 0 <= y <= 31:
            d.point((x, y), fill=fill)


def draw_eye(d, ex, ey, pose, line):
    """Standard expressive eye: bright with catchlight, droopy sick, shut lying, ^ happy."""
    if pose == "happy":
        px(d, [(ex - 1, ey + 1), (ex, ey), (ex + 1, ey + 1)], line)
    elif pose == "lying down":
        d.line([(ex - 1, ey + 1), (ex + 1, ey + 1)], fill=line)
    elif pose == "sick":
        d.line([(ex - 1, ey + 1), (ex + 1, ey + 1)], fill=line)
        px(d, [(ex + 1, ey)], line)
    else:
        d.rectangle([ex, ey, ex + 1, ey + 1], fill=line)
        px(d, [(ex, ey)], WHITE)


def draw_cheek(d, cx, cy, pose, color=CHEEK):
    if pose not in ("sick", "lying down"):
        px(d, [(cx, cy), (cx + 1, cy)], color)


def draw_heart(d, cx, cy, pose):
    """Floating pixel heart above the animal's back on the happy pose."""
    if pose != "happy":
        return
    body = [
        (cx - 1, cy), (cx + 1, cy),
        (cx - 2, cy + 1), (cx - 1, cy + 1), (cx, cy + 1), (cx + 1, cy + 1), (cx + 2, cy + 1),
        (cx - 1, cy + 2), (cx, cy + 2), (cx + 1, cy + 2),
        (cx, cy + 3),
    ]
    px(d, body, HEART)
    px(d, [(cx - 1, cy + 1)], HEART_HI)


def upscale(im):
    return im.resize((SIZE, SIZE), Image.NEAREST)


def donkey(pose="stand"):
    """Stocky grey farm donkey with long ears, tufted mane, cream muzzle, dark hooves."""
    im, d = canvas()
    line = (52, 46, 50, 255)
    coat = (156, 152, 162, 255)
    shade = (122, 118, 128, 255)
    belly = (224, 216, 206, 255)
    muzzle = (238, 228, 212, 255)
    ear_in = (206, 194, 194, 255)
    mane = (62, 56, 62, 255)
    if pose in ("sick", "lying down"):
        coat = sick_tint(coat, 0.44)
        shade = sick_tint(shade, 0.44)
        belly = sick_tint(belly, 0.35)
        muzzle = sick_tint(muzzle, 0.35)
        ear_in = sick_tint(ear_in, 0.38)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 6 if lying else 0
    hy = 2 if sick else (7 if lying else 0)
    hx = 0

    # Far legs (behind body)
    if not lying:
        far_legs = {
            "stand": [(9, 0), (17, 0)],
            "walk 1": [(11, 0), (15, 0)],
            "walk 2": [(8, 0), (19, 1)],
            "sick": [(9, 0), (17, 0)],
            "happy": [(9, 0), (17, 0)],
        }[pose]
        for lx, lift in far_legs:
            d.rectangle([lx, 23, lx + 1, 29 - lift], fill=shade, outline=line)
            d.rectangle([lx, 30 - lift, lx + 1, 31 - lift], fill=mane)
            px(d, [(lx, 24), (lx, 25), (lx, 26), (lx, 27), (lx, 28 - lift)], shade)

    # Tail on left
    if lying:
        d.line([(5, 25), (3, 28), (2, 30)], fill=line)
        d.rectangle([1, 29, 3, 31], fill=mane)
    elif happy:
        d.line([(5, 17), (3, 14), (2, 13)], fill=line)
        d.rectangle([1, 12, 3, 14], fill=mane)
    elif sick:
        d.line([(5, 19), (4, 24), (4, 26)], fill=line)
        d.rectangle([3, 26, 5, 28], fill=mane)
    else:
        tail_dx = -1 if pose == "walk 1" else (1 if pose == "walk 2" else 0)
        d.line([(5, 18), (3 + tail_dx, 22), (3 + tail_dx, 24)], fill=line)
        d.rectangle([2 + tail_dx, 24, 4 + tail_dx, 26], fill=mane)

    # Stocky quadruped body
    d.rounded_rectangle([5, 15 + by, 21, 24 + by], radius=3, fill=coat, outline=line)
    # Cream belly & dorsal stripe
    d.line([(8, 23 + by), (18, 23 + by)], fill=belly)
    d.line([(8, 16 + by), (18, 16 + by)], fill=shade)

    # Near legs (in front) or tucked legs when lying
    if lying:
        # Tucked fore and hind legs resting on y=31
        d.rectangle([6, 28, 12, 31], fill=coat, outline=line)
        d.rectangle([15, 28, 21, 31], fill=coat, outline=line)
        d.rectangle([11, 29, 13, 31], fill=mane)
        d.rectangle([20, 29, 22, 31], fill=mane)
    else:
        near_legs = {
            "stand": [(6, 0), (19, 0)],
            "walk 1": [(6, 1), (20, 1)],
            "walk 2": [(11, 1), (16, 0)],
            "sick": [(6, 0), (19, 0)],
            "happy": [(6, 0), (20, 2)],
        }[pose]
        for lx, lift in near_legs:
            d.rectangle([lx, 22, lx + 2, 29 - lift], fill=coat, outline=line)
            d.rectangle([lx, 29 - lift, lx + 2, 31 - lift], fill=mane)
            d.line([(lx + 1, 22), (lx + 1, 28 - lift)], fill=coat)

    # Neck with upright dark mane
    if not lying:
        d.polygon([(16, 16), (22, 16), (23, 10 + hy), (18, 10 + hy)], fill=coat, outline=line)
        # Blend neck into chest cleanly
        d.rectangle([17, 15, 21, 18], fill=coat)
        # Tufted dark mane along back of neck
        for my in range(10 + hy, 16):
            px(d, [(17, my), (16, my)], mane)
    else:
        d.rectangle([18, 17, 22, 22], fill=coat)
        d.line([(17, 17), (17, 21)], fill=mane)

    # Long donkey ears (tapered tips angled back from crown so hat sits cleanly at (23, 4))
    if sick or lying:
        # Droopy ears angled outward/back
        d.polygon([(20, 6 + hy), (13, 6 + hy), (14, 9 + hy), (19, 8 + hy)], fill=coat, outline=line)
        d.line([(15, 7 + hy), (18, 7 + hy)], fill=ear_in)
    else:
        ear_lift = -1 if happy else 0
        # Far tapered ear
        d.polygon([(19, 5 + hy), (15, 1 + ear_lift), (17, 1 + ear_lift), (20, 5 + hy)], fill=shade, outline=line)
        # Near tapered ear (long, graceful donkey ear)
        d.polygon([(19, 6 + hy), (17, 1 + ear_lift), (19, 0 if happy else 1), (21, 5 + hy)], fill=coat, outline=line)
        d.line([(18, 2 + ear_lift), (20, 5 + hy)], fill=ear_in)

    # Head & broad cream muzzle
    d.rounded_rectangle([18 + hx, 4 + hy, 27 + hx, 12 + hy], radius=3, fill=coat, outline=line)
    # Dark forelock tuft behind crown
    px(d, [(19, 4 + hy), (20, 4 + hy), (19, 5 + hy)], mane)
    # Broad cream muzzle
    d.rounded_rectangle([24 + hx, 8 + hy, 30, 13 + hy], radius=2, fill=muzzle, outline=line)
    # Soften boundary between face and muzzle
    d.line([(24 + hx, 9 + hy), (24 + hx, 11 + hy)], fill=muzzle)
    # Nostril & smile
    px(d, [(28, 9 + hy)], line)
    if happy:
        px(d, [(26, 12 + hy), (27, 12 + hy), (28, 11 + hy)], line)
    else:
        px(d, [(26, 12 + hy), (27, 12 + hy)], line)

    draw_eye(d, 22 + hx, 8 + hy, pose, line)
    draw_cheek(d, 21 + hx, 10 + hy, pose)
    draw_heart(d, 11, 5, pose)
    return upscale(im)


def duck(pose="stand"):
    """Plump sunny yellow farm duck with curved wing, flat orange bill and webbed feet."""
    im, d = canvas()
    line = (74, 52, 30, 255)
    body = (255, 222, 102, 255)
    shade = (238, 184, 62, 255)
    hi = (255, 240, 158, 255)
    orange = (244, 150, 40, 255)
    dark_orange = (202, 108, 22, 255)
    if pose in ("sick", "lying down"):
        body = (214, 222, 122, 255)
        shade = (176, 190, 92, 255)
        hi = (228, 234, 150, 255)

    lying = pose == "lying down"
    by = 4 if lying else 0
    hy = {"sick": 2, "lying down": 7}.get(pose, 0)
    hx = 0

    # Webbed orange feet
    legs = {
        "walk 1": [(11, 1), (19, 0)],
        "walk 2": [(14, 0), (18, 1)],
        "lying down": [],
    }.get(pose, [(12, 0), (18, 0)])
    for x, lift in legs:
        d.line([(x, 25), (x, 29 - lift)], fill=dark_orange)
        d.line([(x + 1, 25), (x + 1, 29 - lift)], fill=orange)
        d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=orange)
        d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=dark_orange)

    # Plump body & upturned tail flick (kept strictly within x >= 2)
    d.ellipse([4, 14 + by, 23, 27 + by], fill=body, outline=line)
    tail_up = -2 if pose == "happy" else (1 if pose == "sick" else 0)
    d.polygon([(5, 19 + by), (2, 14 + by + tail_up), (8, 16 + by)], fill=body, outline=line)
    d.line([(5, 17 + by), (7, 17 + by)], fill=body)
    d.line([(8, 25 + by), (19, 25 + by)], fill=shade)
    d.line([(10, 26 + by), (17, 26 + by)], fill=shade)
    d.line([(9, 16 + by), (16, 16 + by)], fill=hi)

    # Wing (lifted on happy)
    wy = -2 if pose == "happy" else 0
    d.ellipse([8, 17 + by + wy, 17, 23 + by + wy], fill=shade, outline=line)
    d.line([(10, 21 + by + wy), (15, 21 + by + wy)], fill=line)

    # Neck and head
    if not lying:
        d.rectangle([18, 11 + hy, 23, 18], fill=body)
        d.line([(18, 12 + hy), (18, 16)], fill=line)
        d.line([(23, 13 + hy), (23, 17)], fill=line)
    d.ellipse([17 + hx, 4 + hy, 26 + hx, 14 + hy], fill=body, outline=line)
    d.line([(19 + hx, 6 + hy), (23 + hx, 6 + hy)], fill=hi)

    # Flat orange bill (strictly x <= 30)
    if pose == "happy":
        d.rectangle([26, 8, 30, 9], fill=orange, outline=dark_orange)
        d.rectangle([26, 11, 29, 12], fill=orange, outline=dark_orange)
        px(d, [(26, 8), (26, 11)], orange)
    else:
        d.rectangle([26 + hx, 9 + hy, 30, 11 + hy], fill=orange, outline=dark_orange)
        px(d, [(26 + hx, 10 + hy), (27 + hx, 10 + hy), (28 + hx, 10 + hy)], orange)

    draw_eye(d, 22 + hx, 8 + hy, pose, line)
    draw_cheek(d, 21 + hx, 11 + hy, pose)
    draw_heart(d, 11, 5, pose)
    return upscale(im)


def goose(pose="stand"):
    """Tall white farm goose with sleek S-curved neck, pale grey wing and bright orange bill."""
    im, d = canvas()
    line = (78, 84, 96, 255)
    body = (248, 246, 240, 255)
    shade = (214, 218, 226, 255)
    wing = (188, 196, 208, 255)
    orange = (242, 128, 36, 255)
    dark_orange = (192, 88, 18, 255)
    if pose in ("sick", "lying down"):
        body = sick_tint(body, 0.36)
        shade = sick_tint(shade, 0.42)
        wing = sick_tint(wing, 0.44)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 5 if lying else 0
    hy = 2 if sick else (8 if lying else 0)

    # Taller slender orange legs & webbed feet
    legs = {
        "walk 1": [(10, 1), (16, 0)],
        "walk 2": [(12, 0), (17, 1)],
        "lying down": [],
    }.get(pose, [(11, 0), (15, 0)])
    for x, lift in legs:
        d.line([(x, 25), (x, 29 - lift)], fill=dark_orange)
        d.line([(x + 1, 25), (x + 1, 29 - lift)], fill=orange)
        d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=orange)
        d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=dark_orange)

    # Sleek horizontal oval body & pointed tail
    d.ellipse([4, 17 + by, 21, 26 + by], fill=body, outline=line)
    tail_y = -2 if happy else (1 if sick else 0)
    d.polygon([(5, 20 + by), (1, 16 + by + tail_y), (8, 18 + by)], fill=body, outline=line)
    d.line([(5, 19 + by), (7, 19 + by)], fill=body)
    d.line([(7, 25 + by), (17, 25 + by)], fill=shade)

    # Folded grey wing feathers (raised on happy)
    wy = -2 if happy else 0
    d.ellipse([7, 19 + by + wy, 17, 24 + by + wy], fill=wing, outline=line)
    d.line([(9, 21 + by + wy), (14, 21 + by + wy)], fill=shade)
    d.line([(9, 22 + by + wy), (15, 22 + by + wy)], fill=line)

    # Distinctive long slender S-curved upright neck
    if not lying:
        # Neck column narrower than duck's, curving forward from chest to head
        d.polygon([
            (17, 19), (21, 19), (22, 13 + hy), (22, 9 + hy),
            (19, 9 + hy), (18, 13 + hy),
        ], fill=body, outline=line)
        # Blend chest seam
        d.rectangle([17, 18, 20, 21], fill=body)
        d.line([(18, 12 + hy), (18, 17)], fill=shade)
    else:
        # Curved neck folded low when resting
        d.polygon([(17, 24), (22, 24), (23, 18), (19, 18)], fill=body, outline=line)
        d.rectangle([18, 22, 21, 25], fill=body)

    # Sleek head
    d.ellipse([18, 4 + hy, 26, 11 + hy], fill=body, outline=line)
    # Blend neck top into head
    d.line([(19, 11 + hy), (22, 11 + hy)], fill=body)

    # Orange bill with small forehead knob (honking open when happy)
    px(d, [(25, 7 + hy), (26, 7 + hy)], dark_orange)
    if happy:
        d.polygon([(26, 7), (30, 7), (30, 8), (26, 9)], fill=orange, outline=dark_orange)
        d.polygon([(26, 10), (29, 11), (26, 11)], fill=orange, outline=dark_orange)
    else:
        d.rectangle([26, 8 + hy, 30, 10 + hy], fill=orange, outline=dark_orange)
        px(d, [(26, 9 + hy), (27, 9 + hy), (28, 9 + hy)], orange)

    draw_eye(d, 22, 8 + hy, pose, line)
    draw_cheek(d, 21, 10 + hy, pose)
    draw_heart(d, 11, 6, pose)
    return upscale(im)


def turkey(pose="stand"):
    """Plump bronze farm turkey with huge banded fanned tail, red wattle and snood."""
    im, d = canvas()
    line = (52, 32, 20, 255)
    body = (128, 80, 48, 255)
    dark = (80, 48, 30, 255)
    gold = (208, 156, 84, 255)
    cream = (244, 230, 200, 255)
    skin = (132, 150, 186, 255)
    red = (218, 52, 60, 255)
    dark_red = (158, 30, 38, 255)
    beak = (228, 182, 104, 255)
    leg_c = (188, 134, 72, 255)
    leg_d = (138, 92, 44, 255)
    if pose in ("sick", "lying down"):
        body = sick_tint(body, 0.45)
        dark = sick_tint(dark, 0.45)
        gold = sick_tint(gold, 0.45)
        cream = sick_tint(cream, 0.35)
        skin = sick_tint(skin, 0.40)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 5 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Sturdy bipedal legs
    legs = {
        "walk 1": [(12, 1), (18, 0)],
        "walk 2": [(14, 0), (19, 1)],
        "lying down": [],
    }.get(pose, [(13, 0), (18, 0)])
    for x, lift in legs:
        d.line([(x, 25), (x, 29 - lift)], fill=leg_d)
        d.line([(x + 1, 25), (x + 1, 29 - lift)], fill=leg_c)
        d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=leg_c)
        d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=leg_d)

    # Large semicircular fanned tail behind body!
    if lying:
        # Lowered folded fan resting behind body
        d.ellipse([1, 17, 14, 29], fill=cream, outline=line)
        d.ellipse([3, 19, 14, 28], fill=dark)
        d.ellipse([5, 21, 14, 27], fill=gold)
    elif sick:
        # Slightly drooping fan
        d.ellipse([2, 11, 15, 24], fill=cream, outline=line)
        d.ellipse([4, 13, 15, 23], fill=dark)
        d.ellipse([6, 15, 15, 22], fill=gold)
    else:
        # Full proud semicircular fan (extra wide on happy)
        top_y = 7 if happy else 8
        d.ellipse([1, top_y, 16, 23], fill=cream, outline=line)
        d.ellipse([3, top_y + 2, 16, 22], fill=dark)
        d.ellipse([5, top_y + 4, 16, 21], fill=gold)
        d.ellipse([7, top_y + 6, 16, 20], fill=body)
        # Feather ribs for crisp fanned texture
        for fy in (top_y + 4, top_y + 8, top_y + 12):
            d.line([(3, fy), (10, 16)], fill=line)

    # Plump feathered body
    d.ellipse([9, 15 + by, 23, 26 + by], fill=body, outline=line)
    # Barred bronze/gold wing
    wy = -2 if happy else 0
    d.ellipse([11, 18 + by + wy, 20, 24 + by + wy], fill=dark, outline=line)
    d.line([(13, 20 + by + wy), (18, 20 + by + wy)], fill=gold)
    d.line([(13, 22 + by + wy), (18, 22 + by + wy)], fill=cream)

    # Neck (bluish skin with bright red wattle)
    if not lying:
        d.rectangle([19, 10 + hy, 23, 18], fill=skin)
        d.line([(19, 11 + hy), (19, 16)], fill=line)
        d.line([(23, 12 + hy), (23, 16)], fill=line)
    # Head
    d.ellipse([19, 4 + hy, 26, 12 + hy], fill=skin, outline=line)
    # Red caruncle/crown bumps (stay subtle around y=4 so hat fits nicely)
    px(d, [(21, 5 + hy), (22, 5 + hy), (24, 5 + hy)], red)

    # Hanging crimson wattle under chin & neck
    d.rectangle([23, 11 + hy, 25, 15 + hy], fill=red, outline=dark_red)
    px(d, [(24, 12 + hy), (24, 13 + hy), (24, 14 + hy)], red)

    # Short tan beak + red snood draped over beak
    if happy:
        d.polygon([(26, 8), (30, 7), (26, 9)], fill=beak, outline=beak)
        d.line([(26, 7), (30, 7)], fill=line)
        d.polygon([(26, 10), (29, 11), (26, 11)], fill=beak, outline=beak)
        d.line([(26, 11), (29, 11)], fill=line)
    else:
        d.polygon([(26, 8 + hy), (30, 9 + hy), (26, 11 + hy)], fill=beak, outline=line)
        px(d, [(27, 9 + hy)], beak)
    # Crimson snood draping over top of beak
    d.line([(26, 7 + hy), (28, 8 + hy), (28, 10 + hy)], fill=red)
    px(d, [(27, 7 + hy)], dark_red)

    draw_eye(d, 22, 8 + hy, pose, line)
    draw_heart(d, 13, 2, pose)
    return upscale(im)


def rabbit(pose="stand"):
    """Fawn-brown farm rabbit with tall upright pink-lined ears and fluffy white cottontail."""
    im, d = canvas()
    line = (84, 58, 38, 255)
    coat = (206, 162, 118, 255)
    shade = (172, 128, 86, 255)
    belly = (250, 245, 238, 255)
    pink = (242, 162, 174, 255)
    if pose in ("sick", "lying down"):
        coat = sick_tint(coat, 0.42)
        shade = sick_tint(shade, 0.42)
        belly = sick_tint(belly, 0.30)
        pink = sick_tint(pink, 0.40)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 3 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Fluffy white cottontail on left
    tail_y = -1 if happy else 0
    d.ellipse([2, 21 + by + tail_y, 7, 26 + by + tail_y], fill=belly, outline=line)
    px(d, [(4, 23 + by + tail_y), (5, 23 + by + tail_y)], belly)

    # Rounded plump body with arched haunch
    d.ellipse([5, 16 + by, 21, 28 + by], fill=coat, outline=line)
    d.line([(10, 26 + by), (18, 26 + by)], fill=belly)

    # Hind haunch + long hind foot + connected front foreleg & paws
    if lying:
        # Bunny loaf pose resting snugly on y=31
        d.ellipse([6, 22, 16, 31], fill=shade, outline=line)
        d.line([(7, 31), (22, 31)], fill=line)
        d.rectangle([18, 29, 23, 31], fill=coat, outline=line)
        px(d, [(21, 30), (22, 30)], belly)
        d.line([(8, 29), (14, 29)], fill=coat)
    elif pose == "walk 1":
        # Stride A: hind foot planted on y=31, front leg reaching forward & lifted
        d.ellipse([6, 20, 14, 28], fill=shade, outline=line)
        d.rounded_rectangle([6, 29, 13, 31], radius=1, fill=coat, outline=line)
        d.polygon([(18, 24), (21, 24), (23, 28), (19, 28)], fill=coat, outline=line)
        px(d, [(21, 27), (22, 27)], belly)
    elif pose == "walk 2":
        # Stride B: front leg planted on y=31, hind foot lifted slightly
        d.ellipse([7, 19, 15, 27], fill=shade, outline=line)
        d.rounded_rectangle([7, 27, 14, 29], radius=1, fill=coat, outline=line)
        d.rectangle([18, 25, 21, 31], fill=coat, outline=line)
        d.rectangle([19, 29, 23, 31], fill=coat, outline=line)
        d.line([(19, 25), (20, 29)], fill=coat)
        px(d, [(21, 30), (22, 30)], belly)
    elif happy:
        # Joyful perk: hind foot grounded on y=31, front paws lifted happily from chest
        d.ellipse([6, 20, 14, 28], fill=shade, outline=line)
        d.rounded_rectangle([7, 29, 14, 31], radius=1, fill=coat, outline=line)
        d.rounded_rectangle([18, 23, 24, 26], radius=1, fill=coat, outline=line)
        px(d, [(22, 24), (23, 24), (22, 25)], belly)
    else:
        # Stand / sick: grounded hind foot & connected front leg on y=31
        d.ellipse([6, 21, 14, 29], fill=shade, outline=line)
        d.rounded_rectangle([7, 29, 14, 31], radius=1, fill=coat, outline=line)
        d.rectangle([18, 25, 21, 31], fill=coat, outline=line)
        d.rectangle([19, 29, 22, 31], fill=coat, outline=line)
        d.line([(19, 25), (20, 29)], fill=coat)
        px(d, [(20, 30), (21, 30)], belly)

    # Neck bridge
    if not lying:
        d.rectangle([17, 13 + hy, 22, 19], fill=coat)
        d.line([(22, 14 + hy), (22, 19)], fill=line)

    # Tall iconic rabbit ears (positioned at x=16..22 so crown at (23,4) stays open for hats)
    if lying:
        # Ears folded flat back along the spine
        d.rounded_rectangle([11, 12, 21, 15], radius=1, fill=coat, outline=line)
        d.line([(13, 13), (19, 13)], fill=pink)
    elif sick:
        # Droopy lop-tilted ears
        d.polygon([(19, 7 + hy), (12, 9 + hy), (13, 12 + hy), (19, 10 + hy)], fill=coat, outline=line)
        d.line([(14, 10 + hy), (18, 9 + hy)], fill=pink)
    else:
        spread = -1 if happy else 0
        # Far ear
        d.rounded_rectangle([17 + spread, 1, 20 + spread, 8], radius=1, fill=shade, outline=line)
        # Near tall upright ear with pink lining
        d.rounded_rectangle([19, 0, 22, 8], radius=1, fill=coat, outline=line)
        d.line([(20, 2), (20, 6)], fill=pink)
        d.line([(21, 2), (21, 6)], fill=pink)

    # Rounded head & fluffy cheek
    d.ellipse([18, 5 + hy, 28, 15 + hy], fill=coat, outline=line)
    # Soft cream chin/muzzle highlight
    d.line([(23, 13 + hy), (26, 13 + hy)], fill=belly)
    # Pink twitchy nose
    px(d, [(27, 10 + hy), (28, 10 + hy), (27, 11 + hy)], pink)
    # Subtle whisker accents (stay <= 30)
    px(d, [(29, 10 + hy), (29, 12 + hy)], line)

    draw_eye(d, 23, 8 + hy, pose, line)
    draw_cheek(d, 22, 11 + hy, pose)
    draw_heart(d, 10, 6, pose)
    return upscale(im)


def alpaca(pose="stand"):
    """Fluffy cream-oatmeal alpaca with tall woolly neck, scalloped fleece and dainty face."""
    im, d = canvas()
    line = (88, 66, 46, 255)
    fleece = (244, 230, 202, 255)
    shade = (216, 188, 148, 255)
    face = (182, 148, 112, 255)
    hoof = (92, 68, 48, 255)
    if pose in ("sick", "lying down"):
        fleece = sick_tint(fleece, 0.38)
        shade = sick_tint(shade, 0.42)
        face = sick_tint(face, 0.40)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 6 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Far slender legs (dainty cream/tan with crisp shadow)
    if not lying:
        far_legs = {
            "stand": [(10, 0), (16, 0)],
            "walk 1": [(11, 0), (15, 0)],
            "walk 2": [(8, 0), (18, 1)],
            "sick": [(10, 0), (16, 0)],
            "happy": [(10, 0), (16, 0)],
        }[pose]
        for lx, lift in far_legs:
            d.line([(lx, 24), (lx, 29 - lift)], fill=line)
            d.line([(lx + 1, 24), (lx + 1, 29 - lift)], fill=shade)
            d.rectangle([lx, 30 - lift, lx + 1, 31 - lift], fill=hoof)

    # Bobbed fluffy tail on left
    ty = -1 if happy else (1 if sick else 0)
    d.ellipse([2, 17 + by + ty, 6, 21 + by + ty], fill=fleece, outline=line)

    # Scalloped fluffy woolly body (overlapping puffs)
    puffs = [
        (5, 16 + by, 12, 24 + by),
        (9, 15 + by, 17, 24 + by),
        (14, 16 + by, 21, 24 + by),
    ]
    for box in puffs:
        d.ellipse(box, fill=fleece, outline=line)
    # Fill interior cleanly so only outer scalloped silhouette has outlines
    d.rectangle([7, 17 + by, 19, 23 + by], fill=fleece)
    # Wool texture shading
    d.line([(7, 23 + by), (18, 23 + by)], fill=shade)
    px(d, [(8, 19 + by), (9, 20 + by), (13, 18 + by), (14, 19 + by), (17, 20 + by)], shade)

    # Near slender legs or tucked alpaca cush pose
    if lying:
        d.rounded_rectangle([6, 28, 12, 31], radius=1, fill=fleece, outline=line)
        d.rounded_rectangle([15, 28, 21, 31], radius=1, fill=fleece, outline=line)
        d.rectangle([11, 29, 12, 31], fill=hoof)
        d.rectangle([20, 29, 21, 31], fill=hoof)
    else:
        near_legs = {
            "stand": [(7, 0), (19, 0)],
            "walk 1": [(6, 1), (20, 1)],
            "walk 2": [(11, 1), (17, 0)],
            "sick": [(7, 0), (19, 0)],
            "happy": [(7, 0), (20, 2)],
        }[pose]
        for lx, lift in near_legs:
            d.line([(lx, 23), (lx, 29 - lift)], fill=line)
            d.line([(lx + 1, 23), (lx + 1, 29 - lift)], fill=fleece)
            d.line([(lx + 2, 24), (lx + 2, 29 - lift)], fill=line)
            d.rectangle([lx, 30 - lift, lx + 2, 31 - lift], fill=hoof)

    # Tall woolly upright neck (iconic alpaca trait!)
    if not lying:
        d.rounded_rectangle([17, 10 + hy, 22, 19], radius=2, fill=fleece, outline=line)
        d.rectangle([18, 15, 21, 20], fill=fleece)
        px(d, [(18, 13 + hy), (19, 15 + hy)], shade)
    else:
        d.rounded_rectangle([17, 16, 22, 24], radius=2, fill=fleece, outline=line)
        d.rectangle([18, 20, 21, 24], fill=fleece)

    # Short pointed alpaca banana-ears (swept back when sick/lying)
    if sick or lying:
        d.polygon([(19, 6 + hy), (14, 6 + hy), (15, 8 + hy), (19, 8 + hy)], fill=fleece, outline=line)
    else:
        el = -1 if happy else 0
        d.polygon([(18, 5 + hy), (17, 1 + el), (19, 1 + el), (20, 5 + hy)], fill=shade, outline=line)
        d.polygon([(19, 5 + hy), (19, 1 + el), (21, 1 + el), (22, 5 + hy)], fill=fleece, outline=line)
        px(d, [(20, 2 + el), (20, 3 + el)], shade)

    # Fluffy head & topknot + dainty warm-tan face
    d.ellipse([18, 4 + hy, 26, 12 + hy], fill=fleece, outline=line)
    # Fluffy topknot puff around crown
    d.ellipse([20, 3 + hy, 25, 6 + hy], fill=fleece, outline=line)
    d.line([(20, 6 + hy), (24, 6 + hy)], fill=fleece)
    # Dainty tan snout/face extending right
    d.rounded_rectangle([22, 6 + hy, 29, 12 + hy], radius=2, fill=face, outline=line)
    d.line([(22, 7 + hy), (22, 10 + hy)], fill=face)
    # Dark cute nose & smile
    px(d, [(28, 9 + hy), (29, 9 + hy)], hoof)
    if happy:
        px(d, [(26, 11 + hy), (27, 11 + hy), (28, 10 + hy)], line)
    else:
        px(d, [(27, 11 + hy), (28, 11 + hy)], line)

    draw_eye(d, 23, 8 + hy, pose, line)
    draw_cheek(d, 22, 10 + hy, pose)
    draw_heart(d, 10, 5, pose)
    return upscale(im)


def rooster(pose="stand"):
    """Proud russet-and-gold rooster with arching emerald sickle tail and crimson comb."""
    im, d = canvas()
    line = (64, 36, 24, 255)
    body = (206, 88, 48, 255)
    hackle = (242, 178, 62, 255)
    breast = (248, 234, 206, 255)
    tail_dark = (34, 96, 92, 255)
    tail_hi = (66, 150, 136, 255)
    comb = (220, 44, 54, 255)
    comb_dark = (158, 24, 34, 255)
    gold = (238, 178, 44, 255)
    gold_dark = (184, 124, 22, 255)
    if pose in ("sick", "lying down"):
        body = sick_tint(body, 0.46)
        hackle = sick_tint(hackle, 0.45)
        breast = sick_tint(breast, 0.35)
        tail_dark = sick_tint(tail_dark, 0.42)
        tail_hi = sick_tint(tail_hi, 0.42)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 5 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Sturdy yellow legs with little spurs
    legs = {
        "walk 1": [(12, 1), (18, 0)],
        "walk 2": [(14, 0), (19, 1)],
        "lying down": [],
    }.get(pose, [(13, 0), (17, 0)])
    for x, lift in legs:
        d.line([(x, 25), (x, 29 - lift)], fill=gold_dark)
        d.line([(x + 1, 25), (x + 1, 29 - lift)], fill=gold)
        px(d, [(x - 1, 28 - lift)], gold_dark)  # tiny rooster spur
        d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=gold)
        d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=gold_dark)

    # Arching emerald-teal sickle tail plumes!
    if lying:
        # Drooped longer sickle tail plumes resting behind body
        d.polygon([(10, 22), (6, 17), (2, 19), (1, 25), (3, 28), (6, 24), (10, 25)], fill=tail_dark, outline=line)
        d.line([(3, 20), (7, 22)], fill=tail_hi)
        d.line([(2, 24), (7, 24)], fill=tail_hi)
    elif sick:
        d.polygon([(10, 18), (6, 12), (2, 14), (1, 20), (4, 23), (6, 19), (9, 21)], fill=tail_dark, outline=line)
        d.line([(3, 15), (7, 18)], fill=tail_hi)
        d.line([(3, 19), (7, 20)], fill=tail_hi)
    else:
        ty = -2 if happy else 0
        # Tall dramatic arching tail plumes sweeping left
        d.polygon([
            (11, 18), (8, 10 + ty), (3, 9 + ty), (1, 14 + ty),
            (3, 19 + ty), (6, 15 + ty), (9, 20),
        ], fill=tail_dark, outline=line)
        # Second lower sickle plume
        d.polygon([
            (10, 20), (5, 15 + ty), (2, 20 + ty), (5, 22 + ty), (10, 22),
        ], fill=tail_dark, outline=line)
        # Bright teal sheen stripes along the arch
        d.line([(4, 11 + ty), (7, 12 + ty), (9, 16)], fill=tail_hi)
        d.line([(3, 14 + ty), (6, 16 + ty)], fill=tail_hi)
        d.line([(4, 18 + ty), (8, 20)], fill=tail_hi)

    # Proud chest & russet body
    d.ellipse([9, 15 + by, 23, 26 + by], fill=body, outline=line)
    # Warm cream breast curve
    d.line([(20, 18 + by), (21, 22 + by)], fill=breast)
    d.line([(19, 22 + by), (20, 24 + by)], fill=breast)

    # Layered russet/golden wing
    wy = -2 if happy else 0
    d.ellipse([11, 18 + by + wy, 19, 24 + by + wy], fill=hackle, outline=line)
    d.line([(13, 20 + by + wy), (17, 20 + by + wy)], fill=body)
    d.line([(13, 22 + by + wy), (18, 22 + by + wy)], fill=line)

    # Golden neck hackles
    if not lying:
        d.polygon([(17, 17), (23, 17), (23, 10 + hy), (18, 10 + hy)], fill=hackle, outline=line)
        # Scalloped golden feather tips overlapping chest
        px(d, [(18, 17), (20, 18), (22, 17)], hackle)
    else:
        d.rectangle([18, 18, 22, 22], fill=hackle)

    # Head
    d.ellipse([19, 5 + hy, 26, 13 + hy], fill=hackle, outline=line)
    d.line([(19, 12 + hy), (22, 12 + hy)], fill=hackle)

    # Bright crimson 3-peak scalloped comb on top of head
    cy = 1 if sick else 0
    d.rectangle([20, 3 + hy + cy, 25, 5 + hy + cy], fill=comb, outline=comb_dark)
    px(d, [(20, 2 + hy + cy), (22, 2 + hy + cy), (24, 2 + hy + cy)], comb)
    px(d, [(21, 4 + hy + cy), (22, 4 + hy + cy), (23, 4 + hy + cy), (24, 4 + hy + cy)], comb)

    # Crimson wattle under chin
    d.rectangle([24, 11 + hy, 26, 14 + hy], fill=comb, outline=comb_dark)
    px(d, [(25, 12 + hy), (25, 13 + hy)], comb)

    # Pointed golden beak (open crowing on happy!)
    if happy:
        d.polygon([(26, 8), (30, 7), (26, 9)], fill=gold, outline=gold_dark)
        d.polygon([(26, 10), (30, 11), (26, 11)], fill=gold, outline=gold_dark)
    else:
        d.polygon([(26, 8 + hy), (30, 9 + hy), (26, 11 + hy)], fill=gold, outline=gold_dark)
        px(d, [(27, 9 + hy)], gold)

    draw_eye(d, 22, 8 + hy, pose, line)
    draw_cheek(d, 21, 10 + hy, pose)
    draw_heart(d, 13, 3, pose)
    return upscale(im)


def mouse(pose="stand"):
    """Plump harvest-taupe mouse with large round pink-lined ears, whiskers and curling pink tail."""
    im, d = canvas()
    line = (76, 66, 64, 255)
    coat = (180, 168, 160, 255)
    shade = (144, 132, 126, 255)
    belly = (236, 228, 220, 255)
    pink = (240, 162, 174, 255)
    pink_dark = (192, 108, 124, 255)
    if pose in ("sick", "lying down"):
        coat = sick_tint(coat, 0.42)
        shade = sick_tint(shade, 0.42)
        belly = sick_tint(belly, 0.32)
        pink = sick_tint(pink, 0.36)

    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 2 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Long curving pink tail on left (unmistakable mouse cue!)
    if lying:
        tail_pts = [(7, 29), (5, 30), (3, 30), (2, 29), (2, 28)]
    elif sick:
        tail_pts = [(7, 26), (5, 28), (3, 29), (2, 29), (2, 28)]
    elif happy:
        # Cheerful high spiral loop!
        tail_pts = [(7, 25), (4, 24), (2, 21), (2, 17), (3, 14), (5, 13), (6, 14), (5, 16)]
    else:
        wiggle = -1 if pose == "walk 1" else (1 if pose == "walk 2" else 0)
        tail_pts = [(7, 25), (4, 25), (2, 23 + wiggle), (2, 19 + wiggle), (3, 16 + wiggle), (5, 15 + wiggle)]
    d.line(tail_pts, fill=pink_dark, width=2)
    d.line(tail_pts, fill=pink, width=1)

    # Plump rounded low-slung body
    d.ellipse([6, 17 + by, 22, 29 + by], fill=coat, outline=line)
    # Pale cream belly
    d.line([(10, 27 + by), (18, 27 + by)], fill=belly)
    d.line([(11, 28 + by), (17, 28 + by)], fill=belly)
    # Subtle haunch shading
    d.arc([7, 19 + by, 15, 27 + by], start=130, end=240, fill=shade)

    # Dainty pink paws with warm bottom outline on y=30..31
    if lying:
        d.line([(8, 31), (22, 31)], fill=line)
        d.rectangle([19, 29, 23, 30], fill=pink, outline=pink_dark)
        d.line([(19, 31), (23, 31)], fill=line)
    elif pose == "walk 1":
        d.rectangle([8, 29, 12, 30], fill=pink, outline=pink_dark)
        d.line([(8, 31), (12, 31)], fill=line)
        d.rectangle([19, 27, 22, 28], fill=pink, outline=pink_dark)
        d.line([(19, 29), (22, 29)], fill=line)
    elif pose == "walk 2":
        d.rectangle([9, 27, 12, 28], fill=pink, outline=pink_dark)
        d.line([(9, 29), (12, 29)], fill=line)
        d.rectangle([18, 29, 22, 30], fill=pink, outline=pink_dark)
        d.line([(18, 31), (22, 31)], fill=line)
    elif happy:
        # Hind paws grounded at y=31, cute front paws raised in joy!
        d.rectangle([9, 29, 13, 30], fill=pink, outline=pink_dark)
        d.line([(9, 31), (13, 31)], fill=line)
        d.rectangle([19, 24, 22, 25], fill=pink, outline=pink_dark)
        d.line([(19, 26), (22, 26)], fill=line)
    else:
        d.rectangle([9, 29, 13, 30], fill=pink, outline=pink_dark)
        d.line([(9, 31), (13, 31)], fill=line)
        d.rectangle([18, 29, 22, 30], fill=pink, outline=pink_dark)
        d.line([(18, 31), (22, 31)], fill=line)

    # Neck/shoulder bridge up to head
    if not lying:
        d.polygon([(16, 19), (22, 19), (23, 12 + hy), (18, 12 + hy)], fill=coat, outline=line)
        d.rectangle([16, 18, 21, 22], fill=coat)

    # Far large round mouse ear (always clearly visible behind crown)
    ey_off = 1 if (sick or lying) else (-1 if happy else 0)
    ex_off = -1 if (sick or lying) else 0
    d.ellipse([16 + ex_off, 3 + hy + ey_off, 21 + ex_off, 8 + hy + ey_off], fill=shade, outline=line)

    # Head tapering smoothly into cute pointed mouse snout
    d.ellipse([18, 6 + hy, 27, 15 + hy], fill=coat, outline=line)
    d.polygon([(24, 7 + hy), (29, 10 + hy), (29, 11 + hy), (25, 14 + hy)], fill=coat, outline=line)
    # Blend snout seam into head
    d.rectangle([23, 8 + hy, 26, 13 + hy], fill=coat)

    # Near large round mouse ear drawn cleanly overlapping head edge (so ears stay prominent on sick & lying!)
    d.ellipse([16 + ex_off, 4 + hy + ey_off, 22 + ex_off, 10 + hy + ey_off], fill=coat, outline=line)
    d.ellipse([18 + ex_off, 6 + hy + ey_off, 20 + ex_off, 8 + hy + ey_off], fill=pink)

    # Rosy pink nose tip & crisp whiskers (all within x <= 30)
    px(d, [(29, 10 + hy), (29, 11 + hy), (28, 10 + hy)], pink)
    px(d, [(28, 9 + hy), (30, 8 + hy), (28, 12 + hy), (30, 13 + hy)], line)

    draw_eye(d, 23, 8 + hy, pose, line)
    draw_cheek(d, 22, 11 + hy, pose)
    draw_heart(d, 10, 7, pose)
    return upscale(im)


ANIMALS = [
    ("donkey", donkey),
    ("duck", duck),
    ("goose", goose),
    ("turkey", turkey),
    ("rabbit", rabbit),
    ("alpaca", alpaca),
    ("rooster", rooster),
    ("mouse", mouse),
]


def make_strip(draw_fn):
    im = Image.new("RGBA", (SIZE * len(FRAMES), SIZE), (0, 0, 0, 0))
    for i, pose in enumerate(FRAMES):
        im.alpha_composite(draw_fn(pose), (i * SIZE, 0))
    return im


def verify_strip(name, im):
    """Sanity-check hard edges, margins, ground contact and frame uniqueness."""
    assert im.size == (SIZE * len(FRAMES), SIZE), f"{name}: bad size {im.size}"
    pix = im.load()
    frame_bytes = set()
    for f in range(len(FRAMES)):
        x0 = f * SIZE
        solid = 0
        clear = 0
        grounded = 0
        buf = bytearray()
        for y in range(SIZE):
            for x in range(SIZE):
                r, g, b, a = pix[x0 + x, y]
                assert a in (0, 255), f"{name} frame {f} ({x},{y}) has semi-transparent alpha {a}"
                if x in (0, 1, SIZE - 2, SIZE - 1):
                    assert a == 0, f"{name} frame {f} ({x},{y}) touches horizontal edge margin"
                if a == 0:
                    clear += 1
                else:
                    solid += 1
                    if y == SIZE - 1:
                        grounded += 1
                buf.extend((r, g, b, a))
        assert solid > 0 and clear > 0, f"{name} frame {f} invalid solid={solid} clear={clear}"
        assert grounded > 0, f"{name} frame {f} has no feet on bottom edge y={SIZE - 1}"
        frame_bytes.add(bytes(buf))
    assert len(frame_bytes) == len(FRAMES), f"{name}: expected {len(FRAMES)} distinct frames, got {len(frame_bytes)}"


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    for name, fn in ANIMALS:
        im = make_strip(fn)
        verify_strip(name, im)
        out_path = OUT / f"{name}-strip.png"
        im.save(out_path, optimize=True)
        print(f"{out_path.name:20s} {out_path.stat().st_size:5d} bytes")
