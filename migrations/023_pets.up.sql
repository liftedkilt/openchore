-- Optional farm pets: one animal per person, all on one shared farm, kept
-- healthy by doing chores and cared for with points (pet-care rewards).
-- See docs/pets.md.
CREATE TABLE pets (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT    NOT NULL,
    species          TEXT    NOT NULL DEFAULT 'dog',
    health           INTEGER NOT NULL DEFAULT 80,
    fullness         INTEGER NOT NULL DEFAULT 70,
    joy              INTEGER NOT NULL DEFAULT 70,
    accessory        TEXT    NOT NULL DEFAULT '',
    last_missed_date TEXT,
    last_day_checked TEXT    NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Accessories a pet owns (bought as rewards); one is worn at a time.
CREATE TABLE pet_items (
    pet_id      INTEGER NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    item        TEXT    NOT NULL,
    acquired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pet_id, item)
);

-- '' = an ordinary reward; feed / treat / play / accessory = pet care.
-- pet_item names the accessory an "accessory" reward gives.
ALTER TABLE rewards ADD COLUMN pet_action TEXT NOT NULL DEFAULT '';
ALTER TABLE rewards ADD COLUMN pet_item TEXT NOT NULL DEFAULT '';
