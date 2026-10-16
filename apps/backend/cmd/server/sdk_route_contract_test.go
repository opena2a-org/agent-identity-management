package main

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
	"github.com/stretchr/testify/require"
)

// The SDK clients are written by hand against the server's route table, so a
// call can drift from the route it calls: a path the server does not mount, a
// method it answers 405 to, or body members the handler's bind struct does not
// read. A test per route catches that for its own route only.
//
// TestSDKCallsMatchTheRoutesTheyCall reads the Python client
// (sdk/python/aim_sdk/client.py) and the TypeScript clients
// (sdk/typescript/src/**/*Client.ts) as text, takes every HTTP call they make
// (method, path, and the top-level members of the JSON body) and checks each one
// against the routes the server mounts (serverRouteTable): the route exists for
// that method and does not answer 405, and each member is a JSON name of the
// struct the route's handler binds. The handler is the one the route table
// holds; its bind struct is found by parsing the handler's package. No SDK code
// runs. Members nested inside a body member and query strings are not checked.

// sdkRouteContractAllowList holds each mismatch the clients carried when this
// test was written and that a client-side rename does not settle, keyed
// "<file> <function> <METHOD> <path> <what>", with why it stays. The list may
// only shrink: an entry that matches no mismatch fails the test, and
// sdkRouteContractAllowListCeiling caps its length. Remove an entry when its
// call is fixed and lower the ceiling with it; never add an entry or raise the
// ceiling.
var sdkRouteContractAllowList = map[string]string{
	pyClient + " report_execution_status POST /api/v1/sdk-api/verifications/{verification_id}/execution-status member strictMode": "the server takes strict mode from the agent's organization on purpose and never from the request",
	pyClient + " _register_via_oauth POST /api/v1/agents member organizationDomain":                                               allowOrganizationDomain,
	pyClient + " _register_via_api_key POST /api/v1/agents member organizationDomain":                                             allowOrganizationDomain,

	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member agentId":        allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member aimAttestation": allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member capabilities":   allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member createdAt":      allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member displayName":    allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member id":             allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member publicKey":      allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member updatedAt":      allowWholeAgentCard,
	a2aClient + " updateAgentCard PUT /api/v1/a2a/cards/{agentId} member url":            allowWholeAgentCard,

	a2aClient + " verifyRequest POST /api/v1/a2a/verify member body":      allowVerifyRequestShape,
	a2aClient + " verifyRequest POST /api/v1/a2a/verify member method":    allowVerifyRequestShape,
	a2aClient + " verifyRequest POST /api/v1/a2a/verify member path":      allowVerifyRequestShape,
	a2aClient + " verifyRequest POST /api/v1/a2a/verify member publicKey": allowVerifyRequestShape,

	a2aClient + " updateTrustScore PUT /api/v1/a2a/trust/{targetAgentId} route": "the server answers 405 to every trust score write and names the interaction route instead; whether the method stays is an API decision",

	a2aClient + " evaluatePolicy POST /api/v1/a2a/policies/evaluate member action":        allowPolicyShape,
	a2aClient + " evaluatePolicy POST /api/v1/a2a/policies/evaluate member context":       allowPolicyShape,
	a2aClient + " evaluatePolicy POST /api/v1/a2a/policies/evaluate member resource":      allowPolicyShape,
	a2aClient + " evaluatePolicy POST /api/v1/a2a/policies/evaluate member targetAgentId": allowPolicyShape,

	a2aClient + " registerSkill POST /api/v1/a2a/skills member attestationCount": allowWholeSkill,
	a2aClient + " registerSkill POST /api/v1/a2a/skills member inputSchema":      allowWholeSkill,
	a2aClient + " registerSkill POST /api/v1/a2a/skills member isVerified":       allowWholeSkill,
	a2aClient + " registerSkill POST /api/v1/a2a/skills member outputSchema":     allowWholeSkill,
	a2aClient + " registerSkill POST /api/v1/a2a/skills member verifiedAt":       allowWholeSkill,
}

const sdkRouteContractAllowListCeiling = 26

