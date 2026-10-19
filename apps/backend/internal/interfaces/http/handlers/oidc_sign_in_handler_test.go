package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth/oidctest"
)

const oidcTestDashboard = "https://aim.example.com"

type oidcHandlerConfigs struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.OIDCSignInConfig
}

func (m *oidcHandlerConfigs) Get(_ context.Context, org uuid.UUID) (*domain.OIDCSignInConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.rows[org]
	if !ok {
		return nil, domain.ErrOIDCSignInNotConfigured
	}
	return &cfg, nil
}

func (m *oidcHandlerConfigs) Put(_ context.Context, cfg *domain.OIDCSignInConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := *cfg
	stored.UpdatedAt = time.Now().UTC()
	m.rows[cfg.OrganizationID] = stored
	return nil
}

func (m *oidcHandlerConfigs) Delete(_ context.Context, org uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, org)
	return nil
}

type oidcHandlerOrgs map[uuid.UUID]*domain.Organization

func (m oidcHandlerOrgs) GetByID(id uuid.UUID) (*domain.Organization, error) {
	org, ok := m[id]
	if !ok {
		return nil, errors.New("organization not found")
	}
	copied := *org
	return &copied, nil
}

// oidcHandlerUsers holds the users of every organization. It counts the
// sign-ins it records per user.
type oidcHandlerUsers struct {
	mu         sync.Mutex
	rows       map[uuid.UUID]domain.User
	lastLogins map[uuid.UUID]int
}

func (m *oidcHandlerUsers) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.User
	for _, u := range m.rows {
		if u.OrganizationID == orgID {
			listed := u
			out = append(out, &listed)
		}
	}
	return out, nil
}

func (m *oidcHandlerUsers) GetByID(id uuid.UUID) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.rows[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return &u, nil
}

func (m *oidcHandlerUsers) UpdateLastLogin(id uuid.UUID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.rows[id]
	u.LastLoginAt = &at
	m.rows[id] = u
	m.lastLogins[id]++
	return nil
}

func (m *oidcHandlerUsers) EmailInUseOutsideOrganization(email string, orgID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.rows {
		if u.OrganizationID != orgID && strings.EqualFold(u.Email, email) {
			return true, nil
		}
	}
	return false, nil
}

// Create adds the user, refusing a second user of an organization with the
// same email as the users table does.
func (m *oidcHandlerUsers) Create(user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.rows {
		if u.OrganizationID == user.OrganizationID && u.Email == user.Email {
			return errors.New(`duplicate key value violates unique constraint "users_organization_id_email_key"`)
		}
	}
	m.rows[user.ID] = *user
	return nil
}

// oidcAuditRepo records every audit row the handler writes.
type oidcAuditRepo struct {
	domain.AuditLogRepository
	mu   sync.Mutex
	rows []*domain.AuditLog
}

func (r *oidcAuditRepo) Create(row *domain.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, row)
	return nil
}

func (r *oidcAuditRepo) taken() []*domain.AuditLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := r.rows
	r.rows = nil
	return rows
}

func (m *oidcHandlerUsers) all() map[uuid.UUID]domain.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[uuid.UUID]domain.User{}
	for id, u := range m.rows {
		out[id] = u
	}
	return out
}

// oidcHandlerFixture mounts the handler's routes, with the stub provider as
// organization A's and none for organization B. The admin routes take the
// caller's organization and user from the X-Test-Org and X-Test-User headers,
// as the auth middleware would set them from a session.
type oidcHandlerFixture struct {
	app     *fiber.App
	stub    *oidctest.Provider
	jwt     *auth.JWTService
	service *application.OIDCSignInService
	users   *oidcHandlerUsers
	audit   *oidcAuditRepo
	orgA    uuid.UUID
	orgB    uuid.UUID
	alice   domain.User // a member of A
	bob     domain.User // a member of B only
	carolA  domain.User // Carol of A
	carolB  domain.User // Carol of B, an admin there
}

