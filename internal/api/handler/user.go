package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/request"
	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type UserHandler struct {
	UserService         *domain.UserService
	AuthService         *domain.AuthService
	RegistrationService *domain.RegistrationService
}

func NewUserHandler(userService *domain.UserService, authService *domain.AuthService, registrationService *domain.RegistrationService) *UserHandler {
	return &UserHandler{UserService: userService, AuthService: authService, RegistrationService: registrationService}
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
// @Error 400 {object} response.ErrorResponse "invite is expired"
// @Error 409 {object} response.ErrorResponse "invite is already consumed"
// @Error 400 {object} response.ErrorResponse "invite is invalid"
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

	user, err := h.RegistrationService.Register(ctx, payload.InviteCode, payload.Email, payload.Name, payload.Password)
	if err != nil {
		slog.ErrorContext(ctx, "failed to register user",
			slog.Any("error", err),
			slog.Any("payload", payload))

		if errors.Is(err, domain.ErrInviteExpired) {
			response.Error(ctx, w, http.StatusBadRequest, "invite is expired")
		} else if errors.Is(err, domain.ErrInviteConsumed) {
			response.Error(ctx, w, http.StatusConflict, "invite is already consumed")
		} else if errors.Is(err, domain.ErrInviteInvalid) {
			response.Error(ctx, w, http.StatusBadRequest, "invite is invalid")
		} else {
			response.Error(ctx, w, http.StatusInternalServerError, "failed to register")
		}
		return
	}

	slog.InfoContext(ctx, "created user successfully",
		slog.String("user_id", user.ID),
	)
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
// @Router /user/password [post]
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
