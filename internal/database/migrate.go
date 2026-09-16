package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func RunMigrations(db *sql.DB, migrationFS fs.FS) error {
	sourceDriver, err := iofs.New(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}
	defer func() {
		err := sourceDriver.Close()
		if err != nil {
			slog.Error("failed to close migrations source", slog.Any("error", err))
		}
	}()

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration db driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}

	hasVersion := true
	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			hasVersion = false
		} else {
			return fmt.Errorf("failed to fetch db version: %w", err)
		}
	}
	if dirty {
		return fmt.Errorf("database is dirty (version: %d)", version)
	}

	slog.Info("running database migrations...")
	upErr := m.Up()

	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		slog.Error("migration failed, attempting rollback", slog.Any("error", upErr))

		rbErr := tryRollback(m, sourceDriver, version, hasVersion)
		if rbErr != nil {
			return fmt.Errorf("migration failed: %w; rollback failed: %w", upErr, rbErr)
		}
		return fmt.Errorf("failed to run database migrations: %w", upErr)
	}

	slog.Info("database migrations applied successfully")
	return nil
}

func tryRollback(m *migrate.Migrate, src source.Driver, startVersion uint, hasStartVersion bool) error {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return nil
	}
	if err != nil {
		return err
	}

	if dirty {
		prev, err := src.Prev(v)
		if errors.Is(err, os.ErrNotExist) {
			return m.Force(-1)
		} else if err != nil {
			return err
		}
		if err := m.Force(int(prev)); err != nil {
			return err
		}
	}

	if hasStartVersion {
		err = m.Migrate(startVersion)
	} else {
		err = m.Down()
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
