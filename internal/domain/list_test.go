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

var (
	_ Transactor      = (*fakeTransactor)(nil)
	_ ListRepository  = (*mockListRepository)(nil)
	_ GroupRepository = (*mockGroupRepository)(nil)
)

type txCtxKey struct{}
type fakeTransactor struct {
	calls int
	err   error
}

func (f *fakeTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return fn(context.WithValue(ctx, txCtxKey{}, true))
}

var inTx = mock.MatchedBy(func(ctx context.Context) bool {
	v, _ := ctx.Value(txCtxKey{}).(bool)
	return v
})

type mockListRepository struct {
	mock.Mock
}

func (m *mockListRepository) CreateList(ctx context.Context, groupID string, name string) (*List, error) {
	args := m.Called(ctx, groupID, name)
	list, _ := args.Get(0).(*List)
	return list, args.Error(1)
}

func (m *mockListRepository) DeleteList(ctx context.Context, listID string) error {
	args := m.Called(ctx, listID)
	return args.Error(0)
}

func (m *mockListRepository) SetListGroup(ctx context.Context, listID string, groupID string) error {
	args := m.Called(ctx, listID, groupID)
	return args.Error(0)
}

func (m *mockListRepository) SetListName(ctx context.Context, listID string, name string) error {
	args := m.Called(ctx, listID, name)
	return args.Error(0)
}

func (m *mockListRepository) GetLists(ctx context.Context, userID string) ([]List, error) {
	args := m.Called(ctx, userID)
	lists, _ := args.Get(0).([]List)
	return lists, args.Error(1)
}

func (m *mockListRepository) OrderLists(ctx context.Context, userID string, listIDs []string) error {
	args := m.Called(ctx, userID, listIDs)
	return args.Error(0)
}

func (m *mockListRepository) LockUsersLists(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	ids, _ := args.Get(0).([]string)
	return ids, args.Error(1)
}

