package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth/oidctest"
)

// memoryOIDCConfigs is an OIDCSignInConfigRepository over a map.
type memoryOIDCConfigs struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.OIDCSignInConfig
}

func (m *memoryOIDCConfigs) Get(_ context.Context, org uuid.UUID) (*domain.OIDCSignInConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.rows[org]
	if !ok {
		return nil, domain.ErrOIDCSignInNotConfigured
	}
	return &cfg, nil
}

func (m *memoryOIDCConfigs) Put(_ context.Context, cfg *domain.OIDCSignInConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := *cfg
	stored.UpdatedAt = time.Now().UTC()
	m.rows[cfg.OrganizationID] = stored
	return nil
}

func (m *memoryOIDCConfigs) Delete(_ context.Context, org uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, org)
	return nil
}

// memoryOrganizations answers GetByID as the organization repository does.
type memoryOrganizations map[uuid.UUID]*domain.Organization

func (m memoryOrganizations) GetByID(id uuid.UUID) (*domain.Organization, error) {
	org, ok := m[id]
	if !ok {
		return nil, errors.New("organization not found")
	}
	copied := *org
	return &copied, nil
}

// memoryUsers holds users and records each sign-in it is told of. Its
// GetByOrganization leaves deleted_at out, as the repository's query does.
type memoryUsers struct {
	mu         sync.Mutex
	rows       map[uuid.UUID]domain.User
	lastLogins map[uuid.UUID]int
}

func newMemoryUsers(users ...domain.User) *memoryUsers {
	m := &memoryUsers{rows: map[uuid.UUID]domain.User{}, lastLogins: map[uuid.UUID]int{}}
	for _, u := range users {
		m.rows[u.ID] = u
	}
	return m
}

func (m *memoryUsers) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.User
	for _, u := range m.rows {
		if u.OrganizationID == orgID {
			listed := u
			listed.DeletedAt = nil
			out = append(out, &listed)
		}
	}
	return out, nil
}

func (m *memoryUsers) GetByID(id uuid.UUID) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.rows[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return &u, nil
}

func (m *memoryUsers) UpdateLastLogin(id uuid.UUID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.rows[id]
	u.LastLoginAt = &at
	m.rows[id] = u
	m.lastLogins[id]++
	return nil
}

// snapshot returns a copy of every user.
func (m *memoryUsers) snapshot() map[uuid.UUID]domain.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[uuid.UUID]domain.User{}
	for id, u := range m.rows {
		out[id] = u
	}
	return out
}

func newTestOIDCVault(t *testing.T) *crypto.KeyVault {
	t.Helper()
	master := make([]byte, 32)
	_, err := rand.Read(master)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	return vault
}

func oidcTestUser(org uuid.UUID, email string) domain.User {
	return domain.User{ID: uuid.New(), OrganizationID: org, Email: email, Name: email, Role: domain.RoleMember,
		Status: domain.UserStatusActive, Provider: "local"}
}

// oidcSignInFixture is the service over two organizations, A and B, with the
// stub provider configured as A's. B has no provider unless a test gives it
// one.
type oidcSignInFixture struct {
	stub    *oidctest.Provider
	service *OIDCSignInService
	configs *memoryOIDCConfigs
	orgs    memoryOrganizations
	users   *memoryUsers
	orgA    uuid.UUID
	orgB    uuid.UUID
}

func newOIDCSignInFixture(t *testing.T, users func(orgA, orgB uuid.UUID) []domain.User) *oidcSignInFixture {
	t.Helper()
	f := &oidcSignInFixture{
		stub:    oidctest.NewProvider(t),
		configs: &memoryOIDCConfigs{rows: map[uuid.UUID]domain.OIDCSignInConfig{}},
		orgA:    uuid.New(),
		orgB:    uuid.New(),
	}
	f.orgs = memoryOrganizations{
		f.orgA: {ID: f.orgA, Name: "Organization A", IsActive: true},
		f.orgB: {ID: f.orgB, Name: "Organization B", IsActive: true},
	}
	f.users = newMemoryUsers(users(f.orgA, f.orgB)...)
	f.service = NewOIDCSignInService(f.configs, f.orgs, f.users, newTestOIDCVault(t),
		oauth.NewOIDCClient(f.stub.Client()), "https://aim.example.com/")
	_, err := f.service.Configure(context.Background(), f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
	})
	require.NoError(t, err)
	return f
}

