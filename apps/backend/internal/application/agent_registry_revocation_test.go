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

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// registryRevocationWait bounds how long a test waits for the revocation
// request RevokeAgent starts. The first request is made without a delay, so
// reaching this bound is a failure, not a slow pass.
const registryRevocationWait = 10 * time.Second

type registryATCRevocationCall struct {
	agentID string
	reason  string
}

// fakeRegistryATCRevoker answers each RevokeATC call with the next of errs, or
// nil once they run out, and reports every call on calls. With release set, a
// call answers only once release is closed.
type fakeRegistryATCRevoker struct {
	mu      sync.Mutex
	errs    []error
	calls   chan registryATCRevocationCall
	release chan struct{}
}

func newFakeRegistryATCRevoker(errs ...error) *fakeRegistryATCRevoker {
	return &fakeRegistryATCRevoker{errs: errs, calls: make(chan registryATCRevocationCall, 16)}
}

func (f *fakeRegistryATCRevoker) RevokeATC(_ context.Context, agentID, reason string) error {
	f.mu.Lock()
	var err error
	if len(f.errs) > 0 {
		err, f.errs = f.errs[0], f.errs[1:]
	}
	f.mu.Unlock()
	f.calls <- registryATCRevocationCall{agentID: agentID, reason: reason}
	if f.release != nil {
		<-f.release
	}
	return err
}

// revocableAgentService is an AgentService with mock repositories, holding a
// verified agent RevokeAgent can revoke.
func revocableAgentService(t *testing.T) (*AgentService, *MockAgentRepository, *domain.Agent) {
	t.Helper()
	repo := new(MockAgentRepository)
	calc := new(AgentServiceMockTrustScoreCalculator)
	scores := new(AgentServiceMockTrustScoreRepository)
	agent := createTestAgentForService()
	repo.On("GetByID", agent.ID).Return(agent, nil)
	repo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(nil, errors.New("not scored in this test"))
	return &AgentService{agentRepo: repo, trustCalc: calc, trustScoreRepo: scores}, repo, agent
}

func TestRevokeAgentAsksTheRegistryToRevokeTheAgentsCredentials(t *testing.T) {
	s, repo, agent := revocableAgentService(t)
	revoker := newFakeRegistryATCRevoker()
	s.SetRegistryATCRevoker(revoker)

	require.NoError(t, s.RevokeAgent(context.Background(), agent.ID))
	assert.Equal(t, domain.AgentStatusRevoked, agent.Status)
	repo.AssertCalled(t, "Update", agent)

	select {
	case call := <-revoker.calls:
		assert.Equal(t, agent.ID.String(), call.agentID, "the agentId AIM's issuance requests name")
		assert.Equal(t, "revoked", call.reason, "the reason code AIM's own revocation list gives")
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry was not asked to revoke the agent's credentials within %s", registryRevocationWait)
	}
}

// RevokeAgent returns while the Registry has not answered the revocation
// request: the fake Registry answers only when the test ends.
func TestRevokeAgentDoesNotWaitForTheRegistrysAnswer(t *testing.T) {
	s, _, agent := revocableAgentService(t)
	revoker := newFakeRegistryATCRevoker()
	revoker.release = make(chan struct{})
	defer close(revoker.release)
	s.SetRegistryATCRevoker(revoker)

	done := make(chan error, 1)
	go func() { done <- s.RevokeAgent(context.Background(), agent.ID) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(registryRevocationWait):
		t.Fatalf("RevokeAgent had not returned after %s while the Registry had not answered", registryRevocationWait)
	}
	assert.Equal(t, domain.AgentStatusRevoked, agent.Status)
	select {
	case call := <-revoker.calls:
		assert.Equal(t, agent.ID.String(), call.agentID)
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry was not asked to revoke the agent's credentials within %s", registryRevocationWait)
	}
}

// The revocation is AIM's own: a Registry that cannot be reached leaves the
// agent revoked, and RevokeAgent answers as it does without a Registry.
func TestRevokeAgentStandsWhenTheRegistryFails(t *testing.T) {
	s, _, agent := revocableAgentService(t)
	revoker := newFakeRegistryATCRevoker(errors.New("registry: revocation request failed: connection refused"))
	s.SetRegistryATCRevoker(revoker)
	s.atcRevocationRetryDelays = []time.Duration{}

	require.NoError(t, s.RevokeAgent(context.Background(), agent.ID))
	assert.Equal(t, domain.AgentStatusRevoked, agent.Status)

	select {
	case <-revoker.calls:
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry was not asked to revoke the agent's credentials within %s", registryRevocationWait)
	}
}

