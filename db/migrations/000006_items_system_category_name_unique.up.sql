-- System items (user_id IS NULL) must have a unique name within their
-- category so the seed script (db/seeds/items.sql) can be re-run
-- idempotently via ON CONFLICT. Mirrors migration 000002's approach for
-- categories. Case-insensitive to match ItemNameExistsInCategory's
-- LOWER(name) comparison. Scoped to (category_id, LOWER(name)) rather
-- than name alone, since the same item name is allowed to exist in
-- different categories.
CREATE UNIQUE INDEX IF NOT EXISTS idx_items_system_category_name_unique ON items (category_id, LOWER(name))
WHERE
    user_id IS NULL;
