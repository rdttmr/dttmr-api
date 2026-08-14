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
// @Router /api/v1/login [post]
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
// @Router /api/v1/refresh [post]
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
