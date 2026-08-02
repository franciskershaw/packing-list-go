-- System categories (user_id IS NULL). Run manually after migrations.
-- Default: paste into Neon's web SQL Editor (console.neon.tech) — no
-- local Postgres client needed. If you do have psql:
-- psql $DATABASE_URL -f db/seeds/categories.sql
--
-- Idempotent as of migration 000002 (idx_categories_system_name_unique) —
-- safe to re-run; conflicting rows are left as-is, not duplicated.

INSERT INTO categories (name) VALUES
  ('Toiletries'),
  ('Clothing'),
  ('Footwear'),
  ('Electronics'),
  ('Documents'),
  ('Medical'),
  ('Food & Drink'),
  ('Camping Gear'),
  ('Sports & Fitness'),
  ('Accessories'),
  ('Entertainment')
ON CONFLICT (name)
WHERE user_id IS NULL
DO NOTHING;
