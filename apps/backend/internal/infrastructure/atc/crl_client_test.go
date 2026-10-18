package atc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
)

const (
	testListPath     = "/api/v1/trust/revocations"
	testOverridePath = "/mirror/revocations"
	testListKeyID    = "did:web:registry.example#list-1"
	revokedDID       = "did:web:agents.example:revoked-agent"
	otherDID         = "did:web:agents.example:other-agent"
)

// crlSigner signs lists under one key.
type crlSigner struct {
	keyID     string
	algorithm string
	public    []byte
	sign      func(message []byte) []byte
}

func newEd25519Signer(t *testing.T, keyID string) crlSigner {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return crlSigner{keyID: keyID, algorithm: "Ed25519", public: public, sign: func(message []byte) []byte {
		return ed25519.Sign(private, message)
	}}
}

func newMLDSA65Signer(t *testing.T, keyID string) crlSigner {
	t.Helper()
	pair, err := pqc.GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return crlSigner{keyID: keyID, algorithm: "ML-DSA-65", public: pair.PublicKey, sign: func(message []byte) []byte {
		signature, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, pair.PrivateKey, message)
		if err != nil {
			t.Fatal(err)
		}
		return signature
	}}
}

// testList returns a list that revokes dids, with thisUpdate a minute before
// at and nextUpdate four minutes after it.
func testList(at time.Time, dids ...string) map[string]any {
	entries := make([]any, 0, len(dids))
	for i, did := range dids {
		entries = append(entries, map[string]any{
			"agentDid":             did,
			"reason":               "compromised",
			"revokedAt":            at.Add(-time.Hour).UTC().Format(time.RFC3339),
			"revokedByKeyId":       "did:web:registry.example#operator-1",
			"transparencyLogIndex": i + 1,
		})
	}
	return map[string]any{
		"revocations": entries,
		"entryCount":  len(entries),
		"thisUpdate":  at.Add(-time.Minute).UTC().Format(time.RFC3339),
		"nextUpdate":  at.Add(4 * time.Minute).UTC().Format(time.RFC3339),
	}
}

func firstEntry(doc map[string]any) map[string]any {
	return doc["revocations"].([]any)[0].(map[string]any)
}

// signList returns the body of doc with one signature by each signer over the
// canonical form of doc without its signatures member.
func signList(t *testing.T, doc map[string]any, signers ...crlSigner) []byte {
	t.Helper()
	unsigned := map[string]any{}
	for name, value := range doc {
		if name != "signatures" {
			unsigned[name] = value
		}
	}
	raw, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	signed := mustCanonicalJSON(t, raw)
	signatures := []any{}
	for _, s := range signers {
		signatures = append(signatures, map[string]any{
			"keyId":     s.keyID,
			"algorithm": s.algorithm,
			"value":     base64.StdEncoding.EncodeToString(s.sign(signed)),
		})
	}
	unsigned["signatures"] = signatures
	body, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// editList decodes body, applies edit, and encodes the result, so a signed
// list can be changed after it was signed.
func editList(t *testing.T, body []byte, edit func(doc map[string]any)) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func signatureAt(doc map[string]any, i int) map[string]any {
	return doc["signatures"].([]any)[i].(map[string]any)
}

func discoveryFor(signers ...crlSigner) map[string]any {
	keys := []any{}
	for _, s := range signers {
		keys = append(keys, map[string]any{"keyId": s.keyID, "algorithm": s.algorithm, "publicKeyHex": hex.EncodeToString(s.public)})
	}
	return map[string]any{"endpoints": map[string]any{"revocations": testListPath}, "publicKeys": keys}
}

// crlAuthority is a TLS stub of an authority listening on 127.0.0.1. It
// serves a discovery document at /.well-known/atp and /.well-known/opena2a,
// and the list at testListPath and testOverridePath. When an ETag is set for
// the list (etag) or the discovery document (discoveryETag), a request for it
// whose If-None-Match carries that ETag is answered 304.
type crlAuthority struct {
	srv *httptest.Server

	mu             sync.Mutex
	discovery      []byte
	discoveryETag  string
	discoveryReads []discoveryRead
	atpServed      bool
	aliasServed    bool
	list           []byte
	listStatus     int
	etag           string
	redirect       string
	listPaths      []string
	ifNoneMatch    []string
}

// discoveryRead is a request for the discovery document: its path and the
// If-None-Match it carried.
type discoveryRead struct {
	path, ifNoneMatch string
}

func newCRLAuthority(t *testing.T, discovery map[string]any) *crlAuthority {
	t.Helper()
	a := &crlAuthority{atpServed: true, aliasServed: true, listStatus: http.StatusOK}
	a.setDiscovery(t, discovery)
	serveDiscovery := func(served *bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			a.mu.Lock()
			a.discoveryReads = append(a.discoveryReads, discoveryRead{path: r.URL.Path, ifNoneMatch: r.Header.Get("If-None-Match")})
			ok, body, etag := *served, a.discovery, a.discoveryETag
			a.mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			if etag != "" {
				if r.Header.Get("If-None-Match") == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set("ETag", etag)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		}
	}
	serveList := func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.listPaths = append(a.listPaths, r.URL.Path)
		a.ifNoneMatch = append(a.ifNoneMatch, r.Header.Get("If-None-Match"))
		status, body, etag, redirect := a.listStatus, a.list, a.etag, a.redirect
		a.mu.Unlock()
		if redirect != "" {
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
		if etag != "" {
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/atp", serveDiscovery(&a.atpServed))
	mux.HandleFunc("/.well-known/opena2a", serveDiscovery(&a.aliasServed))
	mux.HandleFunc(testListPath, serveList)
	mux.HandleFunc(testOverridePath, serveList)
	a.srv = httptest.NewTLSServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

func (a *crlAuthority) setDiscovery(t *testing.T, doc map[string]any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.discovery = body
	a.mu.Unlock()
}

func (a *crlAuthority) set(edit func(a *crlAuthority)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	edit(a)
}

func (a *crlAuthority) setList(body []byte) { a.set(func(a *crlAuthority) { a.list = body }) }

func (a *crlAuthority) requests() (paths, ifNoneMatch []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.listPaths...), append([]string(nil), a.ifNoneMatch...)
}

func (a *crlAuthority) discoveryRequests() []discoveryRead {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]discoveryRead(nil), a.discoveryReads...)
}

// newTestCRLClient returns a client of a whose clock reads *clock.
func newTestCRLClient(t *testing.T, a *crlAuthority, override string, clock *time.Time) *CachedCRLClient {
	t.Helper()
	c, err := NewCachedCRLClient(a.srv.URL, override, a.srv.Client())
	if err != nil {
		t.Fatalf("NewCachedCRLClient: %v", err)
	}
	c.now = func() time.Time { return *clock }
	return c
}

func requireRefreshOutcome(t *testing.T, err error, outcome string) {
	t.Helper()
	var refreshErr *CRLRefreshError
	if !errors.As(err, &refreshErr) {
		t.Fatalf("Refresh returned %v, want a refusal with outcome %q", err, outcome)
	}
	if refreshErr.Outcome != outcome {
		t.Fatalf("Refresh refused the list with outcome %q (%v), want %q", refreshErr.Outcome, refreshErr.Err, outcome)
	}
}

func requireAnswer(t *testing.T, c *CachedCRLClient, did string, want bool) {
	t.Helper()
	if revoked, err := c.IsRevoked(did); err != nil || revoked != want {
		t.Fatalf("IsRevoked(%q) = %v, %v; want %v, nil", did, revoked, err, want)
	}
}

func requireUnknown(t *testing.T, c *CachedCRLClient, did string) {
	t.Helper()
	if revoked, err := c.IsRevoked(did); !errors.Is(err, ErrCRLStateUnknown) || revoked {
		t.Fatalf("IsRevoked(%q) = %v, %v; want false and an error wrapping ErrCRLStateUnknown", did, revoked, err)
	}
}

// The list address is the endpoints.revocations of the discovery document at
// /.well-known/atp, and the list is held once its signature verifies under
// the key that document lists.
func TestCRLClient_HoldsTheSignedListTheDiscoveryDocumentNames(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now()
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)

	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
	requireAnswer(t, c, otherDID, false)
	if paths, _ := a.requests(); len(paths) != 1 || paths[0] != testListPath {
		t.Fatalf("the list was read at %v, want once at %s", paths, testListPath)
	}
}

