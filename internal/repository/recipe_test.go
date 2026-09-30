package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

// passthroughConverter lets slice arguments (e.g. []string for uuid[]) reach
// sqlmock unchanged, like the pgx driver accepts them.
type passthroughConverter struct{}

func (passthroughConverter) ConvertValue(v any) (driver.Value, error) {
	if s, ok := v.([]string); ok {
		return s, nil
	}
	return driver.DefaultParameterConverter.ConvertValue(v)
}

func newRecipeRepo(t *testing.T) (*RecipeRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
		sqlmock.ValueConverterOption(passthroughConverter{}),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &RecipeRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	insertRecipeQuery       = "INSERT INTO recipes (name, group_id) VALUES ($1, $2) RETURNING id, created_at, modified_at"
	deleteRecipeQuery       = "DELETE FROM recipes WHERE id = $1"
	updateRecipeGroupQuery  = "UPDATE recipes SET group_id = $1 WHERE id = $2"
	deleteOrphanItemsQuery  = "DELETE FROM recipe_items ri USING list_items li, lists l WHERE ri.recipe_id = $1 AND ri.list_item_id = li.id AND li.list_id = l.id AND l.group_id <> $2"
	updateRecipeNameQuery   = "UPDATE recipes SET name = $1, modified_at = NOW() WHERE id = $2"
	selectRecipesQuery      = "SELECT r.id, r.name, r.group_id, r.created_at, r.modified_at, (SELECT COUNT(*) FROM recipe_items AS ri WHERE ri.recipe_id = r.id), COALESCE(rp.position, 0) FROM recipes AS r LEFT JOIN recipe_positions AS rp ON r.id=rp.recipe_id AND rp.user_id = $1 WHERE r.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1) ORDER BY rp.position, r.modified_at"
	isUserInRecipeQuery     = "SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = (SELECT r.group_id FROM recipes r WHERE r.id = $1) AND gm.user_id = $2"
	orderRecipesQuery       = "INSERT INTO recipe_positions (recipe_id, user_id, position) SELECT o.recipe_id, $1, o.idx - 1 FROM unnest($2::uuid[]) WITH ORDINALITY o(recipe_id, idx) ON CONFLICT (recipe_id, user_id) DO UPDATE SET position = EXCLUDED.position"
	lockRecipeOrderQuery    = "SELECT pg_advisory_xact_lock(hashtextextended('recipe_positions:' || $1::text, 0))"
	lockUserRecipesQuery    = "SELECT r.id FROM recipes r WHERE r.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1) ORDER BY r.id FOR KEY SHARE OF r"
	addRecipeItemQuery      = "INSERT INTO recipe_items (recipe_id, list_item_id) SELECT r.id, li.id FROM recipes r INNER JOIN list_items li ON li.id = $2 INNER JOIN lists l ON l.id = li.list_id AND l.group_id = r.group_id WHERE r.id = $1 FOR SHARE OF r, l"
	removeRecipeItemQuery   = "DELETE FROM recipe_items WHERE recipe_id = $1 AND list_item_id = $2"
	selectRecipeItemsQuery  = "SELECT li.id, li.list_id, li.title, li.is_completed, li.created_at, li.modified_at FROM recipe_items AS ri INNER JOIN list_items AS li ON ri.list_item_id=li.id WHERE ri.recipe_id = $1 ORDER BY ri.position"
	uncheckRecipeItemsQuery = "UPDATE list_items SET is_completed = FALSE WHERE id IN (SELECT list_item_id FROM recipe_items WHERE recipe_id = $1)"
)

var (
	recipeColumns     = []string{"id", "name", "group_id", "created_at", "modified_at", "count", "position"}
	recipeItemColumns = []string{"id", "list_id", "title", "is_completed", "created_at", "modified_at"}
)

