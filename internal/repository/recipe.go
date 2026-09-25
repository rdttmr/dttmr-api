package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

type RecipeRepo struct {
	Repo
}

func (r *RecipeRepo) CreateRecipe(ctx context.Context, groupID string, name string) (*domain.Recipe, error) {
	recipe := &domain.Recipe{Name: name, GroupID: groupID}

	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO recipes (name, group_id) VALUES ($1, $2) RETURNING id, created_at, modified_at",
		name, groupID,
	).Scan(&recipe.ID, &recipe.CreatedAt, &recipe.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipe: %w", err)
	}

	return recipe, nil
}

func (r *RecipeRepo) DeleteRecipe(ctx context.Context, recipeID string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM recipes WHERE id = $1", recipeID)
	if err != nil {
		return fmt.Errorf("failed to delete recipe: %w", err)
	}
	return nil
}

func (r *RecipeRepo) SetRecipeGroup(ctx context.Context, recipeID string, groupID string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "UPDATE recipes SET group_id = $1 WHERE id = $2", groupID, recipeID)
	if err != nil {
		return fmt.Errorf("failed to update recipe: %w", err)
	}
	return nil
}

func (r *RecipeRepo) SetRecipeName(ctx context.Context, recipeID string, name string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE recipes SET name = $1, modified_at = NOW() WHERE id = $2",
		name, recipeID,
	)
	if err != nil {
		return fmt.Errorf("failed to update recipe: %w", err)
	}
	return nil
}

func (r *RecipeRepo) GetRecipes(ctx context.Context, userID string) ([]domain.Recipe, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT r.id, r.name, r.group_id, r.created_at, r.modified_at, (SELECT COUNT(*) FROM recipe_items AS ri WHERE ri.recipe_id = r.id), COALESCE(rp.position, 0) AS total_items FROM recipes AS r LEFT JOIN recipe_positions AS rp ON r.id=rp.recipe_id AND rp.user_id = $1 WHERE r.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1) ORDER BY rp.position, r.modified_at",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipes: %w", err)
	}
	defer rows.Close()

	recipes := make([]domain.Recipe, 0, 16)
	for rows.Next() {
		var r domain.Recipe
		err = rows.Scan(&r.ID, &r.Name, &r.GroupID, &r.CreatedAt, &r.ModifiedAt, &r.TotalItems, &r.Position)
		if err != nil {
			return nil, err
		}

		recipes = append(recipes, r)
	}

	return recipes, nil
}

func (r *RecipeRepo) IsUserInRecipe(ctx context.Context, recipeID string, userID string) (bool, error) {
	var cnt int

	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = (SELECT r.group_id FROM recipes r WHERE r.id = $1) AND gm.user_id = $2",
		recipeID, userID,
	).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is in recipe: %w", err)
	}

	return cnt > 0, nil
}

func (r *RecipeRepo) OrderUserRecipes(ctx context.Context, userID string, recipeIDs []string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE recipe_users AS ru SET position = o.idx - 1 FROM unnest($2::uuid[]) WITH ORDINALITY AS o(recipe_id, idx) WHERE ru.recipe_Id = o.recipe_id AND ru.user_id=$1",
		userID, recipeIDs,
	)
	if err != nil {
		return fmt.Errorf("failed to order recipes: %w", err)
	}

	return nil
}

func (r *RecipeRepo) LockUserRecipes(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT recipe_id FROM recipe_users WHERE user_id = $1 FOR UPDATE",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to lock users recipes: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, 16)
	for rows.Next() {
		var recipeID string
		err = rows.Scan(&recipeID)
		if err != nil {
			return nil, fmt.Errorf("failed to read recipe id: %w", err)
		}

		ids = append(ids, recipeID)
	}

	return ids, nil
}

func (r *RecipeRepo) AddListItemToRecipe(ctx context.Context, recipeID string, listItemID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO recipe_items (recipe_id, list_item_id) VALUES ($1, $2)",
		recipeID, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to add list item to recipe: %w", err)
	}

	return nil
}

func (r *RecipeRepo) RemoveListItemFromRecipe(ctx context.Context, recipeID string, listItemID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"DELETE FROM recipe_items WHERE recipe_id = $1 AND list_item_id = $2",
		recipeID, listItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove list item from recipe: %w", err)
	}

	return nil
}

func (r *RecipeRepo) GetListItemsForRecipe(ctx context.Context, recipeID string) ([]domain.ListItem, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT li.id, li.list_id, li.title, li.is_completed, li.created_at, li.modified_at FROM recipe_items AS ri INNER JOIN list_items AS li ON ri.list_item_id=li.id WHERE ri.recipe_id = $1 ORDER BY ri.position",
		recipeID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get list items for recipe: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ListItem, 0, 16)
	for rows.Next() {
		var l domain.ListItem
		err = rows.Scan(&l.ID, &l.ListID, &l.Title, &l.IsCompleted, &l.CreatedAt, &l.ModifiedAt)
		if err != nil {
			return nil, err
		}

		items = append(items, l)
	}

	return items, nil
}

func (r *RecipeRepo) UncheckListItemsFromRecipe(ctx context.Context, recipeID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE list_items SET is_completed = FALSE WHERE id IN (SELECT list_item_id FROM recipe_items WHERE recipe_id = $1)",
		recipeID,
	)
	if err != nil {
		return fmt.Errorf("failed to uncheck list items from recipe: %w", err)
	}

	return nil
}