// An authority that does not serve /.well-known/atp is read at its alias, and
// one that serves neither is refused as status.
func TestCRLClient_ReadsTheDiscoveryAliasWhenTheATPPathAnswers404(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	a.set(func(a *crlAuthority) { a.atpServed = false })
	now := time.Now()
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)

	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)

	a.set(func(a *crlAuthority) { a.aliasServed = false })
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeStatus)
}

// The endpoint override replaces the list address; the keys still come from
// the discovery document, so a list signed by a key it does not list is
// refused at the override as well.
func TestCRLClient_EndpointOverrideReplacesOnlyTheListAddress(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now()
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, a.srv.URL+testOverridePath, &now)

	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
	if paths, _ := a.requests(); len(paths) != 1 || paths[0] != testOverridePath {
		t.Fatalf("the list was read at %v, want once at %s", paths, testOverridePath)
	}

	stranger := newEd25519Signer(t, "did:web:elsewhere.example#key-1")
	a.setList(signList(t, testList(now, otherDID), stranger))
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeSignature)
	requireAnswer(t, c, otherDID, false)
}

// Without a list that passed its checks there is no answer: before the first
// refresh, after a first refresh the list address answers 500, refused as
// status, and after one where nothing listens, refused as fetch, IsRevoked is
// unknown and never not revoked.
func TestCRLClient_WithoutAListThatPassedItsChecksTheStateIsUnknown(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now()
	c := newTestCRLClient(t, a, "", &now)
	requireUnknown(t, c, otherDID)

	a.set(func(a *crlAuthority) { a.listStatus = http.StatusInternalServerError })
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeStatus)
	requireUnknown(t, c, otherDID)

	a.srv.Close()
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeFetch)
	requireUnknown(t, c, otherDID)
}