func (m *mockListRepository) IsUserInList(ctx context.Context, listID string, userID string) (bool, error) {
	args := m.Called(ctx, listID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockListRepository) IsUserInListByItemID(ctx context.Context, listItemID string, userID string) (bool, error) {
	args := m.Called(ctx, listItemID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockListRepository) CreateListItem(ctx context.Context, listID string, title string) (*ListItem, error) {
	args := m.Called(ctx, listID, title)
	item, _ := args.Get(0).(*ListItem)
	return item, args.Error(1)
}

func (m *mockListRepository) DeleteListItem(ctx context.Context, listItemID string) error {
	args := m.Called(ctx, listItemID)
	return args.Error(0)
}

func (m *mockListRepository) UpdateListItem(ctx context.Context, listItemID string, title string, isCompleted bool) error {
	args := m.Called(ctx, listItemID, title, isCompleted)
	return args.Error(0)
}

func (m *mockListRepository) SetListItemTitle(ctx context.Context, listItemID string, title string) error {
	args := m.Called(ctx, listItemID, title)
	return args.Error(0)
}

func (m *mockListRepository) SetListItemCompleted(ctx context.Context, listItemID string, isCompleted bool) error {
	args := m.Called(ctx, listItemID, isCompleted)
	return args.Error(0)
}

func (m *mockListRepository) GetListItemsForList(ctx context.Context, listID string) ([]ListItem, error) {
	args := m.Called(ctx, listID)
	items, _ := args.Get(0).([]ListItem)
	return items, args.Error(1)
}

type mockGroupRepository struct {
	mock.Mock
}

func (m *mockGroupRepository) CreateGroup(ctx context.Context, name string, createdBy string) (*Group, error) {
	args := m.Called(ctx, name, createdBy)
	group, _ := args.Get(0).(*Group)
	return group, args.Error(1)
}

func (m *mockGroupRepository) DeleteGroup(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockGroupRepository) SetGroupName(ctx context.Context, id string, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroups(ctx context.Context, userID string) ([]Group, error) {
	args := m.Called(ctx, userID)
	groups, _ := args.Get(0).([]Group)
	return groups, args.Error(1)
}

func (m *mockGroupRepository) CreateGroupInvite(ctx context.Context, groupID string, codeHash string, expiresAt time.Time, createdBy string) (*GroupInvite, error) {
	args := m.Called(ctx, groupID, codeHash, expiresAt, createdBy)
	invite, _ := args.Get(0).(*GroupInvite)
	return invite, args.Error(1)
}

func (m *mockGroupRepository) DeleteGroupInvite(ctx context.Context, inviteID string, createdBy string) error {
	args := m.Called(ctx, inviteID, createdBy)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroupInvite(ctx context.Context, codeHash string) (*GroupInvite, error) {
	args := m.Called(ctx, codeHash)
	invite, _ := args.Get(0).(*GroupInvite)
	return invite, args.Error(1)
}

func (m *mockGroupRepository) ConsumeGroupInvite(ctx context.Context, inviteID string, usedBy string) error {
	args := m.Called(ctx, inviteID, usedBy)
	return args.Error(0)
}

func (m *mockGroupRepository) AddUserToGroup(ctx context.Context, groupID string, userID string, role string) error {
	args := m.Called(ctx, groupID, userID, role)
	return args.Error(0)
}

func (m *mockGroupRepository) RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	args := m.Called(ctx, groupID)
	members, _ := args.Get(0).([]GroupMember)
	return members, args.Error(1)
}

func (m *mockGroupRepository) IsUserInGroup(ctx context.Context, groupID string, userID string) (bool, error) {
	args := m.Called(ctx, groupID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockGroupRepository) GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error) {
	args := m.Called(ctx, groupID, userID)
	return args.String(0), args.Error(1)
}

func (m *mockGroupRepository) GetDefaultGroupID(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *mockGroupRepository) SetDefaultGroupID(ctx context.Context, userID string, groupID string) error {
	args := m.Called(ctx, userID, groupID)
	return args.Error(0)
}

// newListServiceWithGroups wires a ListService to a real GroupService backed
// by a mock GroupRepository. The group service gets its own transactor, so
// tx.calls only counts transactions started by the list service.
func newListServiceWithGroups(t *testing.T) (*ListService, *mockListRepository, *mockGroupRepository, *fakeTransactor) {
	t.Helper()

	repo := &mockListRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	groups := &mockGroupRepository{}
	groups.Test(t)
	t.Cleanup(func() { groups.AssertExpectations(t) })

	tx := &fakeTransactor{}
	groupService := NewGroupService(&fakeTransactor{}, groups)

	return NewListService(tx, repo, groupService), repo, groups, tx
}

// newListService is for tests that never reach the group service. Any
// unexpected group repository call fails the test.
func newListService(t *testing.T) (*ListService, *mockListRepository, *fakeTransactor) {
	t.Helper()

	svc, repo, _, tx := newListServiceWithGroups(t)
	return svc, repo, tx
}

func callOrder(calls []mock.Call) []string {
	var got []string
	for _, c := range calls {
		got = append(got, c.Method)
	}
	return got
}

func assertCallOrder(t *testing.T, repo *mockListRepository, want ...string) {
	t.Helper()
	assert.Equal(t, want, callOrder(repo.Calls))
}

func assertGroupCallOrder(t *testing.T, groups *mockGroupRepository, want ...string) {
	t.Helper()
	assert.Equal(t, want, callOrder(groups.Calls))
}

func TestListService_CreateList(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the list in the given group", func(t *testing.T) {
		tests := []struct {
			name string
			role string
		}{
			{name: "as owner", role: RoleOwner},
			{name: "as member", role: RoleMember},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, groups, tx := newListServiceWithGroups(t)

				created := &List{ID: "list-1", GroupID: "group-1", Name: "Groceries", CreatedAt: time.Now()}
				groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, nil)
				repo.On("CreateList", mock.Anything, "group-1", "Groceries").Return(created, nil)

				list, err := svc.CreateList(ctx, "user-1", "group-1", "Groceries")

				require.NoError(t, err)
				assert.Equal(t, created, list)
				assert.Zero(t, tx.calls)
				assertGroupCallOrder(t, groups, "GetRoleForGroup")
				assertCallOrder(t, repo, "CreateList")
			})
		}
	})

	t.Run("falls back to the default group when no group id is given", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		created := &List{ID: "list-1", GroupID: "group-default", Name: "Groceries"}
		groups.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-default", "user-1").Return(RoleOwner, nil)
		repo.On("CreateList", mock.Anything, "group-default", "Groceries").Return(created, nil)

		list, err := svc.CreateList(ctx, "user-1", "", "Groceries")

		require.NoError(t, err)
		assert.Equal(t, created, list)
		assertGroupCallOrder(t, groups, "GetDefaultGroupID", "GetRoleForGroup")
		assertCallOrder(t, repo, "CreateList")
	})

	t.Run("default group lookup error is returned before any write", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		lookupErr := errors.New("no default group")
		groups.On("GetDefaultGroupID", mock.Anything, "user-1").Return("", lookupErr)

		list, err := svc.CreateList(ctx, "user-1", "", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, lookupErr)
		assertGroupCallOrder(t, groups, "GetDefaultGroupID")
		assertCallOrder(t, repo)
	})

	t.Run("non-member of the group is refused before any write", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		groups.On("GetRoleForGroup", mock.Anything, "group-foreign", "user-1").Return("", ErrUserNotInGroup)

		list, err := svc.CreateList(ctx, "user-1", "group-foreign", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, ErrUserNoWritePermissions)
		assertCallOrder(t, repo)
	})

	t.Run("role lookup error is propagated, not masked", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		repoErr := errors.New("connection reset")
		groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return("", repoErr)

		list, err := svc.CreateList(ctx, "user-1", "group-1", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, repoErr)
		assert.NotErrorIs(t, err, ErrUserNoWritePermissions)
		assertCallOrder(t, repo)
	})

	t.Run("insert error is propagated", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		repoErr := errors.New("insert failed")
		groups.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleMember, nil)
		repo.On("CreateList", mock.Anything, "group-1", "Groceries").Return(nil, repoErr)

		list, err := svc.CreateList(ctx, "user-1", "group-1", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestListService_SetListGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("moves the list inside one transaction", func(t *testing.T) {
		svc, repo, groups, tx := newListServiceWithGroups(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)
		repo.On("SetListGroup", inTx, "list-1", "group-2").Return(nil)

		err := svc.SetListGroup(ctx, "user-1", "list-1", "group-2")

		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls)
		assertCallOrder(t, repo, "IsUserInList", "SetListGroup")
		assertGroupCallOrder(t, groups, "GetRoleForGroup")
	})

	t.Run("user without access to the list is refused", func(t *testing.T) {
		svc, repo, groups, tx := newListServiceWithGroups(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "intruder").Return(false, nil)

		err := svc.SetListGroup(ctx, "intruder", "list-1", "group-2")

		assert.ErrorIs(t, err, ErrUserNotInList)
		assert.Zero(t, tx.calls)
		assertCallOrder(t, repo, "IsUserInList")
		assertGroupCallOrder(t, groups)
	})

	t.Run("moving into a group the user is not in is refused", func(t *testing.T) {
		svc, repo, groups, tx := newListServiceWithGroups(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-foreign", "user-1").Return("", ErrUserNotInGroup)

		err := svc.SetListGroup(ctx, "user-1", "list-1", "group-foreign")

		assert.ErrorIs(t, err, ErrUserNoWritePermissions)
		assert.Zero(t, tx.calls)
		assertCallOrder(t, repo, "IsUserInList")
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, groups, _ := newListServiceWithGroups(t)

		repoErr := errors.New("update failed")
		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)
		repo.On("SetListGroup", inTx, "list-1", "group-2").Return(repoErr)

		err := svc.SetListGroup(ctx, "user-1", "list-1", "group-2")

		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, groups, tx := newListServiceWithGroups(t)

		tx.err = errors.New("could not begin transaction")
		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		groups.On("GetRoleForGroup", mock.Anything, "group-2", "user-1").Return(RoleMember, nil)

		err := svc.SetListGroup(ctx, "user-1", "list-1", "group-2")

		assert.ErrorIs(t, err, tx.err)
		assertCallOrder(t, repo, "IsUserInList")
	})
}

