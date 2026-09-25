-- Restore the legacy theme names. 'forest' was folded into sunroom on the way
-- up, so those users come back as 'default'.
UPDATE users SET theme = CASE theme
    WHEN 'sunroom' THEN 'default'
    WHEN 'blocks'  THEN 'quest'
    WHEN 'tint'    THEN 'galaxy'
    ELSE theme
END;

ALTER TABLE users DROP COLUMN color;
