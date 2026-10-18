package atc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// Bounds on a revocation list. A body longer than maxCRLBodyBytes is refused
// without being read whole. Within the body, the members other than
// revocations may take maxCRLEnvelopeBytes, and revocations may hold
// maxCRLEntries entries of maxCRLEntryBytes each, the last three counted in
// the canonical (RFC 8785) form.
const (
	maxCRLEntryBytes    = 1258
	maxCRLEntries       = 10_000
	maxCRLEnvelopeBytes = 65_536
	maxCRLBodyBytes     = maxCRLEnvelopeBytes + maxCRLEntries*maxCRLEntryBytes

	// maxCRLAgentDIDBytes bounds an entry's agentDid.
	maxCRLAgentDIDBytes = 512
	// maxCRLRevokedByKeyIDBytes bounds an entry's revokedByKeyId.
	maxCRLRevokedByKeyIDBytes = 576

	// maxDiscoveryBytes bounds the discovery document.
	maxDiscoveryBytes = 65_536
)

const (
	// crlMaxStaleAge is how long after its last successful read a list is
	// answered from.
	crlMaxStaleAge = 5 * time.Minute

	// crlClockSkew is how far a list's thisUpdate may be ahead of this
	// server's clock, and how long after its nextUpdate a list is still held
	// and answered from.
	crlClockSkew = 60 * time.Second

	// crlHTTPTimeout bounds one request for the discovery document or the
	// list when the client is built without an HTTP client of its own.
	crlHTTPTimeout = 10 * time.Second
)

// discoveryPaths are where an authority serves its discovery document: the
// ATP path, then its legacy alias, which is read when the first answers 404.
var discoveryPaths = []string{"/.well-known/atp", "/.well-known/opena2a"}

// The signature algorithms a list may be signed with.
const (
	crlAlgorithmEd25519 = string(pqc.AlgorithmEd25519)
	crlAlgorithmMLDSA65 = string(pqc.AlgorithmMLDSA65)
)

// Outcomes of a refresh that holds no new list. Each but CRLOutcomeFetch is
// also a reason word of CRLRefresher.State.
const (
	// CRLOutcomeStatus: the discovery document or the list was answered with
	// an HTTP status other than 200, or with 304 to a request that carried no
	// If-None-Match.
	CRLOutcomeStatus = "status"
	// CRLOutcomeTimeout: a read of the discovery document or the list had not
	// ended by its deadline.
	CRLOutcomeTimeout = "timeout"
	// CRLOutcomeFetch: a read of the discovery document or the list ended
	// before its deadline with no answer: the connection, TLS, or a redirect
	// the client does not follow.
	CRLOutcomeFetch = "fetch"
	// CRLOutcomeParse: a document is not an I-JSON text (RFC 7493).
	CRLOutcomeParse = "parse"
	// CRLOutcomeShape: a document is longer than its bound or breaks the
	// form of the list contract.
	CRLOutcomeShape = "shape"
	// CRLOutcomeSignature: the list's signatures do not verify under the keys
	// the discovery document lists.
	CRLOutcomeSignature = "signature"
	// CRLOutcomeRollback: the list's thisUpdate is before that of the list
	// held.
	CRLOutcomeRollback = "rollback"
	// CRLOutcomeExpired: the list's thisUpdate is more than the clock skew
	// bound ahead of this server's clock, or its nextUpdate passed more than
	// the clock skew bound ago.
	CRLOutcomeExpired = "expired"
)

// CRLRefreshError is the error of a refresh that holds no new list. Outcome
// names the check that refused it.
type CRLRefreshError struct {
	Outcome string
	Err     error
}

func (e *CRLRefreshError) Error() string {
	return "revocation list " + e.Outcome + ": " + e.Err.Error()
}

func (e *CRLRefreshError) Unwrap() error { return e.Err }

func refreshFailure(outcome string, err error) error {
	return &CRLRefreshError{Outcome: outcome, Err: err}
}

// readOutcome is the outcome of a read that ended with err before its answer
// was read whole: timeout when its deadline passed, fetch otherwise.
func readOutcome(err error) string {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return CRLOutcomeTimeout
	}
	return CRLOutcomeFetch
}

