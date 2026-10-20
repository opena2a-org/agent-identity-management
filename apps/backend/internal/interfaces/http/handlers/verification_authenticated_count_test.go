package handlers

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// Verification creation and the SDK verification read verify the agent's signature
// themselves, and count each request they authenticate in
// aim_agent_requests_authenticated_total, under the scheme the request was signed in
// (type) and the calling SDK: verification-body for a body without signedBytes,
// action-request-v1 for a signed statement, verification-read for the read. A request
// they refuse moves no series of that family.

var authenticatedSeriesLine = regexp.MustCompile(`^aim_agent_requests_authenticated_total\{sdk="([^"]*)",type="([^"]*)"\} (\S+)$`)

// countUserAgents are the User-Agent each SDK sends, and one caller that is not an SDK.
var countUserAgents = []struct{ header, sdk string }{
	{"AIM-Python-SDK/2.0.2", "python"},
	{"AIM-SDK-TypeScript/1.4.0", "typescript"},
	{"AIM-Java-SDK/1.0.0", "java"},
	{"curl/8.7.1", "other"},
}

// scrapeAuthenticated reads aim_agent_requests_authenticated_total from the /metrics
// exposition, each series as "type.sdk".
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

func authenticatedMoved(before, after map[string]float64) map[string]float64 {
	moved := map[string]float64{}
	for series, value := range after {
		if delta := value - before[series]; delta != 0 {
			moved[series] = delta
		}
	}
	return moved
}

