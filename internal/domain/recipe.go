package domain

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var (
	ErrRecipeIDMissing    = errors.New("recipe id is required")
	ErrJoinCodeMissing    = errors.New("code is required")
	ErrUserNotInRecipe    = errors.New("user is not in recipe")
	ErrStaleRecipeIDs     = errors.New("recipe ids out of date")
	ErrListItemNotInGroup = errors.New("list item is not in group")
)

type Recipe struct {
	ID         string    `json:"id"`
	GroupID    string    `json:"group_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
	TotalItems int       `json:"total_items"`
	Position   int       `json:"position"`
}

type RecipeShareCode struct {
	Code string `json:"code"`
}

type RecipeRepository interface {
	CreateRecipe(ctx context.Context, groupID string, name string) (*Recipe, error)
	DeleteRecipe(ctx context.Context, recipeID string) error
	SetRecipeGroup(ctx context.Context, recipeID string, groupID string) error
	SetRecipeName(ctx context.Context, recipeID string, name string) error
	GetRecipes(ctx context.Context, userID string) ([]Recipe, error)
	IsUserInRecipe(ctx context.Context, recipeID string, userID string) (bool, error)
	OrderUserRecipes(ctx context.Context, userID string, recipeIDs []string) error
	LockUserRecipes(ctx context.Context, userID string) ([]string, error)
	AddListItemToRecipe(ctx context.Context, recipeID string, listItemID string) error
	RemoveListItemFromRecipe(ctx context.Context, recipeID string, listItemID string) error
	GetListItemsForRecipe(ctx context.Context, recipeID string) ([]ListItem, error)
	UncheckListItemsFromRecipe(ctx context.Context, recipeID string) error
}

type RecipeService struct {
	tx           Transactor
	repo         RecipeRepository
	GroupService *GroupService
}

func NewRecipeService(tx Transactor, r RecipeRepository, groupService *GroupService) *RecipeService {
	return &RecipeService{tx: tx, repo: r, GroupService: groupService}
}

func (s *RecipeService) CreateRecipe(ctx context.Context, authUserID string, groupID string, name string) (*Recipe, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if name == "" {
		return nil, ErrNameMissing
	}
	if groupID == "" {
		var err error
		groupID, err = s.GroupService.GetDefaultGroupID(ctx, authUserID)
		if err != nil {
			return nil, err
		}
		if groupID == "" {
			return nil, ErrGroupIDMissing
		}
	}

	if err := s.GroupService.UserHasWritePermission(ctx, authUserID, groupID); err != nil {
		return nil, err
	}

	return s.repo.CreateRecipe(ctx, groupID, name)
}

func (s *RecipeService) DeleteRecipe(ctx context.Context, authUserID string, recipeID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}

	return s.repo.DeleteRecipe(ctx, recipeID)
}

func (s *RecipeService) SetRecipeGroup(ctx context.Context, authUserID string, recipeID string, groupID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}
	if groupID == "" {
		return ErrGroupIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}
	if err := s.GroupService.UserHasWritePermission(ctx, authUserID, groupID); err != nil {
		return err
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.repo.SetRecipeGroup(ctx, recipeID, groupID)
	})
}

func (s *RecipeService) SetRecipeName(ctx context.Context, authUserID string, recipeID string, name string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}
	if name == "" {
		return ErrNameMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}

	return s.repo.SetRecipeName(ctx, recipeID, name)
}

func (s *RecipeService) GetRecipes(ctx context.Context, authUserID string) ([]Recipe, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}

	return s.repo.GetRecipes(ctx, authUserID)
}

func (s *RecipeService) OrderRecipes(ctx context.Context, authUserID string, recipeIDs []string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if len(recipeIDs) == 0 {
		return ErrRecipeIDMissing
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		serverIDs, err := s.repo.LockUserRecipes(ctx, authUserID)
		if err != nil {
			return err
		}

		if !IsPermutation(recipeIDs, serverIDs) {
			slog.ErrorContext(ctx, "no permutation",
				slog.Any("client_recipe_ids", recipeIDs),
				slog.Any("server_recipe_ids", serverIDs))
			return ErrStaleRecipeIDs
		}

		err = s.repo.OrderUserRecipes(ctx, authUserID, recipeIDs)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *RecipeService) AddListItemToRecipe(ctx context.Context, authUserID string, recipeID string, listItemID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}
	if listItemID == "" {
		return ErrListItemIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}

	return s.repo.AddListItemToRecipe(ctx, recipeID, listItemID)
}

func (s *RecipeService) RemoveListItemFromRecipe(ctx context.Context, authUserID string, recipeID string, listItemID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}
	if listItemID == "" {
		return ErrListItemIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}

	return s.repo.RemoveListItemFromRecipe(ctx, recipeID, listItemID)
}

func (s *RecipeService) GetListItemsForRecipe(ctx context.Context, authUserID string, recipeID string) ([]ListItem, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if recipeID == "" {
		return nil, ErrRecipeIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return nil, err
	}

	return s.repo.GetListItemsForRecipe(ctx, recipeID)
}

func (s *RecipeService) UncheckListItemsFromRecipe(ctx context.Context, authUserID string, recipeID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if recipeID == "" {
		return ErrRecipeIDMissing
	}

	if err := s.userAllowedToAccessRecipe(ctx, authUserID, recipeID); err != nil {
		return err
	}

	return s.repo.UncheckListItemsFromRecipe(ctx, recipeID)
}

func (s *RecipeService) userAllowedToAccessRecipe(ctx context.Context, authUserID string, recipeID string) error {
	inList, err := s.repo.IsUserInRecipe(ctx, recipeID, authUserID)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to check if user is in recipe",
			slog.String("user_id", authUserID),
			slog.String("recipe_id", recipeID),
			slog.Any("error", err),
		)
		return err
	}
	if !inList {
		slog.WarnContext(ctx,
			"user tried to access recipe without permission",
			slog.String("user_id", authUserID),
			slog.String("recipe_id", recipeID),
		)
		return ErrUserNotInRecipe
	}

	return nil
}
