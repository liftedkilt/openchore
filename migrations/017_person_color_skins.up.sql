-- Redesign: skins and person colours.
--
-- 1. users.theme holds a skin: sunroom, blocks or tint. Map the legacy
--    values; anything unknown becomes '' (resolved on the client by age).
UPDATE users SET theme = CASE theme
    WHEN 'default' THEN 'sunroom'
    WHEN 'forest'  THEN 'sunroom'
    WHEN 'quest'   THEN 'blocks'
    WHEN 'galaxy'  THEN 'tint'
    WHEN 'sunroom' THEN 'sunroom'
    WHEN 'blocks'  THEN 'blocks'
    WHEN 'tint'    THEN 'tint'
    ELSE ''
END;

-- 2. users.color holds a person colour key (not a hex value).
ALTER TABLE users ADD COLUMN color TEXT NOT NULL DEFAULT '';

-- 3a. Users with a #rrggbb line_color get the nearest person colour by
--     squared RGB distance to a representative hex per colour. Ties go to
--     the earlier palette entry.
WITH palette(key, idx, r, g, b) AS (VALUES
    ('coral',  0, 255, 155, 128),  -- #ff9b80
    ('mint',   1, 111, 216, 173),  -- #6fd8ad
    ('butter', 2, 255, 209, 102),  -- #ffd166
    ('sky',    3, 143, 195, 245),  -- #8fc3f5
    ('rose',   4, 245, 154, 184),  -- #f59ab8
    ('leaf',   5, 155, 207, 107),  -- #9bcf6b
    ('lilac',  6, 185, 166, 242),  -- #b9a6f2
    ('sand',   7, 217, 191, 148)   -- #d9bf94
),
hex AS (
    SELECT id, lower(substr(line_color, 2)) AS h
    FROM users
    WHERE length(line_color) = 7
      AND substr(line_color, 1, 1) = '#'
      AND lower(substr(line_color, 2)) NOT GLOB '*[^0-9a-f]*'
),
rgb AS (
    SELECT id,
        (instr('0123456789abcdef', substr(h, 1, 1)) - 1) * 16 + instr('0123456789abcdef', substr(h, 2, 1)) - 1 AS r,
        (instr('0123456789abcdef', substr(h, 3, 1)) - 1) * 16 + instr('0123456789abcdef', substr(h, 4, 1)) - 1 AS g,
        (instr('0123456789abcdef', substr(h, 5, 1)) - 1) * 16 + instr('0123456789abcdef', substr(h, 6, 1)) - 1 AS b
    FROM hex
),
ranked AS (
    SELECT rgb.id, p.key,
        ROW_NUMBER() OVER (
            PARTITION BY rgb.id
            ORDER BY (rgb.r - p.r) * (rgb.r - p.r) + (rgb.g - p.g) * (rgb.g - p.g) + (rgb.b - p.b) * (rgb.b - p.b), p.idx
        ) AS rn
    FROM rgb CROSS JOIN palette p
)
UPDATE users
SET color = (SELECT key FROM ranked WHERE ranked.id = users.id AND rn = 1)
WHERE id IN (SELECT id FROM ranked WHERE rn = 1);

-- 3b. Everyone else, in id order, gets the first colour nobody in the
--     household has yet; once all eight are taken, round-robin through the
--     palette. Siblings therefore differ.
WITH palette(key, idx) AS (VALUES
    ('coral', 0), ('mint', 1), ('butter', 2), ('sky', 3),
    ('rose', 4), ('leaf', 5), ('lilac', 6), ('sand', 7)
),
free AS (
    SELECT key, ROW_NUMBER() OVER (ORDER BY idx) - 1 AS n
    FROM palette
    WHERE key NOT IN (SELECT color FROM users)
),
pending AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY id) - 1 AS n
    FROM users
    WHERE color = ''
)
UPDATE users
SET color = (
    SELECT COALESCE(
        (SELECT key FROM free WHERE free.n = pending.n),
        (SELECT key FROM palette WHERE idx = (pending.n - (SELECT count(*) FROM free)) % 8)
    )
    FROM pending
    WHERE pending.id = users.id
)
WHERE id IN (SELECT id FROM pending);
