package middleware

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// Refusals by the signed-request middlewares are counted, and this file holds that
// in two halves.
//
// The census reads the source of the two middleware files and lists every return
// that refuses a request. Each one has to go through refuseS1 or s1Refused with a
// reason from the closed set in the metrics package, so a refusal added later
// without a reason fails here, at the line it was added on.
//
// The driven half sends one request per branch the census lists and checks that the
// branch moved its own aim_s1_refusals_total series by exactly 1, moved no other
// series, and is on the next "s1_refusals" line. The two halves are compared at the
// end: a branch in the census with no request driving it fails the test.

// ---------------------------------------------------------------------------
// Census
// ---------------------------------------------------------------------------

// The files whose refusals are counted. A middleware file added to the
// signed-request step is added here.
var s1CensusFiles = []string{"ed25519_agent_auth.go", "pqc_agent_auth.go"}

const s1ReasonsSource = "../../../infrastructure/metrics/s1_refusals.go"

const (
	s1Branch   = "branch"   // refuses with a reason constant
	s1Relay    = "relay"    // refuses with the reason a verify helper returned
	s1Untagged = "untagged" // refuses without a reason from the closed set
)

type s1Site struct {
	file   string
	line   int
	fn     string // the top-level function the return is in
	kind   string
	reason string // the metrics constant's name, for a branch
	why    string // what is wrong, for an untagged site
}

func (s s1Site) String() string {
	switch s.kind {
	case s1Branch:
		return fmt.Sprintf("%s:%d %s %s", s.file, s.line, s.fn, s.reason)
	case s1Relay:
		return fmt.Sprintf("%s:%d %s (relays a helper's reason)", s.file, s.line, s.fn)
	default:
		return fmt.Sprintf("%s:%d %s UNTAGGED: %s", s.file, s.line, s.fn, s.why)
	}
}

// s1Census lists every refusal in one source file.
//
// A refusal is a return, in a function whose last result is an error or a
// *s1Refusal, of anything other than nil or a hand-off to the next handler. That
// definition is deliberately wide: it does not look for a status code or a message,
// so a refusal written in a shape nobody anticipated is still listed, as untagged.
func s1Census(filename string, src []byte) ([]s1Site, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var sites []s1Site
	add := func(pos token.Pos, fn, kind, reason, why string) {
		line := fset.Position(pos).Line
		if n := len(sites); n > 0 && kind == s1Untagged && sites[n-1].kind == s1Untagged && sites[n-1].line == line {
			return // one untagged site per line is enough to fail on
		}
		sites = append(sites, s1Site{file: filename, line: line, fn: fn, kind: kind, reason: reason, why: why})
	}

	var walk func(fn string, typ *ast.FuncType, body *ast.BlockStmt)
	walk = func(fn string, typ *ast.FuncType, body *ast.BlockStmt) {
		slot := s1RefusalSlot(typ)
		ast.Inspect(body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				walk(fn, n.Type, n.Body)
				return false
			case *ast.CompositeLit:
				// A reason set by hand skips the constant check that s1Refused gets.
				if id, ok := n.Type.(*ast.Ident); ok && id.Name == "s1Refusal" {
					add(n.Pos(), fn, s1Untagged, "", "s1Refusal built without s1Refused")
				}
			case *ast.ReturnStmt:
				if slot < 0 {
					return true
				}
				if len(n.Results) <= slot {
					add(n.Pos(), fn, s1Untagged, "", "return whose result is not written out")
					return true
				}
				if kind, reason, why := s1ClassifyReturn(n.Results[slot]); kind != "" {
					add(n.Pos(), fn, kind, reason, why)
				}
			}
			return true
		})
	}
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
			walk(fd.Name.Name, fd.Type, fd.Body)
		}
	}
	return sites, nil
}

// s1RefusalSlot is the index of the result a function refuses through, or -1 if it
// returns neither an error nor a *s1Refusal last.
func s1RefusalSlot(typ *ast.FuncType) int {
	if typ.Results == nil || len(typ.Results.List) == 0 {
		return -1
	}
	count := 0
	for _, field := range typ.Results.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	switch last := typ.Results.List[len(typ.Results.List)-1].Type.(type) {
	case *ast.Ident:
		if last.Name == "error" {
			return count - 1
		}
	case *ast.StarExpr:
		if id, ok := last.X.(*ast.Ident); ok && id.Name == "s1Refusal" {
			return count - 1
		}
	}
	return -1
}