func TestRecipeRepo_CreateRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
		modifiedAt := createdAt.Add(time.Minute)
		mock.ExpectQuery(insertRecipeQuery).
			WithArgs("Pancakes", "group-1").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "created_at", "modified_at"}).
					AddRow("recipe-1", createdAt, modifiedAt),
			)

		recipe, err := repo.CreateRecipe(context.Background(), "group-1", "Pancakes")

		require.NoError(t, err)
		assert.Equal(t, &domain.Recipe{
			ID:         "recipe-1",
			GroupID:    "group-1",
			Name:       "Pancakes",
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
		}, recipe)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectQuery(insertRecipeQuery).
			WithArgs("Pancakes", "group-1").
			WillReturnError(dbErr)

		recipe, err := repo.CreateRecipe(context.Background(), "group-1", "Pancakes")

		assert.Nil(t, recipe)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to create recipe")
	})
}

func TestRecipeRepo_DeleteRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(deleteRecipeQuery).
			WithArgs("recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteRecipe(context.Background(), "recipe-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteRecipeQuery).
			WithArgs("recipe-1").
			WillReturnError(dbErr)

		err := repo.DeleteRecipe(context.Background(), "recipe-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete recipe")
	})
}

func TestRecipeRepo_SetRecipeGroup(t *testing.T) {
	t.Run("updates group and removes items of other groups", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(updateRecipeGroupQuery).
			WithArgs("group-2", "recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanItemsQuery).
			WithArgs("recipe-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 3))

		assert.NoError(t, repo.SetRecipeGroup(context.Background(), "recipe-1", "group-2"))
	})

	t.Run("update error skips the cleanup", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectExec(updateRecipeGroupQuery).
			WithArgs("group-2", "recipe-1").
			WillReturnError(dbErr)

		err := repo.SetRecipeGroup(context.Background(), "recipe-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update recipe")
	})

	t.Run("cleanup error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("deadlock detected")
		mock.ExpectExec(updateRecipeGroupQuery).
			WithArgs("group-2", "recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanItemsQuery).
			WithArgs("recipe-1", "group-2").
			WillReturnError(dbErr)

		err := repo.SetRecipeGroup(context.Background(), "recipe-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete recipe items")
	})

	t.Run("both statements use the transaction from the context", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectBegin()
		mock.ExpectExec(updateRecipeGroupQuery).
			WithArgs("group-2", "recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanItemsQuery).
			WithArgs("recipe-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := repo.WithinTx(context.Background(), func(ctx context.Context) error {
			return repo.SetRecipeGroup(ctx, "recipe-1", "group-2")
		})

		require.NoError(t, err)
	})
}

func TestRecipeRepo_SetRecipeName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(updateRecipeNameQuery).
			WithArgs("Waffles", "recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetRecipeName(context.Background(), "recipe-1", "Waffles"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateRecipeNameQuery).
			WithArgs("Waffles", "recipe-1").
			WillReturnError(dbErr)

		err := repo.SetRecipeName(context.Background(), "recipe-1", "Waffles")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update recipe")
	})
}

