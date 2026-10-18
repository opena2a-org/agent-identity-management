package application

import (
	"errors"
	"strings"
	"testing"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// verifierAnswering answers Verify with err.
type verifierAnswering struct{ err error }

func (v verifierAnswering) Verify(string) (*atcdomain.ATCClaims, error) { return nil, v.err }

// resolveBodyATC resolves a signed request that carries its ATC in the body,
// with the service's verifier answering verifyErr.
func resolveBodyATC(t *testing.T, verifyErr error) (*resolveFixture, error) {
	t.Helper()
	f := newResolveFixture(t)
	f.svc.atcVerifier = verifierAnswering{err: verifyErr}
	req := f.signedRequest(freshNonce())
	req.ATCClaims = nil
	req.ATCID = "an-atc-the-verifier-refuses"
	result, err := f.svc.Resolve(f.agentID, req)
	if result != nil {
		t.Fatal("a request whose ATC was refused was answered with a credential")
	}
	return f, err
}

// A body ATC whose revocation state cannot be read refuses the request with
// ErrResolveATCRevocationUnknown before the backend is read, and the audit
// entry names atc_revocation_unknown.
func TestResolve_BodyATC_RevocationUnknownIsItsOwnRefusal(t *testing.T) {
	f, err := resolveBodyATC(t, &atcdomain.ATCError{Code: atcdomain.ErrCodeRevocationUnknown, Message: "state unknown"})

	if !errors.Is(err, ErrResolveATCRevocationUnknown) {
		t.Fatalf("expected ErrResolveATCRevocationUnknown, got %v", err)
	}
	var atcErr *atcdomain.ATCError
	if !errors.As(err, &atcErr) || atcErr.Code != atcdomain.ErrCodeRevocationUnknown {
		t.Fatalf("the refusal does not carry the verifier's ATC error: %v", err)
	}
	if f.backend.retrieves != 0 {
		t.Fatalf("the backend was read %d times for a refused request", f.backend.retrieves)
	}
	if len(f.audit.entries) != 1 || !strings.Contains(f.audit.entries[0].DenyReason, atcdomain.ErrCodeRevocationUnknown) {
		t.Fatalf("the audit entry does not name %s: %+v", atcdomain.ErrCodeRevocationUnknown, f.audit.entries)
	}
}

// A revoked body ATC is refused, and its refusal is not the unknown one.
func TestResolve_BodyATC_RevokedIsRefused(t *testing.T) {
	f, err := resolveBodyATC(t, &atcdomain.ATCError{Code: atcdomain.ErrCodeRevoked, Message: "revoked"})

	if err == nil {
		t.Fatal("a revoked body ATC was not refused")
	}
	if errors.Is(err, ErrResolveATCRevocationUnknown) {
		t.Fatalf("a revoked ATC was refused as unknown: %v", err)
	}
	if f.backend.retrieves != 0 {
		t.Fatalf("the backend was read %d times for a refused request", f.backend.retrieves)
	}
	if len(f.audit.entries) != 1 || !strings.Contains(f.audit.entries[0].DenyReason, atcdomain.ErrCodeRevoked) {
		t.Fatalf("the audit entry does not name %s: %+v", atcdomain.ErrCodeRevoked, f.audit.entries)
	}
}