// s1ClassifyReturn returns an empty kind for a return that refuses nothing.
func s1ClassifyReturn(result ast.Expr) (kind, reason, why string) {
	switch e := result.(type) {
	case *ast.Ident:
		if e.Name == "nil" {
			return "", "", ""
		}
	case *ast.CallExpr:
		switch fun := e.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == "Next" && len(e.Args) == 0 {
				return "", "", ""
			}
		case *ast.Ident:
			switch {
			case fun.Name == "refuseS1" && len(e.Args) == 4:
				return s1ClassifyReason(e.Args[1], true)
			case fun.Name == "s1Refused" && len(e.Args) >= 2:
				return s1ClassifyReason(e.Args[0], false)
			}
		}
	}
	return s1Untagged, "", "refuses without going through refuseS1 or s1Refused"
}

func s1ClassifyReason(arg ast.Expr, relayAllowed bool) (kind, reason, why string) {
	if sel, ok := arg.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok {
			if x.Name == "metrics" {
				return s1Branch, sel.Sel.Name, ""
			}
			if relayAllowed && sel.Sel.Name == "reason" {
				return s1Relay, "", ""
			}
		}
	}
	return s1Untagged, "", "the reason is not a metrics.S1Reason constant"
}

// s1ReasonConstants reads the closed set from the metrics source: constant name to
// label value.
func s1ReasonConstants(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, s1ReasonsSource, nil, parser.SkipObjectResolution)
	require.NoError(t, err)

	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			typ, ok := vs.Type.(*ast.Ident)
			if !ok || typ.Name != "S1RefusalReason" {
				continue
			}
			require.Len(t, vs.Names, 1)
			require.Len(t, vs.Values, 1)
			value, err := strconv.Unquote(vs.Values[0].(*ast.BasicLit).Value)
			require.NoError(t, err)
			constants[vs.Names[0].Name] = value
		}
	}
	require.NotEmpty(t, constants, "no S1RefusalReason constants found in %s", s1ReasonsSource)
	return constants
}

func s1CensusOfRepository(t *testing.T) []s1Site {
	t.Helper()
	var all []s1Site
	for _, name := range s1CensusFiles {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		sites, err := s1Census(name, src)
		require.NoError(t, err)
		all = append(all, sites...)
	}
	return all
}

func TestS1RefusalCensus_EveryRefusalCarriesAReasonFromTheClosedSet(t *testing.T) {
	constants := s1ReasonConstants(t)
	sites := s1CensusOfRepository(t)

	used := map[string]bool{}
	branches := 0
	for _, site := range sites {
		t.Log(site)
		switch site.kind {
		case s1Untagged:
			t.Errorf("%s:%d (%s) refuses a request without counting it: %s", site.file, site.line, site.fn, site.why)
		case s1Branch:
			branches++
			used[site.reason] = true
			if _, ok := constants[site.reason]; !ok {
				t.Errorf("%s:%d (%s) uses metrics.%s, which is not an S1RefusalReason constant", site.file, site.line, site.fn, site.reason)
			}
			if site.reason == "S1ReasonUnclassified" {
				t.Errorf("%s:%d (%s) uses S1ReasonUnclassified; a refusal branch names why it refused", site.file, site.line, site.fn)
			}
		}
	}
	require.NotZero(t, branches, "the census found no refusal branch at all")

	for name := range constants {
		if name != "S1ReasonUnclassified" && !used[name] {
			t.Errorf("metrics.%s is in the closed set but no refusal branch uses it", name)
		}
	}
}

// The control for the census: refusals planted in a copy of each file, in the shapes
// a refusal can be written without a reason. A census that reports nothing for the
// real files only means something if it reports these.
func TestS1RefusalCensus_FindsPlantedRefusals(t *testing.T) {
	const planted = `
func plantedDirectRefusal(c fiber.Ctx) error {
	if c.Get("X-Planted") != "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "planted"}) // planted
	}
	return c.Next()
}

func plantedHelperRefusal(c fiber.Ctx) *s1Refusal {
	if c.Get("X-Planted") == "literal" {
		return &s1Refusal{message: "planted"} // planted
	}
	if c.Get("X-Planted") == "request data as the reason" {
		return s1Refused(metrics.S1RefusalReason(c.Get("X-Agent-ID")), "planted") // planted
	}
	return nil
}

func plantedErrorRefusal(c fiber.Ctx) error {
	if refusal := plantedHelperRefusal(c); refusal != nil {
		return fmt.Errorf("planted: %s", refusal.message) // planted
	}
	relayed := &s1Refusal{reason: "planted"} // planted
	if c.Get("X-Planted") != "" {
		return refuseS1(c, relayed.reason, fiber.StatusUnauthorized, relayed.message)
	}
	return nil
}
`
	for _, name := range s1CensusFiles {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(name)
			require.NoError(t, err)
			source := string(src) + planted

			var want []int
			for i, line := range strings.Split(source, "\n") {
				if strings.HasSuffix(line, "// planted") {
					want = append(want, i+1)
				}
			}
			require.Len(t, want, 5, "the planted source changed; update the count")

			sites, err := s1Census(name, []byte(source))
			require.NoError(t, err)
			var got []int
			for _, site := range sites {
				if site.kind == s1Untagged {
					got = append(got, site.line)
				}
			}
			assert.Equal(t, want, got, "the census must report exactly the planted refusals, at their lines")
		})
	}
}

