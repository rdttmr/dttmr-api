package domain

import (
	"context"
	"errors"
	"time"
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
	CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error)
	UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error
}

type ListService struct {
	repo ListRepository
}

func NewListService(r ListRepository) *ListService {
	return &ListService{repo: r}
}

func (s *ListService) Create(ctx context.Context, name string, userIDs []string) (*List, error) {
	if len(userIDs) == 0 {
		return nil, errors.New("users must have at least one associated user")
	}

	if name == "" {
		return nil, errors.New("list name must not be empty")
	}

	return s.repo.CreateList(ctx, name, userIDs)
}

func (s *ListService) AddUserToList(ctx context.Context, listID string, userID string) error {
	if listID == "" {
		return errors.New("list id must not be empty")
	}
	if userID == "" {
		return errors.New("user id must not be empty")
	}

	return s.repo.AddUserToList(ctx, listID, userID)
}

func (s *ListService) RemoveUserFromList(ctx context.Context, listID string, userID string) error {
	if listID == "" {
		return errors.New("list id must not be empty")
	}
	if userID == "" {
		return errors.New("user id must not be empty")
	}

	return s.repo.RemoveUserFromList(ctx, listID, userID)
}

func (s *ListService) CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error) {
	if listID == "" {
		return nil, errors.New("list id must not be empty")
	}
	if title == "" {
		return nil, errors.New("list item title must not be empty")
	}

	return s.repo.CreateListItem(ctx, listID, title)
}

func (s *ListService) UpdateListItem(ctx context.Context, listItemID, title string, isCompleted bool) error {
	if listItemID == "" {
		return errors.New("list item id must not be empty")
	}
	if title == "" {
		return errors.New("list item title must not be empty")
	}

	return s.repo.UpdateListItem(ctx, listItemID, title, isCompleted)
}
