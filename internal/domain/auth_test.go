package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var _ AuthRepository = (*mockAuthRepository)(nil)

type mockAuthRepository struct {
	mock.Mock
}

func (m *mockAuthRepository) GetUserById(ctx context.Context, id string) (*AuthUser, error) {
	args := m.Called(ctx, id)
	user, _ := args.Get(0).(*AuthUser)
	return user, args.Error(1)
}

func (m *mockAuthRepository) GetUserByEmail(ctx context.Context, email string) (*AuthUser, error) {
	args := m.Called(ctx, email)
	user, _ := args.Get(0).(*AuthUser)
	return user, args.Error(1)
}

func (m *mockAuthRepository) StoreRefreshToken(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, tokenHash, expiresAt)
	return args.Error(0)
}

func (m *mockAuthRepository) ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	args := m.Called(ctx, tokenHash)
	return args.String(0), args.Error(1)
}

func (m *mockAuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *mockAuthRepository) RevokeRefreshTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

var testJWTSecret = []byte("test-secret")

func newAuthService(t *testing.T) (*AuthService, *mockAuthRepository, *fakeTransactor) {
	t.Helper()

	repo := &mockAuthRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	tx := &fakeTransactor{}
	return NewAuthService(tx, repo, testJWTSecret), repo, tx
}

// authUser returns a user whose password is "hunter22". MinCost keeps the
// tests fast; Authenticate works with any cost stored in the hash.
func authUser(t *testing.T) *AuthUser {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("hunter22"), bcrypt.MinCost)
	require.NoError(t, err)

	return &AuthUser{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin", PasswordHash: string(hash)}
}

// signToken signs claims like GenerateAccessToken does, with the given method and secret.
func signToken(t *testing.T, method jwt.SigningMethod, secret any, claims JWTClaims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString(secret)
	require.NoError(t, err)
	return token
}

func assertRefreshToken(t *testing.T, token string) {
	t.Helper()

	assert.Len(t, token, 64)
	_, err := hex.DecodeString(token)
	assert.NoError(t, err, "refresh token is not hex: %q", token)
}

func TestAuthService_Authenticate(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user for the right password", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		user := authUser(t)
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(user, nil)

		got, err := svc.Authenticate(ctx, "robin@dittmar.dev", "hunter22")

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("looks the email up in lowercase", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(authUser(t), nil)

		_, err := svc.Authenticate(ctx, "Robin@Dittmar.DEV", "hunter22")

		require.NoError(t, err)
	})

	t.Run("wrong password and unknown email give the same error", func(t *testing.T) {
		t.Parallel() // the unknown email runs a bcrypt comparison at DefaultCost

		svc, repo, _ := newAuthService(t)

		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(authUser(t), nil)
		repo.On("GetUserByEmail", mock.Anything, "nobody@dittmar.dev").Return(nil, ErrEmailNotFound)

		wrongPasswordUser, wrongPasswordErr := svc.Authenticate(ctx, "robin@dittmar.dev", "wrong")
		unknownEmailUser, unknownEmailErr := svc.Authenticate(ctx, "nobody@dittmar.dev", "hunter22")

		assert.Nil(t, wrongPasswordUser)
		assert.Nil(t, unknownEmailUser)
		assert.ErrorIs(t, wrongPasswordErr, ErrEmailOrPasswordWrong)
		assert.ErrorIs(t, unknownEmailErr, ErrEmailOrPasswordWrong)
		assert.Equal(t, wrongPasswordErr, unknownEmailErr)
		assert.NotErrorIs(t, unknownEmailErr, ErrEmailNotFound)
	})

	// An unknown email must cost a real bcrypt comparison, otherwise response
	// times reveal which emails are registered. The dummy comparison runs at
	// production cost, so it has to take clearly longer than one at MinCost
	// (DefaultCost is 64 times the work of MinCost).
	t.Run("unknown email costs a real bcrypt comparison", func(t *testing.T) {
		t.Parallel()

		svc, repo, _ := newAuthService(t)
		repo.On("GetUserByEmail", mock.Anything, "nobody@dittmar.dev").Return(nil, ErrEmailNotFound)

		minCostHash, err := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.MinCost)
		require.NoError(t, err)
		start := time.Now()
		_ = bcrypt.CompareHashAndPassword(minCostHash, []byte("y"))
		minCostCompare := time.Since(start)

		start = time.Now()
		_, err = svc.Authenticate(ctx, "nobody@dittmar.dev", "hunter22")
		unknownEmail := time.Since(start)

		require.ErrorIs(t, err, ErrEmailOrPasswordWrong)
		assert.Greater(t, unknownEmail, 4*minCostCompare,
			"unknown email took %v, a MinCost bcrypt comparison %v", unknownEmail, minCostCompare)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(nil, repoErr)

		user, err := svc.Authenticate(ctx, "robin@dittmar.dev", "hunter22")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("corrupt stored hash is an error, not a wrong password", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").
			Return(&AuthUser{ID: "user-1", PasswordHash: "not-a-bcrypt-hash"}, nil)

		user, err := svc.Authenticate(ctx, "robin@dittmar.dev", "hunter22")

		assert.Nil(t, user)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, ErrEmailOrPasswordWrong)
	})
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()

	t.Run("issues an access token and stores the hash of a new refresh token", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(authUser(t), nil)
		repo.On("StoreRefreshToken", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		before := time.Now()
		tokens, err := svc.Login(ctx, "robin@dittmar.dev", "hunter22")

		require.NoError(t, err)
		authCtx, err := svc.ParseAccessToken(ctx, tokens.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, &AuthContext{UserID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}, authCtx)

		assertRefreshToken(t, tokens.RefreshToken)
		store := repo.Calls[len(repo.Calls)-1]
		assert.Equal(t, hashToken(tokens.RefreshToken), store.Arguments.String(2))
		assert.WithinDuration(t, before.Add(7*24*time.Hour), store.Arguments.Get(3).(time.Time), 5*time.Second)
	})

	t.Run("failed authentication stores no refresh token", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(authUser(t), nil)

		tokens, err := svc.Login(ctx, "robin@dittmar.dev", "wrong")

		assert.ErrorIs(t, err, ErrEmailOrPasswordWrong)
		assert.Zero(t, tokens)
		assert.Equal(t, []string{"GetUserByEmail"}, callOrder(repo.Calls))
	})

	t.Run("store error is returned and matchable", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		storeErr := errors.New("insert failed")
		repo.On("GetUserByEmail", mock.Anything, "robin@dittmar.dev").Return(authUser(t), nil)
		repo.On("StoreRefreshToken", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(storeErr)

		tokens, err := svc.Login(ctx, "robin@dittmar.dev", "hunter22")

		assert.Zero(t, tokens)
		assert.ErrorContains(t, err, "failed to store refresh token")
		assert.ErrorIs(t, err, storeErr)
	})
}