// documentOutcome is the outcome of a document refused with err: parse when
// it is not an I-JSON text, shape otherwise.
func documentOutcome(err error) string {
	if isNotIJSON(err) {
		return CRLOutcomeParse
	}
	return CRLOutcomeShape
}

// ErrCRLStateUnknown is wrapped by the error IsRevoked returns when it holds
// no list fit to answer from.
var ErrCRLStateUnknown = errors.New("the revocation list state is unknown")

// CachedCRLClient holds the signed revocation list of one authority, keyed by
// agentDid. Refresh reads the authority's discovery document at the base URL
// the client is built with, at /.well-known/atp or its alias
// /.well-known/opena2a, and then the list at the document's
// endpoints.revocations, or at the endpoint override when one is set. A
// request for the discovery document carries in If-None-Match the ETag of the
// last document the client used when that document was read at the same
// address, and a request for the list carries the ETag of the list held when
// that list was read at the same address under the keys the document lists
// now. A list is held when it passes these checks: the form and bounds
// parseCRL reads, a verified Ed25519 signature by a key the discovery
// document lists in publicKeys and no failed signature by a listed key, an
// entryCount equal to the entries it holds, a thisUpdate and nextUpdate
// within the clock skew bound, and a thisUpdate not before that of the list
// held. IsRevoked reads only the list held. Implements atcdomain.CRLClient.
type CachedCRLClient struct {
	base     *url.URL
	override *url.URL
	client   *http.Client
	now      func() time.Time

	refreshMu sync.Mutex
	held      atomic.Pointer[crlList]

	// discovered is the last discovery document the client read and used.
	// Guarded by refreshMu.
	discovered *crlDiscovery
}

var _ atcdomain.CRLClient = (*CachedCRLClient)(nil)

// crlList is a list that passed the client's checks, and when it was last
// read.
type crlList struct {
	revoked    map[string]struct{}
	thisUpdate time.Time
	nextUpdate time.Time
	readAt     time.Time
	endpoint   string
	etag       string
	keys       map[string]crlKey // the keys the list was checked under
}

// crlDiscovery is a discovery document the client read and used: the address
// that answered with it, the ETag of that answer, and the list address and
// keys read from it.
type crlDiscovery struct {
	at       string
	etag     string
	endpoint *url.URL
	keys     map[string]crlKey
}

// etagFor returns the ETag of d when d was read at at, and "" otherwise.
func (d *crlDiscovery) etagFor(at *url.URL) string {
	if d == nil || d.at != at.String() {
		return ""
	}
	return d.etag
}

// NewCachedCRLClient returns a client of the authority whose discovery
// document is served at registryURL. endpointOverride, when it is not empty,
// replaces the list address the discovery document names; the keys are read
// from the discovery document either way. Both must be https URLs with a
// host. A nil httpClient is replaced by one with a 10 second timeout. The
// client follows no redirect to an address that is not https.
func NewCachedCRLClient(registryURL, endpointOverride string, httpClient *http.Client) (*CachedCRLClient, error) {
	if registryURL == "" {
		return nil, errors.New("the revocation list needs the base URL of the authority's discovery document")
	}
	base, err := parseHTTPSURL(registryURL)
	if err != nil {
		return nil, fmt.Errorf("the discovery base URL: %w", err)
	}
	var override *url.URL
	if endpointOverride != "" {
		if override, err = parseHTTPSURL(endpointOverride); err != nil {
			return nil, fmt.Errorf("the revocation list endpoint override: %w", err)
		}
	}
	client := &http.Client{Timeout: crlHTTPTimeout}
	if httpClient != nil {
		copied := *httpClient
		client = &copied
	}
	client.CheckRedirect = refuseRedirectAwayFromHTTPS
	return &CachedCRLClient{base: base, override: override, client: client, now: time.Now}, nil
}

// parseHTTPSURL parses raw as an https URL with a host. Its errors do not
// quote raw, which may carry credentials.
func parseHTTPSURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("not a URL")
	}
	if u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("not an https URL with a host")
	}
	return u, nil
}

