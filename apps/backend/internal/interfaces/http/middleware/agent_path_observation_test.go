package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const observationTestRoute = "GET /agents/:id/trust-score"

// observedRecord is what a held route leaves for the request record, read back
// by the test handler.
type observedRecord struct {
	route   any
	outcome any
}

// observationTestApp mounts gate on the route, after a stand-in for the
// group's authenticator, with a handler that reports what the gate recorded.
func observationTestApp(principal any, authMethod string, gate fiber.Handler) (*fiber.App, *observedRecord) {
	seen := &observedRecord{}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if principal != nil {
			c.Locals("agent_id", principal)
			c.Locals("auth_method", authMethod)
		}
		return c.Next()
	})
	app.Get("/agents/:id/trust-score", gate, func(c fiber.Ctx) error {
		seen.route = c.Locals(agentPathRouteLocal)
		seen.outcome = c.Locals(agentPathOutcomeLocal)
		return c.SendStatus(http.StatusTeapot)
	})
	return app, seen
}

func observe(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// A held route serves every caller; what it records is exactly what the
// binding would have decided.
func TestAgentPathObservation_ServesEveryCallerAndRecordsTheComparison(t *testing.T) {
	self, sibling, unknown := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name      string
		principal any
		path      string
		want      string
	}{
		{"own id", self, "/agents/" + self.String() + "/trust-score", AgentPathOutcomeSelf},
		{"sibling id", self, "/agents/" + sibling.String() + "/trust-score", AgentPathOutcomeOther},
		{"unknown id", self, "/agents/" + unknown.String() + "/trust-score", AgentPathOutcomeOther},
		{"not a uuid", self, "/agents/some-name/trust-score", AgentPathOutcomeOther},
		{"user caller", nil, "/agents/" + sibling.String() + "/trust-score", AgentPathOutcomeNoPrincipal},
		{"unreadable principal", self.String(), "/agents/" + sibling.String() + "/trust-score", AgentPathOutcomeFault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, seen := observationTestApp(tc.principal, "ed25519", AgentPathObservation(observationTestRoute, "id"))
			assert.Equal(t, http.StatusTeapot, observe(t, app, tc.path), "a held route must reach the handler")
			assert.Equal(t, observationTestRoute, seen.route)
			assert.Equal(t, tc.want, seen.outcome)
		})
	}
}

// Whatever the binding refuses, the observation records as other, for every
// agent authenticator.
func TestAgentPathObservation_RecordsWhatTheBindingRefuses(t *testing.T) {
	for _, method := range []string{"ed25519", "mldsa", "hybrid", "api_key", "atc", "service"} {
		t.Run(method, func(t *testing.T) {
			self, sibling := uuid.New(), uuid.New()
			path := "/agents/" + sibling.String() + "/trust-score"

			bound, boundSeen := observationTestApp(self, method, AgentPathBinding(observationTestRoute, "id", nil))
			assert.Equal(t, http.StatusForbidden, observe(t, bound, path))
			assert.Nil(t, boundSeen.outcome, "the bound route refused before the handler")

			held, heldSeen := observationTestApp(self, method, AgentPathObservation(observationTestRoute, "id"))
			assert.Equal(t, http.StatusTeapot, observe(t, held, path))
			assert.Equal(t, AgentPathOutcomeOther, heldSeen.outcome)
		})
	}
}

// Mounted with Use(), the parameter reads empty: every request is recorded as
// a fault, never as self or other, so a mis-mount cannot pass as a zero count.
func TestAgentPathObservation_UseMountRecordsAFault(t *testing.T) {
	self := uuid.New()
	var outcome any
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("agent_id", self)
		return c.Next()
	})
	group := app.Group("/agents")
	group.Use(AgentPathObservation(observationTestRoute, "id"))
	group.Get("/:id/trust-score", func(c fiber.Ctx) error {
		outcome = c.Locals(agentPathOutcomeLocal)
		return c.SendStatus(http.StatusTeapot)
	})

	assert.Equal(t, http.StatusTeapot, observe(t, app, "/agents/"+uuid.New().String()+"/trust-score"))
	assert.Equal(t, AgentPathOutcomeFault, outcome)
}

func TestAgentPathObservation_RequiresRouteAndParam(t *testing.T) {
	assert.Panics(t, func() { AgentPathObservation("", "id") })
	assert.Panics(t, func() { AgentPathObservation(observationTestRoute, "") })
}

