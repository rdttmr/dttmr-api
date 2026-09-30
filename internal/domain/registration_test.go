package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var notInTx = mock.MatchedBy(func(ctx context.Context) bool {
	v, _ := ctx.Value(txCtxKey{}).(bool)
	return !v
})

type registrationMocks struct {
	users   *mockUserRepository
	groups  *mockGroupRepository
	invites *mockInviteRepository
	tx      *fakeTransactor
}

// newRegistrationService wires a RegistrationService to real user, group and
// invite services backed by mock repositories. The group service gets its own
// transactor, so tx.calls only counts transactions started by Register.
func newRegistrationService(t *testing.T) (*RegistrationService, registrationMocks) {
	t.Helper()

	m := registrationMocks{
		users:   &mockUserRepository{},
		groups:  &mockGroupRepository{},
		invites: &mockInviteRepository{},
		tx:      &fakeTransactor{},
	}
	m.users.Test(t)
	m.groups.Test(t)
	m.invites.Test(t)
	t.Cleanup(func() {
		m.users.AssertExpectations(t)
		m.groups.AssertExpectations(t)
		m.invites.AssertExpectations(t)
	})

	svc := NewRegistrationService(
		m.tx,
		NewUserService(m.users),
		NewGroupService(&fakeTransactor{}, m.groups),
		NewInviteService(m.invites),
	)
	return svc, m
}

func openInvite() *Invite {
	return &Invite{ID: "invite-1", Code: "CODE", ExpiresAt: time.Now().Add(time.Hour)}
}

// expectRegistration sets up every repository call of a successful
// registration. Steps after failAt are not expected; failAt returns err.
func expectRegistration(m registrationMocks, failAt string, err error) {
	errFor := func(step string) error {
		if step == failAt {
			return err
		}
		return nil
	}

	m.invites.On("GetInvite", notInTx, hashToken("CODE")).Return(openInvite(), nil)

	if failAt == "CreateUser" {
		m.users.On("CreateUser", inTx, "robin@dittmar.dev", "Robin", mock.AnythingOfType("string")).Return(nil, err)
		return
	}
	m.users.On("CreateUser", inTx, "robin@dittmar.dev", "Robin", mock.AnythingOfType("string")).
		Return(&User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}, nil)

	if failAt == "CreateGroup" {
		m.groups.On("CreateGroup", inTx, "Personal", "user-1").Return(nil, err)
		return
	}
	m.groups.On("CreateGroup", inTx, "Personal", "user-1").Return(&Group{ID: "group-1", Name: "Personal"}, nil)
	m.groups.On("AddUserToGroup", inTx, "group-1", "user-1", RoleOwner).Return(nil)

	m.groups.On("IsUserInGroup", inTx, "group-1", "user-1").Return(true, nil)
	m.groups.On("SetDefaultGroupID", inTx, "user-1", "group-1").Return(errFor("SetDefaultGroupID"))
	if failAt == "SetDefaultGroupID" {
		return
	}

	m.invites.On("ConsumeInvite", inTx, "invite-1", "user-1").Return(errFor("ConsumeInvite"))
}

func TestRegistrationService_Register(t *testing.T) {
	t.Parallel() // CreateUser hashes with bcrypt at DefaultCost, which is slow with -race

	ctx := context.Background()

	t.Run("creates user, personal default group and consumes the invite in one transaction", func(t *testing.T) {
		t.Parallel()

		svc, m := newRegistrationService(t)
		expectRegistration(m, "", nil)

		user, err := svc.Register(ctx, "CODE", "robin@dittmar.dev", "Robin", "hunter22")

		require.NoError(t, err)
		assert.Equal(t, &User{ID: "user-1", Email: "robin@dittmar.dev", Name: "Robin"}, user)
		assert.Equal(t, 1, m.tx.calls)
		assert.Equal(t, []string{"CreateGroup", "AddUserToGroup", "IsUserInGroup", "SetDefaultGroupID"}, callOrder(m.groups.Calls))
		assert.Equal(t, []string{"GetInvite", "ConsumeInvite"}, callOrder(m.invites.Calls))
	})

	stepFailures := []string{"CreateUser", "CreateGroup", "SetDefaultGroupID", "ConsumeInvite"}
	for _, step := range stepFailures {
		t.Run(step+" error aborts the transaction and is returned", func(t *testing.T) {
			t.Parallel()

			svc, m := newRegistrationService(t)
			stepErr := errors.New(step + " failed")
			expectRegistration(m, step, stepErr)

			user, err := svc.Register(ctx, "CODE", "robin@dittmar.dev", "Robin", "hunter22")

			assert.Nil(t, user)
			assert.ErrorIs(t, err, stepErr)
			assert.Equal(t, 1, m.tx.calls)
		})
	}

	t.Run("invite consumed concurrently is returned as invalid", func(t *testing.T) {
		t.Parallel()

		svc, m := newRegistrationService(t)
		expectRegistration(m, "ConsumeInvite", ErrInviteInvalid)

		user, err := svc.Register(ctx, "CODE", "robin@dittmar.dev", "Robin", "hunter22")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrInviteInvalid)
	})

	inviteFailures := []struct {
		name    string
		code    string
		invite  *Invite
		repoErr error
		wantErr error
	}{
		{name: "without invite code", code: "", wantErr: ErrCodeMissing},
		{name: "unknown invite code", code: "CODE", repoErr: ErrInviteInvalid, wantErr: ErrInviteInvalid},
		{
			name:    "expired invite",
			code:    "CODE",
			invite:  &Invite{ID: "invite-1", Code: "CODE", ExpiresAt: time.Now().Add(-time.Hour)},
			wantErr: ErrInviteExpired,
		},
		{
			name:    "consumed invite",
			code:    "CODE",
			invite:  &Invite{ID: "invite-1", Code: "CODE", ExpiresAt: time.Now().Add(time.Hour), ConsumedAt: new(time.Now())},
			wantErr: ErrInviteConsumed,
		},
	}
	for _, tt := range inviteFailures {
		t.Run(tt.name+" is rejected before anything is created", func(t *testing.T) {
			t.Parallel()

			svc, m := newRegistrationService(t)
			if tt.code != "" {
				m.invites.On("GetInvite", notInTx, hashToken(tt.code)).Return(tt.invite, tt.repoErr)
			}

			user, err := svc.Register(ctx, tt.code, "robin@dittmar.dev", "Robin", "hunter22")

			assert.Nil(t, user)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, m.tx.calls)
		})
	}

	t.Run("invalid user input aborts before the group is created", func(t *testing.T) {
		t.Parallel()

		svc, m := newRegistrationService(t)
		m.invites.On("GetInvite", notInTx, hashToken("CODE")).Return(openInvite(), nil)

		user, err := svc.Register(ctx, "CODE", "robin@dittmar.dev", "Robin", "")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrPasswordMissing)
		assert.Equal(t, 1, m.tx.calls)
	})
}
