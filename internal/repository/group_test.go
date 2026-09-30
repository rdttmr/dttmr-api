package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

func newGroupRepo(t *testing.T) (*GroupRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	return &GroupRepo{Repo: NewRepo(NewTransactor(db))}, mock
}

const (
	insertGroupQuery        = "INSERT INTO groups (name, created_by) VALUES ($1, $2) RETURNING id, created_at, modified_at"
	deleteGroupQuery        = "DELETE FROM groups WHERE id = $1"
	updateGroupNameQuery    = "UPDATE groups SET name = $1, modified_at = NOW() WHERE id = $2"
	selectGroupsQuery       = "SELECT g.id, g.name, gm.is_default, gm.role, (SELECT COUNT(*) FROM group_members igm WHERE igm.group_id = g.id), g.created_at, g.modified_at FROM groups AS g INNER JOIN group_members gm ON g.id=gm.group_id WHERE gm.user_id = $1"
	insertGroupInviteQuery  = "INSERT INTO group_invites (group_id, code_hash, expires_at, created_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at"
	deleteGroupInviteQuery  = "DELETE FROM group_invites WHERE id = $1 AND created_by = $2 AND consumed_at IS NULL"
	selectGroupInviteQuery  = "SELECT id, group_id, created_at, expires_at FROM group_invites WHERE code_hash = $1"
	consumeGroupInviteQuery = "UPDATE group_invites SET used_by = $1, consumed_at = NOW() WHERE id = $2 AND expires_at > NOW() AND consumed_at IS NULL"
	insertGroupMemberQuery  = "INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3)"
	deleteGroupMemberQuery  = "DELETE FROM group_members WHERE group_id = $1 AND user_id = $2"
	selectGroupMembersQuery = "SELECT u.id, u.email, u.name, gm.role, gm.created_at FROM group_members gm INNER JOIN users u ON gm.user_id=u.id WHERE gm.group_id = $1"
	isUserInGroupQuery      = "SELECT COUNT(*) FROM group_members WHERE group_id = $1 AND user_id = $2"
	selectRoleQuery         = "SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2"
	selectDefaultGroupQuery = "SELECT group_id FROM group_members WHERE user_id = $1 AND is_default = TRUE"
	resetDefaultGroupQuery  = "UPDATE group_members SET is_default = FALSE WHERE user_id = $1"
	setDefaultGroupQuery    = "UPDATE group_members SET is_default = TRUE WHERE user_id = $1 AND group_id = $2"
)

var (
	groupColumns       = []string{"id", "name", "is_default", "role", "member_count", "created_at", "modified_at"}
	groupMemberColumns = []string{"id", "email", "name", "role", "created_at"}
)

func TestGroupRepo_CreateGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
		modifiedAt := createdAt.Add(time.Minute)
		mock.ExpectQuery(insertGroupQuery).
			WithArgs("Family", "user-1").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "created_at", "modified_at"}).
					AddRow("group-1", createdAt, modifiedAt),
			)

		group, err := repo.CreateGroup(context.Background(), "Family", "user-1")

		require.NoError(t, err)
		assert.Equal(t, &domain.Group{
			ID:         "group-1",
			Name:       "Family",
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
		}, group)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("foreign key violation")
		mock.ExpectQuery(insertGroupQuery).
			WithArgs("Family", "user-1").
			WillReturnError(dbErr)

		group, err := repo.CreateGroup(context.Background(), "Family", "user-1")

		assert.Nil(t, group)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to create group")
	})
}

func TestGroupRepo_DeleteGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(deleteGroupQuery).
			WithArgs("group-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteGroup(context.Background(), "group-1"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteGroupQuery).
			WithArgs("group-1").
			WillReturnError(dbErr)

		err := repo.DeleteGroup(context.Background(), "group-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete group")
	})
}

func TestGroupRepo_SetGroupName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(updateGroupNameQuery).
			WithArgs("Flatmates", "group-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetGroupName(context.Background(), "group-1", "Flatmates"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(updateGroupNameQuery).
			WithArgs("Flatmates", "group-1").
			WillReturnError(dbErr)

		err := repo.SetGroupName(context.Background(), "group-1", "Flatmates")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to update group")
	})
}

