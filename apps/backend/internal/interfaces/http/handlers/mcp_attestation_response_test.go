package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPAttestationVerifier is a captured-flag fake of the
// MCPAttestationVerifier interface used by the AttestMCP handler tests
// to inject specific sentinel errors and observe the response mapping.
type mockMCPAttestationVerifier struct {
	VerifyAndRecordAttestationFunc func(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error)
	calls                          int
}

func (m *mockMCPAttestationVerifier) VerifyAndRecordAttestation(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error) {
	m.calls++
	if m.VerifyAndRecordAttestationFunc != nil {
		return m.VerifyAndRecordAttestationFunc(ctx, mcpServerID, req)
	}
	return nil, nil
}

// A3d-vii.b/AttestMCP P0: a service-layer ErrAttestationFailed (the
// cross-tenant sentinel — also fires for unknown agent, unknown MCP,
// org-mismatch, unverified-agent, low-trust-block) MUST map to a 403
// with a fixed-body response and NO err.Error() echo. The exploit this
// closes (per todo 2026-05-21-attestmcp-body-agentid-class2-idor.md):
// an attacker with a valid Ed25519 token for org A could probe victim
// agents in org B by sending a body AgentID and observing distinct
// error strings (status enum, trust score, agent existence). Both
// sentinels collapse to the same response.
func TestMCPAttestationHandler_AttestMCP_FailedSentinelMapsToFixed403(t *testing.T) {
	mockVerifier := &mockMCPAttestationVerifier{
		VerifyAndRecordAttestationFunc: func(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error) {
			return nil, application.ErrAttestationFailed
		},
	}

	handler := &MCPAttestationHandler{verifier: mockVerifier}

	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	body := `{"attestation":{"agentId":"` + uuid.New().String() + `"},"signature":"sig"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var responseBody map[string]string
	require.NoError(t, json.Unmarshal(bodyBytes, &responseBody))
	assert.Equal(t, "Attestation failed", responseBody["error"])
	assert.Equal(t, "Attestation could not be verified.", responseBody["message"])

	// Critical: no part of the wrapped sentinel text leaks. A response
	// that contained "attestation failed" alone is not enough — the
	// service may wrap with %w to include victim agent status / trust
	// score / UUID. The fixed message above is the entire client-facing
	// payload.
	assert.NotContains(t, string(bodyBytes), "SUSPENDED")
	assert.NotContains(t, string(bodyBytes), "trust")
	assert.NotContains(t, string(bodyBytes), "cannot attest")
}

// A3d-vii.b/AttestMCP P0: a wrapped ErrAttestationFailed (the production
// case — service uses `fmt.Errorf("%w: ...", ErrAttestationFailed)` to
// add server-side log detail) MUST still map to the same fixed 403.
func TestMCPAttestationHandler_AttestMCP_WrappedFailedSentinelMapsToFixed403(t *testing.T) {
	mockVerifier := &mockMCPAttestationVerifier{
		VerifyAndRecordAttestationFunc: func(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error) {
			return nil, errorWrap(application.ErrAttestationFailed, "agent status SUSPENDED, trust 12.3%")
		},
	}

	handler := &MCPAttestationHandler{verifier: mockVerifier}

	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	body := `{"attestation":{"agentId":"` + uuid.New().String() + `"},"signature":"sig"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(bodyBytes), "SUSPENDED")
	assert.NotContains(t, string(bodyBytes), "12.3")
}

// ErrAttestationInvalid (signature/payload validation failures) maps to
// the same fixed 403 — both sentinels collapse to one response so the
// client cannot distinguish "I targeted a victim" from "my signature
// is bad."
func TestMCPAttestationHandler_AttestMCP_InvalidSentinelMapsToFixed403(t *testing.T) {
	mockVerifier := &mockMCPAttestationVerifier{
		VerifyAndRecordAttestationFunc: func(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error) {
			return nil, errorWrap(application.ErrAttestationInvalid, "signature verification failed")
		},
	}

	handler := &MCPAttestationHandler{verifier: mockVerifier}

	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	body := `{"attestation":{"agentId":"` + uuid.New().String() + `"},"signature":"sig"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(bodyBytes), "signature verification")
}

// A non-sentinel error (DB outage, marshaling bug) maps to 500 with the
// same fixed body — never the wrapped error text.
func TestMCPAttestationHandler_AttestMCP_UnknownErrorMapsToFixed500(t *testing.T) {
	mockVerifier := &mockMCPAttestationVerifier{
		VerifyAndRecordAttestationFunc: func(ctx context.Context, mcpServerID uuid.UUID, req *application.AttestMCPRequest) (*application.AttestMCPResponse, error) {
			return nil, errors.New("database connection lost; org_xyz_secret_table")
		},
	}

	handler := &MCPAttestationHandler{verifier: mockVerifier}

	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	body := `{"attestation":{"agentId":"` + uuid.New().String() + `"},"signature":"sig"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(bodyBytes), "org_xyz_secret_table")
	assert.NotContains(t, string(bodyBytes), "database connection")
}

// errorWrap mirrors the fmt.Errorf("%w: %s") pattern the service uses
// to attach server-side log detail to a sentinel without changing the
// sentinel identity for errors.Is.
func errorWrap(sentinel error, detail string) error {
	return wrappedErr{sentinel: sentinel, detail: detail}
}

type wrappedErr struct {
	sentinel error
	detail   string
}

func (e wrappedErr) Error() string { return e.sentinel.Error() + ": " + e.detail }
func (e wrappedErr) Unwrap() error { return e.sentinel }
