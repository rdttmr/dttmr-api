package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

func newListRepo(t *testing.T) (*ListRepo, sqlmock.Sqlmock) {
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

	return &ListRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	insertListQuery          = "INSERT INTO lists (name, group_id) VALUES ($1, $2) RETURNING id, created_at, modified_at"
	deleteListQuery          = "DELETE FROM lists WHERE id = $1"
	updateListGroupQuery     = "UPDATE lists SET group_id = $1, modified_at = NOW() WHERE id = $2"
	deleteOrphanRecipeItems  = "DELETE FROM recipe_items ri USING list_items li, recipes r WHERE ri.list_item_id = li.id AND li.list_id = $1 AND ri.recipe_id = r.id AND r.group_id <> $2"
	updateListNameQuery      = "UPDATE lists SET name = $1, modified_at = NOW() WHERE id = $2"
	selectListsQuery         = "SELECT l.id, l.name, l.group_id, l.created_at, l.modified_at, (SELECT COUNT(*) FROM list_items WHERE list_id=l.id), (SELECT COUNT(*) FROM list_items WHERE list_id=l.id AND is_completed=true), COALESCE(lp.position, 0) FROM lists AS l LEFT JOIN list_positions AS lp ON l.id=lp.list_id AND lp.user_id = $1 WHERE l.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1) ORDER BY lp.position, l.modified_at"
	orderListsQuery          = "INSERT INTO list_positions (list_id, user_id, position) SELECT o.list_id, $1, o.idx - 1 FROM unnest($2::uuid[]) WITH ORDINALITY o(list_id, idx) ON CONFLICT (list_id, user_id) DO UPDATE SET position = EXCLUDED.position"
	lockListOrderQuery       = "SELECT pg_advisory_xact_lock(hashtextextended('list_positions:' || $1::text, 0))"
	lockUsersListsQuery      = "SELECT l.id FROM lists l WHERE l.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1) ORDER BY l.id FOR KEY SHARE OF l"
	isUserInListQuery        = "SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = (SELECT l.group_id FROM lists l WHERE l.id = $1) AND gm.user_id = $2"
	isUserInListByItemQuery  = "SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = (SELECT l.group_id FROM list_items li INNER JOIN lists l ON li.list_id=l.id WHERE li.id = $1) AND gm.user_id = $2"
	insertListItemQuery      = "INSERT INTO list_items (list_id, title) VALUES ($1, $2) RETURNING id, is_completed, created_at, modified_at"
	deleteListItemQuery      = "DELETE FROM list_items WHERE id = $1"
	updateListItemQuery      = "UPDATE list_items SET title = $1, is_completed = $2, modified_at = NOW() WHERE id = $3"
	updateListItemTitleQuery = "UPDATE list_items SET title = $1, modified_at = NOW() WHERE id = $2"
	updateListItemDoneQuery  = "UPDATE list_items SET is_completed = $1, modified_at = NOW() WHERE id = $2"
	selectListItemsQuery     = "SELECT id, title, is_completed, created_at, modified_at FROM list_items WHERE list_id = $1 ORDER BY is_completed, modified_at DESC"
)

var (
	listColumns     = []string{"id", "name", "group_id", "created_at", "modified_at", "total_items", "completed_items", "position"}
	listItemColumns = []string{"id", "title", "is_completed", "created_at", "modified_at"}
)

func TestListRepo_CreateList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
		modifiedAt := createdAt.Add(time.Minute)
		mock.ExpectQuery(insertListQuery).
			WithArgs("Groceries", "group-1").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "created_at", "modified_at"}).
					AddRow("list-1", createdAt, modifiedAt),
			)

		list, err := repo.CreateList(context.Background(), "group-1", "Groceries")

		require.NoError(t, err)
		assert.Equal(t, &domain.List{
			ID:         "list-1",
			GroupID:    "group-1",
			Name:       "Groceries",
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
		}, list)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectQuery(insertListQuery).
			WithArgs("Groceries", "group-1").
			WillReturnError(dbErr)

		list, err := repo.CreateList(context.Background(), "group-1", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to insert list")
	})
}