func TestRecipeRepo_GetRecipes(t *testing.T) {
	createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	modifiedAt := createdAt.Add(time.Hour)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(recipeColumns).
					AddRow("recipe-1", "Pancakes", "group-1", createdAt, modifiedAt, 3, 0).
					AddRow("recipe-2", "Waffles", "group-2", createdAt, modifiedAt, 0, 1),
			)

		recipes, err := repo.GetRecipes(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.Recipe{
			{ID: "recipe-1", Name: "Pancakes", GroupID: "group-1", CreatedAt: createdAt, ModifiedAt: modifiedAt, TotalItems: 3, Position: 0},
			{ID: "recipe-2", Name: "Waffles", GroupID: "group-2", CreatedAt: createdAt, ModifiedAt: modifiedAt, TotalItems: 0, Position: 1},
		}, recipes)
	})

	t.Run("no rows returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(recipeColumns))

		recipes, err := repo.GetRecipes(context.Background(), "user-1")

		require.NoError(t, err)
		assert.NotNil(t, recipes)
		assert.Empty(t, recipes)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectRecipesQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		recipes, err := repo.GetRecipes(context.Background(), "user-1")

		assert.Nil(t, recipes)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get recipes")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(recipeColumns).
					AddRow("recipe-1", "Pancakes", "group-1", createdAt, modifiedAt, "not a number", 0),
			)

		recipes, err := repo.GetRecipes(context.Background(), "user-1")

		assert.Nil(t, recipes)
		assert.Error(t, err)
	})

	t.Run("row iteration error is returned", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(recipeColumns).
					AddRow("recipe-1", "Pancakes", "group-1", createdAt, modifiedAt, 3, 0).
					AddRow("recipe-2", "Waffles", "group-2", createdAt, modifiedAt, 0, 1).
					RowError(1, rowErr),
			)

		recipes, err := repo.GetRecipes(context.Background(), "user-1")

		assert.Nil(t, recipes)
		assert.ErrorIs(t, err, rowErr)
	})
}

func TestRecipeRepo_IsUserInRecipe(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(isUserInRecipeQuery).
			WithArgs("recipe-1", "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		ok, err := repo.IsUserInRecipe(context.Background(), "recipe-1", "user-1")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not a member", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(isUserInRecipeQuery).
			WithArgs("recipe-1", "user-2").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		ok, err := repo.IsUserInRecipe(context.Background(), "recipe-1", "user-2")

		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("invalid input syntax for type uuid")
		mock.ExpectQuery(isUserInRecipeQuery).
			WithArgs("recipe-1", "user-1").
			WillReturnError(dbErr)

		ok, err := repo.IsUserInRecipe(context.Background(), "recipe-1", "user-1")

		assert.False(t, ok)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to check if user is in recipe")
	})
}

func TestRecipeRepo_OrderUserRecipes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		ids := []string{"recipe-2", "recipe-1"}
		mock.ExpectExec(orderRecipesQuery).
			WithArgs("user-1", ids).
			WillReturnResult(sqlmock.NewResult(0, 2))

		assert.NoError(t, repo.OrderUserRecipes(context.Background(), "user-1", ids))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		ids := []string{"recipe-1"}
		dbErr := errors.New("foreign key violation")
		mock.ExpectExec(orderRecipesQuery).
			WithArgs("user-1", ids).
			WillReturnError(dbErr)

		err := repo.OrderUserRecipes(context.Background(), "user-1", ids)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to order recipes")
	})
}

func TestRecipeRepo_LockUserRecipes(t *testing.T) {
	t.Run("locks and returns recipe ids", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUserRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("recipe-1").AddRow("recipe-2"))

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"recipe-1", "recipe-2"}, ids)
	})

	t.Run("no recipes returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUserRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		require.NoError(t, err)
		assert.NotNil(t, ids)
		assert.Empty(t, ids)
	})

	t.Run("advisory lock error skips the query", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("lock timeout")
		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to lock recipe order")
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("could not obtain lock")
		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUserRecipesQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to lock users recipes")
	})

	t.Run("scan error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUserRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(nil))

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorContains(t, err, "failed to read recipe id")
	})

	t.Run("row iteration error is returned", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectExec(lockRecipeOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUserRecipesQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow("recipe-1").
					AddRow("recipe-2").
					RowError(1, rowErr),
			)

		ids, err := repo.LockUserRecipes(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, rowErr)
	})
}

