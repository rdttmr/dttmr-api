package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var _ ExerciseRepository = (*mockExerciseRepository)(nil)

type mockExerciseRepository struct {
	mock.Mock
}

func (m *mockExerciseRepository) CreateExercise(ctx context.Context) (*Exercise, error) {
	args := m.Called(ctx)
	exercise, _ := args.Get(0).(*Exercise)
	return exercise, args.Error(1)
}

func (m *mockExerciseRepository) DeleteExercise(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockExerciseRepository) GetExercises(ctx context.Context, offset int, count int) ([]Exercise, error) {
	args := m.Called(ctx, offset, count)
	exercises, _ := args.Get(0).([]Exercise)
	return exercises, args.Error(1)
}

func (m *mockExerciseRepository) CountExercises(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func newExerciseService(t *testing.T) (*ExerciseService, *mockExerciseRepository) {
	t.Helper()

	repo := &mockExerciseRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	return NewExerciseService(repo), repo
}

func TestExerciseService_GetExercises(t *testing.T) {
	ctx := context.Background()

	pages := []struct {
		page       int
		count      int
		wantOffset int
	}{
		{page: 1, count: 10, wantOffset: 0},
		{page: 2, count: 10, wantOffset: 10},
		{page: 3, count: 25, wantOffset: 50},
	}
	for _, tt := range pages {
		t.Run(fmt.Sprintf("page %d with %d per page starts at offset %d", tt.page, tt.count, tt.wantOffset), func(t *testing.T) {
			svc, repo := newExerciseService(t)

			found := []Exercise{{ID: "ex-1", Name: "Push-up", Metric: MetricReps, Load: LoadBodyweight}}
			repo.On("GetExercises", mock.Anything, tt.wantOffset, tt.count).Return(found, nil)

			exercises, err := svc.GetExercises(ctx, tt.page, tt.count)

			require.NoError(t, err)
			assert.Equal(t, found, exercises)
		})
	}

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo := newExerciseService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetExercises", mock.Anything, 0, 10).Return(nil, repoErr)

		exercises, err := svc.GetExercises(ctx, 1, 10)

		assert.Nil(t, exercises)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestExerciseService_CountExercises(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the count", func(t *testing.T) {
		svc, repo := newExerciseService(t)

		repo.On("CountExercises", mock.Anything).Return(25, nil)

		count, err := svc.CountExercises(ctx)

		require.NoError(t, err)
		assert.Equal(t, 25, count)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo := newExerciseService(t)

		repoErr := errors.New("connection reset")
		repo.On("CountExercises", mock.Anything).Return(0, repoErr)

		count, err := svc.CountExercises(ctx)

		assert.Zero(t, count)
		assert.ErrorIs(t, err, repoErr)
	})
}
