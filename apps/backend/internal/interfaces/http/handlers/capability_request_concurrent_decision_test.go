package handlers

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
)

// Two administrators deciding one pending capability request at once both read
// it as pending. The status write is conditional on the request still being
// pending, so the later decision's write finds it decided: that decision
// answers 409, as a decision on a request read as decided does, and grants
// nothing.

// decidedMeanwhileRequestRepo reads the request as pending, and its status
// write answers writeErr, as the repository does for a request decided by
// another decision since that read, or deleted since.
type decidedMeanwhileRequestRepo struct {
	domain.CapabilityRequestRepository
	request  *domain.CapabilityRequestWithDetails
	writeErr error
	writes   []domain.CapabilityRequestStatus
}

func (r *decidedMeanwhileRequestRepo) GetByID(uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
	read := *r.request
	return &read, nil
}

func (r *decidedMeanwhileRequestRepo) UpdateStatus(_ uuid.UUID, status domain.CapabilityRequestStatus, _ uuid.UUID) error {
	r.writes = append(r.writes, status)
	return r.writeErr
}

// grantRecorder records the capabilities granted.
type grantRecorder struct {
	domain.CapabilityRepository
	granted []*domain.AgentCapability
}

func (g *grantRecorder) CreateCapability(c *domain.AgentCapability) error {
	g.granted = append(g.granted, c)
	return nil
}

// postDecisionWhoseWriteFails posts decision ("approve" or "reject") for a
// request read as pending whose status write answers writeErr.
func postDecisionWhoseWriteFails(t *testing.T, decision string, writeErr error) (int, map[string]interface{}, *decidedMeanwhileRequestRepo, *grantRecorder) {
	t.Helper()
	orgID, userID, agentID, requestID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	agents := &capRequestTestAgentRepo{
		getByID: func(uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "billing-bot"}, nil
		},
	}
	requests := &decidedMeanwhileRequestRepo{
		request: &domain.CapabilityRequestWithDetails{CapabilityRequest: domain.CapabilityRequest{
			ID:             requestID,
			AgentID:        agentID,
			CapabilityType: "file:write",
			Status:         domain.CapabilityRequestStatusPending,
		}},
		writeErr: writeErr,
	}
	grants := &grantRecorder{}
	handler := NewCapabilityRequestHandlers(application.NewCapabilityRequestService(requests, grants, agents, nil), agents)

	route := handler.ApproveCapabilityRequest
	if decision == "reject" {
		route = handler.RejectCapabilityRequest
	}
	app := fiber.New()
	app.Post("/api/v1/admin/capability-requests/:id/"+decision, func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return route(c)
	})
	code, body := postDecision(t, app, "/api/v1/admin/capability-requests/"+requestID.String()+"/"+decision)
	return code, body, requests, grants
}

var capabilityDecisions = map[string]domain.CapabilityRequestStatus{
	"approve": domain.CapabilityRequestStatusApproved,
	"reject":  domain.CapabilityRequestStatusRejected,
}

func TestCapabilityRequestDecision_LosingAConcurrentDecisionAnswers409AndGrantsNothing(t *testing.T) {
	for decision, status := range capabilityDecisions {
		t.Run(decision, func(t *testing.T) {
			code, body, requests, grants := postDecisionWhoseWriteFails(t, decision,
				fmt.Errorf("decide capability request: %w", repository.ErrCapabilityRequestNotPending))

			assert.Equal(t, fiber.StatusConflict, code)
			assert.Equal(t, map[string]interface{}{"error": "capability request is not pending"}, body)
			assert.Equal(t, []domain.CapabilityRequestStatus{status}, requests.writes, "one conditional write, and no undo")
			assert.Empty(t, grants.granted, "nothing is granted")
		})
	}
}

// A request deleted between the decision's read and its write is answered as
// one with no row.
func TestCapabilityRequestDecision_ARequestGoneAtTheWriteAnswers404AndGrantsNothing(t *testing.T) {
	for decision, status := range capabilityDecisions {
		t.Run(decision, func(t *testing.T) {
			code, body, requests, grants := postDecisionWhoseWriteFails(t, decision, repository.ErrCapabilityRequestNotFound)

			assert.Equal(t, fiber.StatusNotFound, code)
			assert.Equal(t, map[string]interface{}{"error": "capability request not found"}, body)
			assert.Equal(t, []domain.CapabilityRequestStatus{status}, requests.writes)
			assert.Empty(t, grants.granted, "nothing is granted")
		})
	}
}
