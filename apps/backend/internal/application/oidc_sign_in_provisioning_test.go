package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Just-in-time provisioning: a configuration that provisions users creates,
// at the first sign-in of a verified email that no user of the organization
// has, a user of that organization, and of no other.

// provisionAt stores the stub provider as org's, provisioning users with
// role at domains.
func (f *oidcSignInFixture) provisionAt(t *testing.T, org uuid.UUID, role string, domains ...string) {
	t.Helper()
	_, err := f.service.Configure(context.Background(), org, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
		Provisioning: OIDCProvisioningInput{Enabled: true, Role: role, EmailDomains: domains},
	})
	require.NoError(t, err)
}

func noUsers(_, _ uuid.UUID) []domain.User { return nil }

func TestOIDCSignInProvisionsAUserOfTheConfiguredOrganization(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	f.provisionAt(t, f.orgA, "member", "example.com")

	id := verified("Dave@Example.com")
	id.Name = "Dave Example"
	start, code, state := f.start(t, f.orgA, id)
	result, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	require.NoError(t, err)
	assert.True(t, result.Provisioned)
	dave := result.User
	assert.Equal(t, f.orgA, dave.OrganizationID)
	assert.Equal(t, "dave@example.com", dave.Email, "stored in lower case")
	assert.Equal(t, "Dave Example", dave.Name)
	assert.Equal(t, domain.RoleMember, dave.Role)
	assert.Equal(t, domain.UserStatusActive, dave.Status)
	assert.Equal(t, "oidc", dave.Provider)
	assert.Equal(t, dave.ID.String(), dave.ProviderID)
	assert.Nil(t, dave.PasswordHash, "a provisioned user has no password")
	assert.False(t, dave.ForcePasswordChange)
	assert.Nil(t, dave.ApprovedBy)
	require.NotNil(t, dave.ApprovedAt)
	require.NotNil(t, dave.LastLoginAt)
	assert.Equal(t, 1, f.users.lastLogins[dave.ID])

	stored := f.users.snapshot()
	require.Len(t, stored, 1)
	assert.Equal(t, f.orgA, stored[dave.ID].OrganizationID)

	// The next sign-in signs the same user in and creates none.
	start, code, state = f.start(t, f.orgA, verified("dave@example.com"))
	result, err = f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	require.NoError(t, err)
	assert.False(t, result.Provisioned)
	assert.Equal(t, dave.ID, result.User.ID)
	assert.Len(t, f.users.snapshot(), 1)
	assert.Equal(t, 1, f.users.creates)
	assert.Equal(t, 2, f.users.lastLogins[dave.ID])
}

func TestOIDCSignInProvisionsAViewerNamedByTheTokenOrTheEmail(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	f.provisionAt(t, f.orgA, "", "example.com")
	cfg, err := f.service.Configuration(context.Background(), f.orgA)
	require.NoError(t, err)
	assert.Equal(t, domain.OIDCProvisioning{Enabled: true, Role: domain.RoleViewer, EmailDomains: []string{"example.com"}},
		cfg.Provisioning, "a configuration that names no role provisions viewers")

	user, err := f.signIn(t, f.orgA, verified("erin@example.com"))
	require.NoError(t, err)
	assert.Equal(t, domain.RoleViewer, user.Role)
	assert.Equal(t, "erin@example.com", user.Name, "a token without a name names the user by the email")

	long := verified("frank@example.com")
	long.Name = strings.Repeat("é", 300)
	user, err = f.signIn(t, f.orgA, long)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("é", 255), user.Name, "the name is cut to the 255 characters of users.name")
}

func TestOIDCSignInProvisionsOnlyAddressesAtTheConfiguredDomains(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	f.provisionAt(t, f.orgA, "viewer", "example.com", "example.org")

	for _, email := range []string{
		"alice@other.test",
		"alice@sub.example.com",
		"alice@example.com.other.test",
		"alice@notexample.com",
		"alice@example.com.",
		"alice@exa mple.com",
		"al ice@example.com",
		"@example.com",
		"alice@bob@example.com",
		strings.Repeat("a", 244) + "@example.com", // 256 bytes, one more than users.email holds
	} {
		user, err := f.signIn(t, f.orgA, verified(email))
		assert.Nil(t, user, email)
		assert.Equal(t, OIDCSignInRefusedDomainNotProvisioned, refusalReason(t, err), email)
	}
	assert.Empty(t, f.users.snapshot(), "no user was created")
	assert.Zero(t, f.users.creates)

	for _, email := range []string{"grace@example.org", "heidi@EXAMPLE.COM", strings.Repeat("a", 243) + "@example.com"} {
		_, err := f.signIn(t, f.orgA, verified(email))
		require.NoError(t, err, email)
	}
	assert.Len(t, f.users.snapshot(), 3)
}