func TestListService_DeleteList(t *testing.T) {
	svc, repo, _ := newListService(t)

	repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
	repo.On("DeleteList", mock.Anything, "list-1").Return(nil)

	err := svc.DeleteList(context.Background(), "user-1", "list-1")

	require.NoError(t, err)
	assertCallOrder(t, repo, "IsUserInList", "DeleteList")
}

func TestListService_SetListName(t *testing.T) {
	svc, repo, _ := newListService(t)

	repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
	repo.On("SetListName", mock.Anything, "list-1", "name-1").Return(nil)

	err := svc.SetListName(context.Background(), "user-1", "list-1", "name-1")

	require.NoError(t, err)
	assertCallOrder(t, repo, "IsUserInList", "SetListName")
}

func TestListService_GetLists(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user's lists", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		want := []List{
			{ID: "list-1", GroupID: "group-1", Name: "Groceries"},
			{ID: "list-2", GroupID: "group-2", Name: "Reading"},
		}
		repo.On("GetLists", mock.Anything, "user-1").Return(want, nil)

		lists, err := svc.GetLists(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, want, lists)
	})

	t.Run("user without lists gets an empty result", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("GetLists", mock.Anything, "user-1").Return([]List{}, nil)

		lists, err := svc.GetLists(ctx, "user-1")

		require.NoError(t, err)
		assert.Empty(t, lists)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetLists", mock.Anything, "user-1").Return(nil, repoErr)

		lists, err := svc.GetLists(ctx, "user-1")

		assert.Nil(t, lists)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestListService_OrderLists(t *testing.T) {
	ctx := context.Background()

	t.Run("stores the client's order inside one transaction", func(t *testing.T) {
		svc, repo, tx := newListService(t)

		repo.On("LockUsersLists", inTx, "user-1").Return([]string{"list-a", "list-b", "list-c"}, nil)
		repo.On("OrderLists", inTx, "user-1", []string{"list-c", "list-a", "list-b"}).Return(nil)

		err := svc.OrderLists(ctx, "user-1", []string{"list-c", "list-a", "list-b"})

		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls)
		assertCallOrder(t, repo, "LockUsersLists", "OrderLists")
	})

	t.Run("ids are compared case-insensitively", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		server := []string{
			"3f2a8c1e-7d4b-4e21-9a6f-2b1c0d9e8f7a",
			"9b1d4e6f-0a2c-4b3d-8e5f-6a7b8c9d0e1f",
		}
		client := []string{
			"9B1D4E6F-0A2C-4B3D-8E5F-6A7B8C9D0E1F",
			"3F2A8C1E-7D4B-4E21-9A6F-2B1C0D9E8F7A",
		}
		repo.On("LockUsersLists", inTx, "user-1").Return(server, nil)
		// The client's spelling is passed on unchanged; Postgres' uuid cast
		// doesn't care about case.
		repo.On("OrderLists", inTx, "user-1", client).Return(nil)

		err := svc.OrderLists(ctx, "user-1", client)

		require.NoError(t, err)
	})

	t.Run("stale ids are rejected without writing", func(t *testing.T) {
		tests := []struct {
			name   string
			server []string
			client []string
		}{
			{name: "client is missing a list", server: []string{"a", "b", "c"}, client: []string{"a", "b"}},
			{name: "client has an extra list", server: []string{"a", "b"}, client: []string{"a", "b", "c"}},
			{name: "client has an unknown list", server: []string{"a", "b"}, client: []string{"a", "x"}},
			{name: "client repeats a list", server: []string{"a", "b"}, client: []string{"a", "a"}},
			{name: "user has no lists anymore", server: []string{}, client: []string{"a"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, _ := newListService(t)

				repo.On("LockUsersLists", inTx, "user-1").Return(tt.server, nil)

				err := svc.OrderLists(ctx, "user-1", tt.client)

				assert.ErrorIs(t, err, ErrStaleListIDs)
				assertCallOrder(t, repo, "LockUsersLists")
			})
		}
	})

	t.Run("lock error is propagated", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		lockErr := errors.New("lock timeout")
		repo.On("LockUsersLists", inTx, "user-1").Return(nil, lockErr)

		err := svc.OrderLists(ctx, "user-1", []string{"list-a"})

		assert.ErrorIs(t, err, lockErr)
		assert.NotErrorIs(t, err, ErrStaleListIDs)
		assertCallOrder(t, repo, "LockUsersLists")
	})

	t.Run("update error is propagated", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		updateErr := errors.New("update failed")
		repo.On("LockUsersLists", inTx, "user-1").Return([]string{"list-a"}, nil)
		repo.On("OrderLists", inTx, "user-1", []string{"list-a"}).Return(updateErr)

		err := svc.OrderLists(ctx, "user-1", []string{"list-a"})

		assert.ErrorIs(t, err, updateErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, tx := newListService(t)

		tx.err = errors.New("could not begin transaction")

		err := svc.OrderLists(ctx, "user-1", []string{"list-a"})

		assert.ErrorIs(t, err, tx.err)
		assertCallOrder(t, repo)
	})
}

