package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A password change or reset marks the user's sdk_tokens rows that are not yet
// revoked as revoked, with the reason password_changed, once the change is
// written, so the dashboard's list of SDK tokens agrees with the server: the
// change refuses every token minted before it. A change that is refused or not
// written revokes nothing, and a revocation that fails leaves the written
// change standing.

// sdkRowRevocations records each RevokeAllForUser call as "<user> <reason>".
// onRevoke, when set, runs at the start of each call.
type sdkRowRevocations struct {
	domain.SDKTokenRepository
	calls    []string
	err      error
	onRevoke func()
}

func (r *sdkRowRevocations) RevokeAllForUser(userID uuid.UUID, reason string) error {
	if r.onRevoke != nil {
		r.onRevoke()
	}
	r.calls = append(r.calls, userID.String()+" "+reason)
	return r.err
}

func TestAuthService_ChangePassword_RevokesTheUsersSDKTokenRows(t *testing.T) {
	users := new(MockUserRepository)
	service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
	user := createTestUser("change@example.com")
	written := false
	rows := &sdkRowRevocations{onRevoke: func() {
		assert.True(t, written, "the rows are revoked after the change is written")
	}}
	service.SetSDKTokenRepository(rows)
	users.On("GetByID", user.ID).Return(user, nil)
	users.On("Update", mock.Anything).Run(func(mock.Arguments) { written = true }).Return(nil)

	require.NoError(t, service.ChangePassword(context.Background(), user.ID, "SecurePass123!", "NewSecurePass123!"))

	assert.Equal(t, []string{user.ID.String() + " " + domain.SDKTokenRevokeReasonPasswordChanged}, rows.calls,
		"one revocation of the user's rows, with the reason password_changed")
	assert.Equal(t, "password_changed", domain.SDKTokenRevokeReasonPasswordChanged)
}

func TestAuthService_ChangePassword_RevokesNoRowWhenTheChangeIsNotWritten(t *testing.T) {
	t.Run("the current password is wrong", func(t *testing.T) {
		users := new(MockUserRepository)
		service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
		rows := &sdkRowRevocations{}
		service.SetSDKTokenRepository(rows)
		user := createTestUser("change@example.com")
		users.On("GetByID", user.ID).Return(user, nil)

		require.ErrorIs(t, service.ChangePassword(context.Background(), user.ID, "WrongPassword123!", "NewSecurePass123!"), ErrCurrentPasswordIncorrect)
		assert.Empty(t, rows.calls)
	})

	t.Run("the user row is not written", func(t *testing.T) {
		users := new(MockUserRepository)
		service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
		rows := &sdkRowRevocations{}
		service.SetSDKTokenRepository(rows)
		user := createTestUser("change@example.com")
		users.On("GetByID", user.ID).Return(user, nil)
		users.On("Update", mock.Anything).Return(errors.New("connection refused"))

		require.Error(t, service.ChangePassword(context.Background(), user.ID, "SecurePass123!", "NewSecurePass123!"))
		assert.Empty(t, rows.calls)
	})
}

// failedRevocationLine is the line a failed revocation logs (captureLog drops
// the logger's flags).
func failedRevocationLine(userID uuid.UUID) string {
	return "Password change: sdk_tokens rows of user " + userID.String() + " not revoked: connection refused"
}

func TestAuthService_ChangePassword_AFailedRevocationLeavesTheChangeStanding(t *testing.T) {
	users := new(MockUserRepository)
	service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
	rows := &sdkRowRevocations{err: errors.New("connection refused")}
	service.SetSDKTokenRepository(rows)
	user := createTestUser("change@example.com")
	var written *domain.User
	users.On("GetByID", user.ID).Return(user, nil)
	users.On("Update", mock.MatchedBy(func(u *domain.User) bool { written = u; return true })).Return(nil)
	logged := captureLog(t)

	require.NoError(t, service.ChangePassword(context.Background(), user.ID, "SecurePass123!", "NewSecurePass123!"),
		"the change was written, so it is answered as made")
	require.NotNil(t, written)
	assert.NotNil(t, written.PasswordChangedAt, "the change is recorded")
	assert.Len(t, rows.calls, 1)
	assert.Contains(t, logged.String(), failedRevocationLine(user.ID), "the failure is logged")
}

// resetWrites is storageUserRepo counting its writes. With failReset set, the
// write of the reset itself, the one that clears the reset token, fails.
type resetWrites struct {
	*storageUserRepo
	writes    int
	failReset bool
}

func (r *resetWrites) Update(u *domain.User) error {
	if r.failReset && u.PasswordResetToken == nil {
		return errors.New("connection refused")
	}
	r.writes++
	return r.storageUserRepo.Update(u)
}

// resetWithRows is storageResetService over resetWrites, with rows attached
// as the service's sdk_tokens rows.
func resetWithRows(t *testing.T, failReset bool, rows *sdkRowRevocations) (*RegistrationService, *resetWrites, *privacyEmailService) {
	t.Helper()
	_, storage, mail := storageResetService(t)
	repo := &resetWrites{storageUserRepo: storage, failReset: failReset}
	svc := NewRegistrationService(nil, repo, nil, NewAuditService(discardAuditRepo{}), mail)
	svc.SetSDKTokenRepository(rows)
	return svc, repo, mail
}

func TestRegistrationService_ResetPassword_RevokesTheUsersSDKTokenRows(t *testing.T) {
	rows := &sdkRowRevocations{}
	svc, repo, mail := resetWithRows(t, false, rows)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	assert.Empty(t, rows.calls, "requesting a reset revokes nothing")
	token := mailedResetToken(t, mail)
	writesBefore := repo.writes
	rows.onRevoke = func() {
		assert.Equal(t, writesBefore+1, repo.writes, "the rows are revoked after the reset is written")
	}

	require.NoError(t, svc.ResetPassword(context.Background(), token, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"))

	assert.Equal(t, []string{repo.user.ID.String() + " " + domain.SDKTokenRevokeReasonPasswordChanged}, rows.calls,
		"one revocation of the user's rows, with the reason password_changed")
}

func TestRegistrationService_ResetPassword_RevokesNoRowWhenTheResetIsNotWritten(t *testing.T) {
	t.Run("the reset token is wrong", func(t *testing.T) {
		rows := &sdkRowRevocations{}
		svc, _, _ := resetWithRows(t, false, rows)
		require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))

		require.ErrorIs(t, svc.ResetPassword(context.Background(), "not-the-mailed-token", "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"), ErrResetTokenInvalid)
		assert.Empty(t, rows.calls)
	})

	t.Run("the user row is not written", func(t *testing.T) {
		rows := &sdkRowRevocations{}
		svc, _, mail := resetWithRows(t, true, rows)
		require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
		token := mailedResetToken(t, mail)

		require.Error(t, svc.ResetPassword(context.Background(), token, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"))
		assert.Empty(t, rows.calls)
	})
}

func TestRegistrationService_ResetPassword_AFailedRevocationLeavesTheResetStanding(t *testing.T) {
	rows := &sdkRowRevocations{err: errors.New("connection refused")}
	svc, repo, mail := resetWithRows(t, false, rows)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	token := mailedResetToken(t, mail)
	logged := captureLog(t)

	require.NoError(t, svc.ResetPassword(context.Background(), token, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"),
		"the reset was written, so it is answered as made")
	assert.NotNil(t, repo.user.PasswordChangedAt, "the reset is recorded")
	assert.Len(t, rows.calls, 1)
	assert.Contains(t, logged.String(), failedRevocationLine(repo.user.ID), "the failure is logged")
}
