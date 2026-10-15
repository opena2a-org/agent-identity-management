package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Registering a user and approving the request writes no line that carries the
// user's address, whether the welcome and approval mails are sent or fail. The
// log names the registration request id and the user id instead, and a
// provider's failure, which often echoes the recipient, is redacted.

const mailLogProbeAddress = "mail.log.probe@example.test"

// mailLogRegistrationRepo keeps the one registration request it is given.
type mailLogRegistrationRepo struct {
	RegistrationRepository
	req *domain.UserRegistrationRequest
}

func (r *mailLogRegistrationRepo) GetRegistrationRequestByEmail(context.Context, string) (*domain.UserRegistrationRequest, error) {
	return nil, fmt.Errorf("not found")
}

func (r *mailLogRegistrationRepo) CreateRegistrationRequest(_ context.Context, req *domain.UserRegistrationRequest) error {
	r.req = req
	return nil
}

func (r *mailLogRegistrationRepo) GetRegistrationRequest(_ context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error) {
	if r.req == nil || r.req.ID != id {
		return nil, fmt.Errorf("not found")
	}
	return r.req, nil
}

func (r *mailLogRegistrationRepo) UpdateRegistrationRequest(context.Context, *domain.UserRegistrationRequest) error {
	return nil
}

// mailLogUserRepo has one active administrator, so registration is accepted,
// and an empty organization, so the approved user is its first member.
type mailLogUserRepo struct {
	domain.UserRepository
	created *domain.User
}

func (r *mailLogUserRepo) GetByEmail(string) (*domain.User, error) {
	return nil, fmt.Errorf("not found")
}

func (r *mailLogUserRepo) CountByRoleAndStatus(domain.UserRole, domain.UserStatus) (int, error) {
	return 1, nil
}

func (r *mailLogUserRepo) GetByOrganization(uuid.UUID) ([]*domain.User, error) { return nil, nil }

func (r *mailLogUserRepo) Create(user *domain.User) error {
	r.created = user
	return nil
}

func TestRegistrationAndApprovalLogsCarryNoAddress(t *testing.T) {
	t.Setenv("AIM_PLATFORM_ADMINS", "")
	cases := []struct {
		name    string
		sendErr error
	}{
		{"mail sent", nil},
		{"mail failed", fmt.Errorf("RCPT TO failed: 550 5.1.1 <%s>: recipient rejected", mailLogProbeAddress)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			regRepo := &mailLogRegistrationRepo{}
			userRepo := &mailLogUserRepo{}
			mail := &privacyEmailService{sendErr: tc.sendErr}
			svc := NewRegistrationService(regRepo, userRepo, nil, NewAuditService(discardAuditRepo{}), mail)

			out := captureAllOutput(t, func() {
				_, err := svc.CreateManualRegistrationRequest(context.Background(), mailLogProbeAddress, "Mail", "Probe", "FAKE-not-real-Pw1!", nil)
				require.NoError(t, err)
				require.NotNil(t, regRepo.req, "the registration request is stored")
				_, err = svc.ApproveRegistrationRequest(context.Background(), regRepo.req.ID, uuid.New(), uuid.New())
				require.NoError(t, err)
			})

			require.Len(t, mail.sent, 2, "control: the welcome and the approval mail are both attempted")
			require.NotNil(t, userRepo.created, "control: the user is created")
			assert.NotContains(t, out, mailLogProbeAddress, "the user's address is not logged")
			assert.Contains(t, out, regRepo.req.ID.String(), "control: the registration request id is logged")
			assert.Contains(t, out, userRepo.created.ID.String(), "control: the user id is logged")
			if tc.sendErr != nil {
				assert.Contains(t, out, "RCPT TO failed", "control: the provider's failure is logged")
				assert.Contains(t, out, "<[address]>", "the address in the provider's failure is redacted")
			}
		})
	}
}