// refuseRedirectAwayFromHTTPS follows at most 10 redirects, each to an https
// address.
func refuseRedirectAwayFromHTTPS(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return errors.New("refused a redirect to an address that is not https")
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// IsRevoked reports whether the list held names agentDID, and reads no
// address. When no list is held, when the list held was last read more than
// crlMaxStaleAge ago, or when its nextUpdate passed more than crlClockSkew
// ago, it returns false and an error that wraps ErrCRLStateUnknown, which is
// never an answer of not revoked.
func (c *CachedCRLClient) IsRevoked(agentDID string) (bool, error) {
	list := c.held.Load()
	if err := answerable(list, c.now()); err != nil {
		return false, err
	}
	_, revoked := list.revoked[agentDID]
	return revoked, nil
}

// answerable returns nil when list is fit to answer from at now. Otherwise it
// returns an error that wraps ErrCRLStateUnknown: when list is nil, when it
// was last read more than crlMaxStaleAge before now, or when its nextUpdate
// passed more than crlClockSkew before now.
func answerable(list *crlList, now time.Time) error {
	if list == nil {
		return fmt.Errorf("%w: no list has passed its checks", ErrCRLStateUnknown)
	}
	if now.Sub(list.readAt) > crlMaxStaleAge {
		return fmt.Errorf("%w: the list held was last read at %s", ErrCRLStateUnknown, list.readAt.UTC().Format(time.RFC3339))
	}
	if now.After(list.nextUpdate.Add(crlClockSkew)) {
		return fmt.Errorf("%w: the nextUpdate of the list held, %s, has passed", ErrCRLStateUnknown, list.nextUpdate.UTC().Format(time.RFC3339))
	}
	return nil
}

// Refresh reads the discovery document and the list, and holds the list when
// it passes the checks CachedCRLClient names. When the list held was read
// from the same address with an ETag, under the keys the discovery document
// lists now, the request carries that ETag in If-None-Match, and a 304 answer
// counts as a read of the list held. When the keys have changed, the list is
// read in full and checked under the keys listed now. An error is a
// *CRLRefreshError, and the list held stays.
func (c *CachedCRLClient) Refresh(ctx context.Context) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	endpoint, keys, err := c.discover(ctx)
	if err != nil {
		return err
	}
	held := c.held.Load()
	ifNoneMatch := ""
	if held != nil && held.endpoint == endpoint.String() && sameCRLKeys(held.keys, keys) {
		ifNoneMatch = held.etag
	}
	answer, err := c.get(ctx, endpoint, maxCRLBodyBytes, ifNoneMatch)
	if err != nil {
		return refreshFailure(readOutcome(err), fmt.Errorf("reading the list at %s: %w", endpoint.Redacted(), err))
	}
	now := c.now()
	switch {
	case answer.status == http.StatusNotModified && ifNoneMatch != "":
		read := *held
		read.readAt = now
		c.held.Store(&read)
		return nil
	case answer.status != http.StatusOK:
		return refreshFailure(CRLOutcomeStatus, fmt.Errorf("the list at %s answered %d", endpoint.Redacted(), answer.status))
	case answer.tooLong:
		return refreshFailure(CRLOutcomeShape, fmt.Errorf("the list at %s is longer than %d bytes", endpoint.Redacted(), maxCRLBodyBytes))
	}
	list, err := checkCRL(answer.body, keys, now)
	if err != nil {
		return err
	}
	if held != nil && list.thisUpdate.Before(held.thisUpdate) {
		return refreshFailure(CRLOutcomeRollback, fmt.Errorf("thisUpdate %s is before %s, the thisUpdate of the list held",
			list.thisUpdate.UTC().Format(time.RFC3339), held.thisUpdate.UTC().Format(time.RFC3339)))
	}
	list.readAt = now
	list.endpoint = endpoint.String()
	list.etag = answer.etag
	list.keys = keys
	c.held.Store(list)
	return nil
}

// sameCRLKeys reports whether a and b list the same keyIds, each with the same
// algorithm and public key.
func sameCRLKeys(a, b map[string]crlKey) bool {
	if len(a) != len(b) {
		return false
	}
	for keyID, key := range a {
		other, listed := b[keyID]
		if !listed || other.algorithm != key.algorithm || !bytes.Equal(other.public, key.public) {
			return false
		}
	}
	return true
}

