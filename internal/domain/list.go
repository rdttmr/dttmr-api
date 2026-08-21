package domain

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"
)

var (
	ErrListIDEmpty        = errors.New("list id must not be empty")
	ErrListNameEmpty      = errors.New("list name must not be empty")
	ErrUserIDEmpty        = errors.New("user id must not be empty")
	ErrListItemIDEmpty    = errors.New("list item id must not be empty")
	ErrListItemTitleEmpty = errors.New("list item title must not be empty")
	ErrUserNotInList      = errors.New("user not in list")
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
	GetListsByUserID(ctx context.Context, userID string) ([]List, error)
	AddUserToList(ctx context.Context, listID string, userID string) error
	RemoveUserFromList(ctx context.Context, listID string, userID string) error
	IsUserInList(ctx context.Context, listID string, userID string) (bool, error)
	IsUserInListByItemID(ctx context.Context, listItemID string, userID string) (bool, error)
	CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error)
	UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error
	SetListItemCompleted(ctx context.Context, listItemID string, isCompleted bool) error
	GetListItems(ctx context.Context, listID string) ([]ListItem, error)
}

type ListService struct {
	repo ListRepository
}

func NewListService(r ListRepository) *ListService {
	return &ListService{repo: r}
}

func (s *ListService) Create(ctx context.Context, authUserID string, name string, userIDs []string) (*List, error) {
	if name == "" {
		return nil, ErrListNameEmpty
	}

	if !slices.Contains(userIDs, authUserID) {
		userIDs = append(userIDs, authUserID)
	}

	return s.repo.CreateList(ctx, name, userIDs)
}

func (s *ListService) GetLists(ctx context.Context, authUserID string) ([]List, error) {
	return s.repo.GetListsByUserID(ctx, authUserID)
}

func (s *ListService) AddUserToList(ctx context.Context, authUserID string, listID string, userID string) error {
	if listID == "" {
		return ErrListIDEmpty
	}
	if userID == "" {
		return ErrUserIDEmpty
	}

	inList, err := s.repo.IsUserInList(ctx, listID, authUserID)
	if err != nil {
		return err
	}
	if !inList {
		return ErrUserNotInList
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

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return err
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

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return nil, err
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

	if err := s.userAllowedToAccessList(ctx, authUserID, listItemID); err != nil {
		return err
	}

	return s.repo.UpdateListItem(ctx, listItemID, title, isCompleted)
}

func (s *ListService) SetListItemCompleted(ctx context.Context, authUserID string, listItemID string, isCompleted bool) error {
	if listItemID == "" {
		return ErrListItemIDEmpty
	}

	if err := s.userAllowedToAccessListItem(ctx, authUserID, listItemID); err != nil {
		return err
	}

	return s.repo.SetListItemCompleted(ctx, listItemID, isCompleted)
}

func (s *ListService) GetListItems(ctx context.Context, authUserID string, listID string) ([]ListItem, error) {
	if listID == "" {
		return nil, ErrListIDEmpty
	}

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return nil, err
	}

	return s.repo.GetListItems(ctx, listID)
}

func (s *ListService) userAllowedToAccessList(ctx context.Context, authUserID string, listID string) error {
	inList, err := s.repo.IsUserInList(ctx, listID, authUserID)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to check if user is in list",
			slog.String("user_id", authUserID),
			slog.String("list_id", listID),
			slog.Any("error", err),
		)
		return err
	}
	if !inList {
		slog.WarnContext(ctx,
			"user tried to access list without permission",
			slog.String("user_id", authUserID),
			slog.String("list_id", listID),
		)
		return ErrUserNotInList
	}

	return nil
}

func (s *ListService) userAllowedToAccessListItem(ctx context.Context, authUserID string, listItemID string) error {
	inList, err := s.repo.IsUserInListByItemID(ctx, listItemID, authUserID)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to check if user is in list",
			slog.String("user_id", authUserID),
			slog.String("list_item_id", listItemID),
			slog.Any("error", err),
		)
		return err
	}
	if !inList {
		slog.WarnContext(ctx,
			"user tried to access list item without permission",
			slog.String("user_id", authUserID),
			slog.String("list_item_id", listItemID),
		)
		return ErrUserNotInList
	}

	return nil
}
