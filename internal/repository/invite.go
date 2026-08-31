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
	db *sql.DB
}

func NewInviteRepo(db *sql.DB) *InviteRepo {
	return &InviteRepo{db: db}
}

func (r *InviteRepo) CreateInvite(ctx context.Context, inviterUserID string, code string, expiresAt time.Time) (*domain.Invite, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
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
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM invites WHERE id = $1 AND invitee_user_id = $2 AND consumed_at IS NULL",
		inviteID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}

	return nil
}

func (r *InviteRepo) ConsumeInvite(ctx context.Context, inviteID string, inviteeUserID string) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE invites SET invitee_user_id=$1, consumed_at=NOW() WHERE id=$2 AND expires_at < NOW() AND consumed_at IS NULL",
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
		return fmt.Errorf("invite not found or expired")
	}

	return nil
}

func (r *InviteRepo) GetInvite(ctx context.Context, code string) (*domain.Invite, error) {
	var invite domain.Invite

	err := r.db.QueryRowContext(ctx,
		"SELECT id, code, expires_at, consumed_at FROM invites WHERE code=$1",
		code,
	).Scan(&invite.ID, &invite.Code, &invite.ExpiresAt, &invite.ConsumedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

func (r *InviteRepo) GetInvites(ctx context.Context, userID string) ([]domain.Invite, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, code, expires_at, consumed_at FROM invites WHERE invitee_user_id=$1", userID,
		userID,
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