func newOIDCHandlerFixture(t *testing.T) *oidcHandlerFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	f := &oidcHandlerFixture{stub: oidctest.NewProvider(t), jwt: auth.NewJWTService(), orgA: uuid.New(), orgB: uuid.New()}

	user := func(org uuid.UUID, email string, role domain.UserRole) domain.User {
		return domain.User{ID: uuid.New(), OrganizationID: org, Email: email, Name: email, Role: role,
			Status: domain.UserStatusActive, Provider: "local"}
	}
	f.alice = user(f.orgA, "alice@example.com", domain.RoleMember)
	f.bob = user(f.orgB, "bob@example.com", domain.RoleMember)
	f.carolA = user(f.orgA, "carol@example.com", domain.RoleMember)
	f.carolB = user(f.orgB, "carol@example.com", domain.RoleAdmin)
	f.users = &oidcHandlerUsers{rows: map[uuid.UUID]domain.User{}, lastLogins: map[uuid.UUID]int{}}
	for _, u := range []domain.User{f.alice, f.bob, f.carolA, f.carolB} {
		f.users.rows[u.ID] = u
	}

	master := make([]byte, 32)
	_, err := rand.Read(master)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)

	orgs := oidcHandlerOrgs{
		f.orgA: {ID: f.orgA, Name: "Organization A", IsActive: true},
		f.orgB: {ID: f.orgB, Name: "Organization B", IsActive: true},
	}
	f.service = application.NewOIDCSignInService(&oidcHandlerConfigs{rows: map[uuid.UUID]domain.OIDCSignInConfig{}}, orgs, f.users,
		vault, oauth.NewOIDCClient(f.stub.Client()), oidcTestDashboard)
	_, err = f.service.Configure(context.Background(), f.orgA, uuid.New(), application.OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: f.stub.ClientSecret,
	})
	require.NoError(t, err)

	f.audit = &oidcAuditRepo{}
	h := NewOIDCSignInHandler(f.service, f.jwt, application.NewAuditService(f.audit))
	f.app = fiber.New()
	f.app.Post("/api/v1/auth/oidc/:organizationId/login", h.StartSignIn)
	f.app.Post("/api/v1/auth/oidc/callback", h.CompleteSignIn)
	admin := f.app.Group("/api/v1/admin", func(c fiber.Ctx) error {
		if org, err := uuid.Parse(c.Get("X-Test-Org")); err == nil {
			c.Locals("organization_id", org)
		}
		if user, err := uuid.Parse(c.Get("X-Test-User")); err == nil {
			c.Locals("user_id", user)
		}
		return c.Next()
	})
	admin.Get("/oidc-sign-in", h.GetConfiguration)
	admin.Put("/oidc-sign-in", h.PutConfiguration)
	admin.Delete("/oidc-sign-in", h.DeleteConfiguration)
	return f
}

// do sends one request to the app and returns the status and, when the answer
// is JSON, its body.
func (f *oidcHandlerFixture) do(t *testing.T, method, path string, body interface{}, headers map[string]string) (int, map[string]interface{}) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// The sign-in routes call the stub provider; give them longer than the
	// default second on a loaded machine.
	resp, err := f.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer map[string]interface{}
	if strings.HasPrefix(resp.Header.Get(fiber.HeaderContentType), fiber.MIMEApplicationJSON) {
		require.NoError(t, json.Unmarshal(raw, &answer), string(raw))
	}
	if method == fiber.MethodPost && strings.HasPrefix(path, "/api/v1/auth/oidc/") && resp.StatusCode == fiber.StatusOK {
		assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl), path)
	}
	return resp.StatusCode, answer
}

// signIn drives a sign-in for org through the routes, with the stub signing
// in id, and returns the callback's status and answer.
func (f *oidcHandlerFixture) signIn(t *testing.T, org uuid.UUID, id oidctest.Identity) (int, map[string]interface{}) {
	t.Helper()
	f.stub.SignInAs(id)
	status, start := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+org.String()+"/login", nil, nil)
	require.Equal(t, fiber.StatusOK, status, "%v", start)
	authURL, _ := start["authorizationUrl"].(string)
	flow, _ := start["flow"].(string)
	require.NotEmpty(t, authURL)
	require.NotEmpty(t, flow)

	code, state := f.stub.Authorize(t, authURL, oidcTestDashboard+"/auth/oidc/callback")
	status, answer := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/callback",
		map[string]string{"code": code, "state": state, "flow": flow}, nil)
	return status, answer
}

func verifiedIdentity(email string) oidctest.Identity {
	return oidctest.Identity{Subject: "sub-" + email, Email: email, EmailVerified: true}
}

