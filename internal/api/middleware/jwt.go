package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

func WithJWT(authService *domain.AuthService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				slog.ErrorContext(ctx, "missing authorization header")
				response.Error(ctx, w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				slog.ErrorContext(ctx, "invalid authorization header")
				response.Error(ctx, w, http.StatusUnauthorized, "invalid authorization header")
				return
			}

			authContext, err := authService.ParseAccessToken(ctx, parts[1])
			if err != nil {
				slog.ErrorContext(ctx, "invalid token", slog.Any("error", err))
				response.Error(ctx, w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx = context.WithValue(ctx, domain.AuthContextKey, authContext)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}