// A list is held only when a signature by a listed Ed25519 key covers it, and
// no signature by a listed key fails.
func TestCRLClient_RefusesAListItsSignaturesDoNotCover(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	stranger := newEd25519Signer(t, "did:web:elsewhere.example#key-1")
	now := time.Now()
	cases := []struct {
		name string
		body func() []byte
	}{
		{"an entry removed after signing", func() []byte {
			body := signList(t, testList(now, revokedDID, otherDID), signer)
			return editList(t, body, func(doc map[string]any) {
				doc["revocations"] = doc["revocations"].([]any)[1:]
				doc["entryCount"] = 1
			})
		}},
		{"signed only by a key the discovery document does not list", func() []byte {
			return signList(t, testList(now, revokedDID), stranger)
		}},
		{"no signatures member", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), signer), func(doc map[string]any) { delete(doc, "signatures") })
		}},
		{"an empty signatures array", func() []byte {
			return signList(t, testList(now, revokedDID))
		}},
		{"a second signature by the listed key that does not verify", func() []byte {
			body := signList(t, testList(now, revokedDID), signer, signer)
			return editList(t, body, func(doc map[string]any) {
				signatureAt(doc, 1)["value"] = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
			})
		}},
		{"a signature by the listed key that names another algorithm", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), signer), func(doc map[string]any) {
				signatureAt(doc, 0)["algorithm"] = "ML-DSA-65"
			})
		}},
		{"a signature value that is not base64", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), signer), func(doc map[string]any) {
				signatureAt(doc, 0)["value"] = "not base64!"
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(tc.body())
			c := newTestCRLClient(t, a, "", &now)
			requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeSignature)
			requireUnknown(t, c, revokedDID)
		})
	}
}

// An ML-DSA-65 signature by a listed key is verified beside the Ed25519 one,
// and is not enough alone.
func TestCRLClient_VerifiesTheMLDSA65SignatureOfAListedKey(t *testing.T) {
	ed := newEd25519Signer(t, testListKeyID)
	pq := newMLDSA65Signer(t, "did:web:registry.example#list-pq-1")
	now := time.Now()
	cases := []struct {
		name    string
		body    func() []byte
		outcome string
	}{
		{"Ed25519 and ML-DSA-65 signatures that verify", func() []byte {
			return signList(t, testList(now, revokedDID), ed, pq)
		}, ""},
		{"an ML-DSA-65 signature that does not verify", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), ed, pq), func(doc map[string]any) {
				value, _ := base64.StdEncoding.DecodeString(signatureAt(doc, 1)["value"].(string))
				value[0] ^= 0xff
				signatureAt(doc, 1)["value"] = base64.StdEncoding.EncodeToString(value)
			})
		}, CRLOutcomeSignature},
		{"an ML-DSA-65 signature alone", func() []byte {
			return signList(t, testList(now, revokedDID), pq)
		}, CRLOutcomeSignature},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newCRLAuthority(t, discoveryFor(ed, pq))
			a.setList(tc.body())
			c := newTestCRLClient(t, a, "", &now)
			err := c.Refresh(context.Background())
			if tc.outcome == "" {
				if err != nil {
					t.Fatalf("Refresh: %v", err)
				}
				requireAnswer(t, c, revokedDID, true)
				return
			}
			requireRefreshOutcome(t, err, tc.outcome)
		})
	}
}

// A signature whose keyId the discovery document does not list is not read:
// beside a verified Ed25519 signature by a listed key, a signature by another
// key whose value does not verify leaves the list held.
func TestCRLClient_ReadsNoSignatureByAKeyTheDiscoveryDocumentDoesNotList(t *testing.T) {
	listed := newEd25519Signer(t, testListKeyID)
	unlisted := newEd25519Signer(t, "did:web:registry.example#list-unlisted")
	now := time.Now()
	body := editList(t, signList(t, testList(now, revokedDID), listed, unlisted), func(doc map[string]any) {
		signatureAt(doc, 1)["value"] = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	})
	a := newCRLAuthority(t, discoveryFor(listed))
	a.setList(body)
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
}

