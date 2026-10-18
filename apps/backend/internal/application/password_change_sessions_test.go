package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A password change records its instant on the user (users.password_changed_at),
// and every session signed in before it is refused from then on: the auth
// middleware and the refresh route compare it with each token, and a device
// code approved before it mints nothing.

// The authenticated change and the forced change both reach
// AuthService.ChangePassword, which records the change with the new hash.
func TestAuthService_ChangePassword_RecordsTheChange(t *testing.T) {
	users := new(MockUserRepository)
	service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
	user := createTestUser("change@example.com")
	user.ForcePasswordChange = true
	var written *domain.User
	users.On("GetByID", user.ID).Return(user, nil)
	users.On("Update", mock.MatchedBy(func(u *domain.User) bool { written = u; return true })).Return(nil)

	before := time.Now()
	require.NoError(t, service.ChangePassword(context.Background(), user.ID, "SecurePass123!", "NewSecurePass123!"))
	after := time.Now()

	require.NotNil(t, written)
	require.NotNil(t, written.PasswordChangedAt, "the change is recorded")
	assert.False(t, written.PasswordChangedAt.Before(before.Truncate(time.Microsecond)))
	assert.False(t, written.PasswordChangedAt.After(after))
	assert.Equal(t, written.PasswordChangedAt.Truncate(time.Microsecond), *written.PasswordChangedAt,
		"the instant carries no more than the column's microseconds")
	assert.False(t, written.ForcePasswordChange)
	users.AssertExpectations(t)
}

// A refused change records nothing.
func TestAuthService_ChangePassword_WrongCurrentPasswordRecordsNothing(t *testing.T) {
	users := new(MockUserRepository)
	service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
	user := createTestUser("change@example.com")
	users.On("GetByID", user.ID).Return(user, nil)

	require.Error(t, service.ChangePassword(context.Background(), user.ID, "WrongPassword123!", "NewSecurePass123!"))
	assert.Nil(t, user.PasswordChangedAt)
	users.AssertNotCalled(t, "Update", mock.Anything)
}

// A reset with the mailed token is a password change too.
func TestRegistrationService_ResetPassword_RecordsTheChange(t *testing.T) {
	svc, repo, mail := storageResetService(t)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	token := mailedResetToken(t, mail)
	assert.Nil(t, repo.user.PasswordChangedAt, "requesting a reset changes nothing")

	before := time.Now()
	require.NoError(t, svc.ResetPassword(context.Background(), token, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"))
	require.NotNil(t, repo.user.PasswordChangedAt, "the reset is recorded")
	assert.False(t, repo.user.PasswordChangedAt.Before(before.Truncate(time.Microsecond)))
}

// A device code approved before a password change mints no pair: the
// approving session was signed in before the change.
func TestDeviceAuthService_PollToken_PasswordChangedAfterApproval_ReceivesNoPair(t *testing.T) {
	svc, user, deviceCode := approvedDeviceGrant(t)
	changedAt := time.Now()
	user.PasswordChangedAt = &changedAt
	tok, err := svc.PollToken(context.Background(), deviceCode)
	assert.ErrorIs(t, err, ErrAccessDenied)
	assert.True(t, tok == nil, "no pair may be minted")
}

func TestDeviceAuthService_PollToken_ApprovalTimeAgainstThePasswordChange(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	changedAt := time.Date(2026, 10, 18, 12, 0, 0, 400_000_000, time.UTC)
	cases := []struct {
		name       string
		approvedAt time.Time
		minted     bool
	}{
		{"approved before the change", changedAt.Add(-time.Minute), false},
		{"approved in the second of the change", changedAt.Add(300 * time.Millisecond), false},
		{"approved at the change's boundary", auth.PasswordChangeBoundary(changedAt), true},
		{"approved after it", changedAt.Add(time.Minute), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, user, deviceCode, codes := approvedDeviceGrantWithCodes(t)
			at := changedAt
			user.PasswordChangedAt = &at
			approvedAt := tc.approvedAt
			codes.mu.Lock()
			codes.codes[deviceCode].ApprovedAt = &approvedAt
			codes.mu.Unlock()

			tok, err := svc.PollToken(context.Background(), deviceCode)
			if tc.minted {
				require.NoError(t, err)
				assert.NotEmpty(t, tok.AccessToken)
				return
			}
			assert.ErrorIs(t, err, ErrAccessDenied)
			assert.True(t, tok == nil, "no pair may be minted")
		})
	}
}

// approvedDeviceGrantWithCodes is approvedDeviceGrant that also returns the
// code store, so a test can set the approval's time.
func approvedDeviceGrantWithCodes(t *testing.T) (*DeviceAuthService, *domain.User, string, *deviceCodeMapRepo) {
	t.Helper()
	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "dev@example.com",
		Role:           domain.RoleAdmin,
		Status:         domain.UserStatusActive,
	}
	codes := &deviceCodeMapRepo{codes: map[string]*domain.DeviceCode{}}
	authSvc := NewAuthService(deviceUserStub{user: user}, nil, nil, nil, nil, nil)
	svc := NewDeviceAuthService(codes, auth.NewJWTService(), authSvc, "http://localhost:3000")
	ctx := context.Background()
	start, err := svc.InitiateDeviceAuth(ctx, "aim-sdk", "", "127.0.0.1", "aim-sdk/test")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveDeviceCode(ctx, start.UserCode, user.ID, user.OrganizationID))
	return svc, user, start.DeviceCode, codes
}
