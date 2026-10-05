package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// btRegistrar stands in for AgentService and records the org and owner of
// every agent it is asked to create.
type btRegistrar struct {
	mu     sync.Mutex
	agents []*domain.Agent
}

func (r *btRegistrar) CreateAgent(_ context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, _ *uuid.UUID, _ *uuid.UUID, _ string) (*domain.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.agents {
		if a.OrganizationID == orgID && a.Name == req.Name {
			return nil, application.ErrAgentNameExists
		}
	}
	agent := &domain.Agent{
		ID: uuid.New(), OrganizationID: orgID, CreatedBy: userID,
		Name: req.Name, DisplayName: req.DisplayName, AgentType: req.AgentType,
		Status: domain.AgentStatusPending,
	}
	r.agents = append(r.agents, agent)
	return agent, nil
}

func (r *btRegistrar) GetAgentCredentials(_ context.Context, agentID uuid.UUID) (string, string, error) {
	return "pub-" + agentID.String(), "priv-" + agentID.String(), nil
}

func (r *btRegistrar) inOrg(orgID uuid.UUID) []*domain.Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Agent
	for _, a := range r.agents {
		if a.OrganizationID == orgID {
			out = append(out, a)
		}
	}
	return out
}

// btDashboardURL stands in for FRONTEND_URL, an address unrelated to the API
// address the exchange is served from.
const btDashboardURL = "https://console.example.com"

type btRig struct {
	app    *fiber.App
	repo   *mocks.MemoryBootstrapTokenRepository
	agents *btRegistrar
	svc    *application.BootstrapTokenService
}

// newBTRig mounts the handler behind a stand-in session middleware that reads
// the caller's org and user from test headers, the way AuthMiddleware sets
// them from a JWT.
func newBTRig(t *testing.T) *btRig {
	t.Helper()
	rig := &btRig{repo: mocks.NewMemoryBootstrapTokenRepository(), agents: &btRegistrar{}}
	rig.svc = application.NewBootstrapTokenService(rig.repo, rig.agents, nil)
	h := NewBootstrapTokenHandler(rig.svc, btDashboardURL+"/")

	session := func(c fiber.Ctx) error {
		if org, err := uuid.Parse(c.Get("X-Test-Org")); err == nil {
			c.Locals("organization_id", org)
		}
		if user, err := uuid.Parse(c.Get("X-Test-User")); err == nil {
			c.Locals("user_id", user)
		}
		return c.Next()
	}
	rig.app = newTestApp()
	rig.app.Post("/api/v1/onboarding/bootstrap-tokens/exchange", h.Exchange)
	rig.app.Post("/api/v1/onboarding/bootstrap-tokens/revoke", session, h.Revoke)
	rig.app.Post("/api/v1/onboarding/bootstrap-tokens", session, h.Mint)
	return rig
}

func (r *btRig) do(t *testing.T, req *http.Request) (int, map[string]interface{}, http.Header) {
	t.Helper()
	resp, err := r.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]interface{}{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	}
	return resp.StatusCode, out, resp.Header
}

func (r *btRig) mint(t *testing.T, orgID, userID uuid.UUID) MintBootstrapTokenResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens", nil)
	req.Header.Set("X-Test-Org", orgID.String())
	req.Header.Set("X-Test-User", userID.String())
	resp, err := r.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	var out MintBootstrapTokenResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func exchangeRequest(t *testing.T, headerToken string, body map[string]interface{}) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/exchange", reader)
	req.Header.Set("Content-Type", "application/json")
	if headerToken != "" {
		req.Header.Set(BootstrapTokenHeader, headerToken)
	}
	return req
}

func TestBootstrapTokenHandler_MintReturnsThePlaintextOnceAndNoStore(t *testing.T) {
	rig := newBTRig(t)
	orgID, userID := uuid.New(), uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens", nil)
	req.Header.Set("X-Test-Org", orgID.String())
	req.Header.Set("X-Test-User", userID.String())
	status, body, header := rig.do(t, req)

	require.Equal(t, fiber.StatusCreated, status)
	token, _ := body["token"].(string)
	require.NoError(t, domain.ValidateBootstrapTokenFormat(token))
	assert.Equal(t, token[len(domain.BootstrapTokenPrefix):len(domain.BootstrapTokenPrefix)+8], body["displayPrefix"])
	assert.Equal(t, domain.BootstrapTokenScopeAgentsRegister, body["scope"])
	assert.Equal(t, "no-store", header.Get(fiber.HeaderCacheControl))

	expires, err := time.Parse(time.RFC3339Nano, body["expiresAt"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), expires, 5*time.Second)
}

