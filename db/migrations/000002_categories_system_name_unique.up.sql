-- System categories (user_id IS NULL) must have unique names so the seed
-- script (db/seeds/categories.sql) can be re-run idempotently via
-- ON CONFLICT. Scoped to user_id IS NULL only — user-owned categories'
-- per-user uniqueness is already enforced at the application layer
-- (CategoryNameExistsForUser) and is a separate concern.
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_system_name_unique ON categories (name)
WHERE
    user_id IS NULL;
