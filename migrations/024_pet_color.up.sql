-- Farm pets: a coat colour per animal ('' = the species' default coat).
ALTER TABLE pets ADD COLUMN color TEXT NOT NULL DEFAULT '';