func TestListService_CreateListItem(t *testing.T) {
	svc, repo, _ := newListService(t)

	created := &ListItem{ID: "item-1", ListID: "list-1", Title: "Milk", CreatedAt: time.Now()}
	repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
	repo.On("CreateListItem", mock.Anything, "list-1", "Milk").Return(created, nil)

	item, err := svc.CreateListItem(context.Background(), "user-1", "list-1", "Milk")

	require.NoError(t, err)
	assert.Equal(t, created, item)
	assertCallOrder(t, repo, "IsUserInList", "CreateListItem")
}

func TestListService_DeleteListItem(t *testing.T) {
	svc, repo, _ := newListService(t)

	repo.On("IsUserInListByItemID", mock.Anything, "item-1", "user-1").Return(true, nil)
	repo.On("DeleteListItem", mock.Anything, "item-1").Return(nil)

	err := svc.DeleteListItem(context.Background(), "user-1", "item-1")

	require.NoError(t, err)
	assertCallOrder(t, repo, "IsUserInListByItemID", "DeleteListItem")
}

func TestListService_UpdateListItem(t *testing.T) {
	svc, repo, _ := newListService(t)

	repo.On("IsUserInListByItemID", mock.Anything, "item-1", "user-1").Return(true, nil)
	repo.On("UpdateListItem", mock.Anything, "item-1", "Oat milk", true).Return(nil)

	err := svc.UpdateListItem(context.Background(), "user-1", "item-1", "Oat milk", true)

	require.NoError(t, err)
	assertCallOrder(t, repo, "IsUserInListByItemID", "UpdateListItem")
}