func TestAuthService_Refresh(t *testing.T) {
	ctx := context.Background()

	t.Run("rotates the refresh token inside one transaction", func(t *testing.T) {
		svc, repo, tx := newAuthService(t)

		repo.On("ConsumeRefreshToken", inTx, hashToken("old-refresh-token")).Return("user-1", nil)
		repo.On("GetUserById", inTx, "user-1").Return(authUser(t), nil)
		repo.On("StoreRefreshToken", inTx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		tokens, err := svc.Refresh(ctx, "old-refresh-token")

		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls)
		assert.Equal(t, []string{"ConsumeRefreshToken", "GetUserById", "StoreRefreshToken"}, callOrder(repo.Calls))

		assertRefreshToken(t, tokens.RefreshToken)
		assert.NotEqual(t, "old-refresh-token", tokens.RefreshToken)
		assert.Equal(t, hashToken(tokens.RefreshToken), repo.Calls[2].Arguments.String(2))

		authCtx, err := svc.ParseAccessToken(ctx, tokens.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "user-1", authCtx.UserID)
	})

	t.Run("unknown or expired token issues nothing", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		consumeErr := errors.New("refresh token not found or expired")
		repo.On("ConsumeRefreshToken", inTx, hashToken("old-refresh-token")).Return("", consumeErr)

		tokens, err := svc.Refresh(ctx, "old-refresh-token")

		assert.Zero(t, tokens)
		assert.ErrorIs(t, err, consumeErr)
		assert.Equal(t, []string{"ConsumeRefreshToken"}, callOrder(repo.Calls))
	})

	t.Run("deleted user issues nothing", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		userErr := errors.New("user not found")
		repo.On("ConsumeRefreshToken", inTx, hashToken("old-refresh-token")).Return("user-1", nil)
		repo.On("GetUserById", inTx, "user-1").Return(nil, userErr)

		tokens, err := svc.Refresh(ctx, "old-refresh-token")

		assert.Zero(t, tokens)
		assert.ErrorIs(t, err, userErr)
	})

	t.Run("store error fails the refresh", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("ConsumeRefreshToken", inTx, hashToken("old-refresh-token")).Return("user-1", nil)
		repo.On("GetUserById", inTx, "user-1").Return(authUser(t), nil)
		repo.On("StoreRefreshToken", inTx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(errors.New("insert failed"))

		tokens, err := svc.Refresh(ctx, "old-refresh-token")

		assert.Zero(t, tokens)
		assert.ErrorContains(t, err, "failed to store refresh token")
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, tx := newAuthService(t)

		tx.err = errors.New("could not begin transaction")

		tokens, err := svc.Refresh(ctx, "old-refresh-token")

		assert.Zero(t, tokens)
		assert.ErrorIs(t, err, tx.err)
		assert.Empty(t, repo.Calls)
	})
}

func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("revokes the token by its hash", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("RevokeRefreshToken", mock.Anything, hashToken("refresh-token")).Return(nil)

		assert.NoError(t, svc.Logout(ctx, "refresh-token"))
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repoErr := errors.New("connection reset")
		repo.On("RevokeRefreshToken", mock.Anything, hashToken("refresh-token")).Return(repoErr)

		assert.ErrorIs(t, svc.Logout(ctx, "refresh-token"), repoErr)
	})
}

