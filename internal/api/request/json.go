package request

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
)

func DecodeJSON[T any](r *http.Request) (T, error) {
	var payload T

	if err := json.UnmarshalRead(r.Body, &payload, json.RejectUnknownMembers(true)); err != nil {
		return payload, fmt.Errorf("error decoding payload: %w", err)
	}
	return payload, nil
}
