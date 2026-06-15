package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJWTOrgID is the organization_id injected by setupVerificationTestApp on
// the JWT verifications route. Tests asserting same-org access must use this
// value for their mock event.OrganizationID.
var testJWTOrgID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

// ===========================
// VerificationHandler Integration Tests
// ===========================

func setupVerificationTestApp(handler *VerificationHandler) *fiber.App {
	app := fiber.New()

	// JWT verifications routes: inject organization_id to mimic AuthMiddleware.
	jwtGroup := app.Group("/api/v1/verifications")
	jwtGroup.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", testJWTOrgID)
		return c.Next()
	})
	jwtGroup.Get("/:id", handler.GetVerification)
	jwtGroup.Post("/:id/result", handler.SubmitVerificationResult)

	// SDK API routes (signature-authed)
	app.Get("/api/v1/sdk-api/verifications/:id", handler.GetVerificationSDK)
	app.Post("/api/v1/sdk-api/verifications/:id/execution-status", handler.UpdateExecutionStatus)

	// Admin routes - require organization context
	adminGroup := app.Group("/api/v1/admin")
	adminGroup.Use(func(c fiber.Ctx) error {
		orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		c.Locals("user_name", "test-admin")
		return c.Next()
	})
	adminGroup.Get("/verifications/pending", handler.ListPendingVerifications)
	adminGroup.Post("/verifications/:id/approve", handler.ApproveVerification)
	adminGroup.Post("/verifications/:id/deny", handler.DenyVerification)

	return app
}

// ===========================
// GetVerification Tests
// ===========================

func TestVerificationHandler_GetVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()
	orgID := testJWTOrgID // match injected JWT context (defect #160 org-scope)
	agentID := uuid.New()
	now := time.Now()
	verifiedResult := domain.VerificationResultVerified

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: orgID,
				AgentID:        &agentID,
				Status:         domain.VerificationEventStatusSuccess,
				Result:         &verifiedResult,
				TrustScore:     0.85,
				CreatedAt:      now,
			}, nil
		},
	}

	mockOrgRepo := &MockOrganizationRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			return &domain.Organization{
				ID:              id,
				EnforcementMode: domain.EnforcementModeStrict,
			}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		mockOrgRepo,
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response VerificationResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response.ID)
	assert.Equal(t, "approved", response.Status)
	assert.Equal(t, 0.85, response.TrustScore)
	assert.Equal(t, "strict", response.EnforcementMode)
}

func TestVerificationHandler_GetVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_GetVerification_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/verifications/invalid-uuid", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_GetVerification_Denied(t *testing.T) {
	// Arrange
	verificationID := uuid.New()
	orgID := testJWTOrgID // match injected JWT context (defect #160 org-scope)
	now := time.Now()
	deniedResult := domain.VerificationResultDenied
	errorReason := "Insufficient trust score"

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: orgID,
				Status:         domain.VerificationEventStatusFailed,
				Result:         &deniedResult,
				ErrorReason:    &errorReason,
				TrustScore:     0.3,
				CreatedAt:      now,
			}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response VerificationResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "denied", response.Status)
	assert.Equal(t, "Insufficient trust score", response.DenialReason)
}

// ===========================
// Defect #160 — GetVerification (JWT) org-scope guard
// ===========================

func TestVerificationHandler_GetVerification_CrossOrg_404(t *testing.T) {
	// Event belongs to a DIFFERENT org than the JWT caller's; handler must
	// return 404, not the event body — no existence oracle.
	verificationID := uuid.New()
	foreignOrgID := uuid.New()
	require.NotEqual(t, testJWTOrgID, foreignOrgID)
	approved := domain.VerificationResultVerified

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: foreignOrgID,
				Result:         &approved,
				CreatedAt:      time.Now(),
			}, nil
		},
	}
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	// Body must not leak any field that distinguishes "cross-org" from "missing"
	assert.NotContains(t, string(body), foreignOrgID.String())
	assert.NotContains(t, string(body), verificationID.String())
}

// ===========================
// Defect #160 — GetVerificationSDK (signature-authed, agent-scoped)
// ===========================

