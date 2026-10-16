package handlers

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// The three credential-minting routes mint only for an access token that
// records a sign-in. An access token minted by refreshing an SDK-download
// token passes the auth middleware (it is a valid bearer) but carries the
// download's family and no auth_time: it is refused at an SDK download, a
// device sign-in approval and an SDK credential recovery with the routes'
// existing 401, recorded once as credential_mint_refused with the cause, so a
// downloaded SDK's credential cannot start a new family. These cells run the
// real auth middleware, so what the routes read is what a bearer yields.

type signInMintFixture struct {
	svc          *auth.JWTService
	audit        *familyAuditRepo
	userID       uuid.UUID
	orgID        uuid.UUID
	sdkAccess    string // minted by refreshing an SDK-download token
	sdkFamily    string
	signInAccess string // a sign-in's access token
}

func newSignInMintFixture(t *testing.T) *signInMintFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	svc := auth.NewJWTService()
	svc.SetRevoker(auth.NewTokenRevoker(&rotationStore{}, false))
	f := &signInMintFixture{svc: svc, audit: &familyAuditRepo{}, userID: uuid.New(), orgID: uuid.New()}
	sdk, err := svc.GenerateSDKRefreshToken(f.userID.String(), f.orgID.String(), "signin@example.com", "admin")
	require.NoError(t, err)
	f.sdkFamily = jtiOfToken(t, svc, sdk)
	f.sdkAccess, _, err = svc.RefreshTokenPair(sdk, "signin@example.com", "admin")
	require.NoError(t, err)
	f.signInAccess, _, err = svc.GenerateTokenPair(f.userID.String(), f.orgID.String(), "signin@example.com", "admin")
	require.NoError(t, err)
	return f
}

func (f *signInMintFixture) users() domain.UserRepository {
	return &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(f.userID, f.orgID, domain.RoleAdmin, "signin@example.com"), nil
	}}
}

// behind builds an app with the real auth middleware in front of the route.
func (f *signInMintFixture) behind(method, path string, h fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Use(middleware.AuthMiddleware(f.svc))
	if method == "GET" {
		app.Get(path, h)
	} else {
		app.Post(path, h)
	}
	return app
}

func sdkTreeFixture(t *testing.T) {
	t.Helper()
	// the zip is built from SDK_BASE_DIR/<sdk>; a two-file fixture stands in for the SDK tree
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "python", "aim_sdk"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "VERSION"), []byte("0.0.0-fixture\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "aim_sdk", "__init__.py"), []byte("# fixture\n"), 0o644))
	t.Setenv("SDK_BASE_DIR", dir)
}

