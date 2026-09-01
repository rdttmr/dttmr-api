package handler

import (
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/response"
)

type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

// VersionHandler handles the version route
//
// @Summary Service version
// @Description Reports the version of the API
// @Tags Version
// @Accept json
// @Produce json
// @Success 200 {object} VersionResponse
// @Router /version [get]
func VersionHandler(version string, commit string, buildTime string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		response.JSON(r.Context(), w, http.StatusOK, VersionResponse{
			Version:   version,
			Commit:    commit,
			BuildTime: buildTime,
		})
	}
}
