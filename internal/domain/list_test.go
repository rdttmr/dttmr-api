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

type fakeTransactor struct {
	calls int
	err   error
}

func (f *fakeTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return fn(ctx)
}

type mockListRepository struct {
	mock.Mock
}

func (m *mockListRepository) CreateList(ctx context.Context, name string) (*List, error) {
	args := m.Called(ctx, name)
	list, _ := args.Get(0).(*List)
	return list, args.Error(1)
}

func (m *mockListRepository) DeleteList(ctx context.Context, listID string) error {
	args := m.Called(ctx, listID)
	return args.Error(0)
}

func (m *mockListRepository) GetLists(ctx context.Context, userID string) ([]List, error) {
	args := m.Called(ctx, userID)
	lists, _ := args.Get(0).([]List)
	return lists, args.Error(1)
}

func (m *mockListRepository) AddUserToList(ctx context.Context, listID string, userID string) error {
	args := m.Called(ctx, listID, userID)
	return args.Error(0)
}

func (m *mockListRepository) RemoveUserFromList(ctx context.Context, listID string, userID string) error {
	args := m.Called(ctx, listID, userID)
	return args.Error(0)
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

func (m *mockListRepository) GetListItems(ctx context.Context, listID string) ([]ListItem, error) {
	args := m.Called(ctx, listID)
	items, _ := args.Get(0).([]ListItem)
	return items, args.Error(1)
}

func newListService(t *testing.T) (*ListService, *mockListRepository, *fakeTransactor) {
	t.Helper()

	repo := &mockListRepository{}
	t.Cleanup(func() { repo.AssertExpectations(t) })

	tx := &fakeTransactor{}

	return NewListService(tx, repo), repo, tx
}

func TestListService_CreateList(t *testing.T) {
	t.Run("creates the list and adds the owner", func(t *testing.T) {
		svc, repo, tx := newListService(t)

		created := &List{ID: "list-1", Name: "Groceries", CreatedAt: time.Now()}
		repo.On("CreateList", mock.Anything, "Groceries").Return(created, nil)
		repo.On("AddUserToList", mock.Anything, "list-1", "user-1").Return(nil)

		list, err := svc.CreateList(context.Background(), "user-1", "Groceries")

		require.NoError(t, err)
		assert.Equal(t, created, list)
		assert.Equal(t, 1, tx.calls)
	})

	t.Run("empty name is rejected before any repo call", func(t *testing.T) {
		svc, _, tx := newListService(t)

		list, err := svc.CreateList(context.Background(), "user-1", "")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, ErrListNameMissing)
		assert.Zero(t, tx.calls)
	})

	t.Run("insert error aborts before adding the user", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repoErr := errors.New("insert failed")
		repo.On("CreateList", mock.Anything, "Groceries").Return(nil, repoErr)

		list, err := svc.CreateList(context.Background(), "user-1", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, repoErr)
		repo.AssertNotCalled(t, "AddUserToList", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("transaction error is returned", func(t *testing.T) {
		svc, _, tx := newListService(t)

		tx.err = errors.New("could not begin transaction")

		list, err := svc.CreateList(context.Background(), "user-1", "Groceries")

		assert.Nil(t, list)
		assert.ErrorIs(t, err, tx.err)
	})
}

func TestListService_DeleteList(t *testing.T) {
	t.Run("deletes when the user is a member", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		repo.On("DeleteList", mock.Anything, "list-1").Return(nil)

		assert.NoError(t, svc.DeleteList(context.Background(), "user-1", "list-1"))
	})

	t.Run("refuses when the user is not a member", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "intruder").Return(false, nil)

		err := svc.DeleteList(context.Background(), "intruder", "list-1")

		assert.ErrorIs(t, err, ErrUserNotInList)
		repo.AssertNotCalled(t, "DeleteList", mock.Anything, mock.Anything)
	})

	t.Run("membership check error is propagated", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repoErr := errors.New("connection reset")
		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(false, repoErr)

		err := svc.DeleteList(context.Background(), "user-1", "list-1")

		assert.ErrorIs(t, err, repoErr)
		assert.NotErrorIs(t, err, ErrUserNotInList)
		repo.AssertNotCalled(t, "DeleteList", mock.Anything, mock.Anything)
	})
}

