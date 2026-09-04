package request

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

var (
	ErrFailedToDecodeRequestQuery = errors.New("failed to decode request query")
	ErrInvalidPageValue           = errors.New("invalid value for page")
	ErrInvalidCountValue          = errors.New("invalid value for count")
)

func ParsePaginatedQueryParams(r *http.Request) (int, int, error) {
	ctx := r.Context()

	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	countStr := r.URL.Query().Get("count")
	if countStr == "" {
		countStr = "10"
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to read page from query",
			slog.String("page", pageStr))
		return 0, 0, ErrFailedToDecodeRequestQuery
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to read count from query",
			slog.String("count", countStr))
		return 0, 0, ErrFailedToDecodeRequestQuery
	}

	if page < 1 {
		slog.ErrorContext(ctx,
			"page parameter is invalid",
			slog.Int("page", page))
		return 0, 0, ErrInvalidPageValue
	}
	if count <= 0 {
		slog.ErrorContext(ctx,
			"count parameter is invalid",
			slog.Int("count", count))
		return 0, 0, ErrInvalidCountValue
	}

	return page, count, nil
}