func TestBootstrapTokenHandler_MintRequiresASession(t *testing.T) {
	rig := newBTRig(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens", nil)
	status, _, _ := rig.do(t, req)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, rig.repo.All())
}

func TestBootstrapTokenHandler_ExchangeWithHeaderRegistersInTheTokensOrg(t *testing.T) {
	rig := newBTRig(t)
	orgID, userID := uuid.New(), uuid.New()
	minted := rig.mint(t, orgID, userID)

	status, body, header := rig.do(t, exchangeRequest(t, minted.Token, nil))
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, orgID.String(), body["organizationId"])
	assert.Equal(t, application.BootstrapDefaultAgentName, body["name"])
	assert.NotEmpty(t, body["privateKey"], "server-generated key pair is returned once")
	assert.Equal(t, "no-store", header.Get(fiber.HeaderCacheControl))
	assert.Len(t, rig.agents.inOrg(orgID), 1)
	// The dashboard address comes from the server's configuration, trailing
	// slash trimmed, not from the API address.
	assert.Equal(t, btDashboardURL, body["dashboardUrl"])
}

func TestBootstrapTokenHandler_ExchangeOmitsAnUnconfiguredDashboardURL(t *testing.T) {
	repo := mocks.NewMemoryBootstrapTokenRepository()
	svc := application.NewBootstrapTokenService(repo, &btRegistrar{}, nil)
	minted, err := svc.Mint(context.Background(), uuid.New(), uuid.New(), application.BootstrapRequestMeta{})
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/api/v1/onboarding/bootstrap-tokens/exchange", NewBootstrapTokenHandler(svc, " ").Exchange)

	resp, err := app.Test(exchangeRequest(t, minted.Plaintext, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	body := map[string]interface{}{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotContains(t, body, "dashboardUrl")
}

func TestBootstrapTokenHandler_ExchangeWithBodyToken(t *testing.T) {
	rig := newBTRig(t)
	orgID := uuid.New()
	minted := rig.mint(t, orgID, uuid.New())

	status, body, _ := rig.do(t, exchangeRequest(t, "", map[string]interface{}{
		"bootstrapToken": minted.Token,
		"name":           "support-bot",
		"agentType":      "langchain",
	}))
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, "support-bot", body["name"])
	assert.Equal(t, orgID.String(), body["organizationId"])
}

// A token minted in org A registers in org A whatever the body claims, and
// org B gains nothing.
func TestBootstrapTokenHandler_ExchangeCannotChooseTheOrganization(t *testing.T) {
	rig := newBTRig(t)
	orgA, orgB := uuid.New(), uuid.New()
	userB := uuid.New()
	tokenA := rig.mint(t, orgA, uuid.New())

	req := exchangeRequest(t, tokenA.Token, map[string]interface{}{
		"organizationId": orgB.String(),
		"orgId":          orgB.String(),
		"userId":         userB.String(),
	})
	// Session headers for org B are ignored: the exchange route reads no session.
	req.Header.Set("X-Test-Org", orgB.String())
	req.Header.Set("X-Test-User", userB.String())

	status, body, _ := rig.do(t, req)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, orgA.String(), body["organizationId"])
	assert.Len(t, rig.agents.inOrg(orgA), 1)
	assert.Empty(t, rig.agents.inOrg(orgB))
}

// A user in org B cannot revoke org A's token, and minting in org B leaves it
// alone.
func TestBootstrapTokenHandler_OrgBCannotRevokeOrgAsToken(t *testing.T) {
	rig := newBTRig(t)
	orgA, orgB := uuid.New(), uuid.New()
	userA := uuid.New()
	tokenA := rig.mint(t, orgA, userA)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/revoke", nil)
	req.Header.Set("X-Test-Org", orgB.String())
	req.Header.Set("X-Test-User", userA.String())
	status, body, _ := rig.do(t, req)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(0), body["revoked"])
	rig.mint(t, orgB, userA)

	status, _, _ = rig.do(t, exchangeRequest(t, tokenA.Token, nil))
	assert.Equal(t, fiber.StatusCreated, status)
}

func TestBootstrapTokenHandler_RevokeThenExchangeIsRefused(t *testing.T) {
	rig := newBTRig(t)
	orgID, userID := uuid.New(), uuid.New()
	minted := rig.mint(t, orgID, userID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/revoke", nil)
	req.Header.Set("X-Test-Org", orgID.String())
	req.Header.Set("X-Test-User", userID.String())
	status, body, _ := rig.do(t, req)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(1), body["revoked"])

	status, body, _ = rig.do(t, exchangeRequest(t, minted.Token, nil))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "bootstrap_token_revoked", body["code"])
}

