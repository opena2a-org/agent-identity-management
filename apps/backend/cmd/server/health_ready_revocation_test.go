package main

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// GET /health/ready carries dependencies.revocationState: notConfigured
// without REGISTRY_URL, ok while the revocation list held is fit to answer
// from, and unavailable with one reason word otherwise. The entry is not
// required: unavailable sets degraded, and the database alone decides the
// HTTP status.

func revocationReadyApp(checkDB func(context.Context) error, checkRevocation func() (bool, string)) *fiber.App {
	app := fiber.New()
	app.Get("/health/ready", newHealthReadyHandler(checkDB, nil, nil, checkRevocation))
	return app
}

func revocationStateEntry(t *testing.T, body []byte) map[string]any {
	t.Helper()
	deps, ok := aim07TopLevel(t, body)["dependencies"].(map[string]any)
	require.True(t, ok, "dependencies must be an object")
	entry, ok := deps["revocationState"].(map[string]any)
	require.True(t, ok, "dependencies carries no revocationState: %s", body)
	return entry
}

func TestHealthReadyRevocationStateByCheck(t *testing.T) {
	schema := aim07LoadSchema(t)
	for _, tc := range []struct {
		name     string
		check    func() (bool, string)
		entry    map[string]any
		degraded bool
	}{
		{"not configured", nil, map[string]any{"status": "notConfigured", "required": false}, false},
		{"a list fit to answer from", func() (bool, string) { return true, "" }, map[string]any{"status": "ok", "required": false}, false},
		{
			"no list held yet",
			func() (bool, string) { return false, "noList" },
			map[string]any{"status": "unavailable", "required": false, "reason": "noList"},
			true,
		},
		{
			"a list answered 503",
			func() (bool, string) { return false, "status" },
			map[string]any{"status": "unavailable", "required": false, "reason": "status"},
			true,
		},
		{
			"a refresher that stopped refreshing",
			func() (bool, string) { return false, "refresherStalled" },
			map[string]any{"status": "unavailable", "required": false, "reason": "refresherStalled"},
			true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, _, body := aim07Get(t, revocationReadyApp(aim07CheckOK, tc.check))

			require.Equal(t, http.StatusOK, status, "the revocation state is not required")
			aim07Validate(t, schema, body)
			assert.Equal(t, tc.entry, revocationStateEntry(t, body))
			assert.Equal(t, true, aim07TopLevel(t, body)["ready"])
			assert.Equal(t, tc.degraded, aim07TopLevel(t, body)["degraded"])
		})
	}

	t.Run("a database that is down", func(t *testing.T) {
		down := func(context.Context) error { return errors.New("ping failed") }
		for _, check := range []func() (bool, string){nil, func() (bool, string) { return true, "" }} {
			status, _, body := aim07Get(t, revocationReadyApp(down, check))
			assert.Equal(t, http.StatusServiceUnavailable, status, "a list fit to answer from does not make the server ready")
			aim07Validate(t, schema, body)
			assert.Equal(t, false, aim07TopLevel(t, body)["ready"])
		}
	})
}

// requireRevocationEntry reads app's GET /health/ready until its
// revocationState entry is want, for up to 10 s.
func requireRevocationEntry(t *testing.T, app *fiber.App, want map[string]any) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, _, body := aim07Get(t, app)
		got := revocationStateEntry(t, body)
		if assert.ObjectsAreEqual(want, got) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("10 s after the start the revocationState entry is %v, want %v", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// With REGISTRY_URL set, the entry reads the refresher the server starts: ok
// once the list the authority's discovery document names is held, and
// unavailable with the reason status while the authority answers 503.
// Without REGISTRY_URL there is no refresher and no check.
func TestHealthReadyRevocationStateReadsTheRefresherOfRegistryURL(t *testing.T) {
	assert.Nil(t, revocationReadyCheck(nil), "no refresher, no check")
	captureLockedLog(t)

	t.Run("an authority that serves its signed list", func(t *testing.T) {
		a := newRevocationListAuthority(t, "did:web:agents.example:revoked-agent")
		refresher, err := startRevocationList(config.RegistryConfig{URL: a.srv.URL}, a.srv.Client())
		require.NoError(t, err)
		t.Cleanup(refresher.Stop)

		requireRevocationEntry(t, revocationReadyApp(aim07CheckOK, revocationReadyCheck(refresher)),
			map[string]any{"status": "ok", "required": false})
	})

	t.Run("an authority that answers 503", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		t.Cleanup(srv.Close)
		refresher, err := startRevocationList(config.RegistryConfig{URL: srv.URL}, srv.Client())
		require.NoError(t, err)
		t.Cleanup(refresher.Stop)

		requireRevocationEntry(t, revocationReadyApp(aim07CheckOK, revocationReadyCheck(refresher)),
			map[string]any{"status": "unavailable", "required": false, "reason": "status"})
	})
}

// main hands the readiness route the state of the refresher it starts, and
// starts the refresher before it registers the route.
func TestMain_HandsTheReadinessRouteTheRevocationListState(t *testing.T) {
	mainFunc := funcDecl(t, "main.go", "main")

	var started token.Pos
	ast.Inspect(mainFunc.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || identName(assign.Lhs[0]) != "revocationList" || len(assign.Rhs) != 1 {
			return true
		}
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok && identName(call.Fun) == "startRevocationList" {
			started = assign.Pos()
		}
		return true
	})
	require.True(t, started.IsValid(), "main assigns revocationList from startRevocationList")

	handlers := callsIn(mainFunc.Body, "newHealthReadyHandler")
	require.Len(t, handlers, 1)
	require.Len(t, handlers[0].Args, 4)
	check, ok := handlers[0].Args[3].(*ast.CallExpr)
	require.True(t, ok, "the fourth argument of newHealthReadyHandler is a call")
	assert.Equal(t, "revocationReadyCheck", identName(check.Fun))
	require.Len(t, check.Args, 1)
	assert.Equal(t, "revocationList", identName(check.Args[0]))
	assert.Less(t, started, handlers[0].Pos(), "the refresher is started before the readiness route is registered")
}
