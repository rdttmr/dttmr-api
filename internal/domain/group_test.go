package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var _ GroupRepository = (*mockGroupRepository)(nil)

type mockGroupRepository struct {
	mock.Mock
}

func (m *mockGroupRepository) CreateGroup(ctx context.Context, name string, createdBy string) (*Group, error) {
	args := m.Called(ctx, name, createdBy)
	group, _ := args.Get(0).(*Group)
	return group, args.Error(1)
}

func (m *mockGroupRepository) DeleteGroup(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockGroupRepository) SetGroupName(ctx context.Context, id string, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroups(ctx context.Context, userID string) ([]Group, error) {
	args := m.Called(ctx, userID)
	groups, _ := args.Get(0).([]Group)
	return groups, args.Error(1)
}

func (m *mockGroupRepository) CreateGroupInvite(ctx context.Context, groupID string, codeHash string, expiresAt time.Time, createdBy string) (*GroupInvite, error) {
	args := m.Called(ctx, groupID, codeHash, expiresAt, createdBy)
	invite, _ := args.Get(0).(*GroupInvite)
	return invite, args.Error(1)
}

func (m *mockGroupRepository) DeleteGroupInvite(ctx context.Context, inviteID string, createdBy string) error {
	args := m.Called(ctx, inviteID, createdBy)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroupInvite(ctx context.Context, codeHash string) (*GroupInvite, error) {
	args := m.Called(ctx, codeHash)
	invite, _ := args.Get(0).(*GroupInvite)
	return invite, args.Error(1)
}

func (m *mockGroupRepository) ConsumeGroupInvite(ctx context.Context, inviteID string, usedBy string) error {
	args := m.Called(ctx, inviteID, usedBy)
	return args.Error(0)
}

func (m *mockGroupRepository) AddUserToGroup(ctx context.Context, groupID string, userID string, role string) error {
	args := m.Called(ctx, groupID, userID, role)
	return args.Error(0)
}

func (m *mockGroupRepository) RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *mockGroupRepository) GetGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	args := m.Called(ctx, groupID)
	members, _ := args.Get(0).([]GroupMember)
	return members, args.Error(1)
}

func (m *mockGroupRepository) IsUserInGroup(ctx context.Context, groupID string, userID string) (bool, error) {
	args := m.Called(ctx, groupID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockGroupRepository) GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error) {
	args := m.Called(ctx, groupID, userID)
	return args.String(0), args.Error(1)
}

func (m *mockGroupRepository) GetDefaultGroupID(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *mockGroupRepository) SetDefaultGroupID(ctx context.Context, userID string, groupID string) error {
	args := m.Called(ctx, userID, groupID)
	return args.Error(0)
}

func assertGroupCallOrder(t *testing.T, groups *mockGroupRepository, want ...string) {
	t.Helper()
	assert.Equal(t, want, callOrder(groups.Calls))
}

func newGroupService(t *testing.T) (*GroupService, *mockGroupRepository, *fakeTransactor) {
	t.Helper()

	repo := &mockGroupRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	tx := &fakeTransactor{}
	return NewGroupService(tx, repo), repo, tx
}

func TestGroupService_CreateGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the group with the creator as owner in one transaction", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		createdAt := time.Now()
		repo.On("CreateGroup", inTx, "Family", "user-1").Return(&Group{ID: "group-1", Name: "Family", CreatedAt: createdAt}, nil)
		repo.On("AddUserToGroup", inTx, "group-1", "user-1", RoleOwner).Return(nil)

		group, err := svc.CreateGroup(ctx, "user-1", "Family")

		require.NoError(t, err)
		assert.Equal(t, &Group{
			ID:          "group-1",
			Name:        "Family",
			IsDefault:   false,
			Role:        RoleOwner,
			MemberCount: 1,
			CreatedAt:   createdAt,
		}, group)
		assert.Equal(t, 1, tx.calls)
		assertGroupCallOrder(t, repo, "CreateGroup", "AddUserToGroup")
	})

	t.Run("create error skips adding the owner", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("insert failed")
		repo.On("CreateGroup", inTx, "Family", "user-1").Return(nil, repoErr)

		group, err := svc.CreateGroup(ctx, "user-1", "Family")

		assert.Nil(t, group)
		assert.ErrorIs(t, err, repoErr)
		assertGroupCallOrder(t, repo, "CreateGroup")
	})

	t.Run("adding the owner fails the whole creation", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("insert failed")
		repo.On("CreateGroup", inTx, "Family", "user-1").Return(&Group{ID: "group-1"}, nil)
		repo.On("AddUserToGroup", inTx, "group-1", "user-1", RoleOwner).Return(repoErr)

		group, err := svc.CreateGroup(ctx, "user-1", "Family")

		assert.Nil(t, group)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		tx.err = errors.New("could not begin transaction")

		group, err := svc.CreateGroup(ctx, "user-1", "Family")

		assert.Nil(t, group)
		assert.ErrorIs(t, err, tx.err)
		assertGroupCallOrder(t, repo)
	})
}