func TestGroupRepo_GetGroups(t *testing.T) {
	createdAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	modifiedAt := createdAt.Add(time.Hour)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(groupColumns).
					AddRow("group-1", "Personal", true, domain.RoleOwner, 1, createdAt, modifiedAt).
					AddRow("group-2", "Family", false, domain.RoleMember, 4, createdAt, modifiedAt),
			)

		groups, err := repo.GetGroups(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.Group{
			{ID: "group-1", Name: "Personal", IsDefault: true, Role: domain.RoleOwner, MemberCount: 1, CreatedAt: createdAt, ModifiedAt: modifiedAt},
			{ID: "group-2", Name: "Family", IsDefault: false, Role: domain.RoleMember, MemberCount: 4, CreatedAt: createdAt, ModifiedAt: modifiedAt},
		}, groups)
	})

	t.Run("no rows returns no groups", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupsQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(groupColumns))

		groups, err := repo.GetGroups(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectGroupsQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		groups, err := repo.GetGroups(context.Background(), "user-1")

		assert.Nil(t, groups)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get groups")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(groupColumns).
					AddRow("group-1", "Personal", true, domain.RoleOwner, "not a number", createdAt, modifiedAt),
			)

		groups, err := repo.GetGroups(context.Background(), "user-1")

		assert.Nil(t, groups)
		assert.Error(t, err)
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectGroupsQuery).
			WithArgs("user-1").
			WillReturnRows(
				sqlmock.NewRows(groupColumns).
					AddRow("group-1", "Personal", true, domain.RoleOwner, 1, createdAt, modifiedAt).
					AddRow("group-2", "Family", false, domain.RoleMember, 4, createdAt, modifiedAt).
					RowError(1, rowErr),
			)

		groups, err := repo.GetGroups(context.Background(), "user-1")

		assert.Nil(t, groups)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get groups")
	})
}

func TestGroupRepo_CreateGroupInvite(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)

	t.Run("success without the code", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		createdAt := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
		mock.ExpectQuery(insertGroupInviteQuery).
			WithArgs("group-1", "hash", expiresAt, "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("invite-1", createdAt))

		invite, err := repo.CreateGroupInvite(context.Background(), "group-1", "hash", expiresAt, "user-1")

		require.NoError(t, err)
		assert.Equal(t, &domain.GroupInvite{
			ID:        "invite-1",
			GroupID:   "group-1",
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		}, invite)
		assert.Empty(t, invite.Code)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectQuery(insertGroupInviteQuery).
			WithArgs("group-1", "hash", expiresAt, "user-1").
			WillReturnError(dbErr)

		invite, err := repo.CreateGroupInvite(context.Background(), "group-1", "hash", expiresAt, "user-1")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to create group invite")
	})
}

func TestGroupRepo_DeleteGroupInvite(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(deleteGroupInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.DeleteGroupInvite(context.Background(), "invite-1", "user-1"))
	})

	t.Run("no row deleted is reported as consumed", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(deleteGroupInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.DeleteGroupInvite(context.Background(), "invite-1", "user-1")

		assert.ErrorIs(t, err, domain.ErrInviteConsumed)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteGroupInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnError(dbErr)

		err := repo.DeleteGroupInvite(context.Background(), "invite-1", "user-1")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to delete group invite")
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(deleteGroupInviteQuery).
			WithArgs("invite-1", "user-1").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.DeleteGroupInvite(context.Background(), "invite-1", "user-1")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})
}

func TestGroupRepo_GetGroupInvite(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		createdAt := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
		expiresAt := createdAt.Add(7 * 24 * time.Hour)
		mock.ExpectQuery(selectGroupInviteQuery).
			WithArgs("hash").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "group_id", "created_at", "expires_at"}).
					AddRow("invite-1", "group-1", createdAt, expiresAt),
			)

		invite, err := repo.GetGroupInvite(context.Background(), "hash")

		require.NoError(t, err)
		assert.Equal(t, &domain.GroupInvite{
			ID:        "invite-1",
			GroupID:   "group-1",
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		}, invite)
	})

	t.Run("unknown code is reported as invalid", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupInviteQuery).
			WithArgs("unknown").
			WillReturnError(sql.ErrNoRows)

		invite, err := repo.GetGroupInvite(context.Background(), "unknown")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, domain.ErrInviteInvalid)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectGroupInviteQuery).
			WithArgs("hash").
			WillReturnError(dbErr)

		invite, err := repo.GetGroupInvite(context.Background(), "hash")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get group invite")
	})
}

