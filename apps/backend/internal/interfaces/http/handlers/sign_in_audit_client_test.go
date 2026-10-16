package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The client address a login or logout audit row records is the same one the
// refresh route's reuse rows record: the direct peer, or, only when
// TRUSTED_PROXIES names the peer, the address the proxy reports. A proxy
// value that does not parse as an address falls back to the peer. In these
// tests the peer is 0.0.0.0, the address fiber's in-process test transport
// reports.

const (
	testPeer        = "0.0.0.0"
	proxiedClientIP = "203.0.113.9"
)

// postProxied sends one request carrying X-Real-IP and the given bearer.
func postProxied(t *testing.T, app *fiber.App, path, body, bearer, realIP string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mint-cell/1")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	// A sign-in verifies a password hash, which outlasts the default test
	// timeout under the race detector.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func rowsWithAction(rows []*domain.AuditLog, action domain.AuditAction) []*domain.AuditLog {
	var out []*domain.AuditLog
	for _, row := range rows {
		if row.Action == action {
			out = append(out, row)
		}
	}
	return out
}

func loginRowAddress(t *testing.T, realIP string) string {
	t.Helper()
	user := issuanceUser(t, false)
	app, _, audit := publicLoginApp(t, user)
	status, body := postProxied(t, app, "/public/login", signInLoginBody(), "", realIP)
	require.Equal(t, fiber.StatusOK, status, body)
	logins := rowsWithAction(audit.rows, domain.AuditActionLogin)
	require.Len(t, logins, 1, "exactly one login row")
	return logins[0].IPAddress
}

func logoutRowAddress(t *testing.T, realIP string) string {
	t.Helper()
	h, svc, repo, userID, orgID := logoutAuditFixture(t)
	access, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "audit@example.com", "admin")
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/api/v1/auth/logout", h.Logout)
	status, body := postProxied(t, app, "/api/v1/auth/logout", `{"refreshToken":"`+refresh+`"}`, access, realIP)
	require.Equal(t, fiber.StatusOK, status, body)
	logouts := rowsWithAction(repo.rows, domain.AuditActionLogout)
	require.Len(t, logouts, 1, "exactly one logout row")
	return logouts[0].IPAddress
}

// Behind a trusted proxy the login row names the client the proxy reports.
func TestSignInAudit_LoginRowRecordsTheTrustedProxyClient(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", testPeer)
	assert.Equal(t, proxiedClientIP, loginRowAddress(t, proxiedClientIP))
}

// Behind a trusted proxy the logout row names the client the proxy reports.
func TestAuthHandler_Logout_AuditRowRecordsTheTrustedProxyClient(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", testPeer)
	assert.Equal(t, proxiedClientIP, logoutRowAddress(t, proxiedClientIP))
}

// Without TRUSTED_PROXIES the header is not trusted: both rows name the peer.
func TestSignInAudit_RowsIgnoreTheProxyHeaderWithoutTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	assert.Equal(t, testPeer, loginRowAddress(t, proxiedClientIP))
	assert.Equal(t, testPeer, logoutRowAddress(t, proxiedClientIP))
}

// A trusted proxy's value that is not an address is not recorded: both rows
// name the peer, as the refresh route's reuse rows do.
func TestSignInAudit_RowsFallBackToThePeerWhenTheProxyValueIsNotAnAddress(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", testPeer)
	assert.Equal(t, testPeer, loginRowAddress(t, "not an address"))
	assert.Equal(t, testPeer, logoutRowAddress(t, "not an address"))
}