func TestListService_SetListItemTitle(t *testing.T) {
	svc, repo, _ := newListService(t)

	repo.On("IsUserInListByItemID", mock.Anything, "item-1", "user-1").Return(true, nil)
	repo.On("SetListItemTitle", mock.Anything, "item-1", "Oat milk").Return(nil)

	err := svc.SetListItemTitle(context.Background(), "user-1", "item-1", "Oat milk")

	require.NoError(t, err)
	assertCallOrder(t, repo, "IsUserInListByItemID", "SetListItemTitle")
}

func TestListService_SetListItemCompleted(t *testing.T) {
	tests := []struct {
		name      string
		completed bool
	}{
		{name: "marks the item as completed", completed: true},
		{name: "marks the item as open again", completed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newListService(t)

			repo.On("IsUserInListByItemID", mock.Anything, "item-1", "user-1").Return(true, nil)
			repo.On("SetListItemCompleted", mock.Anything, "item-1", tt.completed).Return(nil)

			err := svc.SetListItemCompleted(context.Background(), "user-1", "item-1", tt.completed)

			require.NoError(t, err)
			assertCallOrder(t, repo, "IsUserInListByItemID", "SetListItemCompleted")
		})
	}
}

func TestListService_GetListItemsForList(t *testing.T) {
	svc, repo, _ := newListService(t)

	want := []ListItem{
		{ID: "item-1", ListID: "list-1", Title: "Milk"},
		{ID: "item-2", ListID: "list-1", Title: "Bread", IsCompleted: true},
	}
	repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
	repo.On("GetListItemsForList", mock.Anything, "list-1").Return(want, nil)

	items, err := svc.GetListItemsForList(context.Background(), "user-1", "list-1")

	require.NoError(t, err)
	assert.Equal(t, want, items)
	assertCallOrder(t, repo, "IsUserInList", "GetListItemsForList")
}

