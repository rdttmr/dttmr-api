package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var _ RecipeRepository = (*mockRecipeRepository)(nil)

type mockRecipeRepository struct {
	mock.Mock
}

func (m *mockRecipeRepository) CreateRecipe(ctx context.Context, groupID string, name string) (*Recipe, error) {
	args := m.Called(ctx, groupID, name)
	recipe, _ := args.Get(0).(*Recipe)
	return recipe, args.Error(1)
}

func (m *mockRecipeRepository) DeleteRecipe(ctx context.Context, recipeID string) error {
	args := m.Called(ctx, recipeID)
	return args.Error(0)
}

func (m *mockRecipeRepository) SetRecipeGroup(ctx context.Context, recipeID string, groupID string) error {
	args := m.Called(ctx, recipeID, groupID)
	return args.Error(0)
}

func (m *mockRecipeRepository) SetRecipeName(ctx context.Context, recipeID string, name string) error {
	args := m.Called(ctx, recipeID, name)
	return args.Error(0)
}

func (m *mockRecipeRepository) GetRecipes(ctx context.Context, userID string) ([]Recipe, error) {
	args := m.Called(ctx, userID)
	recipes, _ := args.Get(0).([]Recipe)
	return recipes, args.Error(1)
}

func (m *mockRecipeRepository) IsUserInRecipe(ctx context.Context, recipeID string, userID string) (bool, error) {
	args := m.Called(ctx, recipeID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRecipeRepository) OrderUserRecipes(ctx context.Context, userID string, recipeIDs []string) error {
	args := m.Called(ctx, userID, recipeIDs)
	return args.Error(0)
}

func (m *mockRecipeRepository) LockUserRecipes(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	ids, _ := args.Get(0).([]string)
	return ids, args.Error(1)
}

func (m *mockRecipeRepository) AddListItemToRecipe(ctx context.Context, recipeID string, listItemID string) error {
	args := m.Called(ctx, recipeID, listItemID)
	return args.Error(0)
}

func (m *mockRecipeRepository) RemoveListItemFromRecipe(ctx context.Context, recipeID string, listItemID string) error {
	args := m.Called(ctx, recipeID, listItemID)
	return args.Error(0)
}

func (m *mockRecipeRepository) GetListItemsForRecipe(ctx context.Context, recipeID string) ([]ListItem, error) {
	args := m.Called(ctx, recipeID)
	items, _ := args.Get(0).([]ListItem)
	return items, args.Error(1)
}

func (m *mockRecipeRepository) UncheckListItemsFromRecipe(ctx context.Context, recipeID string) error {
	args := m.Called(ctx, recipeID)
	return args.Error(0)
}

// newRecipeServiceWithGroups wires a RecipeService to a real GroupService
// backed by a mock GroupRepository. The group service gets its own
// transactor, so tx.calls only counts transactions started by the recipe
// service.
func newRecipeServiceWithGroups(t *testing.T) (*RecipeService, *mockRecipeRepository, *mockGroupRepository, *fakeTransactor) {
	t.Helper()

	repo := &mockRecipeRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	groups := &mockGroupRepository{}
	groups.Test(t)
	t.Cleanup(func() { groups.AssertExpectations(t) })

	tx := &fakeTransactor{}
	groupService := NewGroupService(&fakeTransactor{}, groups)

	return NewRecipeService(tx, repo, groupService), repo, groups, tx
}

// newRecipeService is for tests that never reach the group service. Any
// unexpected group repository call fails the test.
func newRecipeService(t *testing.T) (*RecipeService, *mockRecipeRepository, *fakeTransactor) {
	t.Helper()

	svc, repo, _, tx := newRecipeServiceWithGroups(t)
	return svc, repo, tx
}

func assertRecipeCallOrder(t *testing.T, repo *mockRecipeRepository, want ...string) {
	t.Helper()
	assert.Equal(t, want, callOrder(repo.Calls))
}

func TestRecipeService_CreateRecipe(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the recipe in the given group", func(t *testing.T) {
		tests := []struct {
			name string
			role string
		}{
			{name: "as owner", role: RoleOwner},
			{name: "as member", role: RoleMember},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, groups, tx := newRecipeServiceWithGroups(t)

				created := &Recipe{ID: "recipe-1", GroupID: "group-1", Name: "Pancakes", CreatedAt: time.Now()}
				groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, nil)
				repo.On("CreateRecipe", mock.Anything, "group-1", "Pancakes").Return(created, nil)

				recipe, err := svc.CreateRecipe(ctx, "user-1", "group-1", "Pancakes")

				require.NoError(t, err)
				assert.Equal(t, created, recipe)
				assert.Zero(t, tx.calls)
				assertGroupCallOrder(t, groups, "GetRoleForGroup")
				assertRecipeCallOrder(t, repo, "CreateRecipe")
			})
		}
	})

	t.Run("falls back to the default group when no group id is given", func(t *testing.T) {
		svc, repo, groups, _ := newRecipeServiceWithGroups(t)

		created := &Recipe{ID: "recipe-1", GroupID: "group-default", Name: "Pancakes"}
		groups.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-default", "user-1").Return(RoleOwner, nil)
		repo.On("CreateRecipe", mock.Anything, "group-default", "Pancakes").Return(created, nil)

		recipe, err := svc.CreateRecipe(ctx, "user-1", "", "Pancakes")

		require.NoError(t, err)
		assert.Equal(t, created, recipe)
		assertGroupCallOrder(t, groups, "GetDefaultGroupID", "GetRoleForGroup")
	})

	t.Run("empty default group id is reported as missing group id", func(t *testing.T) {
		svc, _, groups, _ := newRecipeServiceWithGroups(t)

		groups.On("GetDefaultGroupID", mock.Anything, "user-1").Return("", nil)

		recipe, err := svc.CreateRecipe(ctx, "user-1", "", "Pancakes")

		assert.Nil(t, recipe)
		assert.ErrorIs(t, err, ErrGroupIDMissing)
		assertGroupCallOrder(t, groups, "GetDefaultGroupID")
	})

	t.Run("default group lookup error is propagated", func(t *testing.T) {
		svc, _, groups, _ := newRecipeServiceWithGroups(t)

		lookupErr := errors.New("no default group")
		groups.On("GetDefaultGroupID", mock.Anything, "user-1").Return("", lookupErr)

		recipe, err := svc.CreateRecipe(ctx, "user-1", "", "Pancakes")

		assert.Nil(t, recipe)
		assert.ErrorIs(t, err, lookupErr)
		assertGroupCallOrder(t, groups, "GetDefaultGroupID")
	})

	t.Run("without write permission in the group is refused", func(t *testing.T) {
		tests := []struct {
			name    string
			role    string
			roleErr error
		}{
			{name: "as viewer", role: RoleViewer},
			{name: "as non-member", roleErr: ErrUserNotInGroup},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, groups, _ := newRecipeServiceWithGroups(t)

				groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, tt.roleErr)

				recipe, err := svc.CreateRecipe(ctx, "user-1", "group-1", "Pancakes")

				assert.Nil(t, recipe)
				assert.ErrorIs(t, err, ErrUserNoWritePermissions)
				assertRecipeCallOrder(t, repo)
			})
		}
	})

	t.Run("role lookup error is propagated, not masked", func(t *testing.T) {
		svc, repo, groups, _ := newRecipeServiceWithGroups(t)

		roleErr := errors.New("connection reset")
		groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return("", roleErr)

		recipe, err := svc.CreateRecipe(ctx, "user-1", "group-1", "Pancakes")

		assert.Nil(t, recipe)
		assert.ErrorIs(t, err, roleErr)
		assert.NotErrorIs(t, err, ErrUserNoWritePermissions)
		assertRecipeCallOrder(t, repo)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, groups, _ := newRecipeServiceWithGroups(t)

		repoErr := errors.New("insert failed")
		groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)
		repo.On("CreateRecipe", mock.Anything, "group-1", "Pancakes").Return(nil, repoErr)

		recipe, err := svc.CreateRecipe(ctx, "user-1", "group-1", "Pancakes")

		assert.Nil(t, recipe)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestRecipeService_SetRecipeGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("moves the recipe inside one transaction", func(t *testing.T) {
		svc, repo, groups, tx := newRecipeServiceWithGroups(t)

		repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)
		repo.On("SetRecipeGroup", inTx, "recipe-1", "group-2").Return(nil)

		err := svc.SetRecipeGroup(ctx, "user-1", "recipe-1", "group-2")

		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls)
		assertRecipeCallOrder(t, repo, "IsUserInRecipe", "SetRecipeGroup")
		assertGroupCallOrder(t, groups, "GetRoleForGroup")
	})

	t.Run("user without access to the recipe is refused", func(t *testing.T) {
		svc, repo, groups, tx := newRecipeServiceWithGroups(t)

		repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "intruder").Return(false, nil)

		err := svc.SetRecipeGroup(ctx, "intruder", "recipe-1", "group-2")

		assert.ErrorIs(t, err, ErrUserNotInRecipe)
		assert.Zero(t, tx.calls)
		assertRecipeCallOrder(t, repo, "IsUserInRecipe")
		assertGroupCallOrder(t, groups)
	})

	t.Run("moving into a group without write permission is refused", func(t *testing.T) {
		tests := []struct {
			name    string
			role    string
			roleErr error
		}{
			{name: "as viewer of the target group", role: RoleViewer},
			{name: "as non-member of the target group", roleErr: ErrUserNotInGroup},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, groups, tx := newRecipeServiceWithGroups(t)

				repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
				groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(tt.role, tt.roleErr)

				err := svc.SetRecipeGroup(ctx, "user-1", "recipe-1", "group-2")

				assert.ErrorIs(t, err, ErrUserNoWritePermissions)
				assert.Zero(t, tx.calls)
				assertRecipeCallOrder(t, repo, "IsUserInRecipe")
			})
		}
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, groups, _ := newRecipeServiceWithGroups(t)

		repoErr := errors.New("update failed")
		repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)
		repo.On("SetRecipeGroup", inTx, "recipe-1", "group-2").Return(repoErr)

		err := svc.SetRecipeGroup(ctx, "user-1", "recipe-1", "group-2")

		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, groups, tx := newRecipeServiceWithGroups(t)

		tx.err = errors.New("could not begin transaction")
		repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)

		err := svc.SetRecipeGroup(ctx, "user-1", "recipe-1", "group-2")

		assert.ErrorIs(t, err, tx.err)
		assertRecipeCallOrder(t, repo, "IsUserInRecipe")
	})
}

