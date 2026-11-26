-- Ready-made characters (docs/custom-animals.md): a "character" pet-care
-- reward turns the animal into one picture the server ships, named by
-- pet_item. sprite_character remembers which one the animal is, so that
-- reward shows as owned; it is cleared whenever the picture changes.
ALTER TABLE pets ADD COLUMN sprite_character TEXT NOT NULL DEFAULT '';

-- A household that already turned farm pets on got its pet-care rewards
-- then, so add one reward per character now. One that turns pets on later
-- gets them with the starter set (defaultPetRewards in internal/api/pets.go).
INSERT INTO rewards (name, description, icon, cost, active, pet_action, pet_item, created_by)
SELECT c.name, c.description, c.icon, 100, 1, 'character', c.id,
       (SELECT created_by FROM rewards WHERE pet_action != '' ORDER BY id LIMIT 1)
FROM (
    SELECT 1 AS n, 'dragon' AS id, 'Dragon' AS name, 'Turn your farm animal into a friendly dragon.' AS description, '🐉' AS icon
    UNION ALL SELECT 2, 'gnome', 'Christmas gnome', 'Turn your farm animal into a jolly Christmas gnome.', '🎄'
    UNION ALL SELECT 3, 'hippogriff', 'Hippogriff', 'Turn your farm animal into a hippogriff: part eagle, part horse.', '🪶'
    UNION ALL SELECT 4, 'unicorn', 'Unicorn', 'Turn your farm animal into a rainbow unicorn.', '🦄'
    UNION ALL SELECT 5, 'yeti', 'Yeti', 'Turn your farm animal into a fluffy snow yeti.', '❄️'
    UNION ALL SELECT 6, 'phoenix', 'Phoenix', 'Turn your farm animal into a blazing phoenix.', '🔥'
    UNION ALL SELECT 7, 'griffin', 'Griffin', 'Turn your farm animal into a griffin: part eagle, part lion.', '🦁'
) AS c
WHERE EXISTS (SELECT 1 FROM rewards WHERE pet_action != '')
  AND NOT EXISTS (SELECT 1 FROM rewards WHERE pet_action = 'character')
ORDER BY c.n;