// sendWithUserAgent sends req with the User-Agent header set and returns the status
// and the body.
func sendWithUserAgent(t *testing.T, app *fiber.App, req *httptestRequest, userAgent string) (int, string) {
	t.Helper()
	httpReq := httptest.NewRequest(req.method, req.path, bytes.NewReader(req.body))
	if req.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for name, value := range req.headers {
		httpReq.Header.Set(name, value)
	}
	httpReq.Header.Set("User-Agent", userAgent)
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

type httptestRequest struct {
	method, path string
	body         []byte
	headers      map[string]string
}

// legacyVerificationBody is a verification request body without signedBytes, signed
// over verificationSigningMessage by priv, or by a signature with one bit flipped.
func legacyVerificationBody(t *testing.T, agentID uuid.UUID, priv ed25519.PrivateKey, flip bool) []byte {
	t.Helper()
	req := VerificationRequest{
		AgentID:    agentID.String(),
		Capability: "db:read",
		Resource:   "orders",
		Timestamp:  strconv.FormatInt(time.Now().Unix(), 10),
		PublicKey:  base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
	}
	message, err := verificationSigningMessage(req)
	require.NoError(t, err)
	sig := ed25519.Sign(priv, message)
	if flip {
		sig = flipByte(sig)
	}
	req.Signature = base64.StdEncoding.EncodeToString(sig)
	body, err := json.Marshal(req)
	require.NoError(t, err)
	return body
}

// sdkReadRequest is the SDK verification read of verificationID by agentID, signed by
// priv, or by a signature with one bit flipped.
func sdkReadRequest(verificationID, agentID uuid.UUID, priv ed25519.PrivateKey, flip bool) *httptestRequest {
	ts := time.Now().Unix()
	sig := ed25519.Sign(priv, []byte(fmt.Sprintf("GET\n/api/v1/sdk-api/verifications/%s\n%s\n%d", verificationID, agentID, ts)))
	if flip {
		sig = flipByte(sig)
	}
	return &httptestRequest{method: "GET", path: "/api/v1/sdk-api/verifications/" + verificationID.String(),
		headers: map[string]string{
			"X-AIM-Agent-ID":  agentID.String(),
			"X-AIM-Timestamp": strconv.FormatInt(ts, 10),
			"X-AIM-Signature": base64.StdEncoding.EncodeToString(sig),
		}}
}

func TestCreateVerification_CountsEachAuthenticatedRequestUnderItsScheme(t *testing.T) {
	for _, ua := range countUserAgents {
		t.Run("verification-body, "+ua.sdk, func(t *testing.T) {
			f := newStatementFixture(t)
			req := &httptestRequest{method: "POST", path: sdkVerificationsPath,
				body: legacyVerificationBody(t, f.signer.agentID, f.signer.priv, false)}

			before := scrapeAuthenticated(t)
			status, out := sendWithUserAgent(t, f.app, req, ua.header)
			require.Equal(t, fiber.StatusCreated, status, out)
			assert.Equal(t, map[string]float64{"verification-body." + ua.sdk: 1},
				authenticatedMoved(before, scrapeAuthenticated(t)))
		})

		t.Run("action-request-v1, "+ua.sdk, func(t *testing.T) {
			f := newStatementFixture(t)
			req := &httptestRequest{method: "POST", path: jwtVerificationsPath,
				body: f.signer.statement(f.signer.payload(t, ""))}

			before := scrapeAuthenticated(t)
			status, out := sendWithUserAgent(t, f.app, req, ua.header)
			require.Equal(t, fiber.StatusCreated, status, out)
			assert.Equal(t, map[string]float64{"action-request-v1." + ua.sdk: 1},
				authenticatedMoved(before, scrapeAuthenticated(t)))

			// The same statement again is refused as a replay and not counted.
			req.path = sdkVerificationsPath
			before = scrapeAuthenticated(t)
			status, out = sendWithUserAgent(t, f.app, req, ua.header)
			require.Equal(t, fiber.StatusUnauthorized, status, out)
			assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}
}

func TestCreateVerification_CountsNoRequestItRefuses(t *testing.T) {
	const ua = "AIM-Java-SDK/1.0.0"
	cases := map[string]struct {
		body      func(t *testing.T, f *statementFixture) []byte
		suspended bool
		status    int
	}{
		"verification-body, a signature that does not verify": {
			body: func(t *testing.T, f *statementFixture) []byte {
				return legacyVerificationBody(t, f.signer.agentID, f.signer.priv, true)
			},
			status: fiber.StatusUnauthorized},
		"verification-body, a suspended agent": {
			body: func(t *testing.T, f *statementFixture) []byte {
				return legacyVerificationBody(t, f.signer.agentID, f.signer.priv, false)
			},
			suspended: true, status: fiber.StatusForbidden},
		"action-request-v1, a signature that does not verify": {
			body: func(t *testing.T, f *statementFixture) []byte {
				frame := paeFrame(domain.ActionRequestPayloadType, []byte(f.signer.payload(t, "")))
				return f.signer.bodyWith(frame, flipByte(ed25519.Sign(f.signer.priv, frame)), f.signer.pub)
			},
			status: fiber.StatusUnauthorized},
		"action-request-v1, a suspended agent": {
			body: func(t *testing.T, f *statementFixture) []byte {
				return f.signer.statement(f.signer.payload(t, ""))
			},
			suspended: true, status: fiber.StatusUnauthorized},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newStatementFixture(t)
			if tc.suspended {
				f.agent.Status = domain.AgentStatusSuspended
			}
			req := &httptestRequest{method: "POST", path: sdkVerificationsPath, body: tc.body(t, f)}

			before := scrapeAuthenticated(t)
			status, out := sendWithUserAgent(t, f.app, req, ua)
			require.Equal(t, tc.status, status, out)
			assert.Equal(t, int32(0), f.capabilities.Load(), "the request did not reach the capability decision")
			assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}
}

func TestGetVerificationSDK_CountsEachAuthenticatedRequestAsVerificationRead(t *testing.T) {
	pub, priv := mustKeypair(t)
	agent := refusalAgent(domain.AgentStatusVerified, pub)
	app, vid := sdkReadApp(agent)

	for _, ua := range countUserAgents {
		t.Run(ua.sdk, func(t *testing.T) {
			before := scrapeAuthenticated(t)
			status, out := sendWithUserAgent(t, app, sdkReadRequest(vid, agent.ID, priv, false), ua.header)
			require.Equal(t, fiber.StatusOK, status, out)
			assert.Equal(t, map[string]float64{"verification-read." + ua.sdk: 1},
				authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}
}

func TestGetVerificationSDK_CountsNoRequestItRefuses(t *testing.T) {
	pub, priv := mustKeypair(t)
	_, otherPriv := mustKeypair(t)
	verified := refusalAgent(domain.AgentStatusVerified, pub)
	suspended := refusalAgent(domain.AgentStatusSuspended, pub)
	app, vid := sdkReadApp(verified, suspended)

	cases := map[string]*httptestRequest{
		"a signature that does not verify": sdkReadRequest(vid, verified.ID, priv, true),
		"another agent's key":              sdkReadRequest(vid, verified.ID, otherPriv, false),
		"an unknown agent":                 sdkReadRequest(vid, uuid.New(), priv, false),
		"a suspended agent":                sdkReadRequest(vid, suspended.ID, priv, false),
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			before := scrapeAuthenticated(t)
			status, out := sendWithUserAgent(t, app, req, "AIM-Python-SDK/2.0.2")
			require.Equal(t, fiber.StatusUnauthorized, status, out)
			assert.Empty(t, authenticatedMoved(before, scrapeAuthenticated(t)))
		})
	}
}
