package request

type CreateListPayload struct {
	Name string `json:"name"`
}

type AddUserToListPayload struct {
	ListID string `json:"list_id"`
	Email  string `json:"email"`
}

type RemoveUserFromListPayload struct {
	ListID string `json:"list_id"`
	Email  string `json:"email"`
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

type SetListItemCompletedPayload struct {
	IsCompleted bool `json:"is_completed"`
}