// start starts a sign-in for org and has the stub sign in id, returning the
// started sign-in and the code and state of the stub's redirect.
func (f *oidcSignInFixture) start(t *testing.T, org uuid.UUID, id oidctest.Identity) (*OIDCSignInStart, string, string) {
	t.Helper()
	f.stub.SignInAs(id)
	start, err := f.service.StartSignIn(context.Background(), org)
	require.NoError(t, err)
	code, state := f.stub.Authorize(t, start.AuthorizationURL, f.service.RedirectURL())
	return start, code, state
}

func (f *oidcSignInFixture) signIn(t *testing.T, org uuid.UUID, id oidctest.Identity) (*domain.User, error) {
	t.Helper()
	start, code, state := f.start(t, org, id)
	return f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
}

func refusalReason(t *testing.T, err error) string {
	t.Helper()
	var refusal *OIDCSignInRefusal
	require.True(t, errors.As(err, &refusal), "want an OIDCSignInRefusal, got %v", err)
	return refusal.Reason
}

func verified(email string) oidctest.Identity {
	return oidctest.Identity{Subject: "sub-" + email, Email: email, EmailVerified: true}
}

func TestOIDCSignInEndsAsTheUserOfTheConfiguredOrganization(t *testing.T) {
	var alice domain.User
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		alice = oidcTestUser(a, "alice@example.com")
		return []domain.User{alice}
	})

	start, code, state := f.start(t, f.orgA, verified("alice@example.com"))
	authURL, err := url.Parse(start.AuthorizationURL)
	require.NoError(t, err)
	assert.Equal(t, "https://aim.example.com/auth/oidc/callback", authURL.Query().Get("redirect_uri"))
	assert.True(t, strings.HasPrefix(start.Flow, f.orgA.String()+"."))
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), start.ExpiresAt, time.Minute)

	user, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	require.NoError(t, err)
	assert.Equal(t, alice.ID, user.ID)
	assert.Equal(t, f.orgA, user.OrganizationID)
	require.NotNil(t, user.LastLoginAt)
	assert.Equal(t, 1, f.users.lastLogins[alice.ID])
	assert.Equal(t, 1, f.stub.Redeemed())

	_, err = f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	assert.Equal(t, OIDCSignInRefusedCodeRefused, refusalReason(t, err), "the provider redeems a code once")
}

func TestOIDCSignInMatchesTheEmailWithoutRegardToCase(t *testing.T) {
	var alice domain.User
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		alice = oidcTestUser(a, "alice@example.com")
		return []domain.User{alice}
	})
	user, err := f.signIn(t, f.orgA, verified("Alice@Example.COM"))
	require.NoError(t, err)
	assert.Equal(t, alice.ID, user.ID)
}

// A sign-in through organization A's provider never signs in, creates or
// changes a user of organization B, whatever email the provider asserts.
func TestOIDCSignInNeverSignsInAUserOfAnotherOrganization(t *testing.T) {
	var bobB, carolA, carolB domain.User
	f := newOIDCSignInFixture(t, func(a, b uuid.UUID) []domain.User {
		bobB = oidcTestUser(b, "bob@example.com")
		carolA = oidcTestUser(a, "carol@example.com")
		carolB = oidcTestUser(b, "carol@example.com")
		carolB.Role = domain.RoleAdmin
		return []domain.User{bobB, carolA, carolB}
	})
	before := f.users.snapshot()

	// Bob is a user of B only: A's provider asserting his email signs in no one.
	user, err := f.signIn(t, f.orgA, verified("bob@example.com"))
	assert.Nil(t, user)
	assert.Equal(t, OIDCSignInRefusedNoUser, refusalReason(t, err))
	assert.Equal(t, before, f.users.snapshot(), "no user was created, changed or moved")

	// Carol has a user in each organization: A's provider signs in A's.
	user, err = f.signIn(t, f.orgA, verified("carol@example.com"))
	require.NoError(t, err)
	assert.Equal(t, carolA.ID, user.ID)
	assert.Equal(t, f.orgA, user.OrganizationID)
	assert.Equal(t, domain.RoleMember, user.Role)

	after := f.users.snapshot()
	assert.Len(t, after, len(before), "no user was created")
	assert.Equal(t, before[bobB.ID], after[bobB.ID])
	assert.Equal(t, before[carolB.ID], after[carolB.ID], "B's Carol is not signed in or changed")
	assert.Zero(t, f.users.lastLogins[carolB.ID])
	assert.Zero(t, f.users.lastLogins[bobB.ID])
	for id, u := range after {
		assert.Equal(t, before[id].OrganizationID, u.OrganizationID, "no user changed organization")
	}
}

