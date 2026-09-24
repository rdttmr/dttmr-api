package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

type GroupRepo struct {
	Repo
}

func (r *GroupRepo) CreateGroup(ctx context.Context, name string, createdBy string) (*domain.Group, error) {
	var group domain.Group

	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO groups (name, created_by) VALUES ($1, $2) RETURNING id, created_at, modified_at",
		name, createdBy,
	).Scan(&group.ID, &group.CreatedAt, &group.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	group.Name = name
	return &group, nil
}

func (r *GroupRepo) DeleteGroup(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM groups WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	return nil
}

func (r *GroupRepo) SetGroupName(ctx context.Context, id string, name string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "UPDATE groups SET name = $1, modified_at = NOW() WHERE id = $2", name, id)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}
	return nil
}

func (r *GroupRepo) GetGroups(ctx context.Context, userID string) ([]domain.Group, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT g.id, g.name, g.created_at, g.modified_at FROM groups AS g INNER JOIN group_members gm ON g.id=gm.group_id WHERE gm.user_id = $1",
		userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}
	defer rows.Close()

	groups := make([]domain.Group, 0, 8)
	for rows.Next() {
		var g domain.Group
		err = rows.Scan(&g.ID, &g.Name, &g.CreatedAt, &g.ModifiedAt)
		if err != nil {
			return nil, err
		}

		groups = append(groups, g)
	}

	return groups, nil
}

// CreateGroupInvite does *not* return the invite code in the domain.GroupInvite object
func (r *GroupRepo) CreateGroupInvite(ctx context.Context, groupID string, codeHash string, expiresAt time.Time, createdBy string) (*domain.GroupInvite, error) {
	var invite domain.GroupInvite
	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO group_invites (group_id, code_hash, expires_at, created_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at",
		groupID, codeHash, expiresAt, createdBy,
	).Scan(&invite.ID, &invite.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create group invite: %w", err)
	}

	invite.GroupID = groupID
	invite.ExpiresAt = expiresAt
	return &invite, nil
}

func (r *GroupRepo) DeleteGroupInvite(ctx context.Context, inviteID string) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM group_invites WHERE id = $1", inviteID)
	if err != nil {
		return fmt.Errorf("failed to delete group invite: %w", err)
	}
	return nil
}

func (r *GroupRepo) GetGroupInvite(ctx context.Context, codeHash string) (*domain.GroupInvite, error) {
	var invite domain.GroupInvite
	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT id, group_id, created_at, expires_at FROM group_invites WHERE code_hash = $1",
		codeHash,
	).Scan(&invite.ID, &invite.GroupID, &invite.CreatedAt, &invite.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get group invite: %w", err)
	}

	return &invite, nil
}

func (r *GroupRepo) ConsumeGroupInvite(ctx context.Context, id string, usedBy string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE group_invites SET used_by = $1, consumed_at = NOW() WHERE id = $2",
		usedBy, id,
	)
	if err != nil {
		return fmt.Errorf("failed to consume group invite: %w", err)
	}
	return nil
}

func (r *GroupRepo) AddUserToGroup(ctx context.Context, groupID string, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)",
		groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}
	return nil
}

func (r *GroupRepo) RemoveUserFromGroup(ctx context.Context, groupID string, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"DELETE FROM group_members WHERE group_id = $1 AND user_id = $2",
		groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}
	return nil
}

func (r *GroupRepo) IsUserInGroup(ctx context.Context, groupID string, userID string) (bool, error) {
	var cnt int

	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = $1 AND user_id = $2",
		groupID, userID,
	).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is in group: %w", err)
	}

	return cnt > 0, nil
}

func (r *GroupRepo) GetRoleForGroup(ctx context.Context, groupID string, userID string) (string, error) {
	var role string
	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2",
		groupID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrUserNotInGroup
		}
		return "", fmt.Errorf("failed to get users role: %w", err)
	}

	return role, nil
}

func (r *GroupRepo) GetDefaultGroupID(ctx context.Context, userID string) (string, error) {
	var id string

	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT group_id FROM group_members WHERE user_id = $1 AND is_default = TRUE",
		userID,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to get default group id: %w", err)
	}

	return id, nil
}

// SetDefaultGroupID should always run within a transaction
func (r *GroupRepo) SetDefaultGroupID(ctx context.Context, userID string, groupID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE group_members SET is_default = FALSE WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to reset current default group_id: %w", err)
	}

	_, err = r.conn(ctx).ExecContext(ctx,
		"UPDATE group_members SET is_default = TRUE WHERE user_id = $1 AND group_id = $2",
		userID, groupID,
	)
	if err != nil {
		return fmt.Errorf("failed to set default group_id: %w", err)
	}

	return nil
}
