CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE
    IF NOT EXISTS users (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        google_id TEXT UNIQUE NOT NULL,
        email TEXT UNIQUE NOT NULL,
        display_name TEXT,
        avatar_url TEXT,
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        last_login_at TIMESTAMPTZ
    );

CREATE TABLE
    IF NOT EXISTS categories (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID REFERENCES users (id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

CREATE TABLE
    IF NOT EXISTS items (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        category_id UUID NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
        user_id UUID REFERENCES users (id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

CREATE TABLE
    IF NOT EXISTS templates (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        description TEXT,
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

CREATE TABLE
    IF NOT EXISTS template_items (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        template_id UUID NOT NULL REFERENCES templates (id) ON DELETE CASCADE,
        item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
        quantity INT DEFAULT 1,
        notes TEXT
    );

CREATE TABLE
    IF NOT EXISTS packing_lists (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
        template_id UUID REFERENCES templates (id) ON DELETE SET NULL,
        name TEXT NOT NULL,
        event_date DATE,
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        archived_at TIMESTAMPTZ
    );

CREATE TABLE
    IF NOT EXISTS packing_list_items (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        list_id UUID NOT NULL REFERENCES packing_lists (id) ON DELETE CASCADE,
        item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
        category_id UUID NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
        quantity INT DEFAULT 1,
        is_packed BOOL DEFAULT false,
        notes TEXT,
        sort_order INT
    );