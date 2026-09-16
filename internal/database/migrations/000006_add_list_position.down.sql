DROP INDEX idx_list_users_position;

ALTER TABLE list_users
    DROP COLUMN IF EXISTS position;