// ---------------------------------------------------------------------------
// Driven
// ---------------------------------------------------------------------------

// s1AgentRepo is a domain.AgentRepository stub implementing only GetByID, the one
// method AgentService.GetAgent calls. A nil agent is a failed lookup.
type s1AgentRepo struct {
	domain.AgentRepository
	agent *domain.Agent
}

func (r *s1AgentRepo) GetByID(uuid.UUID) (*domain.Agent, error) {
	if r.agent == nil {
		return nil, errors.New("agent not found")
	}
	return r.agent, nil
}

// s1Fixture is one agent with an Ed25519 and an ML-DSA-65 key, and correct and
// incorrect signatures over the request every case sends: GET /s1 at timestamp.
type s1Fixture struct {
	agentID   uuid.UUID
	timestamp string

	edKey, edOtherKey, edSig, edWrongSig string
	mlKey, mlOtherKey, mlSig, mlWrongSig string
}

const (
	s1NotBase64 = "%%%not-base64%%%"
	s1Path      = "/s1"
)

var s1ShortKey = base64.StdEncoding.EncodeToString([]byte("too short"))

func newS1Fixture(t *testing.T) *s1Fixture {
	t.Helper()
	b64 := base64.StdEncoding.EncodeToString

	f := &s1Fixture{agentID: uuid.New(), timestamp: strconv.FormatInt(time.Now().Unix(), 10)}
	message := []byte("GET\n" + s1Path + "\n" + f.timestamp)
	otherMessage := []byte("GET\n/somewhere-else\n" + f.timestamp)

	edPub, edPriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	edOtherPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	f.edKey, f.edOtherKey = b64(edPub), b64(edOtherPub)
	f.edSig = b64(ed25519.Sign(edPriv, message))
	f.edWrongSig = b64(ed25519.Sign(edPriv, otherMessage))

	ml, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	mlOther, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	mlSig, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, ml.PrivateKey, message)
	require.NoError(t, err)
	mlWrongSig, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, ml.PrivateKey, otherMessage)
	require.NoError(t, err)
	f.mlKey, f.mlOtherKey = b64(ml.PublicKey), b64(mlOther.PublicKey)
	f.mlSig, f.mlWrongSig = b64(mlSig), b64(mlWrongSig)
	return f
}

func (f *s1Fixture) agent() *domain.Agent {
	edKey, mlKey := f.edKey, f.mlKey
	return &domain.Agent{
		ID:                f.agentID,
		OrganizationID:    uuid.New(),
		Status:            domain.AgentStatusVerified,
		PublicKey:         &edKey,
		PQCPublicKey:      &mlKey,
		HybridModeEnabled: true,
	}
}

// The correctly signed request for each algorithm. A case starts from one of these
// and breaks one thing.
func (f *s1Fixture) ed25519Request() map[string]string {
	return map[string]string{
		"X-Agent-ID":   f.agentID.String(),
		"X-Timestamp":  f.timestamp,
		"X-Signature":  f.edSig,
		"X-Public-Key": f.edKey,
	}
}

func (f *s1Fixture) mldsaRequest() map[string]string {
	return map[string]string{
		"X-Agent-ID":       f.agentID.String(),
		"X-Timestamp":      f.timestamp,
		"X-Algorithm":      string(pqc.AlgorithmMLDSA65),
		"X-Signature":      f.mlSig,
		"X-PQC-Public-Key": f.mlKey,
	}
}

func (f *s1Fixture) hybridRequest() map[string]string {
	return map[string]string{
		"X-Agent-ID":          f.agentID.String(),
		"X-Timestamp":         f.timestamp,
		"X-Algorithm":         string(pqc.AlgorithmHybridEd25519MLDSA65),
		"X-Signature-Ed25519": f.edSig,
		"X-Signature-MLDSA":   f.mlSig,
		"X-Public-Key":        f.edKey,
		"X-PQC-Public-Key":    f.mlKey,
	}
}

// The functions a refusal branch can be in, as the census names them.
const (
	s1FnEd25519Middleware = "Ed25519AgentMiddleware"
	s1FnPQCMiddleware     = "PQCAgentMiddleware"
	s1FnVerifyEd25519     = "verifyEd25519Signature"
	s1FnVerifyMLDSA       = "verifyMLDSASignature"
	s1FnVerifyHybrid      = "verifyHybridSignature"
)