// discoveryDocument is the part of an authority's discovery document the list
// client reads.
type discoveryDocument struct {
	Endpoints struct {
		Revocations string `json:"revocations"`
	} `json:"endpoints"`
	PublicKeys []struct {
		KeyID        string `json:"keyId"`
		Algorithm    string `json:"algorithm"`
		PublicKeyHex string `json:"publicKeyHex"`
	} `json:"publicKeys"`
}

// crlKey is a key the discovery document lists.
type crlKey struct {
	algorithm string
	public    []byte
}

// discover reads the discovery document and returns the list address and the
// listed keys by keyId. A request to the address that answered with the last
// document the client used carries that answer's ETag in If-None-Match, and a
// 304 answer returns the list address and keys read from that document.
func (c *CachedCRLClient) discover(ctx context.Context) (*url.URL, map[string]crlKey, error) {
	var at *url.URL
	var answer *httpAnswer
	ifNoneMatch := ""
	for _, path := range discoveryPaths {
		at = c.base.ResolveReference(&url.URL{Path: path})
		ifNoneMatch = c.discovered.etagFor(at)
		var err error
		if answer, err = c.get(ctx, at, maxDiscoveryBytes, ifNoneMatch); err != nil {
			return nil, nil, refreshFailure(readOutcome(err), fmt.Errorf("reading the discovery document at %s: %w", at.Redacted(), err))
		}
		if answer.status != http.StatusNotFound {
			break
		}
	}
	if answer.status == http.StatusNotModified && ifNoneMatch != "" {
		return c.discovered.endpoint, c.discovered.keys, nil
	}
	if answer.status != http.StatusOK {
		return nil, nil, refreshFailure(CRLOutcomeStatus, fmt.Errorf("the discovery document at %s answered %d", at.Redacted(), answer.status))
	}
	if answer.tooLong {
		return nil, nil, refreshFailure(CRLOutcomeShape, fmt.Errorf("the discovery document at %s is longer than %d bytes", at.Redacted(), maxDiscoveryBytes))
	}
	endpoint, keys, err := c.readDiscovery(at, answer.body)
	if err != nil {
		return nil, nil, refreshFailure(documentOutcome(err), fmt.Errorf("the discovery document at %s: %w", at.Redacted(), err))
	}
	c.discovered = &crlDiscovery{at: at.String(), etag: answer.etag, endpoint: endpoint, keys: keys}
	return endpoint, keys, nil
}

// readDiscovery reads the list address and the keys from a discovery document
// read at at. A body that is not a JSON text refuses the document with a
// *notIJSONError. A keyId that publicKeys lists twice refuses the document. A
// key is used when it has a keyId, an algorithm of Ed25519 or ML-DSA-65, and
// publicKeyHex, the hex of a raw public key of that algorithm's size; a key
// written otherwise is not used.
func (c *CachedCRLClient) readDiscovery(at *url.URL, body []byte) (*url.URL, map[string]crlKey, error) {
	var doc discoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return nil, nil, errors.New("not a JSON object of the discovery document's form")
		}
		return nil, nil, notIJSON(errors.New("not a JSON text"))
	}
	keys := make(map[string]crlKey, len(doc.PublicKeys))
	listed := map[string]bool{}
	for _, k := range doc.PublicKeys {
		if k.KeyID == "" {
			continue
		}
		if listed[k.KeyID] {
			return nil, nil, fmt.Errorf("publicKeys lists keyId %q twice", k.KeyID)
		}
		listed[k.KeyID] = true
		public, err := hex.DecodeString(k.PublicKeyHex)
		if err != nil {
			continue
		}
		if (k.Algorithm == crlAlgorithmEd25519 && len(public) == ed25519.PublicKeySize) ||
			(k.Algorithm == crlAlgorithmMLDSA65 && len(public) == pqc.MLDSA65PublicKeySize) {
			keys[k.KeyID] = crlKey{algorithm: k.Algorithm, public: public}
		}
	}
	endpoint := c.override
	if endpoint == nil {
		if doc.Endpoints.Revocations == "" {
			return nil, nil, errors.New("endpoints.revocations is not set")
		}
		ref, err := url.Parse(doc.Endpoints.Revocations)
		if err != nil {
			return nil, nil, errors.New("endpoints.revocations is not a URL")
		}
		if endpoint = at.ResolveReference(ref); endpoint.Scheme != "https" || endpoint.Host == "" {
			return nil, nil, errors.New("endpoints.revocations is not an https address")
		}
	}
	return endpoint, keys, nil
}

