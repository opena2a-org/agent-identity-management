package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The password reset token at rest is a SHA-256 digest of the token mailed
// to the user, and the reset lookup compares digests. A reader of the users
// table sees only the digest, which cannot be used to reset the password.

// storageUserRepo keeps the last value written to password_reset_token and
// answers GetByPasswordResetToken by equality on that stored value, the way
// the SQL lookup does.
type storageUserRepo struct {
	domain.UserRepository
	user    *domain.User
	written []string
}

func (r *storageUserRepo) GetByEmail(email string) (*domain.User, error) {
	if email == r.user.Email {
		return r.user, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *storageUserRepo) GetByPasswordResetToken(value string) (*domain.User, error) {
	if r.user.PasswordResetToken != nil && *r.user.PasswordResetToken == value {
		return r.user, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *storageUserRepo) Update(user *domain.User) error {
	if user.PasswordResetToken != nil {
		r.written = append(r.written, *user.PasswordResetToken)
	}
	r.user = user
	return nil
}

// discardAuditRepo accepts audit records and keeps none.
type discardAuditRepo struct {
	domain.AuditLogRepository
}

func (discardAuditRepo) Create(*domain.AuditLog) error { return nil }

func storageResetService(t *testing.T) (*RegistrationService, *storageUserRepo, *privacyEmailService) {
	t.Helper()
	repo := &storageUserRepo{user: &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          resetProbeAddress,
		Name:           "Reset Probe",
		Status:         domain.UserStatusActive,
	}}
	mail := &privacyEmailService{}
	svc := NewRegistrationService(nil, repo, nil, NewAuditService(discardAuditRepo{}), mail)
	return svc, repo, mail
}

func mailedResetToken(t *testing.T, mail *privacyEmailService) string {
	t.Helper()
	require.Len(t, mail.sent, 1, "exactly one mail")
	link, ok := mail.sent[0].CustomData["ResetLink"].(string)
	require.True(t, ok)
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	token := parsed.Query().Get("token")
	require.NotEmpty(t, token)
	return token
}

// C1: the value written to password_reset_token is the SHA-256 digest of the
// mailed token, never the token itself.
func TestPasswordResetTokenIsStoredAsADigest(t *testing.T) {
	svc, repo, mail := storageResetService(t)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	token := mailedResetToken(t, mail)

	require.Len(t, repo.written, 1, "one write of the reset token")
	stored := repo.written[0]
	sum := sha256.Sum256([]byte(token))
	assert.NotEqual(t, token, stored, "the mailed token is not written to the database")
	assert.NotContains(t, stored, token)
	assert.Equal(t, hex.EncodeToString(sum[:]), stored, "the stored value is the SHA-256 digest of the mailed token")
}

// C2: the mailed token resets the password; the stored digest does not.
func TestPasswordResetAcceptsTheMailedTokenAndRejectsTheStoredDigest(t *testing.T) {
	svc, repo, mail := storageResetService(t)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	token := mailedResetToken(t, mail)
	stored := *repo.user.PasswordResetToken

	err := svc.ResetPassword(context.Background(), stored, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!")
	require.Error(t, err, "the value read from the database is not a reset token")
	assert.Contains(t, err.Error(), "invalid or expired reset token")
	assert.Nil(t, repo.user.PasswordHash, "the password is unchanged")

	require.NoError(t, svc.ResetPassword(context.Background(), token, "N3w-Passw0rd-Probe!", "N3w-Passw0rd-Probe!"))
	assert.NotNil(t, repo.user.PasswordHash, "the mailed token sets the password")
	assert.Nil(t, repo.user.PasswordResetToken, "the reset clears the stored digest")
	assert.Nil(t, repo.user.PasswordResetExpiresAt)

	err = svc.ResetPassword(context.Background(), token, "An0ther-Passw0rd-Probe!", "An0ther-Passw0rd-Probe!")
	assert.Error(t, err, "a reset token is used once")
}
