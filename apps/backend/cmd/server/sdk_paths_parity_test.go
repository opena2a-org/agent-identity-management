package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every request path the shipped clients send is a route the server registers,
// with the method the client sends it with.
//
// The TypeScript client's reportCapabilities posted to a path no route served,
// and nothing compared the clients with the route table: AIM-03 holds one
// feature's path in all three clients, and the Python census in
// sdk_login_routes_contract_test.go holds the Python paths without their
// methods. This guard reads every /api/v1 path literal in the TypeScript,
// Python and Java clients, works out the method each one is sent with from the
// call it sits in, and asks the table the server mounts (serverRouteTable, the
// call main makes, which mounts the SDK-API table through registerSDKAPIRoutes
// as AIM-03 does) whether it answers that method and path.
//
// A path the server does not register today is listed in
// sdkPathsUnregistered with the change that removes it. The list only
// shrinks: an entry whose path the server has come to register, or that no
// client sends any more, fails the test until it is removed.

// sdkPathTree is one shipped client's source tree.
type sdkPathTree struct {
	language string
	dir      string
	ext      string
	// skip reports whether a file under dir is not shipped (a test).
	skip func(rel string) bool
}

var sdkPathTrees = []sdkPathTree{
	{language: "typescript", dir: "sdk/typescript/src", ext: ".ts", skip: func(rel string) bool {
		return strings.HasSuffix(rel, ".test.ts") || strings.HasSuffix(rel, ".spec.ts") ||
			strings.HasSuffix(rel, ".d.ts") || strings.Contains(rel, "/__tests__/")
	}},
	{language: "python", dir: "sdk/python/aim_sdk", ext: ".py", skip: func(rel string) bool {
		return strings.HasPrefix(filepath.Base(rel), "test_") || strings.Contains(rel, "/tests/")
	}},
	{language: "java", dir: "sdk/java/src/main", ext: ".java", skip: func(string) bool { return false }},
}

// sdkUnregisteredPath is a path a client sends that the server does not
// register. fix names the change that removes the entry.
type sdkUnregisteredPath struct {
	method, path string
	fix          string
}

// sdkPathsUnregistered is the measured list of client paths the server does
// not register. It may only shrink.
var sdkPathsUnregistered = []sdkUnregisteredPath{
	{"POST", "/api/v1/a2a/attestations/:param/revoke",
		"the Python and Java A2A clients' revokeAttestation reaches a registered route or is deprecated, as the TypeScript client's is; the server registers no route that revokes a skill attestation"},
	{"GET", "/api/v1/a2a/skills",
		"the Python and Java A2A clients' listSkills reads GET /api/v1/a2a/agents/:id/skills, the route the TypeScript client reads"},
	{"GET", "/api/v1/a2a/consent",
		"the Java A2A client's listUserConsents reads GET /api/v1/a2a/consent/user/:userId, the route the Python and TypeScript clients read"},
	{"GET", "/api/v1/agents/:param/.well-known/agent.json",
		"the Java A2A client's getPublicAgentCard reads a route the server registers; the server serves no agent card under /api/v1/agents"},
	{"GET", "/api/v1/agents/:param/policies",
		"PolicyCache stops fetching a per-agent policy route no server registers (it fails closed today), or the server gains that route"},
}

// sdkPathsUnregisteredCeiling is the length sdkPathsUnregistered had when this
// guard was written. Lower it as entries go; never raise it.
const sdkPathsUnregisteredCeiling = 5

// sdkRegistryPaths are paths the clients send to the OpenA2A Registry, a
// different server: the client joins them to the Registry URL, never to the
// AIM server URL, so the AIM route table is not the place to look for them,
// with whatever method. evidence is where the client joins it to that URL.
var sdkRegistryPaths = []struct{ path, evidence string }{
	{"/api/v1/telemetry/runtime",
		"telemetry/relay.ts and telemetry/relay.py join it to DEFAULT_REGISTRY_URL, arp/telemetry/gtin.ts to resolveRegistryUrl()"},
	{"/api/v1/telemetry/signature", "arp/telemetry/signature/emitter.ts joins it to the configured registryUrl"},
	{"/api/v1/telemetry/sensors/enroll", "arp/telemetry/signature/enroll.ts joins it to resolveRegistryUrl()"},
	{"/api/v1/telemetry/signature/sensor/:param", "arp/telemetry/signature/purge.ts joins it to resolveRegistryUrl()"},
	{"/api/v1/telemetry/behavioral-gradient", "arp/intelligence/runtime-twin.ts posts it to https://api.oa2a.org"},
}

// ---------------------------------------------------------------------------
// The guard
// ---------------------------------------------------------------------------

