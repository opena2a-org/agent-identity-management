package application

import (
	"context"
	"fmt"
	"strings"
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

// The other registration lines name ids too: an address on the
// AIM_PLATFORM_ADMINS list registering (approved at once, its account and
// organization created), an approval whose account cannot be created, and a
// registration refused because no administrator could approve it.

// mailLogOrgRepo has no organization for any domain and stores the one it is given.
type mailLogOrgRepo struct {
	domain.OrganizationRepository
	created *domain.Organization
}

func (r *mailLogOrgRepo) GetByDomain(string) (*domain.Organization, error) {
	return nil, fmt.Errorf("not found")
}

func (r *mailLogOrgRepo) Create(org *domain.Organization) error {
	r.created = org
	return nil
}

func TestPlatformAdminRegistrationLogsCarryNoAddress(t *testing.T) {
	t.Setenv("AIM_PLATFORM_ADMINS", mailLogProbeAddress)
	regRepo := &mailLogRegistrationRepo{}
	userRepo := &mailLogUserRepo{}
	orgRepo := &mailLogOrgRepo{}
	svc := NewRegistrationService(regRepo, userRepo, orgRepo, NewAuditService(discardAuditRepo{}), &privacyEmailService{})

	out := captureAllOutput(t, func() {
		_, err := svc.CreateManualRegistrationRequest(context.Background(), mailLogProbeAddress, "Mail", "Probe", "FAKE-not-real-Pw1!", nil)
		require.NoError(t, err)
	})

	require.NotNil(t, regRepo.req, "the registration request is stored")
	require.Equal(t, domain.RegistrationStatusApproved, regRepo.req.Status, "control: a listed address is approved at once")
	require.NotNil(t, userRepo.created, "control: the account is created")
	require.NotNil(t, orgRepo.created, "control: the organization is created")
	assert.NotContains(t, out, mailLogProbeAddress, "the user's address is not logged")
	assert.Contains(t, out, "Auto-approving platform admin for registration request "+regRepo.req.ID.String())
	assert.Contains(t, out, "Auto-created admin user "+userRepo.created.ID.String()+
		" for registration request "+regRepo.req.ID.String()+" in organization "+orgRepo.created.ID.String())
}

// failingCreateUserRepo refuses to create the account with a database error
// that echoes the address.
type failingCreateUserRepo struct {
	mailLogUserRepo
	createErr error
}

func (r *failingCreateUserRepo) Create(*domain.User) error { return r.createErr }

func TestFailedAccountCreationLogCarriesNoAddress(t *testing.T) {
	t.Setenv("AIM_PLATFORM_ADMINS", "")
	regRepo := &mailLogRegistrationRepo{}
	dbErr := fmt.Errorf(`pq: duplicate key value violates unique constraint "users_email_key": Key (email)=(%s) already exists`, mailLogProbeAddress)
	userRepo := &failingCreateUserRepo{createErr: dbErr}
	svc := NewRegistrationService(regRepo, userRepo, nil, NewAuditService(discardAuditRepo{}), nil)

	var approveErr error
	out := captureAllOutput(t, func() {
		_, err := svc.CreateManualRegistrationRequest(context.Background(), mailLogProbeAddress, "Mail", "Probe", "FAKE-not-real-Pw1!", nil)
		require.NoError(t, err)
		require.NotNil(t, regRepo.req, "the registration request is stored")
		_, approveErr = svc.ApproveRegistrationRequest(context.Background(), regRepo.req.ID, uuid.New(), uuid.New())
	})

	require.ErrorIs(t, approveErr, dbErr, "control: the account is not created")
	assert.NotContains(t, out, mailLogProbeAddress, "the user's address is not logged")
	assert.Contains(t, out, "Registration request: "+regRepo.req.ID.String())
	assert.Contains(t, out, "Key (email)=([address]) already exists", "the address in the database's error is redacted")
	// The returned error is logged by the route that answers 500. Its own text
	// names the registration request; the database's error follows it as given.
	own := strings.TrimSuffix(approveErr.Error(), dbErr.Error())
	assert.NotContains(t, own, mailLogProbeAddress)
	assert.Contains(t, own, regRepo.req.ID.String())
}

// noAdministratorUserRepo has no active administrator.
type noAdministratorUserRepo struct{ mailLogUserRepo }

func (r *noAdministratorUserRepo) CountByRoleAndStatus(domain.UserRole, domain.UserStatus) (int, error) {
	return 0, nil
}

func TestRegistrationRefusedForNoAdministratorLogCarriesNoAddress(t *testing.T) {
	t.Setenv("AIM_PLATFORM_ADMINS", "")
	regRepo := &mailLogRegistrationRepo{}
	svc := NewRegistrationService(regRepo, &noAdministratorUserRepo{}, nil, NewAuditService(discardAuditRepo{}), nil)

	out := captureAllOutput(t, func() {
		_, err := svc.CreateManualRegistrationRequest(context.Background(), mailLogProbeAddress, "Mail", "Probe", "FAKE-not-real-Pw1!", nil)
		require.ErrorIs(t, err, ErrNoAdministrators)
	})

	assert.Nil(t, regRepo.req, "control: nothing is stored")
	assert.NotContains(t, out, mailLogProbeAddress, "the user's address is not logged")
	assert.Contains(t, out, "registration refused: 0 active administrators, 0 valid AIM_PLATFORM_ADMINS entries, registrant listed=false")
}