// sdkSignGET produces the canonical signed-message bytes for the SDK GET route.
// Must match the backend canonical form:
//
//	GET\n/api/v1/sdk-api/verifications/<id>\n<agent_id>\n<timestamp>
func sdkSignGET(t *testing.T, priv ed25519.PrivateKey, verificationID, agentID uuid.UUID, ts int64) string {
	t.Helper()
	msg := fmt.Sprintf("GET\n/api/v1/sdk-api/verifications/%s\n%s\n%d",
		verificationID.String(), agentID.String(), ts)
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(msg)))
}

func TestVerificationHandler_GetVerificationSDK_MissingHeaders_401(t *testing.T) {
	verificationID := uuid.New()
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_BadTimestamp_401(t *testing.T) {
	verificationID := uuid.New()
	agentID := uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, PublicKey: &pubB64}, nil
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	stale := time.Now().Add(-30 * time.Minute).Unix() // way past the 5-min window
	sig := sdkSignGET(t, priv, verificationID, agentID, stale)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", agentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(stale, 10))
	req.Header.Set("X-AIM-Signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_InvalidSignature_401(t *testing.T) {
	verificationID := uuid.New()
	agentID := uuid.New()
	pub, _, err := ed25519.GenerateKey(rand.Reader) // registered key
	require.NoError(t, err)
	_, wrongPriv, err := ed25519.GenerateKey(rand.Reader) // signs with a DIFFERENT key
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, PublicKey: &pubB64}, nil
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	wrongSig := sdkSignGET(t, wrongPriv, verificationID, agentID, ts)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", agentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", wrongSig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_UnknownAgent_401(t *testing.T) {
	verificationID := uuid.New()
	agentID := uuid.New()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return nil, fmt.Errorf("not found")
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	sig := sdkSignGET(t, priv, verificationID, agentID, ts)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", agentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_CrossAgent_404(t *testing.T) {
	// Signature is valid for callerAgent, but the verification event belongs
	// to a different agent. Handler must return 404 (not 403) to avoid an
	// existence oracle.
	verificationID := uuid.New()
	callerAgentID := uuid.New()
	ownerAgentID := uuid.New()
	require.NotEqual(t, callerAgentID, ownerAgentID)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, PublicKey: &pubB64}, nil
	}
	approved := domain.VerificationResultVerified
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: uuid.New(),
				AgentID:        &ownerAgentID,
				Result:         &approved,
				CreatedAt:      time.Now(),
			}, nil
		},
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	sig := sdkSignGET(t, priv, verificationID, callerAgentID, ts)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", callerAgentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(body), ownerAgentID.String())
	assert.NotContains(t, string(body), verificationID.String())
}

func TestVerificationHandler_GetVerificationSDK_CrossVerificationReplay_401(t *testing.T) {
	// Sig is valid for verification A but the request is made against
	// verification B. The canonical message includes the verification ID, so
	// the backend reconstructs a different canonical for B and verification
	// fails. This proves leaked sigs cannot be cross-replayed.
	vidA := uuid.New()
	vidB := uuid.New()
	require.NotEqual(t, vidA, vidB)
	agentID := uuid.New()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, PublicKey: &pubB64}, nil
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	sigForA := sdkSignGET(t, priv, vidA, agentID, ts)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", vidB.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", agentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", sigForA)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_CrossAgentHeaderSwap_401(t *testing.T) {
	// Sig generated by agent X's key, but the request claims X-AIM-Agent-ID = Y.
	// Backend will look up Y's pubkey (different from X's) and verification fails.
	verificationID := uuid.New()
	agentX := uuid.New()
	agentY := uuid.New()
	require.NotEqual(t, agentX, agentY)

	pubX, privX, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubY, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubXB64 := base64.StdEncoding.EncodeToString(pubX)
	pubYB64 := base64.StdEncoding.EncodeToString(pubY)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		switch id {
		case agentX:
			return &domain.Agent{ID: id, PublicKey: &pubXB64}, nil
		case agentY:
			return &domain.Agent{ID: id, PublicKey: &pubYB64}, nil
		default:
			return nil, fmt.Errorf("unknown agent")
		}
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	// Sign as agent X, with the canonical naming agent X as caller.
	sigByX := sdkSignGET(t, privX, verificationID, agentX, ts)

	// Now send the request claiming to be agent Y.
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", agentY.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", sigByX)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationHandler_GetVerificationSDK_Valid_200(t *testing.T) {
	verificationID := uuid.New()
	callerAgentID := uuid.New()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	mockAgent := &MockAgentServiceForVerificationImpl{}
	mockAgent.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, PublicKey: &pubB64}, nil
	}
	approved := domain.VerificationResultVerified
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: uuid.New(),
				AgentID:        &callerAgentID, // caller owns this event
				Result:         &approved,
				TrustScore:     0.91,
				CreatedAt:      time.Now(),
			}, nil
		},
	}
	handler := NewVerificationHandlerWithInterfaces(
		mockAgent,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)
	app := setupVerificationTestApp(handler)

	ts := time.Now().Unix()
	sig := sdkSignGET(t, priv, verificationID, callerAgentID, ts)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/sdk-api/verifications/%s", verificationID.String()), nil)
	req.Header.Set("X-AIM-Agent-ID", callerAgentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", sig)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response VerificationResponse
	body, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(body, &response))
	assert.Equal(t, verificationID.String(), response.ID)
	assert.Equal(t, "approved", response.Status)
	assert.Equal(t, 0.91, response.TrustScore)
}