func TestSdkPathsParity_EveryClientPathIsARegisteredRoute(t *testing.T) {
	table := serverRouteTable(t)
	require.True(t, answers(table, "POST", "/api/v1/sdk-api/agents/:param/capabilities/register"),
		"census sanity: the SDK-API capability registration route must be found")
	require.False(t, answers(table, "GET", "/api/v1/sdk-api/agents/:param/capabilities/register"),
		"census sanity: a registered path asked for with another method must not be found")

	sites := readSDKPathSites(t)

	perLanguage := map[string]int{}
	for _, s := range sites {
		perLanguage[s.language]++
	}
	for _, tree := range sdkPathTrees {
		require.NotZero(t, perLanguage[tree.language],
			"no request path read from %s: the reader is broken, not the client", tree.dir)
	}
	require.Greater(t, len(sites), 150, "the clients send far more paths than were read; the reader is broken")

	unregistered := map[string]sdkUnregisteredPath{}
	for _, u := range sdkPathsUnregistered {
		unregistered[u.method+" "+u.path] = u
	}
	registry := map[string]bool{}
	for _, r := range sdkRegistryPaths {
		registry[r.path] = true
	}

	seenUnregistered := map[string]bool{}
	seenRegistry := map[string]bool{}
	for _, s := range sites {
		if registry[s.path] {
			seenRegistry[s.path] = true
			continue
		}
		if s.method == "" {
			t.Errorf("%s:%d sends %s, and the method it is sent with cannot be read from the call (%s). "+
				"Write the call in a shape the reader knows, or teach the reader the new shape.", s.file, s.line, s.path, s.how)
			continue
		}
		key := s.method + " " + s.path
		served := answers(table, s.method, s.path)
		if u, listed := unregistered[key]; listed {
			seenUnregistered[key] = true
			assert.False(t, served,
				"%s is in sdkPathsUnregistered, and the server registers it now (%s:%d): remove the entry and lower the ceiling (fix: %s)",
				key, s.file, s.line, u.fix)
			continue
		}
		assert.True(t, served, "%s:%d sends %s, which the server does not register (method read from %s)",
			s.file, s.line, key, s.how)
	}

	for _, u := range sdkPathsUnregistered {
		assert.True(t, seenUnregistered[u.method+" "+u.path],
			"no client sends %s %s any more: remove it from sdkPathsUnregistered and lower the ceiling (fix: %s)",
			u.method, u.path, u.fix)
	}
	for _, r := range sdkRegistryPaths {
		assert.True(t, seenRegistry[r.path],
			"no client sends %s to the Registry any more: remove it from sdkRegistryPaths", r.path)
	}
	assert.LessOrEqual(t, len(sdkPathsUnregistered), sdkPathsUnregisteredCeiling,
		"sdkPathsUnregistered only shrinks: a new client path the server does not serve is fixed, not listed")

	t.Logf("checked %d client request paths (typescript %d, python %d, java %d)",
		len(sites), perLanguage["typescript"], perLanguage["python"], perLanguage["java"])
	for _, u := range sdkPathsUnregistered {
		t.Logf("unregistered, allowed until fixed: %s %s (fix: %s)", u.method, u.path, u.fix)
	}
	for _, r := range sdkRegistryPaths {
		t.Logf("sent to the Registry, not checked here: %s (%s)", r.path, r.evidence)
	}
}

// TestSdkPathsParity_DetectionRouterAnnotations holds the API reference to the
// routes main mounts: every @Router annotation in detection_handler.go names a
// method and path the server registers, served by the function it annotates.
func TestSdkPathsParity_DetectionRouterAnnotations(t *testing.T) {
	const file = "apps/backend/internal/interfaces/http/handlers/detection_handler.go"
	src := aim03ReadRepoFile(t, file)

	annotations := regexp.MustCompile(`(?m)^// @Router\s+(\S+)\s+\[(\w+)\]\s*$`).FindAllStringSubmatchIndex(src, -1)
	require.NotEmpty(t, annotations, "no @Router annotation found in %s", file)
	funcDecl := regexp.MustCompile(`(?m)^func \(h \*DetectionHandler\) (\w+)\(`)

	app, _ := strictCensusApp(t)
	routes := app.GetRoutes(true)

	for _, a := range annotations {
		path := src[a[2]:a[3]]
		method := strings.ToUpper(src[a[4]:a[5]])
		decl := funcDecl.FindStringSubmatch(src[a[1]:])
		require.NotNil(t, decl, "the @Router %s annotation in %s annotates no DetectionHandler method", path, file)
		handler := decl[1]

		// @BasePath is /api/v1 (main.go), and swag writes parameters as {id}.
		full := "/api/v1" + path
		want := routeSegments(full)
		var servedBy []string
		for _, r := range routes {
			if r.Method != method || len(r.Handlers) == 0 || !segmentsMatch(routeSegments(r.Path), want) {
				continue
			}
			last := r.Handlers[len(r.Handlers)-1]
			servedBy = append(servedBy, runtime.FuncForPC(reflect.ValueOf(last).Pointer()).Name())
		}
		if !assert.NotEmpty(t, servedBy, "%s annotates %s with @Router %s [%s], and the server registers no %s %s",
			file, handler, path, strings.ToLower(method), method, full) {
			continue
		}

		suffix := ".(*DetectionHandler)." + handler + "-fm"
		var match bool
		for _, name := range servedBy {
			if strings.HasSuffix(name, suffix) {
				match = true
			}
		}
		assert.True(t, match, "%s %s is registered, but served by %v, not by DetectionHandler.%s", method, full, servedBy, handler)
	}
}

