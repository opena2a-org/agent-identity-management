package handlers

// The free-text query values of four handlers: GET /api/v1/a2a/skills/search
// (q), GET /api/v1/mcp-servers/by-name (name), GET /api/v1/admin/alerts
// (severity, status) and GET /api/v1/admin/verifications/pending (status, risk,
// search, search_field). A value that is not UTF-8 text or holds a NUL
// character is answered 400 with the name of its parameter, before the handler
// reads anything; a value that is text still reaches the read it filters.

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// notTextCases are query values that are not UTF-8 text or hold a NUL
// character, written as they appear in a URL.
var notTextCases = []struct {
	name  string
	value string
}{
	{"a NUL character", "%00"},
	{"text holding a NUL character", "ab%00c"},
	{"a byte that is not UTF-8", "%ff"},
	{"a truncated UTF-8 sequence", "%e2%82"},
}

// getAsCaller sends GET target to handler, mounted at path, as a caller of
// orgID, and returns the status and the JSON object answered.
func getAsCaller(t *testing.T, path string, handler fiber.Handler, orgID uuid.UUID, target string) (int, map[string]interface{}) {
	t.Helper()
	app := fiber.New()
	app.Get(path, func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		return c.Next()
	}, handler)

	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body), "the answer is a JSON object: %s", raw)
	return resp.StatusCode, body
}

// assertNotTextRefused asserts a 400 that names param.
func assertNotTextRefused(t *testing.T, param string, status int, body map[string]interface{}) {
	t.Helper()
	assert.Equal(t, fiber.StatusBadRequest, status, "answer: %v", body)
	assert.Equal(t, param+" must be UTF-8 text without a NUL character", body["error"])
}

// skillSearchHandler is the A2A skill search with the server's service and
// skill repository over db.
func skillSearchHandler(db *sql.DB) fiber.Handler {
	svc := application.NewA2AService(
		nil, repository.NewA2ASkillRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return NewA2AHandler(svc, nil, nil).SearchSkills
}

func TestSearchSkills_QThatIsNotUTF8TextIsABadRequest(t *testing.T) {
	for _, tc := range notTextCases {
		t.Run("q is "+tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			status, body := getAsCaller(t, "/a2a/skills/search", skillSearchHandler(db), uuid.New(),
				"/a2a/skills/search?q="+tc.value)

			assertNotTextRefused(t, "q", status, body)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSearchSkills_QThatIsTextReachesTheSearchStatement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		q       string
		pattern string
	}{
		{"ASCII", "abc", "%abc%"},
		{"outside ASCII", "%e2%82%ac", "%\xe2\x82\xac%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(regexp.QuoteMeta("WHERE name ILIKE $1 OR description ILIKE $1 OR skill_id ILIKE $1")).
				WithArgs(tc.pattern, 20).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))

			status, body := getAsCaller(t, "/a2a/skills/search", skillSearchHandler(db), uuid.New(),
				"/a2a/skills/search?q="+tc.q)

			assert.Equal(t, fiber.StatusOK, status, "answer: %v", body)
			assert.Equal(t, float64(0), body["count"])
			require.NoError(t, mock.ExpectationsWereMet(), "the search statement ran with the pattern")
		})
	}
}

// mcpByNameHandler is the MCP server lookup by name over a service that
// records each name it is asked for.
func mcpByNameHandler(looked *[]string) fiber.Handler {
	svc := &MockMCPServiceExtendedImpl{
		GetMCPServerByNameFunc: func(_ context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error) {
			*looked = append(*looked, name)
			return &domain.MCPServer{ID: uuid.New(), OrganizationID: orgID, Name: name}, nil
		},
	}
	return createMCPHandlerWithMocks(svc, nil, nil, nil, nil, nil, nil).GetMCPServerByName
}

func TestGetMCPServerByName_NameThatIsNotUTF8TextIsABadRequest(t *testing.T) {
	for _, tc := range notTextCases {
		t.Run("name is "+tc.name, func(t *testing.T) {
			var looked []string
			status, body := getAsCaller(t, "/mcp-servers/by-name", mcpByNameHandler(&looked), uuid.New(),
				"/mcp-servers/by-name?name="+tc.value)

			assertNotTextRefused(t, "name", status, body)
			assert.Empty(t, looked, "no server is looked up")
		})
	}
}

func TestGetMCPServerByName_NameThatIsTextIsLookedUp(t *testing.T) {
	var looked []string
	status, body := getAsCaller(t, "/mcp-servers/by-name", mcpByNameHandler(&looked), uuid.New(),
		"/mcp-servers/by-name?name=server-%e2%82%ac")

	assert.Equal(t, fiber.StatusOK, status, "answer: %v", body)
	assert.Equal(t, []string{"server-\xe2\x82\xac"}, looked)
	assert.Equal(t, "server-\xe2\x82\xac", body["name"])
}

