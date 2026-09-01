package handler

import (
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/response"
)

type Version struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

func VersionHandler(version string, commit string, buildTime string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		response.JSON(r.Context(), w, http.StatusOK, Version{
			Version:   version,
			Commit:    commit,
			BuildTime: buildTime,
		})
	}
}