const (
	pyClient  = "sdk/python/aim_sdk/client.py"
	a2aClient = "sdk/typescript/src/a2a/A2AClient.ts"

	allowOrganizationDomain = "the agent create struct has no organizationDomain; only the public registration handler reads one, so the intended route is unclear"
	allowWholeAgentCard     = "the client sends whatever card fields its caller passes; the update struct reads name, description, version, endpoint and skills, and url is not plainly endpoint"
	allowVerifyRequestShape = "the server verifies an agentId and requestHash, while the client sends the method, path, body and public key; matching them changes the protocol, not a name"
	allowPolicyShape        = "the server evaluates a different request (event, clientAgentId, remoteAgentId, skillId); no member maps one to one"
	allowWholeSkill         = "the client sends whatever skill fields its caller passes, including ones the server computes; narrowing the parameter type is an API change"
)

// sdkCall is one HTTP call an SDK makes, as read from its source.
type sdkCall struct {
	file     string // repository-relative
	line     int
	function string
	method   string
	path     string   // each path parameter as {name}, no query string
	members  []string // the JSON body's top-level members, sorted
}

func (c sdkCall) where() string { return fmt.Sprintf("%s:%d (%s)", c.file, c.line, c.function) }

// sdkExtraction is what one client's sources yielded: every call site the
// scan found is either a checked call or a call inside a request helper that
// passes on its caller's method and path. unreadable names each call the test
// could not read, and each server path in the sources that no read call uses.
type sdkExtraction struct {
	sites      int
	calls      []sdkCall
	forwarded  []string
	unreadable []string
}