// B has a provider of its own. A flow started for one organization cannot be
// completed for the other: the sealed flow is bound to its organization, and
// its state is not the other flow's.
func TestOIDCSignInFlowIsBoundToItsOrganization(t *testing.T) {
	var aliceA, aliceB domain.User
	f := newOIDCSignInFixture(t, func(a, b uuid.UUID) []domain.User {
		aliceA, aliceB = oidcTestUser(a, "alice@example.com"), oidcTestUser(b, "alice@example.com")
		return []domain.User{aliceA, aliceB}
	})
	// Every httptest TLS server presents the same certificate, so the client
	// that trusts A's provider trusts B's.
	stubB := oidctest.NewProvider(t)
	_, err := f.service.Configure(context.Background(), f.orgB, uuid.New(), OIDCSignInConfigInput{
		Issuer: stubB.Issuer, ClientID: stubB.ClientID, ClientSecret: stubB.ClientSecret,
	})
	require.NoError(t, err)

	startA, codeA, stateA := f.start(t, f.orgA, verified("alice@example.com"))
	stubB.SignInAs(verified("alice@example.com"))
	startB, err := f.service.StartSignIn(context.Background(), f.orgB)
	require.NoError(t, err)
	codeB, stateB := stubB.Authorize(t, startB.AuthorizationURL, f.service.RedirectURL())

	// A's flow with B's id in front is not a flow of B.
	_, sealed, _ := strings.Cut(startA.Flow, ".")
	_, err = f.service.CompleteSignIn(context.Background(), codeA, stateA, f.orgB.String()+"."+sealed)
	assert.Equal(t, OIDCSignInRefusedFlowInvalid, refusalReason(t, err))

	// Neither organization's flow takes the other's code and state.
	_, err = f.service.CompleteSignIn(context.Background(), codeB, stateB, startA.Flow)
	assert.Equal(t, OIDCSignInRefusedStateMismatch, refusalReason(t, err))
	_, err = f.service.CompleteSignIn(context.Background(), codeA, stateA, startB.Flow)
	assert.Equal(t, OIDCSignInRefusedStateMismatch, refusalReason(t, err))
	assert.Zero(t, f.stub.Redeemed()+stubB.Redeemed(), "no code was redeemed")

	user, err := f.service.CompleteSignIn(context.Background(), codeA, stateA, startA.Flow)
	require.NoError(t, err)
	assert.Equal(t, aliceA.ID, user.ID)
	user, err = f.service.CompleteSignIn(context.Background(), codeB, stateB, startB.Flow)
	require.NoError(t, err)
	assert.Equal(t, aliceB.ID, user.ID)
}

func TestOIDCSignInRefusesAFlowThatIsNotOneTheServerSealed(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	start, code, state := f.start(t, f.orgA, verified("alice@example.com"))
	orgPart, sealed, _ := strings.Cut(start.Flow, ".")
	raw, err := base64.StdEncoding.DecodeString(sealed)
	require.NoError(t, err)
	raw[len(raw)/2] ^= 0x01

	for name, flow := range map[string]string{
		"a changed byte":  orgPart + "." + base64.StdEncoding.EncodeToString(raw),
		"no organization": sealed,
		"not a UUID":      "org." + sealed,
		"empty":           "",
	} {
		_, err := f.service.CompleteSignIn(context.Background(), code, state, flow)
		assert.Equal(t, OIDCSignInRefusedFlowInvalid, refusalReason(t, err), name)
	}
	assert.Zero(t, f.stub.Redeemed(), "no code is redeemed for a flow that does not open")
}

