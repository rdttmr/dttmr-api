INSERT INTO list_users (list_id, user_id, position)
    SELECT l.id, gm.user_id, COALESCE(lp.position, 0)
    FROM lists l
        JOIN group_members gm ON gm.group_id = l.group_id
        LEFT JOIN list_positions lp ON lp.list_id = l.id AND lp.user_id = gm.user_id
ON CONFLICT (list_id, user_id) DO NOTHING;

INSERT INTO recipe_users (recipe_id, user_id, position)
    SELECT r.id, gm.user_id, COALESCE(rp.position, 0)
    FROM recipes r
        JOIN group_members gm ON gm.group_id = r.group_id
        LEFT JOIN recipe_positions rp ON rp.recipe_id = r.id AND rp.user_id = gm.user_id
ON CONFLICT (recipe_id, user_id) DO NOTHING;

UPDATE list_users lu
    SET position = lp.position
FROM list_positions lp
    WHERE lp.list_id = lu.list_id AND lp.user_id = lu.user_id;

UPDATE recipe_users ru
    SET position = rp.position
FROM recipe_positions rp
    WHERE rp.recipe_id = ru.recipe_id AND rp.user_id = ru.user_id;

DROP TABLE recipe_positions;
DROP TABLE list_positions;

ALTER TABLE recipes
    DROP COLUMN group_id;
ALTER TABLE lists
    DROP COLUMN group_id;