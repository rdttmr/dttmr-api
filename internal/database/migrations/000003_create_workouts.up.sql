CREATE TABLE exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    equipment TEXT[] NOT NULL DEFAULT '{}'
        CHECK (
            equipment <@ ARRAY [
                'floor',
                'rings',
                'pull_up_bar',
                'dip_bars',
                'parallettes',
        ]::TEXT[]),
    metric TEXT NOT NULL DEFAULT 'reps'
        CHECK metric IN ('reps', 'seconds'),
    load TEXT NOT NULL DEFAULT 'bodyweight'
        CHECK load IN ('bodyweight', 'absolute'),
    tags TEXT[] NOT NULL DEFAULT '{}', -- push/pull/legs/core/skill/shoulders/...
    notes TEXT,
    hidden NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX exercises_name_key ON exercises (lower(name));
CREATE INDEX exercises_equipment_idx ON exercises USING gin (equipment);
CREATE INDEX exercises_tags_idx ON exercises USING gin (tags);


CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE template_exercises (
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL,
    target TEXT,

    PRIMARY KEY (template_id, exercise_id)
);

-- todo: workouts, sets
