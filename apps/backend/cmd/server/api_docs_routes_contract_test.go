package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// apiReferenceDocs are the pages that tell a reader which HTTP paths the
// server answers: the API references, the tutorials and guides that call the
// API, and the troubleshooting pages that name endpoints.
var apiReferenceDocs = []string{
	"docs/API.md",
	"docs/guides/API_REFERENCE.md",
	"docs/guides/API_ENDPOINT_SUMMARY.md",
	"docs/api/direct-api-usage.md",
	"docs/tutorials/api-quickstart.md",
	"docs/tutorials/mcp-registration.md",
	"docs/use-cases/fleet-governance.md",
	"docs/sdk/trust-scoring.md",
	"docs/troubleshooting/README.md",
	"docs/troubleshooting/authentication.md",
	"docs/guides/A2A.md",
	"docs/SDK_TEST_PLAN.md",
	"sdk/python/docs/LANGCHAIN_INTEGRATION.md",
	"docs/specs/capability-scope-v1.md",
}

// docRoute is one method and path the server registers.
type docRoute struct {
	method   string
	segments []string
}

// serverRouteTable returns every route the server answers: the /api/v1 table
// mountAPIRoutes builds (the call main makes), the routes main registers on
// the bare app before it, and the metrics path.
func serverRouteTable(t *testing.T) []docRoute {
	t.Helper()
	app, _ := strictCensusApp(t)
	var table []docRoute
	add := func(method, path string) {
		table = append(table, docRoute{method: method, segments: routeSegments(path)})
	}
	for _, r := range app.GetRoutes(true) {
		add(r.Method, r.Path)
	}

	bare := regexp.MustCompile(`(?m)^\s*app\.(Get|Post|Put|Patch|Delete)\("([^"]+)"`)
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	found := 0
	for _, m := range bare.FindAllStringSubmatch(mainSrc, -1) {
		add(strings.ToUpper(m[1]), m[2])
		found++
	}
	require.GreaterOrEqual(t, found, 4, "main.go registers /health and the discovery documents on the bare app; the scan found too few")
	add(http.MethodGet, metricsPath)
	return table
}

func routeSegments(path string) []string {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return []string{""}
	}
	return strings.Split(path, "/")
}

// docPlaceholder matches a path segment a reader substitutes: {id}, ${ID},
// $AGENT_ID, <agent-id>, :id, {{id}}, AGENT_ID, YOUR_AGENT_ID.
var docPlaceholder = regexp.MustCompile(`^(?:\{.*\}|\$\{?[A-Za-z_]+\}?|<[^>]+>|:[A-Za-z_]+|[A-Z][A-Z0-9_]*)$`)

// answers reports whether the server registers a route that a request for
// docPath would reach, and, when method is not empty, with that method. A
// placeholder segment reaches only a parameter segment; a literal segment
// reaches the same literal or a parameter segment, as Fiber matches it.
func answers(table []docRoute, method, docPath string) bool {
	want := routeSegments(docPath)
	for _, r := range table {
		if method != "" && r.method != method {
			continue
		}
		if segmentsMatch(r.segments, want) {
			return true
		}
	}
	return false
}

func segmentsMatch(have, want []string) bool {
	for i, h := range have {
		if h == "*" || h == "+" {
			return len(want) > i
		}
		if i >= len(want) {
			return false
		}
		w := want[i]
		param := strings.HasPrefix(h, ":")
		switch {
		case docPlaceholder.MatchString(w):
			if !param {
				return false
			}
		case h != w && !param:
			return false
		}
	}
	return len(have) == len(want)
}

// docPathChars is what a documented path is made of: path segments, and the
// placeholders readers substitute.
const docPathChars = `(?:[A-Za-z0-9_.~{}$*-]|:[A-Za-z_]+|<[A-Za-z0-9_-]+>|/)*`

var (
	// "POST /api/v1/agents", "#### GET /auth/me", "`DELETE` `/keys/{id}`".
	docMethodPath = regexp.MustCompile("\\b(GET|POST|PUT|PATCH|DELETE)\\b[`*\\s]*(/" + docPathChars + ")")
	// Any /api/ path, wherever it appears: a curl URL, a code sample, prose.
	docAPIPath = regexp.MustCompile(`/api/` + docPathChars)
	// A path on the server's own address that is not under /api/: /health,
	// /metrics, or a path the server does not have. A named host could be the
	// dashboard's, so only the backend's local port and URL variables count.
	docServerURL = regexp.MustCompile(`(?:localhost:8080|\$\{?(?:API_URL|AIM_URL|AIM_BASE_URL)\}?)(/` + docPathChars + `)`)
	// A repository file, not a route: docs/api/README.md, internal/api/x.go.
	docSourceFile = regexp.MustCompile(`\.(?:md|go|ts|tsx|js|py)$`)
	// A markdown table cell holding only a method: "| POST |".
	docMethodCell = regexp.MustCompile("\\|\\s*`?(GET|POST|PUT|PATCH|DELETE)`?\\s*\\|")
	docCurlMethod = regexp.MustCompile(`(?:-X|--request)\s+'?"?(GET|POST|PUT|PATCH|DELETE)\b`)
	docCurlData   = regexp.MustCompile(`(?:\s-d\s|\s--data(?:-raw|-binary)?[\s=]|\s-F\s|\s--form\s)`)
)

