BEGIN;

ALTER TABLE exercises DROP CONSTRAINT exercises_load_check;

UPDATE exercises SET load = 'absolute' WHERE load = 'external';

ALTER TABLE exercises ADD CONSTRAINT exercises_load_check
    CHECK (load IN ('bodyweight', 'absolute'));

COMMIT;
