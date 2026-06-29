-- System categories (user_id IS NULL). Run manually after migrations.
-- psql $DATABASE_URL -f db/seeds/categories.sql

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
  ('Baby & Kids'),
  ('Accessories'),
  ('Entertainment')
ON CONFLICT DO NOTHING;
