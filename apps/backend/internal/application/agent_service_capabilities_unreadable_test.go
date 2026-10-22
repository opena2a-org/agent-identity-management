package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// VerifyCapability reads the capabilities granted to the agent before it
// decides. When that read fails, the verification is denied with a fixed
// reason and the read's error is returned to the caller. The reason carries
// none of the error's text: the verification route writes the reason into
// its response body, and the error names database tables and hosts.

func TestAgentService_VerifyCapability_AnUnreadableGrantListIsDeniedWithAFixedReason(t *testing.T) {
	readErr := errors.New(`pq: relation "agent_capabilities" does not exist (host policy-store.internal)`)

	agentRepo := new(MockAgentRepository)
	agentRepo.Test(t)
	capabilityRepo := new(MockCapabilityRepository)
	capabilityRepo.Test(t)
	alertRepo := new(MockAlertRepository)
	alertRepo.Test(t)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "grants-unreadable-agent",
		DisplayName:    "Grants Unreadable Agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.90,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}
	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	capabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(nil, readErr)

	service := &AgentService{agentRepo: agentRepo, capabilityRepo: capabilityRepo, alertRepo: alertRepo}

	allowed, reason, auditID, err := service.VerifyCapability(context.Background(), agent.ID, "file:read", "/tmp/report.csv", nil, "")

	assert.False(t, allowed)
	assert.Equal(t, "Action denied: the capabilities granted to the agent could not be read", reason)
	for _, fragment := range []string{readErr.Error(), "pq:", "agent_capabilities", "does not exist", "policy-store.internal"} {
		assert.NotContains(t, reason, fragment)
	}
	assert.ErrorIs(t, err, readErr, "the read's error is returned for the caller to log")
	assert.NotEqual(t, uuid.Nil, auditID)
	capabilityRepo.AssertNotCalled(t, "CreateViolation", mock.Anything)
	alertRepo.AssertNotCalled(t, "Create", mock.Anything)
}
