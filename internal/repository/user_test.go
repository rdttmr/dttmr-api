package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

func newUserRepo(t *testing.T) (*UserRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &UserRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	insertUserQuery = `INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at`
	deleteUserQuery = `DELETE FROM users WHERE id = $1`
	updatePassQuery = `UPDATE users SET password_hash = $1 WHERE id = $2`
	selectUserQuery = `SELECT id, email, name FROM users WHERE email = $1`
)

func TestUserRepo_CreateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
		mock.ExpectQuery(regexp.QuoteMeta(insertUserQuery)).
			WithArgs("robin@dittmar.dev", "Robin", "$2a$10$hash").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "created_at"}).
					AddRow("2f1c...", createdAt),
			)

		user, err := repo.CreateUser(context.Background(), "robin@dittmar.dev", "Robin", "$2a$10$hash")

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, &domain.User{
			ID:        "2f1c...",
			Email:     "robin@dittmar.dev",
			Name:      "Robin",
			CreatedAt: createdAt,
		}, user)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectQuery(regexp.QuoteMeta(insertUserQuery)).
			WithArgs("robin@dittmar.dev", "Robin", "$2a$10$hash").
			WillReturnError(dbErr)

		user, err := repo.CreateUser(context.Background(), "robin@dittmar.dev", "Robin", "$2a$10$hash")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to insert user")
	})
}

func TestUserRepo_DeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectExec(regexp.QuoteMeta(deleteUserQuery)).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteUser(context.Background(), "user-1"))
	})

	t.Run("unknown id is not reported", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectExec(regexp.QuoteMeta(deleteUserQuery)).
			WithArgs("does-not-exist").
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, repo.DeleteUser(context.Background(), "does-not-exist"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(regexp.QuoteMeta(deleteUserQuery)).
			WithArgs("user-1").
			WillReturnError(dbErr)

		err := repo.DeleteUser(context.Background(), "user-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete user")
	})
}

func TestUserRepo_ChangePassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectExec(regexp.QuoteMeta(updatePassQuery)).
			WithArgs("$2a$10$newhash", "user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.ChangePassword(context.Background(), "user-1", "$2a$10$newhash"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		dbErr := errors.New("deadlock detected")
		mock.ExpectExec(regexp.QuoteMeta(updatePassQuery)).
			WithArgs("$2a$10$newhash", "user-1").
			WillReturnError(dbErr)

		err := repo.ChangePassword(context.Background(), "user-1", "$2a$10$newhash")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update user")
	})
}

func TestUserRepo_UsesTransactionFromContext(t *testing.T) {
	repo, mock := newUserRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(deleteUserQuery)).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := repo.db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), txKey{}, tx)
	require.NoError(t, repo.DeleteUser(ctx, "user-1"))
	require.NoError(t, tx.Commit())
}

func TestUserRepo_GetUserByEmail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectUserQuery)).
			WithArgs("robin@dittmar.dev").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "email", "name"}).
					AddRow("user-1", "robin@dittmar.dev", "Robin"),
			)

		user, err := repo.GetUserByEmail(context.Background(), "robin@dittmar.dev")

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "user-1", user.ID)
		assert.Equal(t, "robin@dittmar.dev", user.Email)
		assert.Equal(t, "Robin", user.Name)
		assert.Zero(t, user.CreatedAt) // not selected by this query
	})

	t.Run("not found stays matchable via errors.Is", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectUserQuery)).
			WithArgs("nobody@dittmar.dev").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByEmail(context.Background(), "nobody@dittmar.dev")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorContains(t, err, "failed to get user")
	})

	t.Run("scan error on type mismatch", func(t *testing.T) {
		repo, mock := newUserRepo(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectUserQuery)).
			WithArgs("robin@dittmar.dev").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "email", "name"}).
					AddRow(nil, "robin@dittmar.dev", "Robin"),
			)

		user, err := repo.GetUserByEmail(context.Background(), "robin@dittmar.dev")

		assert.Nil(t, user)
		assert.Error(t, err)
	})
}
