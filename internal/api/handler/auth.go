package handler

import (
	"log/slog"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/request"
	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type AuthHandler struct {
	AuthService *domain.AuthService
}

func NewAuthHandler(authService *domain.AuthService) *AuthHandler {
	return &AuthHandler{AuthService: authService}
}

// Login handles the login of a user
//
// @Summary Login route
// @Description User authorization and token issuing
// @Tags Authorization
// @Accept json
// @Produce json
// @Param payload body request.LoginPayload true "Login payload"
// @Success 200 {object} domain.TokenPair
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to login"
// @Router /login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.LoginPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode login payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	tokens, err := h.AuthService.Login(ctx, payload.Email, payload.Password)
	if err != nil {
		slog.ErrorContext(ctx, "failed to login", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to login")
		return
	}

	response.JSON(ctx, w, http.StatusOK, tokens)
}

// Refresh handles refreshing an access token
//
// @Summary Refresh route
// @Description Token issuing with refresh token
// @Tags Authorization
// @Accept json
// @Produce json
// @Param payload body request.RefreshPayload true "Refresh payload"
// @Success 200 {object} domain.TokenPair
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to refresh token"
// @Router /login/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.RefreshPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode refresh payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	tokens, err := h.AuthService.Refresh(ctx, payload.RefreshToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to refresh token", slog.Any("error", err))
		// TODO: Add error definition to repository, respond with Unauthorized when token is invalid
		response.Error(ctx, w, http.StatusInternalServerError, "failed to refresh token")
		return
	}

	response.JSON(ctx, w, http.StatusOK, tokens)
}

// Logout handles logging out a user
//
// @Summary Logout route
// @Description Logout current user
// @Tags Authorization
// @Accept json
// @Produce json
// @Param payload body request.LogoutPayload true "Logout payload"
// @Success 200 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to logout"
// @Router /logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.LogoutPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode logout payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	err = h.AuthService.Logout(ctx, payload.RefreshToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to logout", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to logout")
		return
	}

	response.JSON(ctx, w, http.StatusOK, nil)
}

// LogoutAllDevices handles logging out a user on all devices
//
// @Summary Logout all route
// @Description Logout user from all devices (revokes all refresh tokens)
// @Tags Authorization
// @Accept json
// @Produce json
// @Success 200 {object} nil
// @Error 401 {object} response.ErrorResponse "failed to get auth context"
// @Error 500 {object} response.ErrorResponse "failed to logout"
// @Router /logout/all [post]
func (h *AuthHandler) LogoutAllDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "failed to get auth context")
		return
	}

	err = h.AuthService.LogoutAllDevices(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to logout", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to logout")
		return
	}

	response.JSON(ctx, w, http.StatusOK, nil)
}
