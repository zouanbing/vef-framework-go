package service

// PagedResult is a generic paginated result.
type PagedResult[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
}
