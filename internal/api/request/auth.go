package request

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutPayload struct {
	RefreshToken string `json:"refresh_token"`
}
