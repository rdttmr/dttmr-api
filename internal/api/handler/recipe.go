package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"git.dittmar.dev/robin/dttmr-api/internal/api/request"
	"git.dittmar.dev/robin/dttmr-api/internal/api/response"
	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

type RecipeHandler struct {
	RecipeService *domain.RecipeService
}

func NewRecipeHandler(recipeService *domain.RecipeService) *RecipeHandler {
	return &RecipeHandler{RecipeService: recipeService}
}

// CreateRecipe handles the creation of a recipe
//
// @Summary Create recipe route
// @Description Create a recipe and associate current user with it
// @Tags Recipe
// @Accept json
// @Produce json
// @Param payload body request.CreateRecipePayload true "Create recipe payload"
// @Success 201 {object} domain.Recipe
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to create recipe"
// @Router /recipes [post]
func (h *RecipeHandler) CreateRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateRecipePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create recipe payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	recipe, err := h.RecipeService.CreateRecipe(ctx, authContext.UserID, payload.GroupID, payload.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create recipe", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	slog.InfoContext(ctx, "created recipe successfully", slog.String("recipe_id", recipe.ID))
	response.JSON(ctx, w, http.StatusCreated, recipe)
}

// DeleteRecipe handles the deletion of a recipe
//
// @Summary Delete recipe route
// @Description Deletes a recipe, cascading to user associations
// @Tags Recipe
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to delete recipe"
// @Router /recipes/{id} [delete]
func (h *RecipeHandler) DeleteRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recipeID := r.PathValue("id")
	if recipeID == "" {
		slog.ErrorContext(ctx, "failed to read recipe id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list")
		return
	}

	err = h.RecipeService.DeleteRecipe(ctx, authContext.UserID, recipeID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list")
		return
	}

	slog.InfoContext(ctx, "deleted recipe successfully", slog.String("recipe_id", recipeID))
	response.Status(w, http.StatusNoContent)
}

// SetRecipeGroup handles updating the group of a recipe
//
// @Summary Updates group of recipe
// @Description Update an existing recipe, moving it to a different group
// @Tags Recipe
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param payload body request.SetRecipeGroupPayload true "Update recipe group payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set recipe group"
// @Router /recipes/{id}/group [post]
func (h *RecipeHandler) SetRecipeGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recipeID := r.PathValue("id")
	if recipeID == "" {
		slog.ErrorContext(ctx, "failed to read recipe id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetListGroupPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set list group payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.RecipeService.SetRecipeGroup(ctx, authContext.UserID, recipeID, payload.GroupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set recipe group", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set recipe group")
		return
	}

	slog.InfoContext(ctx, "updated recipe group successfully", slog.String("recipe_id", recipeID))
	response.Status(w, http.StatusNoContent)
}

// SetRecipeName handles updating "name" of a recipe
//
// @Summary Updates "name" of recipe
// @Description Update an existing recipe, setting the "name" field
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "Recipe ID"
// @Param payload body request.SetRecipeNamePayload true "Update recipe name payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set recipe name"
// @Router /recipes/{id}/name [post]
func (h *RecipeHandler) SetRecipeName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recipeID := r.PathValue("id")
	if recipeID == "" {
		slog.ErrorContext(ctx, "failed to read recipe id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetRecipeNamePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set recipe name payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.RecipeService.SetRecipeName(ctx, authContext.UserID, recipeID, payload.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set recipe name", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set recipe name")
		return
	}

	slog.InfoContext(ctx, "updated recipe name successfully", slog.String("recipe_id", recipeID))
	response.Status(w, http.StatusNoContent)
}

// GetRecipes handles fetching recipes for the current user
//
// @Summary Returns all recipes of the user
// @Description Retrieve all recipes the user is a part of
// @Tags Recipe
// @Accept json
// @Produce json
// @Success 200 {object} []domain.Recipe
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read recipes"
// @Router /recipes [get]
func (h *RecipeHandler) GetRecipes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	recipes, err := h.RecipeService.GetRecipes(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get recipes", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read recipes")
		return
	}

	response.JSON(ctx, w, http.StatusOK, recipes)
}