// A list that breaks the form of the list contract is refused as shape, and a
// body that is not an I-JSON text (RFC 7493) as parse, even when a listed key
// signed it.
func TestCRLClient_RefusesAListThatBreaksTheContractsForm(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	signed := func(edit func(doc map[string]any)) func() []byte {
		return func() []byte {
			doc := testList(now, revokedDID)
			edit(doc)
			return signList(t, doc, signer)
		}
	}
	cases := []struct {
		name string
		body func() []byte
	}{
		{"entryCount above the entries", signed(func(doc map[string]any) { doc["entryCount"] = 2 })},
		{"entryCount below the entries", signed(func(doc map[string]any) { doc["entryCount"] = 0 })},
		{"no entryCount", signed(func(doc map[string]any) { delete(doc, "entryCount") })},
		{"an entryCount written as a string", signed(func(doc map[string]any) { doc["entryCount"] = "1" })},
		{"no revocations member", signed(func(doc map[string]any) { delete(doc, "revocations"); doc["entryCount"] = 0 })},
		{"revocations that is not an array", signed(func(doc map[string]any) { doc["revocations"] = map[string]any{} })},
		{"an agentDid of 513 bytes", signed(func(doc map[string]any) { firstEntry(doc)["agentDid"] = "did:web:" + strings.Repeat("a", 505) })},
		{"an empty agentDid", signed(func(doc map[string]any) { firstEntry(doc)["agentDid"] = "" })},
		{"no agentDid", signed(func(doc map[string]any) { delete(firstEntry(doc), "agentDid") })},
		{"a revokedAt written with an offset", signed(func(doc map[string]any) { firstEntry(doc)["revokedAt"] = "2026-10-17T10:00:00+00:00" })},
		{"a revokedAt that is not a time", signed(func(doc map[string]any) { firstEntry(doc)["revokedAt"] = "yesterday" })},
		{"no revokedAt", signed(func(doc map[string]any) { delete(firstEntry(doc), "revokedAt") })},
		{"a reason that is not a string", signed(func(doc map[string]any) { firstEntry(doc)["reason"] = 7 })},
		{"a revokedByKeyId of 577 bytes", signed(func(doc map[string]any) { firstEntry(doc)["revokedByKeyId"] = strings.Repeat("k", 577) })},
		{"a transparencyLogIndex of 2^63", signed(func(doc map[string]any) { firstEntry(doc)["transparencyLogIndex"] = json.Number("9223372036854775808") })},
		{"a negative transparencyLogIndex", signed(func(doc map[string]any) { firstEntry(doc)["transparencyLogIndex"] = -1 })},
		{"a transparencyLogIndex with a fraction", signed(func(doc map[string]any) { firstEntry(doc)["transparencyLogIndex"] = 1.5 })},
		{"an entry longer than 1258 bytes", signed(func(doc map[string]any) { firstEntry(doc)["note"] = strings.Repeat("n", maxCRLEntryBytes) })},
		{"a thisUpdate that is not RFC 3339", signed(func(doc map[string]any) { doc["thisUpdate"] = "2026-10-17 10:00:00" })},
		{"no nextUpdate", signed(func(doc map[string]any) { delete(doc, "nextUpdate") })},
		{"signatures that is not an array", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), signer), func(doc map[string]any) { doc["signatures"] = "signed" })
		}},
		{"a signature without keyId", func() []byte {
			return editList(t, signList(t, testList(now, revokedDID), signer), func(doc map[string]any) { delete(signatureAt(doc, 0), "keyId") })
		}},
		{"a member named twice", func() []byte {
			return append([]byte(`{"entryCount":1,`), signList(t, testList(now, revokedDID), signer)[1:]...)
		}},
		{"a string that is not UTF-8", func() []byte {
			return bytes.Replace(signList(t, testList(now, revokedDID), signer), []byte("compromised"), []byte("compro\xffmised"), 1)
		}},
		{"data after the list", func() []byte {
			return append(signList(t, testList(now, revokedDID), signer), []byte(`{}`)...)
		}},
		{"an array where the list belongs", func() []byte { return []byte(`[]`) }},
		{"an entry that names a member twice", func() []byte {
			return bytes.Replace(signList(t, testList(now, revokedDID), signer),
				[]byte(`"reason":"compromised"`), []byte(`"reason":"compromised","reason":"compromised"`), 1)
		}},
		{"a number beyond the binary64 range", func() []byte {
			return bytes.Replace(signList(t, testList(now, revokedDID), signer), []byte(`"entryCount":1,`), []byte(`"entryCount":1e999,`), 1)
		}},
		{"a list cut short", func() []byte {
			body := signList(t, testList(now, revokedDID), signer)
			return body[:len(body)/2]
		}},
		{"a body that is not JSON", func() []byte { return []byte(`<html><body>Service Unavailable</body></html>`) }},
	}
	// These bodies are not I-JSON texts, and are refused as parse.
	notIJSON := map[string]bool{
		"a member named twice":               true,
		"a string that is not UTF-8":         true,
		"data after the list":                true,
		"an entry that names a member twice": true,
		"a number beyond the binary64 range": true,
		"a list cut short":                   true,
		"a body that is not JSON":            true,
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(tc.body())
			c := newTestCRLClient(t, a, "", &now)
			outcome := CRLOutcomeShape
			if notIJSON[tc.name] {
				outcome = CRLOutcomeParse
			}
			requireRefreshOutcome(t, c.Refresh(context.Background()), outcome)
			requireUnknown(t, c, revokedDID)
		})
	}
}

// Values at the bounds of the list contract, and entries without the
// optional reason, revokedByKeyId and transparencyLogIndex, are held.
func TestCRLClient_HoldsAListAtTheContractsBounds(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	didAtTheBound := "did:web:" + strings.Repeat("a", maxCRLAgentDIDBytes-len("did:web:"))
	cases := []struct {
		name string
		edit func(doc map[string]any)
		did  string
	}{
		{"an agentDid of 512 bytes", func(doc map[string]any) { firstEntry(doc)["agentDid"] = didAtTheBound }, didAtTheBound},
		{"a revokedByKeyId of 576 bytes", func(doc map[string]any) { firstEntry(doc)["revokedByKeyId"] = strings.Repeat("k", 576) }, revokedDID},
		{"a transparencyLogIndex of 2^63-1", func(doc map[string]any) {
			firstEntry(doc)["transparencyLogIndex"] = json.Number("9223372036854775807")
		}, revokedDID},
		{"a revokedAt with fractional seconds", func(doc map[string]any) { firstEntry(doc)["revokedAt"] = "2026-10-17T10:00:00.123456Z" }, revokedDID},
		{"no reason, revokedByKeyId or transparencyLogIndex", func(doc map[string]any) {
			for _, name := range []string{"reason", "revokedByKeyId", "transparencyLogIndex"} {
				delete(firstEntry(doc), name)
			}
		}, revokedDID},
		{"an entry of 1258 bytes", func(doc map[string]any) { padEntry(t, firstEntry(doc), maxCRLEntryBytes) }, revokedDID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := testList(now, revokedDID)
			tc.edit(doc)
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(signList(t, doc, signer))
			c := newTestCRLClient(t, a, "", &now)
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			requireAnswer(t, c, tc.did, true)
		})
	}

	doc := testList(now, revokedDID)
	padEntry(t, firstEntry(doc), maxCRLEntryBytes+1)
	a := newCRLAuthority(t, discoveryFor(signer))
	a.setList(signList(t, doc, signer))
	requireRefreshOutcome(t, newTestCRLClient(t, a, "", &now).Refresh(context.Background()), CRLOutcomeShape)
}

