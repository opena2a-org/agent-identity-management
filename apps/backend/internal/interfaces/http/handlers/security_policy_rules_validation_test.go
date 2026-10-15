package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// policyRulesCase is one rules object sent to the create and update routes.
type policyRulesCase struct {
	name        string
	policyType  string
	rules       string
	wantStatus  int    // the create route's status; the update route answers 200 where this is 201
	wantMessage string // a fragment of the 400 error; "" when the rules are accepted
}

var policyRulesCases = []policyRulesCase{
	{name: "allowlist floor on the [0,1] scale", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":0.7}`, wantStatus: fiber.StatusCreated},
	{name: "allowlist floor of 1", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":1}`, wantStatus: fiber.StatusCreated},
	{name: "allowlist with no floor", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":0}`, wantStatus: fiber.StatusCreated},
	{name: "allowlist floor written as a percentage", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":70}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.minTrustScore must be from 0 to 1"},
	{name: "allowlist floor below 0", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":-0.1}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.minTrustScore must be from 0 to 1"},
	{name: "allowlist floor written as a string", policyType: "mcp_allowlist", rules: `{"allowedDomains":["*"],"minTrustScore":"50"}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.minTrustScore cannot hold a JSON string"},
	{name: "blocklist domain written as a string", policyType: "mcp_blocklist", rules: `{"blockedDomains":"evil.example"}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.blockedDomains cannot hold a JSON string"},
	{name: "capabilities count written as a string", policyType: "mcp_capabilities", rules: `{"maxCapabilityCount":"3"}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.maxCapabilityCount cannot hold a JSON string"},
	{name: "unverified flag written as a string", policyType: "mcp_unverified", rules: `{"requireManualApproval":"yes"}`, wantStatus: fiber.StatusBadRequest, wantMessage: "rules.requireManualApproval cannot hold a JSON string"},
	// The rules of the non-MCP types are read key by key by SecurityPolicyService and are not
	// checked here.
	{name: "non-MCP rules are stored as sent", policyType: "trust_score_low", rules: `{"minTrustScore":50}`, wantStatus: fiber.StatusCreated},
}

func policyRequestBody(policyType, rules string) string {
	return `{"name":"Floor check","description":"","policyType":"` + policyType +
		`","enforcementAction":"alert_only","severityThreshold":"medium","rules":` + rules +
		`,"appliesTo":"all","isEnabled":false,"priority":100}`
}

func newPolicyRulesApp(orgID uuid.UUID, repo *mockSecurityPolicyRepoForCrossTenant) *fiber.App {
	handler := NewSecurityPolicyHandler(application.NewSecurityPolicyService(repo, nil, nil))
	withOrg := func(next fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("organization_id", orgID)
			c.Locals("user_id", uuid.New())
			return next(c)
		}
	}
	app := fiber.New()
	app.Post("/api/v1/admin/security-policies", withOrg(handler.CreatePolicy))
	app.Put("/api/v1/admin/security-policies/:id", withOrg(handler.UpdatePolicy))
	return app
}

func sendPolicyRequest(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// TestSecurityPolicyHandler_RefusesMCPRulesTheEvaluatorCannotRead posts each rules object to
// POST /api/v1/admin/security-policies and PUT /api/v1/admin/security-policies/:id. The rules
// of an mcp_* policy must decode into the rule struct MCPPolicyEvaluator reads, and an
// mcp_allowlist minTrustScore must lie on the [0,1] scale of an MCP server's trust score: a
// floor of 70 sits above every trust score. A refused request stores nothing.
func TestSecurityPolicyHandler_RefusesMCPRulesTheEvaluatorCannotRead(t *testing.T) {
	for _, tc := range policyRulesCases {
		t.Run("create: "+tc.name, func(t *testing.T) {
			orgID := uuid.New()
			repo := &mockSecurityPolicyRepoForCrossTenant{}
			app := newPolicyRulesApp(orgID, repo)

			status, body := sendPolicyRequest(t, app, "POST", "/api/v1/admin/security-policies",
				policyRequestBody(tc.policyType, tc.rules))

			assert.Equal(t, tc.wantStatus, status, body)
			if tc.wantMessage != "" {
				assert.Contains(t, body, tc.wantMessage)
				assert.False(t, repo.mutationCalled, "a refused policy must not be stored")
				return
			}
			assert.True(t, repo.mutationCalled, "an accepted policy must be stored")
			var stored domain.SecurityPolicy
			require.NoError(t, json.Unmarshal([]byte(body), &stored))
			var sent map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.rules), &sent))
			assert.Equal(t, sent, stored.Rules, "accepted rules are stored as sent")
		})

		t.Run("update: "+tc.name, func(t *testing.T) {
			orgID := uuid.New()
			policyID := uuid.New()
			repo := &mockSecurityPolicyRepoForCrossTenant{policy: &domain.SecurityPolicy{
				ID:             policyID,
				OrganizationID: orgID,
				Name:           "Floor check",
				PolicyType:     domain.PolicyType(tc.policyType),
				Rules:          map[string]interface{}{},
			}}
			app := newPolicyRulesApp(orgID, repo)

			status, body := sendPolicyRequest(t, app, "PUT", "/api/v1/admin/security-policies/"+policyID.String(),
				policyRequestBody(tc.policyType, tc.rules))

			if tc.wantMessage != "" {
				assert.Equal(t, fiber.StatusBadRequest, status, body)
				assert.Contains(t, body, tc.wantMessage)
				assert.False(t, repo.mutationCalled, "a refused update must not be stored")
				return
			}
			assert.Equal(t, fiber.StatusOK, status, body)
			assert.True(t, repo.mutationCalled, "an accepted update must be stored")
		})
	}
}
