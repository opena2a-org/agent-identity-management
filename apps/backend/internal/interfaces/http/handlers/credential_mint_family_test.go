package handlers

import (
	"context"
	"encoding/json"
	"errors"
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
)

// The three credential-minting routes refuse an acting access token whose
// session (family) was revoked: an SDK download, a device sign-in approval
// and an SDK credential recovery. The read is uncached; a confirmed refusal
// records one credential_mint_refused audit row and one SECURITY line with
// identifiers only; a store that did not answer records nothing and follows
// the fail-open setting; a token with no family (minted before families
// existed) is not checked.

const mintRefusal = "Token has been revoked or is invalid"

// mintFixture wires a jwt service with a revocation store, a recording audit
// repository, and a middleware stand-in that sets the acting identifiers.
type mintFixture struct {
	svc    *auth.JWTService
	store  *rotationStore
	audit  *familyAuditRepo
	userID uuid.UUID
	orgID  uuid.UUID
	family string
	jti    string
}

func newMintFixture(t *testing.T, store *rotationStore, failOpen bool) *mintFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	svc := auth.NewJWTService()
	if store != nil {
		svc.SetRevoker(auth.NewTokenRevoker(store, failOpen))
	}
	return &mintFixture{svc: svc, store: store, audit: &familyAuditRepo{}, userID: uuid.New(), orgID: uuid.New(), family: uuid.New().String(), jti: uuid.New().String()}
}

// acting sets what AuthMiddleware sets: the principal, and the acting token's family and id.
func (f *mintFixture) acting(family string, next fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("user_id", f.userID)
		c.Locals("organization_id", f.orgID)
		c.Locals("email", "mint@example.com")
		c.Locals("role", "admin")
		c.Locals("sid", family)
		c.Locals("jti", f.jti)
		return next(c)
	}
}

func (f *mintFixture) revokeFamily() { f.store.Set(context.Background(), "revoked:fam:"+f.family, "1", time.Hour) }

func (f *mintFixture) familyWrites() int { return familyKeyWrites(f.store) }

func doRequest(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "mint-cell/1")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func assertRefusedRow(t *testing.T, f *mintFixture, route string) {
	t.Helper()
	require.Len(t, f.audit.rows, 1, "exactly one row for the refused mint")
	row := f.audit.rows[0]
	assert.Equal(t, domain.AuditActionCredentialMintRefused, row.Action)
	require.NotNil(t, row.UserID)
	assert.Equal(t, f.userID, *row.UserID)
	assert.Equal(t, f.orgID, row.OrganizationID)
	assert.Equal(t, "mint-cell/1", row.UserAgent)
	assert.Equal(t, f.family, row.Metadata["familyId"])
	assert.Equal(t, f.jti, row.Metadata["jti"])
	assert.Equal(t, route, row.Metadata["route"])
}

// ---- S: GET /sdk/download -------------------------------------------------

func sdkDownloadApp(t *testing.T, f *mintFixture, family string) (*fiber.App, *rotationSDKRepo) {
	t.Helper()
	// the zip is built from SDK_BASE_DIR/<sdk>; a two-file fixture stands in for the SDK tree
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "python", "aim_sdk"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "VERSION"), []byte("0.0.0-fixture\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "aim_sdk", "__init__.py"), []byte("# fixture\n"), 0o644))
	t.Setenv("SDK_BASE_DIR", dir)
	repo := &rotationSDKRepo{}
	h := NewSDKHandler(f.svc, repo, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Get("/sdk/download", f.acting(family, h.DownloadSDK))
	return app, repo
}

// S1: a revoked session cannot download an SDK; the refusal is recorded once, identifiers only.
func TestSDKDownload_RefusesARevokedSession(t *testing.T) {
	logBuf := captureLog(t)
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, repo := sdkDownloadApp(t, f, f.family)
	status, body := doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, mintRefusal)
	assert.Empty(t, repo.created, "no SDK token row is created")
	assertRefusedRow(t, f, "sdk_download")
	logged := logBuf.String()
	assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused"))
	assert.Contains(t, logged, "family="+f.family, "control: the family id is in the line")
	assert.NotContains(t, logged, "eyJ", "no token in the log")
	assert.NotContains(t, marshalled(t, f.audit.rows[0].Metadata), "eyJ", "no token in the row")
}

// S2 (control): a live session downloads.
func TestSDKDownload_LiveSessionDownloads(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	app, _ := sdkDownloadApp(t, f, f.family)
	status, _ := doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Empty(t, f.audit.rows)
}

// S3/S4: a store that does not answer records nothing and follows the fail-open setting.
func TestSDKDownload_UnansweredStoreFollowsFailOpen(t *testing.T) {
	logBuf := captureLog(t)
	closed := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, false)
	app, _ := sdkDownloadApp(t, closed, closed.family)
	status, body := doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusUnauthorized, status, "fail-closed refuses")
	assert.Contains(t, body, mintRefusal)
	assert.Empty(t, closed.audit.rows)
	assert.Equal(t, 0, closed.familyWrites())
	assert.NotContains(t, logBuf.String(), "SECURITY")

	open := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, true)
	app, _ = sdkDownloadApp(t, open, open.family)
	status, _ = doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusOK, status, "fail-open allows")
	assert.Empty(t, open.audit.rows)
}