// TestSdkPathsParity_ReaderReadsEveryShapeTheClientsWrite pins the reader on
// the call shapes the three clients use, so a green guard means the paths were
// read and not skipped.
func TestSdkPathsParity_ReaderReadsEveryShapeTheClientsWrite(t *testing.T) {
	cases := []struct {
		language, src string
		want          []string
	}{
		{"typescript", strings.Join([]string{
			"const A2A_BASE_PATH = '/api/v1/a2a';",
			"export const EXCHANGE_PATH = '/api/v1/onboarding/bootstrap-tokens/exchange';",
			"// POST /api/v1/commented/out is not a request",
			"/** `POST /api/v1/doc/comment` neither */",
			"class C {",
			"  a() { return this.request<Agent>('GET', `/api/v1/agents/${this.credentials.agentId}`); }",
			"  b() { return this.post<{ id: string; createdAt: string }>(`${A2A_BASE_PATH}/tasks/${id}/state`, {}); }",
			"  c() {",
			"    const path = skillId",
			"      ? `${A2A_BASE_PATH}/agents/${agentId}/attestations?skillId=${skillId}`",
			"      : `${A2A_BASE_PATH}/agents/${agentId}/attestations`;",
			"    return this.get<A2ASkillAttestation[]>(path);",
			"  }",
			"  async d() {",
			"    const res = await fetch(`${this.baseUrl}${EXCHANGE_PATH}`, { method: 'POST', body: x });",
			"    const url = `${this.baseUrl}/api/v1/oauth/token`;",
			"    const replaced = s.replace(/\\/+$/, '');",
			"    return fetch(url, { method: 'POST' });",
			"  }",
			"  e() { return this.host.request('DELETE', `/api/v1/secrets/namespaces/${namespaceId}`); }",
			"}",
		}, "\n"), []string{
			"6 GET /api/v1/agents/:param",
			"7 POST /api/v1/a2a/tasks/:param/state",
			"10 GET /api/v1/a2a/agents/:param/attestations",
			"11 GET /api/v1/a2a/agents/:param/attestations",
			"15 POST /api/v1/onboarding/bootstrap-tokens/exchange",
			"16 POST /api/v1/oauth/token",
			"20 DELETE /api/v1/secrets/namespaces/:param",
		}},
		{"python", strings.Join([]string{
			`POLICY_ROUTE = "/api/v1/agents/{agent_id}/policies"`,
			`def a(self):`,
			`    """Calls GET /api/v1/docstring/only, which is not a request."""`,
			`    # POST /api/v1/comment/only`,
			`    response = self._make_request(`,
			`        method="POST",`,
			`        endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capabilities/register",`,
			`        data={"capabilityType": c},`,
			`    )`,
			`    get_url = f"{aim_url.rstrip('/')}/api/v1/agents/{agent_id}/tags"`,
			`    r = requests.get(get_url, headers=h, timeout=30)`,
			`    return self._make_request("GET", f"/api/v1/agents?{query_string}")`,
			`def b(aim_url):`,
			`    return requests.get(f"{aim_url}{POLICY_ROUTE.format(agent_id=x)}")`,
		}, "\n"), []string{
			"7 POST /api/v1/sdk-api/agents/:param/capabilities/register",
			"10 GET /api/v1/agents/:param/tags",
			"12 GET /api/v1/agents",
			"14 GET /api/v1/agents/:param/policies",
		}},
		{"java", strings.Join([]string{
			`class C {`,
			`    private static final String A2A_BASE_PATH = "/api/v1/a2a";`,
			`    private static final String VERIFICATIONS_PATH = "/api/v1/sdk-api/verifications/";`,
			`    /** Sends {@code POST /api/v1/javadoc/only}. */`,
			`    void a() {`,
			`        String response = post(A2A_BASE_PATH + "/agents/" + aimClient.getAgentId() +`,
			`                "/peers/" + peerAgentId + "/trust", body);`,
			`        put("/api/v1/agents/" + agentId, payload.toString());`,
			`        Request request = new Request.Builder()`,
			`                .url(aimUrl + "/api/v1/auth/refresh")`,
			`                .post(RequestBody.create(json, JSON))`,
			`                .build();`,
			`        Request read = new Request.Builder().url(aimUrl + VERIFICATIONS_PATH + verificationId).build();`,
			`        String url = "/api/v1/sdk-api/agents/" + agentId + "/capability-requests";`,
			`        String listed = get(url);`,
			`    }`,
			`}`,
		}, "\n"), []string{
			"6 POST /api/v1/a2a/agents/:param/peers/:param/trust",
			"8 PUT /api/v1/agents/:param",
			"10 POST /api/v1/auth/refresh",
			"13 GET /api/v1/sdk-api/verifications/:param",
			"14 GET /api/v1/sdk-api/agents/:param/capability-requests",
		}},
	}
	for _, c := range cases {
		t.Run(c.language, func(t *testing.T) {
			src := newSDKSource(c.language, "example", []byte(c.src))
			var got []string
			for _, s := range sdkPathSitesIn(src, map[string]*sdkSource{"example": src}) {
				got = append(got, fmt.Sprintf("%d %s %s", s.line, s.method, s.path))
			}
			assert.Equal(t, c.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Reading the clients
// ---------------------------------------------------------------------------

// sdkPathSite is one request path a client sends.
type sdkPathSite struct {
	language, file string
	line           int
	method, path   string
	// how says where the method was read from, for a failure message.
	how string
}

// readSDKPathSites reads every request path from every shipped client file.
func readSDKPathSites(t *testing.T) []sdkPathSite {
	t.Helper()
	root := aim03RepoRoot(t)
	var sites []sdkPathSite
	for _, tree := range sdkPathTrees {
		sources := map[string]*sdkSource{}
		var order []string
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(tree.dir)), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != tree.ext {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if tree.skip(rel) {
				return nil
			}
			content, err := os.ReadFile(path) //nolint:gosec // G304: a file under the repository's own SDK tree
			if err != nil {
				return err
			}
			sources[rel] = newSDKSource(tree.language, rel, content)
			order = append(order, rel)
			return nil
		})
		require.NoError(t, err, "walk %s", tree.dir)
		sort.Strings(order)
		for _, rel := range order {
			sites = append(sites, sdkPathSitesIn(sources[rel], sources)...)
		}
	}
	return sites
}

// sdkSource is one client file in two views of the same length, so an offset
// means the same place in both, and the bracket depth at each offset:
//
//	text   the source with its comments (and Python docstrings) blanked
//	code   text with every string literal blanked as well, which is what
//	       brackets, statements and call names are read from
type sdkSource struct {
	language, rel string
	text, code    []byte
	depth         []int
	lineStarts    []int
}

func newSDKSource(language, rel string, src []byte) *sdkSource {
	s := &sdkSource{language: language, rel: rel}
	s.text, s.code = lexSDKSource(language, src)
	s.depth = make([]int, len(s.code)+1)
	d := 0
	for i, c := range s.code {
		s.depth[i] = d
		switch c {
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		}
	}
	s.depth[len(s.code)] = d
	s.lineStarts = []int{0}
	for i, c := range src {
		if c == '\n' {
			s.lineStarts = append(s.lineStarts, i+1)
		}
	}
	return s
}

func (s *sdkSource) lineOf(p int) int {
	return sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > p })
}

// lexSDKSource returns the source with comments blanked (text), and with
// comments and string literals blanked (code). Newlines are kept in both.
func lexSDKSource(language string, src []byte) (text, code []byte) {
	text = append([]byte(nil), src...)
	code = append([]byte(nil), src...)
	blank := func(b []byte, from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	n := len(src)
	prev := byte(0) // the last code byte that is not white space
	for i := 0; i < n; {
		c := src[i]
		switch {
		case language == "python" && c == '#',
			language != "python" && c == '/' && i+1 < n && src[i+1] == '/':
			j := i
			for j < n && src[j] != '\n' {
				j++
			}
			blank(text, i, j)
			blank(code, i, j)
			i = j
		case language != "python" && c == '/' && i+1 < n && src[i+1] == '*':
			j := strings.Index(string(src[i+2:]), "*/")
			end := n
			if j >= 0 {
				end = i + 2 + j + 2
			}
			blank(text, i, end)
			blank(code, i, end)
			i = end
		case language == "typescript" && c == '/' && strings.IndexByte("(,=:[!&|?{};+-*%<>~^", prev) >= 0:
			// A regular expression literal: a quote inside it opens nothing.
			j := sdkSkipRegexLiteral(src, i)
			blank(code, i, j)
			i = j
			prev = 'x'
		case c == '"' || c == '\'' || (c == '`' && language == "typescript"):
			j := sdkSkipString(language, src, i)
			triple := j-i >= 6 && i+2 < n && src[i+1] == c && src[i+2] == c
			if language == "python" && triple && (i == 0 || !sdkIsIdentByte(src[i-1])) {
				// A docstring: prose about a request, not a request.
				blank(text, i, j)
			}
			blank(code, i, j)
			i = j
			prev = 'x'
		default:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				prev = c
			}
			i++
		}
	}
	return text, code
}

// sdkSkipString returns the offset just past the string literal that opens at
// src[i]. A single-line literal ends at its line if it is not closed, so a
// misread quote costs one line and not the rest of the file.
func sdkSkipString(language string, src []byte, i int) int {
	n := len(src)
	q := src[i]
	if language != "typescript" && i+2 < n && src[i+1] == q && src[i+2] == q {
		for j := i + 3; j+2 < n; j++ {
			if src[j] == '\\' {
				j++
				continue
			}
			if src[j] == q && src[j+1] == q && src[j+2] == q {
				return j + 3
			}
		}
		return n
	}
	for j := i + 1; j < n; j++ {
		switch {
		case src[j] == '\\':
			j++
		case src[j] == q:
			return j + 1
		case q == '`' && src[j] == '$' && j+1 < n && src[j+1] == '{':
			j = sdkSkipInterpolation(src, j+2) - 1
		case q != '`' && src[j] == '\n':
			return j
		}
	}
	return n
}

// sdkSkipInterpolation returns the offset just past the } that closes a
// template interpolation whose body starts at src[i].
func sdkSkipInterpolation(src []byte, i int) int {
	depth := 1
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1
			}
		case '\'', '"', '`':
			j = sdkSkipString("typescript", src, j) - 1
		}
	}
	return len(src)
}

