package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// lockedOutFailures reports an active lockout for every email.
type lockedOutFailures struct {
	domain.AuthFailureRepository
}

func (lockedOutFailures) GetActiveLockout(email string) (*domain.AuthLockout, error) {
	return &domain.AuthLockout{Email: email, LockedUntil: time.Now().Add(10 * time.Minute)}, nil
}

// Each check that refuses a password sign-in returns a *SignInRefusal that
// names it, with the matched user's IDs when the email matched a user. The
// error text is the message the check has always returned.
func TestAuthService_LoginWithPassword_RefusalNamesTheCheck(t *testing.T) {
	const email = "refusal@example.com"
	deactivated := createTestUser(email)
	deactivated.Status = domain.UserStatusDeactivated
	noPassword := createTestUser(email)
	noPassword.PasswordHash = nil
	active := createTestUser(email)

	cases := []struct {
		name      string
		user      *domain.User // nil: no user has the email
		failures  domain.AuthFailureRepository
		password  string
		reason    string
		message   string
		namesUser bool
	}{
		{"an active lockout", nil, lockedOutFailures{}, "SecurePass123!", SignInRefusedAccountLocked,
			"account is temporarily locked due to too many failed attempts. Please try again in 10 minutes", false},
		{"an unknown email", nil, nil, "SecurePass123!", SignInRefusedUnknownEmail, "invalid credentials", false},
		{"a deactivated user", deactivated, nil, "SecurePass123!", SignInRefusedAccountDeactivated,
			"your account has been deactivated. Please contact your administrator for assistance", true},
		{"a user without a password", noPassword, nil, "SecurePass123!", SignInRefusedPasswordNotSet,
			"local authentication not configured for this user", true},
		{"a wrong password", active, nil, "WrongPassword123!", SignInRefusedPasswordMismatch, "invalid credentials", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := new(MockUserRepository)
			if tc.failures == nil {
				if tc.user != nil {
					users.On("GetByEmail", email).Return(tc.user, nil)
				} else {
					users.On("GetByEmail", email).Return(nil, errors.New("user not found"))
				}
			}
			service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), tc.failures, nil, new(MockEmailService))

			result, err := service.LoginWithPassword(context.Background(), email, tc.password)
			assert.Nil(t, result)
			var refusal *SignInRefusal
			require.True(t, errors.As(err, &refusal), "a refused sign-in returns a *SignInRefusal, got %v", err)
			assert.Equal(t, tc.reason, refusal.Reason)
			assert.Equal(t, tc.message, err.Error())
			if tc.namesUser {
				assert.Equal(t, tc.user.ID, refusal.UserID)
				assert.Equal(t, tc.user.OrganizationID, refusal.OrganizationID)
			} else {
				assert.Equal(t, uuid.Nil, refusal.UserID)
				assert.Equal(t, uuid.Nil, refusal.OrganizationID)
			}
			users.AssertExpectations(t)
		})
	}
}
