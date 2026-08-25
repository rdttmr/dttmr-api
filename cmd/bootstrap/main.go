package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/robindittmar/dttmr-api/internal/config"
	"github.com/robindittmar/dttmr-api/internal/database"
	"github.com/robindittmar/dttmr-api/internal/database/migrations"
	"github.com/robindittmar/dttmr-api/internal/domain"
	"github.com/robindittmar/dttmr-api/internal/repository"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func main() {
	time.Local, _ = time.LoadLocation("UTC")

	_ = godotenv.Load(".env")
	setupLogging()

	email := flag.String("email", "admin@example.com", "Admin email address")
	name := flag.String("name", "admin", "Admin name")
	password := flag.String("password", "", "Admin password")
	cfg := config.Load()

	slog.Info("starting bootstrap",
		slog.String("version", Version),
		slog.String("commit", Commit),
		slog.String("build_time", BuildTime),
	)
	defer slog.Info("bootstrap complete!")

	db, err := database.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to initialize database", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			slog.Error("failed to close database connection", slog.Any("error", err))
		}
	}()

	if err := database.RunMigrations(db, migrations.MigrationFS); err != nil {
		slog.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	if *password != "" {
		err := seedAdminUser(db, *email, *name, *password)
		if err != nil {
			slog.Error("failed to seed admin user", slog.Any("error", err))
		} else {
			slog.Info("admin user seeded", slog.String("email", *email), slog.String("name", *name))
		}
	} else {
		slog.Info("skipping admin user seeding, no password provided")
	}
}

func seedAdminUser(db *sql.DB, email string, name string, password string) error {
	userRepo := repository.NewUserRepo(db)
	userService := domain.NewUserService(userRepo)

	_, err := userService.CreateUser(context.Background(), email, name, password)
	return err
}

func setupLogging() {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(baseHandler)
	slog.SetDefault(logger)
}