type guardedOp struct {
	name string
	// byItem is true when membership is resolved through a list item id
	// (IsUserInListByItemID) instead of a list id (IsUserInList).
	byItem bool
	// expectRepo registers the delegated repository call, returning err.
	expectRepo func(repo *mockListRepository, err error)
	// call invokes the service method on behalf of userID.
	call func(svc *ListService, userID string) error
}

func (op guardedOp) guardMethod() string {
	if op.byItem {
		return "IsUserInListByItemID"
	}
	return "IsUserInList"
}

func (op guardedOp) expectGuard(repo *mockListRepository, userID string, inList bool, err error) {
	if op.byItem {
		repo.On("IsUserInListByItemID", mock.Anything, "item-1", userID).Return(inList, err)
		return
	}
	repo.On("IsUserInList", mock.Anything, "list-1", userID).Return(inList, err)
}

// guardedOps covers the operations whose only access check is group
// membership via the list (or the item's list). SetListGroup also checks the
// target group and is tested separately.
func guardedOps() []guardedOp {
	ctx := context.Background()

	return []guardedOp{
		{
			name: "SetListName",
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("SetListName", mock.Anything, "list-1", "name-1").Return(err)
			},
			call: func(svc *ListService, userID string) error { return svc.SetListName(ctx, userID, "list-1", "name-1") },
		},
		{
			name: "DeleteList",
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("DeleteList", mock.Anything, "list-1").Return(err)
			},
			call: func(svc *ListService, userID string) error {
				return svc.DeleteList(ctx, userID, "list-1")
			},
		},
		{
			name: "CreateListItem",
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("CreateListItem", mock.Anything, "list-1", "Milk").Return(nil, err)
			},
			call: func(svc *ListService, userID string) error {
				item, err := svc.CreateListItem(ctx, userID, "list-1", "Milk")
				if item != nil {
					return errors.New("expected no item on failure")
				}
				return err
			},
		},
		{
			name: "GetListItemsForList",
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("GetListItemsForList", mock.Anything, "list-1").Return(nil, err)
			},
			call: func(svc *ListService, userID string) error {
				items, err := svc.GetListItemsForList(ctx, userID, "list-1")
				if items != nil {
					return errors.New("expected no items on failure")
				}
				return err
			},
		},
		{
			name:   "DeleteListItem",
			byItem: true,
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("DeleteListItem", mock.Anything, "item-1").Return(err)
			},
			call: func(svc *ListService, userID string) error {
				return svc.DeleteListItem(ctx, userID, "item-1")
			},
		},
		{
			name:   "UpdateListItem",
			byItem: true,
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("UpdateListItem", mock.Anything, "item-1", "Oat milk", true).Return(err)
			},
			call: func(svc *ListService, userID string) error {
				return svc.UpdateListItem(ctx, userID, "item-1", "Oat milk", true)
			},
		},
		{
			name:   "SetListItemTitle",
			byItem: true,
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("SetListItemTitle", mock.Anything, "item-1", "Oat milk").Return(err)
			},
			call: func(svc *ListService, userID string) error {
				return svc.SetListItemTitle(ctx, userID, "item-1", "Oat milk")
			},
		},
		{
			name:   "SetListItemCompleted",
			byItem: true,
			expectRepo: func(repo *mockListRepository, err error) {
				repo.On("SetListItemCompleted", mock.Anything, "item-1", true).Return(err)
			},
			call: func(svc *ListService, userID string) error {
				return svc.SetListItemCompleted(ctx, userID, "item-1", true)
			},
		},
	}
}

