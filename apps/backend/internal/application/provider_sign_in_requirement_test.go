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

// An organization's OIDC sign-in configuration can require its users who are
// not administrators to sign in through the provider. The password sign-in
// then refuses them a correct password, after the password check, and signs
// in its administrators and the users of every other organization as before.

// requirementStub answers the requirement of each organization it names, or
// err, and counts the reads.
type requirementStub struct {
	required map[uuid.UUID]bool
	err      error
	reads    int
}

func (s *requirementStub) RequiresProviderSignIn(_ context.Context, organizationID uuid.UUID) (bool, error) {
	s.reads++
	if s.err != nil {
		return false, s.err
	}
	return s.required[organizationID], nil
}

// countingFailures counts the failed attempts the service records and the
// times it clears them. No email is locked out.
type countingFailures struct {
	domain.AuthFailureRepository
	recorded int
	cleared  int
}

func (f *countingFailures) GetActiveLockout(string) (*domain.AuthLockout, error) { return nil, nil }

func (f *countingFailures) RecordFailure(*domain.AuthFailure) error {
	f.recorded++
	return nil
}

func (f *countingFailures) CountRecentFailures(string, int) (int, error) { return f.recorded, nil }

func (f *countingFailures) ClearFailures(string) error {
	f.cleared++
	return nil
}

// requirementSignIn is a password sign-in service whose one user is user,
// with the requirement attached unless it is nil. The users mock fails the
// test on a call it was not told of, an UpdateLastLogin among them, unless
// signsIn is set.
func requirementSignIn(t *testing.T, user *domain.User, requirement ProviderSignInRequirement, signsIn bool) (*AuthService, *MockUserRepository, *countingFailures) {
	t.Helper()
	users := new(MockUserRepository)
	users.On("GetByEmail", user.Email).Return(user, nil)
	if signsIn {
		users.On("UpdateLastLogin", user.ID, mock.Anything).Return(nil)
	}
	failures := &countingFailures{}
	service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), failures, nil, new(MockEmailService))
	if requirement != nil {
		service.SetProviderSignInRequirement(requirement)
	}
	return service, users, failures
}

func TestAuthService_LoginWithPassword_RefusesAUserWhoMustSignInThroughTheProvider(t *testing.T) {
	member := createTestUser("member@example.com")
	requirement := &requirementStub{required: map[uuid.UUID]bool{member.OrganizationID: true}}
	service, users, failures := requirementSignIn(t, member, requirement, false)

	result, err := service.LoginWithPassword(context.Background(), member.Email, "SecurePass123!")
	assert.Nil(t, result)
	var refusal *SignInRefusal
	require.True(t, errors.As(err, &refusal), "a refused sign-in returns a *SignInRefusal, got %v", err)
	assert.Equal(t, SignInRefusedProviderSignInRequired, refusal.Reason)
	assert.Equal(t, member.ID, refusal.UserID)
	assert.Equal(t, member.OrganizationID, refusal.OrganizationID)
	assert.Equal(t, 1, requirement.reads)
	assert.Zero(t, failures.recorded, "the refusal is not recorded as a failed attempt")
	assert.Zero(t, failures.cleared, "the refusal clears no failed attempt")
	assert.Nil(t, member.LastLoginAt, "the refused sign-in is not recorded in last_login_at")
	users.AssertExpectations(t)

	// CheckPasswordSignIn, which the routes that check the password
	// themselves call, gives the same refusal.
	err = service.CheckPasswordSignIn(context.Background(), member)
	require.True(t, errors.As(err, &refusal), "got %v", err)
	assert.Equal(t, SignInRefusedProviderSignInRequired, refusal.Reason)
}

