package request

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func DecodeJSON[T any](r *http.Request) (T, error) {
	var payload T

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("error decoding payload: %w", err)
	}

	return payload, nil
}
