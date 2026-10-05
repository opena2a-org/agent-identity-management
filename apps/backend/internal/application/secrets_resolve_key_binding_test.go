package application

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// plantedCredential is a synthetic value stored in the test backend.
const plantedCredential = "synthetic-credential-for-key-binding-test"

// decryptResolvedBlob opens a resolve result with the X25519 key derived from
// an Ed25519 private key, the way the SDKs do.
func decryptResolvedBlob(priv ed25519.PrivateKey, result *secrets.ResolutionResult) ([]byte, error) {
	h := sha512.Sum512(priv.Seed())
	xPriv, err := ecdh.X25519().NewPrivateKey(h[:32])
	if err != nil {
		return nil, err
	}
	ephemeral, err := ecdh.X25519().NewPublicKey(result.EphemeralPubKey)
	if err != nil {
		return nil, err
	}
	shared, err := xPriv.ECDH(ephemeral)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(shared)
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, err
	}
	if len(result.EncryptedBlob) < aead.NonceSize() {
		return nil, fmt.Errorf("blob shorter than the AEAD nonce")
	}
	n := aead.NonceSize()
	return aead.Open(nil, result.EncryptedBlob[:n], result.EncryptedBlob[n:], nil)
}

// fakeResolveNonceAdmitter stands in for the admission store: it records
// each (agent, key) pair once. The window on the database clock is covered by
// the store's own integration tests, so this fake does not check time.
type fakeResolveNonceAdmitter struct {
	seen    map[string]bool
	calls   int
	err     error
	outcome domain.ActionRequestAdmission // when set, returned instead
}

func newFakeResolveNonceAdmitter() *fakeResolveNonceAdmitter {
	return &fakeResolveNonceAdmitter{seen: make(map[string]bool)}
}

func (a *fakeResolveNonceAdmitter) Admit(_ context.Context, agentID, _ uuid.UUID, nonce []byte, _ time.Time) (domain.ActionRequestAdmission, error) {
	a.calls++
	if a.err != nil {
		return 0, a.err
	}
	if a.outcome != 0 {
		return a.outcome, nil
	}
	if len(nonce) != 16 {
		return 0, fmt.Errorf("admission key is %d bytes, the store holds 16", len(nonce))
	}
	k := agentID.String() + "|" + string(nonce)
	if a.seen[k] {
		return domain.ActionRequestNonceReused, nil
	}
	a.seen[k] = true
	return domain.ActionRequestAdmitted, nil
}

// resolveFixture is an agent with a namespace, a planted credential and the
// claims ATCAuthMiddleware sets for it.
type resolveFixture struct {
	svc      *SecretsService
	backend  *mockBackend
	admitter *fakeResolveNonceAdmitter
	audit    *mockAuditRepo
	agentID  uuid.UUID
	pub      ed25519.PublicKey
	priv     ed25519.PrivateKey
	claims   *atcdomain.ATCClaims
}

func newResolveFixture(t *testing.T) *resolveFixture {
	t.Helper()
	svc, nsRepo, auditRepo, agentRepo, _, _, backend := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	if err := nsRepo.Create(ns); err != nil {
		t.Fatal(err)
	}
	if err := backend.Store(ns.ID, []byte(plantedCredential), "AES-256-GCM"); err != nil {
		t.Fatal(err)
	}

	return &resolveFixture{
		svc:      svc,
		backend:  backend,
		admitter: svc.nonceAdmitter.(*fakeResolveNonceAdmitter),
		audit:    auditRepo,
		agentID:  agentID,
		pub:      pub,
		priv:     priv,
		claims: &atcdomain.ATCClaims{
			AgentID:      agentID,
			Issuer:       "aim-test",
			Capabilities: []string{CapabilitySecretsResolve},
			ATCID:        "atc-key-binding",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		},
	}
}

// signedRequest is a resolve request the agent signed, on the ATC route.
func (f *resolveFixture) signedRequest(nonce string) *secrets.ResolutionRequest {
	return &secrets.ResolutionRequest{
		Namespace:      "github",
		Operation:      "read",
		Nonce:          nonce,
		Signature:      signRequest(f.priv, "github", "read", nonce),
		AgentPublicKey: f.pub,
		ATCClaims:      f.claims,
	}
}

func freshNonce() string {
	return time.Now().UTC().Format(time.RFC3339Nano) + ":" + uuid.New().String()
}

