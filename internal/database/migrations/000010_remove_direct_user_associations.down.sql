CREATE TABLE list_users (
    list_id UUID NOT NULL REFERENCES lists ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    position BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (list_id, user_id)
);

CREATE UNIQUE INDEX idx_list_users_list_id_user_id ON list_users (list_id, user_id);
CREATE INDEX idx_list_users_user_id ON list_users (user_id);
CREATE INDEX idx_list_users_user_id_position ON list_users (user_id, position);


CREATE TABLE public.recipe_users (
    recipe_id  UUID REFERENCES recipes ON DELETE CASCADE,
    user_id UUID REFERENCES users ON DELETE CASCADE,
    position BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_recipe_users_recipe_id_user_id ON recipe_users (recipe_id, user_id);
CREATE INDEX idx_recipe_users_user_id ON recipe_users (user_id);
CREATE INDEX idx_recipe_users_user_id_position ON recipe_users (user_id, position);


CREATE TABLE public.recipe_invites (
    recipe_id UUID PRIMARY KEY REFERENCES recipes ON DELETE CASCADE,
    code_hash  VARCHAR(64) NOT NULL UNIQUE,
    created_by UUID REFERENCES users,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipe_invites_code_hash ON recipe_invites (code_hash);