// The first slice's sign-in, end to end: the login route answers the stub
// provider's authorization URL, the stub (on 127.0.0.1) signs Alice in and
// redirects with a code, and the callback route answers a login pair for
// Alice in organization A.
func TestOIDCSignInRoutesEndSignedInAsAUserOfTheConfiguredOrganization(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	require.True(t, strings.HasPrefix(f.stub.Issuer, "https://127.0.0.1:"), f.stub.Issuer)

	status, answer := f.signIn(t, f.orgA, verifiedIdentity("alice@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", answer)

	access, err := f.jwt.ValidateToken(answer["accessToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, f.alice.ID.String(), access.UserID)
	assert.Equal(t, f.orgA.String(), access.OrganizationID)
	assert.Equal(t, auth.TokenTypeAccess, access.TokenType)
	refresh, err := f.jwt.ValidateToken(answer["refreshToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, f.orgA.String(), refresh.OrganizationID)

	user := answer["user"].(map[string]interface{})
	assert.Equal(t, f.alice.ID.String(), user["id"])
	assert.Equal(t, f.orgA.String(), user["organizationId"])
	assert.Equal(t, 1, f.users.lastLogins[f.alice.ID])

	// One login row in A, naming the method and the family the pair carries.
	rows := f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, domain.AuditActionLogin, rows[0].Action)
	assert.Equal(t, f.orgA, rows[0].OrganizationID)
	require.NotNil(t, rows[0].UserID)
	assert.Equal(t, f.alice.ID, *rows[0].UserID)
	assert.Equal(t, "oidc", rows[0].Metadata["method"])
	assert.Equal(t, refresh.FamilyID(), rows[0].Metadata["familyId"])
}

// A provider that cannot be reached answers 502 at either route.
func TestOIDCSignInRoutesWhenTheProviderCannotBeReached(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	f.stub.SignInAs(verifiedIdentity("alice@example.com"))
	status, start := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+f.orgA.String()+"/login", nil, nil)
	require.Equal(t, fiber.StatusOK, status)
	code, state := f.stub.Authorize(t, start["authorizationUrl"].(string), oidcTestDashboard+"/auth/oidc/callback")

	f.stub.Server.Close()
	status, answer := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/callback",
		map[string]string{"code": code, "state": state, "flow": start["flow"].(string)}, nil)
	assert.Equal(t, fiber.StatusBadGateway, status)
	assert.NotContains(t, answer, "accessToken")
	status, _ = f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+f.orgA.String()+"/login", nil, nil)
	assert.Equal(t, fiber.StatusBadGateway, status)
	assert.Empty(t, f.audit.taken(), "no sign-in was recorded")
}

// Each ID token the first slice's Done-when names is refused through the
// callback route, and no pair is answered.
func TestOIDCSignInCallbackRefusesIDTokensThatFailVerification(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	for name, edit := range map[string]func(){
		"a wrong issuer":   func() { f.stub.EditIDToken = func(c jwt.MapClaims) { c["iss"] = "https://other.example.com" } },
		"a wrong audience": func() { f.stub.EditIDToken = func(c jwt.MapClaims) { c["aud"] = "another-client" } },
		"a wrong nonce":    func() { f.stub.EditIDToken = func(c jwt.MapClaims) { c["nonce"] = "another-nonce" } },
		"a bad signature":  func() { f.stub.SigningKey = otherKey },
		"an expired token": func() {
			f.stub.EditIDToken = func(c jwt.MapClaims) {
				c["exp"] = time.Now().Add(-10 * time.Minute).Unix()
				c["iat"] = time.Now().Add(-20 * time.Minute).Unix()
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			edit()
			t.Cleanup(func() { f.stub.EditIDToken, f.stub.SigningKey = nil, nil })

			status, answer := f.signIn(t, f.orgA, verifiedIdentity("alice@example.com"))
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, application.OIDCSignInRefusedIDTokenInvalid, answer["code"])
			assert.NotContains(t, answer, "accessToken")
			assert.NotContains(t, answer, "refreshToken")
		})
	}
	assert.Empty(t, f.users.lastLogins)
	assert.Empty(t, f.audit.taken(), "no refused sign-in was recorded as a login")
}

// A sign-in through organization A's provider never signs in, creates or
// changes a user of organization B.
func TestOIDCSignInRoutesNeverSignInAUserOfAnotherOrganization(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	before := f.users.all()

	status, answer := f.signIn(t, f.orgA, verifiedIdentity("bob@example.com"))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, application.OIDCSignInRefusedNoUser, answer["code"])
	assert.NotContains(t, answer, "accessToken")
	assert.Equal(t, before, f.users.all(), "no user was created, changed or moved")

	status, answer = f.signIn(t, f.orgA, verifiedIdentity("carol@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	access, err := f.jwt.ValidateToken(answer["accessToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, f.carolA.ID.String(), access.UserID, "A's provider signs in A's Carol")
	assert.Equal(t, f.orgA.String(), access.OrganizationID)
	assert.Equal(t, string(domain.RoleMember), access.Role, "not the admin role B's Carol holds")

	after := f.users.all()
	assert.Len(t, after, len(before), "no user was created")
	assert.Equal(t, before[f.carolB.ID], after[f.carolB.ID])
	assert.Equal(t, before[f.bob.ID], after[f.bob.ID])

	// B has no provider: its users cannot start a sign-in at all.
	status, _ = f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+f.orgB.String()+"/login", nil, nil)
	assert.Equal(t, fiber.StatusNotFound, status)
}