// OrderRecipes handles re-ordering a users recipes
//
// @Summary Order recipes of a user
// @Description Re-assigns the display order of all users recipes
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.OrderRecipesPayload true "Order recipes payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to order recipes"
// @Router /recipes/order [post]
func (h *RecipeHandler) OrderRecipes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.OrderRecipesPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode order recipes payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.RecipeService.OrderRecipes(ctx, authContext.UserID, payload.RecipeIDs)
	if err != nil {
		if errors.Is(err, domain.ErrStaleRecipeIDs) {
			response.Error(ctx, w, http.StatusBadRequest, "stale recipe ids")
		} else {
			response.Error(ctx, w, http.StatusInternalServerError, "failed to order recipes")
		}

		slog.ErrorContext(ctx, "failed to order recipes", slog.Any("error", err))
		return
	}

	slog.InfoContext(ctx, "recipes re-ordered successfully", slog.String("user_id", authContext.UserID))
	response.Status(w, http.StatusNoContent)
}

// AddListItemToRecipe handles adding an existing list item to a recipe
//
// @Summary Add list item to recipe
// @Description Add an existing list item to an existing recipe
// @Tags Recipe
// @Accept json
// @Produce json
// @Param payload body request.AddListItemToRecipePayload true "Add list item to recipe payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to add list item to recipe"
// @Router /recipes/items [post]
func (h *RecipeHandler) AddListItemToRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.AddListItemToRecipePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode add list item to recipe payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	err = h.RecipeService.AddListItemToRecipe(ctx, authContext.UserID, payload.RecipeID, payload.ListItemID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInRecipe) {
			slog.ErrorContext(ctx, "user not in recipe",
				slog.String("user_id", authContext.UserID),
				slog.String("recipe_id", payload.RecipeID))
			response.Error(ctx, w, http.StatusForbidden, "insufficient permission")
		} else if errors.Is(err, domain.ErrListItemNotInGroup) {
			slog.ErrorContext(ctx, "list item not in group",
				slog.String("recipe_id", payload.RecipeID),
				slog.String("list_item_id", payload.ListItemID))
			response.Error(ctx, w, http.StatusNotFound, "list item not found")
		} else {
			slog.ErrorContext(ctx, "failed to add list item to recipe", slog.Any("error", err))
			response.Error(ctx, w, http.StatusInternalServerError, "failed to add list item to recipe")
		}
		return
	}

	slog.InfoContext(ctx, "added item to recipe successfully",
		slog.String("recipe_id", payload.RecipeID),
		slog.String("list_item_id", payload.ListItemID))
	response.Status(w, http.StatusNoContent)
}

// RemoveListItemFromRecipe handles removing a list item from a recipe
//
// @Summary Remove list item from recipe
// @Description Remove a list item from a recipe
// @Tags Recipe
// @Accept json
// @Produce json
// @Param payload body request.RemoveListItemFromRecipePayload true "Remove list item from recipe payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to remove list item from recipe"
// @Router /recipes/items [delete]
func (h *RecipeHandler) RemoveListItemFromRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.RemoveListItemFromRecipePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode remove list item from recipe payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	err = h.RecipeService.RemoveListItemFromRecipe(ctx, authContext.UserID, payload.RecipeID, payload.ListItemID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to remove list item from recipe", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to remove list item from recipe")
		return
	}

	slog.InfoContext(ctx, "removed item from recipe successfully",
		slog.String("recipe_id", payload.RecipeID),
		slog.String("list_item_id", payload.ListItemID))
	response.Status(w, http.StatusNoContent)
}

// GetListItemsFromRecipe handles fetching list items of a recipe
//
// @Summary Returns all list items of a recipes
// @Description Retrieve all list items of a recipe
// @Tags Recipe
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 200 {object} []domain.ListItem
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read list items"
// @Router /recipes/{id} [get]
func (h *RecipeHandler) GetListItemsFromRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recipeID := r.PathValue("id")
	if recipeID == "" {
		slog.ErrorContext(ctx, "failed to read recipe id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	items, err := h.RecipeService.GetListItemsForRecipe(ctx, authContext.UserID, recipeID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get list items", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read list items")
		return
	}

	response.JSON(ctx, w, http.StatusOK, items)

}

// UncheckAllItemsFromRecipe handles unchecking all list items of a recipe
//
// @Summary Uncheck list items of recipe route
// @Description Uncheck all list items part of the recipe
// @Tags Recipe
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to uncheck all items from recipe"
// @Router /recipes/{id}/uncheck [post]
func (h *RecipeHandler) UncheckAllItemsFromRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recipeID := r.PathValue("id")
	if recipeID == "" {
		slog.ErrorContext(ctx, "failed to read recipe id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.RecipeService.UncheckListItemsFromRecipe(ctx, authContext.UserID, recipeID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to uncheck all items from recipe", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to uncheck all items from recipe")
	}

	slog.InfoContext(ctx, "unchecked all items from recipe successfully", slog.String("recipe_id", recipeID))
	response.Status(w, http.StatusNoContent)
}