func TestOIDCSignInRefusesAStateThatIsNotTheFlows(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	start, code, _ := f.start(t, f.orgA, verified("alice@example.com"))
	for _, state := range []string{"another-state", ""} {
		_, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
		assert.Equal(t, OIDCSignInRefusedStateMismatch, refusalReason(t, err))
	}
}

func TestOIDCSignInRefusesAFlowStartedMoreThanTenMinutesAgo(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	start, code, state := f.start(t, f.orgA, verified("alice@example.com"))

	f.service.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	_, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	assert.Equal(t, OIDCSignInRefusedFlowExpired, refusalReason(t, err))
}

// Every ID token the Done-when of the first slice names is refused: a wrong
// issuer, audience or nonce, a bad signature, an expired token.
func TestOIDCSignInRefusesAnIDTokenThatFailsVerification(t *testing.T) {
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})

	for name, edit := range map[string]func(*oidctest.Provider){
		"wrong issuer": func(p *oidctest.Provider) {
			p.EditIDToken = func(c jwt.MapClaims) { c["iss"] = "https://other.example.com" }
		},
		"wrong audience": func(p *oidctest.Provider) { p.EditIDToken = func(c jwt.MapClaims) { c["aud"] = "another-client" } },
		"wrong nonce":    func(p *oidctest.Provider) { p.EditIDToken = func(c jwt.MapClaims) { c["nonce"] = "another-nonce" } },
		"bad signature":  func(p *oidctest.Provider) { p.SigningKey = otherKey },
		"expired": func(p *oidctest.Provider) {
			p.EditIDToken = func(c jwt.MapClaims) {
				c["exp"] = time.Now().Add(-10 * time.Minute).Unix()
				c["iat"] = time.Now().Add(-20 * time.Minute).Unix()
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			edit(f.stub)
			t.Cleanup(func() { f.stub.EditIDToken, f.stub.SigningKey = nil, nil })

			user, err := f.signIn(t, f.orgA, verified("alice@example.com"))
			assert.Nil(t, user)
			assert.Equal(t, OIDCSignInRefusedIDTokenInvalid, refusalReason(t, err))
		})
	}
	assert.Empty(t, f.users.lastLogins, "no refused token recorded a sign-in")
}

func TestOIDCSignInRefusesAnEmailTheProviderDoesNotAssertAsVerified(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	_, err := f.signIn(t, f.orgA, oidctest.Identity{Subject: "s", Email: "alice@example.com", EmailVerified: false})
	assert.Equal(t, OIDCSignInRefusedEmailNotVerified, refusalReason(t, err))

	_, err = f.signIn(t, f.orgA, oidctest.Identity{Subject: "s", EmailVerified: true})
	assert.Equal(t, OIDCSignInRefusedEmailNotVerified, refusalReason(t, err))
}

func TestOIDCSignInRefusesAUserWhoMayNotHoldASession(t *testing.T) {
	deletedAt := time.Now()
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		deactivated := oidcTestUser(a, "deactivated@example.com")
		deactivated.Status = domain.UserStatusDeactivated
		suspended := oidcTestUser(a, "suspended@example.com")
		suspended.Status = domain.UserStatusSuspended
		deleted := oidcTestUser(a, "deleted@example.com")
		deleted.DeletedAt = &deletedAt
		return []domain.User{deactivated, suspended, deleted}
	})
	for _, email := range []string{"deactivated@example.com", "suspended@example.com", "deleted@example.com"} {
		_, err := f.signIn(t, f.orgA, verified(email))
		assert.Equal(t, OIDCSignInRefusedAccountInactive, refusalReason(t, err), email)
	}
	assert.Empty(t, f.users.lastLogins)
}

func TestOIDCSignInRefusesAnEmailThatMatchesTwoUsers(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "dana@example.com"), oidcTestUser(a, "Dana@example.com")}
	})
	_, err := f.signIn(t, f.orgA, verified("dana@example.com"))
	assert.Equal(t, OIDCSignInRefusedAmbiguousUser, refusalReason(t, err))
}