func TestGroupService_DeleteGroup(t *testing.T) {
	ctx := context.Background()
	onlyOwner := []GroupMember{{ID: "user-1", Role: RoleOwner}}

	// expectChecks registers the checks DeleteGroup runs before deleting.
	expectChecks := func(repo *mockGroupRepository) {
		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		repo.On("GetGroupMembers", mock.Anything, "group-1").Return(onlyOwner, nil)
	}

	t.Run("owner deletes a non-default group without other members", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		expectChecks(repo)
		repo.On("DeleteGroup", mock.Anything, "group-1").Return(nil)

		require.NoError(t, svc.DeleteGroup(ctx, "user-1", "group-1"))
		assertGroupCallOrder(t, repo, "GetRoleForGroup", "GetDefaultGroupID", "GetGroupMembers", "DeleteGroup")
	})

	t.Run("non-owner is refused", func(t *testing.T) {
		tests := []struct {
			name    string
			role    string
			roleErr error
		}{
			{name: "as member", role: RoleMember},
			{name: "as viewer", role: RoleViewer},
			{name: "as non-member", roleErr: ErrUserNotInGroup},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, _ := newGroupService(t)

				repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, tt.roleErr)

				err := svc.DeleteGroup(ctx, "user-1", "group-1")

				assert.ErrorIs(t, err, ErrUserNotOwner)
				assertGroupCallOrder(t, repo, "GetRoleForGroup")
			})
		}
	})

	t.Run("default group is kept", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-default", "user-1").Return(RoleOwner, nil)
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)

		err := svc.DeleteGroup(ctx, "user-1", "group-default")

		assert.ErrorIs(t, err, ErrGroupIsDefault)
		assertGroupCallOrder(t, repo, "GetRoleForGroup", "GetDefaultGroupID")
	})

	t.Run("group with other members is kept", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		repo.On("GetGroupMembers", mock.Anything, "group-1").
			Return([]GroupMember{{ID: "user-1", Role: RoleOwner}, {ID: "user-2", Role: RoleViewer}}, nil)

		err := svc.DeleteGroup(ctx, "user-1", "group-1")

		assert.ErrorIs(t, err, ErrGroupHasMembers)
		assertGroupCallOrder(t, repo, "GetRoleForGroup", "GetDefaultGroupID", "GetGroupMembers")
	})

	t.Run("role lookup error is propagated, not masked", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		roleErr := errors.New("connection reset")
		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return("", roleErr)

		err := svc.DeleteGroup(ctx, "user-1", "group-1")

		assert.ErrorIs(t, err, roleErr)
		assert.NotErrorIs(t, err, ErrUserNotOwner)
	})

	lookupFailures := []string{"GetDefaultGroupID", "GetGroupMembers", "DeleteGroup"}
	for _, failing := range lookupFailures {
		t.Run(failing+" error is propagated", func(t *testing.T) {
			svc, repo, _ := newGroupService(t)

			repoErr := errors.New(failing + " failed")
			expectChecks(repo)
			repo.On("DeleteGroup", mock.Anything, "group-1").Return(nil).Maybe()
			for _, c := range repo.ExpectedCalls {
				if c.Method == failing {
					c.ReturnArguments[len(c.ReturnArguments)-1] = repoErr
				}
			}
			// Steps after the failing one never run.
			for _, c := range repo.ExpectedCalls {
				c.Maybe()
			}

			err := svc.DeleteGroup(ctx, "user-1", "group-1")

			assert.ErrorIs(t, err, repoErr)
			assert.Equal(t, failing, callOrder(repo.Calls)[len(repo.Calls)-1])
		})
	}
}

