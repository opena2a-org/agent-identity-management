package handlers

import (
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
