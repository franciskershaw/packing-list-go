CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories (user_id);

CREATE INDEX IF NOT EXISTS idx_items_category_id ON items (category_id);

CREATE INDEX IF NOT EXISTS idx_items_user_id ON items (user_id);

CREATE INDEX IF NOT EXISTS idx_templates_user_id ON templates (user_id);

CREATE INDEX IF NOT EXISTS idx_template_items_template_id ON template_items (template_id);

CREATE INDEX IF NOT EXISTS idx_template_items_item_id ON template_items (item_id);

CREATE INDEX IF NOT EXISTS idx_packing_lists_user_id ON packing_lists (user_id);

CREATE INDEX IF NOT EXISTS idx_packing_lists_template_id ON packing_lists (template_id);

CREATE INDEX IF NOT EXISTS idx_packing_list_items_list_id ON packing_list_items (list_id);

CREATE INDEX IF NOT EXISTS idx_packing_list_items_item_id ON packing_list_items (item_id);

CREATE INDEX IF NOT EXISTS idx_packing_list_items_category_id ON packing_list_items (category_id);
