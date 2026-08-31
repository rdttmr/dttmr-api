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
	AuthService *domain.AuthService
}

func NewUserHandler(userService *domain.UserService, authService *domain.AuthService) *UserHandler {
	return &UserHandler{UserService: userService, AuthService: authService}
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
// @Router /users [post]
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

// ChangePassword handles changing a users password
//
// @Summary Change password route
// @Description Change password for the authenticated user
// @Tags User
// @Accept json
// @Produce json
// @Param payload body request.ChangePasswordPayload true "Change password payload"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "could not authenticate with current password"
// @Error 500 {object} response.ErrorResponse "could not get auth context"
// @Error 500 {object} response.ErrorResponse "failed to change password"
// @Router /users/password [post]
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.ChangePasswordPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode change password payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "could not get auth context")
		return
	}

	_, err = h.AuthService.Authenticate(ctx, authContext.Email, payload.OldPassword)
	if err != nil {
		slog.ErrorContext(ctx, "failed to authenticate with current password", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "could not authenticate with current password")
		return
	}

	err = h.UserService.ChangePassword(ctx, authContext.UserID, payload.NewPassword)
	if err != nil {
		slog.ErrorContext(ctx, "failed to change password", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to change password")
		return
	}

	err = h.AuthService.LogoutAllDevices(ctx, authContext.UserID)
	if err != nil {
		slog.WarnContext(ctx, "failed to logout user from all devices after password change", slog.Any("error", err))
	}

	slog.InfoContext(ctx, "password changed successfully", slog.Any("user_id", authContext.UserID))
	response.Status(w, http.StatusNoContent)
}