func TestListRepo_DeleteList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(deleteListQuery).
			WithArgs("list-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteList(context.Background(), "list-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteListQuery).
			WithArgs("list-1").
			WillReturnError(dbErr)

		err := repo.DeleteList(context.Background(), "list-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete list")
	})
}

func TestListRepo_SetListGroup(t *testing.T) {
	t.Run("updates group and removes items from recipes of other groups", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(updateListGroupQuery).
			WithArgs("group-2", "list-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanRecipeItems).
			WithArgs("list-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 2))

		assert.NoError(t, repo.SetListGroup(context.Background(), "list-1", "group-2"))
	})

	t.Run("update error skips the cleanup", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectExec(updateListGroupQuery).
			WithArgs("group-2", "list-1").
			WillReturnError(dbErr)

		err := repo.SetListGroup(context.Background(), "list-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update list")
	})

	t.Run("cleanup error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("deadlock detected")
		mock.ExpectExec(updateListGroupQuery).
			WithArgs("group-2", "list-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanRecipeItems).
			WithArgs("list-1", "group-2").
			WillReturnError(dbErr)

		err := repo.SetListGroup(context.Background(), "list-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete orphaned list items from recipe")
	})

	t.Run("both statements use the transaction from the context", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectBegin()
		mock.ExpectExec(updateListGroupQuery).
			WithArgs("group-2", "list-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteOrphanRecipeItems).
			WithArgs("list-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := repo.WithinTx(context.Background(), func(ctx context.Context) error {
			return repo.SetListGroup(ctx, "list-1", "group-2")
		})

		require.NoError(t, err)
	})
}

func TestListRepo_SetListName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(updateListNameQuery).
			WithArgs("Hardware store", "list-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetListName(context.Background(), "list-1", "Hardware store"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateListNameQuery).
			WithArgs("Hardware store", "list-1").
			WillReturnError(dbErr)

		err := repo.SetListName(context.Background(), "list-1", "Hardware store")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update list")
	})
}

func TestListRepo_GetLists(t *testing.T) {
	createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	modifiedAt := createdAt.Add(time.Hour)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(listColumns).
					AddRow("list-1", "Groceries", "group-1", createdAt, modifiedAt, 5, 2, 0).
					AddRow("list-2", "Hardware", "group-2", createdAt, modifiedAt, 0, 0, 1),
			)

		lists, err := repo.GetLists(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.List{
			{ID: "list-1", Name: "Groceries", GroupID: "group-1", CreatedAt: createdAt, ModifiedAt: modifiedAt, TotalItems: 5, CompletedItems: 2, Position: 0},
			{ID: "list-2", Name: "Hardware", GroupID: "group-2", CreatedAt: createdAt, ModifiedAt: modifiedAt, TotalItems: 0, CompletedItems: 0, Position: 1},
		}, lists)
	})

	t.Run("no rows returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListsQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(listColumns))

		lists, err := repo.GetLists(context.Background(), "user-1")

		require.NoError(t, err)
		assert.NotNil(t, lists)
		assert.Empty(t, lists)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectListsQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		lists, err := repo.GetLists(context.Background(), "user-1")

		assert.Nil(t, lists)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get lists")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(listColumns).
					AddRow("list-1", "Groceries", "group-1", createdAt, modifiedAt, "not a number", 2, 0),
			)

		lists, err := repo.GetLists(context.Background(), "user-1")

		assert.Nil(t, lists)
		assert.Error(t, err)
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectListsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(listColumns).
					AddRow("list-1", "Groceries", "group-1", createdAt, modifiedAt, 5, 2, 0).
					AddRow("list-2", "Hardware", "group-2", createdAt, modifiedAt, 0, 0, 1).
					RowError(1, rowErr),
			)

		lists, err := repo.GetLists(context.Background(), "user-1")

		assert.Nil(t, lists)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get lists")
	})
}