// httpAnswer is the status and ETag of an answer, and the body of a 200.
type httpAnswer struct {
	status  int
	etag    string
	body    []byte
	tooLong bool // the body is longer than the bound it was read with
}

// get reads at, with If-None-Match when ifNoneMatch is not empty. It reads the
// body of a 200 answer through io.LimitReader at limit+1 bytes, so a body
// longer than limit is known without being read whole.
func (c *CachedCRLClient) get(ctx context.Context, at *url.URL, limit int64, ifNoneMatch string) (*httpAnswer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, at.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	answer := &httpAnswer{status: resp.StatusCode, etag: resp.Header.Get("ETag")}
	if resp.StatusCode != http.StatusOK {
		return answer, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if answer.tooLong = int64(len(body)) > limit; !answer.tooLong {
		answer.body = body
	}
	return answer, nil
}

// checkCRL checks a list body: its form, then its signatures under keys, then
// its times against now.
func checkCRL(body []byte, keys map[string]crlKey, now time.Time) (*crlList, error) {
	doc, err := parseCRL(body)
	if err != nil {
		return nil, refreshFailure(documentOutcome(err), err)
	}
	if err := verifyCRLSignatures(doc.signed, doc.signatures, keys); err != nil {
		return nil, refreshFailure(CRLOutcomeSignature, err)
	}
	if doc.thisUpdate.After(now.Add(crlClockSkew)) {
		return nil, refreshFailure(CRLOutcomeExpired, fmt.Errorf("thisUpdate %s is more than %s after this server's clock",
			doc.thisUpdate.UTC().Format(time.RFC3339), crlClockSkew))
	}
	if !doc.nextUpdate.After(doc.thisUpdate) {
		return nil, refreshFailure(CRLOutcomeShape, errors.New("nextUpdate is not after thisUpdate"))
	}
	if now.After(doc.nextUpdate.Add(crlClockSkew)) {
		return nil, refreshFailure(CRLOutcomeExpired, fmt.Errorf("nextUpdate %s passed more than %s ago",
			doc.nextUpdate.UTC().Format(time.RFC3339), crlClockSkew))
	}
	return &crlList{revoked: doc.revoked, thisUpdate: doc.thisUpdate, nextUpdate: doc.nextUpdate}, nil
}

// crlDocument is a list body read in the form of the list contract.
type crlDocument struct {
	signed     []byte // the canonical form of the list without its signatures member
	signatures []crlSignature
	revoked    map[string]struct{}
	thisUpdate time.Time
	nextUpdate time.Time
}

// crlSignature is one member of a list's signatures.
type crlSignature struct {
	keyID     string
	algorithm string
	value     string
}

// parseCRL reads a list body: a JSON object that names no member twice, whose
// revocations holds entries crlEntryDID accepts within the entry bounds,
// whose other members take at most maxCRLEnvelopeBytes, whose entryCount
// equals the number of entries, whose thisUpdate and nextUpdate are RFC 3339
// times, and whose signatures, when it is present, is an array of objects with
// string keyId, algorithm and value. Its error for a body that is not an
// I-JSON text is a *notIJSONError.
func parseCRL(body []byte) (*crlDocument, error) {
	r, err := newJSONReader(body)
	if err != nil {
		return nil, err
	}
	tok, err := r.token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, errors.New("the list is not a JSON object")
	}
	members := map[string]any{}
	canonical := map[string][]byte{} // "name":value
	var entries []any
	envelope := 0
	for r.dec.More() {
		tok, err := r.token()
		if err != nil {
			return nil, err
		}
		name, _ := tok.(string)
		if _, named := canonical[name]; named {
			return nil, notIJSON(fmt.Errorf("the list names member %q twice", name))
		}
		key := append(appendCanonicalString(nil, name), ':')
		if name == "revocations" {
			var array []byte
			if array, entries, err = r.crlEntries(); err != nil {
				return nil, err
			}
			canonical[name] = append(key, array...)
			continue
		}
		member, value, err := r.next(key, 1, maxCRLEnvelopeBytes-envelope)
		if errors.Is(err, errCanonicalLimit) {
			return nil, fmt.Errorf("the members other than revocations are longer than %d bytes", maxCRLEnvelopeBytes)
		}
		if err != nil {
			return nil, err
		}
		envelope += len(member)
		canonical[name] = member
		members[name] = value
	}
	if _, err := r.token(); err != nil {
		return nil, err
	}
	if err := r.end(); err != nil {
		return nil, err
	}

	if _, ok := canonical["revocations"]; !ok {
		return nil, errors.New("the list has no revocations member")
	}
	doc := &crlDocument{revoked: make(map[string]struct{}, len(entries))}
	count, err := crlInteger(members["entryCount"])
	if err != nil {
		return nil, fmt.Errorf("entryCount %w", err)
	}
	if count != int64(len(entries)) {
		return nil, fmt.Errorf("entryCount is %d and revocations holds %d entries", count, len(entries))
	}
	if doc.thisUpdate, err = crlTime(members["thisUpdate"]); err != nil {
		return nil, fmt.Errorf("thisUpdate %w", err)
	}
	if doc.nextUpdate, err = crlTime(members["nextUpdate"]); err != nil {
		return nil, fmt.Errorf("nextUpdate %w", err)
	}
	if doc.signatures, err = crlSignatures(members["signatures"]); err != nil {
		return nil, err
	}
	for i, entry := range entries {
		did, err := crlEntryDID(entry)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		doc.revoked[did] = struct{}{}
	}

	// The signed bytes are the canonical form of the list without its
	// signatures member.
	names := make([]string, 0, len(canonical))
	for name := range canonical {
		if name != "signatures" {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return lessUTF16(names[i], names[j]) })
	doc.signed = append(doc.signed, '{')
	for i, name := range names {
		if i > 0 {
			doc.signed = append(doc.signed, ',')
		}
		doc.signed = append(doc.signed, canonical[name]...)
	}
	doc.signed = append(doc.signed, '}')
	return doc, nil
}

