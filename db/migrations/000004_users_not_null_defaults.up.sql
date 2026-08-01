UPDATE users SET avatar_url = '' WHERE avatar_url IS NULL;
UPDATE users SET display_name = '' WHERE display_name IS NULL;
UPDATE users SET last_login_at = COALESCE(created_at, CURRENT_TIMESTAMP) WHERE last_login_at IS NULL;

ALTER TABLE users
    ALTER COLUMN avatar_url SET DEFAULT '',
    ALTER COLUMN avatar_url SET NOT NULL,
    ALTER COLUMN display_name SET DEFAULT '',
    ALTER COLUMN display_name SET NOT NULL,
    ALTER COLUMN last_login_at SET DEFAULT CURRENT_TIMESTAMP,
    ALTER COLUMN last_login_at SET NOT NULL;
