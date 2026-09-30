package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

func newAuthRepo(t *testing.T) (*AuthRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &AuthRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	selectAuthUserByIDQuery    = "SELECT id, email, name, password_hash FROM users WHERE id = $1"
	selectAuthUserByEmailQuery = "SELECT id, email, name, password_hash FROM users WHERE email = $1"
	insertRefreshTokenQuery    = "INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)"
	consumeRefreshTokenQuery   = "DELETE FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW() RETURNING user_id"
	revokeRefreshTokenQuery    = "DELETE FROM refresh_tokens WHERE token_hash = $1"
	revokeRefreshTokensQuery   = "DELETE FROM refresh_tokens WHERE user_id = $1"
)

var authUserColumns = []string{"id", "email", "name", "password_hash"}

func TestAuthRepo_GetUserById(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(selectAuthUserByIDQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(authUserColumns).AddRow("user-1", "robin@dittmar.dev", "Robin", "$2a$10$hash"))

		user, err := repo.GetUserById(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, &domain.AuthUser{
			ID:           "user-1",
			Email:        "robin@dittmar.dev",
			Name:         "Robin",
			PasswordHash: "$2a$10$hash",
		}, user)
	})

	t.Run("not found stays matchable via errors.Is", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(selectAuthUserByIDQuery).
			WithArgs("deleted-user").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserById(context.Background(), "deleted-user")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorContains(t, err, "failed to get user")
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectAuthUserByIDQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		user, err := repo.GetUserById(context.Background(), "user-1")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get user")
	})
}

func TestAuthRepo_GetUserByEmail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(selectAuthUserByEmailQuery).
			WithArgs("robin@dittmar.dev").
			WillReturnRows(sqlmock.NewRows(authUserColumns).AddRow("user-1", "robin@dittmar.dev", "Robin", "$2a$10$hash"))

		user, err := repo.GetUserByEmail(context.Background(), "robin@dittmar.dev")

		require.NoError(t, err)
		assert.Equal(t, &domain.AuthUser{
			ID:           "user-1",
			Email:        "robin@dittmar.dev",
			Name:         "Robin",
			PasswordHash: "$2a$10$hash",
		}, user)
	})

	t.Run("unknown email is reported as not found", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(selectAuthUserByEmailQuery).
			WithArgs("nobody@dittmar.dev").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByEmail(context.Background(), "nobody@dittmar.dev")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, domain.ErrEmailNotFound)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectAuthUserByEmailQuery).
			WithArgs("robin@dittmar.dev").
			WillReturnError(dbErr)

		user, err := repo.GetUserByEmail(context.Background(), "robin@dittmar.dev")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get user")
	})
}

func TestAuthRepo_StoreRefreshToken(t *testing.T) {
	expiresAt := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectExec(insertRefreshTokenQuery).
			WithArgs("user-1", "token-hash", expiresAt).
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.StoreRefreshToken(context.Background(), "user-1", "token-hash", expiresAt))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectExec(insertRefreshTokenQuery).
			WithArgs("user-1", "token-hash", expiresAt).
			WillReturnError(dbErr)

		err := repo.StoreRefreshToken(context.Background(), "user-1", "token-hash", expiresAt)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to store refresh token")
	})
}

func TestAuthRepo_ConsumeRefreshToken(t *testing.T) {
	t.Run("returns the owning user", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(consumeRefreshTokenQuery).
			WithArgs("token-hash").
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user-1"))

		userID, err := repo.ConsumeRefreshToken(context.Background(), "token-hash")

		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("unknown or expired token stays matchable via errors.Is", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectQuery(consumeRefreshTokenQuery).
			WithArgs("token-hash").
			WillReturnError(sql.ErrNoRows)

		userID, err := repo.ConsumeRefreshToken(context.Background(), "token-hash")

		assert.Empty(t, userID)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorContains(t, err, "refresh token not found or expired")
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(consumeRefreshTokenQuery).
			WithArgs("token-hash").
			WillReturnError(dbErr)

		userID, err := repo.ConsumeRefreshToken(context.Background(), "token-hash")

		assert.Empty(t, userID)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to consume refresh token")
	})
}

func TestAuthRepo_RevokeRefreshToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectExec(revokeRefreshTokenQuery).
			WithArgs("token-hash").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.RevokeRefreshToken(context.Background(), "token-hash"))
	})

	t.Run("unknown token is not reported", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectExec(revokeRefreshTokenQuery).
			WithArgs("unknown-hash").
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, repo.RevokeRefreshToken(context.Background(), "unknown-hash"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(revokeRefreshTokenQuery).
			WithArgs("token-hash").
			WillReturnError(dbErr)

		err := repo.RevokeRefreshToken(context.Background(), "token-hash")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to revoke refresh token")
	})
}

func TestAuthRepo_RevokeRefreshTokens(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		mock.ExpectExec(revokeRefreshTokensQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 3))

		assert.NoError(t, repo.RevokeRefreshTokens(context.Background(), "user-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newAuthRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(revokeRefreshTokensQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		err := repo.RevokeRefreshTokens(context.Background(), "user-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to revoke refresh tokens")
	})
}
