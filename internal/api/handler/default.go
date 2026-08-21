package handler

import (
	"log/slog"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/response"
)

type apiResponse struct {
	Method     string            `json:"method"`
	Url        string            `json:"url"`
	Proto      string            `json:"proto"`
	Header     map[string]string `json:"header"`
	Host       string            `json:"host"`
	RemoteAddr string            `json:"remoteAddr"`
	Form       map[string]string `json:"form"`
}

// DefaultHandler handles the default route
//
// @Summary Default route handler
// @Description Default route handler
// @Tags
// @Accept json
// @Produce json
// @Success 200 {object} apiResponse
// @Router / [get]
func DefaultHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := apiResponse{
		Method:     r.Method,
		Url:        r.URL.String(),
		Proto:      r.Proto,
		Header:     make(map[string]string),
		Host:       r.Host,
		RemoteAddr: r.RemoteAddr,
		Form:       make(map[string]string),
	}

	for k, v := range r.Header {
		resp.Header[k] = v[0]
	}

	if err := r.ParseForm(); err == nil {
		for k, v := range r.Form {
			resp.Form[k] = v[0]
		}
	} else {
		slog.ErrorContext(ctx, "error parsing form", slog.Any("error", err))
	}

	response.JSON(ctx, w, http.StatusOK, resp)
}