// ===========================
// SubmitVerificationResult Tests
// ===========================

func TestVerificationHandler_SubmitVerificationResult_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result":   "success",
		"reason":   "Action completed",
		"metadata": map[string]interface{}{"key": "value"},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "result_recorded", response["status"])
	assert.Equal(t, "success", response["result"])
}

func TestVerificationHandler_SubmitVerificationResult_Failure(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "failure",
		"reason": "Action failed due to error",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_InvalidResultWithMocks(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "invalid",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "success",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// ListPendingVerifications Tests
// ===========================

func TestVerificationHandler_ListPendingVerifications_Success(t *testing.T) {
	// Arrange
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	agentID := uuid.New()
	agentName := "test-agent"
	action := "read_database"
	now := time.Now()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgIDParam uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return []*domain.VerificationEvent{
				{
					ID:             uuid.New(),
					OrganizationID: orgID,
					AgentID:        &agentID,
					InitiatorName:  &agentName,
					Action:         &action,
					Status:         domain.VerificationEventStatusPending,
					TrustScore:     0.75,
					CreatedAt:      now,
					Metadata: map[string]interface{}{
						"risk_level": "medium",
					},
				},
			}, 1, &domain.VerificationStatusCounts{Pending: 1}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response PendingVerificationListResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Len(t, response.Verifications, 1)
	assert.Equal(t, 1, response.Pagination.Total)
	assert.Equal(t, "test-agent", response.Verifications[0].AgentName)
	assert.Equal(t, "read_database", response.Verifications[0].ActionType)
}

func TestVerificationHandler_ListPendingVerifications_Empty(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return []*domain.VerificationEvent{}, 0, &domain.VerificationStatusCounts{}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_ListPendingVerifications_WithPagination(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			// Verify pagination params
			assert.Equal(t, 5, params.Limit)
			assert.Equal(t, 10, params.Offset) // page 3 with page_size 5 = offset 10
			return []*domain.VerificationEvent{}, 25, &domain.VerificationStatusCounts{}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending?page=3&page_size=5", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response PendingVerificationListResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, 3, response.Pagination.Page)
	assert.Equal(t, 5, response.Pagination.PageSize)
	assert.Equal(t, 25, response.Pagination.Total)
	assert.Equal(t, 5, response.Pagination.TotalPages)
}

func TestVerificationHandler_ListPendingVerifications_ServiceError(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return nil, 0, nil, fmt.Errorf("database error")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// ApproveVerification Tests
// ===========================

func TestVerificationHandler_ApproveVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			assert.Equal(t, verificationID, id)
			assert.Equal(t, domain.VerificationResultVerified, result)
			assert.Contains(t, metadata, "approvedBy")
			assert.Contains(t, metadata, "manualApproval")
			return nil
		},
	}

	mockAuditService := &MockAuditServiceForVerificationImpl{
		LogFunc: func(ctx context.Context, entry *domain.AuditLog) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		mockAuditService,
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Approved after review",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/approve", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "approved", response["status"])
	assert.Equal(t, "test-admin", response["approvedBy"])
}

func TestVerificationHandler_ApproveVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/approve", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_ApproveVerification_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("POST", "/api/v1/admin/verifications/invalid-uuid/approve", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// DenyVerification Tests
// ===========================

func TestVerificationHandler_DenyVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			assert.Equal(t, verificationID, id)
			assert.Equal(t, domain.VerificationResultDenied, result)
			assert.NotNil(t, reason)
			assert.Equal(t, "Security risk detected", *reason)
			assert.Contains(t, metadata, "deniedBy")
			assert.Contains(t, metadata, "manualDenial")
			return nil
		},
	}

	mockAuditService := &MockAuditServiceForVerificationImpl{
		LogFunc: func(ctx context.Context, entry *domain.AuditLog) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		mockAuditService,
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Security risk detected",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "denied", response["status"])
	assert.Equal(t, "test-admin", response["deniedBy"])
	assert.Equal(t, "Security risk detected", response["denialReason"])
}

