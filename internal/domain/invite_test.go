package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var _ InviteRepository = (*mockInviteRepository)(nil)

type mockInviteRepository struct {
	mock.Mock
}

func (m *mockInviteRepository) CreateInvite(ctx context.Context, inviterUserID string, code string, expiresAt time.Time) (*Invite, error) {
	args := m.Called(ctx, inviterUserID, code, expiresAt)
	invite, _ := args.Get(0).(*Invite)
	return invite, args.Error(1)
}

func (m *mockInviteRepository) DeleteInvite(ctx context.Context, userID string, inviteID string) error {
	args := m.Called(ctx, userID, inviteID)
	return args.Error(0)
}

func (m *mockInviteRepository) ConsumeInvite(ctx context.Context, inviteID string, inviteeUserID string) error {
	args := m.Called(ctx, inviteID, inviteeUserID)
	return args.Error(0)
}

func (m *mockInviteRepository) GetInvite(ctx context.Context, code string) (*Invite, error) {
	args := m.Called(ctx, code)
	invite, _ := args.Get(0).(*Invite)
	return invite, args.Error(1)
}

func (m *mockInviteRepository) GetInvites(ctx context.Context, userID string, offset int, count int) ([]Invite, error) {
	args := m.Called(ctx, userID, offset, count)
	invites, _ := args.Get(0).([]Invite)
	return invites, args.Error(1)
}

func (m *mockInviteRepository) CountInvites(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockInviteRepository) CountInvitesStructured(ctx context.Context, userID string) (*InviteCounts, error) {
	args := m.Called(ctx, userID)
	counts, _ := args.Get(0).(*InviteCounts)
	return counts, args.Error(1)
}

func newInviteService(t *testing.T) (*InviteService, *mockInviteRepository) {
	t.Helper()

	repo := &mockInviteRepository{}
	repo.Test(t)
	t.Cleanup(func() { repo.AssertExpectations(t) })

	return NewInviteService(repo), repo
}

func TestInviteService_CreateInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("returns a random code and stores only its hash, expiring in 7 days", func(t *testing.T) {
		svc, repo := newInviteService(t)

		created := &Invite{ID: "invite-1"}
		repo.On("CreateInvite", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(created, nil)

		before := time.Now()
		invite, err := svc.CreateInvite(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, "invite-1", invite.ID)

		assert.Len(t, invite.Code, 64)
		_, hexErr := hex.DecodeString(invite.Code)
		assert.NoError(t, hexErr, "code is not hex: %q", invite.Code)

		storedCode := repo.Calls[0].Arguments.String(2)
		assert.Equal(t, hashToken(invite.Code), storedCode)
		assert.NotEqual(t, invite.Code, storedCode)

		expiresAt := repo.Calls[0].Arguments.Get(3).(time.Time)
		assert.WithinDuration(t, before.Add(7*24*time.Hour), expiresAt, 5*time.Second)
	})

	t.Run("each invite gets a different code", func(t *testing.T) {
		svc, repo := newInviteService(t)

		// Separate invites per call: the service writes the code onto the returned invite.
		repo.On("CreateInvite", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(&Invite{ID: "invite-1"}, nil).Once()
		repo.On("CreateInvite", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(&Invite{ID: "invite-2"}, nil).Once()

		first, err := svc.CreateInvite(ctx, "user-1")
		require.NoError(t, err)
		second, err := svc.CreateInvite(ctx, "user-1")
		require.NoError(t, err)

		assert.NotEqual(t, first.Code, second.Code)
		assert.NotEqual(t, repo.Calls[0].Arguments.String(2), repo.Calls[1].Arguments.String(2))
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repoErr := errors.New("connection reset")
		repo.On("CreateInvite", mock.Anything, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).
			Return(nil, repoErr)

		invite, err := svc.CreateInvite(ctx, "user-1")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		invite, err := svc.CreateInvite(ctx, "")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, ErrUserIDMissing)
	})
}

func TestInviteService_DeleteInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the invite of the user", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("DeleteInvite", mock.Anything, "user-1", "invite-1").Return(nil)

		assert.NoError(t, svc.DeleteInvite(ctx, "user-1", "invite-1"))
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("DeleteInvite", mock.Anything, "user-1", "invite-1").Return(ErrInviteConsumed)

		assert.ErrorIs(t, svc.DeleteInvite(ctx, "user-1", "invite-1"), ErrInviteConsumed)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		assert.ErrorIs(t, svc.DeleteInvite(ctx, "", "invite-1"), ErrUserIDMissing)
	})

	t.Run("without invite id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		assert.ErrorIs(t, svc.DeleteInvite(ctx, "user-1", ""), ErrInviteIDMissing)
	})
}

func TestInviteService_ConsumeInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("consumes the invite for the invitee", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("ConsumeInvite", mock.Anything, "invite-1", "user-2").Return(nil)

		assert.NoError(t, svc.ConsumeInvite(ctx, "invite-1", "user-2"))
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("ConsumeInvite", mock.Anything, "invite-1", "user-2").Return(ErrInviteInvalid)

		assert.ErrorIs(t, svc.ConsumeInvite(ctx, "invite-1", "user-2"), ErrInviteInvalid)
	})

	t.Run("without invite id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		assert.ErrorIs(t, svc.ConsumeInvite(ctx, "", "user-2"), ErrInviteIDMissing)
	})

	t.Run("without invitee user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		assert.ErrorIs(t, svc.ConsumeInvite(ctx, "invite-1", ""), ErrUserIDMissing)
	})
}