// cleanDocPath trims what follows a path in prose or a URL.
func cleanDocPath(p string) string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	p = strings.TrimRight(p, ".,;:")
	return strings.TrimSuffix(p, "/")
}

type docReference struct {
	file   string
	line   int
	method string
	path   string
}

// documentedRoutes returns every method and path a page names, with the line
// it is on. A curl command's continuation lines are read as one line, so its
// method applies to its URL.
func documentedRoutes(file, src string, rootPaths map[string]bool) []docReference {
	var refs []docReference
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		start := i
		line := lines[i]
		for strings.HasSuffix(strings.TrimSpace(line), "\\") && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(strings.TrimSpace(line), "\\") + " " + lines[i]
		}

		lineMethod := ""
		if m := docMethodCell.FindStringSubmatch(line); m != nil {
			lineMethod = m[1]
		}
		if strings.Contains(line, "curl ") {
			switch m := docCurlMethod.FindStringSubmatch(line); {
			case m != nil:
				lineMethod = m[1]
			case docCurlData.MatchString(line):
				lineMethod = http.MethodPost
			default:
				lineMethod = http.MethodGet
			}
		}

		covered := map[int]bool{}
		for _, m := range docMethodPath.FindAllStringSubmatchIndex(line, -1) {
			method, path := line[m[2]:m[3]], cleanDocPath(line[m[4]:m[5]])
			covered[m[4]] = true
			if !strings.HasPrefix(path, "/api/") && !rootPaths[path] && !strings.HasPrefix(path, "/.well-known/") {
				path = "/api/v1" + path
			}
			refs = append(refs, docReference{file, start + 1, method, path})
		}
		for _, m := range docAPIPath.FindAllStringIndex(line, -1) {
			path := cleanDocPath(line[m[0]:m[1]])
			// "/api/v1" alone names the prefix, not a route.
			if covered[m[0]] || path == "/api" || path == "/api/v1" || docSourceFile.MatchString(path) {
				continue
			}
			refs = append(refs, docReference{file, start + 1, lineMethod, path})
		}
		for _, m := range docServerURL.FindAllStringSubmatchIndex(line, -1) {
			path := cleanDocPath(line[m[2]:m[3]])
			if path == "" || strings.HasPrefix(path, "/api/") || covered[m[2]] {
				continue
			}
			refs = append(refs, docReference{file, start + 1, lineMethod, path})
		}
	}
	return refs
}

// TestEveryDocumentedAPIPathIsARegisteredRoute reads the API reference pages,
// tutorials and guides and fails on every method and path they name that the
// server does not register. The references had been written by hand and drifted
// from the route table: they named /api/v1/keys, /api/v1/trust-scores/{agentId},
// /api/v1/audit-logs and /debug/pprof/heap, none of which the server registers,
// so a reader following them had no route to call.
func TestEveryDocumentedAPIPathIsARegisteredRoute(t *testing.T) {
	requirePublicRepositoryFiles(t)

	table := serverRouteTable(t)
	rootPaths := map[string]bool{}
	for _, r := range table {
		if len(r.segments) > 1 && r.segments[1] != "api" {
			rootPaths[strings.Join(r.segments, "/")] = true
		}
	}

	total := 0
	for _, file := range apiReferenceDocs {
		refs := documentedRoutes(file, aim03ReadRepoFile(t, file), rootPaths)
		for _, ref := range refs {
			total++
			if !answers(table, ref.method, ref.path) {
				what := ref.path
				if ref.method != "" {
					what = ref.method + " " + ref.path
				}
				t.Errorf("%s:%d names %s, which the server does not register", ref.file, ref.line, what)
			}
		}
	}
	require.Greater(t, total, 100, "the pages name far more routes than the scan found; the scan is broken, not the pages")
	t.Logf("checked %d method and path references in %d pages", total, len(apiReferenceDocs))
}