func TestVerificationHandler_DenyVerification_MissingReason(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_DenyVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Some reason",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// UpdateExecutionStatus Tests
// ===========================

func TestVerificationHandler_UpdateExecutionStatus_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			assert.Equal(t, verificationID, id)
			assert.True(t, executed)
			assert.True(t, strictMode)
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	now := time.Now().Format(time.RFC3339)
	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
		"executedAt": now,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "execution_status_recorded", response["status"])
	assert.Equal(t, true, response["executed"])
	assert.Equal(t, true, response["strictMode"])
}

func TestVerificationHandler_UpdateExecutionStatus_NotExecuted(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			assert.False(t, executed)
			assert.NotNil(t, executionError)
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	execError := "Permission denied"
	reqBody := map[string]interface{}{
		"executed":       false,
		"strictMode":     true,
		"executionError": execError,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_UpdateExecutionStatus_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_UpdateExecutionStatus_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", "/api/v1/sdk-api/verifications/invalid-uuid/execution-status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Helper Function Tests
// ===========================

func TestVerificationHandler_determineAlertSeverity(t *testing.T) {
	handler := &VerificationHandler{}

	tests := []struct {
		name       string
		actionType string
		context    map[string]interface{}
		riskLevel  string
		expected   domain.AlertSeverity
	}{
		{
			name:       "explicit critical risk level",
			actionType: "any_action",
			riskLevel:  "critical",
			expected:   domain.AlertSeverityCritical,
		},
		{
			name:       "explicit high risk level",
			actionType: "any_action",
			riskLevel:  "high",
			expected:   domain.AlertSeverityHigh,
		},
		{
			name:       "delete action is critical",
			actionType: "delete_data",
			riskLevel:  "",
			expected:   domain.AlertSeverityCritical,
		},
		{
			name:       "write action is high",
			actionType: "write_database",
			riskLevel:  "",
			expected:   domain.AlertSeverityHigh,
		},
		{
			name:       "read action is warning",
			actionType: "read_file",
			riskLevel:  "",
			expected:   domain.AlertSeverityWarning,
		},
		{
			name:       "unknown action is info",
			actionType: "unknown_action",
			riskLevel:  "",
			expected:   domain.AlertSeverityInfo,
		},
		{
			name:       "context risk level overrides",
			actionType: "read_file",
			context:    map[string]interface{}{"risk_level": "critical"},
			riskLevel:  "",
			expected:   domain.AlertSeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.determineAlertSeverity(tt.actionType, tt.context, tt.riskLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_calculateVerificationConfidence(t *testing.T) {
	handler := &VerificationHandler{}

	tests := []struct {
		name              string
		agent             *domain.Agent
		status            string
		signatureVerified bool
		publicKeyMatched  bool
		expectedMin       float64
		expectedMax       float64
	}{
		{
			name:              "all verified, high trust",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 1.0},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.95,
			expectedMax:       1.0,
		},
		{
			name:              "signature only, low trust",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 0.5},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  false,
			expectedMin:       0.5,
			expectedMax:       0.7,
		},
		{
			name:              "denied reduces confidence",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 1.0},
			status:            "denied",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.4,
			expectedMax:       0.6,
		},
		{
			name:              "pending agent lower bonus",
			agent:             &domain.Agent{Status: domain.AgentStatusPending, TrustScore: 0.8},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.7,
			expectedMax:       0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.calculateVerificationConfidence(tt.agent, tt.status, tt.signatureVerified, tt.publicKeyMatched)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

func TestIsLowRiskCapability(t *testing.T) {
	tests := []struct {
		capability string
		expected   bool
	}{
		{"db:read", true},
		{"file:read", true},
		{"weather:check", true},
		{"products:search", true},
		{"read_database", true},
		{"delete_data", false},
		{"execute_command", false},
		{"admin_action", false},
		{"unknown_action", false},
	}

	for _, tt := range tests {
		t.Run(tt.capability, func(t *testing.T) {
			result := isLowRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDemoHighRiskCapability(t *testing.T) {
	tests := []struct {
		capability string
		expected   bool
	}{
		{"notification:send", true},
		{"refund:process", true},
		{"send_notification", true},
		{"process_refund", true},
		{"delete_data", false},
		{"read_file", false},
	}

	for _, tt := range tests {
		t.Run(tt.capability, func(t *testing.T) {
			result := isDemoHighRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeVerificationStatus(t *testing.T) {
	tests := []struct {
		status   domain.VerificationEventStatus
		expected string
	}{
		{domain.VerificationEventStatusPending, "pending"},
		{domain.VerificationEventStatusSuccess, "approved"},
		{domain.VerificationEventStatusFailed, "denied"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := normalizeVerificationStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestVerificationHandler_ListPendingVerifications_BulkAgentNames locks the
// N+1 fix: events lacking an inline name resolve through a single bulk
// GetAgentsByIDs call, and the per-event GetAgent must never be called.
func TestVerificationHandler_ListPendingVerifications_BulkAgentNames(t *testing.T) {
	agentA := uuid.New()
	agentB := uuid.New()
	inlineName := "inline-initiator"
	now := time.Now()

	mockEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			events := []*domain.VerificationEvent{
				// Two events for agentA and one for agentB, all missing an
				// inline name -> must be resolved via the bulk query.
				{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentA, CreatedAt: now},
				{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentA, CreatedAt: now},
				{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentB, CreatedAt: now},
				// One event already carries an inline name -> agentID not in bulk set.
				{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentA, InitiatorName: &inlineName, CreatedAt: now},
			}
			return events, len(events), &domain.VerificationStatusCounts{}, nil
		},
	}

	var bulkCalls int
	var bulkRequested []uuid.UUID
	mockAgentService := &MockAgentServiceForVerificationImpl{}
	mockAgentService.GetAgentFunc = func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
		t.Errorf("per-event GetAgent called for %s; list path must use the bulk query", id)
		return nil, nil
	}
	mockAgentService.GetAgentsByIDsFunc = func(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
		bulkCalls++
		bulkRequested = ids
		return []*domain.Agent{
			{ID: agentA, DisplayName: "Agent A"},
			{ID: agentB, Name: "agent-b-fallback"}, // no DisplayName -> Name fallback
		}, nil
	}

	handler := NewVerificationHandlerWithInterfaces(
		mockAgentService,
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Exactly one bulk call, for the two distinct name-missing agents only.
	assert.Equal(t, 1, bulkCalls)
	assert.ElementsMatch(t, []uuid.UUID{agentA, agentB}, bulkRequested)

	var listResp PendingVerificationListResponse
	body, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(body, &listResp))
	require.Len(t, listResp.Verifications, 4)

	for _, v := range listResp.Verifications {
		switch {
		case v.AgentID == agentA.String() && v.AgentName == inlineName:
			// inline-named event keeps its inline name
		case v.AgentID == agentA.String():
			assert.Equal(t, "Agent A", v.AgentName)
		case v.AgentID == agentB.String():
			assert.Equal(t, "agent-b-fallback", v.AgentName)
		default:
			t.Fatalf("unexpected agent id %s", v.AgentID)
		}
	}
}