// The request record carries how the caller authenticated and, on a bound or
// held route, the route and how its agent parameter compared with the caller:
// the fields the per-route count that releases a hold is read from.
func TestAnalyticsTracking_RecordsAuthMethodAndAgentPath(t *testing.T) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var got []APICallLog
	saved := recordAPICall
	recordAPICall = func(_ *sql.DB, log APICallLog) {
		defer wg.Done()
		mu.Lock()
		got = append(got, log)
		mu.Unlock()
	}
	defer func() { recordAPICall = saved }()

	self, sibling := uuid.New(), uuid.New()
	app := fiber.New()
	app.Use(AnalyticsTracking(nil))
	app.Use(func(c fiber.Ctx) error {
		if c.Get("X-Test-Agent") != "" {
			c.Locals("agent_id", self)
			c.Locals("auth_method", c.Get("X-Test-Agent"))
		}
		return c.Next()
	})
	app.Get("/agents/:id/trust-score", AgentPathObservation(observationTestRoute, "id"),
		func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/agents/:id/heartbeat", AgentPathBinding("POST /agents/:id/heartbeat", "id", nil),
		func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Get("/agents", func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	send := func(method, path, agentAuth string) {
		req := httptest.NewRequest(method, path, nil)
		if agentAuth != "" {
			req.Header.Set("X-Test-Agent", agentAuth)
		}
		wg.Add(1)
		resp, err := app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}
	send(http.MethodGet, "/agents/"+sibling.String()+"/trust-score", "api_key")
	send(http.MethodGet, "/agents/"+self.String()+"/trust-score", "ed25519")
	send(http.MethodGet, "/agents/"+sibling.String()+"/trust-score", "")
	send(http.MethodPost, "/agents/"+sibling.String()+"/heartbeat", "service")
	send(http.MethodGet, "/agents", "")
	wg.Wait()

	byKey := map[string]APICallLog{}
	for _, log := range got {
		key := log.Method + " " + log.Endpoint
		if log.AuthMethod != nil {
			key += " " + *log.AuthMethod
		}
		byKey[key] = log
	}
	require.Len(t, byKey, 5)

	str := func(v string) *string { return &v }
	for key, want := range map[string]struct {
		status               int
		auth, route, outcome *string
		agent                bool
	}{
		"GET /agents/" + sibling.String() + "/trust-score api_key": {http.StatusOK, str("api_key"), str(observationTestRoute), str(AgentPathOutcomeOther), true},
		"GET /agents/" + self.String() + "/trust-score ed25519":    {http.StatusOK, str("ed25519"), str(observationTestRoute), str(AgentPathOutcomeSelf), true},
		"GET /agents/" + sibling.String() + "/trust-score":         {http.StatusOK, nil, str(observationTestRoute), str(AgentPathOutcomeNoPrincipal), false},
		"POST /agents/" + sibling.String() + "/heartbeat service":  {http.StatusForbidden, str("service"), str("POST /agents/:id/heartbeat"), str(AgentPathOutcomeOther), true},
		"GET /agents": {http.StatusOK, nil, nil, nil, false},
	} {
		log, ok := byKey[key]
		require.True(t, ok, "no record for %s", key)
		assert.Equal(t, want.status, log.StatusCode, key)
		assert.Equal(t, want.auth, log.AuthMethod, key)
		assert.Equal(t, want.route, log.AgentPathRoute, key)
		assert.Equal(t, want.outcome, log.AgentPathOutcome, key)
		if want.agent {
			require.NotNil(t, log.AgentID, key)
			assert.Equal(t, self, *log.AgentID, key)
		} else {
			assert.Nil(t, log.AgentID, key)
		}
	}
}

// The insert writes the three fields to their columns.
func TestLogAPICall_WritesAuthMethodAndAgentPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	orgID, agentID := uuid.New(), uuid.New()
	authMethod, route, outcome := "api_key", "GET /api/v1/agents/:id", AgentPathOutcomeOther
	mock.ExpectExec(regexp.QuoteMeta("error_message, auth_method, agent_path_route, agent_path_outcome, called_at")).
		WithArgs(&orgID, &agentID, nil, "GET", "/api/v1/agents/x", 200, 3, 0, 2, "sdk/1.0", "10.0.0.1", nil,
			&authMethod, &route, &outcome).
		WillReturnResult(sqlmock.NewResult(0, 1))

	logAPICall(db, APICallLog{
		OrganizationID: &orgID, AgentID: &agentID, Method: "GET", Endpoint: "/api/v1/agents/x",
		StatusCode: 200, DurationMs: 3, ResponseSizeBytes: 2, UserAgent: "sdk/1.0", IPAddress: "10.0.0.1",
		AuthMethod: &authMethod, AgentPathRoute: &route, AgentPathOutcome: &outcome,
	})
	require.NoError(t, mock.ExpectationsWereMet())
}
