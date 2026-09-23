package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The device grant approves a code for a user, then mints the pair on the
// CLI's next poll. Between those two moments the account can be suspended,
// deactivated or deleted; the refresh path refuses such an account and the
// poll must apply the same rule, or a deactivated account receives a fresh
// two-hour access token.

type deviceCodeMapRepo struct{ codes map[string]*domain.DeviceCode }

func (r *deviceCodeMapRepo) Create(_ context.Context, dc *domain.DeviceCode) error {
	r.codes[dc.DeviceCode] = dc
	return nil
}

func (r *deviceCodeMapRepo) GetByDeviceCode(_ context.Context, d string) (*domain.DeviceCode, error) {
	if dc, ok := r.codes[d]; ok {
		return dc, nil
	}
	return nil, errors.New("not found")
}

func (r *deviceCodeMapRepo) GetByUserCode(_ context.Context, u string) (*domain.DeviceCode, error) {
	for _, dc := range r.codes {
		if strings.ReplaceAll(dc.UserCode, "-", "") == strings.ReplaceAll(u, "-", "") {
			return dc, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *deviceCodeMapRepo) Approve(ctx context.Context, u string, userID, orgID uuid.UUID) error {
	dc, err := r.GetByUserCode(ctx, u)
	if err != nil {
		return err
	}
	dc.Status = domain.DeviceCodeStatusApproved
	dc.UserID = &userID
	dc.OrganizationID = &orgID
	return nil
}

func (r *deviceCodeMapRepo) Deny(ctx context.Context, u string) error {
	dc, err := r.GetByUserCode(ctx, u)
	if err != nil {
		return err
	}
	dc.Status = domain.DeviceCodeStatusDenied
	return nil
}

func (r *deviceCodeMapRepo) CleanupExpired(context.Context) (int64, error) { return 0, nil }

// deviceUserStub answers GetByID for one user; everything else on the
// interface is unused by the device service.
type deviceUserStub struct {
	domain.UserRepository
	user *domain.User
}

func (r deviceUserStub) GetByID(id uuid.UUID) (*domain.User, error) {
	if id == r.user.ID {
		return r.user, nil
	}
	return nil, errors.New("no such user")
}

func approvedDeviceGrant(t *testing.T) (*DeviceAuthService, *domain.User, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "dev@example.com",
		Role:           domain.RoleAdmin,
		Status:         domain.UserStatusActive,
	}
	authSvc := NewAuthService(deviceUserStub{user: user}, nil, nil, nil, nil, nil)
	svc := NewDeviceAuthService(&deviceCodeMapRepo{codes: map[string]*domain.DeviceCode{}}, auth.NewJWTService(), authSvc, "http://localhost:3000")
	ctx := context.Background()
	start, err := svc.InitiateDeviceAuth(ctx, "aim-sdk", "", "127.0.0.1", "aim-sdk/test")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveDeviceCode(ctx, start.UserCode, user.ID, user.OrganizationID))
	return svc, user, start.DeviceCode
}

// Control: an active account that approved the code receives the pair.
func TestDeviceAuthService_PollToken_ActiveAccount_ReceivesThePair(t *testing.T) {
	svc, _, deviceCode := approvedDeviceGrant(t)
	tok, err := svc.PollToken(context.Background(), deviceCode)
	require.NoError(t, err)
	assert.NotEmpty(t, tok.AccessToken)
	assert.NotEmpty(t, tok.RefreshToken)
}

// An account deactivated between the approval and the poll receives nothing:
// the poll answers access_denied, the same refusal the refresh path gives.
func TestDeviceAuthService_PollToken_DeactivatedAfterApproval_ReceivesNoPair(t *testing.T) {
	svc, user, deviceCode := approvedDeviceGrant(t)
	user.Status = domain.UserStatusDeactivated
	tok, err := svc.PollToken(context.Background(), deviceCode)
	assert.ErrorIs(t, err, ErrAccessDenied)
	assert.True(t, tok == nil, "no pair may be minted")
}

func TestDeviceAuthService_PollToken_SuspendedAfterApproval_ReceivesNoPair(t *testing.T) {
	svc, user, deviceCode := approvedDeviceGrant(t)
	user.Status = domain.UserStatusSuspended
	tok, err := svc.PollToken(context.Background(), deviceCode)
	assert.ErrorIs(t, err, ErrAccessDenied)
	assert.True(t, tok == nil, "no pair may be minted")
}

func TestDeviceAuthService_PollToken_DeletedAfterApproval_ReceivesNoPair(t *testing.T) {
	svc, user, deviceCode := approvedDeviceGrant(t)
	now := time.Now()
	user.DeletedAt = &now
	tok, err := svc.PollToken(context.Background(), deviceCode)
	assert.ErrorIs(t, err, ErrAccessDenied)
	assert.True(t, tok == nil, "no pair may be minted")
}
