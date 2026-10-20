package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
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

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// AgentRequestV2Middleware counts each request it refuses in
// aim_agent_request_v2_refusals_total, under the refusal's reason and the calling SDK,
// and moves no aim_s1_refusals_total series. This file holds that in two halves.
//
// The driven half sends one request per kind of refusal and reads both families from
// the /metrics exposition before and after it: the refusal moved exactly one series,
// its reason's for the request's SDK, by 1. Every reason of the closed set in the
// metrics package is driven.
//
// The census reads agent_request_v2.go: the middleware answers a statement only by
// handing the request on or through agentRequestRefusal.send, which counts, and every
// refusal it builds names a reason of the closed set.

const v2RefusalsSource = "../../../infrastructure/metrics/agent_request_v2_refusals.go"

var v2RefusalSeriesLine = regexp.MustCompile(`^aim_agent_request_v2_refusals_total\{reason="([^"]*)",sdk="([^"]*)"\} (\S+)$`)

// scrapeRefusals reads both refusal families the way an operator does, from the
// /metrics exposition: each aim_agent_request_v2_refusals_total series as
// "v2.reason.sdk" and each aim_s1_refusals_total series as "s1.reason.sdk".
func scrapeRefusals(t *testing.T) map[string]float64 {
	t.Helper()
	app := fiber.New()
	app.Get("/metrics", metrics.PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	series := map[string]float64{}
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "aim_agent_request_v2_refusals_total{") {
			continue
		}
		m := v2RefusalSeriesLine.FindStringSubmatch(line)
		require.NotNil(t, m, "unexpected series line %q: the labels are reason and sdk, nothing else", line)
		value, err := strconv.ParseFloat(m[3], 64)
		require.NoError(t, err)
		series["v2."+m[1]+"."+m[2]] = value
	}
	s1, _ := scrapeS1(t)
	for name, value := range s1 {
		series["s1."+name] = value
	}
	return series
}

// v2RefusalCase is one refused request: the series it moves and the response the
// caller gets.
type v2RefusalCase struct {
	name   string
	reason string // the aim_agent_request_v2_refusals_total reason it moves
	status int
	code   string // the response's reasonCode
	req    v2Request
	// noAudience mounts the middleware without a public origin.
	noAudience bool
	// setup changes the harness before the request is sent.
	setup func(h *v2Harness)
	// replay sends the request once, accepted, before the counted send.
	replay bool
}

