package application

import (
	"context"
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

// The alert VerifyCapability raises for each call of an agent that is not
// verified yet, on an organization in monitoring mode. The Quick start in both
// READMEs names it.
const unverifiedAgentMonitoringAlert = "Unverified Agent Action (Monitoring Mode)"

// The reason VerifyCapability refuses a call of an agent that is not verified
// yet with, on an organization in strict mode. The Quick start in both READMEs
// quotes it.
const unverifiedAgentStrictDenial = "Agent not verified - all actions denied"

// pendingAgentVerification runs VerifyCapability for the Quick start's
// my-first-agent while it is still pending (secure() registers it with its own
// key, so it is not verified automatically), holding db:read and calling
// db:read, on an organization in the given enforcement mode. It returns the
// decision and the alerts raised for it.
func pendingAgentVerification(t *testing.T, mode domain.EnforcementMode) (bool, string, []*domain.Alert) {
	t.Helper()

	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockOrgRepo := new(MockOrganizationRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "my-first-agent",
		DisplayName:    "my-first-agent",
		Status:         domain.AgentStatusPending,
		TrustScore:     0.59,
		CreatedAt:      time.Now().Add(-time.Minute),
		UpdatedAt:      time.Now(),
	}

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		alertRepo:      mockAlertRepo,
		capabilityRepo: mockCapabilityRepo,
		orgRepo:        mockOrgRepo,
	}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: "db:read"},
	}, nil)
	mockOrgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: mode,
	}, nil)

	var alerts []*domain.Alert
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).
		Run(func(args mock.Arguments) { alerts = append(alerts, args.Get(0).(*domain.Alert)) }).
		Return(nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, "db:read", "", nil, "")
	require.NoError(t, err)
	return allowed, reason, alerts
}

// A pending agent's call runs on an organization in monitoring mode, the mode
// a new organization starts in, and raises an alert for it; it is refused only
// on an organization in strict mode.
func TestAgentService_VerifyCapability_PendingAgent_FollowsEnforcementMode(t *testing.T) {
	// The verification route lets a pending agent's request reach this
	// decision; only other statuses are refused before it.
	require.True(t, domain.AgentStatusPermitsAuth(domain.AgentStatusPending))

	t.Run("monitoring", func(t *testing.T) {
		allowed, reason, alerts := pendingAgentVerification(t, domain.EnforcementModeMonitoring)
		assert.True(t, allowed, "a pending agent's call runs in monitoring mode")
		assert.Contains(t, reason, "MONITORING mode")
		require.Len(t, alerts, 1, "each call of a pending agent in monitoring mode raises one alert")
		assert.True(t, strings.HasPrefix(alerts[0].Title, unverifiedAgentMonitoringAlert+": "),
			"alert title: %q", alerts[0].Title)
	})

	t.Run("strict", func(t *testing.T) {
		allowed, reason, alerts := pendingAgentVerification(t, domain.EnforcementModeStrict)
		assert.False(t, allowed, "a pending agent's call is refused in strict mode")
		assert.Equal(t, unverifiedAgentStrictDenial, reason)
		assert.Empty(t, alerts)
	})
}

// The Quick start in the root README and in the Python SDK's README says what a
// pending agent's call does in each mode: it runs and raises the alert above on
// a new organization (monitoring mode), and is refused with the reason above
// only on a strict-mode organization.
func TestQuickStartReadmes_StateWhichModeRefusesAPendingAgent(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for _, rel := range []string{"README.md", filepath.Join("sdk", "python", "README.md")} {
		t.Run(rel, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, rel))
			require.NoError(t, err)

			var paragraph string
			for _, p := range strings.Split(string(raw), "\n\n") {
				if strings.HasPrefix(p, "The agent starts `pending`") {
					paragraph = p
					break
				}
			}
			require.NotEmpty(t, paragraph, "%s: the Quick start must say the agent starts pending", rel)

			quoted := "`" + unverifiedAgentStrictDenial + "`"
			require.Contains(t, paragraph, quoted, "%s: the paragraph must quote the strict-mode refusal", rel)
			assert.Contains(t, paragraph, "`"+unverifiedAgentMonitoringAlert+"`",
				"%s: the paragraph must name the alert a pending agent's call raises in monitoring mode", rel)

			for _, sentence := range regexp.MustCompile(`[.!?](\s+|$)`).Split(paragraph, -1) {
				if strings.Contains(sentence, quoted) {
					assert.Contains(t, sentence, "strict-mode organization",
						"%s: the sentence quoting the refusal must say it needs strict mode: %q", rel, sentence)
				}
				if strings.Contains(sentence, "monitoring mode") {
					assert.Contains(t, sentence, "runs",
						"%s: the sentence naming monitoring mode must say a pending agent's call runs: %q", rel, sentence)
				}
			}
		})
	}
}
