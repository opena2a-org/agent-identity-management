package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// verificationScoreWrites runs VerifyCapability calls times for a verified
// agent at a stored 0.82 that holds db:read, requesting capability, on an
// organization in the given enforcement mode with no security policy rows. It
// returns the last decision and every score the calls wrote to the agent row.
// The service carries a trust calculator with no expectations, so a
// recalculation on the request path fails the test.
func verificationScoreWrites(t *testing.T, mode domain.EnforcementMode, capability string, calls int) (bool, string, []float64) {
	t.Helper()

	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockOrgRepo := new(MockOrganizationRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "my-first-agent",
		DisplayName:    "my-first-agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.82,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	policyService := NewSecurityPolicyService(mockPolicyRepo, mockAlertRepo, nil)
	policyService.SetAgentRepository(mockAgentRepo)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		alertRepo:      mockAlertRepo,
		policyService:  policyService,
		capabilityRepo: mockCapabilityRepo,
		orgRepo:        mockOrgRepo,
	}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: "db:read"},
	}, nil)
	// A granted capability is checked against its definition's minimum; the
	// core db:read definition is low-risk and has none.
	mockCapabilityRepo.On("GetCapabilityDefinition", "db", "read", &agent.OrganizationID).
		Return(definitionWithMinimum("db:read", "low"), nil)
	mockOrgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: mode,
	}, nil)
	mockPolicyRepo.On("GetByType", agent.OrganizationID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAlertRepo.On("CreateCoalesced", mock.AnythingOfType("*domain.Alert"),
		mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(true, nil)
	mockAlertRepo.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)

	var written []float64
	mockAgentRepo.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).
		Run(func(args mock.Arguments) { written = append(written, args.Get(1).(float64)) }).
		Return(nil)
	mockTrustScoreRepo.On("UpdateScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)

	var allowed bool
	var reason string
	for i := 0; i < calls; i++ {
		var err error
		allowed, reason, _, err = service.VerifyCapability(context.Background(), agent.ID, capability, "", nil, "")
		require.NoError(t, err)
	}
	mockTrustCalc.AssertNotCalled(t, "Calculate", mock.Anything)
	return allowed, reason, written
}

// A verification request does not recalculate the trust score. A request for a
// granted capability leaves it unchanged; a request outside the agent's grants
// takes a fixed deduction, 0.10 when blocked and 0.05 when let through. These
// are the numbers docs/server-features.md states, and 0.82 to 0.72 is the
// change a denied call shows in the score history.
func TestAgentService_VerifyCapability_DoesNotRecalculateTheTrustScore(t *testing.T) {
	t.Run("granted capability, three calls", func(t *testing.T) {
		allowed, reason, written := verificationScoreWrites(t, domain.EnforcementModeStrict, "db:read", 3)
		assert.True(t, allowed, reason)
		assert.Empty(t, written, "an allowed call writes no trust score")
	})

	t.Run("capability outside the grants, blocked", func(t *testing.T) {
		allowed, reason, written := verificationScoreWrites(t, domain.EnforcementModeStrict, "db:write", 1)
		assert.False(t, allowed)
		assert.Contains(t, reason, "Capability violation blocked")
		require.Len(t, written, 1)
		assert.InDelta(t, 0.72, written[0], 1e-9)
	})

	t.Run("capability outside the grants, let through by monitoring mode", func(t *testing.T) {
		allowed, reason, written := verificationScoreWrites(t, domain.EnforcementModeMonitoring, "db:write", 1)
		assert.True(t, allowed, reason)
		require.Len(t, written, 1)
		assert.InDelta(t, 0.77, written[0], 1e-9)
	})
}

// The root README says when the server's trust score is calculated and links
// docs/server-features.md, which says what a verification request does to it.
// The README said the score "runs on every action" and "Updates on every
// action", which a run of allowed calls disproves.
func TestRootReadme_StatesWhenTheTrustScoreIsRecalculated(t *testing.T) {
	requirePublicRepositoryFiles(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "README.md"))
	require.NoError(t, err)
	readme := string(raw)
	raw, err = os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "server-features.md"))
	require.NoError(t, err)
	guide := string(raw)

	for name, page := range map[string]string{"README.md": readme, "docs/server-features.md": guide} {
		for _, sentence := range regexp.MustCompile(`[.!?](\s+|$)`).Split(page, -1) {
			lower := strings.ToLower(sentence)
			if !strings.Contains(lower, "trust") {
				continue
			}
			for _, claim := range []string{"every action", "real-time"} {
				assert.NotContains(t, lower, claim, "%s claims the trust score changes per action: %q", name, sentence)
			}
		}
	}

	var paragraph string
	for _, p := range strings.Split(guide, "\n\n") {
		if strings.Contains(p, "A verification request (`POST /api/v1/verifications`)") {
			paragraph = p
			break
		}
	}
	require.NotEmpty(t, paragraph, "docs/server-features.md must say what a verification request does to the trust score")
	assert.Contains(t, paragraph, "does not recalculate the score")
	assert.Contains(t, paragraph, "leaves the score unchanged")

	service := &AgentService{}
	blocked := fmt.Sprintf("%.2f", -float64(service.calculateTrustScoreImpact(true))/100)
	letThrough := fmt.Sprintf("%.2f", -float64(service.calculateTrustScoreImpact(false))/100)
	assert.Contains(t, paragraph, "fixed "+blocked+" if the request is blocked")
	assert.Contains(t, paragraph, letThrough+" if it is let through")

	assert.Contains(t, readme, "or calls `POST /api/v1/agents/:id/trust-score/recalculate`",
		"README.md must name the request that recalculates the score")
	assert.Contains(t, readme, "](docs/server-features.md#trust-gated-capabilities)",
		"README.md must link the passage that says what a verification request does to the score")
}