func TestGroupService_SetGroupName(t *testing.T) {
	ctx := context.Background()

	for _, role := range []string{RoleOwner, RoleMember} {
		t.Run("renames the group as "+role, func(t *testing.T) {
			svc, repo, _ := newGroupService(t)

			repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(role, nil)
			repo.On("SetGroupName", mock.Anything, "group-1", "Flatmates").Return(nil)

			require.NoError(t, svc.SetGroupName(ctx, "user-1", "group-1", "Flatmates"))
			assertGroupCallOrder(t, repo, "GetRoleForGroup", "SetGroupName")
		})
	}

	t.Run("without write permission is refused", func(t *testing.T) {
		tests := []struct {
			name    string
			role    string
			roleErr error
		}{
			{name: "as viewer", role: RoleViewer},
			{name: "as non-member", roleErr: ErrUserNotInGroup},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc, repo, _ := newGroupService(t)

				repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, tt.roleErr)

				err := svc.SetGroupName(ctx, "user-1", "group-1", "Flatmates")

				assert.ErrorIs(t, err, ErrUserNoWritePermissions)
				assertGroupCallOrder(t, repo, "GetRoleForGroup")
			})
		}
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("update failed")
		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)
		repo.On("SetGroupName", mock.Anything, "group-1", "Flatmates").Return(repoErr)

		assert.ErrorIs(t, svc.SetGroupName(ctx, "user-1", "group-1", "Flatmates"), repoErr)
	})
}

func TestGroupService_GetGroups(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user's groups", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		found := []Group{{ID: "group-1", Name: "Personal"}}
		repo.On("GetGroups", mock.Anything, "user-1").Return(found, nil)

		groups, err := svc.GetGroups(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, found, groups)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetGroups", mock.Anything, "user-1").Return(nil, repoErr)

		groups, err := svc.GetGroups(ctx, "user-1")

		assert.Nil(t, groups)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestGroupService_ShareGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("returns a random code and stores only its hash, expiring in 7 days", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleMember, nil)
		repo.On("CreateGroupInvite", mock.Anything, "group-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time"), "user-1").
			Return(&GroupInvite{ID: "invite-1", GroupID: "group-1"}, nil)

		before := time.Now()
		invite, err := svc.ShareGroup(ctx, "user-1", "group-1")

		require.NoError(t, err)
		assert.Equal(t, "invite-1", invite.ID)
		assert.Len(t, invite.Code, 64)
		_, hexErr := hex.DecodeString(invite.Code)
		assert.NoError(t, hexErr, "code is not hex: %q", invite.Code)

		call := repo.Calls[len(repo.Calls)-1]
		assert.Equal(t, hashToken(invite.Code), call.Arguments.String(2))
		assert.NotEqual(t, invite.Code, call.Arguments.String(2))
		assert.WithinDuration(t, before.Add(7*24*time.Hour), call.Arguments.Get(3).(time.Time), 5*time.Second)
	})

	t.Run("without write permission is refused", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleViewer, nil)

		invite, err := svc.ShareGroup(ctx, "user-1", "group-1")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, ErrUserNoWritePermissions)
		assertGroupCallOrder(t, repo, "GetRoleForGroup")
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("insert failed")
		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)
		repo.On("CreateGroupInvite", mock.Anything, "group-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time"), "user-1").
			Return(nil, repoErr)

		invite, err := svc.ShareGroup(ctx, "user-1", "group-1")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestGroupService_DeleteInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the invite of its creator", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("DeleteGroupInvite", mock.Anything, "invite-1", "user-1").Return(nil)

		assert.NoError(t, svc.DeleteInvite(ctx, "user-1", "invite-1"))
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("DeleteGroupInvite", mock.Anything, "invite-1", "user-1").Return(ErrInviteConsumed)

		assert.ErrorIs(t, svc.DeleteInvite(ctx, "user-1", "invite-1"), ErrInviteConsumed)
	})
}

