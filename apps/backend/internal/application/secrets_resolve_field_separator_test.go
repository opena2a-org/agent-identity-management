package application

import (
	"errors"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// A request signed by the agent over namespace|operation|nonce, whose
// namespace or operation holds a vertical bar, a carriage return or a line
// feed, is refused with ErrResolveFieldSeparator before its nonce is spent and
// before the backend is read, and the denied audit entry names the field.
func TestResolve_FieldHoldingASeparatorIsRefusedBeforeTheNonceIsSpent(t *testing.T) {
	cases := []struct {
		name                 string
		namespace, operation string
		denyReason           string
	}{
		{"namespace holding a vertical bar", "git|hub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"namespace holding a carriage return", "git\rhub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"namespace holding a line feed", "git\nhub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"operation holding a vertical bar", "github", "re|ad", "operation contains a vertical bar, a carriage return or a line feed"},
		{"operation holding a carriage return", "github", "re\rad", "operation contains a vertical bar, a carriage return or a line feed"},
		{"operation holding a line feed", "github", "re\nad", "operation contains a vertical bar, a carriage return or a line feed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newResolveFixture(t)
			nonce := freshNonce()
			req := f.signedRequest(nonce)
			req.Namespace, req.Operation = tc.namespace, tc.operation
			req.Signature = signRequest(f.priv, tc.namespace, tc.operation, nonce)

			_, err := f.svc.Resolve(f.agentID, req)
			if !errors.Is(err, ErrResolveFieldSeparator) {
				t.Fatalf("err = %v, want ErrResolveFieldSeparator", err)
			}
			if f.admitter.calls != 0 || f.backend.retrieves != 0 {
				t.Fatalf("admissions=%d retrieves=%d, want 0 and 0", f.admitter.calls, f.backend.retrieves)
			}
			if len(f.audit.entries) != 1 || f.audit.entries[0].Result != secrets.AuditResultDenied {
				t.Fatal("want one denied audit entry")
			}
			if got := f.audit.entries[0].DenyReason; got != tc.denyReason {
				t.Fatalf("deny reason %q, want %q", got, tc.denyReason)
			}
		})
	}
}

// The text git|hub|read|<nonce>, signed once, reads as namespace "git|hub"
// with operation "read" and as namespace "git" with operation "hub|read".
// Neither reading is answered, and the nonce is not spent.
func TestResolve_OneSignedTextIsNotAnsweredAsTwoSetsOfFields(t *testing.T) {
	f := newResolveFixture(t)
	nonce := freshNonce()
	signature := signRequest(f.priv, "git|hub", "read", nonce)

	for _, fields := range [][2]string{{"git|hub", "read"}, {"git", "hub|read"}} {
		req := f.signedRequest(nonce)
		req.Namespace, req.Operation, req.Signature = fields[0], fields[1], signature
		if _, err := f.svc.Resolve(f.agentID, req); !errors.Is(err, ErrResolveFieldSeparator) {
			t.Fatalf("namespace %q, operation %q: err = %v, want ErrResolveFieldSeparator", fields[0], fields[1], err)
		}
	}
	if f.admitter.calls != 0 || f.backend.retrieves != 0 {
		t.Fatalf("admissions=%d retrieves=%d, want 0 and 0", f.admitter.calls, f.backend.retrieves)
	}
}