func s1FileOf(fn string) string {
	if fn == s1FnEd25519Middleware {
		return "ed25519_agent_auth.go"
	}
	return "pqc_agent_auth.go"
}

type s1Case struct {
	name   string
	fn     string
	reason metrics.S1RefusalReason
	// The response the branch has always returned. Counting a refusal must not
	// change what the caller is told.
	status  int // 401 when zero
	message string
	// prefix marks a message that ends with the verifier's own error text.
	prefix bool
	// change breaks the correctly signed request, the registered agent, or both.
	change func(f *s1Fixture, h map[string]string, a *domain.Agent)
	// noAgent makes the agent lookup fail.
	noAgent bool
	// direct calls a verify helper itself, with an algorithm name the middleware
	// does not pass it. It is how the branch for an algorithm with no key size, and
	// the hybrid branches behind that lookup, are driven without depending on which
	// names the middleware passes.
	direct func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal
}

func s1Cases() []s1Case {
	type F = *s1Fixture
	type H = map[string]string
	type A = *domain.Agent
	str := func(s string) *string { return &s }
	past := func() string { return strconv.FormatInt(time.Now().Unix()-120, 10) }
	future := func() string { return strconv.FormatInt(time.Now().Unix()+120, 10) }

	return []s1Case{
		// --- Ed25519AgentMiddleware ---------------------------------------
		{name: "ed25519: agent id is not a uuid", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonInvalidAgentID,
			message: "Invalid agent ID format",
			change:  func(f F, h H, a A) { h["X-Agent-ID"] = "not-a-uuid" }},
		{name: "ed25519: timestamp is not a number", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonInvalidTimestamp,
			message: "Invalid timestamp format",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = "not-a-number" }},
		{name: "ed25519: timestamp is in the past", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonSkewPast,
			message: "Request timestamp expired or invalid",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = past() }},
		{name: "ed25519: timestamp is in the future", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonSkewFuture,
			message: "Request timestamp expired or invalid",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = future() }},
		{name: "ed25519: agent lookup fails", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonAgentLookupFailed,
			message: "Agent not found", noAgent: true},
		{name: "ed25519: agent is revoked", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonAgentStatusDenied,
			message: "Agent is not permitted to authenticate (status: revoked)",
			change:  func(f F, h H, a A) { a.Status = domain.AgentStatusRevoked }},
		{name: "ed25519: agent has no registered key", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonNoRegisteredKey,
			message: "Agent has no registered public key. Register a key first using JWT authentication.",
			change:  func(f F, h H, a A) { a.PublicKey = nil }},
		{name: "ed25519: presented key is not the registered key", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonPublicKeyMismatch,
			message: "Provided public key does not match registered key",
			change:  func(f F, h H, a A) { h["X-Public-Key"] = f.edOtherKey }},
		{name: "ed25519: registered key is not base64", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "Invalid public key format",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1NotBase64); h["X-Public-Key"] = s1NotBase64 }},
		{name: "ed25519: registered key has the wrong size", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "Invalid public key size: expected 32 bytes, got 9",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1ShortKey); h["X-Public-Key"] = s1ShortKey }},
		{name: "ed25519: signature is not base64", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonSignatureMalformed,
			message: "Invalid signature format",
			change:  func(f F, h H, a A) { h["X-Signature"] = s1NotBase64 }},
		{name: "ed25519: signature is over different bytes", fn: s1FnEd25519Middleware, reason: metrics.S1ReasonSignatureInvalidEd25519,
			message: "Invalid signature",
			change:  func(f F, h H, a A) { h["X-Signature"] = f.edWrongSig }},

		// --- PQCAgentMiddleware -------------------------------------------
		{name: "pqc: algorithm is not supported", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonUnsupportedAlgorithm,
			status: fiber.StatusBadRequest, message: "Unsupported algorithm: RSA-2048",
			change: func(f F, h H, a A) { h["X-Algorithm"] = "RSA-2048" }},
		{name: "pqc: agent id is not a uuid", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonInvalidAgentID,
			message: "Invalid agent ID format",
			change:  func(f F, h H, a A) { h["X-Agent-ID"] = "not-a-uuid" }},
		{name: "pqc: timestamp is not a number", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonInvalidTimestamp,
			message: "Invalid timestamp format",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = "not-a-number" }},
		{name: "pqc: timestamp is in the past", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonSkewPast,
			message: "Request timestamp expired or invalid",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = past() }},
		{name: "pqc: timestamp is in the future", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonSkewFuture,
			message: "Request timestamp expired or invalid",
			change:  func(f F, h H, a A) { h["X-Timestamp"] = future() }},
		{name: "pqc: agent lookup fails", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonAgentLookupFailed,
			message: "Agent not found", noAgent: true},
		{name: "pqc: agent is suspended", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonAgentStatusDenied,
			message: "Agent is not permitted to authenticate (status: suspended)",
			change:  func(f F, h H, a A) { a.Status = domain.AgentStatusSuspended }},
		{name: "pqc: hybrid requested by an agent with no ML-DSA key", fn: s1FnPQCMiddleware, reason: metrics.S1ReasonNoRegisteredKey,
			message: "Agent does not have PQC key registered for hybrid mode",
			change: func(f F, h H, a A) {
				h["X-Algorithm"] = string(pqc.AlgorithmHybridEd25519MLDSA65)
				a.HybridModeEnabled = false
				a.PQCPublicKey = nil
			}},

		// --- verifyEd25519Signature (PQC middleware, Ed25519 algorithm) ----
		{name: "pqc ed25519: signature header is absent", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonMissingSignatureHeaders,
			message: "missing Ed25519 signature or public key",
			change:  func(f F, h H, a A) { delete(h, "X-Signature") }},
		{name: "pqc ed25519: agent has no registered key", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonNoRegisteredKey,
			message: "agent has no registered Ed25519 public key",
			change:  func(f F, h H, a A) { a.PublicKey = nil }},
		{name: "pqc ed25519: presented key is not the registered key", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonPublicKeyMismatch,
			message: "provided Ed25519 public key does not match registered key",
			change:  func(f F, h H, a A) { h["X-Public-Key"] = f.edOtherKey }},
		{name: "pqc ed25519: registered key is not base64", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid public key format",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1NotBase64); h["X-Public-Key"] = s1NotBase64 }},
		{name: "pqc ed25519: registered key has the wrong size", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid Ed25519 public key size",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1ShortKey); h["X-Public-Key"] = s1ShortKey }},
		{name: "pqc ed25519: signature is not base64", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonSignatureMalformed,
			message: "invalid signature format",
			change:  func(f F, h H, a A) { h["X-Signature"] = s1NotBase64 }},
		{name: "pqc ed25519: signature is over different bytes", fn: s1FnVerifyEd25519, reason: metrics.S1ReasonSignatureInvalidEd25519,
			message: "invalid Ed25519 signature",
			change:  func(f F, h H, a A) { h["X-Signature"] = f.edWrongSig }},

		// --- verifyMLDSASignature ------------------------------------------
		{name: "ml-dsa: signature header is absent", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonMissingSignatureHeaders,
			message: "missing ML-DSA signature",
			change:  func(f F, h H, a A) { delete(h, "X-Signature") }},
		{name: "ml-dsa: agent has no registered key", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonNoRegisteredKey,
			message: "agent has no registered ML-DSA public key",
			change:  func(f F, h H, a A) { a.PQCPublicKey = nil }},
		{name: "ml-dsa: presented key is not the registered key", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonPublicKeyMismatch,
			message: "provided ML-DSA public key does not match registered key",
			change:  func(f F, h H, a A) { h["X-PQC-Public-Key"] = f.mlOtherKey }},
		{name: "ml-dsa: registered key is not base64", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid PQC public key format",
			change:  func(f F, h H, a A) { a.PQCPublicKey = str(s1NotBase64); delete(h, "X-PQC-Public-Key") }},
		{name: "ml-dsa: algorithm has no key size", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonUnsupportedAlgorithm,
			message: "unsupported ML-DSA algorithm: ML-DSA-0",
			direct: func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal {
				return verifyMLDSASignature(c, a, pqc.Algorithm("ML-DSA-0"), message)
			}},
		{name: "ml-dsa: registered key has the wrong size", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid ML-DSA public key size: expected 1952, got 9",
			change:  func(f F, h H, a A) { a.PQCPublicKey = str(s1ShortKey); delete(h, "X-PQC-Public-Key") }},
		{name: "ml-dsa: signature is not base64", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonSignatureMalformed,
			message: "invalid ML-DSA signature format",
			change:  func(f F, h H, a A) { h["X-Signature"] = s1NotBase64 }},
		{name: "ml-dsa: signature is over different bytes", fn: s1FnVerifyMLDSA, reason: metrics.S1ReasonSignatureInvalidMLDSA,
			message: "invalid ML-DSA signature: ", prefix: true,
			change: func(f F, h H, a A) { h["X-Signature"] = f.mlWrongSig }},

		// --- verifyHybridSignature -----------------------------------------
		{name: "hybrid: ed25519 signature header is absent", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonMissingSignatureHeaders,
			message: "missing Ed25519 signature for hybrid mode",
			change:  func(f F, h H, a A) { delete(h, "X-Signature-Ed25519") }},
		{name: "hybrid: ml-dsa signature header is absent", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonMissingSignatureHeaders,
			message: "missing ML-DSA signature for hybrid mode",
			change:  func(f F, h H, a A) { delete(h, "X-Signature-MLDSA") }},
		{name: "hybrid: agent has no registered ed25519 key", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonNoRegisteredKey,
			message: "agent has no registered Ed25519 public key for hybrid mode",
			change:  func(f F, h H, a A) { a.PublicKey = nil }},
		{name: "hybrid: agent has no registered ml-dsa key", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonNoRegisteredKey,
			message: "agent has no registered ML-DSA public key for hybrid mode",
			change:  func(f F, h H, a A) { a.PQCPublicKey = nil }},
		{name: "hybrid: presented ed25519 key is not the registered key", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonPublicKeyMismatch,
			message: "provided Ed25519 public key does not match registered key",
			change:  func(f F, h H, a A) { h["X-Public-Key"] = f.edOtherKey }},
		{name: "hybrid: presented ml-dsa key is not the registered key", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonPublicKeyMismatch,
			message: "provided ML-DSA public key does not match registered key",
			change:  func(f F, h H, a A) { h["X-PQC-Public-Key"] = f.mlOtherKey }},
		{name: "hybrid: registered ed25519 key is not base64", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid Ed25519 public key format",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1NotBase64); h["X-Public-Key"] = s1NotBase64 }},
		{name: "hybrid: registered ed25519 key has the wrong size", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid Ed25519 public key size",
			change:  func(f F, h H, a A) { a.PublicKey = str(s1ShortKey); h["X-Public-Key"] = s1ShortKey }},
		{name: "hybrid: ed25519 signature is not base64", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonSignatureMalformed,
			message: "invalid Ed25519 signature format",
			change:  func(f F, h H, a A) { h["X-Signature-Ed25519"] = s1NotBase64 }},
		{name: "hybrid: ed25519 signature is over different bytes", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonSignatureInvalidEd25519,
			message: "invalid Ed25519 signature in hybrid mode",
			change:  func(f F, h H, a A) { h["X-Signature-Ed25519"] = f.edWrongSig }},
		{name: "hybrid: registered ml-dsa key is not base64", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid ML-DSA public key format",
			change:  func(f F, h H, a A) { a.PQCPublicKey = str(s1NotBase64); delete(h, "X-PQC-Public-Key") }},
		{name: "hybrid: algorithm has no key size", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonUnsupportedAlgorithm,
			message: "unsupported ML-DSA algorithm in hybrid mode: Ed25519+ML-DSA-0",
			direct: func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal {
				return verifyHybridSignature(c, a, pqc.Algorithm("Ed25519+ML-DSA-0"), message)
			}},
		{name: "hybrid: registered ml-dsa key has the wrong size", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonRegisteredKeyMalformed,
			message: "invalid ML-DSA public key size: expected 1952, got 9",
			change:  func(f F, h H, a A) { a.PQCPublicKey = str(s1ShortKey); delete(h, "X-PQC-Public-Key") },
			direct: func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal {
				return verifyHybridSignature(c, a, pqc.AlgorithmMLDSA65, message)
			}},
		{name: "hybrid: ml-dsa signature is not base64", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonSignatureMalformed,
			message: "invalid ML-DSA signature format",
			change:  func(f F, h H, a A) { h["X-Signature-MLDSA"] = s1NotBase64 },
			direct: func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal {
				return verifyHybridSignature(c, a, pqc.AlgorithmMLDSA65, message)
			}},
		{name: "hybrid: ml-dsa signature is over different bytes", fn: s1FnVerifyHybrid, reason: metrics.S1ReasonSignatureInvalidMLDSA,
			message: "invalid ML-DSA signature in hybrid mode: ", prefix: true,
			change: func(f F, h H, a A) { h["X-Signature-MLDSA"] = f.mlWrongSig },
			direct: func(c fiber.Ctx, a *domain.Agent, message []byte) *s1Refusal {
				return verifyHybridSignature(c, a, pqc.AlgorithmMLDSA65, message)
			}},
	}
}

// The User-Agent each SDK sends, and one caller that is not an SDK.
var s1UserAgents = []struct{ header, sdk string }{
	{"AIM-Python-SDK/2.0.2", "python"},
	{"AIM-SDK-TypeScript/1.4.0", "typescript"},
	{"AIM-Java-SDK/1.0.0", "java"},
	{"curl/8.7.1", "other"},
}

var s1SeriesLine = regexp.MustCompile(`^aim_s1_refusals_total\{reason="([^"]*)",sdk="([^"]*)"\} (\S+)$`)

// scrapeS1 reads aim_s1_refusals_total the way an operator does, from the /metrics
// exposition. It returns each series as "reason.sdk" and the raw series lines.
func scrapeS1(t *testing.T) (map[string]float64, string) {
	t.Helper()
	app := fiber.New()
	app.Get("/metrics", metrics.PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	series := map[string]float64{}
	var raw []string
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "aim_s1_refusals_total{") {
			continue
		}
		m := s1SeriesLine.FindStringSubmatch(line)
		require.NotNil(t, m, "unexpected series line %q: the labels are reason and sdk, nothing else", line)
		value, err := strconv.ParseFloat(m[3], 64)
		require.NoError(t, err)
		series[m[1]+"."+m[2]] = value
		raw = append(raw, line)
	}
	return series, strings.Join(raw, "\n")
}