// v2RefusalCases are the refused requests the driven half sends, with the agents
// they name. Each sends a TypeScript SDK User-Agent.
func v2RefusalCases(t *testing.T) ([]v2RefusalCase, []*domain.Agent) {
	keys := newAgentKeys(t)
	edOnly := v2Agent(keys, domain.AgentStatusVerified, false, false)
	both := v2Agent(keys, domain.AgentStatusVerified, true, false)
	hybrid := v2Agent(keys, domain.AgentStatusVerified, true, true)
	suspended := v2Agent(keys, domain.AgentStatusSuspended, false, false)
	malformed := v2Agent(keys, domain.AgentStatusVerified, false, false)
	notAKey := "AAAA" // three bytes, not an Ed25519 public key
	malformed.PublicKey = &notAKey
	ed, ml := pqc.AlgorithmEd25519, pqc.AlgorithmMLDSA65
	window := time.Duration(domain.ActionRequestWindowSeconds) * time.Second

	headers := func(extra ...string) map[string]string {
		h := map[string]string{"User-Agent": "AIM-SDK-TypeScript/1.4.0"}
		for i := 0; i+1 < len(extra); i += 2 {
			h[extra[i]] = extra[i+1]
		}
		return h
	}
	r := func(agent *domain.Agent) v2Request {
		return v2Request{agentID: agent.ID, keys: keys, headers: headers()}
	}
	with := func(req v2Request, change func(*v2Request)) v2Request {
		change(&req)
		return req
	}

	cases := []v2RefusalCase{
		{name: "no configured public origin", reason: "audience_not_configured",
			status: 503, code: "audienceNotConfigured", req: r(edOnly), noAudience: true},
		{name: "a legacy signature header beside the statement", reason: "conflicting_authentication",
			status: 400, code: "conflictingAuthentication",
			req: with(r(edOnly), func(q *v2Request) { q.headers = headers("X-Timestamp", "1760875200") })},
		{name: "a bearer token beside the statement", reason: "conflicting_authentication",
			status: 400, code: "conflictingAuthentication",
			req: with(r(edOnly), func(q *v2Request) { q.headers = headers("Authorization", "Bearer not-a-real-token") })},
		{name: "the statement header sent twice", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.statementTwice = true })},
		{name: "a statement that is not base64url", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.statement = "not base64url" })},
		{name: "legacy signed-request bytes as the payload", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) {
				q.payload = []byte("GET\n" + v2Path + "\n" + strconv.FormatInt(time.Now().Unix(), 10))
			})},
		{name: "a member the format does not define", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.edit = map[string]string{"resource": `"agents"`} })},
		{name: "a nonce of 15 bytes", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.nonce = make([]byte, 15) })},
		{name: "a nonce of 17 bytes", reason: "statement_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.nonce = make([]byte, 17) })},
		{name: "no nonce", reason: "nonce_missing", status: 400, code: "nonceRequired",
			req: with(r(edOnly), func(q *v2Request) { q.edit = map[string]string{"nonce": ""} })},
		{name: "no signature header", reason: "signature_header_malformed", status: 400, code: "invalidRequest",
			req: with(r(edOnly), func(q *v2Request) { q.signatures = []string{} })},
		{name: "a third signature", reason: "signature_header_malformed", status: 400, code: "invalidRequest",
			req: with(r(hybrid), func(q *v2Request) {
				q.signatures = []string{"Ed25519:" + b64url(make([]byte, 64)), "ML-DSA-65:AAAA", "Ed25519:AAAA"}
			})},
		{name: "two Ed25519 signatures", reason: "signature_header_malformed", status: 400, code: "invalidRequest",
			req: with(r(hybrid), func(q *v2Request) { q.algs = []pqc.Algorithm{ed, ed} })},
		{name: "the audience of another deployment", reason: "audience_mismatch", status: 401, code: "audienceMismatch",
			req: with(r(edOnly), func(q *v2Request) { q.edit = map[string]string{"audience": `"https://aim.other.test"`} })},
		{name: "the method of another request", reason: "method_mismatch", status: 401, code: "requestMismatch",
			req: with(r(edOnly), func(q *v2Request) { q.edit = map[string]string{"method": `"POST"`} })},
		{name: "a target without the query sent", reason: "target_mismatch", status: 401, code: "requestMismatch",
			req: with(r(edOnly), func(q *v2Request) {
				q.target = v2Path + "?limit=10"
				q.edit = map[string]string{"target": jsonString(v2Path)}
			})},
		{name: "body_sha256 absent beside a body", reason: "body_mismatch", status: 401, code: "requestMismatch",
			req: with(r(edOnly), func(q *v2Request) {
				q.method, q.body = fiber.MethodPost, `{"a":1}`
				q.edit = map[string]string{"body_sha256": ""}
			})},
		{name: "body_sha256 beside no body", reason: "body_mismatch", status: 401, code: "requestMismatch",
			req: with(r(edOnly), func(q *v2Request) {
				q.edit = map[string]string{"body_sha256": jsonString(sha256B64url(""))}
			})},
		{name: "body_sha256 of other bytes", reason: "body_mismatch", status: 401, code: "requestMismatch",
			req: with(r(edOnly), func(q *v2Request) {
				q.method, q.body = fiber.MethodPost, `{"a":1}`
				q.edit = map[string]string{"body_sha256": jsonString(sha256B64url(`{"a":2}`))}
			})},
		{name: "an unknown agent", reason: "agent_lookup_failed", status: 401, code: "agentKeyNotRecognized",
			req: with(r(edOnly), func(q *v2Request) { q.agentID = uuid.New() })},
		{name: "an algorithm the agent has no key for", reason: "no_registered_key", status: 401, code: "agentKeyNotRecognized",
			req: with(r(edOnly), func(q *v2Request) { q.algs = []pqc.Algorithm{ml} })},
		{name: "an ML-DSA level other than the registered key's", reason: "no_registered_key", status: 401,
			code: "agentKeyNotRecognized",
			req:  with(r(both), func(q *v2Request) { q.algs = []pqc.Algorithm{pqc.AlgorithmMLDSA44} })},
		{name: "hybrid signatures for an agent with one key", reason: "no_registered_key", status: 401,
			code: "agentKeyNotRecognized",
			req:  with(r(edOnly), func(q *v2Request) { q.algs = []pqc.Algorithm{ed, ml} })},
		{name: "a registered key that does not decode", reason: "registered_key_malformed", status: 401,
			code: "agentKeyNotRecognized", req: r(malformed)},
		{name: "a flipped Ed25519 signature", reason: "signature_invalid_ed25519", status: 401, code: "signatureInvalid",
			req: with(r(edOnly), func(q *v2Request) { q.flip = true })},
		{name: "hybrid agent, Ed25519 signature flipped", reason: "signature_invalid_ed25519", status: 401,
			code: "signatureInvalid",
			req:  with(r(hybrid), func(q *v2Request) { q.algs, q.flipOnly = []pqc.Algorithm{ed, ml}, ed })},
		{name: "a flipped ML-DSA signature", reason: "signature_invalid_mldsa", status: 401, code: "signatureInvalid",
			req: with(r(both), func(q *v2Request) { q.algs, q.flipOnly = []pqc.Algorithm{ml}, ml })},
		{name: "hybrid agent, ML-DSA signature flipped", reason: "signature_invalid_mldsa", status: 401,
			code: "signatureInvalid",
			req:  with(r(hybrid), func(q *v2Request) { q.algs, q.flipOnly = []pqc.Algorithm{ed, ml}, ml })},
		{name: "hybrid agent, Ed25519 alone", reason: "hybrid_required", status: 401, code: "hybridSignatureRequired",
			req: with(r(hybrid), func(q *v2Request) { q.algs = []pqc.Algorithm{ed} })},
		{name: "hybrid agent, ML-DSA alone", reason: "hybrid_required", status: 401, code: "hybridSignatureRequired",
			req: with(r(hybrid), func(q *v2Request) { q.algs = []pqc.Algorithm{ml} })},
		{name: "agent not in hybrid mode, two signatures", reason: "single_signature_required", status: 401,
			code: "singleSignatureRequired",
			req:  with(r(both), func(q *v2Request) { q.algs = []pqc.Algorithm{ed, ml} })},
		{name: "a timestamp behind the window", reason: "skew_past", status: 401, code: "timestampOutsideWindow",
			req: with(r(edOnly), func(q *v2Request) { q.timestamp = time.Now().Add(-window - time.Second) })},
		{name: "a timestamp ahead of the window", reason: "skew_future", status: 401, code: "timestampOutsideWindow",
			req: with(r(edOnly), func(q *v2Request) { q.timestamp = time.Now().Add(window + 2*time.Second) })},
		{name: "a nonce sent again", reason: "nonce_reused", status: 401, code: "nonceReused",
			req: with(r(edOnly), func(q *v2Request) { q.nonce = make([]byte, 16) }), replay: true},
		{name: "an admission that fails", reason: "freshness_check_unavailable", status: 503,
			code: "freshnessCheckUnavailable", req: r(edOnly),
			setup: func(h *v2Harness) { h.nonces.err = errors.New("connection refused") }},
		{name: "no admission store", reason: "freshness_check_unavailable", status: 503,
			code: "freshnessCheckUnavailable", req: r(edOnly),
			setup: func(h *v2Harness) { h.mount(v2Audience, nil) }},
		{name: "a suspended agent", reason: "agent_status_denied", status: 401, code: "agentStatusDenied",
			req: r(suspended)},
	}
	return cases, []*domain.Agent{edOnly, both, hybrid, suspended, malformed}
}

