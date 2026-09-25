package router

import (
	"database/sql"
	"net/http"

	"git.dittmar.dev/robin/dttmr-api/internal/api/handler"
	"git.dittmar.dev/robin/dttmr-api/internal/api/middleware"
	"git.dittmar.dev/robin/dttmr-api/internal/domain"
	"git.dittmar.dev/robin/dttmr-api/internal/repository"
)

type Config struct {
	Database         *sql.DB
	JWTSecret        string
	ServiceVersion   string
	ServiceCommit    string
	ServiceBuildTime string
}

func NewMux(cfg Config) http.Handler {
	store := repository.NewStore(cfg.Database)

	authService := domain.NewAuthService(store.Auth, []byte(cfg.JWTSecret))
	inviteService := domain.NewInviteService(store.Invite)
	userService := domain.NewUserService(store.User)
	groupService := domain.NewGroupService(store.Group)
	registrationService := domain.NewRegistrationService(store, userService, inviteService)
	listService := domain.NewListService(store, store.List)
	recipeService := domain.NewRecipeService(store, store.Recipe)
	exerciseService := domain.NewExerciseService(store.Exercise)

	authHandler := handler.NewAuthHandler(authService)
	inviteHandler := handler.NewInviteHandler(inviteService)
	userHandler := handler.NewUserHandler(userService, authService, registrationService)
	listHandler := handler.NewListHandler(listService, userService, groupService)
	recipeHandler := handler.NewRecipeHandler(recipeService)
	exerciseHandler := handler.NewExerciseHandler(exerciseService)

	protected := middleware.WithJWT(authService)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /version", handler.VersionHandler(
		cfg.ServiceVersion, cfg.ServiceCommit, cfg.ServiceBuildTime))
	apiMux.HandleFunc("GET /health", handler.HealthHandler)

	// Auth
	apiMux.HandleFunc("POST /login", authHandler.Login)
	apiMux.HandleFunc("POST /login/refresh", authHandler.Refresh)
	apiMux.HandleFunc("POST /logout", authHandler.Logout)
	apiMux.HandleFunc("POST /logout/all", protected(authHandler.LogoutAllDevices))

	// Users
	apiMux.HandleFunc("POST /users", userHandler.CreateUser)

	// User
	apiMux.Handle("POST /user/password", protected(userHandler.ChangePassword))

	// Invites
	apiMux.Handle("POST /user/invites", protected(inviteHandler.CreateInvite))
	apiMux.Handle("DELETE /user/invites/{id}", protected(inviteHandler.DeleteInvite))
	apiMux.Handle("GET /user/invites", protected(inviteHandler.GetInvites))
	apiMux.Handle("GET /user/invites/status", protected(inviteHandler.GetInvitesStatus))

	// Lists
	apiMux.Handle("POST /lists", protected(listHandler.CreateList))
	apiMux.Handle("DELETE /lists/{id}", protected(listHandler.DeleteList))
	apiMux.Handle("POST /lists/{id}/name", protected(listHandler.SetListName))
	apiMux.Handle("GET /lists", protected(listHandler.GetLists))
	apiMux.Handle("POST /lists/order", protected(listHandler.OrderLists))
	apiMux.Handle("POST /lists/items", protected(listHandler.CreateListItem))
	apiMux.Handle("DELETE /lists/items/{id}", protected(listHandler.DeleteListItem))
	apiMux.Handle("PUT /lists/items", protected(listHandler.UpdateListItem))
	apiMux.Handle("POST /lists/items/{id}/title", protected(listHandler.SetListItemTitle))
	apiMux.Handle("POST /lists/items/{id}/complete", protected(listHandler.SetListItemCompleted))
	apiMux.Handle("GET /lists/{id}", protected(listHandler.GetListItemsForList))

	// Recipes
	apiMux.Handle("POST /recipes", protected(recipeHandler.CreateRecipe))
	apiMux.Handle("DELETE /recipes/{id}", protected(recipeHandler.DeleteRecipe))
	apiMux.Handle("POST /recipes/{id}/name", protected(recipeHandler.SetRecipeName))
	apiMux.Handle("GET /recipes", protected(recipeHandler.GetRecipes))
	apiMux.Handle("POST /recipes/{id}/share", protected(recipeHandler.ShareRecipe))
	apiMux.Handle("POST /recipes/{code}/join", protected(recipeHandler.JoinSharedRecipe))
	apiMux.Handle("POST /recipes/order", protected(recipeHandler.OrderRecipes))
	apiMux.Handle("POST /recipes/items", protected(recipeHandler.AddListItemToRecipe))
	apiMux.Handle("DELETE /recipes/items", protected(recipeHandler.RemoveListItemFromRecipe))
	apiMux.Handle("GET /recipes/{id}", protected(recipeHandler.GetListItemsFromRecipe))
	apiMux.Handle("POST /recipes/{id}/uncheck", protected(recipeHandler.UncheckAllItemsFromRecipe))

	// Exercises
	apiMux.Handle("GET /exercises", protected(exerciseHandler.GetExercises))

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMux))

	var httpHandler http.Handler = mux
	httpHandler = middleware.WithMaxBytes(1024 * 64)(httpHandler)
	httpHandler = middleware.WithTelemetry(httpHandler)

	return httpHandler
}