func TestOIDCSignInRoutesRefuseMalformedRequests(t *testing.T) {
	f := newOIDCHandlerFixture(t)

	status, _ := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/not-a-uuid/login", nil, nil)
	assert.Equal(t, fiber.StatusBadRequest, status)
	status, _ = f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+uuid.NewString()+"/login", nil, nil)
	assert.Equal(t, fiber.StatusNotFound, status)

	for _, body := range []map[string]string{
		{"state": "s", "flow": "f"},
		{"code": "c", "flow": "f"},
		{"code": "c", "state": "s"},
	} {
		status, _ := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/callback", body, nil)
		assert.Equal(t, fiber.StatusBadRequest, status, "%v", body)
	}

	status, answer := f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/callback",
		map[string]string{"code": "c", "state": "s", "flow": f.orgA.String() + ".bm90LWEtZmxvdw=="}, nil)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, application.OIDCSignInRefusedFlowInvalid, answer["code"])
}

// The administration routes read, store and remove the caller's
// organization's provider, never another organization's, and no answer
// carries the client secret.
func TestOIDCSignInAdministrationRoutes(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	adminB := map[string]string{"X-Test-Org": f.orgB.String(), "X-Test-User": f.carolB.ID.String()}

	status, answer := f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminB)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, map[string]interface{}{"configured": false, "redirectUrl": oidcTestDashboard + "/auth/oidc/callback"}, answer,
		"A's provider is not B's")

	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in",
		map[string]string{"issuer": "http://idp.example.com", "clientId": "c", "clientSecret": "not-a-real-client-secret"}, adminB)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Contains(t, answer["error"], "https")

	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in",
		map[string]string{"issuer": f.stub.Issuer + "/no-such-tenant", "clientId": "c", "clientSecret": "not-a-real-client-secret"}, adminB)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "The issuer's configuration document could not be read, or names an endpoint that is not an https URL", answer["error"])

	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in",
		map[string]string{"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "clientSecret": f.stub.ClientSecret}, adminB)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.Equal(t, true, answer["configured"])
	assert.Equal(t, f.stub.Issuer, answer["issuer"])
	assert.Equal(t, f.stub.ClientID, answer["clientId"])
	assert.Equal(t, true, answer["clientSecretSet"])
	assert.Equal(t, f.carolB.ID.String(), answer["updatedBy"])
	raw, err := json.Marshal(answer)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), f.stub.ClientSecret)
	assert.NotContains(t, answer, "clientSecret")
	assert.NotContains(t, answer, "sealedClientSecret")

	rows := f.audit.taken()
	require.Len(t, rows, 1, "one row for the stored provider; the refused ones wrote none")
	assert.Equal(t, domain.AuditActionUpdate, rows[0].Action)
	assert.Equal(t, "oidc_sign_in_config", rows[0].ResourceType)
	assert.Equal(t, f.orgB, rows[0].OrganizationID)
	assert.Equal(t, f.orgB, rows[0].ResourceID)
	assert.Equal(t, f.carolB.ID, *rows[0].UserID)
	assert.Equal(t, f.stub.Issuer, rows[0].Metadata["issuer"])
	rawRow, err := json.Marshal(rows[0].Metadata)
	require.NoError(t, err)
	assert.NotContains(t, string(rawRow), f.stub.ClientSecret, "the audit row does not carry the secret")

	// B's provider now signs in B's users, and A's still A's.
	status, signedIn := f.signIn(t, f.orgB, verifiedIdentity("carol@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", signedIn)
	access, err := f.jwt.ValidateToken(signedIn["accessToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, f.carolB.ID.String(), access.UserID)

	_ = f.audit.taken() // the login row of Carol's sign-in

	status, _ = f.do(t, fiber.MethodDelete, "/api/v1/admin/oidc-sign-in", nil, adminB)
	assert.Equal(t, fiber.StatusNoContent, status)
	rows = f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, domain.AuditActionDelete, rows[0].Action)
	assert.Equal(t, "oidc_sign_in_config", rows[0].ResourceType)
	assert.Equal(t, f.orgB, rows[0].OrganizationID)
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminB)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, false, answer["configured"])
	status, _ = f.do(t, fiber.MethodPost, "/api/v1/auth/oidc/"+f.orgB.String()+"/login", nil, nil)
	assert.Equal(t, fiber.StatusNotFound, status)

	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": uuid.NewString()}
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, true, answer["configured"], "removing B's provider left A's in place")

	status, _ = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, nil)
	assert.Equal(t, fiber.StatusUnauthorized, status, "no organization in the session")
}
