package domain

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var (
	ErrListIDMissing        = errors.New("list id is required")
	ErrListNameMissing      = errors.New("list name is required")
	ErrListItemIDMissing    = errors.New("list item id is required")
	ErrListItemTitleMissing = errors.New("list item title is required")
	ErrUserNotInList        = errors.New("user not in list")
)

type List struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	ModifiedAt     time.Time `json:"modified_at"`
	TotalItems     int       `json:"total_items"`
	CompletedItems int       `json:"completed_items"`
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
	CreateList(ctx context.Context, name string) (*List, error)
	DeleteList(ctx context.Context, listID string) error
	GetLists(ctx context.Context, userID string) ([]List, error)
	AddUserToList(ctx context.Context, listID string, userID string) error
	RemoveUserFromList(ctx context.Context, listID string, userID string) error
	IsUserInList(ctx context.Context, listID string, userID string) (bool, error)
	IsUserInListByItemID(ctx context.Context, listItemID string, userID string) (bool, error)
	CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error)
	DeleteListItem(ctx context.Context, listItemID string) error
	UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error
	SetListItemCompleted(ctx context.Context, listItemID string, isCompleted bool) error
	GetListItems(ctx context.Context, listID string) ([]ListItem, error)
}

type ListService struct {
	tx   Transactor
	repo ListRepository
}

func NewListService(tx Transactor, r ListRepository) *ListService {
	return &ListService{tx: tx, repo: r}
}

func (s *ListService) CreateList(ctx context.Context, authUserID string, name string) (*List, error) {
	if name == "" {
		return nil, ErrListNameMissing
	}

	var list *List
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		l, err := s.repo.CreateList(ctx, name)
		if err != nil {
			return err
		}

		err = s.repo.AddUserToList(ctx, list.ID, authUserID)
		if err != nil {
			return err
		}

		list = l
		return nil
	})
	if err != nil {
		return nil, err
	}

	return list, nil
}

func (s *ListService) DeleteList(ctx context.Context, authUserID string, listID string) error {
	if listID == "" {
		return ErrListIDMissing
	}

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return err
	}

	return s.repo.DeleteList(ctx, listID)
}

func (s *ListService) GetLists(ctx context.Context, authUserID string) ([]List, error) {
	return s.repo.GetLists(ctx, authUserID)
}

func (s *ListService) AddUserToList(ctx context.Context, authUserID string, listID string, userID string) error {
	if listID == "" {
		return ErrListIDMissing
	}
	if userID == "" {
		return ErrUserIDMissing
	}

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return err
	}

	return s.repo.AddUserToList(ctx, listID, userID)
}

func (s *ListService) RemoveUserFromList(ctx context.Context, authUserID string, listID string, userID string) error {
	if listID == "" {
		return ErrListIDMissing
	}
	if userID == "" {
		return ErrUserIDMissing
	}

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return err
	}

	return s.repo.RemoveUserFromList(ctx, listID, userID)
}

func (s *ListService) CreateListItem(ctx context.Context, authUserID string, listID string, title string) (*ListItem, error) {
	if listID == "" {
		return nil, ErrListIDMissing
	}
	if title == "" {
		return nil, ErrListItemTitleMissing
	}

	if err := s.userAllowedToAccessList(ctx, authUserID, listID); err != nil {
		return nil, err
	}

	return s.repo.CreateListItem(ctx, listID, title)
}

func (s *ListService) DeleteListItem(ctx context.Context, authUserID string, listItemID string) error {
	if listItemID == "" {
		return ErrListItemIDMissing
	}

	if err := s.userAllowedToAccessListItem(ctx, authUserID, listItemID); err != nil {
		return err
	}

	return s.repo.DeleteListItem(ctx, listItemID)
}

func (s *ListService) UpdateListItem(ctx context.Context, authUserID string, listItemID string, title string, isCompleted bool) error {
	if listItemID == "" {
		return ErrListItemIDMissing
	}
	if title == "" {
		return ErrListItemTitleMissing
	}

	if err := s.userAllowedToAccessListItem(ctx, authUserID, listItemID); err != nil {
		return err
	}

	return s.repo.UpdateListItem(ctx, listItemID, title, isCompleted)
}

func (s *ListService) SetListItemCompleted(ctx context.Context, authUserID string, listItemID string, isCompleted bool) error {
	if listItemID == "" {
		return ErrListItemIDMissing
	}

	if err := s.userAllowedToAccessListItem(ctx, authUserID, listItemID); err != nil {
		return err
	}

	return s.repo.SetListItemCompleted(ctx, listItemID, isCompleted)
}

func (s *ListService) GetListItems(ctx context.Context, authUserID string, listID string) ([]ListItem, error) {
	if listID == "" {
		return nil, ErrListIDMissing
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