func TestOIDCSignInAfterTheProviderChangedOrWasRemoved(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	start, code, state := f.start(t, f.orgA, verified("alice@example.com"))
	_, err := f.service.Configure(context.Background(), f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: "another-client", ClientSecret: "not-a-real-client-secret",
	})
	require.NoError(t, err)
	_, err = f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	assert.Equal(t, OIDCSignInRefusedProviderChanged, refusalReason(t, err))

	require.NoError(t, f.service.RemoveConfiguration(context.Background(), f.orgA))
	_, err = f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	assert.Equal(t, OIDCSignInRefusedProviderChanged, refusalReason(t, err))
	_, err = f.service.StartSignIn(context.Background(), f.orgA)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured)
}

func TestOIDCSignInStartForAnOrganizationThatCannotSignIn(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User { return nil })

	_, err := f.service.StartSignIn(context.Background(), f.orgB)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured, "B has no provider")
	_, err = f.service.StartSignIn(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured, "no such organization")

	f.orgs[f.orgA].IsActive = false
	_, err = f.service.StartSignIn(context.Background(), f.orgA)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured, "A is not active")
}

func TestOIDCSignInCompleteForAnOrganizationThatIsNoLongerActive(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	start, code, state := f.start(t, f.orgA, verified("alice@example.com"))
	f.orgs[f.orgA].IsActive = false
	_, err := f.service.CompleteSignIn(context.Background(), code, state, start.Flow)
	assert.Equal(t, OIDCSignInRefusedOrganizationInactive, refusalReason(t, err))
	assert.Zero(t, f.stub.Redeemed())
}

func TestOIDCSignInConfigureStoresTheSecretSealed(t *testing.T) {
	f := newOIDCSignInFixture(t, func(_, _ uuid.UUID) []domain.User { return nil })
	cfg, err := f.service.Configuration(context.Background(), f.orgA)
	require.NoError(t, err)
	assert.Equal(t, f.stub.Issuer, cfg.Issuer)
	assert.Equal(t, f.stub.ClientID, cfg.ClientID)
	assert.NotContains(t, cfg.SealedClientSecret, f.stub.ClientSecret)

	opened, err := f.service.sealer.OpenOIDCClientSecret(f.orgA, cfg.SealedClientSecret)
	require.NoError(t, err)
	assert.Equal(t, f.stub.ClientSecret, opened)
	_, err = f.service.sealer.OpenOIDCClientSecret(f.orgB, cfg.SealedClientSecret)
	assert.Error(t, err, "the sealed secret is bound to A")
}

func TestOIDCSignInConfigureRefuses(t *testing.T) {
	f := newOIDCSignInFixture(t, func(_, _ uuid.UUID) []domain.User { return nil })
	good := OIDCSignInConfigInput{Issuer: f.stub.Issuer, ClientID: "aim-client", ClientSecret: "not-a-real-client-secret"}

	for name, in := range map[string]OIDCSignInConfigInput{
		"an http issuer":           {Issuer: "http://idp.example.com", ClientID: good.ClientID, ClientSecret: good.ClientSecret},
		"an issuer with a query":   {Issuer: f.stub.Issuer + "?x=1", ClientID: good.ClientID, ClientSecret: good.ClientSecret},
		"no issuer":                {ClientID: good.ClientID, ClientSecret: good.ClientSecret},
		"no client id":             {Issuer: good.Issuer, ClientID: "  ", ClientSecret: good.ClientSecret},
		"no client secret":         {Issuer: good.Issuer, ClientID: good.ClientID, ClientSecret: " "},
		"a client id too long":     {Issuer: good.Issuer, ClientID: strings.Repeat("c", 513), ClientSecret: good.ClientSecret},
		"a client secret too long": {Issuer: good.Issuer, ClientID: good.ClientID, ClientSecret: strings.Repeat("s", 2049)},
	} {
		_, err := f.service.Configure(context.Background(), f.orgB, uuid.New(), in)
		assert.ErrorIs(t, err, ErrOIDCSignInConfigInvalid, name)
	}

	// An issuer that serves no configuration document is refused.
	_, err := f.service.Configure(context.Background(), f.orgB, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer + "/no-such-tenant", ClientID: good.ClientID, ClientSecret: good.ClientSecret,
	})
	assert.ErrorIs(t, err, oauth.ErrOIDCProviderUnavailable)

	_, err = f.service.Configuration(context.Background(), f.orgB)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured, "no refused configuration was stored")
}