func TestRecipeService_GetRecipes(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user's recipes", func(t *testing.T) {
		svc, repo, _ := newRecipeService(t)

		found := []Recipe{{ID: "recipe-1", Name: "Pancakes"}}
		repo.On("GetRecipes", mock.Anything, "user-1").Return(found, nil)

		recipes, err := svc.GetRecipes(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, found, recipes)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newRecipeService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetRecipes", mock.Anything, "user-1").Return(nil, repoErr)

		recipes, err := svc.GetRecipes(ctx, "user-1")

		assert.Nil(t, recipes)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestRecipeService_OrderRecipes(t *testing.T) {
	ctx := context.Background()

	t.Run("stores the client's order inside one transaction", func(t *testing.T) {
		svc, repo, tx := newRecipeService(t)

		repo.On("LockUserRecipes", inTx, "user-1").Return([]string{"recipe-a", "recipe-b", "recipe-c"}, nil)
		repo.On("OrderUserRecipes", inTx, "user-1", []string{"recipe-c", "recipe-a", "recipe-b"}).Return(nil)

		err := svc.OrderRecipes(ctx, "user-1", []string{"recipe-c", "recipe-a", "recipe-b"})

		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls)
		assertRecipeCallOrder(t, repo, "LockUserRecipes", "OrderUserRecipes")
	})

	t.Run("stale ids are rejected without writing", func(t *testing.T) {
		tests := []struct {
			name   string
			server []string
			client []string
		}{
			{name: "client is missing a recipe", server: []string{"a", "b", "c"}, client: []string{"a", "b"}},
			{name: "client has an extra recipe", server: []string{"a", "b"}, client: []string{"a", "b", "c"}},
			{name: "client has an unknown recipe", server: []string{"a", "b"}, client: []string{"a", "x"}},
			{name: "client repeats a recipe", server: []string{"a", "b"}, client: []string{"a", "a"}},
			{name: "user has no recipes anymore", server: []string{}, client: []string{"a"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, _ := newRecipeService(t)

				repo.On("LockUserRecipes", inTx, "user-1").Return(tt.server, nil)

				err := svc.OrderRecipes(ctx, "user-1", tt.client)

				assert.ErrorIs(t, err, ErrStaleRecipeIDs)
				assertRecipeCallOrder(t, repo, "LockUserRecipes")
			})
		}
	})

	t.Run("lock error is propagated", func(t *testing.T) {
		svc, repo, _ := newRecipeService(t)

		lockErr := errors.New("lock timeout")
		repo.On("LockUserRecipes", inTx, "user-1").Return(nil, lockErr)

		err := svc.OrderRecipes(ctx, "user-1", []string{"recipe-a"})

		assert.ErrorIs(t, err, lockErr)
		assert.NotErrorIs(t, err, ErrStaleRecipeIDs)
		assertRecipeCallOrder(t, repo, "LockUserRecipes")
	})

	t.Run("update error is propagated", func(t *testing.T) {
		svc, repo, _ := newRecipeService(t)

		updateErr := errors.New("update failed")
		repo.On("LockUserRecipes", inTx, "user-1").Return([]string{"recipe-a"}, nil)
		repo.On("OrderUserRecipes", inTx, "user-1", []string{"recipe-a"}).Return(updateErr)

		err := svc.OrderRecipes(ctx, "user-1", []string{"recipe-a"})

		assert.ErrorIs(t, err, updateErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, tx := newRecipeService(t)

		tx.err = errors.New("could not begin transaction")

		err := svc.OrderRecipes(ctx, "user-1", []string{"recipe-a"})

		assert.ErrorIs(t, err, tx.err)
		assertRecipeCallOrder(t, repo)
	})
}

func TestRecipeService_AddListItemToRecipe(t *testing.T) {
	t.Run("item from another group is reported", func(t *testing.T) {
		svc, repo, _ := newRecipeService(t)

		repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
		repo.On("AddListItemToRecipe", mock.Anything, "recipe-1", "item-foreign").Return(ErrListItemNotInGroup)

		err := svc.AddListItemToRecipe(context.Background(), "user-1", "recipe-1", "item-foreign")

		assert.ErrorIs(t, err, ErrListItemNotInGroup)
	})
}

type recipeGuardedOp struct {
	name string
	// expectRepo registers the delegated repository call, returning err.
	expectRepo func(repo *mockRecipeRepository, err error)
	// call invokes the service method on behalf of userID.
	call func(svc *RecipeService, userID string) error
}

// recipeGuardedOps covers the operations whose only access check is group
// membership via the recipe. SetRecipeGroup also checks the target group and
// is tested separately.
func recipeGuardedOps() []recipeGuardedOp {
	ctx := context.Background()

	return []recipeGuardedOp{
		{
			name: "SetRecipeName",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				repo.On("SetRecipeName", mock.Anything, "recipe-1", "Waffles").Return(err)
			},
			call: func(svc *RecipeService, userID string) error {
				return svc.SetRecipeName(ctx, userID, "recipe-1", "Waffles")
			},
		},
		{
			name: "DeleteRecipe",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				repo.On("DeleteRecipe", mock.Anything, "recipe-1").Return(err)
			},
			call: func(svc *RecipeService, userID string) error {
				return svc.DeleteRecipe(ctx, userID, "recipe-1")
			},
		},
		{
			name: "AddListItemToRecipe",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				repo.On("AddListItemToRecipe", mock.Anything, "recipe-1", "item-1").Return(err)
			},
			call: func(svc *RecipeService, userID string) error {
				return svc.AddListItemToRecipe(ctx, userID, "recipe-1", "item-1")
			},
		},
		{
			name: "RemoveListItemFromRecipe",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				repo.On("RemoveListItemFromRecipe", mock.Anything, "recipe-1", "item-1").Return(err)
			},
			call: func(svc *RecipeService, userID string) error {
				return svc.RemoveListItemFromRecipe(ctx, userID, "recipe-1", "item-1")
			},
		},
		{
			name: "GetListItemsForRecipe",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				var items []ListItem
				if err == nil {
					items = []ListItem{{ID: "item-1", Title: "Flour"}}
				}
				repo.On("GetListItemsForRecipe", mock.Anything, "recipe-1").Return(items, err)
			},
			call: func(svc *RecipeService, userID string) error {
				items, err := svc.GetListItemsForRecipe(ctx, userID, "recipe-1")
				if err == nil && len(items) != 1 {
					return errors.New("expected the repository's items on success")
				}
				if err != nil && items != nil {
					return errors.New("expected no items on failure")
				}
				return err
			},
		},
		{
			name: "UncheckListItemsFromRecipe",
			expectRepo: func(repo *mockRecipeRepository, err error) {
				repo.On("UncheckListItemsFromRecipe", mock.Anything, "recipe-1").Return(err)
			},
			call: func(svc *RecipeService, userID string) error {
				return svc.UncheckListItemsFromRecipe(ctx, userID, "recipe-1")
			},
		},
	}
}