func TestGroupService_JoinGroup(t *testing.T) {
	ctx := context.Background()
	invite := &GroupInvite{ID: "invite-1", GroupID: "group-1"}

	t.Run("consumes the invite and joins as member in one transaction", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		repo.On("GetGroupInvite", notInTx, hashToken("join-code")).Return(invite, nil)
		repo.On("ConsumeGroupInvite", inTx, "invite-1", "user-2").Return(nil)
		repo.On("AddUserToGroup", inTx, "group-1", "user-2", RoleMember).Return(nil)

		groupID, err := svc.JoinGroup(ctx, "user-2", "join-code")

		require.NoError(t, err)
		assert.Equal(t, "group-1", groupID)
		assert.Equal(t, 1, tx.calls)
		assertGroupCallOrder(t, repo, "GetGroupInvite", "ConsumeGroupInvite", "AddUserToGroup")
	})

	t.Run("unknown code is refused before the transaction", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		repo.On("GetGroupInvite", notInTx, hashToken("wrong-code")).Return(nil, ErrInviteInvalid)

		groupID, err := svc.JoinGroup(ctx, "user-2", "wrong-code")

		assert.Empty(t, groupID)
		assert.ErrorIs(t, err, ErrInviteInvalid)
		assert.Zero(t, tx.calls)
	})

	t.Run("consume error skips adding the member", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetGroupInvite", notInTx, hashToken("join-code")).Return(invite, nil)
		repo.On("ConsumeGroupInvite", inTx, "invite-1", "user-2").Return(ErrInviteInvalid)

		groupID, err := svc.JoinGroup(ctx, "user-2", "join-code")

		assert.Empty(t, groupID)
		assert.ErrorIs(t, err, ErrInviteInvalid)
		assertGroupCallOrder(t, repo, "GetGroupInvite", "ConsumeGroupInvite")
	})

	t.Run("adding the member fails the whole join", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("duplicate key value violates unique constraint")
		repo.On("GetGroupInvite", notInTx, hashToken("join-code")).Return(invite, nil)
		repo.On("ConsumeGroupInvite", inTx, "invite-1", "user-2").Return(nil)
		repo.On("AddUserToGroup", inTx, "group-1", "user-2", RoleMember).Return(repoErr)

		groupID, err := svc.JoinGroup(ctx, "user-2", "join-code")

		assert.Empty(t, groupID)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("transaction begin error is returned", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		tx.err = errors.New("could not begin transaction")
		repo.On("GetGroupInvite", notInTx, hashToken("join-code")).Return(invite, nil)

		groupID, err := svc.JoinGroup(ctx, "user-2", "join-code")

		assert.Empty(t, groupID)
		assert.ErrorIs(t, err, tx.err)
		assertGroupCallOrder(t, repo, "GetGroupInvite")
	})
}

func TestGroupService_LeaveGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("member leaves a non-default group", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleMember, nil)
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		repo.On("IsUserInGroup", mock.Anything, "group-1", "user-1").Return(true, nil)
		repo.On("RemoveUserFromGroup", mock.Anything, "group-1", "user-1").Return(nil)

		require.NoError(t, svc.LeaveGroup(ctx, "user-1", "group-1"))
		assert.Equal(t, "RemoveUserFromGroup", callOrder(repo.Calls)[len(repo.Calls)-1])
	})

	t.Run("owner cannot leave", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleOwner, nil)

		assert.ErrorIs(t, svc.LeaveGroup(ctx, "user-1", "group-1"), ErrOwnerCantLeave)
		assert.NotContains(t, callOrder(repo.Calls), "RemoveUserFromGroup")
	})

	t.Run("default group cannot be left", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-default", "user-1").Return(RoleMember, nil)
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil)
		repo.On("IsUserInGroup", mock.Anything, "group-default", "user-1").Return(true, nil).Maybe()

		assert.ErrorIs(t, svc.LeaveGroup(ctx, "user-1", "group-default"), ErrGroupIsDefault)
		assert.NotContains(t, callOrder(repo.Calls), "RemoveUserFromGroup")
	})

	// The checks before the membership check are optional here, so moving the
	// membership check first doesn't break this test.
	t.Run("non-member is refused", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return("", ErrUserNotInGroup).Maybe()
		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil).Maybe()
		repo.On("IsUserInGroup", mock.Anything, "group-1", "user-1").Return(false, nil)

		assert.ErrorIs(t, svc.LeaveGroup(ctx, "user-1", "group-1"), ErrUserNotInGroup)
		assert.NotContains(t, callOrder(repo.Calls), "RemoveUserFromGroup")
	})

	failures := []string{"GetRoleForGroup", "GetDefaultGroupID", "IsUserInGroup", "RemoveUserFromGroup"}
	for _, failing := range failures {
		t.Run(failing+" error is propagated", func(t *testing.T) {
			svc, repo, _ := newGroupService(t)

			repoErr := errors.New(failing + " failed")
			repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleMember, nil).Maybe()
			repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-default", nil).Maybe()
			repo.On("IsUserInGroup", mock.Anything, "group-1", "user-1").Return(true, nil).Maybe()
			repo.On("RemoveUserFromGroup", mock.Anything, "group-1", "user-1").Return(nil).Maybe()
			for _, c := range repo.ExpectedCalls {
				if c.Method == failing {
					c.ReturnArguments[len(c.ReturnArguments)-1] = repoErr
				}
			}

			err := svc.LeaveGroup(ctx, "user-1", "group-1")

			assert.ErrorIs(t, err, repoErr)
			assert.Contains(t, callOrder(repo.Calls), failing)
			if failing != "RemoveUserFromGroup" {
				assert.NotContains(t, callOrder(repo.Calls), "RemoveUserFromGroup")
			}
		})
	}
}

