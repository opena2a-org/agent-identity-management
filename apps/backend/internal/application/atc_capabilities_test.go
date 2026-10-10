package application

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// fakeCapabilityReader stands in for the agent_capabilities table: grants
// returns what GetActiveCapabilitiesByAgentID would, in the order it would.
type fakeCapabilityReader struct {
	grants []*domain.AgentCapability
	err    error
	calls  int
}

func (f *fakeCapabilityReader) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	f.calls++
	return f.grants, f.err
}

func grant(capabilityType string) *domain.AgentCapability {
	return &domain.AgentCapability{ID: uuid.New(), CapabilityType: capabilityType, GrantedAt: time.Now().UTC()}
}

func issuanceServiceForGrants(t *testing.T, agent *domain.Agent, grants *fakeCapabilityReader) (*ATCIssuanceService, *fakeRegistryClient) {
	t.Helper()
	client := &fakeRegistryClient{respond: conformingATC}
	svc := NewATCIssuanceService(
		&fakeAgentReader{agent: agent},
		grants,
		&fakeOrgReader{org: &domain.Organization{Name: "Acme"}},
		&fakeScorer{score: &domain.TrustScore{Score: 0.5}},
		client,
		testATCPublicOrigin,
	)
	return svc, client
}

// The credential's capabilities are the agent's active grants at issuance, not
// the list the SDK reported at registration. Grant A and B, issue, revoke A,
// grant C, issue again: the second credential names exactly B and C.
func TestIssueForAgent_CapabilitiesFollowGrantsAndRevocations(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New(), Capabilities: []string{"legacy:detected"}}
	a, b, c := grant("file:read"), grant("api:call"), grant("db:read")

	reader := &fakeCapabilityReader{grants: []*domain.AgentCapability{b, a}}
	svc, client := issuanceServiceForGrants(t, agent, reader)

	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("first issuance: %v", err)
	}
	if got, want := client.gotReq.Capabilities, []string{"api:call", "file:read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first credential capabilities = %v, want the active grants %v", got, want)
	}

	// Revoke A, grant C. The table answers with the new active set.
	reader.grants = []*domain.AgentCapability{c, b}
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("second issuance: %v", err)
	}
	if got, want := client.gotReq.Capabilities, []string{"api:call", "db:read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second credential capabilities = %v, want the active grants %v", got, want)
	}
	for _, token := range client.gotReq.Capabilities {
		if token == "legacy:detected" {
			t.Fatalf("the credential carries the registration-time list %v", agent.Capabilities)
		}
	}
	if reader.calls != 2 {
		t.Errorf("grants read %d times across two issuances, want one read per issuance", reader.calls)
	}
}

// Two issuances with no grant change carry byte-identical capabilities, whatever
// order the table returns the rows in. The list is covered by the signature, so
// its order must not depend on row order.
func TestIssueForAgent_CapabilitiesAreStableAcrossIssuances(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New()}
	rows := []*domain.AgentCapability{grant("file:read"), grant("api:call"), grant("db:read")}

	reader := &fakeCapabilityReader{grants: rows}
	svc, client := issuanceServiceForGrants(t, agent, reader)
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("first issuance: %v", err)
	}
	first, _ := json.Marshal(client.gotReq.Capabilities)

	reader.grants = []*domain.AgentCapability{rows[2], rows[0], rows[1]}
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("second issuance: %v", err)
	}
	second, _ := json.Marshal(client.gotReq.Capabilities)

	if string(first) != string(second) {
		t.Fatalf("capabilities differ between two issuances of the same grant set: %s then %s", first, second)
	}
}

// A honeytoken grant is a decoy no legitimate workflow exercises; a revoked row
// is not a grant. Neither is a capability the credential asserts.
func TestIssueForAgent_CapabilitiesOmitHoneytokensAndRevokedRows(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New()}
	decoy := grant("payment:transfer")
	decoy.Honeytoken = true
	revoked := grant("admin:all")
	revokedAt := time.Now().UTC()
	revoked.RevokedAt = &revokedAt

	svc, client := issuanceServiceForGrants(t, agent, &fakeCapabilityReader{grants: []*domain.AgentCapability{decoy, grant("file:read"), revoked}})
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("IssueForAgent: %v", err)
	}
	if got, want := client.gotReq.Capabilities, []string{"file:read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
}

// A grant whose type does not match the credential schema's token grammar is
// left out and counted; it never fails the issuance and is never rewritten.
func TestIssueForAgent_CapabilityOutsideTokenGrammarIsDroppedAndCounted(t *testing.T) {
	tokens, dropped := grantedCapabilityTokens([]*domain.AgentCapability{
		grant("file:read"),
		grant("File:Read"),      // upper-case namespace
		grant("no-colon"),       // no operation
		grant("a:b:c"),          // two colons
		grant("file:read"),      // duplicate
		grant("tools:search.*"), // valid: '.' and '*' are allowed in the operation
	})
	if got, want := tokens, []string{"file:read", "tools:search.*"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %v, want %v", got, want)
	}
	if dropped != 3 {
		t.Fatalf("dropped = %d, want 3", dropped)
	}
	pattern := regexp.MustCompile(`^[a-z0-9_-]+:[A-Za-z0-9_.*-]+$`)
	for _, token := range tokens {
		if !pattern.MatchString(token) {
			t.Errorf("token %q does not match the credential schema grammar", token)
		}
	}
}

// An agent with no active grant issues a credential whose capabilities is the
// empty array. The schema requires the field as an array, so null and an absent
// member are both wrong.
func TestIssueForAgent_NoGrantsSendsEmptyCapabilityArray(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New(), Capabilities: []string{"legacy:detected"}}

	svc, client := issuanceServiceForGrants(t, agent, &fakeCapabilityReader{})
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("IssueForAgent: %v", err)
	}
	body, err := json.Marshal(client.gotReq)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	raw, ok := wire["capabilities"]
	if !ok {
		t.Fatalf("request omits capabilities; the credential schema requires the field: %s", body)
	}
	if string(raw) != "[]" {
		t.Fatalf("capabilities on the wire = %s, want []", raw)
	}
}

// A grant read that fails stops the issuance. The registration-time list is
// never the fallback.
func TestIssueForAgent_GrantReadErrorFailsClosed(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New(), Capabilities: []string{"legacy:detected"}}

	svc, client := issuanceServiceForGrants(t, agent, &fakeCapabilityReader{err: errors.New("db down")})
	if _, err := svc.IssueForAgent(context.Background(), id); err == nil {
		t.Fatal("issued a credential without reading the grant table")
	}
	if client.gotReq.AgentID != "" {
		t.Fatalf("the issuer was called with %+v after the grant read failed", client.gotReq)
	}

	svc = NewATCIssuanceService(&fakeAgentReader{agent: agent}, nil, nil,
		&fakeScorer{score: &domain.TrustScore{Score: 0.5}}, &fakeRegistryClient{respond: conformingATC}, testATCPublicOrigin)
	if _, err := svc.IssueForAgent(context.Background(), id); err == nil {
		t.Fatal("issued a credential with no grant source configured")
	}
}

// The request struct always serializes capabilities, so the Registry receives
// the active set even when it is empty.
func TestATCIssuanceRequest_CapabilitiesNeverOmitted(t *testing.T) {
	body, err := json.Marshal(registry.ATCIssuanceRequest{Capabilities: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if raw, ok := wire["capabilities"]; !ok || string(raw) != "[]" {
		t.Fatalf("capabilities on the wire = %s (present: %v), want []", raw, ok)
	}
}
