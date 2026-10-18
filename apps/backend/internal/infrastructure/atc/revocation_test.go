package atc

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// stubRevocation is a revocation source whose answer the test sets. It counts
// its reads and records, for each, the agent and the time left on the read's
// context (-1 when the context has no deadline).
type stubRevocation struct {
	mu       sync.Mutex
	revoked  bool
	err      error
	block    bool
	agentIDs []uuid.UUID
	timeLeft []time.Duration
}

func (s *stubRevocation) AgentRevoked(ctx context.Context, agentID uuid.UUID) (bool, error) {
	s.mu.Lock()
	s.agentIDs = append(s.agentIDs, agentID)
	if deadline, ok := ctx.Deadline(); ok {
		s.timeLeft = append(s.timeLeft, time.Until(deadline))
	} else {
		s.timeLeft = append(s.timeLeft, -1)
	}
	revoked, err, block := s.revoked, s.err, s.block
	s.mu.Unlock()
	if block {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return revoked, err
}

func (s *stubRevocation) setRevoked(revoked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked = revoked
}

func (s *stubRevocation) reads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.agentIDs)
}

// notRevoked is a revocation source that reports no agent revoked, for the
// tests of the verification steps other than revocation.
func notRevoked() *stubRevocation { return &stubRevocation{} }

// newRevocationTestVerifier returns a verifier trusting one issuer, with
// source as its revocation source, and a function that issues an ATC from
// that issuer for an agent.
func newRevocationTestVerifier(t *testing.T, source atcdomain.RevocationSource) (*RealATCVerifier, func(agentID uuid.UUID) string) {
	t.Helper()
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"
	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{{URI: issuerURI, PublicKey: []byte(pub)}}, source)
	return verifier, func(agentID uuid.UUID) string {
		return issueTestATC(t, priv, agentID, issuerURI, []string{"secrets:resolve"}, 5*time.Minute)
	}
}

func requireATCErrorCode(t *testing.T, err error, code string) *atcdomain.ATCError {
	t.Helper()
	var atcErr *atcdomain.ATCError
	if !errors.As(err, &atcErr) {
		t.Fatalf("expected the ATC error %s, got %v", code, err)
	}
	if atcErr.Code != code {
		t.Fatalf("expected the ATC error %s, got %s (%s)", code, atcErr.Code, atcErr.Message)
	}
	return atcErr
}

// An ATC whose agent the revocation source reports revoked is refused with
// atc_revoked, after one read of that agent's state.
func TestVerify_RefusesAnATCWhoseAgentIsRevoked(t *testing.T) {
	source := &stubRevocation{revoked: true}
	verifier, issue := newRevocationTestVerifier(t, source)
	agentID := uuid.New()

	claims, err := verifier.Verify(issue(agentID))
	if claims != nil {
		t.Fatalf("the ATC of a revoked agent was accepted (agent %s)", claims.AgentID)
	}
	requireATCErrorCode(t, err, atcdomain.ErrCodeRevoked)
	if source.reads() != 1 || source.agentIDs[0] != agentID {
		t.Fatalf("expected one read of agent %s, got reads of %v", agentID, source.agentIDs)
	}
}

// A revocation state that cannot be read is atc_revocation_unknown, never an
// accepted ATC. The answer does not carry the store's error text; the server
// log does.
func TestVerify_UnreadableRevocationStateIsUnknown(t *testing.T) {
	var serverLog bytes.Buffer
	log.SetOutput(&serverLog)
	defer log.SetOutput(os.Stderr)

	source := &stubRevocation{err: errors.New("dial tcp 192.0.2.10:5432: connection refused")}
	verifier, issue := newRevocationTestVerifier(t, source)

	claims, err := verifier.Verify(issue(uuid.New()))
	if claims != nil {
		t.Fatal("an ATC whose revocation state could not be read was accepted")
	}
	atcErr := requireATCErrorCode(t, err, atcdomain.ErrCodeRevocationUnknown)
	if atcErr.Message != "the revocation state of this ATC could not be read" {
		t.Errorf("the answer is %q", atcErr.Message)
	}
	if !strings.Contains(serverLog.String(), "192.0.2.10:5432: connection refused") {
		t.Errorf("the server log does not carry the read's error: %q", serverLog.String())
	}
}

// A verifier built without a revocation source can read no revocation state,
// so an ATC that passes the other steps is answered atc_revocation_unknown.
func TestVerify_WithoutARevocationSourceIsUnknown(t *testing.T) {
	verifier, issue := newRevocationTestVerifier(t, nil)

	claims, err := verifier.Verify(issue(uuid.New()))
	if claims != nil {
		t.Fatal("a verifier without a revocation source accepted an ATC")
	}
	requireATCErrorCode(t, err, atcdomain.ErrCodeRevocationUnknown)
}

