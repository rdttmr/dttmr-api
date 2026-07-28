package handler

import (
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/response"
)

type healthResponse struct {
	Status string `json:"status"`
}

// HealthHandler handles the health check route
//
// @Summary Health check
// @Description Health check reports the status of the API
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} healthResponse
// @Router /health [get]
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := healthResponse{
		Status: "ok",
	}
	response.JSON(ctx, w, http.StatusOK, resp)
}