// A request whose signed fields are copied and whose agentPublicKey is
// replaced must not return the credential encrypted to the replacement key,
// and must be refused before the backend is read.
func TestResolve_ATC_SubstitutedAgentPublicKeyIsRefusedBeforeRetrieval(t *testing.T) {
	f := newResolveFixture(t)
	otherPub, otherPriv, _ := ed25519.GenerateKey(rand.Reader)

	copied := f.signedRequest(freshNonce())
	copied.AgentPublicKey = otherPub

	result, err := f.svc.Resolve(f.agentID, copied)

	substitutedKeyDecrypts := false
	if result != nil {
		if plaintext, decErr := decryptResolvedBlob(otherPriv, result); decErr == nil && string(plaintext) == plantedCredential {
			substitutedKeyDecrypts = true
		}
	}
	t.Logf("refused=%t retrieves=%d substitutedKeyDecrypts=%t", err != nil, f.backend.retrieves, substitutedKeyDecrypts)

	if err == nil {
		t.Fatal("a request with a substituted agentPublicKey was answered")
	}
	if substitutedKeyDecrypts {
		t.Fatal("the substituted key decrypts the planted credential")
	}
	if f.backend.retrieves != 0 {
		t.Fatalf("backend Retrieve called %d times, want 0", f.backend.retrieves)
	}
	if f.admitter.calls != 0 {
		t.Fatalf("admission ran %d times, want 0: a refused copy must not spend the agent's nonce", f.admitter.calls)
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Result != secrets.AuditResultDenied {
		t.Fatal("want one denied audit entry")
	}
}

// A nonce must carry a parseable RFC 3339 time inside the window; one that
// does not is refused, never let through unchecked.
func TestResolve_NonceTimeIsRequiredAndChecked(t *testing.T) {
	cases := []struct {
		name  string
		nonce string
	}{
		{"no time", "not-a-time:" + uuid.New().String()},
		{"no separator", uuid.New().String()},
		{"time only", time.Now().UTC().Format(time.RFC3339Nano)},
		{"empty random part", time.Now().UTC().Format(time.RFC3339Nano) + ":"},
		{"older than the window", time.Now().Add(-60*time.Second).UTC().Format(time.RFC3339Nano) + ":" + uuid.New().String()},
		{"older than the window, offset form", time.Now().Add(-60*time.Second).In(time.FixedZone("", -6*3600)).Format(time.RFC3339Nano) + ":" + uuid.New().String()},
		{"ahead of the window", time.Now().Add(60*time.Second).UTC().Format(time.RFC3339Nano) + ":" + uuid.New().String()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newResolveFixture(t)
			_, err := f.svc.Resolve(f.agentID, f.signedRequest(tc.nonce))
			t.Logf("refused=%t retrieves=%d", err != nil, f.backend.retrieves)
			if err == nil {
				t.Fatal("resolve answered")
			}
			if f.backend.retrieves != 0 {
				t.Fatalf("backend Retrieve called %d times, want 0", f.backend.retrieves)
			}
			if f.admitter.calls != 0 {
				t.Fatalf("admission ran %d times, want 0", f.admitter.calls)
			}
		})
	}
}

// The same signed request sent twice is answered once.
func TestResolve_SignedRequestIsAdmittedOnce(t *testing.T) {
	f := newResolveFixture(t)
	req := f.signedRequest(freshNonce())

	if _, err := f.svc.Resolve(f.agentID, req); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	copied := *req
	_, err := f.svc.Resolve(f.agentID, &copied)
	t.Logf("secondRefused=%t retrieves=%d", err != nil, f.backend.retrieves)
	if err == nil {
		t.Fatal("a repeated signed request was answered")
	}
	if f.backend.retrieves != 1 {
		t.Fatalf("backend Retrieve called %d times, want 1", f.backend.retrieves)
	}
}

// The credential is encrypted to the registered key, whether the request
// names that key or omits it.
func TestResolve_EncryptsToTheRegisteredKey(t *testing.T) {
	for _, omit := range []bool{false, true} {
		t.Run(fmt.Sprintf("agentPublicKey omitted=%t", omit), func(t *testing.T) {
			f := newResolveFixture(t)
			req := f.signedRequest(freshNonce())
			if omit {
				req.AgentPublicKey = nil
			}
			result, err := f.svc.Resolve(f.agentID, req)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			plaintext, err := decryptResolvedBlob(f.priv, result)
			if err != nil {
				t.Fatalf("the registered key does not open the result: %v", err)
			}
			if string(plaintext) != plantedCredential {
				t.Fatal("decrypted value is not the planted credential")
			}
			if f.admitter.calls != 1 || f.backend.retrieves != 1 {
				t.Fatalf("admissions=%d retrieves=%d, want 1 and 1", f.admitter.calls, f.backend.retrieves)
			}
		})
	}
}

