package domain

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrListIDEmpty        = errors.New("list id must not be empty")
	ErrListNameEmpty      = errors.New("list name must not be empty")
	ErrUserIDEmpty        = errors.New("user id must not be empty")
	ErrUserIDsEmpty       = errors.New("user ids must not be empty")
	ErrListItemIDEmpty    = errors.New("list item id must not be empty")
	ErrListItemTitleEmpty = errors.New("list item title must not be empty")
)

type List struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ListItem struct {
	ID          string    `json:"id"`
	ListID      string    `json:"list_id"`
	Title       string    `json:"title"`
	IsCompleted bool      `json:"is_completed"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

type ListRepository interface {
	CreateList(ctx context.Context, name string, userIDs []string) (*List, error)
	AddUserToList(ctx context.Context, listID string, userID string) error
	RemoveUserFromList(ctx context.Context, listID string, userID string) error
	IsUserInList(ctx context.Context, listID string, userID string) (bool, error)
	IsUserInListByListItem(ctx context.Context, listItemID string, userID string) (bool, error)
	CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error)
	UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error
}

type ListService struct {
	repo ListRepository
}

func NewListService(r ListRepository) *ListService {
	return &ListService{repo: r}
}

func (s *ListService) Create(ctx context.Context, authUserID string, name string, userIDs []string) (*List, error) {
	if len(userIDs) == 0 {
		return nil, ErrUserIDsEmpty
	}

	if name == "" {
		return nil, ErrListNameEmpty
	}

	if !slices.Contains(userIDs, authUserID) {
		userIDs = append(userIDs, authUserID)
	}

	return s.repo.CreateList(ctx, name, userIDs)
}

func (s *ListService) AddUserToList(ctx context.Context, authUserID string, listID string, userID string) error {
	if listID == "" {
		return ErrListIDEmpty
	}
	if userID == "" {
		return ErrUserIDEmpty
	}

	return s.repo.AddUserToList(ctx, listID, userID)
}

func (s *ListService) RemoveUserFromList(ctx context.Context, authUserID string, listID string, userID string) error {
	if listID == "" {
		return ErrListIDEmpty
	}
	if userID == "" {
		return ErrUserIDEmpty
	}

	return s.repo.RemoveUserFromList(ctx, listID, userID)
}

func (s *ListService) CreateListItem(ctx context.Context, authUserID string, listID string, title string) (*ListItem, error) {
	if listID == "" {
		return nil, ErrListIDEmpty
	}
	if title == "" {
		return nil, ErrListItemTitleEmpty
	}

	return s.repo.CreateListItem(ctx, listID, title)
}

func (s *ListService) UpdateListItem(ctx context.Context, authUserID string, listItemID string, title string, isCompleted bool) error {
	if listItemID == "" {
		return ErrListItemIDEmpty
	}
	if title == "" {
		return ErrListItemTitleEmpty
	}

	return s.repo.UpdateListItem(ctx, listItemID, title, isCompleted)
}
