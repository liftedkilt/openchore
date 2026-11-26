-- Character rewards nothing points at go; any others stay as ordinary,
-- inactive rewards so their history still has a name.
DELETE FROM rewards WHERE pet_action = 'character'
  AND id NOT IN (SELECT reward_id FROM reward_redemptions)
  AND id NOT IN (SELECT reward_id FROM reward_commitments)
  AND id NOT IN (SELECT reward_id FROM shared_commitment_pools);
UPDATE rewards SET pet_action = '', pet_item = '', active = 0 WHERE pet_action = 'character';
ALTER TABLE pets DROP COLUMN sprite_character;
