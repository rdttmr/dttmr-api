package request

type CreateGroupPayload struct {
	Name string `json:"name"`
}

type SetGroupNamePayload struct {
	Name string `json:"name"`
}

type JoinGroupPayload struct {
	Code string `json:"code"`
}
