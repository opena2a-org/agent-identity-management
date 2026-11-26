package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// sdkShortestRegistration is the body the Python SDK posts for its shortest
// call, secure("my-first-agent", capabilities=["db:read"]): the name and the
// capabilities the caller gave, the display name and description the SDK
// derives from the name, and the public half of the keypair the SDK
// generates, whose private half never leaves the client.
func sdkShortestRegistration(t *testing.T) *CreateAgentRequest {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &CreateAgentRequest{
		Name:         "my-first-agent",
		DisplayName:  "my-first-agent",
		Description:  "Agent my-first-agent registered via AIM SDK",
		AgentType:    domain.AgentTypeAI,
		Version:      "1.0.0",
		PublicKey:    base64.StdEncoding.EncodeToString(publicKey),
		Capabilities: []string{"db:read"},
	}
}

// registerInMode registers req in an organization in mode, with the trust
// calculator answering scores in turn.
func registerInMode(t *testing.T, mode domain.EnforcementMode, req *CreateAgentRequest, scores ...float64) *domain.Agent {
	t.Helper()
	orgID := uuid.New()
	s, agents, history, _ := scoredRegistrar(t, orgID, mode, scores...)
	s.webhooks = nil
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	s.capabilityRepo.(*MockCapabilityRepository).On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)

	agent, err := s.CreateAgent(context.Background(), req, orgID, uuid.New(), nil, nil, "")
	require.NoError(t, err)
	return agent
}

// The quick start's first agent: in monitoring mode, the mode a new
// organization starts in, an agent registered with its own key and a
// sufficient trust score is verified at registration.
func TestCreateAgent_SDKShortestCallInMonitoringModeIsVerified(t *testing.T) {
	agent := registerInMode(t, domain.EnforcementModeMonitoring, sdkShortestRegistration(t), 0.59, 0.75)

	assert.Equal(t, domain.AgentStatusVerified, agent.Status)
	assert.NotNil(t, agent.VerifiedAt)
	assert.Nil(t, agent.EncryptedPrivateKey, "the SDK keeps the private key")
	assert.Empty(t, agent.PendingReason)
}

// Strict mode keeps verification by hand, and says so.
func TestCreateAgent_SDKShortestCallInStrictModeIsPendingWithItsReason(t *testing.T) {
	agent := registerInMode(t, domain.EnforcementModeStrict, sdkShortestRegistration(t), 0.59)

	assert.Equal(t, domain.AgentStatusPending, agent.Status)
	assert.Nil(t, agent.VerifiedAt)
	assert.Equal(t, "pending: the organization is in strict mode; an administrator verifies the agent under Agents in the dashboard",
		agent.PendingReason)
}

// A trust score under the threshold leaves the agent pending in monitoring
// mode, and the reason names the score and the threshold.
func TestCreateAgent_TrustScoreUnderTheThresholdIsPendingWithItsReason(t *testing.T) {
	agent := registerInMode(t, domain.EnforcementModeMonitoring, sdkShortestRegistration(t), 0.25)

	assert.Equal(t, domain.AgentStatusPending, agent.Status)
	assert.Nil(t, agent.VerifiedAt)
	assert.Equal(t, "pending: trust score 0.25 is below the threshold 0.30", agent.PendingReason)
}

func TestAutoVerifyRefusal_NamesTheCheckTheAgentDidNotPass(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	clientKey := base64.StdEncoding.EncodeToString(publicKey)
	stringPtr := func(s string) *string { return &s }
	qualifying := func() *domain.Agent {
		return &domain.Agent{
			Name: "billing-agent", DisplayName: "Billing Agent", Description: "Reconciles invoices",
			TrustScore: 0.59, PublicKey: stringPtr(clientKey),
		}
	}

	for _, tc := range []struct {
		name   string
		change func(*domain.Agent)
		want   string
	}{
		{"its own key", func(*domain.Agent) {}, ""},
		{"a key the server holds", func(a *domain.Agent) {
			a.PublicKey, a.EncryptedPrivateKey = stringPtr("server-key"), stringPtr("encrypted")
		}, ""},
		{"exactly the threshold", func(a *domain.Agent) { a.TrustScore = 0.3 }, ""},
		{"no public key", func(a *domain.Agent) { a.PublicKey = nil }, "pending: the agent has no public key"},
		{"an empty public key", func(a *domain.Agent) { a.PublicKey = stringPtr("") }, "pending: the agent has no public key"},
		{"a public key that is not an Ed25519 key", func(a *domain.Agent) { a.PublicKey = stringPtr("sdk-provided-public-key-base64") },
			"pending: the agent's public key is not a base64-encoded Ed25519 public key"},
		{"a low trust score", func(a *domain.Agent) { a.TrustScore = 0.29 }, "pending: trust score 0.29 is below the threshold 0.30"},
		{"no description", func(a *domain.Agent) { a.Description = "" }, "pending: the agent has no description"},
		{"no display name or description", func(a *domain.Agent) { a.DisplayName, a.Description = "", "" },
			"pending: the agent has no display name, description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := qualifying()
			tc.change(agent)
			assert.Equal(t, tc.want, autoVerifyRefusal(agent))
			assert.Equal(t, tc.want == "", (&AgentService{}).shouldAutoVerifyAgent(agent))
		})
	}
}