// flushS1Line ends the current period and returns what it logged, one entry per
// line, without the logger's timestamp prefix.
func flushS1Line(t *testing.T) []string {
	t.Helper()
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	metrics.FlushS1RefusalLine()
	log.SetOutput(previous)

	var lines []string
	for _, line := range strings.Split(strings.TrimRight(logged.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		i := strings.Index(line, "s1_refusals ")
		require.GreaterOrEqual(t, i, 0, "unexpected log line %q", line)
		lines = append(lines, line[i:])
	}
	return lines
}

// driveS1 sends one request and returns the response status and error message, and
// every value that went into the request or the agent record.
func driveS1(t *testing.T, f *s1Fixture, tc s1Case, userAgent string) (status int, message string, sent []string) {
	t.Helper()

	var headers map[string]string
	switch tc.fn {
	case s1FnVerifyMLDSA:
		headers = f.mldsaRequest()
	case s1FnVerifyHybrid:
		headers = f.hybridRequest()
	default:
		headers = f.ed25519Request()
	}
	agent := f.agent()
	if tc.change != nil {
		tc.change(f, headers, agent)
	}
	sent = append(sent, userAgent, f.agentID.String(), agent.OrganizationID.String())
	for _, key := range []*string{agent.PublicKey, agent.PQCPublicKey} {
		if key != nil {
			sent = append(sent, *key)
		}
	}
	for _, value := range headers {
		sent = append(sent, value)
	}

	repo := &s1AgentRepo{agent: agent}
	if tc.noAgent {
		repo.agent = nil
	}
	agents := application.NewAgentService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// The header buffer the server runs with; an ML-DSA key and signature do not
	// fit in the default.
	app := fiber.New(fiber.Config{ReadBufferSize: 16384})
	switch {
	case tc.direct != nil:
		app.Get(s1Path, func(c fiber.Ctx) error {
			message := []byte(c.Method() + "\n" + c.OriginalURL() + "\n" + c.Get("X-Timestamp"))
			if refusal := tc.direct(c, agent, message); refusal != nil {
				return refuseS1(c, refusal.reason, fiber.StatusUnauthorized, refusal.message)
			}
			return c.SendStatus(fiber.StatusOK)
		})
	case tc.fn == s1FnEd25519Middleware:
		app.Use(Ed25519AgentMiddleware(agents))
	default:
		app.Use(PQCAgentMiddleware(agents))
	}
	app.Get(s1Path, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest("GET", s1Path, nil)
	req.Header.Set("User-Agent", userAgent)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body struct {
		Error string `json:"error"`
	}
	if len(raw) > 0 && raw[0] == '{' {
		require.NoError(t, json.Unmarshal(raw, &body), "response body %q", raw)
	}
	return resp.StatusCode, body.Error, sent
}

// s1Moved lists the series whose value differs between two scrapes.
func s1Moved(before, after map[string]float64) map[string]float64 {
	moved := map[string]float64{}
	for series, value := range after {
		if delta := value - before[series]; delta != 0 {
			moved[series] = delta
		}
	}
	return moved
}

func TestS1Refusal_EachBranchIsCountedOnceAndReachesTheNextLine(t *testing.T) {
	f := newS1Fixture(t)
	cases := s1Cases()

	for i, tc := range cases {
		ua := s1UserAgents[i%len(s1UserAgents)]
		t.Run(tc.name, func(t *testing.T) {
			flushS1Line(t) // refusals counted by earlier tests belong to an earlier line
			before, _ := scrapeS1(t)

			status, message, sent := driveS1(t, f, tc, ua.header)

			wantStatus := tc.status
			if wantStatus == 0 {
				wantStatus = fiber.StatusUnauthorized
			}
			require.Equal(t, wantStatus, status, "the request must be refused by the branch under test")
			if tc.prefix {
				require.True(t, strings.HasPrefix(message, tc.message), "error %q, want prefix %q", message, tc.message)
			} else {
				require.Equal(t, tc.message, message, "the request reached a different branch than the one under test")
			}

			after, exposition := scrapeS1(t)
			series := string(tc.reason) + "." + ua.sdk
			assert.Equal(t, map[string]float64{series: 1}, s1Moved(before, after),
				"the refusal must move aim_s1_refusals_total{reason=%q,sdk=%q} by exactly 1 and no other series",
				tc.reason, ua.sdk)

			lines := flushS1Line(t)
			assert.Equal(t, []string{"s1_refusals period_s=64 total=1 " + series + "=1"}, lines,
				"the refusal must be on the next period line, alone")

			// The refusal precedes authentication: nothing the caller sent, and nothing
			// from the agent record it named, may be recorded.
			for _, value := range sent {
				assert.NotContains(t, exposition, value, "a request value reached a metric label")
				assert.NotContains(t, strings.Join(lines, "\n"), value, "a request value reached the log line")
			}
		})
	}

	// Every branch the census lists has a request driving it, and each request
	// drives a different branch.
	constants := s1ReasonConstants(t)
	type slot struct{ file, fn, reason string }
	inSource := map[slot][]int{}
	for _, site := range s1CensusOfRepository(t) {
		if site.kind == s1Branch {
			key := slot{site.file, site.fn, constants[site.reason]}
			inSource[key] = append(inSource[key], site.line)
		}
	}
	driven := map[slot]int{}
	seen := map[string]string{}
	for _, tc := range cases {
		driven[slot{s1FileOf(tc.fn), tc.fn, string(tc.reason)}]++
		branch := tc.fn + "|" + string(tc.reason) + "|" + tc.message
		if other, dup := seen[branch]; dup {
			t.Errorf("cases %q and %q expect the same response from the same function; they drive one branch twice", other, tc.name)
		}
		seen[branch] = tc.name
	}
	var slots []slot
	for key := range inSource {
		slots = append(slots, key)
	}
	for key := range driven {
		if _, ok := inSource[key]; !ok {
			slots = append(slots, key)
		}
	}
	sort.Slice(slots, func(i, j int) bool { return fmt.Sprint(slots[i]) < fmt.Sprint(slots[j]) })
	for _, key := range slots {
		if len(inSource[key]) != driven[key] {
			t.Errorf("%s: %s has %d refusal branch(es) with reason %q (lines %v) and %d case(s) driving them",
				key.file, key.fn, len(inSource[key]), key.reason, inSource[key], driven[key])
		}
	}
}

// A request the middleware accepts, and one it hands to another auth scheme, are not
// refusals.
func TestS1Refusal_AcceptedAndUnsignedRequestsAreNotCounted(t *testing.T) {
	f := newS1Fixture(t)
	requests := []struct {
		name string
		tc   s1Case
	}{
		{"ed25519 middleware, correctly signed", s1Case{fn: s1FnEd25519Middleware}},
		{"pqc middleware, correctly signed with ed25519", s1Case{fn: s1FnVerifyEd25519}},
		{"pqc middleware, correctly signed with ml-dsa", s1Case{fn: s1FnVerifyMLDSA}},
		{"ed25519 middleware, no signature headers", s1Case{fn: s1FnEd25519Middleware,
			change: func(f *s1Fixture, h map[string]string, a *domain.Agent) { delete(h, "X-Signature") }}},
		{"pqc middleware, no agent id", s1Case{fn: s1FnPQCMiddleware,
			change: func(f *s1Fixture, h map[string]string, a *domain.Agent) { delete(h, "X-Agent-ID") }}},
		{"pqc middleware, bearer token present", s1Case{fn: s1FnPQCMiddleware,
			change: func(f *s1Fixture, h map[string]string, a *domain.Agent) {
				h["Authorization"] = "Bearer not-for-this-middleware"
				h["X-Timestamp"] = "not-a-number"
			}}},
	}
	for _, r := range requests {
		t.Run(r.name, func(t *testing.T) {
			flushS1Line(t)
			before, _ := scrapeS1(t)

			status, _, _ := driveS1(t, f, r.tc, "AIM-Python-SDK/2.0.2")

			require.Equal(t, fiber.StatusOK, status)
			after, _ := scrapeS1(t)
			assert.Empty(t, s1Moved(before, after), "a request that was not refused moved a refusal series")
			assert.Empty(t, flushS1Line(t), "a request that was not refused produced a refusal line")
		})
	}
}
