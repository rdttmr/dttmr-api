package response

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// TODO: Move to own file
func Status(ctx context.Context, w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func JSON(ctx context.Context, w http.ResponseWriter, status int, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"error": "internal server error: failed to marshal response"}`))
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal json", slog.Any("error", err))
			return
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(payload)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write response", slog.Any("error", err))
		return
	}
}

func Error(ctx context.Context, w http.ResponseWriter, status int, message string) {
	JSON(ctx, w, status, map[string]string{"error": message})
}
