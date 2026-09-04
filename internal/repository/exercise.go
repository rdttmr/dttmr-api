package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

var m = pgtype.NewMap()

type ExerciseRepo struct {
	Repo
}

func (r *ExerciseRepo) CreateExercise(ctx context.Context) (*domain.Exercise, error) {
	_ = ctx
	return nil, nil
}

func (r *ExerciseRepo) DeleteExercise(ctx context.Context, id string) error {
	_, _ = ctx, id
	return nil
}

func (r *ExerciseRepo) GetExercises(ctx context.Context, offset int, count int) ([]domain.Exercise, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT id, name, equipment, metric, load, tags, notes, modified_at FROM exercises WHERE user_id IS NULL ORDER BY modified_at DESC OFFSET $1 LIMIT $2",
		offset, count,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get exercises: %w", err)
	}
	defer rows.Close()

	exercises := make([]domain.Exercise, 0, count)
	for rows.Next() {
		var e domain.Exercise
		err = rows.Scan(&e.ID, &e.Name, (*domain.EquipmentSet)(&e.Equipment), &e.Metric, &e.Load, m.SQLScanner(&e.Tags), &e.Notes, &e.ModifiedAt)
		if err != nil {
			return nil, err
		}

		exercises = append(exercises, e)
	}

	return exercises, nil
}

func (r *ExerciseRepo) CountExercises(ctx context.Context) (int, error) {
	var count int
	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM exercises",
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count exercises: %w", err)
	}

	return count, nil
}