func TestAgentRequestV2_CountsEachRefusalUnderItsReason(t *testing.T) {
	cases, agents := v2RefusalCases(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audience := v2Audience
			if tc.noAudience {
				audience = ""
			}
			h := newV2Harness(audience, agents...)
			if tc.setup != nil {
				tc.setup(h)
			}

			before := scrapeRefusals(t)
			if tc.replay {
				status, body := h.send(t, tc.req)
				require.Equal(t, fiber.StatusOK, status, body)
				require.Empty(t, authenticatedMoved(before, scrapeRefusals(t)), "an accepted request moved a refusal series")
			}
			reached := h.reached
			var status int
			var body string
			_ = captureLog(t, func() { status, body = h.send(t, tc.req) })
			require.Equal(t, tc.status, status, body)
			var response map[string]string
			require.NoError(t, json.Unmarshal([]byte(body), &response), body)
			assert.Equal(t, tc.code, response["reasonCode"], body)
			assert.Equal(t, reached, h.reached, "the next handler ran")

			assert.Equal(t, map[string]float64{"v2." + tc.reason + ".typescript": 1},
				authenticatedMoved(before, scrapeRefusals(t)))
		})
	}
}

// The sdk label is derived from the User-Agent as for aim_s1_refusals_total: one of
// the three SDKs, or other.
func TestAgentRequestV2_CountsARefusalUnderTheCallingSDK(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	for _, ua := range s1UserAgents {
		t.Run(ua.sdk, func(t *testing.T) {
			h := newV2Harness(v2Audience, agent)
			before := scrapeRefusals(t)
			status, body := h.send(t, v2Request{agentID: agent.ID, keys: keys, flip: true,
				headers: map[string]string{"User-Agent": ua.header}})
			require.Equal(t, fiber.StatusUnauthorized, status, body)
			assert.Equal(t, map[string]float64{"v2.signature_invalid_ed25519." + ua.sdk: 1},
				authenticatedMoved(before, scrapeRefusals(t)))
		})
	}
}

