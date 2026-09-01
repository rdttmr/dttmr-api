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
	store := repository.NewStore(cfg.Database)

	authService := domain.NewAuthService(store.Auth, []byte(cfg.JWTSecret))
	inviteService := domain.NewInviteService(store.Invite)
	userService := domain.NewUserService(store.User)
	registrationService := domain.NewRegistrationService(store, userService, inviteService)
	listService := domain.NewListService(store.List)

	authHandler := handler.NewAuthHandler(authService)
	inviteHandler := handler.NewInviteHandler(inviteService)
	userHandler := handler.NewUserHandler(userService, authService, registrationService)
	listHandler := handler.NewListHandler(listService, userService)

	protected := middleware.WithJWT(authService)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /health", handler.HealthHandler)

	// Auth
	apiMux.HandleFunc("POST /login", authHandler.Login)
	apiMux.HandleFunc("POST /login/refresh", authHandler.Refresh)
	apiMux.HandleFunc("POST /logout", authHandler.Logout)
	apiMux.HandleFunc("POST /logout/all", authHandler.LogoutAllDevices)

	// Users
	apiMux.HandleFunc("POST /users", userHandler.CreateUser)

	// User
	apiMux.Handle("POST /user/password", protected(userHandler.ChangePassword))

	// Invites
	apiMux.Handle("POST /user/invites", protected(inviteHandler.CreateInvite))
	apiMux.Handle("DELETE /user/invites/{id}", protected(inviteHandler.DeleteInvite))
	apiMux.Handle("GET /user/invites", protected(inviteHandler.GetInvites))

	// Lists
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
