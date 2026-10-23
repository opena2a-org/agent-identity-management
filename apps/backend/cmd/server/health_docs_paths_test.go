package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// servedHealthRoute captures each health path main.go mounts on the app.
	servedHealthRoute = regexp.MustCompile(`app\.Get\("(/health[^"]*)"`)
	docURL            = regexp.MustCompile("https?://[^\\s\"'`()<>\\[\\]]+")
	jsonFence         = regexp.MustCompile("(?s)```json\n(.*?)```")
	// summaryReadinessRow captures the description cell of the API endpoint
	// summary's GET /health/ready row.
	summaryReadinessRow = regexp.MustCompile("^\\| GET \\| `/health/ready` \\|([^|]*)\\|")
)

// docsWalkSkip names the directories that hold no documentation of this
// repository: dependencies, build output and version control.
var docsWalkSkip = map[string]bool{
	".git": true, "node_modules": true, ".next": true, ".turbo": true, "dist": true,
	"build": true, "target": true, "vendor": true, ".venv": true, "venv": true,
}

// isAIMOrigin reports whether a documented URL addresses the AIM server: the
// backend's port, or a deployment's domain, whose dashboard proxies /health and
// /health/* to the backend (apps/web/next.config.ts).
func isAIMOrigin(u *url.URL) bool {
	return u.Port() == "8080" || u.Hostname() == "your-domain.com" || u.Hostname() == "aim.oa2a.org"
}

// isHealthPath reports whether a path has a segment named health.
func isHealthPath(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if seg == "health" {
			return true
		}
	}
	return false
}

