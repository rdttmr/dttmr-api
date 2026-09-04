package domain

import (
	"context"
	"time"
)

type ExerciseRepository interface {
	CreateExercise(ctx context.Context) (*Exercise, error)
	DeleteExercise(ctx context.Context, id string) error
	GetExercises(ctx context.Context, offset int, count int) ([]Exercise, error)
	CountExercises(ctx context.Context) (int, error)
}

type Exercise struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Equipment  []Equipment `json:"equipment"`
	Metric     Metric      `json:"metric"`
	Load       Load        `json:"load"`
	Tags       []string    `json:"tags"`
	Notes      *string     `json:"notes"`
	ModifiedAt time.Time   `json:"modified_at"`
}

type ExerciseService struct {
	repo ExerciseRepository
}

func NewExerciseService(r ExerciseRepository) *ExerciseService {
	return &ExerciseService{repo: r}
}

func (s *ExerciseService) GetExercises(ctx context.Context, page int, count int) ([]Exercise, error) {
	offset := (page - 1) * count
	return s.repo.GetExercises(ctx, offset, count)
}

func (s *ExerciseService) CountExercises(ctx context.Context) (int, error) {
	return s.repo.CountExercises(ctx)
}
