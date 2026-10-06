package application

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every verification event records who stands behind its outcome. The source
// is an argument the calling code path sets; it is not a field of the request
// type, so nothing bound from a request body can choose it, and an unknown
// source is refused before the agent is read or a row is written.

func sourceFixture() (*VerificationEventService, *ownershipEventRepo, *ownershipAgentRepo, uuid.UUID, uuid.UUID) {
	org, agentID := uuid.New(), uuid.New()
	events := &ownershipEventRepo{}
	agents := &ownershipAgentRepo{agents: map[uuid.UUID]*domain.Agent{
		agentID: {ID: agentID, OrganizationID: org, DisplayName: "Source Fixture", Status: domain.AgentStatusVerified},
	}}
	return NewVerificationEventService(events, agents, nil), events, agents, org, agentID
}

func TestVerificationEventSource_UnknownSourceIsRefused(t *testing.T) {
	for _, bad := range []domain.VerificationEventSource{"", "future_source", "SERVICE"} {
		t.Run("create/"+string(bad), func(t *testing.T) {
			svc, events, agents, org, agentID := sourceFixture()
			event, err := svc.CreateVerificationEvent(context.Background(), bad, eventRequest(org, agentID, nil))
			assert.ErrorIs(t, err, domain.ErrVerificationEventSourceRequired)
			assert.Nil(t, event)
			assert.Zero(t, agents.lookups, "refused before the agent is read")
			assert.Empty(t, events.created, "refused before a row is written")
		})
		t.Run("log/"+string(bad), func(t *testing.T) {
			svc, events, agents, org, agentID := sourceFixture()
			event, err := svc.LogVerificationEvent(context.Background(), bad, org, agentID,
				domain.VerificationProtocolA2A, domain.VerificationTypeCapability,
				domain.VerificationEventStatusSuccess, 0, domain.InitiatorTypeAgent, nil, nil)
			assert.ErrorIs(t, err, domain.ErrVerificationEventSourceRequired)
			assert.Nil(t, event)
			assert.Zero(t, agents.lookups)
			assert.Empty(t, events.created)
		})
	}
}

func TestVerificationEventSource_StoredAsGiven(t *testing.T) {
	for _, src := range []domain.VerificationEventSource{
		domain.VerificationEventSourceService,
		domain.VerificationEventSourceSystem,
		domain.VerificationEventSourceCallerReported,
		domain.VerificationEventSourceAgentReported,
	} {
		t.Run(string(src), func(t *testing.T) {
			svc, events, _, org, agentID := sourceFixture()

			_, err := svc.CreateVerificationEvent(context.Background(), src, eventRequest(org, agentID, nil))
			require.NoError(t, err)
			_, err = svc.LogVerificationEvent(context.Background(), src, org, agentID,
				domain.VerificationProtocolA2A, domain.VerificationTypeCapability,
				domain.VerificationEventStatusSuccess, 0, domain.InitiatorTypeAgent, nil, nil)
			require.NoError(t, err)

			require.Len(t, events.created, 2)
			assert.Equal(t, src, events.created[0].Source)
			assert.Equal(t, src, events.created[1].Source)
		})
	}
}

// The request type carries no source, so a body decoded into it, or copied
// field by field from one, cannot set the source a row is recorded under.
func TestVerificationEventSource_RequestTypeCannotCarryASource(t *testing.T) {
	sourceType := reflect.TypeOf(domain.VerificationEventSource(""))
	rt := reflect.TypeOf(CreateVerificationEventRequest{})
	for i := 0; i < rt.NumField(); i++ {
		assert.NotEqual(t, sourceType, rt.Field(i).Type,
			"field %s would let a request choose the source", rt.Field(i).Name)
	}

	// encoding/json matches keys case-insensitively, so a field named Source,
	// or tagged "source", of any type would bind or fail here.
	var req CreateVerificationEventRequest
	require.NoError(t, json.Unmarshal([]byte(`{"source":"service"}`), &req))
	assert.Equal(t, CreateVerificationEventRequest{}, req, "a body's source binds nothing")
}

// The outcome in an action-result report is the agent's own claim, so the
// event is recorded as agent_reported and no trust factor counts it.
func TestVerificationEventSource_ActionResultIsAgentReported(t *testing.T) {
	for _, success := range []bool{true, false} {
		evSvc, events, agents, _, agentID := sourceFixture()
		svc := &AgentService{agentRepo: agents, verificationEventService: evSvc}

		require.NoError(t, svc.LogCapabilityResult(context.Background(), agentID, uuid.New(), success, "", nil))

		require.Len(t, events.created, 1)
		assert.Equal(t, domain.VerificationEventSourceAgentReported, events.created[0].Source,
			"success=%v reported by the agent", success)
	}
}