// alertFilter is the severity and status one alert read was asked for.
type alertFilter struct{ severity, status string }

// adminAlertsHandler is the admin alert list over an alert service that
// records the filter of each read.
func adminAlertsHandler(read *[]alertFilter) fiber.Handler {
	alerts := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(_ context.Context, _ uuid.UUID, severity, status string, _, _ int) ([]*domain.Alert, int, error) {
			*read = append(*read, alertFilter{severity, status})
			return []*domain.Alert{}, 0, nil
		},
	}
	return createAdminHandlerWithMocks(nil, nil, nil, nil, &MockAuditServiceImpl{}, alerts, nil, nil).GetAlerts
}

func TestAdminGetAlerts_FilterThatIsNotUTF8TextIsABadRequest(t *testing.T) {
	for _, param := range []string{"severity", "status"} {
		for _, tc := range notTextCases {
			t.Run(param+" is "+tc.name, func(t *testing.T) {
				var read []alertFilter
				status, body := getAsCaller(t, "/admin/alerts", adminAlertsHandler(&read), uuid.New(),
					"/admin/alerts?"+param+"="+tc.value)

				assertNotTextRefused(t, param, status, body)
				assert.Empty(t, read, "no alert is read")
			})
		}
	}

	t.Run("severity is named before status", func(t *testing.T) {
		var read []alertFilter
		status, body := getAsCaller(t, "/admin/alerts", adminAlertsHandler(&read), uuid.New(),
			"/admin/alerts?status=%ff&severity=%00")

		assertNotTextRefused(t, "severity", status, body)
		assert.Empty(t, read, "no alert is read")
	})
}

func TestAdminGetAlerts_FilterThatIsTextReachesTheAlertRead(t *testing.T) {
	var read []alertFilter
	status, body := getAsCaller(t, "/admin/alerts", adminAlertsHandler(&read), uuid.New(),
		"/admin/alerts?severity=critical&status=unacknowledged")

	assert.Equal(t, fiber.StatusOK, status, "answer: %v", body)
	assert.Equal(t, []alertFilter{{"critical", "unacknowledged"}}, read)
}

// pendingVerificationsHandler is the admin verification list with the
// server's verification event service and repository over db.
func pendingVerificationsHandler(db *sql.DB) fiber.Handler {
	svc := application.NewVerificationEventService(repository.NewVerificationEventRepository(db), nil, nil)
	return NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		svc,
		&MockOrganizationRepositoryerImpl{},
	).ListPendingVerifications
}

func TestListPendingVerifications_ValueThatIsNotUTF8TextIsABadRequest(t *testing.T) {
	for _, param := range []string{"status", "risk", "search", "search_field"} {
		for _, tc := range notTextCases {
			t.Run(param+" is "+tc.name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()

				status, body := getAsCaller(t, "/admin/verifications/pending", pendingVerificationsHandler(db), uuid.New(),
					"/admin/verifications/pending?"+param+"="+tc.value)

				assertNotTextRefused(t, param, status, body)
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}

	t.Run("status is named before search", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		status, body := getAsCaller(t, "/admin/verifications/pending", pendingVerificationsHandler(db), uuid.New(),
			"/admin/verifications/pending?search=%00&status=%ff")

		assertNotTextRefused(t, "status", status, body)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// Values that are text, in any letter case, are lowercased as before: status
// APPROVED selects successful events, risk HIGH and search Read€ reach the
// statements in lower case, and search_field Agent searches the agent name.
func TestListPendingVerifications_TextValuesReachTheSearchStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	orgID := uuid.New()
	filter := "WHERE organization_id = $1 AND status = 'success' AND " +
		"((LOWER(metadata ->> 'risk_level') = $2) OR (LOWER(metadata -> 'context' ->> 'risk_level') = $2)) AND " +
		"LOWER(COALESCE(agent_name, '')) LIKE $3"

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM verification_events "+filter)).
		WithArgs(orgID, "high", "%read\xe2\x82\xac%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("COUNT(*) FILTER (WHERE result = 'expired') AS expired")).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"pending", "approved", "denied", "expired"}).AddRow(0, 0, 0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(filter)).
		WithArgs(orgID, "high", "%read\xe2\x82\xac%", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	status, body := getAsCaller(t, "/admin/verifications/pending", pendingVerificationsHandler(db), orgID,
		"/admin/verifications/pending?status=APPROVED&risk=HIGH&search=Read%e2%82%ac&search_field=Agent")

	assert.Equal(t, fiber.StatusOK, status, "answer: %v", body)
	require.NoError(t, mock.ExpectationsWereMet(), "each statement ran with the lowercased values")
}