func TestGroupRepo_ConsumeGroupInvite(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(consumeGroupInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.ConsumeGroupInvite(context.Background(), "invite-1", "user-2"))
	})

	t.Run("no row updated means invite is invalid", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(consumeGroupInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.ConsumeGroupInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, domain.ErrInviteInvalid)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(consumeGroupInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnError(dbErr)

		err := repo.ConsumeGroupInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to consume group invite")
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(consumeGroupInviteQuery).
			WithArgs("user-2", "invite-1").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.ConsumeGroupInvite(context.Background(), "invite-1", "user-2")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})
}

func TestGroupRepo_AddUserToGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(insertGroupMemberQuery).
			WithArgs("group-1", "user-2", domain.RoleMember).
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.AddUserToGroup(context.Background(), "group-1", "user-2", domain.RoleMember))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectExec(insertGroupMemberQuery).
			WithArgs("group-1", "user-2", domain.RoleMember).
			WillReturnError(dbErr)

		err := repo.AddUserToGroup(context.Background(), "group-1", "user-2", domain.RoleMember)

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to add user to group")
	})
}

func TestGroupRepo_RemoveUserFromGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(deleteGroupMemberQuery).
			WithArgs("group-1", "user-2").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.RemoveUserFromGroup(context.Background(), "group-1", "user-2"))
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(deleteGroupMemberQuery).
			WithArgs("group-1", "user-2").
			WillReturnError(dbErr)

		err := repo.RemoveUserFromGroup(context.Background(), "group-1", "user-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to remove user from group")
	})
}

func TestGroupRepo_GetGroupMembers(t *testing.T) {
	joinedAt := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)

	t.Run("maps rows in query order", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupMembersQuery).
			WithArgs("group-1").
			WillReturnRows(
				sqlmock.NewRows(groupMemberColumns).
					AddRow("user-1", "owner@example.com", "Owner", domain.RoleOwner, joinedAt).
					AddRow("user-2", "viewer@example.com", "Viewer", domain.RoleViewer, joinedAt),
			)

		members, err := repo.GetGroupMembers(context.Background(), "group-1")

		require.NoError(t, err)
		assert.Equal(t, []domain.GroupMember{
			{ID: "user-1", Email: "owner@example.com", Name: "Owner", Role: domain.RoleOwner, CreatedAt: joinedAt},
			{ID: "user-2", Email: "viewer@example.com", Name: "Viewer", Role: domain.RoleViewer, CreatedAt: joinedAt},
		}, members)
	})

	t.Run("no rows returns no members", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupMembersQuery).
			WithArgs("group-1").
			WillReturnRows(sqlmock.NewRows(groupMemberColumns))

		members, err := repo.GetGroupMembers(context.Background(), "group-1")

		require.NoError(t, err)
		assert.Empty(t, members)
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectGroupMembersQuery).
			WithArgs("group-1").
			WillReturnError(dbErr)

		members, err := repo.GetGroupMembers(context.Background(), "group-1")

		assert.Nil(t, members)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get group members")
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectGroupMembersQuery).
			WithArgs("group-1").
			WillReturnRows(
				sqlmock.NewRows(groupMemberColumns).
					AddRow("user-1", "owner@example.com", "Owner", domain.RoleOwner, "not a time"),
			)

		members, err := repo.GetGroupMembers(context.Background(), "group-1")

		assert.Nil(t, members)
		assert.Error(t, err)
	})

	t.Run("row iteration error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		rowErr := errors.New("connection lost mid-result")
		mock.ExpectQuery(selectGroupMembersQuery).
			WithArgs("group-1").
			WillReturnRows(
				sqlmock.NewRows(groupMemberColumns).
					AddRow("user-1", "owner@example.com", "Owner", domain.RoleOwner, joinedAt).
					AddRow("user-2", "viewer@example.com", "Viewer", domain.RoleViewer, joinedAt).
					RowError(1, rowErr),
			)

		members, err := repo.GetGroupMembers(context.Background(), "group-1")

		assert.Nil(t, members)
		assert.ErrorIs(t, err, rowErr)
		assert.ErrorContains(t, err, "failed to get group members")
	})
}