func TestListService_AccessControl(t *testing.T) {
	for _, op := range guardedOps() {
		t.Run(op.name, func(t *testing.T) {
			t.Run("non-member is refused before any write", func(t *testing.T) {
				svc, repo, _ := newListService(t)

				op.expectGuard(repo, "intruder", false, nil)

				err := op.call(svc, "intruder")

				assert.ErrorIs(t, err, ErrUserNotInList)
				assertCallOrder(t, repo, op.guardMethod())
			})

			t.Run("membership check error is propagated, not masked", func(t *testing.T) {
				svc, repo, _ := newListService(t)

				repoErr := errors.New("connection reset")
				op.expectGuard(repo, "user-1", false, repoErr)

				err := op.call(svc, "user-1")

				assert.ErrorIs(t, err, repoErr)
				assert.NotErrorIs(t, err, ErrUserNotInList)
				assertCallOrder(t, repo, op.guardMethod())
			})

			t.Run("repository error is propagated", func(t *testing.T) {
				svc, repo, _ := newListService(t)

				repoErr := errors.New("write failed")
				op.expectGuard(repo, "user-1", true, nil)
				op.expectRepo(repo, repoErr)

				err := op.call(svc, "user-1")

				assert.ErrorIs(t, err, repoErr)
				assertCallOrder(t, repo, op.guardMethod(), op.name)
			})
		})
	}
}

