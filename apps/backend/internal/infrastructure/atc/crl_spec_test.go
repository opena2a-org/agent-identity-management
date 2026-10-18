package atc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
)

// Section 5 of docs/specs/atc-verification-v1.md gave the revocation list's
// address as GET /api/v1/crl/latest, a path the pinned Registry route table
// does not carry, and its form as revokedATCs entries keyed by atcId, a form
// this client does not read (#778). The tests in this file hold section 5 to
// the client: each JSON example in it passes the client's own checks, the
// bounds and times it states are the client's constants, each Registry path
// it names is a GET route of the pinned table, and the conditions it states
// for the list request's If-None-Match are the ones the client sends it under.

var (
	// atcSpecPath is the specification, from this package's directory.
	atcSpecPath = filepath.Join("..", "..", "..", "..", "..", "docs", "specs", "atc-verification-v1.md")

	// atcSpecRoutesPath is the pinned Registry route table that
	// scripts/pin-registry-routes.py writes, from this package's directory.
	atcSpecRoutesPath = filepath.Join("..", "..", "..", "testdata", "registry-routes.txt")

	// atcSpecJSONBlock matches a fenced JSON block and captures its body.
	atcSpecJSONBlock = regexp.MustCompile("(?s)```json\n(.*?)```")

	// atcSpecRegistryPath captures a path under /api/, /internal/ or
	// /.well-known/ that starts the text, follows white space, a quote, a
	// backtick or a parenthesis, or follows the host of an http or https URL.
	// A repository file path such as apps/backend/internal/... is not one.
	atcSpecRegistryPath = regexp.MustCompile("(?:^|[\\s\"'`(]|https?://[A-Za-z0-9.-]+(?::[0-9]+)?)(/(?:api|internal|\\.well-known)/[A-Za-z0-9._~%:/-]*)")
)