// Nothing is sent for a revocation AIM refused or did not store.
func TestRevokeAgentAsksTheRegistryNothingWhenTheRevocationIsNotStored(t *testing.T) {
	t.Run("already revoked", func(t *testing.T) {
		s, repo, agent := revocableAgentService(t)
		agent.Status = domain.AgentStatusRevoked
		revoker := newFakeRegistryATCRevoker()
		s.SetRegistryATCRevoker(revoker)

		require.Error(t, s.RevokeAgent(context.Background(), agent.ID))
		repo.AssertNotCalled(t, "Update", mock.Anything)
		assertNoRegistryRevocation(t, revoker)
	})

	t.Run("not stored", func(t *testing.T) {
		repo := new(MockAgentRepository)
		agent := createTestAgentForService()
		repo.On("GetByID", agent.ID).Return(agent, nil)
		repo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errors.New("database unavailable"))
		s := &AgentService{agentRepo: repo}
		revoker := newFakeRegistryATCRevoker()
		s.SetRegistryATCRevoker(revoker)

		require.Error(t, s.RevokeAgent(context.Background(), agent.ID))
		assertNoRegistryRevocation(t, revoker)
	})
}

func assertNoRegistryRevocation(t *testing.T, revoker *fakeRegistryATCRevoker) {
	t.Helper()
	select {
	case call := <-revoker.calls:
		t.Fatalf("the Registry was asked to revoke agent %s", call.agentID)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRevokeRegistryATCsRetriesWhatTheRegistryMayAnswerLater(t *testing.T) {
	revoker := newFakeRegistryATCRevoker(
		errors.New("registry: revocation request failed: connection refused"),
		&registry.ATCStatusError{StatusCode: http.StatusServiceUnavailable},
		&registry.ATCStatusError{StatusCode: http.StatusTooManyRequests},
	)

	err := revokeRegistryATCs(revoker, "agent-1", []time.Duration{0, 0, 0, 0})

	require.NoError(t, err)
	assert.Len(t, revoker.calls, 4, "three failed attempts and the one the Registry answered")
}

func TestRevokeRegistryATCsGivesUpAfterTheLastRetry(t *testing.T) {
	unavailable := &registry.ATCStatusError{StatusCode: http.StatusServiceUnavailable}
	revoker := newFakeRegistryATCRevoker(unavailable, unavailable, unavailable, unavailable)

	err := revokeRegistryATCs(revoker, "agent-1", []time.Duration{0, 0})

	assert.ErrorIs(t, err, unavailable)
	assert.Len(t, revoker.calls, 3, "the first attempt and one per retry delay")
}

// A refusal that a later request repeats is not retried.
func TestRevokeRegistryATCsStopsAtARefusalThatRepeats(t *testing.T) {
	for name, refusal := range map[string]error{
		"no token":        registry.ErrTokenNotConfigured,
		"forbidden":       &registry.ATCStatusError{StatusCode: http.StatusForbidden},
		"route not found": &registry.ATCStatusError{StatusCode: http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			revoker := newFakeRegistryATCRevoker(refusal)

			err := revokeRegistryATCs(revoker, "agent-1", []time.Duration{0, 0, 0})

			assert.ErrorIs(t, err, refusal)
			assert.Len(t, revoker.calls, 1)
		})
	}
}

// End to end through the Registry client main.go wires: revoking an agent
// sends the Registry's internal revoke route the agent's ID and the reason.
func TestRevokeAgentRevokesTheAgentsCredentialsThroughTheRegistryClient(t *testing.T) {
	type received struct {
		method, path, auth string
		body               map[string]any
	}
	requests := make(chan received, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := received{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization")}
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		requests <- got
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ATC revoked"}`))
	}))
	defer srv.Close()

	s, _, agent := revocableAgentService(t)
	s.SetRegistryATCRevoker(registry.NewATCClient(srv.URL, "not-a-real-token", srv.Client()))

	require.NoError(t, s.RevokeAgent(context.Background(), agent.ID))

	select {
	case got := <-requests:
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "/internal/atc/"+agent.ID.String()+"/revoke", got.path)
		assert.Equal(t, "Bearer not-a-real-token", got.auth)
		assert.Equal(t, map[string]any{"reason": "revoked"}, got.body)
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry received no revocation within %s", registryRevocationWait)
	}
}