// S5: an access token with no family (minted before families existed) is not checked.
func TestSDKDownload_NoFamilyIsNotChecked(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, _ := sdkDownloadApp(t, f, "")
	status, _ := doRequest(t, app, "GET", "/sdk/download", "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Empty(t, f.audit.rows)
}

// ---- D: POST /oauth/device/approve ---------------------------------------

type mintDeviceRepo struct {
	code     *domain.DeviceCode
	approved []string
}

func (r *mintDeviceRepo) Create(context.Context, *domain.DeviceCode) error { return nil }
func (r *mintDeviceRepo) GetByDeviceCode(context.Context, string) (*domain.DeviceCode, error) {
	return r.code, nil
}
func (r *mintDeviceRepo) GetByUserCode(_ context.Context, u string) (*domain.DeviceCode, error) {
	if r.code == nil || r.code.UserCode != u {
		return nil, errors.New("not found")
	}
	return r.code, nil
}
func (r *mintDeviceRepo) Approve(_ context.Context, u string, _ uuid.UUID, _ uuid.UUID) error {
	r.approved = append(r.approved, u)
	return nil
}
func (r *mintDeviceRepo) Deny(context.Context, string) error           { return nil }
func (r *mintDeviceRepo) CleanupExpired(context.Context) (int64, error) { return 0, nil }

func deviceApproveApp(f *mintFixture, family string) (*fiber.App, *mintDeviceRepo) {
	repo := &mintDeviceRepo{code: &domain.DeviceCode{ID: uuid.New(), DeviceCode: "dev", UserCode: "ABCDEFGH", ClientID: "aim-sdk", ExpiresAt: time.Now().Add(10 * time.Minute), Status: domain.DeviceCodeStatusPending, CreatedAt: time.Now()}}
	svc := application.NewDeviceAuthService(repo, f.svc, nil, "http://localhost:3000")
	h := NewDeviceAuthHandler(svc, f.svc, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Post("/oauth/device/approve", f.acting(family, h.ApproveDevice))
	return app, repo
}

// D1: a revoked session cannot approve a device sign-in; the code stays pending.
func TestDeviceApprove_RefusesARevokedSession(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, repo := deviceApproveApp(f, f.family)
	status, body := doRequest(t, app, "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, mintRefusal)
	assert.Empty(t, repo.approved, "the device code is not approved")
	assertRefusedRow(t, f, "device_approve")
}

// D2 (control): a live session approves.
func TestDeviceApprove_LiveSessionApproves(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	app, repo := deviceApproveApp(f, f.family)
	status, _ := doRequest(t, app, "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, []string{"ABCDEFGH"}, repo.approved)
	assert.Empty(t, f.audit.rows)
}

// D3/D4: an unanswered store follows the fail-open setting and records nothing.
func TestDeviceApprove_UnansweredStoreFollowsFailOpen(t *testing.T) {
	closed := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, false)
	app, repo := deviceApproveApp(closed, closed.family)
	status, _ := doRequest(t, app, "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, repo.approved)
	assert.Empty(t, closed.audit.rows)

	open := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, true)
	app, repo = deviceApproveApp(open, open.family)
	status, _ = doRequest(t, app, "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, repo.approved, 1)
	assert.Empty(t, open.audit.rows)
}

// D5: no family, not checked.
func TestDeviceApprove_NoFamilyIsNotChecked(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, repo := deviceApproveApp(f, "")
	status, _ := doRequest(t, app, "POST", "/oauth/device/approve", `{"userCode":"ABCD-EFGH"}`)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, repo.approved, 1)
}

// ---- V: POST /auth/sdk/recover ---------------------------------------------

func sdkRecoverApp(f *mintFixture, family string) (*fiber.App, *rotationSDKRepo, string) {
	repo := &rotationSDKRepo{token: revokedSDKToken(f.userID, f.orgID)}
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(f.userID, f.orgID, domain.RoleAdmin, "mint@example.com"), nil
	}}
	h := NewSDKTokenRecoveryHandler(application.NewSDKTokenService(repo), f.svc, users, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Post("/auth/sdk/recover", f.acting(family, h.RecoverRevokedToken))
	old, _ := f.svc.GenerateSDKRefreshToken(f.userID.String(), f.orgID.String(), "mint@example.com", "admin")
	return app, repo, old
}

func recoverBody(old string) string {
	b, _ := json.Marshal(map[string]string{"oldRefreshToken": old})
	return string(b)
}

// V1: a revoked session cannot recover an SDK credential; no pair is minted.
func TestSDKRecover_RefusesARevokedSession(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, repo, old := sdkRecoverApp(f, f.family)
	status, body := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, mintRefusal)
	assert.Empty(t, repo.created, "no recovered token row is created")
	assertRefusedRow(t, f, "sdk_recover")
}

// V2 (control): a live session recovers.
func TestSDKRecover_LiveSessionRecovers(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	app, repo, old := sdkRecoverApp(f, f.family)
	status, _ := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, repo.created, 1)
	assert.Empty(t, f.audit.rows)
}

// V3/V4: an unanswered store follows the fail-open setting and records nothing.
func TestSDKRecover_UnansweredStoreFollowsFailOpen(t *testing.T) {
	closed := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, false)
	app, repo, old := sdkRecoverApp(closed, closed.family)
	status, _ := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, repo.created)
	assert.Empty(t, closed.audit.rows)

	open := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, true)
	app, repo, old = sdkRecoverApp(open, open.family)
	status, _ = doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, repo.created, 1)
	assert.Empty(t, open.audit.rows)
}

// V5: no family, the existing behaviour.
func TestSDKRecover_NoFamilyIsNotChecked(t *testing.T) {
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	app, repo, old := sdkRecoverApp(f, "")
	status, _ := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, repo.created, 1)
}