// crlEntries reads the revocations array: at most maxCRLEntries entries of at
// most maxCRLEntryBytes each in canonical form. It returns the array's
// canonical form and its entries.
func (r *jsonReader) crlEntries() ([]byte, []any, error) {
	tok, err := r.token()
	if err != nil {
		return nil, nil, err
	}
	if tok != json.Delim('[') {
		return nil, nil, errors.New("revocations is not an array")
	}
	out := []byte{'['}
	entries := []any{}
	for r.dec.More() {
		if len(entries) == maxCRLEntries {
			return nil, nil, fmt.Errorf("revocations holds more than %d entries", maxCRLEntries)
		}
		entry, value, err := r.next(nil, 2, maxCRLEntryBytes)
		if errors.Is(err, errCanonicalLimit) {
			return nil, nil, fmt.Errorf("entry %d is longer than %d bytes", len(entries), maxCRLEntryBytes)
		}
		if err != nil {
			return nil, nil, err
		}
		if len(entries) > 0 {
			out = append(out, ',')
		}
		out = append(out, entry...)
		entries = append(entries, value)
	}
	if _, err := r.token(); err != nil {
		return nil, nil, err
	}
	return append(out, ']'), entries, nil
}

// crlEntryDID checks an entry and returns its agentDid. agentDid is a string
// of 1 to maxCRLAgentDIDBytes bytes and revokedAt an RFC 3339 time in UTC
// written with Z; when they are present, reason is a string, revokedByKeyId a
// string of at most maxCRLRevokedByKeyIDBytes bytes, and transparencyLogIndex
// an integer from 0 to 2^63-1.
func crlEntryDID(v any) (string, error) {
	entry, ok := v.(map[string]any)
	if !ok {
		return "", errors.New("is not an object")
	}
	did, ok := entry["agentDid"].(string)
	if !ok || did == "" || len(did) > maxCRLAgentDIDBytes {
		return "", fmt.Errorf("agentDid is not a string of 1 to %d bytes", maxCRLAgentDIDBytes)
	}
	revokedAt, ok := entry["revokedAt"].(string)
	if !ok || !strings.HasSuffix(revokedAt, "Z") {
		return "", errors.New("revokedAt is not an RFC 3339 time in UTC written with Z")
	}
	if _, err := time.Parse(time.RFC3339, revokedAt); err != nil {
		return "", errors.New("revokedAt is not an RFC 3339 time in UTC written with Z")
	}
	if reason, present := entry["reason"]; present {
		if _, ok := reason.(string); !ok {
			return "", errors.New("reason is not a string")
		}
	}
	if keyID, present := entry["revokedByKeyId"]; present {
		if s, ok := keyID.(string); !ok || len(s) > maxCRLRevokedByKeyIDBytes {
			return "", fmt.Errorf("revokedByKeyId is not a string of at most %d bytes", maxCRLRevokedByKeyIDBytes)
		}
	}
	if index, present := entry["transparencyLogIndex"]; present {
		if _, err := crlInteger(index); err != nil {
			return "", fmt.Errorf("transparencyLogIndex %w", err)
		}
	}
	return did, nil
}