func TestGroupService_GetGroupMembers(t *testing.T) {
	ctx := context.Background()

	t.Run("member sees the members", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		members := []GroupMember{{ID: "user-1", Role: RoleOwner}, {ID: "user-2", Role: RoleViewer}}
		repo.On("IsUserInGroup", mock.Anything, "group-1", "user-2").Return(true, nil)
		repo.On("GetGroupMembers", mock.Anything, "group-1").Return(members, nil)

		got, err := svc.GetGroupMembers(ctx, "user-2", "group-1")

		require.NoError(t, err)
		assert.Equal(t, members, got)
	})

	t.Run("non-member is refused", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("IsUserInGroup", mock.Anything, "group-1", "intruder").Return(false, nil)

		got, err := svc.GetGroupMembers(ctx, "intruder", "group-1")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrUserNotInGroup)
		assertGroupCallOrder(t, repo, "IsUserInGroup")
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repoErr := errors.New("connection reset")
		repo.On("IsUserInGroup", mock.Anything, "group-1", "user-1").Return(true, nil)
		repo.On("GetGroupMembers", mock.Anything, "group-1").Return(nil, repoErr)

		got, err := svc.GetGroupMembers(ctx, "user-1", "group-1")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestGroupService_SetDefaultGroupID(t *testing.T) {
	ctx := context.Background()

	t.Run("member sets the default inside a transaction", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		repo.On("IsUserInGroup", mock.Anything, "group-2", "user-1").Return(true, nil)
		repo.On("SetDefaultGroupID", inTx, "user-1", "group-2").Return(nil)

		require.NoError(t, svc.SetDefaultGroupID(ctx, "user-1", "group-2"))
		assert.Equal(t, 1, tx.calls)
		assertGroupCallOrder(t, repo, "IsUserInGroup", "SetDefaultGroupID")
	})

	t.Run("non-member is refused before the transaction", func(t *testing.T) {
		svc, repo, tx := newGroupService(t)

		repo.On("IsUserInGroup", mock.Anything, "group-foreign", "user-1").Return(false, nil)

		assert.ErrorIs(t, svc.SetDefaultGroupID(ctx, "user-1", "group-foreign"), ErrUserNotInGroup)
		assert.Zero(t, tx.calls)
	})

	t.Run("repository error is propagated", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("IsUserInGroup", mock.Anything, "group-2", "user-1").Return(true, nil)
		repo.On("SetDefaultGroupID", inTx, "user-1", "group-2").Return(ErrUserNotInGroup)

		assert.ErrorIs(t, svc.SetDefaultGroupID(ctx, "user-1", "group-2"), ErrUserNotInGroup)
	})
}

