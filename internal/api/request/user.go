package request

type CreateUserPayload struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

type ChangePasswordPayload struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}