// atcSpecSection5 returns section 5 of the specification: its heading and the
// lines up to the next second-level heading.
func atcSpecSection5(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(atcSpecPath)
	if err != nil {
		t.Fatalf("read the ATC verification specification: %v", err)
	}
	text := string(raw)
	start := strings.Index(text, "\n## 5. ")
	if start < 0 {
		t.Fatal("the ATC verification specification has no section 5")
	}
	section := text[start+1:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

// atcSpecSubsection returns the subsection of section 5 numbered number: its
// heading and the lines up to the next third-level heading.
func atcSpecSubsection(t *testing.T, section, number string) string {
	t.Helper()
	start := strings.Index(section, "\n### "+number+" ")
	if start < 0 {
		t.Fatalf("section 5 has no subsection %s", number)
	}
	part := section[start+1:]
	if end := strings.Index(part, "\n### "); end >= 0 {
		part = part[:end]
	}
	return part
}

// atcSpecWords returns text with each run of white space replaced by one
// space, so a phrase wrapped over two lines still matches.
func atcSpecWords(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// atcSpecExamples returns the two JSON examples of section 5: the discovery
// document, which has an endpoints member, and the list, which has a
// revocations member. Any other JSON example fails the test, so no example in
// section 5 goes unchecked.
func atcSpecExamples(t *testing.T, section string) (discovery, list []byte) {
	t.Helper()
	for _, block := range atcSpecJSONBlock.FindAllStringSubmatch(section, -1) {
		body := []byte(block[1])
		var members map[string]json.RawMessage
		if err := json.Unmarshal(body, &members); err != nil {
			t.Fatalf("a JSON example of section 5 is not a JSON object: %v\n%s", err, body)
		}
		_, isDiscovery := members["endpoints"]
		_, isList := members["revocations"]
		switch {
		case isDiscovery && !isList && discovery == nil:
			discovery = body
		case isList && !isDiscovery && list == nil:
			list = body
		default:
			t.Fatalf("section 5 holds a JSON example that is neither its one discovery document (with an endpoints member) nor its one list (with a revocations member):\n%s", body)
		}
	}
	if discovery == nil || list == nil {
		t.Fatalf("section 5 needs a discovery document example and a list example; discovery document found: %t, list found: %t",
			discovery != nil, list != nil)
	}
	return discovery, list
}

// atcSpecPinnedRoutes returns the routes of the pinned Registry route table,
// each as "METHOD path".
func atcSpecPinnedRoutes(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(atcSpecRoutesPath)
	if err != nil {
		t.Fatalf("read the pinned Registry route table: %v", err)
	}
	routes := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Fields(line); len(fields) == 2 {
			routes[fields[0]+" "+fields[1]] = true
		}
	}
	if len(routes) == 0 {
		t.Fatalf("the pinned Registry route table %s holds no route", atcSpecRoutesPath)
	}
	return routes
}

// atcSpecCount writes n as section 5 does, with a comma before each group of
// three digits.
func atcSpecCount(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// atcSpecDuration writes d as section 5 does: in seconds below two minutes,
// and in minutes from two minutes up.
func atcSpecDuration(d time.Duration) string {
	n, unit := int64(d/time.Second), "second"
	if d >= 2*time.Minute {
		n, unit = int64(d/time.Minute), "minute"
	}
	if n != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", n, unit)
}

// The discovery document example names a list address the client accepts on
// a GET route of the pinned Registry table, the list example is of the form
// the client reads, and, once signed by the key the discovery document example
// lists, the list example is a list the client holds.
func TestATCSpecSection5_ExamplesPassTheListClientsChecks(t *testing.T) {
	requirePublicRepositoryFiles(t)
	discovery, list := atcSpecExamples(t, atcSpecSection5(t))
	at := &url.URL{Scheme: "https", Host: "registry.example.com", Path: discoveryPaths[0]}

	endpoint, _, err := (&CachedCRLClient{}).readDiscovery(at, discovery)
	if err != nil {
		t.Fatalf("the client refuses the discovery document example: %v", err)
	}
	if !atcSpecPinnedRoutes(t)["GET "+endpoint.Path] {
		t.Errorf("the discovery document example names the list at %s, which is not a GET route of the pinned Registry route table", endpoint.Path)
	}

	parsed, err := parseCRL(list)
	if err != nil {
		t.Fatalf("the client refuses the list example: %v", err)
	}
	if len(parsed.revoked) == 0 {
		t.Fatal("the list example revokes no agent")
	}

	dec := json.NewDecoder(bytes.NewReader(list))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	signatures, _ := doc["signatures"].([]any)
	if len(signatures) == 0 {
		t.Fatal("the list example carries no signature")
	}
	keyID, _ := signatures[0].(map[string]any)["keyId"].(string)
	signer := newEd25519Signer(t, keyID)
	withKey := editList(t, discovery, func(d map[string]any) {
		keys, _ := d["publicKeys"].([]any)
		for _, k := range keys {
			if key, ok := k.(map[string]any); ok && key["keyId"] == keyID {
				key["publicKeyHex"] = hex.EncodeToString(signer.public)
			}
		}
	})
	_, keys, err := (&CachedCRLClient{}).readDiscovery(at, withKey)
	if err != nil {
		t.Fatalf("the client refuses the discovery document example with a key in place of its placeholder: %v", err)
	}
	if keys[keyID].algorithm != crlAlgorithmEd25519 {
		t.Fatalf("the discovery document example lists no Ed25519 key with keyId %q, the keyId of the list example's signature", keyID)
	}
	thisUpdate, err := time.Parse(time.RFC3339, fmt.Sprint(doc["thisUpdate"]))
	if err != nil {
		t.Fatalf("the list example's thisUpdate: %v", err)
	}
	held, err := checkCRL(signList(t, doc, signer), keys, thisUpdate)
	if err != nil {
		t.Fatalf("the list example, signed by the key the discovery document example lists, is refused: %v", err)
	}
	for did := range parsed.revoked {
		if _, revoked := held.revoked[did]; !revoked {
			t.Errorf("the list held does not revoke %q, an agentDid of the list example", did)
		}
	}
}

// The bounds, times and key sizes section 5 states are the client's.
func TestATCSpecSection5_StatesTheListClientsBounds(t *testing.T) {
	requirePublicRepositoryFiles(t)
	section := atcSpecWords(atcSpecSection5(t))

	if maxCRLBodyBytes != maxCRLEnvelopeBytes+maxCRLEntries*maxCRLEntryBytes {
		t.Fatalf("maxCRLBodyBytes is %d, not the envelope bound plus the entries bound", maxCRLBodyBytes)
	}
	for _, want := range []string{
		// Section 5.1.
		fmt.Sprintf("%d bytes for `Ed25519`, %s bytes for `ML-DSA-65`", ed25519.PublicKeySize, atcSpecCount(pqc.MLDSA65PublicKeySize)),
		// Section 5.2.
		fmt.Sprintf("| `agentDid` | Required. The DID of the revoked agent, a string of 1 to %d bytes |", maxCRLAgentDIDBytes),
		fmt.Sprintf("| `revokedByKeyId` | Optional. A string of at most %d bytes |", maxCRLRevokedByKeyIDBytes),
		// Section 5.4.
		fmt.Sprintf("| Discovery document | At most %s bytes |", atcSpecCount(maxDiscoveryBytes)),
		fmt.Sprintf("| List body | At most %s bytes, and a longer body is refused without being read whole |", atcSpecCount(maxCRLBodyBytes)),
		fmt.Sprintf("| Members other than `revocations`, `signatures` included | At most %s bytes in canonical form |", atcSpecCount(maxCRLEnvelopeBytes)),
		fmt.Sprintf("| Entries in `revocations` | At most %s |", atcSpecCount(maxCRLEntries)),
		fmt.Sprintf("| One entry | At most %s bytes in canonical form |", atcSpecCount(maxCRLEntryBytes)),
		fmt.Sprintf("| Arrays and objects in the list | Nested at most %d levels deep, the list's own object included |", maxJSONDepth),
		fmt.Sprintf("The list body bound is %s bytes for the members other than `revocations`, plus %s bytes for each of %s entries.",
			atcSpecCount(maxCRLEnvelopeBytes), atcSpecCount(maxCRLEntryBytes), atcSpecCount(maxCRLEntries)),
		fmt.Sprintf("whose `thisUpdate` is more than %s after its clock", atcSpecDuration(crlClockSkew)),
		fmt.Sprintf("whose `nextUpdate` passed more than %s ago", atcSpecDuration(crlClockSkew)),
		// Section 5.5.
		fmt.Sprintf("| Answered from | The list held, until %s after its last read and until %s after its `nextUpdate`, whichever comes first |",
			atcSpecDuration(crlMaxStaleAge), atcSpecDuration(crlClockSkew)),
		fmt.Sprintf("| Next refresh, after one that holds a list or reads the list held again | %s later, or %s before the `nextUpdate` of the list held when that is sooner |",
			atcSpecDuration(crlRefreshInterval), atcSpecDuration(crlRefreshAhead)),
		fmt.Sprintf("| Next refresh, after one that fails or whose list is refused, or that would start in less than %s | After a wait drawn at random from %d to %s |",
			atcSpecDuration(crlHoldDownMin), int64(crlHoldDownMin/time.Second), atcSpecDuration(crlHoldDownMax)),
		fmt.Sprintf("| One refresh | Ended after %s |", atcSpecDuration(crlRefreshTimeout)),
		fmt.Sprintf("| One request | Ended after %s, the timeout of the HTTP client the list client builds for itself when it is given none, as in the AIM backend |",
			atcSpecDuration(crlHTTPTimeout)),
	} {
		if !strings.Contains(section, want) {
			t.Errorf("section 5 does not say: %s", want)
		}
	}

	// The list's own object is the first of the levels the nesting bound
	// counts.
	nested := func(levels int) []byte {
		arrays := levels - 1
		return []byte(`{"entryCount":0,"nest":` + strings.Repeat("[", arrays) + strings.Repeat("]", arrays) +
			`,"nextUpdate":"2026-10-18T05:05:00Z","revocations":[],"thisUpdate":"2026-10-18T05:00:00Z"}`)
	}
	if _, err := parseCRL(nested(maxJSONDepth)); err != nil {
		t.Errorf("a list nested %d levels deep, its own object included, is refused: %v", maxJSONDepth, err)
	}
	if _, err := parseCRL(nested(maxJSONDepth + 1)); err == nil {
		t.Errorf("a list nested %d levels deep, its own object included, is accepted", maxJSONDepth+1)
	}

	// The HTTP client the list client builds when it is given none.
	c, err := NewCachedCRLClient("https://registry.example.com", "", nil)
	if err != nil {
		t.Fatalf("NewCachedCRLClient: %v", err)
	}
	if c.client.Timeout != crlHTTPTimeout {
		t.Errorf("the HTTP client the list client builds has a timeout of %s, want %s", c.client.Timeout, crlHTTPTimeout)
	}
}

// Each Registry path section 5 names is a GET route of the pinned Registry
// route table, and the discovery paths it names are the client's, in the order
// the client reads them.
func TestATCSpecSection5_NamesOnlyRoutesOfThePinnedRegistryTable(t *testing.T) {
	requirePublicRepositoryFiles(t)
	section := atcSpecSection5(t)
	routes := atcSpecPinnedRoutes(t)

	named := atcSpecRegistryPath.FindAllStringSubmatch(section, -1)
	if len(named) == 0 {
		t.Fatal("no Registry path found in section 5; the path scan is broken, not the section")
	}
	for _, m := range named {
		path := strings.TrimRight(m[1], ".,:;")
		if !routes["GET "+path] {
			t.Errorf("section 5 names %s, which is not a GET route of the pinned Registry route table %s", path, atcSpecRoutesPath)
		}
	}

	if len(discoveryPaths) != 2 {
		t.Fatalf("the client reads %d discovery paths, %v; section 5 names two", len(discoveryPaths), discoveryPaths)
	}
	want := fmt.Sprintf("at `GET %s`, or at `GET %s` when the first answers `404`", discoveryPaths[0], discoveryPaths[1])
	if !strings.Contains(atcSpecWords(section), want) {
		t.Errorf("section 5 does not name the discovery paths the client reads: want %q", want)
	}
}

// Section 5.1 named one of the two conditions under which the list request
// carries the ETag of the list held in If-None-Match, the same address, and
// left out the other: the keys the client uses from the discovery document are
// the ones that list was checked under (#785). The second list request here
// carries the ETag while the discovery document is unchanged or adds a key of
// an algorithm the client does not use, and carries none once the document
// lists another key the client uses, lists the key's keyId with another public
// key, or names another list address. Section 5.1 states both conditions.
func TestATCSpecSection5_StatesWhenTheListRequestCarriesIfNoneMatch(t *testing.T) {
	requirePublicRepositoryFiles(t)
	signer := newEd25519Signer(t, testListKeyID)
	successor := newEd25519Signer(t, "did:web:registry.example#list-2")
	rekeyed := newEd25519Signer(t, testListKeyID)
	unused := discoveryFor(signer)
	unused["publicKeys"] = append(unused["publicKeys"].([]any), map[string]any{
		"keyId": "did:web:registry.example#list-3", "algorithm": "ES256", "publicKeyHex": hex.EncodeToString(successor.public),
	})
	moved := discoveryFor(signer)
	moved["endpoints"] = map[string]any{"revocations": testOverridePath}
	cases := []struct {
		name      string
		discovery map[string]any // the discovery document at the second refresh
		path      string         // the address of the second list request
		etagSent  bool
		outcome   string // the second refresh's refusal; empty when it holds a list or reads the list held again
	}{
		{"the discovery document is unchanged", discoveryFor(signer), testListPath, true, ""},
		{"it adds a key of an algorithm the client does not use", unused, testListPath, true, ""},
		{"it lists another key the client uses beside the key", discoveryFor(signer, successor), testListPath, false, ""},
		{"it lists the key's keyId with another public key", discoveryFor(rekeyed), testListPath, false, CRLOutcomeSignature},
		{"it names another list address", moved, testOverridePath, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			doc := testList(now, revokedDID)
			doc["nextUpdate"] = now.Add(time.Hour).Format(time.RFC3339)
			a := newCRLAuthority(t, discoveryFor(signer))
			a.setList(signList(t, doc, signer))
			a.set(func(a *crlAuthority) { a.etag = `"list-1"` })
			c := newTestCRLClient(t, a, "", &now)
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh: %v", err)
			}

			a.setDiscovery(t, tc.discovery)
			if err := c.Refresh(context.Background()); tc.outcome != "" {
				requireRefreshOutcome(t, err, tc.outcome)
			} else if err != nil {
				t.Fatalf("Refresh with the discovery document changed: %v", err)
			}
			wantPaths, wantIfNoneMatch := []string{testListPath, tc.path}, []string{"", ""}
			if tc.etagSent {
				wantIfNoneMatch[1] = `"list-1"`
			}
			if paths, ifNoneMatch := a.requests(); !reflect.DeepEqual(paths, wantPaths) || !reflect.DeepEqual(ifNoneMatch, wantIfNoneMatch) {
				t.Fatalf("list requests: paths %q, If-None-Match %q; want paths %q, If-None-Match %q",
					paths, ifNoneMatch, wantPaths, wantIfNoneMatch)
			}
		})
	}

	part := atcSpecWords(atcSpecSubsection(t, atcSpecSection5(t), "5.1"))
	for _, want := range []string{
		"When the list it holds was read from the same address with an `ETag`, under the keys the client uses from the discovery document now (the same `keyId`s, each with the same `algorithm` and public key), the request carries that `ETag` in `If-None-Match`, and a `304` answer counts as a read of the list held.",
		"When the address or the keys differ, the request carries no `If-None-Match`, and the list is read in full and checked under the keys used now.",
	} {
		if !strings.Contains(part, want) {
			t.Errorf("section 5.1 does not say: %s", want)
		}
	}
}
