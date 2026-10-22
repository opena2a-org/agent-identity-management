package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// GET /api/v1/admin/alerts reads a limit below 1 as the default page size of
// 100. The route runs here on its service and its repository, behind the
// recovery and admin middleware the server mounts, from an admin's session,
// over a database that serves one alert.

// adminAlertsLimitArg stands for the page query's LIMIT argument. It keeps
// the integer limits the query is sent and matches a limit of 0 or more; a
// negative limit does not match, and the store refuses the query, as
// PostgreSQL 14 refuses a bound LIMIT of -1 or -100 with "LIMIT must not be
// negative".
type adminAlertsLimitArg struct{ sent []int64 }

func (a *adminAlertsLimitArg) Match(v driver.Value) bool {
	n, ok := v.(int64)
	if ok {
		a.sent = append(a.sent, n)
	}
	return ok && n >= 0
}

// adminAlertsLimitAudit keeps the audit rows the route writes.
type adminAlertsLimitAudit struct {
	domain.AuditLogRepository
	rows []*domain.AuditLog
}

func (a *adminAlertsLimitAudit) Create(log *domain.AuditLog) error {
	a.rows = append(a.rows, log)
	return nil
}

// adminAlertsLimitAdmin marks the request as an admin's session of org.
func adminAlertsLimitAdmin(org uuid.UUID) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", org)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleAdmin))
		return c.Next()
	}
}

type adminAlertsLimitResult struct {
	status  int
	body    []byte
	alertID string
	limits  []int64            // the limits the page query was sent
	audit   []*domain.AuditLog // the audit rows the route wrote
}

// adminAlertsLimitGet requests GET /api/v1/admin/alerts?query over a database
// whose page query expects offset wantOffset, and checks the store saw every
// query the route was expected to send.
func adminAlertsLimitGet(t *testing.T, query string, wantOffset int) adminAlertsLimitResult {
	t.Helper()
	db, store, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	org := uuid.New()
	audit := &adminAlertsLimitAudit{}
	alerts := application.NewAlertService(repository.NewAlertRepository(db), nil, db)
	h := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, application.NewAuditService(audit), alerts, nil, nil, nil)

	alertID := uuid.New()
	limit := &adminAlertsLimitArg{}
	store.ExpectQuery(`FROM alerts\s+WHERE organization_id = \$1\s+ORDER BY created_at DESC\s+LIMIT \$2 OFFSET \$3`).
		WithArgs(org, limit, wantOffset).
		WillReturnRows(listOffsetAlertRows(alertID, org, uuid.New()))
	for i := 0; i < 2; i++ { // the filtered count, then the count of every alert
		store.ExpectQuery(`^SELECT COUNT\(\*\) FROM alerts WHERE organization_id = \$1$`).
			WithArgs(org).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	}
	store.ExpectQuery(`FROM alerts\s+WHERE organization_id = \$1 AND is_acknowledged = false`).
		WithArgs(org).
		WillReturnRows(sqlmock.NewRows(listOffsetAlertColumns))
	store.ExpectQuery(`COUNT\(\*\) FILTER`).
		WithArgs(org).
		WillReturnRows(sqlmock.NewRows([]string{"critical_count", "high_count", "warning_count", "info_count"}).
			AddRow(0, 0, 1, 0))

	app := listOffsetApp()
	app.Get("/api/v1/admin/alerts", adminAlertsLimitAdmin(org), middleware.AdminMiddleware(), h.GetAlerts)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/alerts?"+query, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NoError(t, store.ExpectationsWereMet(), "the route sends each of its queries")
	return adminAlertsLimitResult{
		status:  resp.StatusCode,
		body:    body,
		alertID: alertID.String(),
		limits:  limit.sent,
		audit:   audit.rows,
	}
}

// assertPage checks the route wrote the view to the audit log with page size
// limit and page number page, and answered 200 with the one alert, echoing
// limit and offset.
func (r adminAlertsLimitResult) assertPage(t *testing.T, limit, offset, page int) {
	t.Helper()
	assert.Len(t, r.audit, 1, "the view is written to the audit log")
	require.Equal(t, fiber.StatusOK, r.status, string(r.body))

	var got struct {
		Alerts []map[string]json.RawMessage `json:"alerts"`
		Total  int                          `json:"total"`
		Limit  int                          `json:"limit"`
		Offset int                          `json:"offset"`
	}
	require.NoError(t, json.Unmarshal(r.body, &got))
	assert.Equal(t, []string{r.alertID}, listOffsetIDs(t, got.Alerts))
	assert.Equal(t, 1, got.Total)
	assert.Equal(t, limit, got.Limit, "the body echoes the limit read")
	assert.Equal(t, offset, got.Offset)

	require.Len(t, r.audit, 1)
	assert.Equal(t, domain.AuditActionView, r.audit[0].Action)
	assert.Equal(t, "alerts", r.audit[0].ResourceType)
	assert.Equal(t, limit, r.audit[0].Metadata["page_size"])
	assert.Equal(t, page, r.audit[0].Metadata["page_number"])
}

// With no limit sent, the page query is sent the default page size of 100.
func TestAdminAlerts_NoLimitIsTheDefaultPageSize(t *testing.T) {
	got := adminAlertsLimitGet(t, "", 0)
	assert.Equal(t, []int64{100}, got.limits)
	got.assertPage(t, 100, 0, 1)
}

// A limit below 1 reaches the page query as 100, and the page is answered
// 200 and written to the audit log.
func TestAdminAlerts_LimitBelowOneReadsAsDefaultPageSize(t *testing.T) {
	for _, tc := range []struct {
		query        string
		offset, page int
	}{
		{"limit=0", 0, 1},
		{"limit=-1", 0, 1},
		{"limit=-100", 0, 1},
		{"limit=0&offset=200", 200, 3},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := adminAlertsLimitGet(t, tc.query, tc.offset)
			assert.Equal(t, []int64{100}, got.limits, "the page query is sent the default page size")
			got.assertPage(t, 100, tc.offset, tc.page)
		})
	}
}

// A limit of 1 or more reaches the page query as sent.
func TestAdminAlerts_LimitOfOneOrMoreIsSentAsIs(t *testing.T) {
	for _, tc := range []struct {
		query               string
		limit, offset, page int
	}{
		{"limit=1", 1, 0, 1},
		{"limit=2", 2, 0, 1},
		{"limit=2&offset=4", 2, 4, 3},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := adminAlertsLimitGet(t, tc.query, tc.offset)
			assert.Equal(t, []int64{int64(tc.limit)}, got.limits)
			got.assertPage(t, tc.limit, tc.offset, tc.page)
		})
	}
}
