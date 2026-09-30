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

func newInviteRepo(t *testing.T) (*InviteRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &InviteRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	insertInviteQuery           = "INSERT INTO invites (inviter_user_id, code, expires_at) VALUES ($1, $2, $3) RETURNING id"
	deleteInviteQuery           = "DELETE FROM invites WHERE id = $1 AND inviter_user_id = $2 AND consumed_at IS NULL"
	consumeInviteQuery          = "UPDATE invites SET invitee_user_id=$1, consumed_at=NOW() WHERE id=$2 AND expires_at > NOW() AND consumed_at IS NULL"
	selectInviteQuery           = "SELECT id, code, expires_at, consumed_at FROM invites WHERE code=$1"
	selectInvitesQuery          = "SELECT id, code, expires_at, consumed_at FROM invites WHERE inviter_user_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3"
	countInvitesQuery           = "SELECT COUNT(*) FROM invites WHERE inviter_user_id=$1"
	countInvitesStructuredQuery = "SELECT COUNT(*) FILTER (WHERE expires_at > NOW() AND consumed_at IS NULL), COUNT(*) FILTER (WHERE expires_at <= NOW() AND consumed_at IS NULL), COUNT(consumed_at) FROM invites WHERE inviter_user_id = $1"
)

var inviteColumns = []string{"id", "code", "expires_at", "consumed_at"}

func TestInviteRepo_CreateInvite(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(insertInviteQuery).
			WithArgs("user-1", "ABC123", expiresAt).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("invite-1"))

		invite, err := repo.CreateInvite(context.Background(), "user-1", "ABC123", expiresAt)

		require.NoError(t, err)
		assert.Equal(t, &domain.Invite{
			ID:         "invite-1",
			Code:       "ABC123",
			ExpiresAt:  expiresAt,
			ConsumedAt: nil,
		}, invite)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectQuery(insertInviteQuery).
			WithArgs("user-1", "ABC123", expiresAt).
			WillReturnError(dbErr)

		invite, err := repo.CreateInvite(context.Background(), "user-1", "ABC123", expiresAt)

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to insert invite")
	})
}

func TestInviteRepo_DeleteInvite(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectExec(deleteInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteInvite(context.Background(), "user-1", "invite-1"))
	})

	t.Run("no row deleted is reported as consumed", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectExec(deleteInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.DeleteInvite(context.Background(), "user-1", "invite-1")

		assert.ErrorIs(t, err, domain.ErrInviteConsumed)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnError(dbErr)

		err := repo.DeleteInvite(context.Background(), "user-1", "invite-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete invite")
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(deleteInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.DeleteInvite(context.Background(), "user-1", "invite-1")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})
}

func TestInviteRepo_ConsumeInvite(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectExec(consumeInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.ConsumeInvite(context.Background(), "invite-1", "user-2"))
	})

	t.Run("no row updated means invite is invalid", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectExec(consumeInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.ConsumeInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, domain.ErrInviteInvalid)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(consumeInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnError(dbErr)

		err := repo.ConsumeInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update invite")
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(consumeInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.ConsumeInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})
}

func TestInviteRepo_GetInvite(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	consumedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	t.Run("open invite", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInviteQuery).
			WithArgs("ABC123").
			WillReturnRows(sqlmock.NewRows(inviteColumns).AddRow("invite-1", "ABC123", expiresAt, nil))

		invite, err := repo.GetInvite(context.Background(), "ABC123")

		require.NoError(t, err)
		assert.Equal(t, &domain.Invite{ID: "invite-1", Code: "ABC123", ExpiresAt: expiresAt}, invite)
	})

	t.Run("consumed invite", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInviteQuery).
			WithArgs("ABC123").
			WillReturnRows(sqlmock.NewRows(inviteColumns).AddRow("invite-1", "ABC123", expiresAt, consumedAt))

		invite, err := repo.GetInvite(context.Background(), "ABC123")

		require.NoError(t, err)
		require.NotNil(t, invite.ConsumedAt)
		assert.Equal(t, consumedAt, *invite.ConsumedAt)
	})

	t.Run("unknown code is reported as invalid", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInviteQuery).
			WithArgs("NOPE").
			WillReturnError(sql.ErrNoRows)

		invite, err := repo.GetInvite(context.Background(), "NOPE")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, domain.ErrInviteInvalid)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectInviteQuery).
			WithArgs("ABC123").
			WillReturnError(dbErr)

		invite, err := repo.GetInvite(context.Background(), "ABC123")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get invite")
	})
}

