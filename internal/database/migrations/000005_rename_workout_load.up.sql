BEGIN;

ALTER TABLE exercises DROP CONSTRAINT exercises_load_check;

UPDATE exercises SET load = 'external' WHERE load = 'absolute';

ALTER TABLE exercises ADD CONSTRAINT exercises_load_check
    CHECK (load IN ('bodyweight', 'external'));

COMMIT;
