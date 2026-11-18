-- Custom animals: a person's own picture shown in place of the drawn animal.
-- sprite: '' = the built-in animal; otherwise the id of a PNG under
-- uploads/pets. sprite_frames: how many equal square frames that PNG holds
-- side by side (1 = a single picture; up to 6: stand, walk, walk, sick,
-- lying down, happy). See docs/custom-animals.md.
ALTER TABLE pets ADD COLUMN sprite TEXT NOT NULL DEFAULT '';
ALTER TABLE pets ADD COLUMN sprite_frames INTEGER NOT NULL DEFAULT 1;
-- A parent has looked at the picture and said it may be sent to friend
-- families. Cleared whenever the picture changes.
ALTER TABLE pets ADD COLUMN sprite_approved INTEGER NOT NULL DEFAULT 0;