// Each Verify call reads the revocation state once, a call the cache of
// verified tokens answers included, so an agent revoked after its ATC was
// first accepted is refused on the next call.
func TestVerify_ReadsRevocationOnEveryCallIncludingCachedTokens(t *testing.T) {
	source := notRevoked()
	verifier, issue := newRevocationTestVerifier(t, source)
	token := issue(uuid.New())

	for call := 1; call <= 2; call++ {
		if _, err := verifier.Verify(token); err != nil {
			t.Fatalf("call %d: Verify failed: %v", call, err)
		}
		if got := source.reads(); got != call {
			t.Fatalf("after call %d the revocation state was read %d times, want %d", call, got, call)
		}
	}

	source.setRevoked(true)
	claims, err := verifier.Verify(token)
	if claims != nil {
		t.Fatal("a cached ATC was accepted after its agent was revoked")
	}
	requireATCErrorCode(t, err, atcdomain.ErrCodeRevoked)
	if got := source.reads(); got != 3 {
		t.Fatalf("the revocation state was read %d times in three calls", got)
	}
}

// The read is bounded: its context ends revocationReadTimeout (2 s) after the
// read starts, and a read that has not answered by then is unknown.
func TestVerify_RevocationReadIsBounded(t *testing.T) {
	if revocationReadTimeout != 2*time.Second {
		t.Fatalf("revocationReadTimeout is %s, want 2s", revocationReadTimeout)
	}

	source := notRevoked()
	verifier, issue := newRevocationTestVerifier(t, source)
	if _, err := verifier.Verify(issue(uuid.New())); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if left := source.timeLeft[0]; left <= 0 || left > revocationReadTimeout {
		t.Fatalf("the read's context had %s left, want a deadline within %s", left, revocationReadTimeout)
	}

	blocking := &stubRevocation{block: true}
	verifier, issue = newRevocationTestVerifier(t, blocking)
	verifier.revocationTimeout = 20 * time.Millisecond
	start := time.Now()
	_, err := verifier.Verify(issue(uuid.New()))
	requireATCErrorCode(t, err, atcdomain.ErrCodeRevocationUnknown)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("a read that never answered held Verify for %s", elapsed)
	}
}

// stubAgentStatusReader answers AgentStatus with the values the test sets and
// keeps the context it was given.
type stubAgentStatusReader struct {
	status domain.AgentStatus
	found  bool
	err    error
	ctx    context.Context
}

func (r *stubAgentStatusReader) AgentStatus(ctx context.Context, _ uuid.UUID) (domain.AgentStatus, bool, error) {
	r.ctx = ctx
	return r.status, r.found, r.err
}

// An agent's ATCs are revoked exactly when the agent authentication paths
// refuse the agent (domain.AgentStatusPermitsAuth), or no agent has the id.
func TestAgentStatusRevocation_FollowsTheAgentAuthenticationAllowList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  domain.AgentStatus
		found   bool
		revoked bool
	}{
		{"verified", domain.AgentStatusVerified, true, false},
		{"pending", domain.AgentStatusPending, true, false},
		{"suspended", domain.AgentStatusSuspended, true, true},
		{"revoked", domain.AgentStatusRevoked, true, true},
		{"empty status", "", true, true},
		{"unrecognised status", "archived", true, true},
		{"no such agent", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &stubAgentStatusReader{status: tc.status, found: tc.found}
			revoked, err := NewAgentStatusRevocation(reader).AgentRevoked(context.Background(), uuid.New())
			if err != nil {
				t.Fatalf("AgentRevoked: %v", err)
			}
			if revoked != tc.revoked {
				t.Fatalf("status %q found=%t: revoked=%t, want %t", tc.status, tc.found, revoked, tc.revoked)
			}
		})
	}
}

// A failed store read is an error, which the verifier answers as unknown, and
// the read runs under the caller's context.
func TestAgentStatusRevocation_AFailedReadIsAnError(t *testing.T) {
	reader := &stubAgentStatusReader{status: domain.AgentStatusVerified, found: true, err: errors.New("read failed")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := NewAgentStatusRevocation(reader).AgentRevoked(ctx, uuid.New()); err == nil {
		t.Fatal("a failed store read was answered as a revocation state")
	}
	if reader.ctx != ctx {
		t.Fatal("the store read did not run under the caller's context")
	}

	if _, err := NewAgentStatusRevocation(nil).AgentRevoked(ctx, uuid.New()); err == nil {
		t.Fatal("a revocation source without a store answered a revocation state")
	}
}
