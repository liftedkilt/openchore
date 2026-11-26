-- Optional pet deaths (docs/pets.md): a parent can choose that an animal
-- that stays sick for a number of days dies. sick_days counts the finished,
-- unpaused days in a row it ended sick; a death leaves a grave on the farm.
ALTER TABLE pets ADD COLUMN sick_days INTEGER NOT NULL DEFAULT 0;

CREATE TABLE pet_graves (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    species    TEXT    NOT NULL,
    color      TEXT    NOT NULL DEFAULT '',
    adopted_on TEXT    NOT NULL,
    died_on    TEXT    NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