func TestRecipeService_AccessControl(t *testing.T) {
	for _, op := range recipeGuardedOps() {
		t.Run(op.name, func(t *testing.T) {
			t.Run("member is allowed", func(t *testing.T) {
				svc, repo, _ := newRecipeService(t)

				repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
				op.expectRepo(repo, nil)

				require.NoError(t, op.call(svc, "user-1"))
				assertRecipeCallOrder(t, repo, "IsUserInRecipe", op.name)
			})

			t.Run("non-member is refused before any write", func(t *testing.T) {
				svc, repo, _ := newRecipeService(t)

				repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "intruder").Return(false, nil)

				err := op.call(svc, "intruder")

				assert.ErrorIs(t, err, ErrUserNotInRecipe)
				assertRecipeCallOrder(t, repo, "IsUserInRecipe")
			})

			t.Run("membership check error is propagated, not masked", func(t *testing.T) {
				svc, repo, _ := newRecipeService(t)

				repoErr := errors.New("connection reset")
				repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(false, repoErr)

				err := op.call(svc, "user-1")

				assert.ErrorIs(t, err, repoErr)
				assert.NotErrorIs(t, err, ErrUserNotInRecipe)
				assertRecipeCallOrder(t, repo, "IsUserInRecipe")
			})

			t.Run("repository error is propagated", func(t *testing.T) {
				svc, repo, _ := newRecipeService(t)

				repoErr := errors.New("write failed")
				repo.On("IsUserInRecipe", mock.Anything, "recipe-1", "user-1").Return(true, nil)
				op.expectRepo(repo, repoErr)

				err := op.call(svc, "user-1")

				assert.ErrorIs(t, err, repoErr)
				assertRecipeCallOrder(t, repo, "IsUserInRecipe", op.name)
			})
		})
	}
}

