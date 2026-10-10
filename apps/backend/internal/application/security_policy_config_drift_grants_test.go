package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configDriftService answers one enabled, alert-only config drift policy with
// rules for the organization and reads agents' grants from grants.
func configDriftService(orgID uuid.UUID, rules map[string]interface{}, grants activeCapabilityReader) *SecurityPolicyService {
	policies := new(MockSecurityPolicyRepository)
	policies.On("GetByType", orgID, domain.PolicyTypeConfigDrift).Return([]*domain.SecurityPolicy{{
		ID:                uuid.New(),
		OrganizationID:    orgID,
		Name:              "Capability drift",
		PolicyType:        domain.PolicyTypeConfigDrift,
		EnforcementAction: domain.EnforcementAlertOnly,
		Rules:             rules,
		AppliesTo:         "all",
		IsEnabled:         true,
	}}, nil)
	service := NewSecurityPolicyService(policies, nil, nil)
	service.SetCapabilityRepository(grants)
	return service
}

// The config drift policy reads an agent's capabilities from its active grants,
// not from the list the SDK reported at registration, which no grant or
// revocation updates. Each case gives the two lists different answers.
func TestConfigDriftCapabilityChecksReadTheActiveGrants(t *testing.T) {
	escalation := map[string]interface{}{"check_permission_escalation": true}
	drift := map[string]interface{}{
		"check_capability_changes": true,
		"baseline_capabilities":    []interface{}{"api:call", "file:read"},
	}
	decoy := grant("system:admin")
	decoy.Honeytoken = true

	for _, tc := range []struct {
		name       string
		rules      map[string]interface{}
		registered []string
		grants     []*domain.AgentCapability
		alert      bool
	}{
		{
			name:       "a dangerous capability granted after registration is seen",
			rules:      escalation,
			registered: nil,
			grants:     []*domain.AgentCapability{grant("file:read"), grant("admin:all")},
			alert:      true,
		},
		{
			name:       "a dangerous capability revoked since registration is not seen",
			rules:      escalation,
			registered: []string{"admin:all", "file:read"},
			grants:     []*domain.AgentCapability{grant("file:read")},
			alert:      false,
		},
		{
			name:       "a honeytoken grant is not a capability",
			rules:      escalation,
			registered: []string{"file:read"},
			grants:     []*domain.AgentCapability{grant("file:read"), decoy},
			alert:      false,
		},
		{
			name:       "a capability granted beyond the baseline is drift",
			rules:      drift,
			registered: []string{"api:call", "file:read"},
			grants:     []*domain.AgentCapability{grant("file:read"), grant("api:call"), grant("db:write")},
			alert:      true,
		},
		{
			name:       "grants matching the baseline are not drift whatever the registration list says",
			rules:      drift,
			registered: []string{"db:write", "file:read"},
			grants:     []*domain.AgentCapability{grant("api:call"), grant("file:read")},
			alert:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "ops-agent", Capabilities: tc.registered}
			reader := &fakeCapabilityReader{grants: tc.grants}
			service := configDriftService(agent.OrganizationID, tc.rules, reader)

			block, alert, policyName, err := service.EvaluateConfigDrift(context.Background(), agent, "file:read", "", uuid.New())

			require.NoError(t, err)
			assert.False(t, block, "the policy is alert-only")
			assert.Equal(t, tc.alert, alert)
			if tc.alert {
				assert.Equal(t, "Capability drift", policyName)
			}
			assert.Equal(t, 1, reader.calls, "the grant table is read once per evaluation")
		})
	}
}

// An agent with no grant is read once too: the empty list is an answer, and the
// second policy's check uses it instead of reading the grant table again.
func TestConfigDriftReadsAnAgentWithNoGrantOnce(t *testing.T) {
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "ops-agent", Capabilities: []string{"admin:all"}}
	escalation := func(name string) *domain.SecurityPolicy {
		return &domain.SecurityPolicy{
			ID:                uuid.New(),
			OrganizationID:    agent.OrganizationID,
			Name:              name,
			PolicyType:        domain.PolicyTypeConfigDrift,
			EnforcementAction: domain.EnforcementAlertOnly,
			Rules:             map[string]interface{}{"check_permission_escalation": true},
			AppliesTo:         "all",
			IsEnabled:         true,
		}
	}
	policies := new(MockSecurityPolicyRepository)
	policies.On("GetByType", agent.OrganizationID, domain.PolicyTypeConfigDrift).Return(
		[]*domain.SecurityPolicy{escalation("Escalation A"), escalation("Escalation B")}, nil)
	reader := &fakeCapabilityReader{}
	service := NewSecurityPolicyService(policies, nil, nil)
	service.SetCapabilityRepository(reader)

	block, alert, _, err := service.EvaluateConfigDrift(context.Background(), agent, "file:read", "", uuid.New())

	require.NoError(t, err)
	assert.False(t, block)
	assert.False(t, alert, "an agent with no grant holds no dangerous capability")
	assert.Equal(t, 1, reader.calls, "the grant table is read once per evaluation")
}

// When the grant table cannot be read, the capability checks do not fall back
// to the registration-time list: the evaluation fails and decides nothing.
func TestConfigDriftFailsWhenGrantsCannotBeRead(t *testing.T) {
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "ops-agent", Capabilities: []string{"admin:all"}}
	reader := &fakeCapabilityReader{err: errors.New("agent_capabilities: connection reset")}
	service := configDriftService(agent.OrganizationID, map[string]interface{}{"check_permission_escalation": true}, reader)

	block, alert, _, err := service.EvaluateConfigDrift(context.Background(), agent, "file:read", "", uuid.New())

	require.Error(t, err)
	assert.False(t, block)
	assert.False(t, alert)
}
