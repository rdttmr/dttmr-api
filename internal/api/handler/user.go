package handler

import (
	"log/slog"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/request"
	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type UserHandler struct {
	UserService *domain.UserService
}

func NewUserHandler(userService *domain.UserService) *UserHandler {
	return &UserHandler{UserService: userService}
}

// CreateUser handles the creation of a user
//
// @Summary Create user route
// @Description Create a user
// @Tags User
// @Accept json
// @Produce json
// @Param payload body request.CreateUserPayload true "Create user payload"
// @Success 201 {object} domain.User
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to create user"
// @Router /api/v1/user [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateUserPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create user payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	user, err := h.UserService.CreateUser(ctx, payload.Email, payload.Name, payload.Password)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create user", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create user")
		return
	}

	slog.InfoContext(ctx, "created user successfully", slog.Any("user_id", user.ID))
	response.JSON(ctx, w, http.StatusCreated, user)
}