func TestRecipeService_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		call    func(svc *RecipeService) error
		wantErr error
	}{
		{
			name: "CreateRecipe without auth user id",
			call: func(svc *RecipeService) error {
				_, err := svc.CreateRecipe(ctx, "", "group-1", "Pancakes")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "CreateRecipe without name",
			call: func(svc *RecipeService) error {
				_, err := svc.CreateRecipe(ctx, "user-1", "group-1", "")
				return err
			},
			wantErr: ErrNameMissing,
		},
		{
			name: "CreateRecipe without name and without group id",
			call: func(svc *RecipeService) error {
				_, err := svc.CreateRecipe(ctx, "user-1", "", "")
				return err
			},
			wantErr: ErrNameMissing,
		},
		{
			name:    "DeleteRecipe without auth user id",
			call:    func(svc *RecipeService) error { return svc.DeleteRecipe(ctx, "", "recipe-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "DeleteRecipe without recipe id",
			call:    func(svc *RecipeService) error { return svc.DeleteRecipe(ctx, "user-1", "") },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "SetRecipeGroup without auth user id",
			call:    func(svc *RecipeService) error { return svc.SetRecipeGroup(ctx, "", "recipe-1", "group-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetRecipeGroup without recipe id",
			call:    func(svc *RecipeService) error { return svc.SetRecipeGroup(ctx, "user-1", "", "group-1") },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "SetRecipeGroup without group id",
			call:    func(svc *RecipeService) error { return svc.SetRecipeGroup(ctx, "user-1", "recipe-1", "") },
			wantErr: ErrGroupIDMissing,
		},
		{
			name:    "SetRecipeName without auth user id",
			call:    func(svc *RecipeService) error { return svc.SetRecipeName(ctx, "", "recipe-1", "Waffles") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetRecipeName without recipe id",
			call:    func(svc *RecipeService) error { return svc.SetRecipeName(ctx, "user-1", "", "Waffles") },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "SetRecipeName without name",
			call:    func(svc *RecipeService) error { return svc.SetRecipeName(ctx, "user-1", "recipe-1", "") },
			wantErr: ErrNameMissing,
		},
		{
			name: "GetRecipes without auth user id",
			call: func(svc *RecipeService) error {
				_, err := svc.GetRecipes(ctx, "")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "OrderRecipes without auth user id",
			call:    func(svc *RecipeService) error { return svc.OrderRecipes(ctx, "", []string{"recipe-1"}) },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "OrderRecipes without recipe ids",
			call:    func(svc *RecipeService) error { return svc.OrderRecipes(ctx, "user-1", nil) },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "AddListItemToRecipe without auth user id",
			call:    func(svc *RecipeService) error { return svc.AddListItemToRecipe(ctx, "", "recipe-1", "item-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "AddListItemToRecipe without recipe id",
			call:    func(svc *RecipeService) error { return svc.AddListItemToRecipe(ctx, "user-1", "", "item-1") },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "AddListItemToRecipe without list item id",
			call:    func(svc *RecipeService) error { return svc.AddListItemToRecipe(ctx, "user-1", "recipe-1", "") },
			wantErr: ErrListItemIDMissing,
		},
		{
			name:    "RemoveListItemFromRecipe without auth user id",
			call:    func(svc *RecipeService) error { return svc.RemoveListItemFromRecipe(ctx, "", "recipe-1", "item-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "RemoveListItemFromRecipe without recipe id",
			call:    func(svc *RecipeService) error { return svc.RemoveListItemFromRecipe(ctx, "user-1", "", "item-1") },
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "RemoveListItemFromRecipe without list item id",
			call:    func(svc *RecipeService) error { return svc.RemoveListItemFromRecipe(ctx, "user-1", "recipe-1", "") },
			wantErr: ErrListItemIDMissing,
		},
		{
			name: "GetListItemsForRecipe without auth user id",
			call: func(svc *RecipeService) error {
				_, err := svc.GetListItemsForRecipe(ctx, "", "recipe-1")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "GetListItemsForRecipe without recipe id",
			call: func(svc *RecipeService) error {
				_, err := svc.GetListItemsForRecipe(ctx, "user-1", "")
				return err
			},
			wantErr: ErrRecipeIDMissing,
		},
		{
			name:    "UncheckListItemsFromRecipe without auth user id",
			call:    func(svc *RecipeService) error { return svc.UncheckListItemsFromRecipe(ctx, "", "recipe-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "UncheckListItemsFromRecipe without recipe id",
			call:    func(svc *RecipeService) error { return svc.UncheckListItemsFromRecipe(ctx, "user-1", "") },
			wantErr: ErrRecipeIDMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, groups, tx := newRecipeServiceWithGroups(t)

			err := tt.call(svc)

			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, tx.calls)
			assertRecipeCallOrder(t, repo)
			assertGroupCallOrder(t, groups)
		})
	}
}
