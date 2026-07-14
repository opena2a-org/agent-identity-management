package middleware

import (
	"database/sql"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAPIKeyDB returns a *sql.DB whose api_keys lookup resolves to an org-scoped,
// agent-anchored key (the shape of the Cartographer's machine key). active controls
// whether the key is usable — an inactive key must NOT authenticate, which is how the
// control below is kept honest.
func newAPIKeyDB(t *testing.T, active bool) *sql.DB {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	rows := sqlmock.NewRows([]string{"id", "organization_id", "agent_id", "user_id", "name", "is_active", "expires_at"}).
		AddRow(uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), "honeymap-cartographer-machine-key", active, nil)
	mock.ExpectQuery("api_keys").WithArgs(sqlmock.AnyArg()).WillReturnRows(rows)
	// The UPDATE last_used_at only runs when the key is active (middleware returns early otherwise).
	if active {
		mock.ExpectExec("UPDATE api_keys").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// registerGated wires the real OptionalAPIKeyMiddleware, then (optionally) the
// MemberMiddleware gate, then a sentinel that echoes the resolved auth principal.
// The sentinel returns c.Locals("auth_method") as the body so a 200 response is only
// meaningful if the key actually authenticated (empty body => no principal). This is
// what keeps the control below from being a tautology.
func registerGated(app *fiber.App, method, pattern string, gate bool, db *sql.DB) {
	app.Use(OptionalAPIKeyMiddleware(db))
	if gate {
		app.Use(MemberMiddleware())
	}
	reached := func(c fiber.Ctx) error {
		am, _ := c.Locals("auth_method").(string)
		return c.Status(fiber.StatusOK).SendString(am)
	}
	switch method {
	case "GET":
		app.Get(pattern, reached)
	case "POST":
		app.Post(pattern, reached)
	case "PUT":
		app.Put(pattern, reached)
	}
}

func concretePath(pattern string) string {
	return strings.Replace(pattern, ":id", "00000000-0000-0000-0000-000000000000", 1)
}

func callBearer(t *testing.T, method, pattern string, gate, activeKey bool) (int, string) {
	t.Helper()
	app := fiber.New()
	registerGated(app, method, pattern, gate, newAPIKeyDB(t, activeKey))
	req := httptest.NewRequest(method, concretePath(pattern), nil)
	req.Header.Set("Authorization", "Bearer aim_live_test_machine_key")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// TestPrivateKeyRoutes_BareAPIKeyBlocked proves, through the REAL api-key auth path,
// that a bare org-scoped API key cannot reach the agent routes that return or derive
// private key material once they are MemberMiddleware()-gated:
//
//	GET  /agents/:id/credentials        -> decrypted Ed25519 private key
//	GET  /agents/:id/sdk                -> SDK zip with the private key embedded
//	POST /agents/:id/rotate-credentials -> returns a fresh private key (LOW-2)
//	PUT  /agents/:id/keys               -> replaces the signing key
//
// The control asserts the SAME key on the UNgated route both reaches the handler (200)
// AND was recognized as an api_key principal (body == "api_key"). That second assertion
// is load-bearing: without it a 401 on the gated route could be a false pass caused by an
// unrecognized key (no principal -> no role -> 401 anyway). Remove the gate and the gated
// route returns 200/"api_key"; break api-key auth and the control's body assertion fails.
func TestPrivateKeyRoutes_BareAPIKeyBlocked(t *testing.T) {
	cases := []struct{ name, method, pattern string }{
		{"credentials", "GET", "/api/v1/agents/:id/credentials"},
		{"sdk", "GET", "/api/v1/agents/:id/sdk"},
		{"rotate-credentials", "POST", "/api/v1/agents/:id/rotate-credentials"},
		{"keys", "PUT", "/api/v1/agents/:id/keys"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Control: the active api key authenticates (auth_method=api_key) and, ungated,
			// reaches the handler. Proves the key is a real recognized principal.
			code, body := callBearer(t, tc.method, tc.pattern, false, true)
			assert.Equal(t, fiber.StatusOK, code, "control: an active api key must reach the ungated handler")
			assert.Equal(t, "api_key", body, "control: the REAL OptionalAPIKeyMiddleware must recognize the key as an api_key principal")

			// Gated: MemberMiddleware() rejects the api_key principal (no role) with 401.
			gcode, _ := callBearer(t, tc.method, tc.pattern, true, true)
			assert.Equal(t, fiber.StatusUnauthorized, gcode,
				"a bare org-scoped API key must NOT reach %s (returns/derives private key material)", tc.pattern)
		})
	}
}

// TestControlHasTeeth guards the control assertion itself: an INACTIVE key must not
// authenticate, so even on the UNgated route the sentinel echoes no principal. If this
// ever returns "api_key", the api-key auth mock has drifted and the control above would
// be a tautology again (the exact defect the adversarial review caught).
func TestControlHasTeeth(t *testing.T) {
	code, body := callBearer(t, "GET", "/api/v1/agents/:id/credentials", false, false)
	assert.Equal(t, fiber.StatusOK, code, "sentinel always returns 200; the signal is the body")
	assert.NotEqual(t, "api_key", body, "an inactive key must NOT be recognized as an api_key principal")
}