// The requirement is read only once the password matched: a wrong password
// is refused and recorded as it was, and says nothing of the requirement.
func TestAuthService_LoginWithPassword_ReadsTheRequirementOnlyForTheUsersPassword(t *testing.T) {
	member := createTestUser("member@example.com")
	requirement := &requirementStub{required: map[uuid.UUID]bool{member.OrganizationID: true}}
	service, users, failures := requirementSignIn(t, member, requirement, false)

	result, err := service.LoginWithPassword(context.Background(), member.Email, "WrongPassword123!")
	assert.Nil(t, result)
	var refusal *SignInRefusal
	require.True(t, errors.As(err, &refusal), "got %v", err)
	assert.Equal(t, SignInRefusedPasswordMismatch, refusal.Reason)
	assert.Equal(t, "invalid credentials", err.Error())
	assert.Zero(t, requirement.reads)
	assert.Equal(t, 1, failures.recorded, "the wrong password counts toward a lockout as before")
	users.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_SignsInWhomTheRequirementDoesNotReach(t *testing.T) {
	t.Run("an administrator of the organization, whose sign-in reads no requirement", func(t *testing.T) {
		admin := createTestUser("admin@example.com")
		admin.Role = domain.RoleAdmin
		requirement := &requirementStub{required: map[uuid.UUID]bool{admin.OrganizationID: true}}
		service, users, failures := requirementSignIn(t, admin, requirement, true)

		result, err := service.LoginWithPassword(context.Background(), admin.Email, "SecurePass123!")
		require.NoError(t, err)
		assert.Equal(t, admin.ID, result.ID)
		assert.Zero(t, requirement.reads)
		assert.Equal(t, 1, failures.cleared, "a sign-in clears the failed attempts as before")
		users.AssertExpectations(t)
	})

	t.Run("a user of another organization", func(t *testing.T) {
		member := createTestUser("member@example.com")
		requirement := &requirementStub{required: map[uuid.UUID]bool{uuid.New(): true}}
		service, users, _ := requirementSignIn(t, member, requirement, true)

		result, err := service.LoginWithPassword(context.Background(), member.Email, "SecurePass123!")
		require.NoError(t, err)
		assert.Equal(t, member.ID, result.ID)
		assert.Equal(t, 1, requirement.reads)
		users.AssertExpectations(t)
	})

	t.Run("a user of an organization when no requirement is attached", func(t *testing.T) {
		member := createTestUser("member@example.com")
		service, users, _ := requirementSignIn(t, member, nil, true)

		result, err := service.LoginWithPassword(context.Background(), member.Email, "SecurePass123!")
		require.NoError(t, err)
		assert.Equal(t, member.ID, result.ID)
		users.AssertExpectations(t)
	})
}

// A requirement that cannot be read refuses the sign-in of a user it would
// reach, with an error that is not a refusal, and signs in an administrator.
func TestAuthService_LoginWithPassword_RefusesWhenTheRequirementCannotBeRead(t *testing.T) {
	member := createTestUser("member@example.com")
	requirement := &requirementStub{err: errors.New("the database is unreachable")}
	service, users, failures := requirementSignIn(t, member, requirement, false)

	result, err := service.LoginWithPassword(context.Background(), member.Email, "SecurePass123!")
	assert.Nil(t, result)
	require.Error(t, err)
	var refusal *SignInRefusal
	assert.False(t, errors.As(err, &refusal), "the error is the server's, not a refusal: %v", err)
	assert.Contains(t, err.Error(), "the database is unreachable")
	assert.Zero(t, failures.recorded)
	assert.Nil(t, member.LastLoginAt)
	users.AssertExpectations(t)

	admin := createTestUser("admin@example.com")
	admin.Role = domain.RoleAdmin
	service, users, _ = requirementSignIn(t, admin, requirement, true)
	result, err = service.LoginWithPassword(context.Background(), admin.Email, "SecurePass123!")
	require.NoError(t, err)
	assert.Equal(t, admin.ID, result.ID)
	users.AssertExpectations(t)
}

// unreadableOIDCConfigs answers every read with an error.
type unreadableOIDCConfigs struct {
	domain.OIDCSignInConfigRepository
}

func (unreadableOIDCConfigs) Get(context.Context, uuid.UUID) (*domain.OIDCSignInConfig, error) {
	return nil, errors.New("the database is unreachable")
}

// Configure stores the requirement, and RequiresProviderSignIn answers it
// for the organization that stored it and for no other. An organization
// without a configuration requires none.
func TestOIDCSignInConfigureStoresTheRequirementOfProviderSignIn(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	ctx := context.Background()

	for org, want := range map[uuid.UUID]bool{f.orgA: false, f.orgB: false} {
		required, err := f.service.RequiresProviderSignIn(ctx, org)
		require.NoError(t, err)
		assert.Equal(t, want, required)
	}

	cfg, err := f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
		RequireProviderSignIn: true,
	})
	require.NoError(t, err)
	assert.True(t, cfg.RequireProviderSignIn)
	required, err := f.service.RequiresProviderSignIn(ctx, f.orgA)
	require.NoError(t, err)
	assert.True(t, required)
	required, err = f.service.RequiresProviderSignIn(ctx, f.orgB)
	require.NoError(t, err)
	assert.False(t, required, "A's requirement is not B's")

	// A configuration stored without it requires none.
	cfg, err = f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
	})
	require.NoError(t, err)
	assert.False(t, cfg.RequireProviderSignIn)
	required, err = f.service.RequiresProviderSignIn(ctx, f.orgA)
	require.NoError(t, err)
	assert.False(t, required)

	// Removing the configuration removes the requirement with it.
	_, err = f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
		RequireProviderSignIn: true,
	})
	require.NoError(t, err)
	require.NoError(t, f.service.RemoveConfiguration(ctx, f.orgA))
	required, err = f.service.RequiresProviderSignIn(ctx, f.orgA)
	require.NoError(t, err)
	assert.False(t, required)

	unreadable := NewOIDCSignInService(unreadableOIDCConfigs{}, nil, nil, nil, nil, "https://aim.example.com")
	required, err = unreadable.RequiresProviderSignIn(ctx, f.orgA)
	assert.Error(t, err)
	assert.False(t, required)
}

// The requirement is about passwords: the users it reaches still sign in
// through the provider.
func TestOIDCSignInSignsInAUserWhoMustSignInThroughTheProvider(t *testing.T) {
	f := newOIDCSignInFixture(t, func(orgA, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(orgA, "alice@example.com")}
	})
	_, err := f.service.Configure(context.Background(), f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
		RequireProviderSignIn: true,
	})
	require.NoError(t, err)

	user, err := f.signIn(t, f.orgA, verified("alice@example.com"))
	require.NoError(t, err)
	assert.Equal(t, f.orgA, user.OrganizationID)
	assert.Equal(t, "alice@example.com", user.Email)
}
