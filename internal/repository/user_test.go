package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/robindittmar/dttmr-api/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_CreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewUserRepo(db)

	ctx := context.Background()
	email := "test@example.com"
	name := "Test User"
	passwordHash := "hashedpassword123"

	now := time.Now()
	expectedUser := &domain.User{
		ID:        "1",
		Email:     email,
		Name:      name,
		CreatedAt: now,
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectBegin()

		mock.ExpectQuery(`^INSERT INTO users \(email, name, password_hash\) VALUES \(\$1, \$2, \$3\) RETURNING id, created_at$`).
			WithArgs(email, name, passwordHash).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(expectedUser.ID, expectedUser.CreatedAt))

		mock.ExpectCommit()

		user, err := repo.CreateUser(ctx, email, name, passwordHash)
		assert.NoError(t, err)
		assert.Equal(t, expectedUser, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("begin_tx_error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(fmt.Errorf("tx error"))

		user, err := repo.CreateUser(ctx, email, name, passwordHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "begin transaction")
		assert.Nil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("insert_error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`^INSERT INTO users \(email, name, password_hash\) VALUES \(\$1, \$2, \$3\) RETURNING id, created_at$`).
			WithArgs(email, name, passwordHash).
			WillReturnError(fmt.Errorf("insert error"))
		mock.ExpectRollback()

		user, err := repo.CreateUser(ctx, email, name, passwordHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert user")
		assert.Nil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("commit_error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(`^INSERT INTO users \(email, name, password_hash\) VALUES \(\$1, \$2, \$3\) RETURNING id, created_at$`).
			WithArgs(email, name, passwordHash).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(expectedUser.ID, expectedUser.CreatedAt))
		mock.ExpectCommit().WillReturnError(fmt.Errorf("commit error"))

		user, err := repo.CreateUser(ctx, email, name, passwordHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "commit transaction")
		assert.Nil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserRepo_CreateUser2(t *testing.T) {
	email := "test@example.com"
	name := "Test User"
	passwordHash := "hashedpassword123"
	now := time.Now()
	expectedID := "42"

	insertQuery := regexp.QuoteMeta(
		"INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at",
	)

	testCases := []struct {
		name          string
		setupMock     func(mock sqlmock.Sqlmock)
		expectedError string
	}{
		{
			name: "Success: User created perfectly",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				rows := sqlmock.NewRows([]string{"id", "created_at"}).
					AddRow(expectedID, now)

				mock.ExpectQuery(insertQuery).
					WithArgs(email, name, passwordHash).
					WillReturnRows(rows)

				mock.ExpectCommit()
			},
			expectedError: "",
		},
		{
			name: "Failure: Database connection fails on BeginTx",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("db connection failed"))
			},
			expectedError: "begin transaction: db connection failed",
		},
		{
			name: "Failure: Query fails (e.g., duplicate email)",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(insertQuery).
					WithArgs(email, name, passwordHash).
					WillReturnError(errors.New("unique constraint violation"))

				mock.ExpectRollback()
			},
			expectedError: "failed to insert user: unique constraint violation",
		},
		{
			name: "Failure: Commit fails (e.g., network timeout)",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				rows := sqlmock.NewRows([]string{"id", "created_at"}).
					AddRow(expectedID, now)

				mock.ExpectQuery(insertQuery).
					WithArgs(email, name, passwordHash).
					WillReturnRows(rows)

				mock.ExpectCommit().WillReturnError(errors.New("commit timeout"))
			},
			expectedError: "commit transaction: commit timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tc.setupMock(mock)

			repo := NewUserRepo(db)

			user, err := repo.CreateUser(context.Background(), email, name, passwordHash)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, expectedID, user.ID)
				assert.Equal(t, email, user.Email)
				assert.Equal(t, name, user.Name)
				assert.Equal(t, now, user.CreatedAt)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
