package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/robindittmar/dttmr-api/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockListRepo struct {
	mock.Mock
}

func (m *mockListRepo) CreateList(ctx context.Context, name string, userIDs []string) (*domain.List, error) {
	args := m.Called(ctx, name, userIDs)
	var list *domain.List
	if l := args.Get(0); l != nil {
		list = l.(*domain.List)
	}
	return list, args.Error(1)
}

func TestListService_Create_Success(t *testing.T) {
	expectedList := &domain.List{
		ID:         "1",
		Name:       "My List",
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	repo := new(mockListRepo)
	repo.On("CreateList", mock.Anything, "My List", []string{"user1", "user2"}).Return(expectedList, nil)

	service := domain.NewListService(repo)
	list, err := service.Create(context.Background(), "My List", []string{"user1", "user2"})

	require.NoError(t, err)
	assert.Equal(t, expectedList, list)
	repo.AssertExpectations(t)
}

func TestListService_Create_EmptyName(t *testing.T) {
	repo := new(mockListRepo)
	service := domain.NewListService(repo)

	list, err := service.Create(context.Background(), "", []string{"user1"})

	require.Error(t, err)
	assert.EqualError(t, err, "list name must not be empty")
	assert.Nil(t, list)
	repo.AssertExpectations(t)
}

func TestListService_Create_EmptyUsers(t *testing.T) {
	repo := new(mockListRepo)
	service := domain.NewListService(repo)

	list, err := service.Create(context.Background(), "My List", []string{})

	require.Error(t, err)
	assert.EqualError(t, err, "users must have at least one associated user")
	assert.Nil(t, list)
	repo.AssertExpectations(t)
}

func TestListService_Create_RepoError(t *testing.T) {
	expectedErr := errors.New("database error")
	repo := new(mockListRepo)
	repo.On("CreateList", mock.Anything, "My List", []string{"user1"}).Return(nil, expectedErr)

	service := domain.NewListService(repo)

	list, err := service.Create(context.Background(), "My List", []string{"user1"})

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, list)
	repo.AssertExpectations(t)
}
