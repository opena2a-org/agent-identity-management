package middleware

import (
	"io"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// The signed-request middlewares count each request they authenticate in
// aim_agent_requests_authenticated_total, under the scheme the request was signed in
// (type) and the calling SDK. A request a middleware refuses, or hands to the next
// handler without authenticating it, moves no series of that family.
//
// The series are read the way an operator reads them, from the /metrics exposition,
// and every cell compares the whole family before and after one request.

var authenticatedSeriesLine = regexp.MustCompile(`^aim_agent_requests_authenticated_total\{sdk="([^"]*)",type="([^"]*)"\} (\S+)$`)

// scrapeAuthenticated returns each aim_agent_requests_authenticated_total series as
// "type.sdk".
func scrapeAuthenticated(t *testing.T) map[string]float64 {
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
		if !strings.HasPrefix(line, "aim_agent_requests_authenticated_total{") {
			continue
		}
		m := authenticatedSeriesLine.FindStringSubmatch(line)
		require.NotNil(t, m, "unexpected series line %q: the labels are sdk and type, nothing else", line)
		value, err := strconv.ParseFloat(m[3], 64)
		require.NoError(t, err)
		series[m[2]+"."+m[1]] = value
	}
	return series
}

// authenticatedMoved lists the series that moved between two scrapes, and by how much.
func authenticatedMoved(before, after map[string]float64) map[string]float64 {
	moved := map[string]float64{}
	for series, value := range after {
		if delta := value - before[series]; delta != 0 {
			moved[series] = delta
		}
	}
	return moved
}

func TestLegacySignedRequestMiddlewares_CountEachAuthenticatedRequestAsAgentRequestV1(t *testing.T) {
	f := newS1Fixture(t)
	hybridAgent := func(_ *s1Fixture, _ map[string]string, a *domain.Agent) { a.HybridModeEnabled = true }

	cases := []struct {
		name string
		tc   s1Case
	}{
		{"PQCAgentMiddleware, Ed25519", s1Case{fn: s1FnPQCMiddleware}},
		{"PQCAgentMiddleware, ML-DSA-65", s1Case{fn: s1FnPQCMiddleware, request: (*s1Fixture).mldsaRequest}},
		{"PQCAgentMiddleware, hybrid", s1Case{fn: s1FnPQCMiddleware, request: (*s1Fixture).hybridRequest, change: hybridAgent}},
		{"Ed25519AgentMiddleware", s1Case{fn: s1FnEd25519Middleware}},
	}
	for _, tc := range cases {
		for _, ua := range s1UserAgents {
			t.Run(tc.name+", "+ua.sdk, func(t *testing.T) {
				before := scrapeAuthenticated(t)
				status, message, _ := driveS1(t, f, tc.tc, ua.header)
				require.Equal(t, fiber.StatusOK, status, message)
				assert.Equal(t, map[string]float64{"agent-request-v1." + ua.sdk: 1},
					authenticatedMoved(before, scrapeAuthenticated(t)))
			})
		}
	}
}

