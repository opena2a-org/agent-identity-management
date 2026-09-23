package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiKeyRegistrationApp composes the /agents chain in the order main.go mounts
// the three middlewares that decide an API-key caller's fate at POST /api/v1/agents:
// OptionalAPIKeyMiddleware, then AuthMiddleware (JWT fallback, skipped when
// auth_method is api_key), then the per-route MemberOrAPIKeyMiddleware.
func apiKeyRegistrationApp(t *testing.T, db *sql.DB) *fiber.App {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	app := fiber.New()
	app.Use(OptionalAPIKeyMiddleware(db))
	app.Use(AuthMiddleware(auth.NewJWTService()))
	app.Post("/api/v1/agents", MemberOrAPIKeyMiddleware(), func(c fiber.Ctx) error {
		m, _ := c.Locals("auth_method").(string)
		org, _ := c.Locals("organization_id").(uuid.UUID)
		return c.JSON(fiber.Map{"authMethod": m, "organizationId": org.String()})
	})
	return app
}

func apiKeyHashForTest(key string) string {
	h := sha256.Sum256([]byte(key))
	return base64.StdEncoding.EncodeToString(h[:])
}

// A valid X-API-Key on an active key for a verified agent is admitted at
// POST /api/v1/agents with auth_method=api_key and the key's organization.
func TestXAPIKeyHeaderIsAdmittedAtPostAgents(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	key := "aim_live_test_key_not_real"
	orgID, agentID, userID, keyID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mock.ExpectQuery("SELECT ak.id, ak.organization_id, ak.agent_id").
		WithArgs(apiKeyHashForTest(key)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "agent_id", "user_id", "name", "is_active", "expires_at", "status"}).
			AddRow(keyID, orgID, agentID, userID, "k", true, nil, "verified"))
	mock.ExpectExec("UPDATE api_keys SET last_used_at").WithArgs(keyID).WillReturnResult(sqlmock.NewResult(0, 1))

	app := apiKeyRegistrationApp(t, db)
	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("X-API-Key", key)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, 200, resp.StatusCode, string(body))
	var out map[string]string
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "api_key", out["authMethod"])
	assert.Equal(t, orgID.String(), out["organizationId"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// No credential at all is refused before the handler.
func TestNoCredentialAtPostAgentsIs401(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	app := apiKeyRegistrationApp(t, db)
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/agents", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

// An unknown X-API-Key is looked up, dropped by the optional middleware, and
// then refused by the JWT fallback.
func TestUnknownXAPIKeyAtPostAgentsIs401(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT ak.id, ak.organization_id, ak.agent_id").WillReturnError(sql.ErrNoRows)
	app := apiKeyRegistrationApp(t, db)
	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("X-API-Key", "aim_live_unknown_not_real")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The header the Python SDK sent until aim-sdk 2.0.3 is not read: no lookup
// is attempted and the request is refused.
func TestXAIMAPIKeyHeaderIsNotRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	app := apiKeyRegistrationApp(t, db)
	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("X-AIM-API-Key", "aim_live_test_key_not_real")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode, "a header the middleware does not read must not authenticate")
	assert.NoError(t, mock.ExpectationsWereMet(), "no lookup is attempted for X-AIM-API-Key")
}