// padEntry adds a note member to entry so that its canonical form is size
// bytes long.
func padEntry(t *testing.T, entry map[string]any, size int) {
	t.Helper()
	entry["note"] = ""
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	entry["note"] = strings.Repeat("n", size-len(mustCanonicalJSON(t, raw)))
	raw, err = json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(mustCanonicalJSON(t, raw)); got != size {
		t.Fatalf("the padded entry is %d bytes, want %d", got, size)
	}
}

// envelopeBytes is the canonical size of the members of a list body other
// than revocations.
func envelopeBytes(t *testing.T, body []byte) int {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatal(err)
	}
	size := 0
	for name, value := range members {
		if name == "revocations" {
			continue
		}
		raw, err := json.Marshal(map[string]json.RawMessage{name: value})
		if err != nil {
			t.Fatal(err)
		}
		size += len(mustCanonicalJSON(t, raw)) - len("{}")
	}
	return size
}

// The members other than revocations, signatures included, may take
// maxCRLEnvelopeBytes and no more.
func TestCRLClient_HoldsAListWhoseOtherMembersTakeTheEnvelopeBound(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	withEnvelope := func(size int) []byte {
		doc := testList(now, revokedDID)
		doc["padding"] = ""
		doc["padding"] = strings.Repeat("p", size-envelopeBytes(t, signList(t, doc, signer)))
		body := signList(t, doc, signer)
		if got := envelopeBytes(t, body); got != size {
			t.Fatalf("the members other than revocations take %d bytes, want %d", got, size)
		}
		return body
	}
	a := newCRLAuthority(t, discoveryFor(signer))
	c := newTestCRLClient(t, a, "", &now)

	a.setList(withEnvelope(maxCRLEnvelopeBytes + 1))
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeShape)

	a.setList(withEnvelope(maxCRLEnvelopeBytes))
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh at the envelope bound: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
}

// revocations may hold maxCRLEntries entries and no more.
func TestCRLClient_HoldsAtMostMaxCRLEntriesEntries(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	dids := make([]string, maxCRLEntries+1)
	for i := range dids {
		dids[i] = fmt.Sprintf("did:web:agents.example:agent-%05d", i)
	}
	a := newCRLAuthority(t, discoveryFor(signer))
	c := newTestCRLClient(t, a, "", &now)

	a.setList(signList(t, testList(now, dids...), signer))
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeShape)

	a.setList(signList(t, testList(now, dids[:maxCRLEntries]...), signer))
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh of %d entries: %v", maxCRLEntries, err)
	}
	requireAnswer(t, c, dids[maxCRLEntries-1], true)
	requireAnswer(t, c, dids[maxCRLEntries], false)
}

// A body longer than maxCRLBodyBytes is refused as shape, and a list padded
// with white space to exactly maxCRLBodyBytes is held.
func TestCRLClient_RefusesABodyLongerThanTheBodyBound(t *testing.T) {
	if maxCRLBodyBytes != 65_536+10_000*1258 {
		t.Fatalf("maxCRLBodyBytes is %d", maxCRLBodyBytes)
	}
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	a := newCRLAuthority(t, discoveryFor(signer))
	c := newTestCRLClient(t, a, "", &now)

	a.setList(bytes.Repeat([]byte(" "), maxCRLBodyBytes+1))
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeShape)

	body := signList(t, testList(now, revokedDID), signer)
	a.setList(append(body, bytes.Repeat([]byte(" "), maxCRLBodyBytes-len(body))...))
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh of a body of maxCRLBodyBytes: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
}

// thisUpdate may be up to 60 s ahead of this server's clock, and a list is
// read up to 60 s after its nextUpdate; past either it is refused as expired.
// A nextUpdate that is not after thisUpdate is refused as shape.
func TestCRLClient_RefusesAListOutsideTheClockSkewBound(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now().UTC().Truncate(time.Second)
	cases := []struct {
		name                   string
		thisUpdate, nextUpdate time.Duration
		outcome                string
	}{
		{"thisUpdate 60 s ahead", 60 * time.Second, 5 * time.Minute, ""},
		{"thisUpdate 61 s ahead", 61 * time.Second, 5 * time.Minute, CRLOutcomeExpired},
		{"nextUpdate 60 s ago", -5 * time.Minute, -60 * time.Second, ""},
		{"nextUpdate 61 s ago", -5 * time.Minute, -61 * time.Second, CRLOutcomeExpired},
		{"nextUpdate equal to thisUpdate", -time.Minute, -time.Minute, CRLOutcomeShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := testList(now, revokedDID)
			doc["thisUpdate"] = now.Add(tc.thisUpdate).Format(time.RFC3339)
			doc["nextUpdate"] = now.Add(tc.nextUpdate).Format(time.RFC3339)
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(signList(t, doc, signer))
			c := newTestCRLClient(t, a, "", &now)
			err := c.Refresh(context.Background())
			if tc.outcome == "" {
				if err != nil {
					t.Fatalf("Refresh: %v", err)
				}
				requireAnswer(t, c, revokedDID, true)
				return
			}
			requireRefreshOutcome(t, err, tc.outcome)
			requireUnknown(t, c, revokedDID)
		})
	}
}

