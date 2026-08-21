package domain

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRepository interface {
	CreateUser(ctx context.Context, email string, name string, passwordHash string) (*User, error)
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
		return nil, errors.New("email is required")
	}
	if len(name) == 0 {
		return nil, errors.New("name is required")
	}
	if len(password) == 0 {
		return nil, errors.New("password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateUser(ctx, email, name, string(hash))
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}