// crlInteger reads an integer from 0 to 2^63-1, written without a fraction or
// an exponent.
func crlInteger(v any) (int64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, errors.New("is not an integer from 0 to 2^63-1")
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || i < 0 {
		return 0, errors.New("is not an integer from 0 to 2^63-1")
	}
	return i, nil
}

// crlTime reads an RFC 3339 time.
func crlTime(v any) (time.Time, error) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, errors.New("is not an RFC 3339 time")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errors.New("is not an RFC 3339 time")
	}
	return t, nil
}

// crlSignatures reads the signatures member. An absent member is no
// signature, which verifyCRLSignatures refuses.
func crlSignatures(v any) ([]crlSignature, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, errors.New("signatures is not an array")
	}
	signatures := make([]crlSignature, 0, len(items))
	for i, item := range items {
		m, isObject := item.(map[string]any)
		keyID, okKeyID := m["keyId"].(string)
		algorithm, okAlgorithm := m["algorithm"].(string)
		value, okValue := m["value"].(string)
		if !isObject || !okKeyID || !okAlgorithm || !okValue {
			return nil, fmt.Errorf("signature %d is not an object with string keyId, algorithm and value", i)
		}
		signatures = append(signatures, crlSignature{keyID: keyID, algorithm: algorithm, value: value})
	}
	return signatures, nil
}

// verifyCRLSignatures verifies a list's signatures over signed. A signature
// whose keyId the discovery document does not list is not read. A signature
// whose keyId it lists refuses the list when it names another algorithm than
// the key's, is not base64, or does not verify; and at least one Ed25519
// signature by a listed key must verify.
func verifyCRLSignatures(signed []byte, signatures []crlSignature, keys map[string]crlKey) error {
	if len(signatures) == 0 {
		return errors.New("the list carries no signature")
	}
	ed25519Verified := false
	for _, s := range signatures {
		key, listed := keys[s.keyID]
		if !listed {
			continue
		}
		if s.algorithm != key.algorithm {
			return fmt.Errorf("the signature by %q names algorithm %q and the key is %s", s.keyID, s.algorithm, key.algorithm)
		}
		value, err := decodeSignatureValue(s.value)
		if err != nil {
			return fmt.Errorf("the signature by %q is not base64", s.keyID)
		}
		switch key.algorithm {
		case crlAlgorithmEd25519:
			if len(value) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(key.public), signed, value) {
				return fmt.Errorf("the Ed25519 signature by %q does not verify", s.keyID)
			}
			ed25519Verified = true
		case crlAlgorithmMLDSA65:
			if err := pqc.VerifyMLDSA(pqc.AlgorithmMLDSA65, key.public, signed, value); err != nil {
				return fmt.Errorf("the ML-DSA-65 signature by %q does not verify", s.keyID)
			}
		}
	}
	if !ed25519Verified {
		return errors.New("no Ed25519 signature by a key the discovery document lists verifies")
	}
	return nil
}

// decodeSignatureValue decodes a signature value written in standard base64,
// with or without padding.
func decodeSignatureValue(s string) ([]byte, error) {
	if value, err := base64.StdEncoding.DecodeString(s); err == nil {
		return value, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
