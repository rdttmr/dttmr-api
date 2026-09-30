package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

func newExerciseRepo(t *testing.T) (*ExerciseRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &ExerciseRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	selectExercisesQuery = "SELECT id, name, equipment, metric, load, tags, notes, modified_at FROM exercises WHERE user_id IS NULL ORDER BY modified_at DESC OFFSET $1 LIMIT $2"
	countExercisesQuery  = "SELECT COUNT(*) FROM exercises WHERE user_id IS NULL"
)

var exerciseColumns = []string{"id", "name", "equipment", "metric", "load", "tags", "notes", "modified_at"}

func TestExerciseRepo_GetExercises(t *testing.T) {
	modifiedAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	note := "Keep elbows close"

	t.Run("maps rows and decodes enums and arrays", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		mock.ExpectQuery(selectExercisesQuery).
			WithArgs(10, 5).
			WillReturnRows(
				sqlmock.NewRows(exerciseColumns).
					AddRow("ex-1", "Push-up", "{floor,rings,parallettes}", "reps", "bodyweight", "{push,chest,triceps}", note, modifiedAt).
					AddRow("ex-2", "Lateral raise", "{floor}", "reps", "external", "{shoulders,isolation}", nil, modifiedAt).
					AddRow("ex-3", "Plank", "{}", "seconds", "bodyweight", "{}", nil, modifiedAt),
			)

		exercises, err := repo.GetExercises(context.Background(), 10, 5)

		require.NoError(t, err)
		assert.Equal(t, []domain.Exercise{
			{
				ID:         "ex-1",
				Name:       "Push-up",
				Equipment:  []domain.Equipment{domain.EquipmentFloor, domain.EquipmentRings, domain.EquipmentParallettes},
				Metric:     domain.MetricReps,
				Load:       domain.LoadBodyweight,
				Tags:       []string{"push", "chest", "triceps"},
				Notes:      &note,
				ModifiedAt: modifiedAt,
			},
			{
				ID:         "ex-2",
				Name:       "Lateral raise",
				Equipment:  []domain.Equipment{domain.EquipmentFloor},
				Metric:     domain.MetricReps,
				Load:       domain.LoadExternal,
				Tags:       []string{"shoulders", "isolation"},
				Notes:      nil,
				ModifiedAt: modifiedAt,
			},
			{
				ID:         "ex-3",
				Name:       "Plank",
				Equipment:  []domain.Equipment{},
				Metric:     domain.MetricSeconds,
				Load:       domain.LoadBodyweight,
				Tags:       []string{},
				Notes:      nil,
				ModifiedAt: modifiedAt,
			},
		}, exercises)
	})

	t.Run("no rows returns no exercises", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		mock.ExpectQuery(selectExercisesQuery).
			WithArgs(0, 10).
			WillReturnRows(sqlmock.NewRows(exerciseColumns))

		exercises, err := repo.GetExercises(context.Background(), 0, 10)

		require.NoError(t, err)
		assert.Empty(t, exercises)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectExercisesQuery).
			WithArgs(0, 10).
			WillReturnError(dbErr)

		exercises, err := repo.GetExercises(context.Background(), 0, 10)

		assert.Nil(t, exercises)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get exercises")
	})

	scanErrors := map[string][]driver.Value{
		"unknown equipment": {"ex-1", "Push-up", "{floor,trampoline}", "reps", "bodyweight", "{push}", nil, modifiedAt},
		"unknown metric":    {"ex-1", "Push-up", "{floor}", "meters", "bodyweight", "{push}", nil, modifiedAt},
		"unknown load":      {"ex-1", "Push-up", "{floor}", "reps", "absolute", "{push}", nil, modifiedAt},
		"malformed tags":    {"ex-1", "Push-up", "{floor}", "reps", "bodyweight", "push", nil, modifiedAt},
	}
	for name, row := range scanErrors {
		t.Run("scan error on "+name, func(t *testing.T) {
			repo, mock := newExerciseRepo(t)

			mock.ExpectQuery(selectExercisesQuery).
				WithArgs(0, 10).
				WillReturnRows(sqlmock.NewRows(exerciseColumns).AddRow(row...))

			exercises, err := repo.GetExercises(context.Background(), 0, 10)

			assert.Nil(t, exercises)
			assert.Error(t, err)
		})
	}

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectExercisesQuery).
			WithArgs(0, 10).
			WillReturnRows(
				sqlmock.NewRows(exerciseColumns).
					AddRow("ex-1", "Push-up", "{floor}", "reps", "bodyweight", "{push}", nil, modifiedAt).
					AddRow("ex-2", "Plank", "{floor}", "seconds", "bodyweight", "{core}", nil, modifiedAt).
					RowError(1, rowErr),
			)

		exercises, err := repo.GetExercises(context.Background(), 0, 10)

		assert.Nil(t, exercises)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get exercises")
	})
}

func TestExerciseRepo_CountExercises(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		mock.ExpectQuery(countExercisesQuery).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

		count, err := repo.CountExercises(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 25, count)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newExerciseRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(countExercisesQuery).
			WillReturnError(dbErr)

		count, err := repo.CountExercises(context.Background())

		assert.Zero(t, count)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to count exercises")
	})
}