func TestGroupRepo_IsUserInGroup(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(isUserInGroupQuery).
			WithArgs("group-1", "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		ok, err := repo.IsUserInGroup(context.Background(), "group-1", "user-1")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not a member", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(isUserInGroupQuery).
			WithArgs("group-1", "user-2").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		ok, err := repo.IsUserInGroup(context.Background(), "group-1", "user-2")

		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("invalid input syntax for type uuid")
		mock.ExpectQuery(isUserInGroupQuery).
			WithArgs("group-1", "user-1").
			WillReturnError(dbErr)

		ok, err := repo.IsUserInGroup(context.Background(), "group-1", "user-1")

		assert.False(t, ok)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to check if user is in group")
	})
}

func TestGroupRepo_GetRoleForGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectRoleQuery).
			WithArgs("group-1", "user-1").
			WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(domain.RoleOwner))

		role, err := repo.GetRoleForGroup(context.Background(), "group-1", "user-1")

		require.NoError(t, err)
		assert.Equal(t, domain.RoleOwner, role)
	})

	t.Run("no membership is reported as not in group", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectRoleQuery).
			WithArgs("group-1", "user-2").
			WillReturnError(sql.ErrNoRows)

		role, err := repo.GetRoleForGroup(context.Background(), "group-1", "user-2")

		assert.Empty(t, role)
		assert.ErrorIs(t, err, domain.ErrUserNotInGroup)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectRoleQuery).
			WithArgs("group-1", "user-1").
			WillReturnError(dbErr)

		role, err := repo.GetRoleForGroup(context.Background(), "group-1", "user-1")

		assert.Empty(t, role)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get users role")
	})
}

func TestGroupRepo_GetDefaultGroupID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow("group-1"))

		id, err := repo.GetDefaultGroupID(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, "group-1", id)
	})

	t.Run("no default group stays matchable via errors.Is", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectQuery(selectDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnError(sql.ErrNoRows)

		id, err := repo.GetDefaultGroupID(context.Background(), "user-1")

		assert.Empty(t, id)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorContains(t, err, "failed to get default group id")
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectQuery(selectDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		id, err := repo.GetDefaultGroupID(context.Background(), "user-1")

		assert.Empty(t, id)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to get default group id")
	})
}

func TestGroupRepo_SetDefaultGroupID(t *testing.T) {
	t.Run("resets the old default before setting the new one", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetDefaultGroupID(context.Background(), "user-1", "group-2"))
	})

	t.Run("reset error skips setting the new default", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("connection reset")
		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnError(dbErr)

		err := repo.SetDefaultGroupID(context.Background(), "user-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to reset current default group_id")
	})

	t.Run("set error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		dbErr := errors.New("duplicate key value violates unique constraint")
		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-2").
			WillReturnError(dbErr)

		err := repo.SetDefaultGroupID(context.Background(), "user-1", "group-2")

		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "failed to set default group_id")
	})

	t.Run("no row updated means user is not in group", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-other").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.SetDefaultGroupID(context.Background(), "user-1", "group-other")

		assert.ErrorIs(t, err, domain.ErrUserNotInGroup)
	})

	t.Run("rows affected error is wrapped", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		resErr := errors.New("rows affected not supported")
		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-2").
			WillReturnResult(sqlmock.NewErrorResult(resErr))

		err := repo.SetDefaultGroupID(context.Background(), "user-1", "group-2")

		assert.ErrorIs(t, err, resErr)
		assert.ErrorContains(t, err, "could not get rows affected")
	})

	t.Run("not in group rolls back the reset within a transaction", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectBegin()
		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-other").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		err := repo.WithinTx(context.Background(), func(ctx context.Context) error {
			return repo.SetDefaultGroupID(ctx, "user-1", "group-other")
		})

		assert.ErrorIs(t, err, domain.ErrUserNotInGroup)
	})

	t.Run("both statements use the transaction from the context", func(t *testing.T) {
		repo, mock := newGroupRepo(t)

		mock.ExpectBegin()
		mock.ExpectExec(resetDefaultGroupQuery).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(setDefaultGroupQuery).
			WithArgs("user-1", "group-2").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := repo.WithinTx(context.Background(), func(ctx context.Context) error {
			return repo.SetDefaultGroupID(ctx, "user-1", "group-2")
		})

		require.NoError(t, err)
	})
}
