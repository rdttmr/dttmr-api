package domain

import (
	"context"
	"log/slog"
	"time"
)

type RegistrationService struct {
	tx            Transactor
	UserService   *UserService
	InviteService *InviteService
}

func NewRegistrationService(tx Transactor, u *UserService, i *InviteService) *RegistrationService {
	return &RegistrationService{tx: tx, UserService: u, InviteService: i}
}

func (s *RegistrationService) Register(ctx context.Context, inviteCode string, email string, username string, password string) (*User, error) {
	invite, err := s.InviteService.GetInvite(ctx, inviteCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get invite", slog.Any("error", err))
		return nil, err
	}

	if invite.ConsumedAt != nil {
		return nil, ErrInviteConsumed
	}
	if invite.ExpiresAt.Before(time.Now()) {
		return nil, ErrInviteExpired
	}

	var user *User
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		user, err = s.UserService.CreateUser(ctx, email, username, password)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create user", slog.Any("error", err))
			return err
		}

		err = s.InviteService.ConsumeInvite(ctx, invite.ID, user.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to consume invite", slog.Any("error", err))
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "user registration complete",
		slog.String("user_id", user.ID),
		slog.String("invite_id", invite.ID))
	return user, nil
}
