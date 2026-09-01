package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInviteIDMissing = errors.New("invite id is required")
	ErrCodeMissing     = errors.New("invite code is required")
	ErrInviteInvalid   = errors.New("invite is invalid")
	ErrInviteExpired   = errors.New("invite is expired")
	ErrInviteConsumed  = errors.New("invite is already consumed")
)

type Invite struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at"`
}

type InviteCounts struct {
	Active  int `json:"active"`
	Expired int `json:"expired"`
	Used    int `json:"used"`
}

type InviteRepository interface {
	CreateInvite(ctx context.Context, inviterUserID string, code string, expiresAt time.Time) (*Invite, error)
	DeleteInvite(ctx context.Context, userID string, inviteID string) error
	ConsumeInvite(ctx context.Context, inviteID string, inviteeUserID string) error
	GetInvite(ctx context.Context, code string) (*Invite, error)
	GetInvites(ctx context.Context, userID string, offset int, count int) ([]Invite, error)
	CountInvites(ctx context.Context, userID string) (int, error)
	CountInvitesStructured(ctx context.Context, userID string) (*InviteCounts, error)
}

type InviteService struct {
	repo InviteRepository
}

func NewInviteService(r InviteRepository) *InviteService {
	return &InviteService{repo: r}
}

func (s *InviteService) CreateInvite(ctx context.Context, inviterUserID string) (*Invite, error) {
	if inviterUserID == "" {
		return nil, ErrUserIDMissing
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}
	code := hashToken(token)
	expiresAt := time.Now().Add(time.Hour * 24 * 7)

	return s.repo.CreateInvite(ctx, inviterUserID, code, expiresAt)
}

func (s *InviteService) DeleteInvite(ctx context.Context, userID string, inviteID string) error {
	if userID == "" {
		return ErrUserIDMissing
	}
	if inviteID == "" {
		return ErrInviteIDMissing
	}

	return s.repo.DeleteInvite(ctx, userID, inviteID)
}

func (s *InviteService) ConsumeInvite(ctx context.Context, inviteID string, inviteeUserID string) error {
	if inviteeUserID == "" {
		return ErrUserIDMissing
	}

	return s.repo.ConsumeInvite(ctx, inviteID, inviteeUserID)
}

func (s *InviteService) GetInvite(ctx context.Context, code string) (*Invite, error) {
	if code == "" {
		return nil, ErrCodeMissing
	}

	invite, err := s.repo.GetInvite(ctx, code)
	if err != nil {
		return nil, err
	}
	if invite.ExpiresAt.Before(time.Now()) {
		return nil, ErrInviteExpired
	}
	if invite.ConsumedAt != nil {
		return nil, ErrInviteConsumed
	}

	return invite, nil
}

func (s *InviteService) GetInvites(ctx context.Context, userID string, page int, countPerPage int) ([]Invite, error) {
	if userID == "" {
		return nil, ErrUserIDMissing
	}

	offset := (page - 1) * countPerPage
	return s.repo.GetInvites(ctx, userID, offset, countPerPage)
}

func (s *InviteService) CountInvites(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, ErrUserIDMissing
	}

	return s.repo.CountInvites(ctx, userID)
}

func (s *InviteService) CountInvitesStructured(ctx context.Context, userID string) (*InviteCounts, error) {
	if userID == "" {
		return nil, ErrUserIDMissing
	}

	return s.repo.CountInvitesStructured(ctx, userID)
}