// A's configuration provisions users at example.com, the domain of B's
// users. It never creates a user, in A or in B, for the email of a user of B,
// deleted or not, and leaves B's users as they were.
func TestOIDCSignInNeverProvisionsTheEmailOfAnotherOrganizationsUser(t *testing.T) {
	deletedAt := time.Now()
	var bobB domain.User
	f := newOIDCSignInFixture(t, func(_, b uuid.UUID) []domain.User {
		bobB = oidcTestUser(b, "bob@example.com")
		ivanB := oidcTestUser(b, "ivan@example.com")
		ivanB.DeletedAt = &deletedAt
		return []domain.User{bobB, ivanB}
	})
	f.provisionAt(t, f.orgA, "manager", "example.com")
	before := f.users.snapshot()

	for _, email := range []string{"bob@example.com", "BOB@example.com", "ivan@example.com"} {
		user, err := f.signIn(t, f.orgA, verified(email))
		assert.Nil(t, user, email)
		assert.Equal(t, OIDCSignInRefusedEmailInOtherOrganization, refusalReason(t, err), email)
	}
	assert.Equal(t, before, f.users.snapshot(), "no user was created, changed or moved")
	assert.Zero(t, f.users.creates)
	assert.Empty(t, f.users.lastLogins)

	// An email no organization has is provisioned in A, and only there.
	user, err := f.signIn(t, f.orgA, verified("judy@example.com"))
	require.NoError(t, err)
	assert.Equal(t, f.orgA, user.OrganizationID)
	after := f.users.snapshot()
	assert.Len(t, after, len(before)+1)
	for id, u := range before {
		assert.Equal(t, u, after[id], "B's users are as they were")
	}
}

func TestOIDCSignInWithoutProvisioningCreatesNoUser(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	cfg, err := f.service.Configuration(context.Background(), f.orgA)
	require.NoError(t, err)
	assert.Equal(t, domain.OIDCProvisioning{}, cfg.Provisioning, "a configuration provisions no user unless it says so")

	_, err = f.signIn(t, f.orgA, verified("judy@example.com"))
	assert.Equal(t, OIDCSignInRefusedNoUser, refusalReason(t, err))

	// A provisioning that is not enabled stores no role and no domain,
	// whatever it names.
	_, err = f.service.Configure(context.Background(), f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
		Provisioning: OIDCProvisioningInput{Role: "admin", EmailDomains: []string{"example.com"}},
	})
	require.NoError(t, err)
	cfg, err = f.service.Configuration(context.Background(), f.orgA)
	require.NoError(t, err)
	assert.Equal(t, domain.OIDCProvisioning{}, cfg.Provisioning)
	_, err = f.signIn(t, f.orgA, verified("judy@example.com"))
	assert.Equal(t, OIDCSignInRefusedNoUser, refusalReason(t, err))

	assert.Empty(t, f.users.snapshot())
	assert.Zero(t, f.users.creates)
}

// A stored provisioning whose role Configure refuses, written around it,
// provisions no one.
func TestOIDCSignInDoesNotProvisionWithAStoredRoleConfigureRefuses(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	cfg, err := f.service.Configuration(context.Background(), f.orgA)
	require.NoError(t, err)

	for _, role := range []domain.UserRole{domain.RoleAdmin, "owner", ""} {
		cfg.Provisioning = domain.OIDCProvisioning{Enabled: true, Role: role, EmailDomains: []string{"example.com"}}
		require.NoError(t, f.configs.Put(context.Background(), cfg))
		_, err := f.signIn(t, f.orgA, verified("mallory@example.com"))
		assert.Equal(t, OIDCSignInRefusedNoUser, refusalReason(t, err), string(role))
	}
	assert.Empty(t, f.users.snapshot())
	assert.Zero(t, f.users.creates)
}