// A list whose thisUpdate is before that of the list held is refused as
// rollback, and the list held stays; a later list replaces it.
func TestCRLClient_RefusesAListOlderThanTheListHeld(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now().UTC().Truncate(time.Second)
	a := newCRLAuthority(t, discoveryFor(signer))
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	older := testList(now, otherDID)
	older["thisUpdate"] = now.Add(-2 * time.Minute).Format(time.RFC3339)
	a.setList(signList(t, older, signer))
	requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeRollback)
	requireAnswer(t, c, revokedDID, true)
	requireAnswer(t, c, otherDID, false)

	a.setList(signList(t, testList(now.Add(time.Minute), otherDID), signer))
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh of a later list: %v", err)
	}
	requireAnswer(t, c, otherDID, true)
	requireAnswer(t, c, revokedDID, false)
}

// The list held answers for five minutes after its last read, and up to 60 s
// after its nextUpdate; past either, the state is unknown.
func TestCRLClient_StateIsUnknownOnceTheListHeldIsStale(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	start := time.Now().UTC().Truncate(time.Second)
	cases := []struct {
		name       string
		nextUpdate time.Duration
		answers    time.Duration
	}{
		{"five minutes after its last read", time.Hour, crlMaxStaleAge},
		{"60 s after its nextUpdate", 2 * time.Minute, 2*time.Minute + crlClockSkew},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := start
			doc := testList(now, revokedDID)
			doc["nextUpdate"] = now.Add(tc.nextUpdate).Format(time.RFC3339)
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(signList(t, doc, signer))
			c := newTestCRLClient(t, a, "", &now)
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			now = start.Add(tc.answers)
			requireAnswer(t, c, revokedDID, true)
			now = start.Add(tc.answers + time.Second)
			requireUnknown(t, c, revokedDID)
		})
	}
}

// A refresh sends the ETag of the list held as If-None-Match, and a 304
// answer counts as a read of that list.
func TestCRLClient_ANotModifiedAnswerCountsAsAReadOfTheListHeld(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	start := time.Now().UTC().Truncate(time.Second)
	now := start
	doc := testList(now, revokedDID)
	doc["nextUpdate"] = now.Add(time.Hour).Format(time.RFC3339)
	a := newCRLAuthority(t, discoveryFor(signer))
	a.setList(signList(t, doc, signer))
	a.set(func(a *crlAuthority) { a.etag = `"list-1"` })
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	now = start.Add(4 * time.Minute)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh answered 304: %v", err)
	}
	if _, ifNoneMatch := a.requests(); len(ifNoneMatch) != 2 || ifNoneMatch[0] != "" || ifNoneMatch[1] != `"list-1"` {
		t.Fatalf("If-None-Match of the list requests: %q", ifNoneMatch)
	}
	now = start.Add(4*time.Minute + crlMaxStaleAge)
	requireAnswer(t, c, revokedDID, true)
	now = start.Add(4*time.Minute + crlMaxStaleAge + time.Second)
	requireUnknown(t, c, revokedDID)
}

// A refresh sends the ETag of the discovery document read before as
// If-None-Match, and a 304 answer reuses the list address and the keys read
// from that document: the document the authority would send in full now names
// another list address and lists no key, and the list is still read at the
// address and checked under the key read before.
func TestCRLClient_ANotModifiedDiscoveryDocumentIsTheDocumentReadBefore(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	start := time.Now().UTC().Truncate(time.Second)
	now := start
	a := newCRLAuthority(t, discoveryFor(signer))
	a.set(func(a *crlAuthority) { a.discoveryETag = `"discovery-1"` })
	a.setList(signList(t, testList(now, otherDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	a.setDiscovery(t, map[string]any{"endpoints": map[string]any{"revocations": testOverridePath}, "publicKeys": []any{}})
	now = start.Add(4 * time.Minute)
	a.setList(signList(t, testList(now, revokedDID), signer))
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh with the discovery document answered 304: %v", err)
	}
	requireAnswer(t, c, revokedDID, true)
	want := []discoveryRead{{"/.well-known/atp", ""}, {"/.well-known/atp", `"discovery-1"`}}
	if got := a.discoveryRequests(); !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery requests (path, If-None-Match): %q, want %q", got, want)
	}
	if paths, _ := a.requests(); !reflect.DeepEqual(paths, []string{testListPath, testListPath}) {
		t.Fatalf("the list was read at %v, want twice at %s", paths, testListPath)
	}
}

// The ETag of a discovery document is sent only to the address that answered
// with it: read at the alias while /.well-known/atp answers 404, it is sent to
// the alias; once /.well-known/atp serves the document, the first request
// there carries none, and the next carries the ETag that address answered.
func TestCRLClient_SendsADiscoveryETagOnlyToTheAddressThatAnsweredWithIt(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	a := newCRLAuthority(t, discoveryFor(signer))
	a.set(func(a *crlAuthority) {
		a.atpServed = false
		a.discoveryETag = `"discovery-1"`
	})
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	refresh := func() {
		t.Helper()
		if err := c.Refresh(context.Background()); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
	}
	refresh()
	refresh()
	a.set(func(a *crlAuthority) { a.atpServed = true })
	refresh()
	refresh()

	want := []discoveryRead{
		{"/.well-known/atp", ""}, {"/.well-known/opena2a", ""},
		{"/.well-known/atp", ""}, {"/.well-known/opena2a", `"discovery-1"`},
		{"/.well-known/atp", ""},
		{"/.well-known/atp", `"discovery-1"`},
	}
	if got := a.discoveryRequests(); !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery requests (path, If-None-Match):\n got %q\nwant %q", got, want)
	}
	requireAnswer(t, c, revokedDID, true)
}