func TestBootstrapTokenHandler_RegenerateRevokesThePreviousToken(t *testing.T) {
	rig := newBTRig(t)
	orgID, userID := uuid.New(), uuid.New()
	first := rig.mint(t, orgID, userID)
	second := rig.mint(t, orgID, userID)

	status, body, _ := rig.do(t, exchangeRequest(t, first.Token, nil))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "bootstrap_token_revoked", body["code"])

	status, _, _ = rig.do(t, exchangeRequest(t, second.Token, nil))
	assert.Equal(t, fiber.StatusCreated, status)
}

func TestBootstrapTokenHandler_SecondExchangeIsRefusedAsUsed(t *testing.T) {
	rig := newBTRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	status, _, _ := rig.do(t, exchangeRequest(t, minted.Token, nil))
	require.Equal(t, fiber.StatusCreated, status)
	status, body, _ := rig.do(t, exchangeRequest(t, minted.Token, map[string]interface{}{"name": "again"}))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "bootstrap_token_used", body["code"])
}

func TestBootstrapTokenHandler_ExpiredTokenIsRefused(t *testing.T) {
	rig := newBTRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())
	rig.svc.SetClock(func() time.Time { return time.Now().Add(domain.BootstrapTokenTTL + time.Second) })

	status, body, _ := rig.do(t, exchangeRequest(t, minted.Token, nil))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "bootstrap_token_expired", body["code"])
}

func TestBootstrapTokenHandler_TokenInTheQueryStringIsRefusedAndRevoked(t *testing.T) {
	for _, key := range []string{"token", "bootstrapToken", "anything"} {
		t.Run(key, func(t *testing.T) {
			rig := newBTRig(t)
			minted := rig.mint(t, uuid.New(), uuid.New())

			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/onboarding/bootstrap-tokens/exchange?"+url.Values{key: {minted.Token}}.Encode(), nil)
			status, body, _ := rig.do(t, req)
			assert.Equal(t, fiber.StatusBadRequest, status)
			assert.Equal(t, "bootstrap_token_in_url", body["code"])
			assert.NotContains(t, body["error"], minted.Token)

			// The exposed token no longer works, even from the header.
			status, body, _ = rig.do(t, exchangeRequest(t, minted.Token, nil))
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, "bootstrap_token_revoked", body["code"])
		})
	}
}

func TestBootstrapTokenHandler_ExchangeRefusals(t *testing.T) {
	rig := newBTRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())
	other, _, _, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)

	cases := []struct {
		name   string
		req    *http.Request
		status int
		code   string
	}{
		{"no token", exchangeRequest(t, "", nil), fiber.StatusUnauthorized, "bootstrap_token_invalid"},
		{"malformed", exchangeRequest(t, "aim_ob_nope", nil), fiber.StatusUnauthorized, "bootstrap_token_invalid"},
		{"unknown", exchangeRequest(t, other, nil), fiber.StatusUnauthorized, "bootstrap_token_invalid"},
		{"header and body differ", exchangeRequest(t, minted.Token, map[string]interface{}{"bootstrapToken": other}), fiber.StatusBadRequest, ""},
		{"bad agent type", exchangeRequest(t, minted.Token, map[string]interface{}{"agentType": "nope"}), fiber.StatusBadRequest, ""},
		{"bad public key", exchangeRequest(t, minted.Token, map[string]interface{}{"publicKey": "AAAA"}), fiber.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body, _ := rig.do(t, tc.req)
			assert.Equal(t, tc.status, status, "body: %v", body)
			if tc.code != "" {
				assert.Equal(t, tc.code, body["code"])
			}
		})
	}

	// None of the refusals consumed the token.
	status, _, _ := rig.do(t, exchangeRequest(t, minted.Token, nil))
	assert.Equal(t, fiber.StatusCreated, status)
}

func TestBootstrapTokenHandler_DuplicateNameIs409AndTheTokenSurvives(t *testing.T) {
	rig := newBTRig(t)
	orgID := uuid.New()
	first := rig.mint(t, orgID, uuid.New())
	status, _, _ := rig.do(t, exchangeRequest(t, first.Token, nil))
	require.Equal(t, fiber.StatusCreated, status)

	second := rig.mint(t, orgID, uuid.New())
	status, _, _ = rig.do(t, exchangeRequest(t, second.Token, nil))
	assert.Equal(t, fiber.StatusConflict, status, "default name already taken in the org")

	status, _, _ = rig.do(t, exchangeRequest(t, second.Token, map[string]interface{}{"name": "my-second-agent"}))
	assert.Equal(t, fiber.StatusCreated, status)
}
