package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Every route that issues a login pair records the sign-in: one login audit
// row in the user's organization naming the family it issued (familyId, the
// sid the issued tokens carry), so a family met later on a refresh, a logout
// or a refusal resolves to the user and organization that signed in. One test
// per route that mints a pair.

// issuedFamily reads the sid of the pair in a sign-in answer and checks that
// both tokens carry the same one.
func issuedFamily(t *testing.T, svc *auth.JWTService, body string) string {
	t.Helper()
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	refresh, err := svc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, refresh.SessionID, "the issued refresh token names its family")
	access, err := svc.ValidateToken(out.AccessToken)
	require.NoError(t, err)
	require.Equal(t, refresh.SessionID, access.SessionID, "the issued pair is one family")
	return refresh.SessionID
}

// assertSignInRow checks the one login row the sign-in wrote.
func assertSignInRow(t *testing.T, rows []*domain.AuditLog, userID, orgID uuid.UUID, family, method string) {
	t.Helper()
	var logins []*domain.AuditLog
	for _, row := range rows {
		if row.Action == domain.AuditActionLogin {
			logins = append(logins, row)
		}
	}
	require.Len(t, logins, 1, "exactly one login row for the sign-in")
	row := logins[0]
	assert.Equal(t, orgID, row.OrganizationID)
	require.NotNil(t, row.UserID)
	assert.Equal(t, userID, *row.UserID)
	assert.Equal(t, userID, row.ResourceID)
	assert.Equal(t, "mint-cell/1", row.UserAgent)
	assert.Equal(t, family, row.Metadata["familyId"], "the row names the issued family")
	assert.Equal(t, method, row.Metadata["method"])
	assert.NotContains(t, marshalled(t, row.Metadata), "eyJ", "no token in the row")
	// Metadata carries route facts only; the account is named by the row's
	// own columns, so no copy of its email, name or role rides along.
	for _, key := range []string{"email", "name", "role"} {
		assert.NotContains(t, marshalled(t, row.Metadata), key, "no %s in the row's metadata", key)
	}
}

func signInLoginBody() string {
	b, _ := json.Marshal(map[string]string{"email": "issuance@example.com", "password": issuancePassword})
	return string(b)
}

// POST /auth/login/local
func TestSignInAudit_LocalLoginNamesTheIssuedFamily(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	user := issuanceUser(t, false)
	users := &issuanceUserRepo{user: user}
	svc := auth.NewJWTService()
	audit := &familyAuditRepo{}
	h := NewAuthHandler(application.NewAuthService(users, nil, nil, nil, nil, nil), svc, nil, application.NewAuditService(audit), nil)
	app := fiber.New()
	app.Post("/auth/login/local", h.LocalLogin)

	status, body := doRequest(t, app, "POST", "/auth/login/local", signInLoginBody())
	require.Equal(t, fiber.StatusOK, status, body)
	assertSignInRow(t, audit.rows, user.ID, user.OrganizationID, issuedFamily(t, svc, body), "password")
}

func publicLoginApp(t *testing.T, user *domain.User) (*fiber.App, *auth.JWTService, *familyAuditRepo) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	users := &issuanceUserRepo{user: user}
	authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
	svc := auth.NewJWTService()
	audit := &familyAuditRepo{}
	h := NewPublicRegistrationHandler(application.NewRegistrationService(refusalRegistrationRepo{}, users, nil, nil, nil), authService, svc, application.NewAuditService(audit))
	app := fiber.New()
	app.Post("/public/login", h.Login)
	return app, svc, audit
}

// POST /public/login, an approved account
func TestSignInAudit_PublicLoginNamesTheIssuedFamily(t *testing.T) {
	user := issuanceUser(t, false)
	app, svc, audit := publicLoginApp(t, user)
	status, body := doRequest(t, app, "POST", "/public/login", signInLoginBody())
	require.Equal(t, fiber.StatusOK, status, body)
	assertSignInRow(t, audit.rows, user.ID, user.OrganizationID, issuedFamily(t, svc, body), "password")
}

// POST /public/login, an account that must change its password first
func TestSignInAudit_PublicLoginPasswordChangeNamesTheIssuedFamily(t *testing.T) {
	user := issuanceUser(t, true)
	app, svc, audit := publicLoginApp(t, user)
	status, body := doRequest(t, app, "POST", "/public/login", signInLoginBody())
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Contains(t, body, `"requiresPasswordChange":true`)
	assertSignInRow(t, audit.rows, user.ID, user.OrganizationID, issuedFamily(t, svc, body), "password")
}

// signInDeviceRepo holds one approved device code and mints on Consume.
type signInDeviceRepo struct{ mintDeviceRepo }

func (r *signInDeviceRepo) Consume(_ context.Context, _ string, mint func() error) error {
	return mint()
}

// POST /oauth/device/token, an approved code
func TestSignInAudit_DeviceTokenNamesTheIssuedFamily(t *testing.T) {
	f := newMintFixture(t, nil, false)
	user := activeUser(f.userID, f.orgID, domain.RoleAdmin, "mint@example.com")
	users := &issuanceUserRepo{user: user}
	repo := &signInDeviceRepo{mintDeviceRepo{code: &domain.DeviceCode{ID: uuid.New(), DeviceCode: "dev", UserCode: "ABCDEFGH", ClientID: "aim-sdk", ExpiresAt: time.Now().Add(10 * time.Minute), Status: domain.DeviceCodeStatusApproved, UserID: &f.userID, OrganizationID: &f.orgID, CreatedAt: time.Now()}}}
	svc := application.NewDeviceAuthService(repo, f.svc, application.NewAuthService(users, nil, nil, nil, nil, nil), "http://localhost:3000")
	h := NewDeviceAuthHandler(svc, f.svc, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Post("/oauth/device/token", h.PollDeviceToken)

	status, body := doRequest(t, app, "POST", "/oauth/device/token", `{"deviceCode":"dev","grantType":"urn:ietf:params:oauth:grant-type:device_code"}`)
	require.Equal(t, fiber.StatusOK, status, body)
	assertSignInRow(t, f.audit.rows, f.userID, f.orgID, issuedFamily(t, f.svc, body), "device_code")
}

// POST /auth/sdk/recover, a revoked SDK credential
func TestSignInAudit_SDKRecoverNamesTheIssuedFamily(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	app, repo, old := sdkRecoverApp(f, f.family)
	status, body := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	require.Equal(t, fiber.StatusOK, status, body)
	require.Len(t, repo.created, 1)
	assertSignInRow(t, f.audit.rows, f.userID, f.orgID, issuedFamily(t, f.svc, body), "sdk_token_recovery")
}
