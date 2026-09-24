package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrGroupIDMissing = errors.New("group id is required")
	ErrUserNotInGroup = errors.New("user is not in group")
	ErrUserNotOwner   = errors.New("user is not owner")
)

type Group struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

type GroupInvite struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GroupRepository interface {
	CreateGroup(ctx context.Context, name string, createdBy string) (*Group, error)
	DeleteGroup(ctx context.Context, id string) error
	SetGroupName(ctx context.Context, id string, name string) error
	GetGroups(ctx context.Context, userID string) ([]Group, error)
	CreateGroupInvite(ctx context.Context, groupID string, codeHash string, expiresAt time.Time, createdBy string) (*GroupInvite, error)
	DeleteGroupInvite(ctx context.Context, inviteID string) error
	GetGroupInvite(ctx context.Context, codeHash string) (*GroupInvite, error)
	ConsumeGroupInvite(ctx context.Context, id string, usedBy string) error
	AddUserToGroup(ctx context.Context, groupID string, userID string) error
	RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error
	IsUserInGroup(ctx context.Context, groupID string, userID string) (bool, error)
	GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error)
	GetDefaultGroupID(ctx context.Context, userID string) (string, error)
	SetDefaultGroupID(ctx context.Context, userID string, groupID string) error
}

type GroupService struct {
	repo GroupRepository
}

func NewGroupService(r GroupRepository) *GroupService {
	return &GroupService{repo: r}
}

func (s *GroupService) CreateGroup(ctx context.Context, authUserID string, name string) (*Group, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if name == "" {
		return nil, ErrNameMissing
	}

	return s.repo.CreateGroup(ctx, name, authUserID)
}

func (s *GroupService) DeleteGroup(ctx context.Context, authUserID string, groupID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if groupID == "" {
		return ErrGroupIDMissing
	}

	if err := s.UserIsOwner(ctx, authUserID, groupID); err != nil {
		return err
	}

	return s.repo.DeleteGroup(ctx, groupID)
}

func (s *GroupService) GetDefaultGroupID(ctx context.Context, userID string) (string, error) {
	return s.repo.GetDefaultGroupID(ctx, userID)
}

func (s *GroupService) GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error) {
	if groupID == "" {
		return "", ErrGroupIDMissing
	}
	if userID == "" {
		return "", ErrUserIDMissing
	}

	return s.repo.GetRoleForGroup(ctx, groupID, userID)
}

func (s *GroupService) UserIsOwner(ctx context.Context, userID string, groupID string) error {
	role, err := s.repo.GetRoleForGroup(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return ErrUserNotOwner
	}
	return nil
}
