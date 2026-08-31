package domain

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserIDMissing   = errors.New("user id is required")
	ErrEmailMissing    = errors.New("email is required")
	ErrNameMissing     = errors.New("name is required")
	ErrPasswordMissing = errors.New("password is required")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRepository interface {
	CreateUser(ctx context.Context, email string, name string, passwordHash string) (*User, error)
	ChangePassword(ctx context.Context, userID string, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

type UserService struct {
	repo UserRepository
}

func NewUserService(r UserRepository) *UserService {
	return &UserService{repo: r}
}

func (s *UserService) CreateUser(ctx context.Context, email string, name string, password string) (*User, error) {
	if len(email) == 0 {
		return nil, ErrEmailMissing
	}
	if len(name) == 0 {
		return nil, ErrNameMissing
	}
	if len(password) == 0 {
		return nil, ErrPasswordMissing
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateUser(ctx, email, name, string(hash))
}

func (s *UserService) ChangePassword(ctx context.Context, userID string, password string) error {
	if len(userID) == 0 {
		return ErrUserIDMissing
	}
	if len(password) == 0 {
		return ErrPasswordMissing
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.ChangePassword(ctx, userID, string(hash))
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}