func sdkSkipRegexLiteral(src []byte, i int) int {
	inClass := false
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '\n':
			return j
		case '/':
			if !inClass {
				j++
				for j < len(src) && sdkIsIdentByte(src[j]) {
					j++
				}
				return j
			}
		}
	}
	return len(src)
}

func sdkIsIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func sdkIsSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// sdkPathConstant matches a constant whose whole value is one /api/v1 literal:
// a base path the client joins more path to, or a path it sends as is.
var sdkPathConstant = map[string]*regexp.Regexp{
	"typescript": regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Z][A-Z0-9_]*)\s*(?::\s*\w+\s*)?=\s*(['"\x60])/api/v1`),
	"python":     regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)\s*(?::\s*\w+\s*)?=\s*(["'])/api/v1`),
	"java":       regexp.MustCompile(`(?m)^\s*(?:(?:private|public|protected|static|final)\s+)+String\s+([A-Z][A-Z0-9_]*)\s*=\s*(")/api/v1`),
}

// sdkPathSitesIn returns the request paths src sends. sources is every file of
// the same client, so a path constant exported by one file is read where
// another file uses it.
func sdkPathSitesIn(src *sdkSource, sources map[string]*sdkSource) []sdkPathSite {
	var sites []sdkPathSite
	add := func(p int, path string) {
		method, how := src.methodAt(p, 3)
		sites = append(sites, sdkPathSite{
			language: src.language, file: src.rel, line: src.lineOf(p),
			method: method, path: path, how: how,
		})
	}

	// The literals, minus each constant's own definition: a constant is a
	// request where it is used.
	defined := map[int]bool{}
	for _, m := range sdkPathConstant[src.language].FindAllSubmatchIndex(src.text, -1) {
		defined[m[4]] = true
	}
	for p := 0; ; {
		k := strings.Index(string(src.text[p:]), "/api/v1")
		if k < 0 {
			break
		}
		at := p + k
		p = at + len("/api/v1")
		if p < len(src.text) && sdkIsIdentByte(src.text[p]) {
			continue // /api/v10
		}
		open := src.literalOpening(at)
		if open < 0 || defined[open] {
			continue
		}
		add(open, src.readPath(at, src.text[open]))
	}

	// The uses of every path constant visible from this file.
	for name, value := range sdkConstantsVisibleFrom(src, sources) {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		for _, m := range re.FindAllIndex(src.text, -1) {
			if src.isDefinitionOrImport(m[0]) || !src.atRequestPosition(m[0]) {
				continue
			}
			add(m[0], src.readAfterConstant(m[0], m[1], value))
		}
	}

	sort.SliceStable(sites, func(i, j int) bool { return sites[i].line < sites[j].line })
	return sites
}

// sdkConstantsVisibleFrom maps each path constant name src can use to its
// path: the file's own definitions first, then a name exactly one other file
// of the client defines.
func sdkConstantsVisibleFrom(src *sdkSource, sources map[string]*sdkSource) map[string]string {
	definedBy := map[string]map[string]string{}
	for rel, other := range sources {
		for _, m := range sdkPathConstant[other.language].FindAllSubmatchIndex(other.text, -1) {
			name := string(other.text[m[2]:m[3]])
			// Untrimmed: "/api/v1/sdk-api/verifications/" + id needs its slash.
			var value strings.Builder
			other.readPathInto(&value, m[5], other.text[m[4]])
			if definedBy[name] == nil {
				definedBy[name] = map[string]string{}
			}
			definedBy[name][rel] = value.String()
		}
	}
	visible := map[string]string{}
	for name, byFile := range definedBy {
		if path, ok := byFile[src.rel]; ok {
			visible[name] = path
			continue
		}
		if len(byFile) == 1 && strings.Contains(string(src.code), name) {
			for _, path := range byFile {
				visible[name] = path
			}
		}
	}
	return visible
}

// literalOpening returns the offset of the quote that opens the string literal
// at holds /api/v1 at the start of, or -1. The path may follow the quote
// directly, a scheme and host, or one leading interpolation (the server URL:
// `${this.baseUrl}/api/v1/...`, f"{aim_url}/api/v1/..."). Anywhere else in a
// string (an error message, a signed canonical string) it is not a request.
func (s *sdkSource) literalOpening(at int) int {
	quotes := `"'`
	if s.language == "typescript" {
		quotes += "`"
	}
	isQuote := func(i int) bool {
		return i >= 0 && strings.IndexByte(quotes, s.text[i]) >= 0 && s.code[i] == ' '
	}
	if isQuote(at - 1) {
		return at - 1
	}
	i := at - 1
	if i >= 0 && s.text[i] == '}' {
		depth := 0
		for ; i >= 0; i-- {
			if s.text[i] == '}' {
				depth++
			} else if s.text[i] == '{' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if i > 0 && s.language == "typescript" && s.text[i-1] == '$' {
			i--
		}
		if isQuote(i - 1) {
			return i - 1
		}
		return -1
	}
	start := at - 200
	if start < 0 {
		start = 0
	}
	if m := sdkURLHost.FindIndex(s.text[start:at]); m != nil && isQuote(start+m[0]-1) {
		return start + m[0] - 1
	}
	return -1
}

// readPath reads the path that starts at s.text[at] inside a literal opened by
// quote, through any concatenation that follows the literal. A parameter, in
// any of the clients' forms, is written :param.
func (s *sdkSource) readPath(at int, quote byte) string {
	var b strings.Builder
	s.readPathInto(&b, at, quote)
	return strings.TrimSuffix(b.String(), "/")
}

// readPathInto appends to b the path in the literal body from s.text[i], and
// in each literal or expression concatenated after it: "..." + expr + "..."
// (Java, and the odd TypeScript or Python join).
func (s *sdkSource) readPathInto(b *strings.Builder, i int, quote byte) {
	for {
		var closed bool
		i, closed = s.readLiteralBody(b, i, quote)
		if !closed {
			return
		}
		j, q, ok := s.readConcatenation(b, i)
		if !ok {
			return
		}
		i, quote = j, q
	}
}

// readLiteralBody appends the path characters of a literal body from
// s.text[i], and reports whether the literal's closing quote was reached with
// nothing after the path in it.
func (s *sdkSource) readLiteralBody(b *strings.Builder, i int, quote byte) (int, bool) {
	for i < len(s.text) {
		c := s.text[i]
		switch {
		case c == quote:
			return i + 1, true
		case s.language == "typescript" && quote == '`' && c == '$' && i+1 < len(s.text) && s.text[i+1] == '{':
			b.WriteString(":param")
			i = sdkSkipInterpolation(s.text, i+2)
		case s.language == "python" && c == '{':
			b.WriteString(":param")
			depth := 0
			for ; i < len(s.text); i++ {
				if s.text[i] == '{' {
					depth++
				} else if s.text[i] == '}' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
			}
		case sdkIsIdentByte(c) || strings.IndexByte("/.-~", c) >= 0:
			b.WriteByte(c)
			i++
		default:
			// A query string, an escape, white space: the path is over.
			return i, false
		}
	}
	return i, false
}

var (
	sdkConcatString = regexp.MustCompile(`^\s*\+\s*(["'\x60])`)
	sdkConcatExpr   = regexp.MustCompile(`^\s*\+\s*[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*|\([^()]*\))*`)
)

// readConcatenation reads one `+ "literal"` or `+ expression` after offset i.
// It returns where the next literal body starts and its quote, or ok=false
// when the path ends here.
func (s *sdkSource) readConcatenation(b *strings.Builder, i int) (int, byte, bool) {
	for {
		rest := s.text[i:]
		if m := sdkConcatString.FindSubmatchIndex(rest); m != nil {
			return i + m[1], rest[m[2]], true
		}
		m := sdkConcatExpr.FindIndex(rest)
		if m == nil || !strings.HasSuffix(b.String(), "/") {
			return i, 0, false
		}
		b.WriteString(":param")
		i += m[1]
		if sdkConcatString.Match(s.text[i:]) {
			continue
		}
		return i, 0, false
	}
}

// readAfterConstant returns the path a use of a path constant builds: the
// constant's value, and what the template or concatenation joins to it.
func (s *sdkSource) readAfterConstant(start, end int, value string) string {
	var b strings.Builder
	b.WriteString(value)
	i := end
	switch {
	case s.language == "typescript" && start >= 2 && string(s.text[start-2:start]) == "${" && i < len(s.text) && s.text[i] == '}':
		// `${NAME}/rest` inside a template literal.
		if open := s.openingQuote(start); open >= 0 {
			s.readPathInto(&b, i+1, s.text[open])
		}
	case s.language == "python" && start >= 1 && s.text[start-1] == '{':
		// f"{base}{NAME}/rest": what follows the constant is more of the literal.
		if open := s.openingQuote(start); open >= 0 {
			for i < len(s.text) && s.text[i] != '}' {
				i++
			}
			s.readLiteralBody(&b, i+1, s.text[open])
		}
	default:
		if j, q, ok := s.readConcatenation(&b, i); ok {
			s.readPathInto(&b, j, q)
		}
	}
	return strings.TrimSuffix(strings.ReplaceAll(b.String(), "//", "/"), "/")
}

// openingQuote returns the offset of the quote that opens the string literal
// offset p is inside, or -1 when p is in code: string bodies are blank in
// code, so it is the first quote after the last byte of code before p.
func (s *sdkSource) openingQuote(p int) int {
	if s.code[p] != ' ' {
		return -1
	}
	i := p - 1
	for i >= 0 && (s.code[i] == ' ' || s.code[i] == '\n') {
		i--
	}
	for i++; i < p; i++ {
		if strings.IndexByte("\"'`", s.text[i]) >= 0 {
			return i
		}
	}
	return -1
}

// isDefinitionOrImport reports whether the name at p is a constant's own
// definition, or sits in an import or export list.
func (s *sdkSource) isDefinitionOrImport(p int) bool {
	start, _ := s.statement(p)
	stmt := strings.TrimSpace(string(s.code[start:p]))
	if strings.HasPrefix(stmt, "import") || strings.HasPrefix(stmt, "from ") || strings.HasPrefix(stmt, "export {") {
		return true
	}
	// import {\n  A,\n  B,\n} from './x' and from x import (\n A,\n B\n).
	if o := s.opener(p); o >= 0 && (s.code[o] == '{' || s.code[o] == '(') {
		from := o - 40
		if from < 0 {
			from = 0
		}
		if sdkImportListOpener.Match(s.code[from:o]) {
			return true
		}
	}
	return sdkConstantDefinition.Match(s.text[p:])
}

// atRequestPosition reports whether a constant used at p inside a string
// literal starts the path the literal holds, as literalOpening asks of a
// literal: `${NAME}/x` or `${baseUrl}${NAME}`, never "serves ${NAME}, so".
func (s *sdkSource) atRequestPosition(p int) bool {
	if s.code[p] != ' ' {
		return true // not inside a string literal
	}
	open := s.openingQuote(p)
	return open >= 0 && sdkLeadingInterpolation.MatchString(string(s.text[open+1:p]))
}

// ---------------------------------------------------------------------------
// Reading the method
// ---------------------------------------------------------------------------

var (
	sdkMethodToken   = regexp.MustCompile(`["'](GET|POST|PUT|PATCH|DELETE)(?:\\n)?["']`)
	sdkFetchMethod   = regexp.MustCompile(`method\s*:\s*['"](GET|POST|PUT|PATCH|DELETE)['"]`)
	sdkBuilderMethod = regexp.MustCompile(`(?i)\.(post|put|patch|delete)\s*\(|\.(get)\s*\(\s*\)|\.method\s*\(\s*"([A-Z]+)"`)
	sdkAssignment    = regexp.MustCompile(`^\s*(?:(?:export|private|public|protected|static|final|readonly|const|let|var)\s+)*(?:[A-Z][\w.<>]*\s+)?([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\s*(?::\s*[^=]+?)?\s*=[^=]`)
	sdkReassignment  = regexp.MustCompile(`^\s*=[^=]`)
	// A Python def or class line: group 1 is its indentation, group 2 its name.
	sdkPythonDefName = regexp.MustCompile(`(?m)^([ \t]*)(?:async\s+def|def|class)\s+(\w+)`)
	// get, post, put, patch, delete, and helpers named for one
	// (postWithSignature).
	sdkVerbCall = regexp.MustCompile(`^((?i:get|post|put|patch|delete))(?:With[A-Z]\w*)?$`)
	// Calls that build a URL and send nothing: HttpUrl.get parses one.
	sdkURLWrappers = map[string]bool{"HttpUrl.get": true, "HttpUrl.parse": true, "URI.create": true}
	// An import or export list a name may sit in: import { A, B } from ...
	sdkImportListOpener = regexp.MustCompile(`(?:^|[^\w$])(?:import|export)(?:\s+type)?\s*(?:[\w$]+\s*,\s*)?$`)

	sdkURLHost              = regexp.MustCompile(`https?://[A-Za-z0-9.\-:]+$`)
	sdkLeadingInterpolation = regexp.MustCompile(`^(?:\$?\{[^{}]*\})?\$?\{$`)
	sdkConstantDefinition   = regexp.MustCompile(`^[A-Z][A-Z0-9_]*\s*(?::\s*\w+\s*)?=[^=]`)
	// fetch, and the injected fetch the clients call it through.
	sdkFetchCall = regexp.MustCompile(`^(?:fetch|doFetch|fetchImpl|fetchFn)$`)
)

// methodAt reads the HTTP method a path at offset p is sent with, from the
// call it is an argument of, or, when it is assigned to a variable, from the
// call that variable is passed to. It returns "" when it cannot tell.
func (s *sdkSource) methodAt(p, hops int) (string, string) {
	start, end := s.statement(p)

	for o := s.opener(p); o >= start && o >= 0; o = s.opener(o) {
		if s.code[o] != '(' {
			break
		}
		name := s.callName(o)
		last := name[strings.LastIndex(name, ".")+1:]
		receiver := strings.TrimSuffix(name, last)
		closeAt := s.closer(o)
		verb := sdkVerbCall.FindStringSubmatch(last)
		switch {
		case verb != nil && !sdkURLWrappers[name] && (receiver == "" || s.isFirstArgument(o, p)):
			// requests.get(url), this.post(path), and the Java clients'
			// post(client, path, body) and postWithSignature(path, body).
			return strings.ToUpper(verb[1]), "the call " + name + "()"
		case last == "url" && strings.HasPrefix(name, "."):
			if m := sdkBuilderMethod.FindSubmatch(s.text[closeAt:end]); m != nil {
				for _, g := range m[1:] {
					if len(g) > 0 {
						return strings.ToUpper(string(g)), "the request builder"
					}
				}
			}
			return "GET", "a request builder with no method (GET)"
		case sdkFetchCall.MatchString(last):
			if m := sdkFetchMethod.FindSubmatch(s.text[o:closeAt]); m != nil {
				return string(m[1]), "fetch's method option"
			}
			return "GET", "fetch with no method option (GET)"
		}
		if m := s.topLevelMethodToken(o, closeAt); m != "" {
			return m, "a method argument of " + name + "()"
		}
	}

	if hops > 0 {
		if m := sdkAssignment.FindSubmatch(s.text[start:p]); m != nil && s.onlyWrappedIn(start, p) {
			variable := string(m[1])
			for _, use := range s.usesAfter(variable, start, end) {
				if method, how := s.methodAt(use, hops-1); method != "" {
					return method, how + " that " + variable + " is passed to"
				}
			}
		}
		// def _policy_url(self): return f"{aim_url}/api/v1/..." is sent by
		// whoever calls _policy_url().
		if fn := s.pythonFunctionReturning(start, p); fn != "" {
			for _, use := range s.callsOf(fn) {
				if method, how := s.methodAt(use, hops-1); method != "" {
					return method, how + " that " + fn + "() is passed to"
				}
			}
		}
	}

	// A canonical string signed for the request: "GET\n" + PATH + ...
	if tokens := s.methodTokens(start, end, -1); len(tokens) == 1 {
		return tokens[0], "the statement's one method name"
	}
	return "", "no call or method name around it"
}

// statement returns the bounds of the statement offset p is in.
func (s *sdkSource) statement(p int) (int, int) {
	if s.language == "python" {
		start := 0
		for i := p - 1; i >= 0; i-- {
			if s.code[i] == '\n' && s.depth[i] == 0 && (i == 0 || s.code[i-1] != '\\') {
				start = i + 1
				break
			}
		}
		end := len(s.code)
		for i := p; i < len(s.code); i++ {
			if s.code[i] == '\n' && s.depth[i] == 0 && (i == 0 || s.code[i-1] != '\\') {
				end = i
				break
			}
		}
		return start, end
	}
	o := s.opener(p)
	for o >= 0 && s.code[o] != '{' {
		o = s.opener(o)
	}
	base := 0
	if o >= 0 {
		base = s.depth[o] + 1
	}
	start := o + 1
	for i := p - 1; i > o; i-- {
		c := s.code[i]
		if c == ';' && s.depth[i] == base {
			start = i + 1
			break
		}
		if c == '}' && s.depth[i+1] == base && sdkFollowedByNewline(s.code, i+1) {
			start = i + 1
			break
		}
	}
	end := len(s.code)
	for i := p; i < len(s.code); i++ {
		if (s.code[i] == ';' || s.code[i] == '}') && s.depth[i] == base {
			end = i
			break
		}
	}
	return start, end
}

func sdkFollowedByNewline(b []byte, i int) bool {
	for ; i < len(b); i++ {
		switch b[i] {
		case ' ', '\t', '\r':
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

// opener returns the offset of the innermost unclosed bracket before p, or -1.
func (s *sdkSource) opener(p int) int {
	depth := 0
	for i := p - 1; i >= 0; i-- {
		switch s.code[i] {
		case ')', ']', '}':
			depth++
		case '(', '[', '{':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// closer returns the offset of the bracket that closes the one at o.
func (s *sdkSource) closer(o int) int {
	depth := 0
	for i := o; i < len(s.code); i++ {
		switch s.code[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s.code)
}

// callName returns the dotted name called with the ( at o: this.request,
// requests.get, post, .url. Type arguments (request<Agent>) are skipped.
func (s *sdkSource) callName(o int) string {
	j := o - 1
	for j >= 0 && sdkIsSpace(s.code[j]) {
		j--
	}
	if j >= 0 && s.code[j] == '>' {
		depth := 0
		for ; j >= 0; j-- {
			if s.code[j] == '>' {
				depth++
			} else if s.code[j] == '<' {
				depth--
				if depth == 0 {
					j--
					break
				}
			}
		}
		for j >= 0 && sdkIsSpace(s.code[j]) {
			j--
		}
	}
	end := j + 1
	for j >= 0 && (sdkIsIdentByte(s.code[j]) || s.code[j] == '.') {
		j--
	}
	return string(s.code[j+1 : end])
}

// isFirstArgument reports whether p is in the first argument of the call
// opened at o.
func (s *sdkSource) isFirstArgument(o, p int) bool {
	for i := o + 1; i < p; i++ {
		if s.code[i] == ',' && s.depth[i] == s.depth[o]+1 {
			return false
		}
	}
	return true
}

// onlyWrappedIn reports whether every bracket around p that opens after start
// is a call that only builds a URL (HttpUrl.get(...)), so the statement assigns
// the path itself and not the result of a request.
func (s *sdkSource) onlyWrappedIn(start, p int) bool {
	for o := s.opener(p); o >= start && o >= 0; o = s.opener(o) {
		if s.code[o] != '(' || !sdkURLWrappers[s.callName(o)] {
			return false
		}
	}
	return true
}

// pythonFunctionReturning returns the name of the Python function whose
// return statement spans start..p, or "".
func (s *sdkSource) pythonFunctionReturning(start, p int) string {
	if s.language != "python" || !strings.HasPrefix(strings.TrimSpace(string(s.code[start:p])), "return") {
		return ""
	}
	indent := s.indentAt(start)
	var name string
	for _, m := range sdkPythonDefName.FindAllSubmatchIndex(s.text[:start], -1) {
		if m[3]-m[2] < indent {
			name = string(s.text[m[4]:m[5]])
		}
	}
	return name
}

// callsOf returns the offsets of every call of the function or method name.
func (s *sdkSource) callsOf(name string) []int {
	re := regexp.MustCompile(`(?:^|[^\w$])(?:self\.|this\.)?(` + regexp.QuoteMeta(name) + `)\s*\(`)
	var calls []int
	for _, m := range re.FindAllSubmatchIndex(s.text, -1) {
		if strings.Contains(string(s.text[s.lineStartOf(m[2]):m[2]]), "def ") {
			continue // the definition, not a call
		}
		calls = append(calls, m[2])
	}
	return calls
}

func (s *sdkSource) lineStartOf(p int) int {
	return s.lineStarts[s.lineOf(p)-1]
}

// indentAt returns the indentation of the line offset p is on.
func (s *sdkSource) indentAt(p int) int {
	i := s.lineStartOf(p)
	n := 0
	for i+n < len(s.text) && (s.text[i+n] == ' ' || s.text[i+n] == '\t') {
		n++
	}
	return n
}

// topLevelMethodToken returns the one method name passed as an argument of
// the call opened at o, or "".
func (s *sdkSource) topLevelMethodToken(o, closeAt int) string {
	tokens := s.methodTokens(o+1, closeAt, s.depth[o]+1)
	if len(tokens) == 1 {
		return tokens[0]
	}
	return ""
}

// methodTokens returns the distinct method-name literals in text[from:to], at
// bracket depth depth (any depth when depth is -1).
func (s *sdkSource) methodTokens(from, to, depth int) []string {
	if to > len(s.text) {
		to = len(s.text)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range sdkMethodToken.FindAllSubmatchIndex(s.text[from:to], -1) {
		if depth >= 0 && s.depth[from+m[0]] != depth {
			continue
		}
		method := string(s.text[from+m[2] : from+m[3]])
		if !seen[method] {
			seen[method] = true
			out = append(out, method)
		}
	}
	return out
}

// usesAfter returns the offsets where variable is read after the statement
// start..from that assigns it, within the function it is assigned in. A field
// (this.url, self.url) is read anywhere after it.
func (s *sdkSource) usesAfter(variable string, start, from int) []int {
	limit := len(s.text)
	switch {
	case strings.HasPrefix(variable, "this.") || strings.HasPrefix(variable, "self."):
	case s.language == "python":
		// The function ends at the next def or class indented less than the
		// assignment; a def nested inside it (a retry closure) is still in it.
		indent := s.indentAt(start)
		for _, m := range sdkPythonDefName.FindAllSubmatchIndex(s.text[from:], -1) {
			if m[3]-m[2] < indent {
				limit = from + m[0]
				break
			}
		}
	default:
		base := s.depth[from]
		for i := from; i < len(s.code); i++ {
			if s.depth[i] < base {
				limit = i
				break
			}
		}
	}
	re := regexp.MustCompile(`(?:^|[^\w$.])(` + regexp.QuoteMeta(variable) + `)\b`)
	var uses []int
	for _, m := range re.FindAllSubmatchIndex(s.text[from:limit], -1) {
		at := from + m[2]
		if sdkReassignment.Match(s.text[from+m[3]:]) {
			continue // reassigned, not read
		}
		uses = append(uses, at)
	}
	return uses
}
