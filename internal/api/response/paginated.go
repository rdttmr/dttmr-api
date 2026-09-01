package response

type Paginated[T any] struct {
	Count int `json:"count"`
	Total int `json:"total"`
	Data  []T `json:"data"`
}
