CREATE TABLE exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    equipment TEXT[] NOT NULL DEFAULT '{}'
        CHECK (
            equipment <@ ARRAY [
                'floor',
                'rings',
                'pull_up_bar',
                'parallel_bars',
                'low_bar',
                'parallettes',
                'resistance_band'
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


CREATE TABLE workouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    template_id UUID REFERENCES templates(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    rpe smallint CHECK (rpe BETWEEN 1 AND 10),
    bodyweight_kg NUMERIC(5,2),
    notes TEXT
);

CREATE INDEX workouts_time_idx ON workouts (started_at DESC);


CREATE TABLE workout_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_id UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reps SMALLINT,
    seconds SMALLINT,
    weight_kg NUMERIC(5,2),
    note TEXT,

    CONSTRAINT set_has_number CHECK (reps IS NOT NULL OR seconds IS NOT NULL)
);

CREATE INDEX sets_workout_idx ON workout_sets (workout_id, logged_at);
CREATE INDEX sets_exercise_ifx ON workout_sets (exercise_id, logged_at DESC);

INSERT INTO exercises (name, metric, load, tags, equipment) VALUES
    ('Push-up',              'reps',    'bodyweight', '{push,chest,triceps}', '{floor,rings,parallettes}'),
    ('Diamond push-up',      'reps',    'bodyweight', '{push,triceps}', '{floor,rings,parallettes}'),
    ('Pseudo planche push-up','reps',   'bodyweight', '{push,shoulders,skill}', '{floor,rings,parallettes}'),
    ('Pike push-up',         'reps',    'bodyweight', '{push,shoulders}', '{floor,rings,parallettes}'),
    ('Dip',                  'reps',    'bodyweight', '{push,chest,triceps}', '{rings,parallel_bars}'),
    ('Handstand hold',       'seconds', 'bodyweight', '{push,shoulders,skill}', '{floor,rings,parallettes}'),
    ('Handstand Push-up',       'reps', 'bodyweight', '{push,shoulders,skill}', '{floor,rings,parallettes}'),
    ('Pull-up',              'reps',    'bodyweight', '{pull,back,biceps}', '{rings,pull_up_bar}'),
    ('Chin-up',              'reps',    'bodyweight', '{pull,back,biceps}', '{rings,pull_up_bar}'),
    ('Inverted row',         'reps',    'bodyweight', '{pull,back}', '{rings,parallel_bars,low_bar}'),
    ('Muscle-up',            'reps',    'bodyweight', '{pull,push,skill}', '{rings,pull_up_bar}'),
    ('Dead hang',            'seconds', 'bodyweight', '{pull,grip}', '{rings,pull_up_bar}'),
    ('Front lever hold',     'seconds', 'bodyweight', '{pull,core,skill}', '{rings,pull_up_bar,low_bar}'),
    ('Pistol squat',         'reps',    'bodyweight', '{legs}', '{floor}'),
    ('Bulgarian split squat','reps',    'bodyweight', '{legs}', '{floor}'),
    ('Nordic curl',          'reps',    'bodyweight', '{legs,hamstrings}', '{floor}'),
    ('Calf raise',           'reps',    'bodyweight', '{legs}', '{floor}'),
    ('Plank',                'seconds', 'bodyweight', '{core}', '{floor}'),
    ('Hollow body hold',     'seconds', 'bodyweight', '{core}', '{floor}'),
    ('L-sit',                'seconds', 'bodyweight', '{core,skill}', '{floor,parallettes,parallel_bars}'),
    ('Hanging leg raise',    'reps',    'bodyweight', '{core}', '{rings,pull_up_bar}'),
    ('Face pull',            'reps',    'bodyweight', '{pull,shoulders}', '{rings,resistance_band}'),
    ('Lateral raise',        'reps',    'absolute',   '{shoulders,isolation}', '{floor}'),
