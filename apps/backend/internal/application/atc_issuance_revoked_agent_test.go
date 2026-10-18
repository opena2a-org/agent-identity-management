package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// revocableAgentReader serves one agent as the agent repository does, a fresh
// copy on each read, and serves it revoked once revoke has been called.
type revocableAgentReader struct {
	mu      sync.Mutex
	agent   domain.Agent
	revoked bool
	reads   int

	// failFromRead, when above zero, makes that read and every later one fail.
	failFromRead int
}

func (r *revocableAgentReader) GetByID(id uuid.UUID) (*domain.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	if r.failFromRead > 0 && r.reads >= r.failFromRead {
		return nil, errors.New("database unavailable")
	}
	if id != r.agent.ID {
		return nil, errors.New("agent not found")
	}
	agent := r.agent
	if r.revoked {
		agent.Status = domain.AgentStatusRevoked
	}
	return &agent, nil
}

// revoke stores the agent's revocation, as AgentService.RevokeAgent does.
func (r *revocableAgentReader) revoke() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked = true
}

// countingScorer answers with score and counts the calls. Each call recomputes
// and stores the agent's trust score (IssueForAgent's side effect).
type countingScorer struct {
	score *domain.TrustScore
	calls int
}

func (f *countingScorer) CalculateTrustScore(context.Context, uuid.UUID) (*domain.TrustScore, error) {
	f.calls++
	return f.score, nil
}

func verifiedAgentForIssuance() domain.Agent {
	return domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Status: domain.AgentStatusVerified}
}

// A revoked agent gets no credential: the Registry is not asked to issue one,
// and the agent's trust score is neither computed nor stored. Nothing was
// issued, so no revocation is sent either: RevokeAgent sent the agent's.
func TestIssueForAgentRefusesARevokedAgent(t *testing.T) {
	agent := verifiedAgentForIssuance()
	agent.Status = domain.AgentStatusRevoked
	grants := &fakeCapabilityReader{grants: []*domain.AgentCapability{grant("file:read")}}
	scorer := &countingScorer{score: &domain.TrustScore{Score: 0.8}}
	client := &fakeRegistryClient{respond: conformingATC}
	svc := NewATCIssuanceService(&revocableAgentReader{agent: agent}, grants, nil, scorer, client, testATCPublicOrigin)
	revoker := newFakeRegistryATCRevoker()
	svc.SetRegistryATCRevoker(revoker)

	res, err := svc.IssueForAgent(context.Background(), agent.ID)

	require.ErrorIs(t, err, ErrATCAgentRevoked)
	assert.Nil(t, res)
	assert.Empty(t, client.gotReq.AgentID, "the Registry was asked to issue a credential for a revoked agent")
	assert.Zero(t, scorer.calls, "the trust score of a revoked agent was computed for a credential")
	assert.Zero(t, grants.calls, "the grants of a revoked agent were read for a credential")
	assertNoRegistryRevocation(t, revoker)
}

// An agent revoked while the Registry is issuing its credential: RevokeAgent's
// revocation request may have reached the Registry before the issuance did, so
// the credential is not handed on and the Registry is asked again to revoke the
// agent's credentials. It is asked when the issuance's answer did not arrive
// too, as the Registry may have signed the credential all the same.
func TestIssueForAgentHandsOnNoCredentialForAnAgentRevokedDuringIssuance(t *testing.T) {
	for name, issueErr := range map[string]error{
		"the Registry issued the credential":   nil,
		"the Registry's answer did not arrive": errors.New("registry: issuance request failed: context deadline exceeded"),
	} {
		t.Run(name, func(t *testing.T) {
			agent := verifiedAgentForIssuance()
			reader := &revocableAgentReader{agent: agent}
			client := &fakeRegistryClient{err: issueErr, respond: func(req registry.ATCIssuanceRequest) *registry.AgentTrustCredential {
				reader.revoke()
				if issueErr != nil {
					return nil
				}
				return conformingATC(req)
			}}
			svc := NewATCIssuanceService(reader, &fakeCapabilityReader{}, nil, &countingScorer{score: &domain.TrustScore{Score: 0.8}}, client, testATCPublicOrigin)
			revoker := newFakeRegistryATCRevoker()
			svc.SetRegistryATCRevoker(revoker)

			res, err := svc.IssueForAgent(context.Background(), agent.ID)

			require.ErrorIs(t, err, ErrATCAgentRevoked)
			assert.Nil(t, res)
			assert.Equal(t, agent.ID.String(), client.gotReq.AgentID, "the agent was verified when the Registry was asked")
			select {
			case call := <-revoker.calls:
				assert.Equal(t, agent.ID.String(), call.agentID)
				assert.Equal(t, "revoked", call.reason)
			case <-time.After(registryRevocationWait):
				t.Fatalf("the Registry was not asked again to revoke the agent's credentials within %s", registryRevocationWait)
			}
		})
	}
}

