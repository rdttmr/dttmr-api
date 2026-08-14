package repository

import (
	"context"
	"database/sql"
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
	_, err := r.db.ExecContext(ctx, "UPDATE list_items SET title = $1, is_completed = $2 WHERE id = $3",
		title, isCompleted, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list item: %w", err)
	}

	return nil
}
