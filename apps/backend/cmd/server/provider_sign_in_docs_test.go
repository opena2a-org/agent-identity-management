package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The config reads GOOGLE_*, MICROSOFT_* and OKTA_* settings, but the server
// mounts no route that signs a user in with one of those providers: users sign
// in with an email and password, the dashboard through POST /api/v1/public/login
// and API clients through POST /api/v1/auth/login/local. These tests hold the
// route table to that, and the documentation to the route table. Building
// provider sign-in means replacing both.

// providerSignInPaths are the login and callback paths the guides gave for
// each provider.
var providerSignInPaths = []string{
	"/api/v1/auth/login/google",
	"/api/v1/auth/login/microsoft",
	"/api/v1/auth/login/okta",
	"/api/v1/auth/callback/google",
	"/api/v1/auth/callback/microsoft",
	"/api/v1/auth/callback/okta",
}

// A request with no credentials to one of these paths is answered 401 by the
// authentication the /auth group applies for /auth/me, so each path is probed
// signed in: what remains is the route table's answer.
func TestProviderSignInRoutesAreNotMounted(t *testing.T) {
	app, jwtService := strictCensusApp(t)
	token, err := jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), "census@example.com", "admin")
	require.NoError(t, err)

	for i, path := range providerSignInPaths {
		req := httptest.NewRequest(fiber.MethodGet, path, nil)
		req.Header.Set("X-Real-IP", fmt.Sprintf("198.51.100.%d", i+1))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err, path)
		_ = resp.Body.Close()
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode, "GET %s", path)
	}

	var signIn []string
	for _, r := range app.GetRoutes(true) {
		if strings.HasPrefix(r.Path, "/api/v1/auth/login/") || strings.HasPrefix(r.Path, "/api/v1/auth/callback") {
			signIn = append(signIn, r.Method+" "+r.Path)
		}
	}
	assert.Equal(t, []string{"POST /api/v1/auth/login/local"}, signIn,
		"the only login or callback route under /auth is the email and password login")
}

// providerSettingPattern matches a provider sign-in setting by name.
var providerSettingPattern = regexp.MustCompile(`\b(GOOGLE|MICROSOFT|OKTA)_(CLIENT_ID|CLIENT_SECRET|REDIRECT_URL|TENANT_ID|DOMAIN)\b`)

