-- Clear any stored theme value on admin users. Admins never have a theme —
-- the admin UI always uses the default styling.
UPDATE users SET theme = '' WHERE role = 'admin';
