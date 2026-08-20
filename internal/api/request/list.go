package request

type CreateListPayload struct {
	Name    string   `json:"name"`
	UserIDs []string `json:"user_ids"`
}

type AddUserToListPayload struct {
	ListID string `json:"list_id"`
	UserID string `json:"user_id"`
}

type RemoveUserFromListPayload struct {
	ListID string `json:"list_id"`
	UserID string `json:"user_id"`
}

type CreateListItemPayload struct {
	ListID string `json:"list_id"`
	Title  string `json:"title"`
}

type UpdateListItemPayload struct {
	ListItemID  string `json:"list_item_id"`
	Title       string `json:"title"`
	IsCompleted bool   `json:"is_completed"`
}
