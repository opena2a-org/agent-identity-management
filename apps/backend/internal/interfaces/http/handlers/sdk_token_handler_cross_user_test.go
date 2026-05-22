package handlers

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sdkTokenTestRepo is a minimal domain.SDKTokenRepository stub. Only
// GetByID + Revoke are implemented (the methods RevokeToken touches);
// every other interface method panics if reached.
type sdkTokenTestRepo struct {
	domain.SDKTokenRepository
	getByID    func(id uuid.UUID) (*domain.SDKToken, error)
	revokeCh   chan struct{}
}

func (r *sdkTokenTestRepo) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	return r.getByID(id)
}

func (r *sdkTokenTestRepo) Revoke(id uuid.UUID, reason string) error {
	// Bypass-detection signal: if a cross-user probe ever reaches
	// the Revoke path, this fires and the test fails.
	r.revokeCh <- struct{}{}
	return nil
}

// ===========================
// SDKTokenHandler.RevokeToken existence-oracle regression test.
//
// Pre-fix shape: service returned distinct error strings
// ("token not found: …" vs "unauthorized: token belongs to different
// user") which the handler mapped to 500 vs 403 with distinct body
// messages — an existence side channel that let an attacker
// enumerate token UUIDs and distinguish "exists owned by another
// user" from "doesn't exist".
//
// Post-fix: both branches collapse to ErrSDKTokenNotFound at the
// service layer, and the handler maps the sentinel to a fixed 404
// {"error":"not found"}.
//
// Regression-proofing: captured-flag pattern (per memory
// feedback_fiber_app_test_goroutine_t_fatalf_race). A real
// SDKTokenService is constructed with the stub repo. The stub's
// Revoke method pushes to a channel; the test asserts the channel
// is empty after the cross-user call — proving the gate fires
// BEFORE any mutation.
// ===========================

func TestSDKTokenHandler_RevokeToken_CrossUserReturns404(t *testing.T) {
	callerUserID := uuid.New()
	victimUserID := uuid.New()
	victimTokenID := uuid.New()

	// Buffer 1 so a bypass-write doesn't block; assert empty at end.
	revokeCh := make(chan struct{}, 1)

	repoStub := &sdkTokenTestRepo{
		getByID: func(id uuid.UUID) (*domain.SDKToken, error) {
			// Return a token owned by the VICTIM user, not the caller.
			return &domain.SDKToken{
				ID:     victimTokenID,
				UserID: victimUserID,
			}, nil
		},
		revokeCh: revokeCh,
	}

	svc := application.NewSDKTokenService(repoStub)
	handler := &SDKTokenHandler{sdkTokenService: svc}

	app := fiber.New()
	app.Post("/users/me/sdk-tokens/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("user_id", callerUserID)
		return handler.RevokeToken(c)
	})

	req := httptest.NewRequest("POST", "/users/me/sdk-tokens/"+victimTokenID.String()+"/revoke", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
		"cross-user revoke must return 404, got %d", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	assert.JSONEq(t, `{"error":"not found"}`, bodyStr,
		"cross-user 404 body must be existence-secrecy shape, got %s", bodyStr)
	// Defense-in-depth: the pre-fix leak strings must not appear.
	assert.NotContains(t, bodyStr, "unauthorized",
		"response must not contain pre-fix cross-user leak string")
	assert.NotContains(t, bodyStr, "permission",
		"response must not contain pre-fix 403 'permission' wording")

	// Bypass detection: the sentinel must fire BEFORE the repo's
	// Revoke is called. An empty channel means the mutation never ran.
	select {
	case <-revokeCh:
		t.Fatalf("SDKTokenService.RevokeToken called repo.Revoke on a cross-user probe — ErrSDKTokenNotFound sentinel did not fire")
	default:
		// expected: gate fired at service layer, no mutation
	}
}

func TestSDKTokenHandler_RevokeToken_NotFoundReturns404(t *testing.T) {
	callerUserID := uuid.New()
	nonExistentTokenID := uuid.New()

	revokeCh := make(chan struct{}, 1)

	repoStub := &sdkTokenTestRepo{
		getByID: func(id uuid.UUID) (*domain.SDKToken, error) {
			return nil, assert.AnError
		},
		revokeCh: revokeCh,
	}

	svc := application.NewSDKTokenService(repoStub)
	handler := &SDKTokenHandler{sdkTokenService: svc}

	app := fiber.New()
	app.Post("/users/me/sdk-tokens/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("user_id", callerUserID)
		return handler.RevokeToken(c)
	})

	req := httptest.NewRequest("POST", "/users/me/sdk-tokens/"+nonExistentTokenID.String()+"/revoke", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
		"not-found revoke must return same 404 as cross-user revoke")
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body),
		"not-found 404 body must match cross-user body byte-for-byte (no oracle)")

	select {
	case <-revokeCh:
		t.Fatalf("repo.Revoke called for not-found token")
	default:
	}
}
