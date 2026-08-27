package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthRepository interface {
	GetUserById(ctx context.Context, id string) (*AuthUser, error)
	GetUserByEmail(ctx context.Context, email string) (*AuthUser, error)
	StoreRefreshToken(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error
	ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeRefreshTokens(ctx context.Context, userID string) error
}

type AuthService struct {
	repo      AuthRepository
	jwtSecret []byte
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthUser struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
}

type AuthContext struct {
	UserID string
	Email  string
	Name   string
}

func GetAuthContext(ctx context.Context) (*AuthContext, error) {
	v := ctx.Value(AuthContextKey)
	if v == nil {
		return nil, errors.New("no auth context")
	}
	ac, ok := v.(*AuthContext)
	if !ok {
		return nil, errors.New("invalid auth context")
	}
	return ac, nil
}

func NewAuthService(r AuthRepository, jwtSecret []byte) *AuthService {
	return &AuthService{repo: r, jwtSecret: jwtSecret}
}

func (s *AuthService) authenticate(ctx context.Context, email string, password string) (*AuthUser, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return user, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return user, errors.New("invalid email or password")
		}
		return user, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (TokenPair, error) {
	user, err := s.authenticate(ctx, email, password)
	if err != nil {
		return TokenPair{}, err
	}

	return s.issueTokens(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	tokenHash := hashToken(refreshToken)

	userID, err := s.repo.ConsumeRefreshToken(ctx, tokenHash)
	if err != nil {
		return TokenPair{}, err
	}

	authUser, err := s.repo.GetUserById(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}

	return s.issueTokens(ctx, authUser)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)

	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *AuthService) LogoutAllDevices(ctx context.Context, userID string) error {
	return s.repo.RevokeRefreshTokens(ctx, userID)
}

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

type contextKey string

const AuthContextKey = contextKey("auth")

func (s *AuthService) issueTokens(ctx context.Context, authUser *AuthUser) (TokenPair, error) {
	accessToken, err := s.GenerateAccessToken(authUser)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to issue access token: %s", err)
	}

	refreshToken, err := s.GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to issue refresh token: %s", err)
	}

	err = s.repo.StoreRefreshToken(ctx, authUser.ID, hashToken(refreshToken), time.Now().Add(time.Hour*24*7))
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to store refresh token: %s", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) GenerateAccessToken(authUser *AuthUser) (string, error) {
	claims := JWTClaims{
		UserID:    authUser.ID,
		Email:     authUser.Email,
		Name:      authUser.Name,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 15)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *AuthService) GenerateRefreshToken() (string, error) {
	rawToken, err := generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return rawToken, nil
}

func (s *AuthService) ParseAccessToken(ctx context.Context, tokenString string) (*AuthContext, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		slog.ErrorContext(ctx, "invalid or expired token", slog.Any("token", token))
		return nil, err
	}

	return &AuthContext{
		UserID: claims.UserID,
		Email:  claims.Email,
		Name:   claims.Name,
	}, nil
}

func generateSecureToken(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