func TestListRepo_OrderLists(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		ids := []string{"list-2", "list-1"}
		mock.ExpectExec(orderListsQuery).
			WithArgs("user-1", ids).
			WillReturnResult(sqlmock.NewResult(0, 2))

		assert.NoError(t, repo.OrderLists(context.Background(), "user-1", ids))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		ids := []string{"list-1"}
		dbErr := errors.New("foreign key violation")
		mock.ExpectExec(orderListsQuery).
			WithArgs("user-1", ids).
			WillReturnError(dbErr)

		err := repo.OrderLists(context.Background(), "user-1", ids)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to order lists")
	})
}

func TestListRepo_LockUsersLists(t *testing.T) {
	t.Run("locks and returns list ids", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUsersListsQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("list-1").AddRow("list-2"))

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"list-1", "list-2"}, ids)
	})

	t.Run("no lists returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUsersListsQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		require.NoError(t, err)
		assert.NotNil(t, ids)
		assert.Empty(t, ids)
	})

	t.Run("advisory lock error skips the query", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("lock timeout")
		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to lock list order")
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("could not obtain lock")
		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUsersListsQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to lock users lists")
	})

	t.Run("scan error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUsersListsQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(nil))

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorContains(t, err, "failed to read list id")
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectExec(lockListOrderQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(lockUsersListsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow("list-1").
					AddRow("list-2").
					RowError(1, rowErr),
			)

		ids, err := repo.LockUsersLists(context.Background(), "user-1")

		assert.Nil(t, ids)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to lock users lists")
	})
}

func TestListRepo_IsUserInList(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(isUserInListQuery).
			WithArgs("list-1", "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		ok, err := repo.IsUserInList(context.Background(), "list-1", "user-1")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not a member", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(isUserInListQuery).
			WithArgs("list-1", "user-2").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		ok, err := repo.IsUserInList(context.Background(), "list-1", "user-2")

		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("invalid input syntax for type uuid")
		mock.ExpectQuery(isUserInListQuery).
			WithArgs("list-1", "user-1").
			WillReturnError(dbErr)

		ok, err := repo.IsUserInList(context.Background(), "list-1", "user-1")

		assert.False(t, ok)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to check if user is in list")
	})
}

func TestListRepo_IsUserInListByItemID(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(isUserInListByItemQuery).
			WithArgs("item-1", "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		ok, err := repo.IsUserInListByItemID(context.Background(), "item-1", "user-1")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not a member", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(isUserInListByItemQuery).
			WithArgs("item-1", "user-2").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		ok, err := repo.IsUserInListByItemID(context.Background(), "item-1", "user-2")

		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("invalid input syntax for type uuid")
		mock.ExpectQuery(isUserInListByItemQuery).
			WithArgs("item-1", "user-1").
			WillReturnError(dbErr)

		ok, err := repo.IsUserInListByItemID(context.Background(), "item-1", "user-1")

		assert.False(t, ok)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to check if user is in list")
	})
}

func TestListRepo_CreateListItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
		modifiedAt := createdAt.Add(time.Minute)
		mock.ExpectQuery(insertListItemQuery).
			WithArgs("list-1", "Milk").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "is_completed", "created_at", "modified_at"}).
					AddRow("item-1", false, createdAt, modifiedAt),
			)

		item, err := repo.CreateListItem(context.Background(), "list-1", "Milk")

		require.NoError(t, err)
		assert.Equal(t, &domain.ListItem{
			ID:          "item-1",
			ListID:      "list-1",
			Title:       "Milk",
			IsCompleted: false,
			CreatedAt:   createdAt,
			ModifiedAt:  modifiedAt,
		}, item)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectQuery(insertListItemQuery).
			WithArgs("list-1", "Milk").
			WillReturnError(dbErr)

		item, err := repo.CreateListItem(context.Background(), "list-1", "Milk")

		assert.Nil(t, item)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to insert list item")
	})
}

func TestListRepo_DeleteListItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(deleteListItemQuery).
			WithArgs("item-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteListItem(context.Background(), "item-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteListItemQuery).
			WithArgs("item-1").
			WillReturnError(dbErr)

		err := repo.DeleteListItem(context.Background(), "item-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete list item")
	})
}

