package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/robindittmar/dttmr-api/internal/domain"
)

type ListRepo struct {
	db *sql.DB
}

func NewListRepo(db *sql.DB) *ListRepo {
	return &ListRepo{db: db}
}

func (r *ListRepo) CreateList(ctx context.Context, name string, userIDs []string) (*domain.List, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	list := &domain.List{Name: name}

	err = tx.QueryRowContext(ctx,
		"INSERT INTO lists (name) VALUES ($1) RETURNING id, created_at, modified_at",
		name,
	).Scan(&list.ID, &list.CreatedAt, &list.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert list: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO list_users (list_id, user_id) VALUES ($1, $2)")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare user/list association statement: %w", err)
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			slog.Error("failed to close user/list association statement", slog.Any("error", err))
		}
	}()

	for _, userID := range userIDs {
		if _, err = stmt.ExecContext(ctx, list.ID, userID); err != nil {
			return nil, fmt.Errorf("failed to insert user/list association: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return list, nil
}

func (r *ListRepo) GetLists(ctx context.Context, userID string) ([]domain.List, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, lists.created_at, modified_at FROM lists INNER JOIN list_users ON lists.id=list_users.list_id WHERE list_users.user_id = $1",
		userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}
	defer rows.Close()

	var lists []domain.List
	for rows.Next() {
		var l domain.List
		err = rows.Scan(&l.ID, &l.Name, &l.CreatedAt, &l.ModifiedAt)
		if err != nil {
			return nil, err
		}

		lists = append(lists, l)
	}

	return lists, nil

}

func (r *ListRepo) AddUserToList(ctx context.Context, listID string, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO list_users (list_id, user_id) VALUES ($1, $2)",
		listID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to associate user/list: %w", err)
	}

	return nil
}

func (r *ListRepo) RemoveUserFromList(ctx context.Context, listID string, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM list_users WHERE list_id = $1 AND user_id = $2",
		listID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove user from list: %w", err)
	}

	return nil
}

func (r *ListRepo) IsUserInList(ctx context.Context, listID string, userID string) (bool, error) {
	var cnt int

	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM list_users WHERE list_id = $1 AND user_id = $2",
		listID, userID,
	).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is in list: %w", err)
	}

	return cnt > 0, nil
}

func (r *ListRepo) IsUserInListByItemID(ctx context.Context, listItemID string, userID string) (bool, error) {
	var cnt int

	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM list_users WHERE list_id = (SELECT list_id FROM list_items WHERE id = $1) AND user_id = $2",
		listItemID, userID,
	).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is in list: %w", err)
	}

	return cnt > 0, nil
}

func (r *ListRepo) CreateListItem(ctx context.Context, listID string, title string) (*domain.ListItem, error) {
	l := &domain.ListItem{Title: title}

	err := r.db.QueryRowContext(ctx,
		"INSERT INTO list_items (list_id, title) VALUES ($1, $2) RETURNING id, is_completed, created_at, modified_at",
		listID, title,
	).Scan(&l.ID, &l.IsCompleted, &l.CreatedAt, &l.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert list item: %w", err)
	}

	return l, nil
}

func (r *ListRepo) UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE list_items SET title = $1, is_completed = $2, modified_at = NOW() WHERE id = $3",
		title, isCompleted, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list item: %w", err)
	}

	return nil
}

func (r *ListRepo) SetListItemCompleted(ctx context.Context, listItemID string, isCompleted bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE list_items SET is_completed = $1, modified_at = NOW() WHERE id = $2",
		isCompleted, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to complete list item: %w", err)
	}

	return nil
}

func (r *ListRepo) GetListItems(ctx context.Context, listID string) ([]domain.ListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, title, is_completed, created_at, modified_at FROM list_items WHERE list_id = $1",
		listID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get list items: %w", err)
	}
	defer rows.Close()

	var items []domain.ListItem
	for rows.Next() {
		var l domain.ListItem
		err = rows.Scan(&l.ID, &l.Title, &l.IsCompleted, &l.CreatedAt, &l.ModifiedAt)
		if err != nil {
			return nil, err
		}

		l.ListID = listID
		items = append(items, l)
	}

	return items, nil
}
