-- System items (user_id IS NULL), attached to their built-in category by
-- name lookup. Run manually after migration 000006 has applied in this
-- environment (paste into Neon's web SQL Editor at console.neon.tech, or
-- via psql: psql $DATABASE_URL -f db/seeds/items.sql).
--
-- Idempotent as of migration 000006
-- (idx_items_system_category_name_unique) — safe to re-run or extend;
-- conflicting rows are left as-is, not duplicated.

INSERT INTO
    items (category_id, name)
SELECT c.id, v.item_name
FROM (
    VALUES
        ('Clothing', 'Pants'),
        ('Clothing', 'Socks'),
        ('Clothing', 'Jeans'),
        ('Clothing', 'Shorts'),
        ('Clothing', 'T-Shirt'),
        ('Clothing', 'Shirt'),
        ('Clothing', 'Jumper'),
        ('Clothing', 'Beanie Hat'),
        ('Clothing', 'Cap'),
        ('Clothing', 'Jacket'),
        ('Clothing', 'Waterproof'),
        ('Toiletries', 'Toothbrush'),
        ('Toiletries', 'Toothpaste'),
        ('Toiletries', 'Moisturiser'),
        ('Toiletries', 'Sun Cream'),
        ('Footwear', 'Trainers'),
        ('Footwear', 'Flip Flops'),
        ('Footwear', 'Walking Boots'),
        ('Footwear', 'Wellington Boots'),
        ('Footwear', 'Sandals'),
        ('Electronics', 'Laptop'),
        ('Electronics', 'Laptop Charger'),
        ('Electronics', 'Phone'),
        ('Electronics', 'Phone Charger'),
        ('Electronics', 'Phone Charging Cable'),
        ('Electronics', 'Portable Charger'),
        ('Electronics', 'Headphones'),
        ('Documents', 'Passport'),
        ('Documents', 'ID'),
        ('Medical', 'Paracetamol'),
        ('Medical', 'Ibuprofen'),
        ('Medical', 'Berocca'),
        ('Food & Drink', 'Beer'),
        ('Food & Drink', 'Wine'),
        ('Camping Gear', 'Tent'),
        ('Camping Gear', 'Sleeping Bag'),
        ('Camping Gear', 'Air Mattress'),
        ('Camping Gear', 'Rug'),
        ('Camping Gear', 'Pillow')
) AS v (category_name, item_name)
    JOIN categories c ON c.name = v.category_name AND c.user_id IS NULL
ON CONFLICT (category_id, LOWER(name))
WHERE
    user_id IS NULL
DO NOTHING;
