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
	authRepo := repository.NewAuthRepo(cfg.Database)
	authService := domain.NewAuthService(authRepo, []byte(cfg.JWTSecret))
	authHandler := handler.NewAuthHandler(authService)

	userRepo := repository.NewUserRepo(cfg.Database)
	userService := domain.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, authService)

	listRepo := repository.NewListRepo(cfg.Database)
	listService := domain.NewListService(listRepo)
	listHandler := handler.NewListHandler(listService, userService)

	protected := middleware.WithJWT(authService)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/", handler.DefaultHandler)
	apiMux.HandleFunc("GET /health", handler.HealthHandler)

	apiMux.HandleFunc("POST /login", authHandler.Login)
	apiMux.HandleFunc("POST /login/refresh", authHandler.Refresh)
	apiMux.HandleFunc("POST /logout", authHandler.Logout)
	apiMux.HandleFunc("POST /logout/all", authHandler.LogoutAllDevices)

	apiMux.Handle("POST /users", protected(userHandler.CreateUser))
	apiMux.Handle("POST /users/password", protected(userHandler.ChangePassword))

	apiMux.Handle("POST /lists", protected(listHandler.CreateList))
	apiMux.Handle("DELETE /lists/{id}", protected(listHandler.DeleteList))
	apiMux.Handle("GET /lists", protected(listHandler.GetLists))
	apiMux.Handle("POST /lists/user", protected(listHandler.AddUserToList))
	apiMux.Handle("DELETE /lists/user", protected(listHandler.RemoveUserFromList))
	apiMux.Handle("POST /lists/item", protected(listHandler.CreateListItem))
	apiMux.Handle("DELETE /lists/item/{id}", protected(listHandler.DeleteListItem))
	apiMux.Handle("PUT /lists/item", protected(listHandler.UpdateListItem))
	apiMux.Handle("POST /lists/items/{id}", protected(listHandler.SetListItemCompleted))
	apiMux.Handle("GET /lists/{id}", protected(listHandler.GetListItems))

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMux))

	var httpHandler http.Handler = mux
	httpHandler = middleware.WithMaxBytes(1024 * 64)(httpHandler)
	httpHandler = middleware.WithTelemetry(httpHandler)

	return httpHandler
}