func TestLegacySignedRequestMiddlewares_CountNoRequestTheyRefuseOrPassOn(t *testing.T) {
	f := newS1Fixture(t)
	stale := func(_ *s1Fixture, h map[string]string, _ *domain.Agent) {
		h["X-Timestamp"] = strconv.FormatInt(time.Now().Unix()-120, 10)
	}
	suspended := func(_ *s1Fixture, _ map[string]string, a *domain.Agent) { a.Status = domain.AgentStatusSuspended }
	edOnlyForHybridAgent := func(_ *s1Fixture, _ map[string]string, a *domain.Agent) { a.HybridModeEnabled = true }
	bearer := func(_ *s1Fixture, h map[string]string, _ *domain.Agent) {
		h["Authorization"] = "Bearer not-a-real-token"
	}

	cases := []struct {
		name   string
		tc     s1Case
		status int
	}{
		{"PQCAgentMiddleware, stale timestamp", s1Case{fn: s1FnPQCMiddleware, change: stale}, fiber.StatusUnauthorized},
		{"PQCAgentMiddleware, suspended agent", s1Case{fn: s1FnPQCMiddleware, change: suspended}, fiber.StatusUnauthorized},
		{"PQCAgentMiddleware, hybrid agent signing with Ed25519 alone", s1Case{fn: s1FnPQCMiddleware, change: edOnlyForHybridAgent}, fiber.StatusUnauthorized},
		{"PQCAgentMiddleware, unknown agent", s1Case{fn: s1FnPQCMiddleware, noAgent: true}, fiber.StatusUnauthorized},
		{"Ed25519AgentMiddleware, stale timestamp", s1Case{fn: s1FnEd25519Middleware, change: stale}, fiber.StatusUnauthorized},
		{"Ed25519AgentMiddleware, suspended agent", s1Case{fn: s1FnEd25519Middleware, change: suspended}, fiber.StatusUnauthorized},
		// A bearer token is the next authenticator's to check: the request reaches the
		// route without these middlewares authenticating it.
		{"PQCAgentMiddleware, bearer token", s1Case{fn: s1FnPQCMiddleware, change: bearer}, fiber.StatusOK},
		{"Ed25519AgentMiddleware, bearer token", s1Case{fn: s1FnEd25519Middleware, change: bearer}, fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := scrapeAuthenticated(t)
			status, message, _ := driveS1(t, f, tc.tc, "AIM-Python-SDK/2.0.2")
			require.Equal(t, tc.status, status, message)
			assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}
}

func TestAgentRequestV2_CountsEachAuthenticatedRequestAsAgentRequestV2(t *testing.T) {
	keys := newAgentKeys(t)
	edOnly := v2Agent(keys, domain.AgentStatusVerified, false, false)
	hybrid := v2Agent(keys, domain.AgentStatusVerified, true, true)
	ed, ml := pqc.AlgorithmEd25519, pqc.AlgorithmMLDSA65

	for _, ua := range s1UserAgents {
		for _, tc := range []struct {
			name  string
			agent *domain.Agent
			algs  []pqc.Algorithm
		}{
			{"Ed25519", edOnly, []pqc.Algorithm{ed}},
			{"hybrid", hybrid, []pqc.Algorithm{ed, ml}},
		} {
			t.Run(tc.name+", "+ua.sdk, func(t *testing.T) {
				h := newV2Harness(v2Audience, tc.agent)
				req := v2Request{agentID: tc.agent.ID, keys: keys, algs: tc.algs, nonce: v2Nonce(t),
					headers: map[string]string{"User-Agent": ua.header}}

				before := scrapeAuthenticated(t)
				status, body := h.send(t, req)
				require.Equal(t, fiber.StatusOK, status, body)
				assert.Equal(t, map[string]float64{"agent-request-v2." + ua.sdk: 1},
					authenticatedMoved(before, scrapeAuthenticated(t)))

				// The same request again is refused as a replay and not counted.
				before = scrapeAuthenticated(t)
				status, body = h.send(t, req)
				require.Equal(t, fiber.StatusUnauthorized, status, body)
				assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
			})
		}
	}
}

func TestAgentRequestV2_CountsNoRequestItRefusesOrPassesOn(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	suspended := v2Agent(keys, domain.AgentStatusSuspended, false, false)
	ua := map[string]string{"User-Agent": "AIM-SDK-TypeScript/1.4.0"}

	cases := map[string]struct {
		audience string
		req      v2Request
		status   int
	}{
		"a signature that does not verify": {v2Audience,
			v2Request{agentID: agent.ID, keys: keys, flip: true, headers: ua}, fiber.StatusUnauthorized},
		"a suspended agent": {v2Audience,
			v2Request{agentID: suspended.ID, keys: keys, headers: ua}, fiber.StatusUnauthorized},
		"a timestamp behind the window": {v2Audience,
			v2Request{agentID: agent.ID, keys: keys, timestamp: time.Now().Add(-time.Hour), headers: ua}, fiber.StatusUnauthorized},
		"no configured audience": {"",
			v2Request{agentID: agent.ID, keys: keys, headers: ua}, fiber.StatusServiceUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newV2Harness(tc.audience, agent, suspended)
			before := scrapeAuthenticated(t)
			status, body := h.send(t, tc.req)
			require.Equal(t, tc.status, status, body)
			assert.Equal(t, 0, h.reached)
			assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}

	t.Run("a request without the statement", func(t *testing.T) {
		h := newV2Harness(v2Audience, agent)
		before := scrapeAuthenticated(t)
		req := httptest.NewRequest("GET", v2Path, nil)
		req.Header.Set("User-Agent", "AIM-SDK-TypeScript/1.4.0")
		resp, err := h.app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.Equal(t, 1, h.reached, "the request reaches the next handler")
		assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
	})
}