func TestListService_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		call    func(svc *ListService) error
		wantErr error
	}{
		{
			name:    "SetListName without auth user id",
			call:    func(svc *ListService) error { return svc.SetListName(ctx, "", "list-1", "name-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetListName without list id",
			call:    func(svc *ListService) error { return svc.SetListName(ctx, "user-1", "", "name-1") },
			wantErr: ErrListIDMissing,
		},
		{
			name:    "SetListName without name",
			call:    func(svc *ListService) error { return svc.SetListName(ctx, "user-1", "list-1", "") },
			wantErr: ErrListNameMissing,
		},
		{
			name: "CreateList without auth user id",
			call: func(svc *ListService) error {
				_, err := svc.CreateList(ctx, "", "group-1", "name-1")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "CreateList without name",
			call: func(svc *ListService) error {
				_, err := svc.CreateList(ctx, "user-1", "group-1", "")
				return err
			},
			wantErr: ErrListNameMissing,
		},
		{
			name: "CreateList without name and without group id",
			call: func(svc *ListService) error {
				_, err := svc.CreateList(ctx, "user-1", "", "")
				return err
			},
			wantErr: ErrListNameMissing,
		},
		{
			name:    "SetListGroup without auth user id",
			call:    func(svc *ListService) error { return svc.SetListGroup(ctx, "", "list-1", "group-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetListGroup without list id",
			call:    func(svc *ListService) error { return svc.SetListGroup(ctx, "user-1", "", "group-1") },
			wantErr: ErrListIDMissing,
		},
		{
			name:    "SetListGroup without group id",
			call:    func(svc *ListService) error { return svc.SetListGroup(ctx, "user-1", "list-1", "") },
			wantErr: ErrGroupIDMissing,
		},
		{
			name:    "DeleteList without auth user id",
			call:    func(svc *ListService) error { return svc.DeleteList(ctx, "", "list-1") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "DeleteList without list id",
			call:    func(svc *ListService) error { return svc.DeleteList(ctx, "user-1", "") },
			wantErr: ErrListIDMissing,
		},
		{
			name: "GetLists without auth user id",
			call: func(svc *ListService) error {
				_, err := svc.GetLists(ctx, "")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "OrderLists without auth user id",
			call:    func(svc *ListService) error { return svc.OrderLists(ctx, "", []string{"list-1"}) },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "OrderLists with nil list ids",
			call:    func(svc *ListService) error { return svc.OrderLists(ctx, "user-1", nil) },
			wantErr: ErrListIDMissing,
		},
		{
			name:    "OrderLists with empty list ids",
			call:    func(svc *ListService) error { return svc.OrderLists(ctx, "user-1", []string{}) },
			wantErr: ErrListIDMissing,
		},
		{
			name: "CreateListItem without auth user id",
			call: func(svc *ListService) error {
				_, err := svc.CreateListItem(ctx, "", "list-1", "Milk")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "CreateListItem without list id",
			call: func(svc *ListService) error {
				_, err := svc.CreateListItem(ctx, "user-1", "", "Milk")
				return err
			},
			wantErr: ErrListIDMissing,
		},
		{
			name: "CreateListItem without title",
			call: func(svc *ListService) error {
				_, err := svc.CreateListItem(ctx, "user-1", "list-1", "")
				return err
			},
			wantErr: ErrListItemTitleMissing,
		},
		{
			name:    "DeleteListItem without auth user id",
			call:    func(svc *ListService) error { return svc.DeleteListItem(ctx, "", "Milk") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "DeleteListItem without item id",
			call:    func(svc *ListService) error { return svc.DeleteListItem(ctx, "user-1", "") },
			wantErr: ErrListItemIDMissing,
		},
		{
			name:    "UpdateListItem without auth user id",
			call:    func(svc *ListService) error { return svc.UpdateListItem(ctx, "", "item-1", "Milk", false) },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "UpdateListItem without item id",
			call:    func(svc *ListService) error { return svc.UpdateListItem(ctx, "user-1", "", "Milk", false) },
			wantErr: ErrListItemIDMissing,
		},
		{
			name:    "UpdateListItem without title",
			call:    func(svc *ListService) error { return svc.UpdateListItem(ctx, "user-1", "item-1", "", false) },
			wantErr: ErrListItemTitleMissing,
		},
		{
			name:    "SetListItemTitle without auth user id",
			call:    func(svc *ListService) error { return svc.SetListItemTitle(ctx, "", "item-1", "Milk") },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetListItemTitle without item id",
			call:    func(svc *ListService) error { return svc.SetListItemTitle(ctx, "user-1", "", "Milk") },
			wantErr: ErrListItemIDMissing,
		},
		{
			name:    "SetListItemTitle without title",
			call:    func(svc *ListService) error { return svc.SetListItemTitle(ctx, "user-1", "item-1", "") },
			wantErr: ErrListItemTitleMissing,
		},
		{
			name:    "SetListItemCompleted without auth user id",
			call:    func(svc *ListService) error { return svc.SetListItemCompleted(ctx, "", "item-1", true) },
			wantErr: ErrUserIDMissing,
		},
		{
			name:    "SetListItemCompleted without item id",
			call:    func(svc *ListService) error { return svc.SetListItemCompleted(ctx, "user-1", "", true) },
			wantErr: ErrListItemIDMissing,
		},
		{
			name: "GetListItemsForList without auth user id",
			call: func(svc *ListService) error {
				_, err := svc.GetListItemsForList(ctx, "", "list-1")
				return err
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "GetListItemsForList without list id",
			call: func(svc *ListService) error {
				_, err := svc.GetListItemsForList(ctx, "user-1", "")
				return err
			},
			wantErr: ErrListIDMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, groups, tx := newListServiceWithGroups(t)

			err := tt.call(svc)

			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, tx.calls, "no transaction may be started")
			assertCallOrder(t, repo)
			assertGroupCallOrder(t, groups)
		})
	}
}