func TestGroupService_PassThrough(t *testing.T) {
	ctx := context.Background()

	t.Run("AddUserToGroup", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("AddUserToGroup", mock.Anything, "group-1", "user-2", RoleViewer).Return(nil)

		assert.NoError(t, svc.AddUserToGroup(ctx, "group-1", "user-2", RoleViewer))
	})

	t.Run("RemoveUserFromGroup", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("RemoveUserFromGroup", mock.Anything, "group-1", "user-2").Return(nil)

		assert.NoError(t, svc.RemoveUserFromGroup(ctx, "group-1", "user-2"))
	})

	t.Run("GetRoleForGroup", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(RoleMember, nil)

		role, err := svc.GetRoleForGroup(ctx, "group-1", "user-1")

		require.NoError(t, err)
		assert.Equal(t, RoleMember, role)
	})

	t.Run("GetDefaultGroupID", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("GetDefaultGroupID", mock.Anything, "user-1").Return("group-1", nil)

		groupID, err := svc.GetDefaultGroupID(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, "group-1", groupID)
	})
}

func TestGroupService_Permissions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		role      string
		roleErr   error
		wantOwner error
		wantWrite error
	}{
		{name: "owner", role: RoleOwner, wantOwner: nil, wantWrite: nil},
		{name: "member", role: RoleMember, wantOwner: ErrUserNotOwner, wantWrite: nil},
		{name: "viewer", role: RoleViewer, wantOwner: ErrUserNotOwner, wantWrite: ErrUserNoWritePermissions},
		{name: "unknown role", role: "admin", wantOwner: ErrUserNotOwner, wantWrite: ErrUserNoWritePermissions},
		{name: "non-member", roleErr: ErrUserNotInGroup, wantOwner: ErrUserNotOwner, wantWrite: ErrUserNoWritePermissions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("UserIsOwner", func(t *testing.T) {
				svc, repo, _ := newGroupService(t)

				repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, tt.roleErr)

				err := svc.UserIsOwner(ctx, "user-1", "group-1")

				if tt.wantOwner == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, tt.wantOwner)
				}
			})

			t.Run("UserHasWritePermission", func(t *testing.T) {
				svc, repo, _ := newGroupService(t)

				repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return(tt.role, tt.roleErr)

				err := svc.UserHasWritePermission(ctx, "user-1", "group-1")

				if tt.wantWrite == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, tt.wantWrite)
				}
			})
		})
	}

	t.Run("role lookup error is propagated, not masked", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		roleErr := errors.New("connection reset")
		repo.On("GetRoleForGroup", mock.Anything, "group-1", "user-1").Return("", roleErr)

		ok, err := svc.UserHasRole(ctx, "user-1", "group-1", []string{RoleOwner})

		assert.False(t, ok)
		assert.ErrorIs(t, err, roleErr)
	})

	t.Run("UserInGroup", func(t *testing.T) {
		svc, repo, _ := newGroupService(t)

		repo.On("IsUserInGroup", mock.Anything, "group-1", "user-1").Return(true, nil)

		assert.NoError(t, svc.UserInGroup(ctx, "user-1", "group-1"))
	})
}