// TestDocumentedHealthURLsAreServed pins every health URL the repository's
// markdown gives for the AIM server to a path main.go mounts. The setup guide,
// the deployment checklist, the troubleshooting guide and the examples README
// told readers to check /api/v1/health, a path the server does not mount. A
// document that names a health path the server does not mount fails here.
func TestDocumentedHealthURLsAreServed(t *testing.T) {
	served := map[string]bool{}
	for _, m := range servedHealthRoute.FindAllStringSubmatch(aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go"), -1) {
		served[m[1]] = true
	}
	require.True(t, served["/health"], "main.go mounts no GET /health; the route scan is broken, not the documents")
	require.True(t, served["/health/ready"], "main.go mounts no GET /health/ready; the route scan is broken, not the documents")

	root := aim03RepoRoot(t)
	checked := 0
	var findings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && docsWalkSkip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		src, err := os.ReadFile(p) //nolint:gosec // G304: walking the repository's own documents
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		for i, line := range strings.Split(string(src), "\n") {
			for _, raw := range docURL.FindAllString(line, -1) {
				u, err := url.Parse(strings.TrimRight(raw, ".,;:"))
				if err != nil || !isAIMOrigin(u) || !isHealthPath(u.Path) {
					continue
				}
				checked++
				if !served[strings.TrimSuffix(u.Path, "/")] {
					findings = append(findings, fmt.Sprintf("%s:%d: %s", rel, i+1, raw))
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, checked, "no documented AIM health URL found; the document scan is broken, not the documents")

	paths := make([]string, 0, len(served))
	for p := range served {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	assert.Empty(t, findings, "each documented health URL must name a path the server mounts (%s):\n%s",
		strings.Join(paths, ", "), strings.Join(findings, "\n"))
}

// TestSetupGuideHealthOutputIsTheHandlers pins the response the setup guide
// shows for its health check to the fields the GET /health handler returns.
func TestSetupGuideHealthOutputIsTheHandlers(t *testing.T) {
	requirePublicRepositoryFiles(t)

	guide := aim03ReadRepoFile(t, "docs/guides/SETUP_GUIDE.md")
	start := strings.Index(guide, "### 1. Check Backend Health")
	require.NotEqual(t, -1, start, "the setup guide has no backend health step")
	step := guide[start:]
	if end := strings.Index(step[1:], "\n### "); end != -1 {
		step = step[:end+1]
	}
	assert.Contains(t, step, "curl http://localhost:8080/health\n", "the step checks the server's health path")

	block := jsonFence.FindStringSubmatch(step)
	require.NotNil(t, block, "the step shows no JSON response")
	var documented map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(block[1]), &documented), "the documented response must be JSON")

	handler := healthHandlerSrc.FindStringSubmatch(aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go"))
	require.NotNil(t, handler, "main.go mounts no GET /health handler returning a fiber.Map")
	var servedKeys []string
	for _, field := range healthHandlerField.FindAllStringSubmatch(handler[1], -1) {
		servedKeys = append(servedKeys, field[1])
	}
	documentedKeys := make([]string, 0, len(documented))
	for k := range documented {
		documentedKeys = append(documentedKeys, k)
	}
	sort.Strings(servedKeys)
	sort.Strings(documentedKeys)
	assert.Equal(t, servedKeys, documentedKeys, "the documented response has the fields the server returns")
	assert.Equal(t, "healthy", documented["status"])
	assert.Equal(t, "agent-identity-management", documented["service"])
}

// readinessDependencies returns each dependency GET /health/ready reports,
// with its "required" value, read from the handler's body with no optional
// dependency set up and with every one set up.
func readinessDependencies(t *testing.T) map[string]bool {
	t.Helper()
	required := map[string]bool{}
	for _, handler := range []fiber.Handler{
		newHealthReadyHandler(aim07CheckOK, nil, nil),
		newHealthReadyHandler(aim07CheckOK, aim07CheckOK, func() (bool, string) { return true, "" }),
	} {
		app := fiber.New()
		app.Get("/health/ready", handler)
		_, _, body := aim07Get(t, app)
		deps, ok := aim07TopLevel(t, body)["dependencies"].(map[string]any)
		require.True(t, ok, "dependencies must be an object: %s", body)
		for name, raw := range deps {
			entry, ok := raw.(map[string]any)
			require.True(t, ok, "dependency %q must be an object: %s", name, body)
			flag, ok := entry["required"].(bool)
			require.True(t, ok, "dependency %q must carry a boolean required: %s", name, body)
			required[name] = flag
		}
	}
	return required
}

// TestEndpointSummaryReadinessRowNamesTheReportedDependencies pins the API
// endpoint summary's GET /health/ready row to the dependencies the readiness
// handler reports: the row names each one, and writes "`<name>` (required"
// for each one the body reports as required and for no other. The row read
// "Readiness check (database + redis)" while the body also reported siem and
// revocationState.
func TestEndpointSummaryReadinessRowNamesTheReportedDependencies(t *testing.T) {
	requirePublicRepositoryFiles(t)

	const summary = "docs/guides/API_ENDPOINT_SUMMARY.md"
	var rows []string
	for _, line := range strings.Split(aim03ReadRepoFile(t, summary), "\n") {
		if m := summaryReadinessRow.FindStringSubmatch(line); m != nil {
			rows = append(rows, m[1])
		}
	}
	require.Len(t, rows, 1, "%s lists GET /health/ready in one table row", summary)
	row := rows[0]

	deps := readinessDependencies(t)
	require.Contains(t, deps, "database", "the handler reported no database; the body read is broken, not the summary")
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		assert.Regexp(t, `\b`+regexp.QuoteMeta(name)+`\b`, row,
			"the %s row names the %s dependency the body reports", summary, name)
		assert.Equal(t, deps[name], strings.Contains(row, "`"+name+"` (required"),
			"the body reports %s with \"required\": %t; the row writes \"`%s` (required\" exactly when it is required",
			name, deps[name], name)
	}
}

// TestEndpointSummaryReadinessRowNamesTheBodysKeys pins the API endpoint
// summary's GET /health/ready row to the top-level keys of the readiness body:
// the row names, in backquotes, each key the body carries in a build stamped
// with syncedFrom, run in a Container Apps revision.
func TestEndpointSummaryReadinessRowNamesTheBodysKeys(t *testing.T) {
	requirePublicRepositoryFiles(t)

	const summary = "docs/guides/API_ENDPOINT_SUMMARY.md"
	var rows []string
	for _, line := range strings.Split(aim03ReadRepoFile(t, summary), "\n") {
		if m := summaryReadinessRow.FindStringSubmatch(line); m != nil {
			rows = append(rows, m[1])
		}
	}
	require.Len(t, rows, 1, "%s lists GET /health/ready in one table row", summary)

	prev := buildSyncedFrom
	t.Cleanup(func() { buildSyncedFrom = prev })
	buildSyncedFrom = syncedFromTestCommit
	setServingRevision(t, revisionTestName)
	_, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))

	var keys []string
	for key := range aim07TopLevel(t, body) {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	require.Contains(t, keys, "revision", "the body carries no revision; the body read is broken, not the summary")
	require.Contains(t, keys, "syncedFrom", "the body carries no syncedFrom; the body read is broken, not the summary")
	for _, key := range keys {
		assert.Contains(t, rows[0], "`"+key+"`", "the %s row names the %s key the body carries", summary, key)
	}
}