// A concurrent first sign-in with the same email creates the user between
// this sign-in's lookup and its create: this sign-in ends as that user and
// creates no second one.
func TestOIDCSignInProvisioningEndsAsTheUserAConcurrentSignInCreated(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	f.provisionAt(t, f.orgA, "viewer", "example.com")
	var concurrent domain.User
	f.users.beforeCreate = func(user *domain.User) {
		f.users.beforeCreate = nil
		concurrent = *user
		concurrent.ID = uuid.New()
		concurrent.ProviderID = concurrent.ID.String()
		f.users.mu.Lock()
		f.users.rows[concurrent.ID] = concurrent
		f.users.mu.Unlock()
	}

	start, code, state := f.start(t, f.orgA, verified("kim@example.com"))
	result, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	require.NoError(t, err)
	assert.False(t, result.Provisioned, "this sign-in created no user")
	assert.Equal(t, concurrent.ID, result.User.ID)
	assert.Len(t, f.users.snapshot(), 1)
	assert.Equal(t, 1, f.users.lastLogins[concurrent.ID])
}

// A create that fails, with no concurrent sign-in to explain it, fails the
// sign-in with an error rather than a refusal, and signs no one in.
func TestOIDCSignInProvisioningThatCannotCreateTheUserIsAnError(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	f.provisionAt(t, f.orgA, "viewer", "example.com")
	f.users.beforeCreate = func(user *domain.User) {
		// Another user takes the provider id the create names.
		other := *user
		other.ID = uuid.New()
		other.Email = "someone-else@example.com"
		f.users.mu.Lock()
		f.users.rows[other.ID] = other
		f.users.mu.Unlock()
	}

	user, err := f.signIn(t, f.orgA, verified("leo@example.com"))
	assert.Nil(t, user)
	require.Error(t, err)
	var refusal *OIDCSignInRefusal
	assert.False(t, errors.As(err, &refusal), "a failed create is not a refusal: %v", err)
	assert.Empty(t, f.users.lastLogins)
}

func TestOIDCSignInConfigureProvisioning(t *testing.T) {
	f := newOIDCSignInFixture(t, noUsers)
	ctx := context.Background()
	withProvisioning := func(p OIDCProvisioningInput) OIDCSignInConfigInput {
		return OIDCSignInConfigInput{Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret, Provisioning: p}
	}

	longest := strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 61) // 253 characters
	cfg, err := f.service.Configure(ctx, f.orgB, uuid.New(), withProvisioning(OIDCProvisioningInput{
		Enabled: true, Role: "manager", EmailDomains: []string{" Example.COM. ", "example.org", "example.com", longest},
	}))
	require.NoError(t, err)
	assert.Equal(t, domain.OIDCProvisioning{Enabled: true, Role: domain.RoleManager,
		EmailDomains: []string{"example.com", "example.org", longest}}, cfg.Provisioning,
		"domains are stored in lower case, without a final dot, each once")

	for _, role := range []string{"admin", "owner", "Viewer"} {
		_, err := f.service.Configure(ctx, f.orgB, uuid.New(), withProvisioning(OIDCProvisioningInput{
			Enabled: true, Role: role, EmailDomains: []string{"example.com"},
		}))
		assert.ErrorIs(t, err, ErrOIDCProvisioningRoleInvalid, role)
		assert.ErrorIs(t, err, ErrOIDCSignInConfigInvalid, role)
	}

	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("d%d.example.com", i)
	}
	cfg, err = f.service.Configure(ctx, f.orgA, uuid.New(), withProvisioning(OIDCProvisioningInput{
		Enabled: true, EmailDomains: tooMany[:20],
	}))
	require.NoError(t, err, "20 domains are stored")
	assert.Equal(t, tooMany[:20], cfg.Provisioning.EmailDomains)

	for name, domains := range map[string][]string{
		"no domain":         nil,
		"21 domains":        tooMany,
		"an empty domain":   {""},
		"a wildcard":        {"*.example.com"},
		"one label":         {"example"},
		"an address":        {"user@example.com"},
		"a URL":             {"https://example.com"},
		"a space":           {"exa mple.com"},
		"an underscore":     {"ex_ample.com"},
		"a leading hyphen":  {"-example.com"},
		"a trailing hyphen": {"example-.com"},
		"an empty label":    {"example..com"},
		"a label of 64":     {strings.Repeat("a", 64) + ".com"},
		"254 characters":    {strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 62)},
		"a non-ASCII name":  {"bücher.example"},
		"one bad of two":    {"example.com", "*.example.org"},
	} {
		_, err := f.service.Configure(ctx, f.orgB, uuid.New(), withProvisioning(OIDCProvisioningInput{
			Enabled: true, EmailDomains: domains,
		}))
		assert.ErrorIs(t, err, ErrOIDCProvisioningDomainsInvalid, name)
		assert.ErrorIs(t, err, ErrOIDCSignInConfigInvalid, name)
	}

	cfg, err = f.service.Configuration(ctx, f.orgB)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleManager, cfg.Provisioning.Role, "no refused configuration replaced the stored one")
}
