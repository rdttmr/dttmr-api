BEGIN;

ALTER TABLE IF EXISTS list_users
    ADD COLUMN IF NOT EXISTS position NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_list_users_user_id_position ON list_users (user_id, position);

UPDATE list_users lu
    SET position = r.rn - 1
FROM (SELECT list_id,
             user_id,
             row_number() OVER (
                 PARTITION BY user_id
                 ORDER BY created_at, list_id
                 ) AS rn
      FROM list_users) r
WHERE lu.list_id = r.list_id
  AND lu.user_id = r.user_id;

COMMIT;
