package domain

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var (
	ErrRecipeIDMissing = errors.New("recipe id is required")
	ErrJoinCodeMissing = errors.New("code is required")
	ErrUserNotInRecipe = errors.New("user is not in recipe")
	ErrStaleRecipeIDs  = errors.New("recipe ids out of date")
)

type Recipe struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
	Position   int       `json:"position"`
}

type RecipeShareCode struct {
	Code string `json:"code"`
}

type RecipeRepository interface {
	CreateRecipe(ctx context.Context, name string) (*Recipe, error)
	DeleteRecipe(ctx context.Context, recipeID string) error
	SetRecipeName(ctx context.Context, recipeID string, name string) error
	GetRecipes(ctx context.Context, userID string) ([]Recipe, error)
	UpsertShareCodeHash(ctx context.Context, userID string, recipeID string, codeHash string) error
	GetRecipeIDFromShareCode(ctx context.Context, codeHash string) (string, error)
	AddUserToRecipe(ctx context.Context, recipeID string, userID string) error
	RemoveUserFromRecipe(ctx context.Context, recipeID string, userID string) error
	IsUserInRecipe(ctx context.Context, recipeID string, userID string) (bool, error)
	OrderUserRecipes(ctx context.Context, userID string, recipeIDs []string) error
	LockUserRecipes(ctx context.Context, userID string) ([]string, error)
	AddListItemToRecipe(ctx context.Context, recipeID string, listItemID string) error
	RemoveListItemFromRecipe(ctx context.Context, recipeID string, listItemID string) error
	GetListItemsForRecipe(ctx context.Context, recipeID string) ([]ListItem, error)
	UncheckListItemsFromRecipe(ctx context.Context, recipeID string) error
}

type RecipeService struct {
	tx   Transactor
	repo RecipeRepository
}

func NewRecipeService(tx Transactor, r RecipeRepository) *RecipeService {
	return &RecipeService{tx: tx, repo: r}
}

func (s *RecipeService) CreateRecipe(ctx context.Context, authUserID string, name string) (*Recipe, error) {
	if name == "" {
		return nil, ErrNameMissing
	}

	var recipe *Recipe
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.repo.CreateRecipe(ctx, name)
		if err != nil {
			return err
		}

		err = s.repo.AddUserToRecipe(ctx, r.ID, authUserID)
		if err != nil {
			return err
		}

		recipe = r
		return nil
	})
	if err != nil {
		return nil, err
	}

	return recipe, nil
}

func (s *RecipeService) DeleteRecipe(ctx context.Context, authUserID, recipeID string) error {
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

func (s *RecipeService) SetRecipeName(ctx context.Context, authUserID, recipeID string, name string) error {
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

func (s *RecipeService) ShareRecipe(ctx context.Context, authUserID string, recipeID string) (*RecipeShareCode, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if recipeID == "" {
		return nil, ErrRecipeIDMissing
	}

	code, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	err = s.repo.UpsertShareCodeHash(ctx, authUserID, recipeID, hashToken(code))
	if err != nil {
		return nil, err
	}

	return &RecipeShareCode{Code: code}, nil
}

func (s *RecipeService) JoinSharedRecipe(ctx context.Context, authUserID string, joinCode string) (string, error) {
	if authUserID == "" {
		return "", ErrUserIDMissing
	}
	if joinCode == "" {
		return "", ErrJoinCodeMissing
	}

	recipeID, err := s.repo.GetRecipeIDFromShareCode(ctx, hashToken(joinCode))
	if err != nil {
		return "", err
	}

	err = s.repo.AddUserToRecipe(ctx, recipeID, authUserID)
	if err != nil {
		return "", err
	}

	return recipeID, nil
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