func TestAuthService_LogoutAllDevices(t *testing.T) {
	ctx := context.Background()

	t.Run("revokes all tokens of the user", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repo.On("RevokeRefreshTokens", mock.Anything, "user-1").Return(nil)

		assert.NoError(t, svc.LogoutAllDevices(ctx, "user-1"))
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newAuthService(t)

		repoErr := errors.New("connection reset")
		repo.On("RevokeRefreshTokens", mock.Anything, "user-1").Return(repoErr)

		assert.ErrorIs(t, svc.LogoutAllDevices(ctx, "user-1"), repoErr)
	})
}

func TestAuthService_AccessToken(t *testing.T) {
	ctx := context.Background()
	claims := func(expiresAt time.Time) JWTClaims {
		return JWTClaims{
			UserID:           "user-1",
			Email:            "robin@dittmar.dev",
			Name:             "Robin",
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)},
		}
	}

	t.Run("round trip keeps the user and expires after 15 minutes", func(t *testing.T) {
		svc, _, _ := newAuthService(t)

		before := time.Now()
		token, err := svc.GenerateAccessToken(&AuthUser{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"})
		require.NoError(t, err)

		authCtx, err := svc.ParseAccessToken(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, &AuthContext{UserID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}, authCtx)

		parsed := &JWTClaims{}
		_, _, err = jwt.NewParser().ParseUnverified(token, parsed)
		require.NoError(t, err)
		assert.WithinDuration(t, before.Add(15*time.Minute), parsed.ExpiresAt.Time, 5*time.Second)
	})

	rejected := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{
			name: "expired token",
			token: func(t *testing.T) string {
				return signToken(t, jwt.SigningMethodHS256, testJWTSecret, claims(time.Now().Add(-time.Minute)))
			},
		},
		{
			name: "token signed with another secret",
			token: func(t *testing.T) string {
				return signToken(t, jwt.SigningMethodHS256, []byte("other"), claims(time.Now().Add(time.Hour)))
			},
		},
		{
			name: "unsigned token (alg none)",
			token: func(t *testing.T) string {
				return signToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, claims(time.Now().Add(time.Hour)))
			},
		},
		{
			name: "tampered claims",
			token: func(t *testing.T) string {
				valid := signToken(t, jwt.SigningMethodHS256, testJWTSecret, claims(time.Now().Add(time.Hour)))
				other := claims(time.Now().Add(time.Hour))
				other.UserID = "user-2"
				forged := signToken(t, jwt.SigningMethodHS256, []byte("other"), other)
				// Payload of the forged token, signature of the valid one.
				return forged[:len(forged)-43] + valid[len(valid)-43:]
			},
		},
		{
			name:  "garbage",
			token: func(t *testing.T) string { return "not.a.jwt" },
		},
		{
			name:  "empty",
			token: func(t *testing.T) string { return "" },
		},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			svc, _, _ := newAuthService(t)

			authCtx, err := svc.ParseAccessToken(ctx, tt.token(t))

			assert.Nil(t, authCtx)
			assert.Error(t, err)
		})
	}
}

func TestAuthService_GenerateRefreshToken(t *testing.T) {
	svc, _, _ := newAuthService(t)

	first, err := svc.GenerateRefreshToken()
	require.NoError(t, err)
	second, err := svc.GenerateRefreshToken()
	require.NoError(t, err)

	assertRefreshToken(t, first)
	assertRefreshToken(t, second)
	assert.NotEqual(t, first, second)
}

func TestGetAuthContext(t *testing.T) {
	t.Run("returns the stored auth context", func(t *testing.T) {
		want := &AuthContext{UserID: "user-1"}
		ctx := context.WithValue(context.Background(), AuthContextKey, want)

		got, err := GetAuthContext(ctx)

		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("missing auth context", func(t *testing.T) {
		got, err := GetAuthContext(context.Background())

		assert.Nil(t, got)
		assert.Error(t, err)
	})

	t.Run("value of the wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), AuthContextKey, AuthContext{UserID: "user-1"})

		got, err := GetAuthContext(ctx)

		assert.Nil(t, got)
		assert.Error(t, err)
	})
}

func TestHashToken(t *testing.T) {
	// SHA-256 test vector from FIPS 180-2.
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", hashToken("abc"))
}

func TestGenerateSecureToken(t *testing.T) {
	token, err := generateSecureToken(16)

	require.NoError(t, err)
	assert.Len(t, token, 32)
	_, err = hex.DecodeString(token)
	assert.NoError(t, err)
}

// The dummy hash must cost as much as a stored password hash, otherwise the
// unknown-email path is faster again and leaks which emails exist.
func TestDummyPasswordHash_MatchesPasswordCost(t *testing.T) {
	cost, err := bcrypt.Cost(dummyPasswordHash)

	require.NoError(t, err)
	assert.Equal(t, bcrypt.DefaultCost, cost)
}
