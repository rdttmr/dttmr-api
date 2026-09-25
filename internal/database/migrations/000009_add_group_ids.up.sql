ALTER TABLE lists
    ADD COLUMN group_id UUID REFERENCES groups(id) ON DELETE RESTRICT;

ALTER TABLE recipes
    ADD COLUMN group_id UUID REFERENCES groups(id) ON DELETE RESTRICT;

-- create default/personal group for every existing user
WITH new_groups AS (
    INSERT INTO groups (id, name, created_by)
        SELECT gen_random_uuid(), 'Personal', u.id
        FROM users u
        WHERE NOT EXISTS (
            SELECT 1 FROM group_members gm
            WHERE gm.user_id = u.id AND gm.is_default
        )
        RETURNING id, created_by
)
INSERT INTO group_members (group_id, user_id, role, is_default)
SELECT id, created_by, 'owner', TRUE
FROM new_groups;

-- set group_id for existing lists (earliest created_at is "owner")
UPDATE lists l
SET group_id = gm.group_id
FROM (
    SELECT DISTINCT ON (list_id) list_id, user_id
    FROM list_users
    ORDER BY list_id, created_at, user_id
) o
    JOIN group_members gm ON gm.user_id = o.user_id AND gm.is_default
WHERE l.id = o.list_id;

-- set group_id for existing recipes (earliest created_at is "owner")
UPDATE recipes r
SET group_id = gm.group_id
FROM (
    SELECT DISTINCT ON (recipe_id) recipe_id, user_id
    FROM recipe_users
    ORDER BY recipe_id, created_at, user_id
) o
    JOIN group_members gm ON gm.user_id = o.user_id AND gm.is_default
WHERE r.id = o.recipe_id;

ALTER TABLE lists
    ALTER COLUMN group_id SET NOT NULL;
CREATE INDEX idx_lists_group_id ON lists (group_id);

ALTER TABLE recipes
    ALTER COLUMN group_id SET NOT NULL;
CREATE INDEX idx_recipes_group_id ON recipes (group_id);

CREATE TABLE list_positions (
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (list_id, user_id)
);
CREATE INDEX idx_list_positions_user_id_position ON list_positions (user_id, position);

INSERT INTO list_positions (list_id, user_id, position)
    SELECT list_id, user_id, position FROM list_users;

CREATE TABLE recipe_positions (
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (recipe_id, user_id)
);
CREATE INDEX idx_recipe_positions_user_id_position ON recipe_positions (user_id, position);

INSERT INTO recipe_positions (recipe_id, user_id, position)
    SELECT recipe_id, user_id, position FROM recipe_users;