// An agent that is not revoked when the Registry answers gets the credential,
// and no revocation is sent.
func TestIssueForAgentHandsOnTheCredentialOfAnAgentNotRevoked(t *testing.T) {
	agent := verifiedAgentForIssuance()
	client := &fakeRegistryClient{respond: conformingATC}
	svc := NewATCIssuanceService(&revocableAgentReader{agent: agent}, &fakeCapabilityReader{}, nil, &countingScorer{score: &domain.TrustScore{Score: 0.8}}, client, testATCPublicOrigin)
	revoker := newFakeRegistryATCRevoker()
	svc.SetRegistryATCRevoker(revoker)

	res, err := svc.IssueForAgent(context.Background(), agent.ID)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, domain.BuildAgentDID(agent.ID), res.Credential.AgentDID)
	assertNoRegistryRevocation(t, revoker)
}

// When the agent cannot be read once the Registry has answered, whether it was
// revoked meanwhile is not known: the credential is not handed on, and no
// revocation is sent for an agent that may not be revoked.
func TestIssueForAgentHandsOnNoCredentialWhenTheAgentCannotBeReadAgain(t *testing.T) {
	agent := verifiedAgentForIssuance()
	client := &fakeRegistryClient{respond: conformingATC}
	svc := NewATCIssuanceService(&revocableAgentReader{agent: agent, failFromRead: 2}, &fakeCapabilityReader{}, nil, &countingScorer{score: &domain.TrustScore{Score: 0.8}}, client, testATCPublicOrigin)
	revoker := newFakeRegistryATCRevoker()
	svc.SetRegistryATCRevoker(revoker)

	res, err := svc.IssueForAgent(context.Background(), agent.ID)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrATCAgentRevoked)
	assert.Nil(t, res)
	assert.Equal(t, agent.ID.String(), client.gotReq.AgentID, "the agent was read once and the Registry asked")
	assertNoRegistryRevocation(t, revoker)
}

// End to end through the Registry client main.go wires for both issuance and
// revocation: the agent is revoked while the Registry is issuing, and the
// Registry receives a revocation request for the agent after the issuance.
func TestIssueForAgentSendsTheRegistryARevocationAfterAnIssuanceTheAgentWasRevokedDuring(t *testing.T) {
	agent := verifiedAgentForIssuance()
	reader := &revocableAgentReader{agent: agent}

	var mu sync.Mutex
	var received []string
	revocations := make(chan map[string]any, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/internal/atc/issue":
			var req registry.ATCIssuanceRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			reader.revoke()
			_ = json.NewEncoder(w).Encode(conformingATC(req))
		case "/internal/atc/" + agent.ID.String() + "/revoke":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			revocations <- body
			_, _ = w.Write([]byte(`{"message":"ATC revoked"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := registry.NewATCClient(srv.URL, "not-a-real-token", srv.Client())
	svc := NewATCIssuanceService(reader, &fakeCapabilityReader{}, nil, &countingScorer{score: &domain.TrustScore{Score: 0.8}}, client, testATCPublicOrigin)
	svc.SetRegistryATCRevoker(client)

	res, err := svc.IssueForAgent(context.Background(), agent.ID)

	require.ErrorIs(t, err, ErrATCAgentRevoked)
	assert.Nil(t, res)
	select {
	case body := <-revocations:
		assert.Equal(t, map[string]any{"reason": "revoked"}, body)
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry received no revocation within %s", registryRevocationWait)
	}
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{
		"POST /internal/atc/issue",
		"POST /internal/atc/" + agent.ID.String() + "/revoke",
	}, received)
}
