package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/robindittmar/dttmr-api/internal/domain"
)

type InviteRepo struct {
	Repo
}

func (r *InviteRepo) CreateInvite(ctx context.Context, inviterUserID string, code string, expiresAt time.Time) (*domain.Invite, error) {
	var id string
	err := r.conn(ctx).QueryRowContext(ctx,
		"INSERT INTO invites (inviter_user_id, code, expires_at) VALUES ($1, $2, $3) RETURNING id",
		inviterUserID, code, expiresAt,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to insert invites: %w", err)
	}

	return &domain.Invite{
		ID:         id,
		Code:       code,
		ExpiresAt:  expiresAt,
		ConsumedAt: nil,
	}, nil
}

func (r *InviteRepo) DeleteInvite(ctx context.Context, userID string, inviteID string) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"DELETE FROM invites WHERE id = $1 AND inviter_user_id = $2 AND consumed_at IS NULL",
		inviteID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: %w", err)
	}
	if affected < 1 {
		// It's more of an assumption,
		// but unless I encounter this being wrong, I'll keep it.
		return domain.ErrInviteConsumed
	}

	return nil
}

func (r *InviteRepo) ConsumeInvite(ctx context.Context, inviteID string, inviteeUserID string) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE invites SET invitee_user_id=$1, consumed_at=NOW() WHERE id=$2 AND expires_at > NOW() AND consumed_at IS NULL",
		inviteeUserID, inviteID,
	)
	if err != nil {
		return fmt.Errorf("failed to update invite: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: %w", err)
	}
	if affected < 1 {
		return domain.ErrInviteInvalid
	}

	return nil
}

func (r *InviteRepo) GetInvite(ctx context.Context, code string) (*domain.Invite, error) {
	var invite domain.Invite

	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT id, code, expires_at, consumed_at FROM invites WHERE code=$1",
		code,
	).Scan(&invite.ID, &invite.Code, &invite.ExpiresAt, &invite.ConsumedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInviteInvalid
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

func (r *InviteRepo) GetInvites(ctx context.Context, userID string, offset int, count int) ([]domain.Invite, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		"SELECT id, code, expires_at, consumed_at FROM invites WHERE inviter_user_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3",
		userID, offset, count,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get invites: %w", err)
	}
	defer rows.Close()

	var invites []domain.Invite
	for rows.Next() {
		var i domain.Invite
		err = rows.Scan(&i.ID, &i.Code, &i.ExpiresAt, &i.ConsumedAt)
		if err != nil {
			return nil, err
		}

		invites = append(invites, i)
	}

	return invites, nil
}

func (r *InviteRepo) CountInvites(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.conn(ctx).QueryRowContext(ctx,
		"SELECT COUNT(*) FROM invites WHERE inviter_user_id=$1",
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count invites: %w", err)
	}

	return count, nil
}

func (r *InviteRepo) CountInvitesStructured(ctx context.Context, userID string) (*domain.InviteCounts, error) {
	var counts domain.InviteCounts
	conn := r.conn(ctx)
	err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM invites WHERE inviter_user_id=$1 AND expires_at > NOW() AND consumed_at IS NULL",
		userID,
	).Scan(&counts.Active)
	if err != nil {
		return nil, fmt.Errorf("failed to count active invites: %w", err)
	}

	err = conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM invites WHERE inviter_user_id=$1 AND expires_at < NOW() AND consumed_at IS NULL",
		userID,
	).Scan(&counts.Expired)
	if err != nil {
		return nil, fmt.Errorf("failed to count expired invites: %w", err)
	}

	err = conn.QueryRowContext(ctx,
		"SELECT COUNT(consumed_at) FROM invites WHERE inviter_user_id=$1",
		userID,
	).Scan(&counts.Used)
	if err != nil {
		return nil, fmt.Errorf("failed to count consumed invites: %w", err)
	}

	return &counts, nil
}