func TestRecipeRepo_AddListItemToRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(addRecipeItemQuery).
			WithArgs("recipe-1", "item-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.AddListItemToRecipe(context.Background(), "recipe-1", "item-1"))
	})

	t.Run("no row inserted means item is not in the recipe's group", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(addRecipeItemQuery).
			WithArgs("recipe-1", "item-other-group").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.AddListItemToRecipe(context.Background(), "recipe-1", "item-other-group")

		assert.ErrorIs(t, err, domain.ErrListItemNotInGroup)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectExec(addRecipeItemQuery).
			WithArgs("recipe-1", "item-1").
			WillReturnError(dbErr)

		err := repo.AddListItemToRecipe(context.Background(), "recipe-1", "item-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to add list item to recipe")
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(addRecipeItemQuery).
			WithArgs("recipe-1", "item-1").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.AddListItemToRecipe(context.Background(), "recipe-1", "item-1")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})
}

func TestRecipeRepo_RemoveListItemFromRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(removeRecipeItemQuery).
			WithArgs("recipe-1", "item-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.RemoveListItemFromRecipe(context.Background(), "recipe-1", "item-1"))
	})

	t.Run("item not in recipe is not reported", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(removeRecipeItemQuery).
			WithArgs("recipe-1", "item-unknown").
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, repo.RemoveListItemFromRecipe(context.Background(), "recipe-1", "item-unknown"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(removeRecipeItemQuery).
			WithArgs("recipe-1", "item-1").
			WillReturnError(dbErr)

		err := repo.RemoveListItemFromRecipe(context.Background(), "recipe-1", "item-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to remove list item from recipe")
	})
}

func TestRecipeRepo_GetListItemsForRecipe(t *testing.T) {
	createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	modifiedAt := createdAt.Add(time.Hour)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnRows(
				sqlmock.NewRows(recipeItemColumns).
					AddRow("item-1", "list-1", "Flour", false, createdAt, modifiedAt).
					AddRow("item-2", "list-1", "Eggs", true, createdAt, modifiedAt),
			)

		items, err := repo.GetListItemsForRecipe(context.Background(), "recipe-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.ListItem{
			{ID: "item-1", ListID: "list-1", Title: "Flour", IsCompleted: false, CreatedAt: createdAt, ModifiedAt: modifiedAt},
			{ID: "item-2", ListID: "list-1", Title: "Eggs", IsCompleted: true, CreatedAt: createdAt, ModifiedAt: modifiedAt},
		}, items)
	})

	t.Run("no rows returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnRows(sqlmock.NewRows(recipeItemColumns))

		items, err := repo.GetListItemsForRecipe(context.Background(), "recipe-1")

		require.NoError(t, err)
		assert.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnError(dbErr)

		items, err := repo.GetListItemsForRecipe(context.Background(), "recipe-1")

		assert.Nil(t, items)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get list items for recipe")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectQuery(selectRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnRows(
				sqlmock.NewRows(recipeItemColumns).
					AddRow("item-1", "list-1", "Flour", "not a bool", createdAt, modifiedAt),
			)

		items, err := repo.GetListItemsForRecipe(context.Background(), "recipe-1")

		assert.Nil(t, items)
		assert.Error(t, err)
	})

	t.Run("row iteration error is returned", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnRows(
				sqlmock.NewRows(recipeItemColumns).
					AddRow("item-1", "list-1", "Flour", false, createdAt, modifiedAt).
					AddRow("item-2", "list-1", "Eggs", true, createdAt, modifiedAt).
					RowError(1, rowErr),
			)

		items, err := repo.GetListItemsForRecipe(context.Background(), "recipe-1")

		assert.Nil(t, items)
		assert.ErrorIs(t, err, rowErr)
	})
}

func TestRecipeRepo_UncheckListItemsFromRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		mock.ExpectExec(uncheckRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnResult(sqlmock.NewResult(0, 4))

		assert.NoError(t, repo.UncheckListItemsFromRecipe(context.Background(), "recipe-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newRecipeRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(uncheckRecipeItemsQuery).
			WithArgs("recipe-1").
			WillReturnError(dbErr)

		err := repo.UncheckListItemsFromRecipe(context.Background(), "recipe-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to uncheck list items from recipe")
	})
}