func TestListService_GetLists(t *testing.T) {
	svc, repo, _ := newListService(t)

	want := []List{{ID: "list-1", Name: "Groceries"}, {ID: "list-2", Name: "Reading"}}
	repo.On("GetLists", mock.Anything, "user-1").Return(want, nil)

	lists, err := svc.GetLists(context.Background(), "user-1")

	require.NoError(t, err)
	assert.Equal(t, want, lists)
}

func TestListService_CreateListItem(t *testing.T) {
	t.Run("creates the item for a member", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		created := &ListItem{ID: "item-1", ListID: "list-1", Title: "Milk"}
		repo.On("IsUserInList", mock.Anything, "list-1", "user-1").Return(true, nil)
		repo.On("CreateListItem", mock.Anything, "list-1", "Milk").Return(created, nil)

		item, err := svc.CreateListItem(context.Background(), "user-1", "list-1", "Milk")

		require.NoError(t, err)
		assert.Equal(t, created, item)
	})

	t.Run("refuses for a non-member", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("IsUserInList", mock.Anything, "list-1", "intruder").Return(false, nil)

		item, err := svc.CreateListItem(context.Background(), "intruder", "list-1", "Milk")

		assert.Nil(t, item)
		assert.ErrorIs(t, err, ErrUserNotInList)
		repo.AssertNotCalled(t, "CreateListItem", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestListService_SetListItemCompleted(t *testing.T) {
	t.Run("checks membership through the item id", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("IsUserInListByItemID", mock.Anything, "item-1", "user-1").Return(true, nil)
		repo.On("SetListItemCompleted", mock.Anything, "item-1", true).Return(nil)

		assert.NoError(t, svc.SetListItemCompleted(context.Background(), "user-1", "item-1", true))
	})

	t.Run("refuses for a non-member", func(t *testing.T) {
		svc, repo, _ := newListService(t)

		repo.On("IsUserInListByItemID", mock.Anything, "item-1", "intruder").Return(false, nil)

		err := svc.SetListItemCompleted(context.Background(), "intruder", "item-1", false)

		assert.ErrorIs(t, err, ErrUserNotInList)
		repo.AssertNotCalled(t, "SetListItemCompleted", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestListService_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		call    func(svc *ListService) error
		wantErr error
	}{
		{
			name:    "DeleteList without list id",
			call:    func(svc *ListService) error { return svc.DeleteList(context.Background(), "user-1", "") },
			wantErr: ErrListIDMissing,
		},
		{
			name: "AddUserToList without list id",
			call: func(svc *ListService) error {
				return svc.AddUserToList(context.Background(), "user-1", "", "user-2")
			},
			wantErr: ErrListIDMissing,
		},
		{
			name: "AddUserToList without user id",
			call: func(svc *ListService) error {
				return svc.AddUserToList(context.Background(), "user-1", "list-1", "")
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "RemoveUserFromList without user id",
			call: func(svc *ListService) error {
				return svc.RemoveUserFromList(context.Background(), "user-1", "list-1", "")
			},
			wantErr: ErrUserIDMissing,
		},
		{
			name: "CreateListItem without title",
			call: func(svc *ListService) error {
				_, err := svc.CreateListItem(context.Background(), "user-1", "list-1", "")
				return err
			},
			wantErr: ErrListItemTitleMissing,
		},
		{
			name: "DeleteListItem without item id",
			call: func(svc *ListService) error {
				return svc.DeleteListItem(context.Background(), "user-1", "")
			},
			wantErr: ErrListItemIDMissing,
		},
		{
			name: "UpdateListItem without title",
			call: func(svc *ListService) error {
				return svc.UpdateListItem(context.Background(), "user-1", "item-1", "", false)
			},
			wantErr: ErrListItemTitleMissing,
		},
		{
			name: "SetListItemTitle without item id",
			call: func(svc *ListService) error {
				return svc.SetListItemTitle(context.Background(), "user-1", "", "Milk")
			},
			wantErr: ErrListItemIDMissing,
		},
		{
			name: "SetListItemCompleted without item id",
			call: func(svc *ListService) error {
				return svc.SetListItemCompleted(context.Background(), "user-1", "", true)
			},
			wantErr: ErrListItemIDMissing,
		},
		{
			name: "GetListItems without list id",
			call: func(svc *ListService) error {
				_, err := svc.GetListItems(context.Background(), "user-1", "")
				return err
			},
			wantErr: ErrListIDMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newListService(t)

			assert.ErrorIs(t, tt.call(svc), tt.wantErr)
		})
	}
}
