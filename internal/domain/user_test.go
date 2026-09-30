package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var _ UserRepository = (*mockUserRepository)(nil)

type mockUserRepository struct {
	mock.Mock
}

func (m *mockUserRepository) CreateUser(ctx context.Context, email string, name string, passwordHash string) (*User, error) {
	args := m.Called(ctx, email, name, passwordHash)
	user, _ := args.Get(0).(*User)
	return user, args.Error(1)
}

func (m *mockUserRepository) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepository) ChangePassword(ctx context.Context, userID string, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	args := m.Called(ctx, email)
	user, _ := args.Get(0).(*User)
	return user, args.Error(1)
}

func newUserService(t *testing.T) (*UserService, *mockUserRepository) {
	t.Helper()

	repo := &mockUserRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	return NewUserService(repo), repo
}

// assertBcryptHashOf checks that hash is a bcrypt hash of password at the
// default cost. It runs once per test on purpose: a mock.MatchedBy matcher is
// evaluated several times per call and bcrypt is slow, especially with -race.
func assertBcryptHashOf(t *testing.T, hash string, password string) {
	t.Helper()

	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err, "not a bcrypt hash: %q", hash)
	assert.Equal(t, bcrypt.DefaultCost, cost)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)))
	assert.NotContains(t, hash, password)
}

func TestUserService_CreateUser(t *testing.T) {
	t.Parallel() // bcrypt at DefaultCost is slow, especially with -race

	ctx := context.Background()

	t.Run("stores a bcrypt hash instead of the password", func(t *testing.T) {
		t.Parallel()

		svc, repo := newUserService(t)

		created := &User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin", CreatedAt: time.Now()}
		repo.On("CreateUser", mock.Anything, "robin@dittmar.dev", "Robin", mock.AnythingOfType("string")).
			Return(created, nil)

		user, err := svc.CreateUser(ctx, "robin@dittmar.dev", "Robin", "hunter22")

		require.NoError(t, err)
		assert.Equal(t, created, user)
		assertBcryptHashOf(t, repo.Calls[0].Arguments.String(3), "hunter22")
	})

	t.Run("stores the email in lowercase", func(t *testing.T) {
		t.Parallel()

		svc, repo := newUserService(t)

		created := &User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}
		repo.On("CreateUser", mock.Anything, "robin@dittmar.dev", "Robin", mock.AnythingOfType("string")).
			Return(created, nil)

		user, err := svc.CreateUser(ctx, "Robin@Dittmar.DEV", "Robin", "hunter22")

		require.NoError(t, err)
		assert.Equal(t, created, user)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		t.Parallel()

		svc, repo := newUserService(t)

		repoErr := errors.New("duplicate key value violates unique constraint")
		repo.On("CreateUser", mock.Anything, "robin@dittmar.dev", "Robin", mock.AnythingOfType("string")).
			Return(nil, repoErr)

		user, err := svc.CreateUser(ctx, "robin@dittmar.dev", "Robin", "hunter22")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("password longer than 72 bytes is rejected before the repository", func(t *testing.T) {
		t.Parallel()

		svc, _ := newUserService(t)

		user, err := svc.CreateUser(ctx, "robin@dittmar.dev", "Robin", strings.Repeat("a", 73))

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrPasswordTooLong)
	})

	validation := []struct {
		name     string
		email    string
		userName string
		password string
		wantErr  error
	}{
		{name: "without email", email: "", userName: "Robin", password: "hunter22", wantErr: ErrEmailMissing},
		{name: "without name", email: "robin@dittmar.dev", userName: "", password: "hunter22", wantErr: ErrNameMissing},
		{name: "without password", email: "robin@dittmar.dev", userName: "Robin", password: "", wantErr: ErrPasswordMissing},
		{name: "with nothing reports email first", email: "", userName: "", password: "", wantErr: ErrEmailMissing},
	}
	for _, tt := range validation {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newUserService(t)

			user, err := svc.CreateUser(ctx, tt.email, tt.userName, tt.password)

			assert.Nil(t, user)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the user", func(t *testing.T) {
		svc, repo := newUserService(t)

		repo.On("DeleteUser", mock.Anything, "user-1").Return(nil)

		assert.NoError(t, svc.DeleteUser(ctx, "user-1"))
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newUserService(t)

		repoErr := errors.New("connection reset")
		repo.On("DeleteUser", mock.Anything, "user-1").Return(repoErr)

		assert.ErrorIs(t, svc.DeleteUser(ctx, "user-1"), repoErr)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newUserService(t)

		assert.ErrorIs(t, svc.DeleteUser(ctx, ""), ErrUserIDMissing)
	})
}

func TestUserService_ChangePassword(t *testing.T) {
	t.Parallel() // bcrypt at DefaultCost is slow, especially with -race

	ctx := context.Background()

	t.Run("stores a bcrypt hash instead of the password", func(t *testing.T) {
		t.Parallel()

		svc, repo := newUserService(t)

		repo.On("ChangePassword", mock.Anything, "user-1", mock.AnythingOfType("string")).Return(nil)

		require.NoError(t, svc.ChangePassword(ctx, "user-1", "correct horse"))

		assertBcryptHashOf(t, repo.Calls[0].Arguments.String(2), "correct horse")
	})

	t.Run("repository error is returned", func(t *testing.T) {
		t.Parallel()

		svc, repo := newUserService(t)

		repoErr := errors.New("connection reset")
		repo.On("ChangePassword", mock.Anything, "user-1", mock.AnythingOfType("string")).Return(repoErr)

		assert.ErrorIs(t, svc.ChangePassword(ctx, "user-1", "correct horse"), repoErr)
	})

	t.Run("password longer than 72 bytes is rejected before the repository", func(t *testing.T) {
		t.Parallel()

		svc, _ := newUserService(t)

		err := svc.ChangePassword(ctx, "user-1", strings.Repeat("a", 73))

		assert.ErrorIs(t, err, ErrPasswordTooLong)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newUserService(t)

		assert.ErrorIs(t, svc.ChangePassword(ctx, "", "correct horse"), ErrUserIDMissing)
	})

	t.Run("without password", func(t *testing.T) {
		svc, _ := newUserService(t)

		assert.ErrorIs(t, svc.ChangePassword(ctx, "user-1", ""), ErrPasswordMissing)
	})
}

func TestUserService_GetUserByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user from the repository", func(t *testing.T) {
		svc, repo := newUserService(t)

		found := &User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(found, nil)

		user, err := svc.GetUserByEmail(ctx, "robin@dittmar.dev")

		require.NoError(t, err)
		assert.Equal(t, found, user)
	})

	t.Run("looks the email up in lowercase", func(t *testing.T) {
		svc, repo := newUserService(t)

		found := &User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(found, nil)

		user, err := svc.GetUserByEmail(ctx, "Robin@Dittmar.DEV")

		require.NoError(t, err)
		assert.Equal(t, found, user)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newUserService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(nil, repoErr)

		user, err := svc.GetUserByEmail(ctx, "robin@dittmar.dev")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)
	})
}