// A request the middleware authenticates, or hands on because it carries no
// statement, is not a refusal.
func TestAgentRequestV2_CountsNoRefusalForARequestItAcceptsOrHandsOn(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	h := newV2Harness(v2Audience, agent)

	before := scrapeRefusals(t)
	status, body := h.send(t, v2Request{agentID: agent.ID, keys: keys})
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Empty(t, authenticatedMoved(before, scrapeRefusals(t)))

	before = scrapeRefusals(t)
	req := httptest.NewRequest(fiber.MethodGet, v2Path, nil)
	req.Header.Set("User-Agent", "AIM-SDK-TypeScript/1.4.0")
	resp, err := h.app.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, h.reached)
	assert.Empty(t, authenticatedMoved(before, scrapeRefusals(t)))
}

// v2RefusalReasons reads the closed set from the metrics source: each constant of type
// AgentRequestV2RefusalReason, by name, with its value.
func v2RefusalReasons(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, v2RefusalsSource, nil, 0)
	require.NoError(t, err)
	reasons := map[string]string{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			typ, ok := vs.Type.(*ast.Ident)
			if !ok || typ.Name != "AgentRequestV2RefusalReason" {
				continue
			}
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				require.True(t, ok, "%s is not a string literal", name.Name)
				value, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				reasons[name.Name] = value
			}
		}
	}
	require.NotEmpty(t, reasons, "no AgentRequestV2RefusalReason constant found in %s", v2RefusalsSource)
	return reasons
}

// Every reason of the closed set but unclassified is driven above, and nothing else is.
func TestAgentRequestV2_RefusalCasesDriveEveryReason(t *testing.T) {
	want := []string{}
	for _, value := range v2RefusalReasons(t) {
		if value != string(metrics.AgentRequestV2ReasonUnclassified) {
			want = append(want, value)
		}
	}
	cases, _ := v2RefusalCases(t)
	seen := map[string]bool{}
	got := []string{}
	for _, tc := range cases {
		if !seen[tc.reason] {
			seen[tc.reason] = true
			got = append(got, tc.reason)
		}
	}
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got)
}

// v2Census is what the census found in one source file.
type v2Census struct {
	// problems are the lines that break a rule, each with what is wrong.
	problems map[int][]string
	// handler reports that the walk found the handler AgentRequestV2Middleware
	// returns; nexts and sends count its returns of c.Next() and of send(c).
	handler      bool
	nexts, sends int
	// send reports that the walk found agentRequestRefusal.send.
	send bool
	// literals counts the agentRequestRefusal literals, and named the reason
	// constants the file names.
	literals int
	named    map[string]bool
}

