CREATE TABLE recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE recipe_items (
    recipe_id UUID REFERENCES recipes (id) ON DELETE CASCADE,
    list_item_id UUID REFERENCES list_items (id) ON DELETE CASCADE,
    position BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (recipe_id, list_item_id)
);

CREATE UNIQUE INDEX idx_recipe_items_recipe_id_list_item_id ON recipe_items(recipe_id, list_item_id);
CREATE INDEX idx_recipe_items_recipe_id ON recipe_items(recipe_id);

CREATE TABLE recipe_users (
    recipe_id UUID REFERENCES recipes (id) ON DELETE CASCADE,
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    position BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_recipe_users_recipe_id_user_id ON recipe_users(recipe_id, user_id);
CREATE INDEX idx_recipe_users_user_id ON recipe_users(user_id);

CREATE TABLE recipe_invites (
    recipe_id UUID PRIMARY KEY REFERENCES recipes (id) ON DELETE CASCADE,
    code_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipe_invites_code_hash ON recipe_invites (code_hash);
