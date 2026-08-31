package response

import (
	"net/http"
)

func Status(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}
