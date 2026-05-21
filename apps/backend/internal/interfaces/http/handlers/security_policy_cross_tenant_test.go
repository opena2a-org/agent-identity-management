package handlers

import (
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

// mockSecurityPolicyRepoForCrossTenant implements
// domain.SecurityPolicyRepository for the cross-tenant tests. Only
// GetByID is exercised by the LoadOwned guard; mutation methods
// (Update/Delete) must NOT be called for cross-tenant requests — if
// they are, the test fails via the mutationCalled flag.
type mockSecurityPolicyRepoForCrossTenant struct {
	policy         *domain.SecurityPolicy
	getByIDCalled  bool
	mutationCalled bool
}

func (m *mockSecurityPolicyRepoForCrossTenant) Create(policy *domain.SecurityPolicy) error {
	m.mutationCalled = true
	return nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) GetByID(id uuid.UUID) (*domain.SecurityPolicy, error) {
	m.getByIDCalled = true
	return m.policy, nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) GetByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	return nil, nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) GetActiveByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	return nil, nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) GetByType(orgID uuid.UUID, policyType domain.PolicyType) ([]*domain.SecurityPolicy, error) {
	return nil, nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) Update(policy *domain.SecurityPolicy) error {
	m.mutationCalled = true
	return nil
}
func (m *mockSecurityPolicyRepoForCrossTenant) Delete(id uuid.UUID) error {
	m.mutationCalled = true
	return nil
}

// TestSecurityPolicyHandler_CrossOrg_AllPathIDRoutesReturn404 asserts
// that GetPolicy / UpdatePolicy / DeletePolicy / TogglePolicy ALL
// return 404 for a cross-tenant request and do NOT mutate the victim
// org's policy.
//
// Pre-fix all four handlers accepted any policy UUID and called the
// service directly with no org check. The fix wraps each in
// LoadOwned against policyService.GetPolicy. The captured-flag test:
// the policy repo's GetByID is called (LoadOwned loader), and the
// repo's Update/Delete methods MUST NOT be called for the cross-org
// case — the mutationCalled flag asserts this.
func TestSecurityPolicyHandler_CrossOrg_AllPathIDRoutesReturn404(t *testing.T) {
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()
	policyID := uuid.New()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		mount  func(app *fiber.App, h *SecurityPolicyHandler)
	}{
		{
			name:   "GetPolicy",
			method: "GET",
			path:   "/policies/" + policyID.String(),
			mount: func(app *fiber.App, h *SecurityPolicyHandler) {
				app.Get("/policies/:id", h.GetPolicy)
			},
		},
		{
			name:   "UpdatePolicy",
			method: "PUT",
			path:   "/policies/" + policyID.String(),
			body:   `{"name":"x","policyType":"trust","enforcementAction":"alert","severityThreshold":"high","appliesTo":"agents","priority":1}`,
			mount: func(app *fiber.App, h *SecurityPolicyHandler) {
				app.Put("/policies/:id", h.UpdatePolicy)
			},
		},
		{
			name:   "DeletePolicy",
			method: "DELETE",
			path:   "/policies/" + policyID.String(),
			mount: func(app *fiber.App, h *SecurityPolicyHandler) {
				app.Delete("/policies/:id", h.DeletePolicy)
			},
		},
		{
			name:   "TogglePolicy",
			method: "PATCH",
			path:   "/policies/" + policyID.String() + "/toggle",
			body:   `{"isEnabled":true}`,
			mount: func(app *fiber.App, h *SecurityPolicyHandler) {
				app.Patch("/policies/:id/toggle", h.TogglePolicy)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockSecurityPolicyRepoForCrossTenant{
				policy: &domain.SecurityPolicy{
					ID:             policyID,
					OrganizationID: victimOrgID,
					Name:           "victim-policy",
				},
			}
			svc := application.NewSecurityPolicyService(repo, nil, nil)
			handler := &SecurityPolicyHandler{}
			// SecurityPolicyHandler is a thin struct with one field;
			// constructor wires policyService. We reach in directly
			// to avoid the test depending on the NewSecurityPolicyHandler
			// constructor signature.
			*handler = SecurityPolicyHandler{}
			handlerWithSvc := NewSecurityPolicyHandler(svc)

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Locals("organization_id", callerOrgID)
				c.Locals("user_id", uuid.New())
				return c.Next()
			})
			tc.mount(app, handlerWithSvc)

			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			resp, err := app.Test(r)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"cross-org policy request must return 404 (existence-secrecy)")
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body))
			assert.NotContains(t, string(body), victimOrgID.String(), "must not echo victim org UUID")
			assert.NotContains(t, string(body), "victim-policy", "must not echo victim policy name")

			assert.True(t, repo.getByIDCalled, "GetByID must be called to perform the ownership check")
			assert.False(t, repo.mutationCalled,
				"Update/Delete on the repo must NOT be called for a cross-org request — security regression")
		})
	}
}
