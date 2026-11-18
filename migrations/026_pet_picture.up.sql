-- Custom animals: a person's own picture shown in place of the drawn animal.
-- '' = the built-in animal; otherwise the id of a PNG under uploads/pets.
ALTER TABLE pets ADD COLUMN sprite TEXT NOT NULL DEFAULT '';
