package request

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

func DecodeLogin(r *http.Request) (LoginPayload, error) {
	var payload LoginPayload

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("error decoding login payload: %w", err)
	}

	return payload, nil
}

func DecodeRefresh(r *http.Request) (RefreshPayload, error) {
	var payload RefreshPayload

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("error decoding refresh payload: %w", err)
	}

	return payload, nil
}