func bearerRequest(t *testing.T, app *fiber.App, method, path, token, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "sign-in-cell/1")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// R1: at each of the three routes an access token minted from an SDK-download
// token gets 401 and one credential_mint_refused record with cause noSignIn;
// a sign-in's access token gets 200 at each.
func TestCredentialMint_RefusesAnAccessTokenMintedFromAnSDKDownloadToken(t *testing.T) {
	logs := captureLog(t)
	f := newSignInMintFixture(t)
	auditSvc := application.NewAuditService(f.audit)
	sdkTreeFixture(t)

	downloadRepo := &rotationSDKRepo{}
	download := f.behind("GET", "/sdk/download", NewSDKHandler(f.svc, downloadRepo, auditSvc).DownloadSDK)

	deviceRepo := &mintDeviceRepo{code: &domain.DeviceCode{ID: uuid.New(), DeviceCode: "dev", UserCode: "ABCDEFGH", ClientID: "aim-sdk", ExpiresAt: time.Now().Add(10 * time.Minute), Status: domain.DeviceCodeStatusPending, CreatedAt: time.Now()}}
	approve := f.behind("POST", "/oauth/device/approve", NewDeviceAuthHandler(application.NewDeviceAuthService(deviceRepo, f.svc, nil, "http://localhost:3000"), f.svc, auditSvc).ApproveDevice)

	recoverRepo := &rotationSDKRepo{token: revokedSDKToken(f.userID, f.orgID)}
	recover := f.behind("POST", "/auth/sdk/recover", NewSDKTokenRecoveryHandler(application.NewSDKTokenService(recoverRepo), f.svc, f.users(), auditSvc).RecoverRevokedToken)
	old, err := f.svc.GenerateSDKRefreshToken(f.userID.String(), f.orgID.String(), "signin@example.com", "admin")
	require.NoError(t, err)

	routes := []struct {
		route, method, path, body string
		app                       *fiber.App
		minted                    func() int
	}{
		{"sdk_download", "GET", "/sdk/download?sdk=python", "", download, func() int { return len(downloadRepo.created) }},
		{"device_approve", "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`, approve, func() int { return len(deviceRepo.approved) }},
		{"sdk_recover", "POST", "/auth/sdk/recover", recoverBody(old), recover, func() int { return len(recoverRepo.created) }},
	}
	for _, r := range routes {
		t.Run(r.route, func(t *testing.T) {
			logs.Reset()
			f.audit.rows = nil
			status, body := bearerRequest(t, r.app, r.method, r.path, f.sdkAccess, r.body)
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Contains(t, body, mintRefusal)
			assert.Equal(t, 0, r.minted(), "nothing is minted or approved")
			refused := refusedRows(f.audit.rows)
			require.Len(t, refused, 1, "one credential_mint_refused row")
			assert.Equal(t, "noSignIn", refused[0].Metadata["cause"])
			assert.Equal(t, f.sdkFamily, refused[0].Metadata["familyId"], "the row names the download's family")
			assert.Equal(t, r.route, refused[0].Metadata["route"])
			require.NotNil(t, refused[0].UserID)
			assert.Equal(t, f.userID, *refused[0].UserID)
			assert.NotContains(t, marshalled(t, refused[0].Metadata), "eyJ", "no token in the row")
			logged := logs.String()
			assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused"))
			assert.Contains(t, logged, "cause=noSignIn")
			assert.Contains(t, logged, "family="+f.sdkFamily)
			assert.NotContains(t, logged, "eyJ", "no token in the log")

			// Control: a sign-in's access token mints at the same route.
			f.audit.rows = nil
			status, body = bearerRequest(t, r.app, r.method, r.path, f.signInAccess, r.body)
			assert.Equal(t, fiber.StatusOK, status, body)
			assert.Equal(t, 1, r.minted())
			assert.Empty(t, refusedRows(f.audit.rows), "no refusal is recorded")
		})
	}
}

// R2: the stored-credential sequence (refresh an SDK-download token, then
// download a fresh SDK with the access token it yields) fails at the download,
// and the chain does not start a new family; the dashboard's sign-in does.
func TestCredentialMint_RefreshThenDownloadFailsAtTheDownload(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status, "the refresh itself succeeds")
	require.Len(t, f.repo.rows, 2, "the download's row and its successor")

	download := fiber.New()
	download.Use(middleware.AuthMiddleware(f.svc))
	download.Get("/sdk/download", NewSDKHandler(f.svc, f.repo, application.NewAuditService(f.audit)).DownloadSDK)

	status, body := bearerRequest(t, download, "GET", "/sdk/download?sdk=python", out.AccessToken, "")
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, mintRefusal)
	assert.Len(t, f.repo.rows, 2, "no new family: no third row")
	require.Len(t, refusedRows(f.audit.rows), 1)
	assert.Equal(t, "noSignIn", refusedRows(f.audit.rows)[0].Metadata["cause"])

	// Control: a sign-in's access token downloads a new SDK, a new family.
	signIn, _, err := f.svc.GenerateTokenPair(f.userID.String(), f.orgID.String(), "sdkfam@example.com", "admin")
	require.NoError(t, err)
	status, _ = bearerRequest(t, download, "GET", "/sdk/download?sdk=python", signIn, "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, f.repo.rows, 3)
}

// R3: a sign-in's access token whose session was revoked is still refused and
// recorded with its own cause, so the two refusals stay apart in the record.
func TestCredentialMint_RevokedSessionRecordsItsOwnCause(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, _ := sdkDownloadApp(t, f, f.family)
	status, _ := doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assertRefusedRow(t, f, "sdk_download")
	assert.Equal(t, "sessionRevoked", f.audit.rows[0].Metadata["cause"])
}