// The list is read with the ETag of the list held only while the discovery
// document lists the keys that list was checked under. When they change, the
// list is read in full and checked under the keys listed now: a list whose
// key was withdrawn, or is listed under its keyId with another public key, is
// refused as signature, the list held is answered from until five minutes
// after its last read, and a list signed by the key listed now is held; a
// list whose key is still listed, beside another, is held again.
func TestCRLClient_ReadsTheListInFullWhenTheDiscoveryDocumentsKeysChange(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	successor := newEd25519Signer(t, "did:web:registry.example#list-2")
	rekeyed := newEd25519Signer(t, testListKeyID)
	cases := []struct {
		name    string
		keys    []crlSigner
		outcome string // empty for a list held again
	}{
		{"the key that signed the list held is withdrawn", []crlSigner{successor}, CRLOutcomeSignature},
		{"its keyId is listed with another public key", []crlSigner{rekeyed}, CRLOutcomeSignature},
		{"another key is listed beside it", []crlSigner{signer, successor}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now().UTC().Truncate(time.Second)
			now := start
			doc := testList(now, revokedDID)
			doc["nextUpdate"] = now.Add(time.Hour).Format(time.RFC3339)
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(signList(t, doc, signer))
			a.set(func(a *crlAuthority) { a.etag = `"list-1"` })
			c := newTestCRLClient(t, a, "", &now)
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh: %v", err)
			}

			a.setDiscovery(t, discoveryFor(tc.keys...))
			now = start.Add(4 * time.Minute)
			err := c.Refresh(context.Background())
			if tc.outcome != "" {
				requireRefreshOutcome(t, err, tc.outcome)
			} else if err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			if _, ifNoneMatch := a.requests(); !reflect.DeepEqual(ifNoneMatch, []string{"", ""}) {
				t.Fatalf("If-None-Match of the list requests: %q, want two requests without it", ifNoneMatch)
			}
			if tc.outcome == "" {
				now = start.Add(4*time.Minute + crlMaxStaleAge)
				requireAnswer(t, c, revokedDID, true)
				return
			}

			now = start.Add(crlMaxStaleAge)
			requireAnswer(t, c, revokedDID, true)
			now = start.Add(crlMaxStaleAge + time.Second)
			requireUnknown(t, c, revokedDID)

			// A list the authority signs under the key it lists now is held.
			a.setList(signList(t, testList(now, otherDID), tc.keys[0]))
			a.set(func(a *crlAuthority) { a.etag = `"list-2"` })
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh of a list signed by the key listed now: %v", err)
			}
			requireAnswer(t, c, otherDID, true)
			requireAnswer(t, c, revokedDID, false)
		})
	}
}

// A discovery document the client cannot use refuses the refresh.
func TestCRLClient_RefusesADiscoveryDocumentItCannotUse(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	key := discoveryFor(signer)["publicKeys"]
	cases := []struct {
		name      string
		discovery map[string]any
		outcome   string
	}{
		{"no endpoints.revocations", map[string]any{"publicKeys": key}, CRLOutcomeShape},
		{"an endpoints.revocations that is not https", map[string]any{
			"endpoints": map[string]any{"revocations": "http://127.0.0.1" + testListPath}, "publicKeys": key,
		}, CRLOutcomeShape},
		{"a keyId listed twice", map[string]any{
			"endpoints":  map[string]any{"revocations": testListPath},
			"publicKeys": append(append([]any{}, key.([]any)...), key.([]any)...),
		}, CRLOutcomeShape},
		{"a publicKeys that is not an array", map[string]any{
			"endpoints": map[string]any{"revocations": testListPath}, "publicKeys": "none",
		}, CRLOutcomeShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newCRLAuthority(t, tc.discovery)
			a.setList(signList(t, testList(now, revokedDID), signer))
			c := newTestCRLClient(t, a, "", &now)
			requireRefreshOutcome(t, c.Refresh(context.Background()), tc.outcome)
			if paths, _ := a.requests(); len(paths) != 0 {
				t.Fatalf("the list was read at %v", paths)
			}
		})
	}
}

// The client follows no redirect from the list address to an address that is
// not https.
func TestCRLClient_FollowsNoRedirectAwayFromHTTPS(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	now := time.Now()
	a := newCRLAuthority(t, discoveryFor(signer))
	a.set(func(a *crlAuthority) { a.redirect = "http://127.0.0.1:9" + testListPath })
	c := newTestCRLClient(t, a, "", &now)

	err := c.Refresh(context.Background())
	requireRefreshOutcome(t, err, CRLOutcomeFetch)
	if !strings.Contains(err.Error(), "refused a redirect to an address that is not https") {
		t.Fatalf("Refresh: %v", err)
	}
}