// v2RefusalCensus reads one source file and holds it to three rules:
//
//   - the handler AgentRequestV2Middleware returns answers only with c.Next() or
//     with a refusal's send(c);
//   - nothing but agentRequestRefusal.send writes a response;
//   - every refusal names its reason where it is built: an agentRequestRefusal
//     literal has a reason field, and every reason given to a literal or to a
//     function's reason parameter is a constant of the closed set other than
//     unclassified, or the reason parameter of the function it is given in. No
//     refusal's reason is set after it is built.
func v2RefusalCensus(t *testing.T, filename string, src []byte, reasons map[string]string) v2Census {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	require.NoError(t, err)
	out := v2Census{problems: map[int][]string{}, named: map[string]bool{}}
	flag := func(n ast.Node, format string, args ...any) {
		line := fset.Position(n.Pos()).Line
		out.problems[line] = append(out.problems[line], fmt.Sprintf(format, args...))
	}

	// The middleware's handler returns c.Next() or a refusal's send(c), nothing else.
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "AgentRequestV2Middleware" {
			continue
		}
		for _, stmt := range fd.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			lit, ok := ret.Results[0].(*ast.FuncLit)
			if !ok {
				continue
			}
			out.handler = true
			ast.Inspect(lit.Body, func(n ast.Node) bool {
				if _, nested := n.(*ast.FuncLit); nested {
					return false
				}
				ret, ok := n.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				switch {
				case len(ret.Results) == 1 && isCallOn(ret.Results[0], "c", "Next", 0):
					out.nexts++
				case len(ret.Results) == 1 && isSendToC(ret.Results[0]):
					out.sends++
				default:
					flag(ret, "the handler returns something other than c.Next() or a refusal's send(c)")
				}
				return true
			})
		}
	}

	// Only send writes a response.
	writes := map[string]bool{"Status": true, "SendStatus": true, "JSON": true, "Send": true, "SendString": true,
		"Write": true, "WriteString": true, "Redirect": true}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		isSend := fd.Recv != nil && fd.Name.Name == "send"
		out.send = out.send || isSend
		ast.Inspect(fd, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && writes[sel.Sel.Name] {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "c" && !isSend {
					flag(call, "%s writes a response outside agentRequestRefusal.send", fd.Name.Name)
				}
			}
			return true
		})
	}

	// The functions that take a reason, and at which parameter positions.
	reasonParams := map[string][]int{}
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
			i := 0
			for _, field := range fd.Type.Params.List {
				n := len(field.Names)
				if n == 0 {
					n = 1
				}
				for j := 0; j < n; j++ {
					if isReasonType(field.Type) {
						reasonParams[fd.Name.Name] = append(reasonParams[fd.Name.Name], i)
					}
					i++
				}
			}
		}
	}

	// Every refusal names its reason where it is built: a constant of the closed set
	// other than unclassified, or the reason parameter of the function it is built in.
	counted := map[string]bool{}
	for name, value := range reasons {
		counted[name] = value != string(metrics.AgentRequestV2ReasonUnclassified)
	}
	check := func(fn *ast.FuncDecl, expr ast.Expr, where string) {
		if sel, ok := expr.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "metrics" && counted[sel.Sel.Name] {
				out.named[sel.Sel.Name] = true
				return
			}
		}
		if id, ok := expr.(*ast.Ident); ok && fn != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name && isReasonType(field.Type) {
						return
					}
				}
			}
		}
		flag(expr, "%s is not a reason of the closed set other than unclassified", where)
	}
	walk := func(fn *ast.FuncDecl, root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if id, ok := x.Type.(*ast.Ident); ok && id.Name == "agentRequestRefusal" {
					out.literals++
					found := false
					for _, elt := range x.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							flag(x, "an agentRequestRefusal literal without field names")
							continue
						}
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "reason" {
							found = true
							check(fn, kv.Value, "the reason of an agentRequestRefusal literal")
						}
					}
					if !found {
						flag(x, "an agentRequestRefusal is built without a reason")
					}
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok {
					for _, i := range reasonParams[id.Name] {
						if i >= len(x.Args) {
							flag(x, "%s is called without its reason", id.Name)
							continue
						}
						check(fn, x.Args[i], fmt.Sprintf("the reason passed to %s", id.Name))
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "reason" {
						flag(x, "a refusal's reason is set after it is built")
					}
				}
			}
			return true
		})
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			walk(d, d)
		case *ast.GenDecl:
			walk(nil, d)
		}
	}
	return out
}

