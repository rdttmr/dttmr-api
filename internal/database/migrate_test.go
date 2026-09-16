//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/database"
	"git.dittmar.dev/robin/dttmr-api/internal/database/migrations"
)

func TestRunMigrations_Real(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, database.RunMigrations(db, migrations.MigrationFS))
	require.NoError(t, database.RunMigrations(db, migrations.MigrationFS))

	_, dirty, ok := dbVersion(t, db)
	require.True(t, ok)
	require.False(t, dirty)
}

func TestMigrations_DownUpRoundtrip(t *testing.T) {
	db := newTestDB(t)
	m := newMigrator(t, db, migrations.MigrationFS)

	require.NoError(t, m.Up())
	afterUp := schemaSnapshot(t, db)

	require.NoError(t, m.Down())
	require.Empty(t, schemaSnapshot(t, db), "down migrations left objects behind")

	require.NoError(t, m.Up())
	require.Equal(t, afterUp, schemaSnapshot(t, db), "schema differs after down + up")
}

var (
	migA = map[string]string{
		"1_a.up.sql":   "CREATE TABLE a (id int);",
		"1_a.down.sql": "DROP TABLE a;",
	}
	migB = map[string]string{
		"2_b.up.sql":   "CREATE TABLE b (id int);",
		"2_b.down.sql": "DROP TABLE b;",
	}
	migBroken = map[string]string{
		"3_broken.up.sql":   "CREATE TABLE c (id int); SELECT does_not_exist();",
		"3_broken.down.sql": "DROP TABLE c;",
	}
)

func TestRunMigrations_RollsBackOnFailure(t *testing.T) {
	t.Run("back to previous version", func(t *testing.T) {
		db := newTestDB(t)
		require.NoError(t, database.RunMigrations(db, fsOf(migA)))

		err := database.RunMigrations(db, fsOf(migA, migB, migBroken))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "rollback failed")

		version, dirty, ok := dbVersion(t, db)
		require.True(t, ok)
		require.EqualValues(t, 1, version)
		require.False(t, dirty)
		require.True(t, tableExists(t, db, "a"))
		require.False(t, tableExists(t, db, "b"))
		require.False(t, tableExists(t, db, "c"))
	})

	t.Run("back to empty database", func(t *testing.T) {
		db := newTestDB(t)

		err := database.RunMigrations(db, fsOf(migA, migB, migBroken))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "rollback failed")

		_, _, ok := dbVersion(t, db)
		require.False(t, ok)
		for _, table := range []string{"a", "b", "c"} {
			require.False(t, tableExists(t, db, table))
		}
	})

	t.Run("first migration fails", func(t *testing.T) {
		db := newTestDB(t)

		err := database.RunMigrations(db, fsOf(migBroken))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "rollback failed")

		_, _, ok := dbVersion(t, db)
		require.False(t, ok)
	})
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	admin := connect(t, baseURL)

	name := fmt.Sprintf("test_%d", time.Now().UnixNano())
	_, err := admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})

	u, err := url.Parse(baseURL)
	require.NoError(t, err)
	u.Path = "/" + name
	return connect(t, u.String())
}

func connect(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		db, err := database.New(ctx, dsn)
		if err == nil {
			if err = db.PingContext(ctx); err == nil {
				t.Cleanup(func() { _ = db.Close() })
				return db
			}
			_ = db.Close()
		}
		select {
		case <-ctx.Done():
			t.Fatalf("database not reachable: %v", err)
		case <-time.After(time.Second):
		}
	}
}

func newMigrator(t *testing.T, db *sql.DB, fsys fs.FS) *migrate.Migrate {
	t.Helper()
	src, err := iofs.New(fsys, ".")
	require.NoError(t, err)
	drv, err := postgres.WithInstance(db, &postgres.Config{})
	require.NoError(t, err)
	m, err := migrate.NewWithInstance("iofs", src, "postgres", drv)
	require.NoError(t, err)
	return m
}

func fsOf(sets ...map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for _, set := range sets {
		for name, content := range set {
			out[name] = &fstest.MapFile{Data: []byte(content)}
		}
	}
	return out
}

func dbVersion(t *testing.T, db *sql.DB) (version int64, dirty bool, ok bool) {
	t.Helper()
	err := db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false
	}
	require.NoError(t, err)
	return version, dirty, true
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow("SELECT to_regclass('public.' || $1) IS NOT NULL", name).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func schemaSnapshot(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`
		SELECT 'column ' || table_name || '.' || column_name || ' ' || data_type
		       || ' ' || is_nullable || ' ' || coalesce(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name <> 'schema_migrations'
		UNION ALL
		SELECT 'index ' || indexdef
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename <> 'schema_migrations'
		UNION ALL
		SELECT 'constraint ' || conrelid::regclass || ' ' || pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE connamespace = 'public'::regnamespace
		  AND conrelid::regclass::text <> 'schema_migrations'
		UNION ALL
		SELECT 'enum ' || t.typname
		FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'public' AND t.typtype = 'e'
		ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}