func TestInviteRepo_GetInvites(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	consumedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInvitesQuery).
			WithArgs("user-1", 20, 10).
			WillReturnRows(
				sqlmock.NewRows(inviteColumns).
					AddRow("invite-2", "DEF456", expiresAt, nil).
					AddRow("invite-1", "ABC123", expiresAt, consumedAt),
			)

		invites, err := repo.GetInvites(context.Background(), "user-1", 20, 10)

		require.NoError(t, err)
		assert.Equal(t, []domain.Invite{
			{ID: "invite-2", Code: "DEF456", ExpiresAt: expiresAt},
			{ID: "invite-1", Code: "ABC123", ExpiresAt: expiresAt, ConsumedAt: &consumedAt},
		}, invites)
	})

	t.Run("no rows returns no invites", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInvitesQuery).
			WithArgs("user-1", 0, 10).
			WillReturnRows(sqlmock.NewRows(inviteColumns))

		invites, err := repo.GetInvites(context.Background(), "user-1", 0, 10)

		require.NoError(t, err)
		assert.Empty(t, invites)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectInvitesQuery).
			WithArgs("user-1", 0, 10).
			WillReturnError(dbErr)

		invites, err := repo.GetInvites(context.Background(), "user-1", 0, 10)

		assert.Nil(t, invites)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get invites")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(selectInvitesQuery).
			WithArgs("user-1", 0, 10).
			WillReturnRows(sqlmock.NewRows(inviteColumns).AddRow("invite-1", "ABC123", "not a time", nil))

		invites, err := repo.GetInvites(context.Background(), "user-1", 0, 10)

		assert.Nil(t, invites)
		assert.Error(t, err)
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectInvitesQuery).
			WithArgs("user-1", 0, 10).
			WillReturnRows(
				sqlmock.NewRows(inviteColumns).
					AddRow("invite-2", "DEF456", expiresAt, nil).
					AddRow("invite-1", "ABC123", expiresAt, nil).
					RowError(1, rowErr),
			)

		invites, err := repo.GetInvites(context.Background(), "user-1", 0, 10)

		assert.Nil(t, invites)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get invites")
	})
}

func TestInviteRepo_CountInvites(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(countInvitesQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

		count, err := repo.CountInvites(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, 7, count)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(countInvitesQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		count, err := repo.CountInvites(context.Background(), "user-1")

		assert.Zero(t, count)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to count invites")
	})
}

func TestInviteRepo_CountInvitesStructured(t *testing.T) {
	countColumns := []string{"active", "expired", "used"}

	t.Run("success", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(countInvitesStructuredQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(countColumns).AddRow(3, 2, 5))

		counts, err := repo.CountInvitesStructured(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, &domain.InviteCounts{Active: 3, Expired: 2, Used: 5}, counts)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(countInvitesStructuredQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		counts, err := repo.CountInvitesStructured(context.Background(), "user-1")

		assert.Nil(t, counts)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to count structured invites")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newInviteRepo(t)

		mock.ExpectQuery(countInvitesStructuredQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(countColumns).AddRow(3, "not a number", 5))

		counts, err := repo.CountInvitesStructured(context.Background(), "user-1")

		assert.Nil(t, counts)
		assert.ErrorContains(t, err, "failed to count structured invites")
	})
}
