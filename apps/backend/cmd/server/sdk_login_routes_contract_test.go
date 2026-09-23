package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routeCensus composes "METHOD /full/path" for every route main.go registers on
// a Fiber group rooted at app, following x := parent.Group("...") chains.
func routeCensus(src string) map[string]bool {
	groupRe := regexp.MustCompile(`(?m)^\s*(\w+)\s*:?=\s*(\w+)\.Group\("([^"]*)"`)
	routeRe := regexp.MustCompile(`(?m)^\s*(\w+)\.(Get|Post|Put|Patch|Delete|All)\("([^"]*)"`)
	prefix := map[string]string{"app": ""}
	for _, m := range groupRe.FindAllStringSubmatch(src, -1) {
		parent, ok := prefix[m[2]]
		if !ok {
			continue
		}
		prefix[m[1]] = parent + m[3]
	}
	routes := map[string]bool{}
	for _, m := range routeRe.FindAllStringSubmatch(src, -1) {
		p, ok := prefix[m[1]]
		if !ok {
			continue
		}
		routes[strings.ToUpper(m[2])+" "+p+m[3]] = true
	}
	return routes
}

func routeServed(routes map[string]bool, path string) bool {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "ALL"} {
		if routes[m+" "+path] {
			return true
		}
	}
	return false
}

// TestSDKLoginRoutesContract pins the pair that lets `aim-sdk login --url`
// complete against a stack built from this repository: every URL the CLI
// builds on the server URL must be a route main.go registers, and the login
// must be the RFC 8628 device grant the backend serves. Measured 2026-09-22:
// the CLI posted its authorization code to /api/v1/auth/token and opened
// /auth/login with PKCE parameters; no backend in this repository registers
// the first, and the dashboard ignores the second, so no self-hosted stack
// completed the login.
func TestSDKLoginRoutesContract(t *testing.T) {
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	routes := routeCensus(mainSrc)
	require.True(t, routes["POST /api/v1/auth/login/local"], "census sanity: the local login route must be found")
	require.True(t, routes["POST /api/v1/auth/refresh"], "census sanity: the refresh route must be found")

	t.Run("backend serves the device grant", func(t *testing.T) {
		for _, r := range []string{
			"POST /api/v1/oauth/device/code",
			"POST /api/v1/oauth/device/token",
			"POST /api/v1/oauth/device/approve",
			"GET /api/v1/oauth/device/verify",
		} {
			assert.True(t, routes[r], "%s must be registered", r)
		}
	})

	cli := aim03ReadRepoFile(t, "sdk/python/aim_sdk/cli.py")
	t.Run("every aim_url-rooted URL in cli.py is a registered route", func(t *testing.T) {
		urlRe := regexp.MustCompile(`f"\{aim_url(?:\.rstrip\('/'\))?\}(/[^"?{]*)`)
		found := urlRe.FindAllStringSubmatch(cli, -1)
		require.NotEmpty(t, found, "cli.py must build at least one URL on aim_url")
		for _, m := range found {
			assert.True(t, routeServed(routes, m[1]),
				"cli.py builds %s on aim_url but main.go registers no such route", m[1])
		}
	})
	t.Run("the login is the device grant by name", func(t *testing.T) {
		assert.True(t, strings.Contains(cli, "/api/v1/oauth/device/code"), "login must request a device code")
		assert.True(t, strings.Contains(cli, "/api/v1/oauth/device/token"), "login must poll the device token route")
		assert.True(t, strings.Contains(cli, "urn:ietf:params:oauth:grant-type:device_code"), "RFC 8628 grant type")
		assert.False(t, strings.Contains(cli, "/api/v1/auth/token"), "the exchange route no backend registers must be gone")
		assert.False(t, strings.Contains(cli, "code_challenge"), "PKCE has no server side in this repository")
	})
}

// TestDeviceVerificationPageAndURI pins the two dashboard-side facts the device
// grant needs on a self-hosted stack: the verification URI the backend hands the
// CLI is rooted on the dashboard origin, and the dashboard serves the page
// before login so the user code on the URL survives.
func TestDeviceVerificationPageAndURI(t *testing.T) {
	t.Run("verification URI is rooted on the dashboard origin", func(t *testing.T) {
		mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
		ctor := regexp.MustCompile(`(?s)application\.NewDeviceAuthService\((.*?)\)\n`).FindStringSubmatch(mainSrc)
		require.Len(t, ctor, 2, "the NewDeviceAuthService call must be found")
		assert.True(t, strings.Contains(ctor[1], "cfg.Server.FrontendURL"),
			"the device verification URI must be built from FRONTEND_URL (the dashboard), not the API origin")
	})
	t.Run("/device is public in the edge middleware", func(t *testing.T) {
		mw := aim03ReadRepoFile(t, "apps/web/middleware.ts")
		pub := regexp.MustCompile(`const publicRoutes = \[([^\]]*)\]`).FindStringSubmatch(mw)
		require.Len(t, pub, 2, "middleware.ts publicRoutes must be found")
		assert.True(t, strings.Contains(pub[1], `'/device'`),
			"/device must render before login so the user code on the URL survives the redirect")
	})
	t.Run("the dashboard has a device approval page", func(t *testing.T) {
		page := aim03ReadRepoFile(t, "apps/web/app/device/page.tsx")
		assert.True(t, strings.Contains(page, "/api/v1/oauth/device/approve"), "the page must call the approve route")
	})
}
