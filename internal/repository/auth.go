package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/robindittmar/dttmr-api/internal/domain"
)

type AuthRepo struct {
	db *sql.DB
}

func NewAuthRepo(db *sql.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (r *AuthRepo) GetUserById(ctx context.Context, id string) (*domain.AuthUser, error) {
	user := &domain.AuthUser{}

	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, name, password_hash FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*domain.AuthUser, error) {
	user := &domain.AuthUser{}

	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, name, password_hash FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (r *AuthRepo) StoreRefreshToken(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}
func (r *AuthRepo) ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx,
		"DELETE FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW() RETURNING user_id",
		tokenHash,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("refresh token not found or expired: %w", err)
		}
		return "", fmt.Errorf("failed to consume refresh token: %w", err)
	}

	return userID, nil
}