// The census of agent_request_v2.go.
func TestAgentRequestV2_EveryRefusalIsCountedUnderAReasonOfTheClosedSet(t *testing.T) {
	reasons := v2RefusalReasons(t)
	src, err := os.ReadFile("agent_request_v2.go")
	require.NoError(t, err)
	census := v2RefusalCensus(t, "agent_request_v2.go", src, reasons)

	lines := make([]int, 0, len(census.problems))
	for line := range census.problems {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	for _, line := range lines {
		for _, problem := range census.problems[line] {
			t.Errorf("agent_request_v2.go:%d: %s", line, problem)
		}
	}
	require.True(t, census.handler, "the walk did not find the handler AgentRequestV2Middleware returns")
	require.True(t, census.send, "the walk did not find agentRequestRefusal.send")
	assert.Greater(t, census.nexts, 0, "returns of c.Next()")
	assert.Greater(t, census.sends, 0, "returns of a refusal's send(c)")
	assert.Greater(t, census.literals, 0, "agentRequestRefusal literals")

	// Each reason of the set but unclassified is named in the file, so none is left
	// with no refusal to count.
	want, got := []string{}, []string{}
	for name, value := range reasons {
		if value != string(metrics.AgentRequestV2ReasonUnclassified) {
			want = append(want, name)
		}
	}
	for name := range census.named {
		got = append(got, name)
	}
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got, "the reasons agent_request_v2.go names")
}

// v2PlantedBeside is source appended to agent_request_v2.go: each line marked
// planted breaks a rule of the census.
const v2PlantedBeside = `
var v2PlantedWithoutReason = &agentRequestRefusal{status: fiber.StatusUnauthorized, body: nil} // planted

func v2PlantedRefusals(c fiber.Ctx) *agentRequestRefusal {
	switch c.Get("X-Planted") {
	case "request data as the reason":
		return agentRequestRefused(fiber.StatusUnauthorized, metrics.AgentRequestV2RefusalReason(c.Get("X-Agent-ID")), "planted", "planted") // planted
	case "a string as the reason":
		return agentRequestMismatch("made_up", "planted") // planted
	case "the guard value as the reason":
		return agentRequestKeyNotRecognized(metrics.AgentRequestV2ReasonUnclassified) // planted
	case "a literal without field names":
		return &agentRequestRefusal{fiber.StatusUnauthorized, metrics.AgentRequestV2ReasonNonceReused, nil} // planted
	case "a reason left out":
		return agentRequestMismatch() // planted
	}
	refusal := agentRequestRefused(fiber.StatusUnauthorized, metrics.AgentRequestV2ReasonNonceReused, "planted", "planted")
	refusal.reason = metrics.AgentRequestV2ReasonSkewPast // planted
	return refusal
}

func v2PlantedResponse(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "planted"}) // planted
}
`

// v2PlantedHandler is a whole source file whose handler answers a statement
// without send.
const v2PlantedHandler = `package middleware

func AgentRequestV2Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get("X-A") != "" {
			return fiber.ErrUnauthorized // planted
		}
		if c.Get("X-B") != "" {
			return c.SendStatus(fiber.StatusUnauthorized) // planted
		}
		if c.Get("X-C") != "" {
			return agentRequestNonceReused.send(c)
		}
		return c.Next()
	}
}

func (r *agentRequestRefusal) send(c fiber.Ctx) error {
	return c.Status(r.status).JSON(r.body)
}
`

// The census reports exactly the lines planted to break its rules.
func TestAgentRequestV2RefusalCensus_FindsPlantedRefusals(t *testing.T) {
	reasons := v2RefusalReasons(t)
	src, err := os.ReadFile("agent_request_v2.go")
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		source  string
		planted int
	}{
		"refusals beside the middleware":            {string(src) + v2PlantedBeside, 8},
		"a handler that answers a statement itself": {v2PlantedHandler, 2},
	} {
		t.Run(name, func(t *testing.T) {
			var want []int
			for i, line := range strings.Split(tc.source, "\n") {
				if strings.HasSuffix(line, "// planted") {
					want = append(want, i+1)
				}
			}
			require.Len(t, want, tc.planted, "the planted source changed; update the count")

			census := v2RefusalCensus(t, "planted.go", []byte(tc.source), reasons)
			got := make([]int, 0, len(census.problems))
			for line := range census.problems {
				got = append(got, line)
			}
			sort.Ints(got)
			assert.Equal(t, want, got, "the census must report exactly the planted lines: %v", census.problems)
		})
	}
}

// isCallOn reports whether expr is recv.method(...) with n arguments.
func isCallOn(expr ast.Expr, recv, method string, n int) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != n {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == recv
}

// isSendToC reports whether expr is <refusal>.send(c).
func isSendToC(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "send" {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && arg.Name == "c"
}

// isReasonType reports whether typ is metrics.AgentRequestV2RefusalReason.
func isReasonType(typ ast.Expr) bool {
	sel, ok := typ.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "AgentRequestV2RefusalReason" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "metrics"
}