func TestGroupService_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		call    func(svc *GroupService) error
		wantErr error
	}{
		{name: "CreateGroup without auth user id", call: func(svc *GroupService) error { _, err := svc.CreateGroup(ctx, "", "Family"); return err }, wantErr: ErrUserIDMissing},
		{name: "CreateGroup without name", call: func(svc *GroupService) error { _, err := svc.CreateGroup(ctx, "user-1", ""); return err }, wantErr: ErrNameMissing},
		{name: "DeleteGroup without auth user id", call: func(svc *GroupService) error { return svc.DeleteGroup(ctx, "", "group-1") }, wantErr: ErrUserIDMissing},
		{name: "DeleteGroup without group id", call: func(svc *GroupService) error { return svc.DeleteGroup(ctx, "user-1", "") }, wantErr: ErrGroupIDMissing},
		{name: "SetGroupName without auth user id", call: func(svc *GroupService) error { return svc.SetGroupName(ctx, "", "group-1", "Flatmates") }, wantErr: ErrUserIDMissing},
		{name: "SetGroupName without group id", call: func(svc *GroupService) error { return svc.SetGroupName(ctx, "user-1", "", "Flatmates") }, wantErr: ErrGroupIDMissing},
		{name: "SetGroupName without name", call: func(svc *GroupService) error { return svc.SetGroupName(ctx, "user-1", "group-1", "") }, wantErr: ErrNameMissing},
		{name: "GetGroups without auth user id", call: func(svc *GroupService) error { _, err := svc.GetGroups(ctx, ""); return err }, wantErr: ErrUserIDMissing},
		{name: "ShareGroup without auth user id", call: func(svc *GroupService) error { _, err := svc.ShareGroup(ctx, "", "group-1"); return err }, wantErr: ErrUserIDMissing},
		{name: "ShareGroup without group id", call: func(svc *GroupService) error { _, err := svc.ShareGroup(ctx, "user-1", ""); return err }, wantErr: ErrGroupIDMissing},
		{name: "DeleteInvite without auth user id", call: func(svc *GroupService) error { return svc.DeleteInvite(ctx, "", "invite-1") }, wantErr: ErrUserIDMissing},
		{name: "DeleteInvite without invite id", call: func(svc *GroupService) error { return svc.DeleteInvite(ctx, "user-1", "") }, wantErr: ErrInviteIDMissing},
		{name: "JoinGroup without auth user id", call: func(svc *GroupService) error { _, err := svc.JoinGroup(ctx, "", "join-code"); return err }, wantErr: ErrUserIDMissing},
		{name: "JoinGroup without join code", call: func(svc *GroupService) error { _, err := svc.JoinGroup(ctx, "user-1", ""); return err }, wantErr: ErrJoinCodeMissing},
		{name: "LeaveGroup without auth user id", call: func(svc *GroupService) error { return svc.LeaveGroup(ctx, "", "group-1") }, wantErr: ErrUserIDMissing},
		{name: "LeaveGroup without group id", call: func(svc *GroupService) error { return svc.LeaveGroup(ctx, "user-1", "") }, wantErr: ErrGroupIDMissing},
		{name: "AddUserToGroup without group id", call: func(svc *GroupService) error { return svc.AddUserToGroup(ctx, "", "user-1", RoleMember) }, wantErr: ErrGroupIDMissing},
		{name: "AddUserToGroup without user id", call: func(svc *GroupService) error { return svc.AddUserToGroup(ctx, "group-1", "", RoleMember) }, wantErr: ErrUserIDMissing},
		{name: "AddUserToGroup without role", call: func(svc *GroupService) error { return svc.AddUserToGroup(ctx, "group-1", "user-1", "") }, wantErr: ErrRoleMissing},
		{name: "RemoveUserFromGroup without group id", call: func(svc *GroupService) error { return svc.RemoveUserFromGroup(ctx, "", "user-1") }, wantErr: ErrGroupIDMissing},
		{name: "RemoveUserFromGroup without user id", call: func(svc *GroupService) error { return svc.RemoveUserFromGroup(ctx, "group-1", "") }, wantErr: ErrUserIDMissing},
		{name: "GetGroupMembers without auth user id", call: func(svc *GroupService) error { _, err := svc.GetGroupMembers(ctx, "", "group-1"); return err }, wantErr: ErrUserIDMissing},
		{name: "GetGroupMembers without group id", call: func(svc *GroupService) error { _, err := svc.GetGroupMembers(ctx, "user-1", ""); return err }, wantErr: ErrGroupIDMissing},
		{name: "GetRoleForGroup without group id", call: func(svc *GroupService) error { _, err := svc.GetRoleForGroup(ctx, "", "user-1"); return err }, wantErr: ErrGroupIDMissing},
		{name: "GetRoleForGroup without user id", call: func(svc *GroupService) error { _, err := svc.GetRoleForGroup(ctx, "group-1", ""); return err }, wantErr: ErrUserIDMissing},
		{name: "GetDefaultGroupID without user id", call: func(svc *GroupService) error { _, err := svc.GetDefaultGroupID(ctx, ""); return err }, wantErr: ErrUserIDMissing},
		{name: "SetDefaultGroupID without user id", call: func(svc *GroupService) error { return svc.SetDefaultGroupID(ctx, "", "group-1") }, wantErr: ErrUserIDMissing},
		{name: "SetDefaultGroupID without group id", call: func(svc *GroupService) error { return svc.SetDefaultGroupID(ctx, "user-1", "") }, wantErr: ErrGroupIDMissing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, tx := newGroupService(t)

			err := tt.call(svc)

			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, tx.calls)
			assertGroupCallOrder(t, repo)
		})
	}
}
