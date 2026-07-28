package router

import (
	"database/sql"
	"net/http"

	"github.com/robindittmar/dttmr-api/internal/api/handler"
	"github.com/robindittmar/dttmr-api/internal/api/middleware"
	"github.com/robindittmar/dttmr-api/internal/domain"
	"github.com/robindittmar/dttmr-api/internal/repository"
)

type Config struct {
	Database  *sql.DB
	JWTSecret string
}

func NewMux(cfg Config) http.Handler {
	userRepo := repository.NewUserRepo(cfg.Database)
	userService := domain.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	authRepo := repository.NewAuthRepo(cfg.Database)
	authService := domain.NewAuthService(authRepo, []byte(cfg.JWTSecret))
	authHandler := handler.NewAuthHandler(authService)

	listRepo := repository.NewListRepo(cfg.Database)
	listService := domain.NewListService(listRepo)
	listHandler := handler.NewListHandler(listService)

	protected := middleware.WithJWT(authService)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/", handler.DefaultHandler)
	apiMux.HandleFunc("POST /login", authHandler.Login)

	apiMux.Handle("POST /users", protected(userHandler.CreateUser))

	apiMux.Handle("POST /lists", protected(listHandler.CreateList))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.HealthHandler)
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMux))

	var httpHandler http.Handler = mux
	httpHandler = middleware.WithMaxBytes(1024 * 64)(httpHandler)
	httpHandler = middleware.WithTelemetry(httpHandler)

	return httpHandler
}
