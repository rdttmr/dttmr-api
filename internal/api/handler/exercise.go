package handler

import (
	"log/slog"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/request"
	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type ExerciseHandler struct {
	ExerciseService *domain.ExerciseService
}

func NewExerciseHandler(exerciseService *domain.ExerciseService) *ExerciseHandler {
	return &ExerciseHandler{ExerciseService: exerciseService}
}

// GetExercises handles fetching the list of exercises
//
// @Summary Get exercises route
// @Description Gets a list of all exercises
// @Tags Exercise
// @Accept json
// @Produce json
// @Param page query int false "page"
// @Param count query int false "count"
// @Success 200 {object} response.Paginated[domain.Exercise]
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 400 {object} response.ErrorResponse "invalid value for page"
// @Error 400 {object} response.ErrorResponse "invalid value for count"
// @Error 500 {object} response.ErrorResponse "failed to get exercises"
// @Router /exercises [get]
func (h *ExerciseHandler) GetExercises(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	page, count, err := request.ParsePaginatedQueryParams(r)
	if err != nil {
		response.Error(ctx, w, http.StatusBadRequest, err.Error())
		return
	}

	exercises, err := h.ExerciseService.GetExercises(ctx, page, count)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get exercises", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get exercises")
		return
	}

	total, err := h.ExerciseService.CountExercises(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count exercises", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get exercises")
		return
	}

	response.JSON(ctx, w, http.StatusOK, response.Paginated[domain.Exercise]{
		Count: len(exercises),
		Total: total,
		Data:  exercises,
	})
}
