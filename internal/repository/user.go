package repository

import (
	"context"
	"fmt"

	"github.com/robindittmar/dttmr-api/internal/domain"
)

type UserRepo struct {
	Repo
}

func (r *UserRepo) CreateUser(ctx context.Context, email string, name string, passwordHash string) (*domain.User, error) {
	user := &domain.User{Email: email, Name: name}

	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at",
		email, name, passwordHash,
	).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return user, nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"DELETE FROM users WHERE id = $1",
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (r *UserRepo) ChangePassword(ctx context.Context, userID string, passwordHash string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE users SET password_hash = $1 WHERE id = $2",
		passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}

	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT id, email, name FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}
