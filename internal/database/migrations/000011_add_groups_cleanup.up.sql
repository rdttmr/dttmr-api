DELETE FROM recipe_items ri
    USING recipes r, list_items li, lists l
WHERE ri.recipe_id = r.id
    AND ri.list_item_id = li.id
    AND li.list_id = l.id
    AND l.group_id <> r.group_id;


ALTER TABLE groups
    DROP CONSTRAINT groups_created_by_fkey;
ALTER TABLE groups
    ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE groups
    ADD CONSTRAINT groups_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL;