// A discovery document answered with another status than 200, or with 304 to
// a request that carried no If-None-Match, is refused as status, one that is
// not a JSON text as parse, and a JSON text that is not of the document's form
// as shape. The list is not read.
func TestCRLClient_ClassifiesADiscoveryDocumentItCannotRead(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		outcome string
	}{
		{"an answer of 500", http.StatusInternalServerError, "", CRLOutcomeStatus},
		{"an answer of 503", http.StatusServiceUnavailable, "", CRLOutcomeStatus},
		{"an answer of 304 to a request without If-None-Match", http.StatusNotModified, "", CRLOutcomeStatus},
		{"a body that is not JSON", http.StatusOK, `<html><body>Service Unavailable</body></html>`, CRLOutcomeParse},
		{"a document cut short", http.StatusOK, `{"endpoints":{"revocations":`, CRLOutcomeParse},
		{"an array where the document belongs", http.StatusOK, `[]`, CRLOutcomeShape},
		{"an endpoints member that is a string", http.StatusOK, `{"endpoints":"` + testListPath + `"}`, CRLOutcomeShape},
		{"a document longer than its bound", http.StatusOK, `{"endpoints":{"revocations":"` + testListPath + `"}}` + strings.Repeat(" ", maxDiscoveryBytes), CRLOutcomeShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			var mu sync.Mutex
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			c, err := NewCachedCRLClient(srv.URL, "", srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			requireRefreshOutcome(t, c.Refresh(context.Background()), tc.outcome)
			mu.Lock()
			defer mu.Unlock()
			if len(paths) != 1 || paths[0] != "/.well-known/atp" {
				t.Fatalf("the authority was asked for %v, want /.well-known/atp alone", paths)
			}
		})
	}
}

// A list address answered with another status than 200, or with 304 to a
// request that carried no If-None-Match, is refused as status.
func TestCRLClient_RefusesAListAnsweredWithAnotherStatusThan200(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusNotModified} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			signer := newEd25519Signer(t, testListKeyID)
			a := newCRLAuthority(t, discoveryFor(signer))
			now := time.Now()
			a.setList(signList(t, testList(now, revokedDID), signer))
			a.set(func(a *crlAuthority) { a.listStatus = status })
			c := newTestCRLClient(t, a, "", &now)

			requireRefreshOutcome(t, c.Refresh(context.Background()), CRLOutcomeStatus)
			requireUnknown(t, c, revokedDID)
			if _, ifNoneMatch := a.requests(); len(ifNoneMatch) != 1 || ifNoneMatch[0] != "" {
				t.Fatalf("If-None-Match of the list requests: %q, want one request without it", ifNoneMatch)
			}
		})
	}
}

// A read that has not ended by its deadline is refused as timeout: the
// deadline of the refresh's context and the timeout of the HTTP client, while
// the authority sends no answer, and while it sends a list body that stops.
func TestCRLClient_RefusesAReadThatOutlivesItsDeadlineAsTimeout(t *testing.T) {
	discovery, err := json.Marshal(map[string]any{"endpoints": map[string]any{"revocations": testListPath}, "publicKeys": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	silent := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(silent.Close)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/atp", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(discovery)
	})
	mux.HandleFunc(testListPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entryCount":0,`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	stopping := httptest.NewTLSServer(mux)
	t.Cleanup(stopping.Close)

	withTimeout := func(srv *httptest.Server) *http.Client {
		client := *srv.Client()
		client.Timeout = 200 * time.Millisecond
		return &client
	}
	cases := []struct {
		name     string
		srv      *httptest.Server
		client   *http.Client
		deadline time.Duration
	}{
		{"no answer by the refresh's deadline", silent, silent.Client(), 200 * time.Millisecond},
		{"no answer within the HTTP client's timeout", silent, withTimeout(silent), 0},
		{"a list body that stops before the refresh's deadline", stopping, stopping.Client(), 200 * time.Millisecond},
		{"a list body that stops within the HTTP client's timeout", stopping, withTimeout(stopping), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewCachedCRLClient(tc.srv.URL, "", tc.client)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if tc.deadline > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.deadline)
				defer cancel()
			}
			requireRefreshOutcome(t, c.Refresh(ctx), CRLOutcomeTimeout)
		})
	}
}

// Both addresses must be https URLs with a host, and the client needs the base.
func TestNewCachedCRLClient_RefusesAnAddressThatIsNotHTTPS(t *testing.T) {
	cases := []struct {
		name, base, override string
	}{
		{"no base URL", "", ""},
		{"an http base URL", "http://registry.example", ""},
		{"a base URL without a host", "https://", ""},
		{"a base URL that does not parse", "https://registry.example/%zz", ""},
		{"an http endpoint override", "https://registry.example", "http://registry.example" + testListPath},
		{"an endpoint override without a scheme", "https://registry.example", "registry.example" + testListPath},
	}
	for _, tc := range cases {
		if _, err := NewCachedCRLClient(tc.base, tc.override, nil); err == nil {
			t.Errorf("%s: NewCachedCRLClient(%q, %q) built a client", tc.name, tc.base, tc.override)
		}
	}
	if _, err := NewCachedCRLClient("https://registry.example", "https://mirror.example"+testListPath, nil); err != nil {
		t.Fatalf("https base and override: %v", err)
	}
}