func TestSDKCallsMatchTheRoutesTheyCall(t *testing.T) {
	requirePublicRepositoryFiles(t)
	root := aim03RepoRoot(t)
	table := serverRouteTable(t)
	binds := newBindIndex(t, filepath.Join(root, "apps", "backend"))

	clients := []struct {
		name string
		ex   sdkExtraction
	}{
		{"Python", extractPythonCalls(t, root)},
		{"TypeScript", extractTypeScriptCalls(t, root)},
	}

	found := map[string]string{}
	for _, client := range clients {
		ex := client.ex
		require.Empty(t, ex.unreadable, "%s: calls the test could not read; teach the extractor the new shape rather than skipping the call", client.name)
		require.Equal(t, ex.sites, len(ex.calls)+len(ex.forwarded),
			"%s: %d call sites, but %d calls were read and %d forwarded by a request helper; a call was skipped", client.name, ex.sites, len(ex.calls), len(ex.forwarded))
		require.NotEmpty(t, ex.calls, "%s: no calls read; the scan is broken, not the client", client.name)
		mismatches := 0
		for _, call := range ex.calls {
			for key, detail := range checkSDKCall(table, binds, call) {
				found[key] = detail
				mismatches++
			}
		}
		t.Logf("%s: %d call sites; %d calls checked, %d inside request helpers; %d mismatches", client.name, ex.sites, len(ex.calls), len(ex.forwarded), mismatches)
	}

	var unlisted, stale []string
	for key, detail := range found {
		if _, ok := sdkRouteContractAllowList[key]; !ok {
			unlisted = append(unlisted, detail+"\n    allow-list key: "+key)
		}
	}
	for key := range sdkRouteContractAllowList {
		if _, ok := found[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	require.Empty(t, unlisted, "SDK calls that disagree with the route they call:\n%s", strings.Join(unlisted, "\n"))
	require.Empty(t, stale, "allow-list entries that match no mismatch any more; remove them and lower sdkRouteContractAllowListCeiling")
	require.LessOrEqual(t, len(sdkRouteContractAllowList), sdkRouteContractAllowListCeiling, "the allow-list only shrinks")
	t.Logf("%d mismatches, all allow-listed", len(found))
}

// checkSDKCall returns each way call disagrees with the server, keyed for the
// allow-list.
func checkSDKCall(table []docRoute, binds *bindIndex, call sdkCall) map[string]string {
	out := map[string]string{}
	key := func(what string) string {
		return strings.Join([]string{call.file, call.function, call.method, call.path, what}, " ")
	}
	report := func(what, detail string) {
		out[key(what)] = fmt.Sprintf("%s: %s %s: %s", call.where(), call.method, call.path, detail)
	}

	want := routeSegments(call.path)
	var route *docRoute
	var methods []string
	for i := range table {
		if !segmentsMatch(table[i].segments, want) {
			continue
		}
		if table[i].method == call.method {
			route = &table[i]
			break
		}
		methods = append(methods, table[i].method)
	}
	if route == nil {
		if len(methods) == 0 {
			report("route", "the server mounts no route at this path")
		} else {
			report("route", "the server mounts this path for "+strings.Join(sortedUnique(methods), ", ")+", not "+call.method)
		}
		return out
	}

	handler, run := routeHandler(route.handlers)
	if run != nil && refusesMethod(run, call.method) {
		report("route", "the server answers 405 to "+call.method+" here")
		return out
	}
	if handler == "" {
		if len(call.members) > 0 {
			report("handler", "the route's handler is not a handler method, so its bind struct cannot be read")
		}
		return out
	}
	if len(call.members) == 0 {
		return out
	}
	bound, err := binds.bodyFields(handler)
	if err != nil {
		report("body", err.Error())
		return out
	}
	for _, member := range call.members {
		if bound.reads(member) {
			continue
		}
		if len(bound) == 0 {
			report("member "+member, fmt.Sprintf("sends %q, but %s binds no JSON body", member, shortHandlerName(handler)))
		} else {
			report("member "+member, fmt.Sprintf("sends %q, which the struct %s binds does not read (it reads %s)", member, shortHandlerName(handler), strings.Join(bound.names(), ", ")))
		}
	}
	return out
}

// boundBody is each struct a handler decodes the request body into, with the
// JSON names it carries and whether the decoder first turns snake_case keys
// into camelCase (utils.UnmarshalFlexibleJSON).
type boundBody []boundStruct

type boundStruct struct {
	fields   map[string]bool
	flexible bool
}

// reads reports whether one of the decodes sets a field for member: encoding/json
// matches a JSON name exactly or ignoring case.
func (b boundBody) reads(member string) bool {
	for _, body := range b {
		key := member
		if body.flexible {
			for k := range utils.CamelCaseKeys(map[string]interface{}{member: nil}) {
				key = k
			}
		}
		for name := range body.fields {
			if name == key || strings.EqualFold(name, key) {
				return true
			}
		}
	}
	return false
}

func (b boundBody) names() []string {
	all := map[string]bool{}
	for _, body := range b {
		merge(all, body.fields)
	}
	return sortedKeys(all)
}

// boundHandlerName matches a method value's function name:
// ".../handlers.(*AgentHandler).CreateAgent-fm".
var boundHandlerName = regexp.MustCompile(`^(.+)\.\(\*?(\w+)\)\.(\w+)-fm$`)

// routeHandler returns the last of a route's handlers that is a method of the
// handlers package (the middleware runs before it) with its name; when none
// is, the route's last handler and no name.
func routeHandler(handlers []fiber.Handler) (string, fiber.Handler) {
	for i := len(handlers) - 1; i >= 0; i-- {
		name := runtime.FuncForPC(reflect.ValueOf(handlers[i]).Pointer()).Name()
		if m := boundHandlerName.FindStringSubmatch(name); m != nil && strings.HasSuffix(m[1], "/interfaces/http/handlers") {
			return name, handlers[i]
		}
	}
	if len(handlers) == 0 {
		return "", nil
	}
	return "", handlers[len(handlers)-1]
}

func shortHandlerName(name string) string {
	if m := boundHandlerName.FindStringSubmatch(name); m != nil {
		return m[2] + "." + m[3]
	}
	return name
}

// refusesMethod reports whether h, run on its own with no request state,
// answers 405: the route mountA2AVerifyRoute registers for every method but
// POST, or a handler that refuses a method outright. A handler that needs its
// services panics, which the probe answers 500.
func refusesMethod(h fiber.Handler, method string) bool {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) (err error) {
		defer func() {
			if recover() != nil {
				err = c.SendStatus(fiber.StatusInternalServerError)
			}
		}()
		return c.Next()
	})
	app.Add([]string{method}, "/probe", h)
	resp, err := app.Test(httptest.NewRequest(method, "/probe", nil))
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == fiber.StatusMethodNotAllowed
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
