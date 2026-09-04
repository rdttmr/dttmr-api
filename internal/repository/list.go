package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/robindittmar/dttmr-api/internal/domain"
)

type ListRepo struct {
	Repo
}

func (r *ListRepo) CreateList(ctx context.Context, name string) (*domain.List, error) {
	list := &domain.List{Name: name}

	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO lists (name) VALUES ($1) RETURNING id, created_at, modified_at",
		name,
	).Scan(&list.ID, &list.CreatedAt, &list.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert list: %w", err)
	}

	return list, nil
}

func (r *ListRepo) DeleteList(ctx context.Context, listID string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM lists WHERE id = $1", listID)
	if err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	return nil
}

func (r *ListRepo) GetLists(ctx context.Context, userID string) ([]domain.List, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT l.id, l.name, l.created_at, l.modified_at, (SELECT COUNT(*) FROM list_items WHERE list_id=l.id), (SELECT COUNT(*) FROM list_items WHERE list_id=l.id AND is_completed=true) FROM lists AS l INNER JOIN list_users ON l.id=list_users.list_id WHERE list_users.user_id = $1",
		userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}
	defer rows.Close()

	lists := make([]domain.List, 0, 16)
	for rows.Next() {
		var l domain.List
		err = rows.Scan(&l.ID, &l.Name, &l.CreatedAt, &l.ModifiedAt, &l.TotalItems, &l.CompletedItems)
		if err != nil {
			return nil, err
		}

		lists = append(lists, l)
	}

	return lists, nil
}

func (r *ListRepo) AddUserToList(ctx context.Context, listID string, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO list_users (list_id, user_id) VALUES ($1, $2)",
		listID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to associate user/list: %w", err)
	}

	return nil
}

func (r *ListRepo) RemoveUserFromList(ctx context.Context, listID string, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
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

	err := r.conn(ctx).QueryRowContext(ctx,
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

	err := r.conn(ctx).QueryRowContext(ctx,
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

	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO list_items (list_id, title) VALUES ($1, $2) RETURNING id, is_completed, created_at, modified_at",
		listID, title,
	).Scan(&l.ID, &l.IsCompleted, &l.CreatedAt, &l.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert list item: %w", err)
	}

	l.ListID = listID
	return l, nil
}

func (r *ListRepo) DeleteListItem(ctx context.Context, listItemID string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM list_items WHERE id = $1", listItemID)
	if err != nil {
		return fmt.Errorf("failed to delete list item: %w", err)
	}

	return nil
}

func (r *ListRepo) UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error {
	_, err := r.conn(ctx).ExecContext(ctx, "UPDATE list_items SET title = $1, is_completed = $2, modified_at = NOW() WHERE id = $3",
		title, isCompleted, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list item: %w", err)
	}

	return nil
}

func (r *ListRepo) SetListItemTitle(ctx context.Context, listItemID string, title string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "UPDATE list_items SET title = $1, modified_at = NOW() WHERE id = $2",
		title, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list item title: %w", err)
	}

	return nil
}

func (r *ListRepo) SetListItemCompleted(ctx context.Context, listItemID string, isCompleted bool) error {
	_, err := r.conn(ctx).ExecContext(ctx, "UPDATE list_items SET is_completed = $1, modified_at = NOW() WHERE id = $2",
		isCompleted, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to complete list item: %w", err)
	}

	return nil
}

func (r *ListRepo) GetListItems(ctx context.Context, listID string) ([]domain.ListItem, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT id, title, is_completed, created_at, modified_at FROM list_items WHERE list_id = $1 ORDER BY is_completed, modified_at DESC",
		listID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get list items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ListItem, 0, 32)
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
