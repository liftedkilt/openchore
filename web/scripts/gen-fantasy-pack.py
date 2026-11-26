#!/usr/bin/env python3
"""Generate the fantasy characters in internal/sprite/characters/.

Seven 6-frame transparent 384x64 strips: dragon, gnome (Christmas style),
hippogriff, unicorn, yeti, phoenix and griffin. Each is its own pet-care
reward ("character", docs/custom-animals.md), so the server keeps the
pictures (embedded in the binary) and the web app shows them from
/api/pets/characters/<id>.png.

Same rules as the farm starter pack (web/scripts/gen-starter-pack.py, whose
helpers this reuses): each frame is drawn on a 32x32 grid and scaled 2x
with hard edges, faces right, feet on the bottom edge, head upper right so
accessories land at the shared anchors.

Frame order (left to right):
  0: stand   1: walk 1   2: walk 2   3: sick   4: lying down   5: happy

All artwork is original to OpenChore (MIT licensed).

Run from the repo root:  python3 web/scripts/gen-fantasy-pack.py
"""
import importlib.util
from pathlib import Path

HERE = Path(__file__).resolve().parent
_spec = importlib.util.spec_from_file_location("starter", HERE / "gen-starter-pack.py")
starter = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(starter)

canvas, px, draw_eye, draw_cheek, draw_heart = starter.canvas, starter.px, starter.draw_eye, starter.draw_cheek, starter.draw_heart
sick_tint, upscale, make_strip, verify_strip = starter.sick_tint, starter.upscale, starter.make_strip, starter.verify_strip
WHITE = starter.WHITE

OUT = HERE.parent.parent / "internal" / "sprite" / "characters"

LEG_POSES = {
    # (x, lift) for the far pair and the near pair, like the starter pack's donkey.
    "far": {
        "stand": [(9, 0), (17, 0)],
        "walk 1": [(11, 0), (15, 0)],
        "walk 2": [(8, 0), (19, 1)],
        "sick": [(9, 0), (17, 0)],
        "happy": [(9, 0), (17, 0)],
    },
    "near": {
        "stand": [(6, 0), (19, 0)],
        "walk 1": [(6, 1), (20, 1)],
        "walk 2": [(11, 1), (16, 0)],
        "sick": [(6, 0), (19, 0)],
        "happy": [(6, 0), (20, 2)],
    },
}


def tinted(colors, pose, strength=0.44):
    if pose not in ("sick", "lying down"):
        return dict(colors)
    keep = colors.get("_keep", ())
    return {k: (v if k in keep or k == "_keep" else sick_tint(v, strength)) for k, v in colors.items()}


# --- Feet -------------------------------------------------------------------

def hoof_leg(d, lx, top, lift, fill, line, hoof):
    d.rectangle([lx, top, lx + 2, 29 - lift], fill=fill, outline=line)
    d.line([(lx + 1, top), (lx + 1, 28 - lift)], fill=fill)
    d.rectangle([lx, 29 - lift, lx + 2, 31 - lift], fill=hoof)


def talon_leg(d, lx, top, lift, fill, line, claw, claw_dark):
    """A feathered leg ending in a yellow foot with dark claws pointing forward."""
    d.rectangle([lx, top, lx + 2, 28 - lift], fill=fill, outline=line)
    d.line([(lx + 1, top), (lx + 1, 27 - lift)], fill=fill)
    d.rectangle([lx, 29 - lift, lx + 2, 30 - lift], fill=claw)
    d.line([(lx, 31 - lift), (lx + 3, 31 - lift)], fill=claw)
    px(d, [(lx + 4, 31 - lift), (lx - 1, 31 - lift)], claw_dark)


def paw_leg(d, lx, top, lift, fill, line, toe):
    """A sturdy lion or dragon leg with a rounded paw."""
    d.rectangle([lx, top, lx + 2, 29 - lift], fill=fill, outline=line)
    d.line([(lx + 1, top), (lx + 1, 28 - lift)], fill=fill)
    d.rectangle([lx, 29 - lift, lx + 3, 31 - lift], fill=fill, outline=line)
    px(d, [(lx + 3, 30 - lift)], fill)
    px(d, [(lx + 2, 31 - lift), (lx + 4, 31 - lift)], toe)


def draw_leg(d, kind, lx, top, lift, c, near):
    fill = c["coat"] if near else c["shade"]
    if kind == "hoof":
        hoof_leg(d, lx, top, lift, fill, c["line"], c["hoof"])
    elif kind == "talon":
        fill = c["feather"] if near else c["feather_shade"]
        talon_leg(d, lx, top, lift, fill, c["line"], c["claw"], c["claw_dark"])
    else:
        paw_leg(d, lx, top, lift, fill, c["line"], c.get("toe", c["line"]))


# --- Tails ------------------------------------------------------------------