// providerSignInDocPatterns match text that gives a provider login or callback
// route, a provider setting, or sign-in with Google, Microsoft, Okta or SSO.
var providerSignInDocPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)/auth/(login|callback)/(google|microsoft|okta|:provider)\b`),
	regexp.MustCompile(`(?i)/o?auth/(google|microsoft|okta)\b`),
	regexp.MustCompile(`(?i)/auth/callback\b`),
	providerSettingPattern,
	regexp.MustCompile(`\bOAUTH_REDIRECT_URL\b`),
	regexp.MustCompile(`(?i)\b(sign|log)\w*[ -]?(in|on)\s+(with|via|using|through)\s+(an? )?(Google|Microsoft|Okta|OAuth|SSO)\b`),
	regexp.MustCompile(`(?i)\b(Google|Microsoft|Okta)\s+(OAuth|SSO)\b`),
	regexp.MustCompile(`\bSSO\b`),
}

// providerSignInDocFindings returns file:line for every line of text that
// matches a provider sign-in pattern. A line that names aim-cloud describes the
// hosted service, which is built from that private mirror, not this server.
func providerSignInDocFindings(name, text string) []string {
	var findings []string
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "aim-cloud") {
			continue
		}
		for _, re := range providerSignInDocPatterns {
			if re.MatchString(line) {
				findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
				break
			}
		}
	}
	return findings
}

func TestProviderSignInDocFindings(t *testing.T) {
	sample := strings.Join([]string{
		"curl http://localhost:8080/api/v1/auth/login/google",
		"| GET | `/api/v1/auth/login/:provider` | OAuth2 login | None |",
		"- Google (`/auth/google`)",
		"OAUTH_REDIRECT_URL=http://localhost:3000/auth/callback",
		"GOOGLE_CLIENT_ID=your_google_client_id",
		"# OKTA_DOMAIN=",
		`You should see the landing page with "Sign in with Google" button.`,
		"- ✅ Log in via Google OAuth",
		"- **OAuth/OIDC Support**: SSO integration",
		"### Microsoft OAuth Setup (Optional)",
		"1. User logs in via OAuth (Google/Microsoft)",
		"GET /api/v1/oauth/google/login",
		"The hosted service deploys from opena2a-org/aim-cloud, which adds Google OAuth.",
		"POST /api/v1/auth/login/local signs a user in with an email and password.",
		"POST /api/v1/oauth/token issues an access token for a jwt-bearer grant.",
		"See GET /api/v1/oauth/device/verify.",
	}, "\n")
	assert.Equal(t, []string{
		"sample.md:1: curl http://localhost:8080/api/v1/auth/login/google",
		"sample.md:2: | GET | `/api/v1/auth/login/:provider` | OAuth2 login | None |",
		"sample.md:3: - Google (`/auth/google`)",
		"sample.md:4: OAUTH_REDIRECT_URL=http://localhost:3000/auth/callback",
		"sample.md:5: GOOGLE_CLIENT_ID=your_google_client_id",
		"sample.md:6: # OKTA_DOMAIN=",
		`sample.md:7: You should see the landing page with "Sign in with Google" button.`,
		"sample.md:8: - ✅ Log in via Google OAuth",
		"sample.md:9: - **OAuth/OIDC Support**: SSO integration",
		"sample.md:10: ### Microsoft OAuth Setup (Optional)",
		"sample.md:11: 1. User logs in via OAuth (Google/Microsoft)",
		"sample.md:12: GET /api/v1/oauth/google/login",
	}, providerSignInDocFindings("sample.md", sample))
}

// providerSignInDocFiles returns, relative to root and in git's order, every
// tracked Markdown file and environment template: the files a reader of the
// repository is given. The list is read from the index, so dependencies, build
// output and any other untracked or ignored file are left out without being
// named. A changelog records what changed and is not read as a guide.
func providerSignInDocFiles(t *testing.T, root string) []string {
	t.Helper()
	requireGit(t)
	out, err := gitIn(root, "ls-files", "-z")
	require.NoError(t, err, "git ls-files in %s: this check reads the tracked file list and cannot run on an export", root)

	var files []string
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		name := rel[strings.LastIndex(rel, "/")+1:]
		if name == "CHANGELOG.md" {
			continue
		}
		if strings.HasSuffix(name, ".md") || name == ".env.example" {
			files = append(files, rel)
		}
	}
	return files
}

// scratchRepositoryDocFiles builds a git repository that holds tracked,
// untracked and ignored files, and returns the doc files read from it.
func scratchRepositoryDocFiles(t *testing.T) []string {
	t.Helper()
	requireGit(t)
	top := t.TempDir()
	noHooks := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		base := []string{"-c", "core.hooksPath=" + noHooks}
		if _, err := gitIn(top, append(base, args...)...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	git("init", "--quiet")

	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(top, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	tracked := []string{".env.example", ".gitignore", "CHANGELOG.md", "README.md", "docs/guide.md", "notes.txt"}
	write(".gitignore", "build/\n")
	for _, rel := range tracked {
		if rel != ".gitignore" {
			write(rel, "tracked "+rel+"\n")
		}
	}
	// Staged with -f: these files are tracked whatever this machine's own git
	// ignore rules name (a global excludes file, for one).
	git(append([]string{"add", "-f", "--"}, tracked...)...)

	// Untracked, in a dotted directory, and ignored: none is a guide.
	write("LOCAL.md", "untracked\n")
	write(".tool/NOTES.md", "untracked\n")
	write("build/out.md", "ignored\n")
	write("build/.env.example", "ignored\n")

	return providerSignInDocFiles(t, top)
}

// The doc files are the tracked guides and templates: a file that is untracked
// or ignored is not read, whatever its name or directory, and a tracked file
// that is neither Markdown nor an environment template is not read either.
func TestProviderSignInDocFilesAreTheTrackedGuides(t *testing.T) {
	assert.Equal(t, []string{".env.example", "README.md", "docs/guide.md"}, scratchRepositoryDocFiles(t))
}

// The scratch repository does not depend on the developer's git configuration:
// a global excludes file that names every tracked fixture changes nothing.
func TestProviderSignInDocFilesWithAGlobalExcludesFile(t *testing.T) {
	requireGit(t)
	home := t.TempDir()
	excludes := filepath.Join(home, "excludes")
	require.NoError(t, os.WriteFile(excludes, []byte(".env.example\n.gitignore\n*.md\nnotes.txt\n"), 0o600))
	globalConfig := filepath.Join(home, "gitconfig")
	_, err := gitIn(home, "config", "--file", globalConfig, "core.excludesFile", excludes)
	require.NoError(t, err)
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)

	inEffect, err := gitIn(home, "config", "--global", "--get", "core.excludesFile")
	require.NoError(t, err)
	require.Equal(t, excludes, strings.TrimSpace(inEffect), "git reads the global configuration this test wrote")

	assert.Equal(t, []string{".env.example", "README.md", "docs/guide.md"}, scratchRepositoryDocFiles(t))
}

// No guide, README or environment template tells a reader to configure or use
// provider sign-in.
func TestDocsDescribeNoProviderSignIn(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)

	var findings []string
	for _, rel := range providerSignInDocFiles(t, root) {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			// Tracked but deleted from the working tree: there is no text to read.
			continue
		}
		require.NoError(t, err, rel)
		findings = append(findings, providerSignInDocFindings(rel, string(body))...)
	}
	assert.Empty(t, findings,
		"no route serves provider sign-in; describe POST /api/v1/auth/login/local instead:\n%s",
		strings.Join(findings, "\n"))
}

// The API guides that listed the provider routes list the email and password
// login, the sign-in route the server mounts.
func TestAPIGuidesNameTheLocalLogin(t *testing.T) {
	requirePublicRepositoryFiles(t)

	for _, rel := range []string{
		"docs/guides/API_REFERENCE.md",
		"docs/guides/API_ENDPOINT_SUMMARY.md",
	} {
		assert.Contains(t, aim03ReadRepoFile(t, rel), "/api/v1/auth/login/local", rel)
	}
}

// With no provider sign-in route, a flag or check the server derives from a
// provider setting describes nothing it does: GET /api/v1/status reported an
// "oauth" feature computed from GOOGLE_CLIENT_ID. No source file of this
// package reads a provider setting; the config package still reads them.
func TestServerPackageReadsNoProviderSetting(t *testing.T) {
	requirePublicRepositoryFiles(t)

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var findings []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		require.NoError(t, err, name)
		for i, line := range strings.Split(string(body), "\n") {
			if providerSettingPattern.MatchString(line) {
				findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	assert.Empty(t, findings,
		"no route serves provider sign-in, so nothing in the server package reads a provider setting:\n%s",
		strings.Join(findings, "\n"))
}
