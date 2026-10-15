package handlers

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A decision on a capability request that was decided first is answered 409,
// whether the request was approved, auto-approved or rejected. One that is
// gone when the decision reads it again is answered 404, and a read that fails
// is still answered 500. Nothing is written in any of these cases.

// decideCapabilityRequestApp mounts both decision routes over a request that
// the ownership check reads as first and the decision reads as second.
func decideCapabilityRequestApp(t *testing.T, second func(*domain.CapabilityRequestWithDetails) (*domain.CapabilityRequestWithDetails, error)) (*fiber.App, string, *capRequestTestRequestRepo) {
	t.Helper()
	orgID, userID, agentID, requestID := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	agents := &capRequestTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "billing-bot"}, nil
		},
	}
	reads := 0
	requests := &capRequestTestRequestRepo{
		getByID: func(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
			pending := &domain.CapabilityRequestWithDetails{CapabilityRequest: domain.CapabilityRequest{
				ID:             requestID,
				AgentID:        agentID,
				CapabilityType: "file:write",
				Status:         domain.CapabilityRequestStatusPending,
			}}
			reads++
			if reads == 1 {
				return pending, nil
			}
			return second(pending)
		},
		updateStatusCh: make(chan struct{}, 2),
	}
	service := application.NewCapabilityRequestService(requests, nil, agents, nil)
	handler := NewCapabilityRequestHandlers(service, agents)

	app := fiber.New()
	withAdmin := func(route fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("organization_id", orgID)
			c.Locals("user_id", userID)
			return route(c)
		}
	}
	app.Post("/api/v1/admin/capability-requests/:id/approve", withAdmin(handler.ApproveCapabilityRequest))
	app.Post("/api/v1/admin/capability-requests/:id/reject", withAdmin(handler.RejectCapabilityRequest))
	return app, "/api/v1/admin/capability-requests/" + requestID.String(), requests
}

func postDecision(t *testing.T, app *fiber.App, path string) (int, map[string]interface{}) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("POST", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func TestCapabilityRequestDecision_OnADecidedRequestAnswers409(t *testing.T) {
	for _, decision := range []string{"approve", "reject"} {
		for _, status := range []domain.CapabilityRequestStatus{
			domain.CapabilityRequestStatusApproved,
			domain.CapabilityRequestStatusAutoApproved,
			domain.CapabilityRequestStatusRejected,
		} {
			t.Run(decision+" after "+string(status), func(t *testing.T) {
				app, base, requests := decideCapabilityRequestApp(t,
					func(r *domain.CapabilityRequestWithDetails) (*domain.CapabilityRequestWithDetails, error) {
						r.Status = status
						return r, nil
					})

				code, body := postDecision(t, app, base+"/"+decision)

				assert.Equal(t, fiber.StatusConflict, code)
				assert.Equal(t, map[string]interface{}{"error": "capability request is not pending"}, body)
				assert.Empty(t, requests.updateStatusCh, "nothing is written")
			})
		}
	}
}

func TestCapabilityRequestDecision_OnARequestGoneSinceTheOwnershipCheckAnswers404(t *testing.T) {
	for _, decision := range []string{"approve", "reject"} {
		t.Run(decision, func(t *testing.T) {
			app, base, requests := decideCapabilityRequestApp(t,
				func(*domain.CapabilityRequestWithDetails) (*domain.CapabilityRequestWithDetails, error) {
					return nil, repository.ErrCapabilityRequestNotFound
				})

			code, body := postDecision(t, app, base+"/"+decision)

			assert.Equal(t, fiber.StatusNotFound, code)
			assert.Equal(t, map[string]interface{}{"error": "capability request not found"}, body)
			assert.Empty(t, requests.updateStatusCh, "nothing is written")
		})
	}
}

func TestCapabilityRequestDecision_WhenTheReadFailsAnswers500(t *testing.T) {
	for _, decision := range []string{"approve", "reject"} {
		t.Run(decision, func(t *testing.T) {
			app, base, requests := decideCapabilityRequestApp(t,
				func(*domain.CapabilityRequestWithDetails) (*domain.CapabilityRequestWithDetails, error) {
					return nil, errors.New("pq: connection reset by peer")
				})

			code, _ := postDecision(t, app, base+"/"+decision)

			assert.Equal(t, fiber.StatusInternalServerError, code)
			assert.Empty(t, requests.updateStatusCh, "nothing is written")
		})
	}
}