def horse_tail(d, pose, c):
    hair, hair_hi = c["hair"], c.get("hair_hi", c["hair"])
    if pose == "lying down":
        d.polygon([(6, 25), (2, 27), (1, 31), (5, 31), (6, 28)], fill=hair, outline=c["line"])
        d.line([(3, 28), (4, 30)], fill=hair_hi)
    elif pose == "happy":
        d.polygon([(6, 16), (2, 12), (1, 15), (3, 19), (6, 19)], fill=hair, outline=c["line"])
        d.line([(3, 14), (5, 17)], fill=hair_hi)
    elif pose == "sick":
        d.polygon([(6, 17), (4, 22), (4, 28), (6, 27), (7, 20)], fill=hair, outline=c["line"])
    else:
        dx = -1 if pose == "walk 1" else (1 if pose == "walk 2" else 0)
        d.polygon([(6, 16), (3 + dx, 19), (2 + dx, 25), (4 + dx, 27), (6, 21)], fill=hair, outline=c["line"])
        d.line([(4 + dx, 20), (3 + dx, 24)], fill=hair_hi)


def lion_tail(d, pose, c):
    line, tuft = c["line"], c["tuft"]
    if pose == "lying down":
        d.line([(6, 27), (3, 29), (2, 30)], fill=line)
        d.rectangle([1, 29, 3, 31], fill=tuft)
    elif pose == "happy":
        d.line([(6, 17), (4, 14), (4, 10)], fill=line)
        d.rectangle([3, 8, 5, 10], fill=tuft)
    elif pose == "sick":
        d.line([(6, 19), (4, 23), (4, 27)], fill=line)
        d.rectangle([3, 27, 5, 29], fill=tuft)
    else:
        dx = -1 if pose == "walk 1" else (1 if pose == "walk 2" else 0)
        d.line([(6, 18), (3, 20 + dx), (2, 17 + dx)], fill=line)
        d.rectangle([1, 15 + dx, 3, 17 + dx], fill=tuft)


def dragon_tail(d, pose, c):
    line, coat, spike = c["line"], c["coat"], c["spike"]
    if pose == "lying down":
        d.polygon([(7, 24), (7, 29), (3, 31), (1, 31), (3, 28)], fill=coat, outline=line)
        d.polygon([(1, 28), (3, 28), (1, 31)], fill=spike)
    elif pose == "happy":
        d.polygon([(7, 17), (7, 22), (4, 19), (2, 13), (3, 12)], fill=coat, outline=line)
        d.polygon([(1, 10), (4, 11), (2, 13)], fill=spike, outline=line)
    elif pose == "sick":
        d.polygon([(7, 19), (7, 23), (4, 27), (2, 30), (2, 28), (4, 23)], fill=coat, outline=line)
        d.polygon([(1, 29), (3, 29), (2, 31)], fill=spike)
    else:
        dy = -1 if pose == "walk 1" else (1 if pose == "walk 2" else 0)
        d.polygon([(7, 18), (7, 23), (4, 24 + dy), (2, 24 + dy), (4, 21 + dy)], fill=coat, outline=line)
        d.polygon([(1, 22 + dy), (3, 21 + dy), (2, 25 + dy)], fill=spike, outline=line)


# --- Wings ------------------------------------------------------------------

WING_TIPS = {
    # Polygon for a wing folded on the back; the tip moves with the pose.
    "stand": [(9, 17), (17, 16), (14, 10), (7, 6), (8, 12)],
    "walk 1": [(9, 17), (17, 16), (14, 9), (6, 5), (8, 11)],
    "walk 2": [(9, 17), (17, 16), (14, 11), (8, 8), (8, 13)],
    "happy": [(9, 17), (17, 16), (15, 8), (9, 1), (8, 9)],
    "sick": [(9, 17), (17, 16), (13, 15), (5, 17), (7, 18)],
    "lying down": [(8, 22), (17, 21), (13, 19), (5, 19), (6, 21)],
}


