package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/robindittmar/dttmr-api/internal/api/router"
	"github.com/robindittmar/dttmr-api/internal/config"
	"github.com/robindittmar/dttmr-api/internal/database"
	"github.com/robindittmar/dttmr-api/internal/telemetry"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// @title dttmr-api
// @version 0.1.0
// @description API documentation for dttmr-api service.
// @termsOfService http://swagger.io/terms/

// @contact.name Robin Dittmar
// @contact.email robindittmar@gmail.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey Bearer Token
// @in Header
// @name Authorization
func main() {
	if err := run(); err != nil {
		slog.Error("service crashed")
		os.Exit(1)
	}
}

func run() error {
	serivceName := "dttmr-api"
	time.Local, _ = time.LoadLocation("UTC")

	_ = godotenv.Load(".env")
	setupLogging()

	slog.Info("starting service",
		slog.String("service", serivceName),
		slog.String("version", Version),
		slog.String("commit", Commit),
		slog.String("build_time", BuildTime),
	)
	defer slog.Info("service shutdown!")

	cfg := config.Load()

	telCfg := telemetry.Config{
		ServiceName:    serivceName,
		ServiceVersion: Version,
		Endpoint:       cfg.OTLPEndpoint,
		Environment:    cfg.Environment,
	}
	shutdownTelemetry, err := telemetry.Init(context.Background(), telCfg)
	if err != nil {
		slog.Error("failed to initialize telemetry", err)
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Error("failed to shutdown telemetry", slog.Any("error", err))
		}
	}()

	db, err := database.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to initialize database", slog.Any("error", err))
		return err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			slog.Error("failed to close database connection", slog.Any("error", err))
		}
	}()

	srv := makeServer(db, cfg)
	go func() {
		slog.Info("starting http server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start http server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down server...", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", slog.Any("error", err))
		return err
	}

	return nil
}

func setupLogging() {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	traceHandler := &telemetry.TraceHandler{Handler: baseHandler}

	logger := slog.New(traceHandler)
	slog.SetDefault(logger)
}

func makeServer(db *sql.DB, cfg *config.Config) *http.Server {
	routerConfig := router.Config{
		Database:  db,
		JWTSecret: cfg.JWTSecret,
	}
	mux := router.NewMux(routerConfig)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return srv
}
