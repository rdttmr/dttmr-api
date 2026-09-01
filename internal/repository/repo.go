package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
)

type Transaction interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

type Repo struct {
	*Transactor
}

func NewRepo(t *Transactor) Repo {
	return Repo{t}
}

func (r *Repo) conn(ctx context.Context) Transaction {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

type txKey struct{}

type Transactor struct {
	db *sql.DB
	sp atomic.Uint64
}

func NewTransactor(db *sql.DB) *Transactor {
	return &Transactor{db: db, sp: atomic.Uint64{}}
}

func (t *Transactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return t.withinSavepoint(ctx, tx, fn)
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = fn(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (t *Transactor) withinSavepoint(ctx context.Context, tx *sql.Tx, fn func(context.Context) error) error {
	name := fmt.Sprintf("sp_%d", t.sp.Add(1))
	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		return err
	}

	if err := fn(ctx); err != nil {
		_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT"+name)
		return err
	}

	_, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT"+name)
	return err
}
