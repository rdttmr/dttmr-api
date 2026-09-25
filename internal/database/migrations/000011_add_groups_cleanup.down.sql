UPDATE groups g SET created_by = (
    SELECT gm.user_id
    FROM group_members gm
    WHERE gm.group_id = g.id
    ORDER BY (gm.role = 'owner') DESC, gm.created_at, gm.user_id
    LIMIT 1
) WHERE g.created_by IS NULL;

ALTER TABLE groups
    DROP CONSTRAINT groups_created_by_fkey;
ALTER TABLE groups
    ALTER COLUMN created_by SET NOT NULL;
ALTER TABLE groups
    ADD CONSTRAINT groups_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id);