def feather_wing(d, pose, c):
    pts = WING_TIPS[pose]
    d.polygon(pts, fill=c["wing"], outline=c["line"])
    (bx, by), (ex, ey), (mx, my), (tx, ty) = pts[0], pts[1], pts[2], pts[3]
    # Two rows of feather tips toward the tip.
    d.line([((bx + ex) // 2, (by + ey) // 2), (tx + 2, ty + 2)], fill=c["wing_shade"])
    d.line([(mx - 1, my + 1), ((bx + tx) // 2 + 1, (by + ty) // 2)], fill=c["wing_shade"])


def membrane_wing(d, pose, c):
    pts = WING_TIPS[pose]
    d.polygon(pts, fill=c["membrane"], outline=c["line"])
    (bx, by), (ex, ey), (mx, my), (tx, ty) = pts[0], pts[1], pts[2], pts[3]
    # Wing bones from the shoulder fanning to the scalloped edge.
    d.line([(ex - 1, ey), (tx, ty)], fill=c["line"])
    d.line([(ex - 1, ey), ((bx + tx) // 2, (by + ty) // 2 + 1)], fill=c["bone"])
    px(d, [(tx, ty)], c["spike"])


# --- Heads ------------------------------------------------------------------

def horse_head(d, pose, hy, c):
    line = c["line"]
    sick = pose in ("sick", "lying down")
    # Small pointed ears.
    if sick:
        d.polygon([(19, 6 + hy), (16, 6 + hy), (18, 8 + hy)], fill=c["coat"], outline=line)
    else:
        lift = -1 if pose == "happy" else 0
        d.polygon([(19, 6 + hy), (19, 2 + lift), (21, 5 + hy)], fill=c["coat"], outline=line)
    d.rounded_rectangle([18, 4 + hy, 27, 12 + hy], radius=3, fill=c["coat"], outline=line)
    d.rounded_rectangle([24, 8 + hy, 30, 13 + hy], radius=2, fill=c["muzzle"], outline=line)
    d.line([(24, 9 + hy), (24, 11 + hy)], fill=c["muzzle"])
    px(d, [(28, 9 + hy)], line)
    smile = [(26, 12 + hy), (27, 12 + hy)] + ([(28, 11 + hy)] if pose == "happy" else [])
    px(d, smile, line)
    draw_eye(d, 22, 8 + hy, pose, line)
    draw_cheek(d, 21, 10 + hy, pose, c.get("cheek", starter.CHEEK))


def eagle_head(d, pose, hy, c, ear_tufts=False):
    line = c["line"]
    # Feather tuft at the back of the head.
    px(d, [(17, 6 + hy), (18, 5 + hy), (17, 7 + hy)], c["feather_shade"])
    if ear_tufts and pose not in ("sick", "lying down"):
        d.polygon([(19, 5 + hy), (19, 1), (21, 4 + hy)], fill=c["feather_shade"], outline=line)
    d.ellipse([18, 4 + hy, 27, 12 + hy], fill=c["feather"], outline=line)
    # Hooked beak.
    d.polygon([(26, 7 + hy), (29, 7 + hy), (30, 9 + hy), (30, 11 + hy), (28, 10 + hy), (26, 10 + hy)], fill=c["claw"], outline=c["claw_dark"])
    px(d, [(29, 11 + hy)], c["claw_dark"])
    if pose == "happy":
        px(d, [(27, 10 + hy), (28, 10 + hy)], line)  # beak open a crack
    # Brow ridge.
    d.line([(21, 6 + hy), (24, 6 + hy)], fill=c["feather_shade"])
    draw_eye(d, 22, 7 + hy, pose, line)


def dragon_head(d, pose, hy, c):
    line = c["line"]
    sick = pose in ("sick", "lying down")
    # Two swept-back horns.
    if sick:
        d.polygon([(19, 6 + hy), (15, 5 + hy), (19, 8 + hy)], fill=c["horn"], outline=line)
    else:
        lift = -1 if pose == "happy" else 0
        d.polygon([(19, 6 + hy), (16, 2 + lift), (17, 1 + lift), (21, 5 + hy)], fill=c["horn"], outline=line)
        d.polygon([(22, 5 + hy), (21, 1 + lift), (22, 0 + max(lift, 0)), (24, 4 + hy)], fill=c["horn"], outline=line)
    d.rounded_rectangle([18, 4 + hy, 27, 12 + hy], radius=3, fill=c["coat"], outline=line)
    # Long snout with a lighter jaw.
    d.rounded_rectangle([23, 8 + hy, 30, 13 + hy], radius=2, fill=c["coat"], outline=line)
    d.line([(24, 12 + hy), (29, 12 + hy)], fill=c["belly"])
    d.line([(23, 9 + hy), (23, 11 + hy)], fill=c["coat"])
    px(d, [(28, 9 + hy)], line)  # nostril
    px(d, [(27, 13 + hy)], WHITE)  # little fang
    if pose == "happy":
        # A friendly puff of smoke from the nostril.
        px(d, [(29, 7), (30, 6), (29, 5), (30, 4)], c["smoke"])
    # Spines on the crown.
    px(d, [(20, 3 + hy), (19, 4 + hy)], c["spike"])
    draw_eye(d, 21, 7 + hy, pose, line)
    if pose not in ("sick", "lying down", "happy"):
        px(d, [(22, 8 + hy)], c["iris"])
    draw_cheek(d, 21, 10 + hy, pose)


# --- Four-legged characters ---------------------------------------------------

def quad(pose, c, head, front="hoof", back="hoof", tail=horse_tail, wing=None, neck="coat", mane=None, spines=False):
    im, d = canvas()
    line = c["line"]
    lying = pose == "lying down"
    by = 6 if lying else 0
    hy = 2 if pose == "sick" else (7 if lying else 0)

    if not lying:
        for i, (lx, lift) in enumerate(LEG_POSES["far"][pose]):
            draw_leg(d, back if i == 0 else front, lx, 23, lift, c, near=False)
    tail(d, pose, c)

    d.rounded_rectangle([5, 15 + by, 21, 24 + by], radius=3, fill=c["coat"], outline=line)
    d.line([(8, 23 + by), (18, 23 + by)], fill=c["belly"])
    if spines:
        for sx in (8, 11, 14):
            px(d, [(sx, 14 + by), (sx + 1, 13 + by), (sx + 1, 14 + by)], c["spike"])

    if lying:
        d.rectangle([6, 28, 12, 31], fill=c["coat"], outline=line)
        d.rectangle([15, 28, 21, 31], fill=c["coat"], outline=line)
        foot = c.get("hoof", c.get("claw", line))
        d.rectangle([11, 29, 13, 31], fill=foot)
        d.rectangle([20, 29, 22, 31], fill=c.get("claw", foot) if front == "talon" else foot)
    else:
        for i, (lx, lift) in enumerate(LEG_POSES["near"][pose]):
            draw_leg(d, back if i == 0 else front, lx, 22, lift, c, near=True)

    # Neck, in the head's colour for the eagle-fronted ones.
    nc = c[neck]
    if not lying:
        d.polygon([(16, 16), (22, 16), (23, 10 + hy), (18, 10 + hy)], fill=nc, outline=line)
        d.rectangle([17, 15, 21, 18], fill=nc)
        if neck == "feather":
            # Ragged feather ruff where the eagle front meets the body.
            px(d, [(15, 17), (16, 19), (17, 18), (15, 20)], c["feather"])
            px(d, [(15, 18)], c["feather_shade"])
        if mane:
            for i, my in enumerate(range(10 + hy, 16)):
                px(d, [(17, my), (16, my)], mane[i % len(mane)])
    else:
        d.rectangle([18, 17, 22, 22], fill=nc)
        if mane:
            for i, my in enumerate(range(17, 22)):
                px(d, [(17, my)], mane[i % len(mane)])

    if wing:
        wing(d, pose, c)
    head(d, pose, hy, c)
    if pose == "happy":
        draw_heart(d, 3, 9 if wing else 5, pose) if wing else draw_heart(d, 11, 5, pose)
    return upscale(im)


def dragon(pose="stand"):
    """Friendly jade-green dragon with cream belly, bat wings, horns and a spade tail."""
    c = tinted({
        "line": (30, 52, 44, 255),
        "coat": (86, 170, 104, 255),
        "shade": (58, 132, 82, 255),
        "belly": (238, 222, 150, 255),
        "membrane": (232, 132, 92, 255),
        "bone": (186, 92, 62, 255),
        "spike": (246, 194, 86, 255),
        "horn": (244, 232, 204, 255),
        "iris": (250, 214, 74, 255),
        "smoke": (206, 206, 214, 255),
        "toe": (244, 232, 204, 255),
        "_keep": ("line", "horn", "smoke"),
    }, pose)
    return quad(pose, c, dragon_head, front="paw", back="paw", tail=dragon_tail, wing=membrane_wing, spines=True)


def unicorn(pose="stand"):
    """Snow-white unicorn with a pastel rainbow mane and tail and a golden spiral horn."""
    c = tinted({
        "line": (70, 58, 90, 255),
        "coat": (250, 248, 252, 255),
        "shade": (214, 210, 228, 255),
        "belly": (234, 230, 244, 255),
        "muzzle": (252, 226, 236, 255),
        "hoof": (176, 156, 214, 255),
        "hair": (246, 156, 196, 255),
        "hair_hi": (150, 206, 246, 255),
        "gold": (246, 200, 76, 255),
        "gold_hi": (255, 238, 160, 255),
        "_keep": ("line",),
    }, pose, 0.38)
    mane = [(246, 156, 196, 255), (252, 214, 120, 255), (150, 220, 170, 255), (150, 206, 246, 255), (196, 160, 240, 255)]
    if pose in ("sick", "lying down"):
        mane = [sick_tint(m, 0.4) for m in mane]

    def head(d, p, hy, cc):
        horse_head(d, p, hy, cc)
        # Spiral horn rising from the forehead, tip up and forward.
        tilt = 2 if p in ("sick", "lying down") else 0
        horn = [(23, 4 + hy), (24, 3 + hy), (24, 2 + hy + tilt // 2), (25, 1 + hy + tilt // 2), (25, 0 + hy + tilt)]
        px(d, [(22, 4 + hy), (23, 3 + hy)], cc["gold"])
        px(d, horn, cc["gold"])
        px(d, [(24, 3 + hy), (25, 1 + hy + tilt // 2)], cc["gold_hi"])
        # Forelock in the rainbow colours.
        px(d, [(19, 4 + hy), (20, 4 + hy), (19, 5 + hy)], mane[0])
        px(d, [(20, 5 + hy)], mane[3])

    return quad(pose, c, head, tail=horse_tail, mane=mane)


def hippogriff(pose="stand"):
    """Hippogriff: a white eagle's head, wings and talons on a chestnut horse's body."""
    c = tinted({
        "line": (56, 40, 34, 255),
        "coat": (176, 106, 62, 255),
        "shade": (140, 80, 46, 255),
        "belly": (206, 146, 100, 255),
        "hoof": (58, 46, 44, 255),
        "hair": (70, 46, 36, 255),
        "hair_hi": (110, 74, 54, 255),
        "feather": (246, 244, 236, 255),
        "feather_shade": (196, 192, 184, 255),
        "wing": (226, 222, 212, 255),
        "wing_shade": (168, 160, 150, 255),
        "claw": (246, 196, 70, 255),
        "claw_dark": (150, 104, 28, 255),
        "_keep": ("line", "hoof"),
    }, pose)
    return quad(pose, c, eagle_head, front="talon", back="hoof", tail=horse_tail, wing=feather_wing, neck="feather")


def griffin(pose="stand"):
    """Griffin: a golden eagle's head, wings and talons on a tawny lion's body with a tufted tail."""
    c = tinted({
        "line": (62, 42, 24, 255),
        "coat": (222, 170, 88, 255),
        "shade": (186, 134, 60, 255),
        "belly": (240, 212, 150, 255),
        "toe": (120, 80, 40, 255),
        "tuft": (132, 80, 38, 255),
        "feather": (150, 96, 50, 255),
        "feather_shade": (104, 64, 34, 255),
        "wing": (168, 112, 58, 255),
        "wing_shade": (110, 70, 36, 255),
        "claw": (248, 204, 70, 255),
        "claw_dark": (150, 104, 28, 255),
        "_keep": ("line",),
    }, pose)

    def head(d, p, hy, cc):
        eagle_head(d, p, hy, cc, ear_tufts=True)
        # Pale face patch around the eye.
        px(d, [(24, 8 + hy), (24, 9 + hy), (25, 9 + hy)], (236, 214, 170, 255))

    return quad(pose, c, head, front="talon", back="paw", tail=lion_tail, wing=feather_wing, neck="feather")


# --- Upright characters -------------------------------------------------------

def gnome(pose="stand"):
    """A Christmas gnome: tall red hat with fur trim and holly, white beard, round nose, green coat."""
    im, d = canvas()
    c = tinted({
        "line": (60, 34, 34, 255),
        "hat": (214, 50, 56, 255),
        "hat_shade": (164, 30, 40, 255),
        "fur": (250, 248, 244, 255),
        "fur_shade": (214, 214, 214, 255),
        "skin": (250, 206, 176, 255),
        "nose": (238, 128, 120, 255),
        "coat": (52, 140, 82, 255),
        "coat_shade": (34, 104, 60, 255),
        "belt": (76, 48, 34, 255),
        "gold": (246, 200, 76, 255),
        "boot": (98, 62, 44, 255),
        "boot_dark": (66, 40, 30, 255),
        "holly": (40, 120, 60, 255),
        "berry": (226, 36, 48, 255),
        "_keep": ("line", "fur", "berry"),
    }, pose, 0.4)
    line = c["line"]
    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    # Sitting down when very sick: everything above the legs drops.
    dy = 5 if lying else (1 if sick else 0)

    # Far arm behind the coat.
    if not lying:
        swing = {"walk 1": 1, "walk 2": -1}.get(pose, 0)
        d.rectangle([12 + swing, 17 + dy, 14 + swing, 23 + dy], fill=c["coat_shade"], outline=line)
        d.rectangle([12 + swing, 23 + dy, 14 + swing, 25 + dy], fill=c["skin"], outline=line)

    # Legs and pointy boots.
    if lying:
        d.rectangle([18, 27, 27, 29], fill=c["coat_shade"], outline=line)
        d.polygon([(24, 26), (28, 26), (30, 27), (29, 31), (24, 31)], fill=c["boot"], outline=line)
        d.polygon([(20, 28), (23, 28), (25, 29), (25, 31), (20, 31)], fill=c["boot_dark"], outline=line)
    else:
        legs = {"walk 1": [(13, 1), (19, 0)], "walk 2": [(15, 0), (17, 1)]}.get(pose, [(14, 0), (18, 0)])
        for i, (lx, lift) in enumerate(legs):
            boot = c["boot_dark"] if i == 0 else c["boot"]
            d.rectangle([lx, 26, lx + 2, 28 - lift], fill=c["coat_shade"], outline=line)
            d.polygon([(lx - 1, 29 - lift), (lx + 3, 29 - lift), (lx + 5, 30 - lift), (lx + 5, 31 - lift), (lx - 1, 31 - lift)], fill=boot, outline=line)
            px(d, [(lx + 5, 29 - lift)], boot)  # curled toe

    # Round green coat with a white hem and a gold buckle.
    top, bottom = 16 + dy, 27 + dy if not lying else 30
    d.polygon([(14, top), (23, top), (25, bottom), (12, bottom)], fill=c["coat"], outline=line)
    d.line([(13, bottom - 1), (24, bottom - 1)], fill=c["fur"])
    d.line([(13, 22 + dy), (24, 22 + dy)], fill=c["belt"])
    d.rectangle([18, 21 + dy, 20, 23 + dy], fill=c["gold"], outline=line)
    px(d, [(19, 22 + dy)], c["belt"])

    # Bushy white beard over the chest.
    d.polygon([(18, 11 + dy), (27, 11 + dy), (27, 15 + dy), (24, 20 + dy), (21, 21 + dy), (18, 18 + dy), (17, 13 + dy)], fill=c["fur"], outline=line)
    px(d, [(20, 15 + dy), (23, 17 + dy), (25, 14 + dy)], c["fur_shade"])

    # Near arm with a mitten.
    if happy:
        d.rectangle([22, 13, 24, 17], fill=c["coat"], outline=line)
        d.rectangle([23, 11, 26, 13], fill=c["hat"], outline=line)  # waving mitten
    elif not lying:
        swing = {"walk 1": -1, "walk 2": 1}.get(pose, 0)
        d.rectangle([20 + swing, 17 + dy, 22 + swing, 23 + dy], fill=c["coat"], outline=line)
        d.rectangle([20 + swing, 23 + dy, 23 + swing, 25 + dy], fill=c["hat"], outline=line)
    else:
        d.rectangle([16, 23, 22, 25], fill=c["coat"], outline=line)
        d.rectangle([22, 23, 24, 25], fill=c["hat"], outline=line)

    # Face peeking between the hat and the beard.
    d.rectangle([19, 8 + dy, 26, 11 + dy], fill=c["skin"], outline=line)
    d.line([(20, 11 + dy), (25, 11 + dy)], fill=c["skin"])
    draw_eye(d, 22, 9 + dy, pose, line)
    # Moustache and round nose.
    d.line([(22, 12 + dy), (27, 12 + dy)], fill=c["fur"])
    d.rectangle([24, 9 + dy, 27, 12 + dy], fill=c["nose"], outline=line)
    px(d, [(25, 10 + dy)], (255, 196, 190, 255))

    # Tall hat: fur brim, cone leaning back, pompom; it flops when unwell.
    d.rectangle([17, 7 + dy, 27, 8 + dy], fill=c["fur"], outline=line)
    if sick or lying:
        d.polygon([(17, 7 + dy), (27, 7 + dy), (22, 3 + dy), (14, 5 + dy)], fill=c["hat"], outline=line)
        d.line([(18, 6 + dy), (22, 4 + dy)], fill=c["hat_shade"])
        d.ellipse([11, 4 + dy, 14, 7 + dy], fill=c["fur"], outline=line)
    else:
        lean = 1 if pose == "walk 1" else (-1 if pose == "walk 2" else 0)
        d.polygon([(17, 7), (27, 7), (22, 2), (17 + lean, 0)], fill=c["hat"], outline=line)
        d.line([(19, 6), (18 + lean, 2)], fill=c["hat_shade"])
        d.ellipse([14 + lean, 0, 17 + lean, 3], fill=c["fur"], outline=line)
    # A sprig of holly on the brim.
    px(d, [(25, 6 + dy), (26, 5 + dy), (24, 5 + dy)], c["holly"])
    px(d, [(25, 7 + dy), (26, 7 + dy)], c["berry"])

    draw_cheek(d, 20, 10 + dy, pose)
    draw_heart(d, 6, 9, pose)
    return upscale(im)


def yeti(pose="stand"):
    """A shaggy snow-white yeti with an icy blue face, long arms and big feet."""
    im, d = canvas()
    c = tinted({
        "line": (52, 62, 92, 255),
        "fur": (246, 250, 255, 255),
        "fur_shade": (198, 214, 236, 255),
        "face": (128, 170, 220, 255),
        "face_dark": (92, 132, 186, 255),
        "horn": (232, 220, 190, 255),
        "mouth": (70, 60, 96, 255),
        "_keep": ("line", "mouth", "horn"),
    }, pose, 0.36)
    line = c["line"]
    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    dy = 6 if lying else (1 if sick else 0)

    # Far arm.
    if not lying:
        swing = {"walk 1": 1, "walk 2": -1}.get(pose, 0)
        if happy:
            d.rectangle([8, 6, 11, 14], fill=c["fur_shade"], outline=line)
        else:
            d.rectangle([7 + swing, 13 + dy, 10 + swing, 25 + dy], fill=c["fur_shade"], outline=line)

    # Legs and big flat feet.
    if lying:
        d.rectangle([20, 27, 29, 31], fill=c["fur"], outline=line)
        d.rectangle([26, 25, 29, 31], fill=c["face"], outline=line)
    else:
        legs = {"walk 1": [(10, 1), (17, 0)], "walk 2": [(12, 0), (15, 1)]}.get(pose, [(11, 0), (16, 0)])
        for lx, lift in legs:
            d.rectangle([lx, 25, lx + 3, 29 - lift], fill=c["fur_shade"], outline=line)
            d.rectangle([lx - 1, 29 - lift, lx + 5, 31 - lift], fill=c["face"], outline=line)

    # Big shaggy body.
    d.rounded_rectangle([8, 10 + dy, 24, 27 + dy if not lying else 31], radius=5, fill=c["fur"], outline=line)
    for sx, sy in ((10, 15), (13, 19), (17, 23), (11, 23), (20, 18)):
        px(d, [(sx, sy + dy), (sx + 1, sy + 1 + dy)], c["fur_shade"])
    # Ragged fur along the bottom edge.
    if not lying:
        px(d, [(9, 27), (12, 28), (19, 28), (23, 27)], c["fur"])

    # Head with little horns and a blue face.
    d.polygon([(18, 4 + dy), (17, 1 + dy), (19, 3 + dy)], fill=c["horn"], outline=line)
    d.polygon([(25, 4 + dy), (26, 1 + dy), (27, 4 + dy)], fill=c["horn"], outline=line)
    d.rounded_rectangle([16, 2 + dy, 28, 14 + dy], radius=4, fill=c["fur"], outline=line)
    d.rounded_rectangle([20, 6 + dy, 28, 13 + dy], radius=2, fill=c["face"], outline=line)
    # Shaggy fringe over the brow.
    px(d, [(20, 6 + dy), (22, 6 + dy), (24, 6 + dy), (26, 6 + dy)], c["fur"])
    draw_eye(d, 23, 8 + dy, pose, line)
    px(d, [(27, 10 + dy)], c["face_dark"])  # nose
    if happy:
        d.rectangle([23, 11, 26, 12], fill=c["mouth"])
        px(d, [(24, 11)], WHITE)
    elif sick or lying:
        d.line([(23, 12 + dy), (25, 12 + dy)], fill=c["mouth"])
    else:
        px(d, [(23, 11 + dy), (24, 12 + dy), (25, 12 + dy), (26, 11 + dy)], c["mouth"])

    # Near arm, long and shaggy.
    if happy:
        d.rectangle([14, 3, 17, 12], fill=c["fur"], outline=line)
        d.rectangle([14, 1, 17, 3], fill=c["face"], outline=line)
    elif lying:
        d.rectangle([15, 22, 25, 25], fill=c["fur"], outline=line)
        d.rectangle([25, 22, 28, 25], fill=c["face"], outline=line)
    else:
        swing = {"walk 1": -1, "walk 2": 1}.get(pose, 0)
        d.rectangle([18 + swing, 15 + dy, 21 + swing, 25 + dy], fill=c["fur"], outline=line)
        d.rectangle([18 + swing, 25 + dy, 22 + swing, 27 + dy], fill=c["face"], outline=line)

    draw_cheek(d, 21, 10 + dy, pose, (176, 200, 246, 255))
    draw_heart(d, 4, 4, pose)
    return upscale(im)


def phoenix(pose="stand"):
    """A phoenix: crimson and orange bird with a flame crest and a long tail of fire."""
    im, d = canvas()
    c = tinted({
        "line": (90, 30, 20, 255),
        "body": (222, 62, 44, 255),
        "body_hi": (246, 122, 58, 255),
        "wing": (246, 148, 54, 255),
        "flame": (252, 206, 72, 255),
        "flame_hot": (255, 244, 170, 255),
        "gold": (246, 196, 64, 255),
        "gold_dark": (178, 116, 22, 255),
        "_keep": ("line",),
    }, pose, 0.48)
    line = c["line"]
    lying = pose == "lying down"
    sick = pose == "sick"
    happy = pose == "happy"
    by = 5 if lying else 0
    hy = 2 if sick else (7 if lying else 0)

    # Thin golden legs.
    if not lying:
        legs = {"walk 1": [(12, 1), (18, 0)], "walk 2": [(14, 0), (19, 1)]}.get(pose, [(13, 0), (17, 0)])
        for x, lift in legs:
            d.line([(x, 25), (x, 29 - lift)], fill=c["gold_dark"])
            d.line([(x + 1, 25), (x + 1, 29 - lift)], fill=c["gold"])
            d.line([(x - 1, 30 - lift), (x + 3, 30 - lift)], fill=c["gold"])
            d.line([(x - 1, 31 - lift), (x + 3, 31 - lift)], fill=c["gold_dark"])

    # Tail of flame sweeping back and down.
    if lying:
        d.polygon([(10, 23), (4, 21), (1, 26), (2, 31), (8, 31), (10, 28)], fill=c["body_hi"], outline=line)
        d.polygon([(8, 25), (4, 25), (3, 30), (7, 30)], fill=c["flame"])
    elif sick:
        d.polygon([(10, 18), (5, 19), (2, 26), (4, 30), (7, 25), (10, 23)], fill=c["body_hi"], outline=line)
        d.line([(4, 26), (8, 21)], fill=c["flame"])
    else:
        lift = -3 if happy else {"walk 1": -1, "walk 2": 1}.get(pose, 0)
        d.polygon([(10, 17), (6, 10 + lift), (1, 8 + lift), (3, 13 + lift), (1, 16 + lift), (4, 18), (1, 22), (5, 23), (2, 28), (8, 24), (10, 23)], fill=c["body_hi"], outline=line)
        d.line([(3, 10 + lift), (8, 16)], fill=c["flame"])
        d.line([(2, 16 + lift), (8, 19)], fill=c["flame"])
        d.line([(4, 22), (8, 21)], fill=c["flame"])
        px(d, [(2, 9 + lift), (2, 27)], c["flame_hot"])

    # Body.
    d.ellipse([9, 14 + by, 23, 26 + by], fill=c["body"], outline=line)
    d.line([(20, 18 + by), (21, 22 + by)], fill=c["body_hi"])

    # Wing: raised in a blaze when happy.
    if happy:
        d.polygon([(12, 19), (18, 18), (16, 10), (11, 3), (9, 10)], fill=c["wing"], outline=line)
        d.line([(11, 6), (14, 16)], fill=c["flame"])
        px(d, [(11, 4)], c["flame_hot"])
    else:
        wy = 2 if sick else 0
        d.ellipse([11, 17 + by + wy, 20, 23 + by + wy], fill=c["wing"], outline=line)
        d.line([(13, 20 + by + wy), (18, 20 + by + wy)], fill=c["flame"])

    # Neck and head.
    if not lying:
        d.polygon([(17, 17), (23, 17), (23, 10 + hy), (18, 10 + hy)], fill=c["body"], outline=line)
        px(d, [(18, 17), (20, 18), (22, 17)], c["body"])
    else:
        d.rectangle([18, 18, 22, 22], fill=c["body"])
    d.ellipse([19, 5 + hy, 26, 12 + hy], fill=c["body"], outline=line)

    # Flame crest: three tongues of fire, small when unwell.
    if sick or lying:
        px(d, [(20, 4 + hy), (21, 3 + hy), (22, 4 + hy)], c["flame"])
    else:
        tall = -1 if happy else 0
        d.polygon([(19, 6), (17, 1 + tall), (21, 4)], fill=c["flame"], outline=c["gold_dark"])
        d.polygon([(21, 5), (21, 0 + max(tall, 0)), (23, 4)], fill=c["flame"], outline=c["gold_dark"])
        d.polygon([(23, 5), (25, 2), (25, 6)], fill=c["flame"], outline=c["gold_dark"])
        px(d, [(18, 3), (21, 2)], c["flame_hot"])

    # Golden beak.
    if happy:
        d.polygon([(26, 7), (30, 7), (26, 9)], fill=c["gold"], outline=c["gold_dark"])
        d.polygon([(26, 10), (29, 11), (26, 11)], fill=c["gold"], outline=c["gold_dark"])
    else:
        d.polygon([(26, 8 + hy), (30, 9 + hy), (26, 11 + hy)], fill=c["gold"], outline=c["gold_dark"])

    draw_eye(d, 22, 8 + hy, pose, line)
    draw_cheek(d, 21, 10 + hy, pose, (252, 196, 90, 255))
    draw_heart(d, 27, 1, pose)
    return upscale(im)


CHARACTERS = [
    ("dragon", dragon),
    ("gnome", gnome),
    ("hippogriff", hippogriff),
    ("unicorn", unicorn),
    ("yeti", yeti),
    ("phoenix", phoenix),
    ("griffin", griffin),
]


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    for name, fn in CHARACTERS:
        im = make_strip(fn)
        verify_strip(name, im)
        out_path = OUT / f"{name}.png"
        im.save(out_path, optimize=True)
        print(f"{out_path.name:16s} {out_path.stat().st_size:5d} bytes")
