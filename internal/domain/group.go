package domain

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrGroupIDMissing         = errors.New("group id is required")
	ErrMustHaveOneGroup       = errors.New("user must have at least one group")
	ErrRoleMissing            = errors.New("role is required")
	ErrUserNotInGroup         = errors.New("user is not in group")
	ErrUserNotOwner           = errors.New("user is not owner")
	ErrUserNoWritePermissions = errors.New("user has no permissions to write")
)

var (
	RoleOwner  = "owner"
	RoleMember = "member"
	RoleViewer = "viewer"
)

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IsDefault   bool      `json:"is_default"`
	Role        string    `json:"role"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

type GroupInvite struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GroupMember struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupRepository interface {
	CreateGroup(ctx context.Context, name string, createdBy string) (*Group, error)
	DeleteGroup(ctx context.Context, id string) error
	SetGroupName(ctx context.Context, id string, name string) error
	GetGroups(ctx context.Context, userID string) ([]Group, error)
	CreateGroupInvite(ctx context.Context, groupID string, codeHash string, expiresAt time.Time, createdBy string) (*GroupInvite, error)
	DeleteGroupInvite(ctx context.Context, inviteID string, createdBy string) error
	GetGroupInvite(ctx context.Context, codeHash string) (*GroupInvite, error)
	ConsumeGroupInvite(ctx context.Context, inviteID string, usedBy string) error
	AddUserToGroup(ctx context.Context, groupID string, userID string, role string) error
	RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error
	GetGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error)
	IsUserInGroup(ctx context.Context, groupID string, userID string) (bool, error)
	GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error)
	GetDefaultGroupID(ctx context.Context, userID string) (string, error)
	SetDefaultGroupID(ctx context.Context, userID string, groupID string) error
}

type GroupService struct {
	tx   Transactor
	repo GroupRepository
}

func NewGroupService(tx Transactor, r GroupRepository) *GroupService {
	return &GroupService{tx: tx, repo: r}
}

func (s *GroupService) CreateGroup(ctx context.Context, authUserID string, name string) (*Group, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if name == "" {
		return nil, ErrNameMissing
	}

	var group *Group
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		g, err := s.repo.CreateGroup(ctx, name, authUserID)
		if err != nil {
			return err
		}

		err = s.repo.AddUserToGroup(ctx, g.ID, authUserID, RoleOwner)
		if err != nil {
			return err
		}

		g.IsDefault = false
		g.MemberCount = 1
		g.Role = RoleOwner
		group = g
		return nil
	})
	if err != nil {
		return nil, err
	}

	return group, nil
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

	groups, err := s.GetGroups(ctx, authUserID)
	if err != nil {
		return err
	}
	if len(groups) < 2 {
		return ErrMustHaveOneGroup
	}

	return s.repo.DeleteGroup(ctx, groupID)
}

func (s *GroupService) SetGroupName(ctx context.Context, authUserID string, groupID string, name string) error {
	if groupID == "" {
		return ErrGroupIDMissing
	}
	if name == "" {
		return ErrNameMissing
	}

	if err := s.UserHasWritePermission(ctx, authUserID, groupID); err != nil {
		return err
	}

	return s.repo.SetGroupName(ctx, groupID, name)
}

func (s *GroupService) GetGroups(ctx context.Context, authUserID string) ([]Group, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}

	groups, err := s.repo.GetGroups(ctx, authUserID)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *GroupService) ShareGroup(ctx context.Context, authUserID string, groupID string) (*GroupInvite, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if groupID == "" {
		return nil, ErrGroupIDMissing
	}

	if err := s.UserHasWritePermission(ctx, authUserID, groupID); err != nil {
		return nil, err
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}
	code := hashToken(token)
	expiresAt := time.Now().Add(time.Hour * 24 * 7)

	invite, err := s.repo.CreateGroupInvite(ctx, groupID, code, expiresAt, authUserID)
	if err != nil {
		return nil, err
	}

	invite.Code = token
	return invite, nil
}

func (s *GroupService) DeleteInvite(ctx context.Context, authUserID string, inviteID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if inviteID == "" {
		return ErrInviteIDMissing
	}

	return s.repo.DeleteGroupInvite(ctx, inviteID, authUserID)
}

func (s *GroupService) JoinGroup(ctx context.Context, authUserID, joinCode string) (string, error) {
	if authUserID == "" {
		return "", ErrUserIDMissing
	}
	if joinCode == "" {
		return "", ErrJoinCodeMissing
	}

	invite, err := s.repo.GetGroupInvite(ctx, hashToken(joinCode))
	if err != nil {
		return "", err
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		err := s.repo.ConsumeGroupInvite(ctx, invite.ID, authUserID)
		if err != nil {
			return err
		}

		err = s.repo.AddUserToGroup(ctx, invite.GroupID, authUserID, RoleMember)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return invite.GroupID, nil
}

func (s *GroupService) LeaveGroup(ctx context.Context, authUserID string, groupID string) error {
	if authUserID == "" {
		return ErrUserIDMissing
	}
	if groupID == "" {
		return ErrGroupIDMissing
	}

	if err := s.UserInGroup(ctx, authUserID, groupID); err != nil {
		return err
	}

	return s.repo.RemoveUserFromGroup(ctx, groupID, authUserID)
}

func (s *GroupService) AddUserToGroup(ctx context.Context, groupID string, userID string, role string) error {
	if groupID == "" {
		return ErrGroupIDMissing
	}
	if userID == "" {
		return ErrUserIDMissing
	}
	if role == "" {
		return ErrRoleMissing
	}

	return s.repo.AddUserToGroup(ctx, groupID, userID, role)
}

func (s *GroupService) RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error {
	if groupID == "" {
		return ErrGroupIDMissing
	}
	if userID == "" {
		return ErrUserIDMissing
	}

	return s.repo.RemoveUserFromGroup(ctx, groupID, userID)
}

func (s *GroupService) GetGroupMembers(ctx context.Context, authUserID string, groupID string) ([]GroupMember, error) {
	if authUserID == "" {
		return nil, ErrUserIDMissing
	}
	if groupID == "" {
		return nil, ErrGroupIDMissing
	}

	if err := s.UserInGroup(ctx, authUserID, groupID); err != nil {
		return nil, err
	}

	return s.repo.GetGroupMembers(ctx, groupID)
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

func (s *GroupService) GetDefaultGroupID(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", ErrUserIDMissing
	}

	return s.repo.GetDefaultGroupID(ctx, userID)
}

func (s *GroupService) SetDefaultGroupID(ctx context.Context, userID string, groupID string) error {
	if userID == "" {
		return ErrUserIDMissing
	}
	if groupID == "" {
		return ErrGroupIDMissing
	}

	if err := s.UserInGroup(ctx, userID, groupID); err != nil {
		return err
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.repo.SetDefaultGroupID(ctx, userID, groupID)
	})
}

func (s *GroupService) UserInGroup(ctx context.Context, userID string, groupID string) error {
	inGroup, err := s.repo.IsUserInGroup(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !inGroup {
		return ErrUserNotInGroup
	}
	return nil
}

// UserHasRole checks if user has one of the provided roles
func (s *GroupService) UserHasRole(ctx context.Context, userID string, groupID string, roles []string) (bool, error) {
	role, err := s.repo.GetRoleForGroup(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotInGroup) {
			return false, nil
		}
		return false, err
	}
	if !slices.Contains(roles, role) {
		return false, nil
	}

	return true, nil
}

func (s *GroupService) UserIsOwner(ctx context.Context, userID string, groupID string) error {
	isOwner, err := s.UserHasRole(ctx, userID, groupID, []string{RoleOwner})
	if err != nil {
		return err
	} else if !isOwner {
		return ErrUserNotOwner
	}
	return nil
}

func (s *GroupService) UserHasWritePermission(ctx context.Context, userID, groupID string) error {
	hasWrite, err := s.UserHasRole(ctx, userID, groupID, []string{RoleOwner, RoleMember})
	if err != nil {
		return err
	} else if !hasWrite {
		return ErrUserNoWritePermissions
	}
	return nil
}
