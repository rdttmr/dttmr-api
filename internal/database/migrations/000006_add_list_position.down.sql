BEGIN;

DROP INDEX IF EXISTS idx_list_users_position;

ALTER TABLE IF EXISTS list_users
    DROP COLUMN IF EXISTS position;

COMMIT;