func TestListRepo_UpdateListItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(updateListItemQuery).
			WithArgs("Oat milk", true, "item-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.UpdateListItem(context.Background(), "item-1", "Oat milk", true))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateListItemQuery).
			WithArgs("Oat milk", true, "item-1").
			WillReturnError(dbErr)

		err := repo.UpdateListItem(context.Background(), "item-1", "Oat milk", true)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update list item")
	})
}

func TestListRepo_SetListItemTitle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectExec(updateListItemTitleQuery).
			WithArgs("Oat milk", "item-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetListItemTitle(context.Background(), "item-1", "Oat milk"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateListItemTitleQuery).
			WithArgs("Oat milk", "item-1").
			WillReturnError(dbErr)

		err := repo.SetListItemTitle(context.Background(), "item-1", "Oat milk")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update list item title")
	})
}

func TestListRepo_SetListItemCompleted(t *testing.T) {
	for _, completed := range []bool{true, false} {
		t.Run(fmt.Sprintf("success completed=%t", completed), func(t *testing.T) {
			repo, mock := newListRepo(t)

			mock.ExpectExec(updateListItemDoneQuery).
				WithArgs(completed, "item-1").
				WillReturnResult(sqlmock.NewResult(0, 1))

			assert.NoError(t, repo.SetListItemCompleted(context.Background(), "item-1", completed))
		})
	}

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateListItemDoneQuery).
			WithArgs(true, "item-1").
			WillReturnError(dbErr)

		err := repo.SetListItemCompleted(context.Background(), "item-1", true)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to complete list item")
	})
}

func TestListRepo_GetListItemsForList(t *testing.T) {
	createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	modifiedAt := createdAt.Add(time.Hour)

	t.Run("maps rows in query order and sets the list id", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListItemsQuery).
			WithArgs("list-1").
			WillReturnRows(
				sqlmock.NewRows(listItemColumns).
					AddRow("item-1", "Milk", false, createdAt, modifiedAt).
					AddRow("item-2", "Eggs", true, createdAt, modifiedAt),
			)

		items, err := repo.GetListItemsForList(context.Background(), "list-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.ListItem{
			{ID: "item-1", ListID: "list-1", Title: "Milk", IsCompleted: false, CreatedAt: createdAt, ModifiedAt: modifiedAt},
			{ID: "item-2", ListID: "list-1", Title: "Eggs", IsCompleted: true, CreatedAt: createdAt, ModifiedAt: modifiedAt},
		}, items)
	})

	t.Run("no rows returns empty non-nil slice", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListItemsQuery).
			WithArgs("list-1").
			WillReturnRows(sqlmock.NewRows(listItemColumns))

		items, err := repo.GetListItemsForList(context.Background(), "list-1")

		require.NoError(t, err)
		assert.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectListItemsQuery).
			WithArgs("list-1").
			WillReturnError(dbErr)

		items, err := repo.GetListItemsForList(context.Background(), "list-1")

		assert.Nil(t, items)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get list items for list")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newListRepo(t)

		mock.ExpectQuery(selectListItemsQuery).
			WithArgs("list-1").
			WillReturnRows(
				sqlmock.NewRows(listItemColumns).
					AddRow("item-1", "Milk", "not a bool", createdAt, modifiedAt),
			)

		items, err := repo.GetListItemsForList(context.Background(), "list-1")

		assert.Nil(t, items)
		assert.Error(t, err)
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newListRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectListItemsQuery).
			WithArgs("list-1").
			WillReturnRows(
				sqlmock.NewRows(listItemColumns).
					AddRow("item-1", "Milk", false, createdAt, modifiedAt).
					AddRow("item-2", "Eggs", true, createdAt, modifiedAt).
					RowError(1, rowErr),
			)

		items, err := repo.GetListItemsForList(context.Background(), "list-1")

		assert.Nil(t, items)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get list items for list")
	})
}