// Two requests that differ only in the signed nonce are both answered: the
// admission key is per signed request.
func TestResolve_DistinctNoncesAreEachAdmitted(t *testing.T) {
	f := newResolveFixture(t)
	for i := 0; i < 2; i++ {
		if _, err := f.svc.Resolve(f.agentID, f.signedRequest(freshNonce())); err != nil {
			t.Fatalf("resolve %d: %v", i, err)
		}
	}
	if f.backend.retrieves != 2 {
		t.Fatalf("backend Retrieve called %d times, want 2", f.backend.retrieves)
	}
}

// When the single-use check cannot run, or the store refuses the time, the
// request is refused and the backend is not read.
func TestResolve_AdmissionRefusalsAndFailures(t *testing.T) {
	storeErr := errors.New("store down")
	cases := []struct {
		name            string
		configure       func(f *resolveFixture)
		wantUnavailable bool
	}{
		{"no store", func(f *resolveFixture) { f.svc.nonceAdmitter = nil }, true},
		{"store error", func(f *resolveFixture) { f.admitter.err = storeErr }, true},
		{"purge not running", func(f *resolveFixture) { f.admitter.err = domain.ErrActionRequestNoncePurgeNotRunning }, true},
		{"behind the database clock", func(f *resolveFixture) { f.admitter.outcome = domain.ActionRequestBehindClock }, false},
		{"ahead of the database clock", func(f *resolveFixture) { f.admitter.outcome = domain.ActionRequestAheadOfClock }, false},
		{"already used", func(f *resolveFixture) { f.admitter.outcome = domain.ActionRequestNonceReused }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newResolveFixture(t)
			tc.configure(f)
			_, err := f.svc.Resolve(f.agentID, f.signedRequest(freshNonce()))
			if err == nil {
				t.Fatal("resolve answered")
			}
			if got := errors.Is(err, ErrSecretsNonceAdmissionUnavailable); got != tc.wantUnavailable {
				t.Fatalf("errors.Is(err, ErrSecretsNonceAdmissionUnavailable) = %t, want %t (%v)", got, tc.wantUnavailable, err)
			}
			if f.backend.retrieves != 0 {
				t.Fatalf("backend Retrieve called %d times, want 0", f.backend.retrieves)
			}
			if len(f.audit.entries) != 1 || f.audit.entries[0].Result != secrets.AuditResultDenied {
				t.Fatal("want one denied audit entry")
			}
		})
	}
}

// The admission key is 16 bytes, depends on every signed field, and is not
// the raw nonce.
func TestResolveNonceKey(t *testing.T) {
	a := resolveNonceKey([]byte("github|read|n1"))
	if len(a) != 16 {
		t.Fatalf("key length %d, want 16", len(a))
	}
	for _, other := range []string{"gitlab|read|n1", "github|write|n1", "github|read|n2"} {
		if string(resolveNonceKey([]byte(other))) == string(a) {
			t.Fatalf("%q has the same key", other)
		}
	}
	plain := sha256.Sum256([]byte("github|read|n1"))
	if string(plain[:16]) == string(a) {
		t.Fatal("the key is an unlabelled digest")
	}
}

func TestParseResolveNonceTime(t *testing.T) {
	ts := time.Date(2026, 10, 5, 14, 22, 0, 123456789, time.UTC)
	for _, nonce := range []string{
		ts.Format(time.RFC3339Nano) + ":" + uuid.New().String(),
		ts.In(time.FixedZone("", -6*3600)).Format(time.RFC3339Nano) + ":" + uuid.New().String(),
		"2026-10-05T14:22:00.123456789Z:abc",
	} {
		got, err := parseResolveNonceTime(nonce)
		if err != nil {
			t.Fatalf("%q: %v", nonce, err)
		}
		if !got.Equal(ts) {
			t.Fatalf("%q: got %s, want %s", nonce, got, ts)
		}
	}
	for _, nonce := range []string{"", ":", "abc", "abc:def", "2026-10-05T14:22:00Z", "2026-10-05T14:22:00Z:", ":abc", "now:abc", "2026-10-05 14:22:00Z:abc"} {
		if _, err := parseResolveNonceTime(nonce); err == nil {
			t.Fatalf("%q parsed", nonce)
		}
	}
}