// C1, C3: creating an API key takes a sign-in's access token, as the three
// routes above do. An access token minted from an SDK-download token is
// refused with the same 401 before the body is read: no agent is read, no key
// is created, and the refusal is recorded once under api_key_create with the
// cause noSignIn. A sign-in's access token creates the key. The route runs
// behind the real auth and member middleware, as it is mounted.
func TestCredentialMint_APIKeyCreationNeedsASignIn(t *testing.T) {
	logs := captureLog(t)
	f := newSignInMintFixture(t)
	var calls []string
	keys := &mintAPIKeyRepo{calls: &calls}
	agents := mintAgentRepo(&mintFixture{orgID: f.orgID}, &calls)
	h := NewAPIKeyHandler(application.NewAPIKeyService(keys, agents), application.NewAuditService(f.audit), keys, f.svc)
	app := fiber.New()
	app.Use(middleware.AuthMiddleware(f.svc))
	app.Post("/api-keys", middleware.MemberMiddleware(), h.CreateAPIKey)
	body := fmt.Sprintf(`{"agentId":%q,"name":"sign-in-key"}`, uuid.NewString())

	t.Run("C1 an SDK-download token's access token is refused", func(t *testing.T) {
		calls, f.audit.rows = nil, nil
		logs.Reset()
		status, resp := bearerRequest(t, app, "POST", "/api-keys", f.sdkAccess, body)
		assert.Equal(t, fiber.StatusUnauthorized, status)
		// the body is not printed: a route that was not refused answers with the key
		assert.True(t, strings.Contains(resp, mintRefusal), "the body is the refresh route's refusal")
		assert.Empty(t, calls, "no agent is read and no key is created")
		refused := refusedRows(f.audit.rows)
		require.Len(t, refused, 1, "one credential_mint_refused row")
		assert.Equal(t, "noSignIn", refused[0].Metadata["cause"])
		assert.Equal(t, "api_key_create", refused[0].Metadata["route"])
		assert.Equal(t, f.sdkFamily, refused[0].Metadata["familyId"], "the row names the download's family")
		require.NotNil(t, refused[0].UserID)
		assert.Equal(t, f.userID, *refused[0].UserID)
		logged := logs.String()
		assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused route=api_key_create cause=noSignIn "))
		assert.NotContains(t, logged, "eyJ", "no token in the log")
	})

	t.Run("C3 a sign-in's access token creates the key", func(t *testing.T) {
		calls, f.audit.rows = nil, nil
		status, _ := bearerRequest(t, app, "POST", "/api-keys", f.signInAccess, body)
		assert.Equal(t, fiber.StatusCreated, status)
		assert.Equal(t, 1, countOf(calls, "APIKeyRepository.Create"), "calls: %v", calls)
		assert.Empty(t, refusedRows(f.audit.rows), "no refusal is recorded")
	})

	t.Run("an access token a sign-in's refresh mints creates the key", func(t *testing.T) {
		_, refresh, err := f.svc.GenerateTokenPair(f.userID.String(), f.orgID.String(), "signin@example.com", "admin")
		require.NoError(t, err)
		refreshed, _, err := f.svc.RefreshTokenPair(refresh, "signin@example.com", "admin")
		require.NoError(t, err)
		calls, f.audit.rows = nil, nil
		status, _ := bearerRequest(t, app, "POST", "/api-keys", refreshed, body)
		assert.Equal(t, fiber.StatusCreated, status)
		assert.Equal(t, 1, countOf(calls, "APIKeyRepository.Create"), "calls: %v", calls)
		assert.Empty(t, refusedRows(f.audit.rows), "no refusal is recorded")
	})
}

// A sign-in's access token whose session was revoked is refused at an API
// key's creation with the cause sessionRevoked, so the two refusals stay apart
// in the record; an access token that carries no session (and so no sign-in
// time) is not checked and creates the key.
func TestCredentialMint_APIKeyCreationRefusalCauses(t *testing.T) {
	var cell sessionRoute
	for _, r := range sessionRoutes() {
		if r.label == "api_key_create" {
			cell = r
		}
	}
	require.Equal(t, "api_key_create", cell.label)

	t.Run("a revoked session", func(t *testing.T) {
		f := newMintFixture(t, &rotationStore{}, false)
		f.revokeFamily()
		var calls []string
		status, _ := sendSessionRoute(t, f, f.family, cell, &calls)
		assert.Equal(t, fiber.StatusUnauthorized, status)
		assert.Empty(t, calls, "no agent is read and no key is created")
		assertRefusedRow(t, f, "api_key_create")
		assert.Equal(t, "sessionRevoked", f.audit.rows[0].Metadata["cause"])
	})

	t.Run("no session and no sign-in time", func(t *testing.T) {
		f := newMintFixture(t, &rotationStore{}, false)
		var calls []string
		app := fiber.New()
		app.Post("/api-keys", func(c fiber.Ctx) error {
			c.Locals("user_id", f.userID)
			c.Locals("organization_id", f.orgID)
			c.Locals("role", "admin")
			return cell.handler(f, &calls)(c)
		})
		status, _ := doRequest(t, app, "POST", "/api-keys", cell.body)
		assert.Equal(t, fiber.StatusCreated, status)
		assert.Equal(t, 1, countOf(calls, cell.mint), "calls: %v", calls)
		assert.Empty(t, refusedRows(f.audit.rows))
	})
}
