package main

import (
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

// providerSignInDocPatterns match text that gives a provider login or callback
// route, a provider setting, or sign-in with Google, Microsoft, Okta or SSO.
var providerSignInDocPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)/auth/(login|callback)/(google|microsoft|okta|:provider)\b`),
	regexp.MustCompile(`(?i)/o?auth/(google|microsoft|okta)\b`),
	regexp.MustCompile(`(?i)/auth/callback\b`),
	regexp.MustCompile(`\b(GOOGLE|MICROSOFT|OKTA)_(CLIENT_ID|CLIENT_SECRET|REDIRECT_URL|TENANT_ID|DOMAIN)\b`),
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

// providerSignInDocSkipDirs are directories that hold no documentation of this
// repository: dependencies, build output and local tool state.
var providerSignInDocSkipDirs = map[string]bool{
	".git": true, "node_modules": true, ".next": true, "dist": true, "build": true,
	"coverage": true, ".turbo": true, "vendor": true, ".venv": true, "venv": true,
	".claude": true,
}

// No guide, README or environment template tells a reader to configure or use
// provider sign-in. A changelog records what changed and is not read as a guide.
func TestDocsDescribeNoProviderSignIn(t *testing.T) {
	root := aim03RepoRoot(t)

	var findings []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && providerSignInDocSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name == "CHANGELOG.md" || name == "CLAUDE.md" {
			return nil
		}
		if !strings.HasSuffix(name, ".md") && name != ".env.example" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		findings = append(findings, providerSignInDocFindings(filepath.ToSlash(rel), string(body))...)
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, findings,
		"no route serves provider sign-in; describe POST /api/v1/auth/login/local instead:\n%s",
		strings.Join(findings, "\n"))
}

// The API guides that listed the provider routes list the email and password
// login, the sign-in route the server mounts.
func TestAPIGuidesNameTheLocalLogin(t *testing.T) {
	for _, rel := range []string{
		"docs/guides/API_REFERENCE.md",
		"docs/guides/API_ENDPOINT_SUMMARY.md",
	} {
		assert.Contains(t, aim03ReadRepoFile(t, rel), "/api/v1/auth/login/local", rel)
	}
}