func TestInviteService_GetInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("returns an open invite", func(t *testing.T) {
		svc, repo := newInviteService(t)

		open := &Invite{ID: "invite-1", Code: "CODE", ExpiresAt: time.Now().Add(time.Hour)}
		repo.On("GetInvite", mock.Anything, hashToken("CODE")).Return(open, nil)

		invite, err := svc.GetInvite(ctx, "CODE")

		require.NoError(t, err)
		assert.Same(t, open, invite)
	})

	rejected := []struct {
		name    string
		invite  *Invite
		wantErr error
	}{
		{
			name:    "expired invite",
			invite:  &Invite{ID: "invite-1", ExpiresAt: time.Now().Add(-time.Hour)},
			wantErr: ErrInviteExpired,
		},
		{
			name:    "consumed invite",
			invite:  &Invite{ID: "invite-1", ExpiresAt: time.Now().Add(time.Hour), ConsumedAt: new(time.Now())},
			wantErr: ErrInviteConsumed,
		},
		{
			name:    "expired and consumed invite reports expired",
			invite:  &Invite{ID: "invite-1", ExpiresAt: time.Now().Add(-time.Hour), ConsumedAt: new(time.Now().Add(-2 * time.Hour))},
			wantErr: ErrInviteExpired,
		},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newInviteService(t)

			repo.On("GetInvite", mock.Anything, hashToken("CODE")).Return(tt.invite, nil)

			invite, err := svc.GetInvite(ctx, "CODE")

			assert.Nil(t, invite)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("GetInvite", mock.Anything, hashToken("CODE")).Return(nil, ErrInviteInvalid)

		invite, err := svc.GetInvite(ctx, "CODE")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, ErrInviteInvalid)
	})

	t.Run("without code", func(t *testing.T) {
		svc, _ := newInviteService(t)

		invite, err := svc.GetInvite(ctx, "")

		assert.Nil(t, invite)
		assert.ErrorIs(t, err, ErrCodeMissing)
	})
}

func TestInviteService_GetInvites(t *testing.T) {
	ctx := context.Background()

	pages := []struct {
		page       int
		count      int
		wantOffset int
	}{
		{page: 1, count: 10, wantOffset: 0},
		{page: 2, count: 10, wantOffset: 10},
		{page: 3, count: 25, wantOffset: 50},
	}
	for _, tt := range pages {
		t.Run(fmt.Sprintf("page %d with %d per page starts at offset %d", tt.page, tt.count, tt.wantOffset), func(t *testing.T) {
			svc, repo := newInviteService(t)

			found := []Invite{{ID: "invite-1"}}
			repo.On("GetInvites", mock.Anything, "user-1", tt.wantOffset, tt.count).Return(found, nil)

			invites, err := svc.GetInvites(ctx, "user-1", tt.page, tt.count)

			require.NoError(t, err)
			assert.Equal(t, found, invites)
		})
	}

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repoErr := errors.New("connection reset")
		repo.On("GetInvites", mock.Anything, "user-1", 0, 10).Return(nil, repoErr)

		invites, err := svc.GetInvites(ctx, "user-1", 1, 10)

		assert.Nil(t, invites)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		invites, err := svc.GetInvites(ctx, "", 1, 10)

		assert.Nil(t, invites)
		assert.ErrorIs(t, err, ErrUserIDMissing)
	})
}

func TestInviteService_CountInvites(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the count", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repo.On("CountInvites", mock.Anything, "user-1").Return(7, nil)

		count, err := svc.CountInvites(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, 7, count)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repoErr := errors.New("connection reset")
		repo.On("CountInvites", mock.Anything, "user-1").Return(0, repoErr)

		count, err := svc.CountInvites(ctx, "user-1")

		assert.Zero(t, count)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		count, err := svc.CountInvites(ctx, "")

		assert.Zero(t, count)
		assert.ErrorIs(t, err, ErrUserIDMissing)
	})
}

func TestInviteService_CountInvitesStructured(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the counts", func(t *testing.T) {
		svc, repo := newInviteService(t)

		counts := &InviteCounts{Active: 3, Expired: 2, Used: 5}
		repo.On("CountInvitesStructured", mock.Anything, "user-1").Return(counts, nil)

		got, err := svc.CountInvitesStructured(ctx, "user-1")

		require.NoError(t, err)
		assert.Same(t, counts, got)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc, repo := newInviteService(t)

		repoErr := errors.New("connection reset")
		repo.On("CountInvitesStructured", mock.Anything, "user-1").Return(nil, repoErr)

		got, err := svc.CountInvitesStructured(ctx, "user-1")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("without user id", func(t *testing.T) {
		svc, _ := newInviteService(t)

		got, err := svc.CountInvitesStructured(ctx, "")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrUserIDMissing)
	})
}
