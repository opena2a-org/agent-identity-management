//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A registration made through the transition recorder stores its calculated
// trust score with the agent's row and sends trust_score.changed once for it,
// from no score, as a registration without a recorder does.
func TestTransitionRecordedRegistrationSendsItsTrustScore(t *testing.T) {
	f := newTransitionFixture(t)
	svc := f.registrar(t)
	dispatcher := newRecordingDispatcher()
	svc.webhooks = dispatcher
	f.setEnforcement(t, domain.EnforcementModeStrict)

	agent, err := svc.CreateAgent(context.Background(), registration("reg-score-"+uuid.NewString()[:8], ""),
		f.orgID, f.userID, nil, nil, "")
	require.NoError(t, err)
	require.Equal(t, domain.AgentStatusPending, agent.Status, "strict mode verifies by hand")

	got := receiveEvents(t, dispatcher, 2)
	assert.ElementsMatch(t, []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged}, eventNames(got))
	assertRegisteredTrustScore(t, got[domain.WebhookEventTrustScoreChanged], agent, 0.5)
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM agents WHERE id = $1 AND trust_score = 0.5`, agent.ID),
		"the agent's row holds the score the event carries")
}
