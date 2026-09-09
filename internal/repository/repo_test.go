package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unrelatedKey struct{}

func newTransactor(t *testing.T) (*Transactor, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return NewTransactor(db), mock
}

func TestNewRepo(t *testing.T) {
	tr, _ := newTransactor(t)

	repo := NewRepo(tr)

	assert.Same(t, tr, repo.Transactor)
}

func TestRepo_Conn(t *testing.T) {
	t.Run("falls back to the pool without a transaction", func(t *testing.T) {
		tr, mock := newTransactor(t)
		repo := NewRepo(tr)

		mock.ExpectExec("DELETE FROM users WHERE id = $1").
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		ctx := context.Background()
		assert.Same(t, tr.db, repo.conn(ctx))

		_, err := repo.conn(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = $1", "user-1")
		require.NoError(t, err)
	})

	t.Run("returns the transaction carried by the context", func(t *testing.T) {
		tr, mock := newTransactor(t)
		repo := NewRepo(tr)

		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := tr.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		ctx := context.WithValue(context.Background(), txKey{}, tx)
		assert.Same(t, tx, repo.conn(ctx))

		require.NoError(t, tx.Rollback())
	})

	t.Run("ignores values stored under other keys", func(t *testing.T) {
		tr, _ := newTransactor(t)
		repo := NewRepo(tr)

		ctx := context.WithValue(context.Background(), unrelatedKey{}, "irrelevant")

		assert.Same(t, tr.db, repo.conn(ctx))
	})

	t.Run("ignores a value of the wrong type under txKey", func(t *testing.T) {
		tr, _ := newTransactor(t)
		repo := NewRepo(tr)

		ctx := context.WithValue(context.Background(), txKey{}, "not a transaction")

		assert.Same(t, tr.db, repo.conn(ctx))
	})
}

func TestRepo_WithinTxIsPromoted(t *testing.T) {
	tr, mock := newTransactor(t)
	repo := NewRepo(tr)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM users WHERE id = $1").
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.WithinTx(context.Background(), func(ctx context.Context) error {
		_, execErr := repo.conn(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = $1", "user-1")
		return execErr
	})

	require.NoError(t, err)
}

func TestRepo_SiblingReposShareTheTransaction(t *testing.T) {
	tr, mock := newTransactor(t)
	users := NewRepo(tr)
	lists := NewRepo(tr)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO lists (name) VALUES ($1)").
		WithArgs("Groceries").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM users WHERE id = $1").
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
		assert.Same(t, lists.conn(ctx), users.conn(ctx))

		if _, err := lists.conn(ctx).ExecContext(ctx, "INSERT INTO lists (name) VALUES ($1)", "Groceries"); err != nil {
			return err
		}

		_, err := users.conn(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = $1", "user-1")
		return err
	})

	require.NoError(t, err)
}

func TestTransactor_WithinTx(t *testing.T) {
	t.Run("commits and routes statements through the tx", func(t *testing.T) {
		tr, mock := newTransactor(t)
		repo := NewRepo(tr)

		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM users WHERE id = $1").
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			tx, ok := ctx.Value(txKey{}).(*sql.Tx)
			require.True(t, ok)
			assert.Same(t, tx, repo.conn(ctx))

			_, execErr := repo.conn(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = $1", "user-1")
			return execErr
		})

		require.NoError(t, err)
	})

	t.Run("callback error rolls back and propagates", func(t *testing.T) {
		tr, mock := newTransactor(t)

		mock.ExpectBegin()
		mock.ExpectRollback()

		fnErr := errors.New("business rule violated")
		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			return fnErr
		})

		assert.ErrorIs(t, err, fnErr)
	})

	t.Run("begin error skips the callback", func(t *testing.T) {
		tr, mock := newTransactor(t)

		beginErr := errors.New("too many connections")
		mock.ExpectBegin().WillReturnError(beginErr)

		called := false
		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			called = true
			return nil
		})

		assert.ErrorIs(t, err, beginErr)
		assert.False(t, called)
	})

	t.Run("commit error is returned", func(t *testing.T) {
		tr, mock := newTransactor(t)

		commitErr := errors.New("could not serialize access")
		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(commitErr)

		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			return nil
		})

		assert.ErrorIs(t, err, commitErr)
	})
}

func TestTransactor_WithinTxNested(t *testing.T) {
	t.Run("reuses the outer tx and releases the savepoint", func(t *testing.T) {
		tr, mock := newTransactor(t)

		mock.ExpectBegin()
		mock.ExpectExec("SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		var outerTx *sql.Tx
		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			outerTx = ctx.Value(txKey{}).(*sql.Tx)

			return tr.WithinTx(ctx, func(ctx context.Context) error {
				assert.Same(t, outerTx, ctx.Value(txKey{}).(*sql.Tx))
				return nil
			})
		})

		require.NoError(t, err)
	})

	t.Run("inner error rolls back to the savepoint", func(t *testing.T) {
		tr, mock := newTransactor(t)

		mock.ExpectBegin()
		mock.ExpectExec("SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("ROLLBACK TO SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		innerErr := errors.New("nested failure")
		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			return tr.WithinTx(ctx, func(ctx context.Context) error {
				return innerErr
			})
		})

		assert.ErrorIs(t, err, innerErr)
	})

	t.Run("savepoint creation error skips the callback", func(t *testing.T) {
		tr, mock := newTransactor(t)

		spErr := errors.New("savepoint failed")
		mock.ExpectBegin()
		mock.ExpectExec("SAVEPOINT sp_1").WillReturnError(spErr)
		mock.ExpectRollback()

		called := false
		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			return tr.WithinTx(ctx, func(ctx context.Context) error {
				called = true
				return nil
			})
		})

		assert.ErrorIs(t, err, spErr)
		assert.False(t, called)
	})

	t.Run("release error is returned", func(t *testing.T) {
		tr, mock := newTransactor(t)

		releaseErr := errors.New("no such savepoint")
		mock.ExpectBegin()
		mock.ExpectExec("SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT sp_1").WillReturnError(releaseErr)
		mock.ExpectRollback()

		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			return tr.WithinTx(ctx, func(ctx context.Context) error {
				return nil
			})
		})

		assert.ErrorIs(t, err, releaseErr)
	})

	t.Run("each savepoint gets its own name", func(t *testing.T) {
		tr, mock := newTransactor(t)

		mock.ExpectBegin()
		mock.ExpectExec("SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT sp_1").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("SAVEPOINT sp_2").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT sp_2").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		noop := func(ctx context.Context) error { return nil }

		err := tr.WithinTx(context.Background(), func(ctx context.Context) error {
			if err := tr.WithinTx(ctx, noop); err != nil {
				return err
			}
			return tr.WithinTx(ctx, noop)
		})

		require.NoError(t, err)
	})
}
