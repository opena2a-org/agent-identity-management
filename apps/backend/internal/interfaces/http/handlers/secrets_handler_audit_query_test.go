package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// GET /api/v1/secrets/audit reads three optional filters: since, an RFC 3339
// time, and limit and offset, integers of 0 or more. A filter the route cannot
// read is refused with 400 and an error line that names it, before the agent
// or the audit log is read; a filter it reads reaches the audit repository as
// parsed, and an absent or empty one reads as its default.

// auditListCall is one ListByAgent call an auditListRecorder received.
type auditListCall struct {
	agentID       uuid.UUID
	since         *time.Time
	limit, offset int
}

func (c auditListCall) String() string {
	since := "<nil>"
	if c.since != nil {
		since = c.since.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("ListByAgent(%s, since=%s, limit=%d, offset=%d)", c.agentID, since, c.limit, c.offset)
}

// auditListRecorder is a secrets.AuditRepository that records each ListByAgent
// call and answers no entries; its other methods are not reached.
type auditListRecorder struct {
	secrets.AuditRepository
	calls []auditListCall
}

func (r *auditListRecorder) ListByAgent(agentID uuid.UUID, since *time.Time, limit, offset int) ([]*secrets.SecretAuditEntry, error) {
	r.calls = append(r.calls, auditListCall{agentID: agentID, since: since, limit: limit, offset: offset})
	return []*secrets.SecretAuditEntry{}, nil
}

// secretsAuditQueryFixture serves the audit route for one agent of the
// caller's organization over the real handler and service.
type secretsAuditQueryFixture struct {
	orgID, agentID uuid.UUID
	audit          *auditListRecorder
}

func newSecretsAuditQueryFixture() *secretsAuditQueryFixture {
	return &secretsAuditQueryFixture{orgID: uuid.New(), agentID: uuid.New(), audit: &auditListRecorder{}}
}

// get sends GET /api/v1/secrets/audit?agentId=<the agent>&<query> and returns
// the status and the error line of the answer, empty when the answer has none.
func (f *secretsAuditQueryFixture) get(t *testing.T, query string) (int, string) {
	t.Helper()
	agents := &secretsTestAgentRepo{getByID: func(id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, OrganizationID: f.orgID, Name: "audited-agent"}, nil
	}}
	svc := application.NewSecretsService(nil, f.audit, agents, nil, nil, nil)
	h := NewSecretsHandler(svc, agents, nil, nil)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", f.orgID)
		return c.Next()
	})
	app.Get("/api/v1/secrets/audit", h.GetAuditLog)

	path := "/api/v1/secrets/audit?agentId=" + f.agentID.String()
	if query != "" {
		path += "&" + query
	}
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &answer), "the answer is a JSON object: %s", body)
	return resp.StatusCode, answer.Error
}

const (
	sinceNotATimeLine   = "since must be an RFC 3339 time such as 2026-10-17T09:30:00Z"
	limitNotACountLine  = "limit must be an integer of 0 or more"
	offsetNotACountLine = "offset must be an integer of 0 or more"
)

func TestSecretsHandler_GetAuditLog_RefusesAFilterItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name, query, want string
	}{
		{"since that is not a time", "since=not-a-time", sinceNotATimeLine},
		{"since that is a date without a time", "since=2026-10-16", sinceNotATimeLine},
		{"since that has no UTC offset", "since=2026-10-16T10:00:00", sinceNotATimeLine},
		// A `+` sent as is in a query is decoded as a space, so the offset
		// `+02:00` reaches the parser as ` 02:00`.
		{"since whose positive offset was sent unencoded", "since=2026-10-16T10:00:00+02:00", sinceNotATimeLine},
		{"limit that is not a number", "limit=abc", limitNotACountLine},
		{"limit that is not an integer", "limit=1.5", limitNotACountLine},
		{"limit below 0", "limit=-1", limitNotACountLine},
		{"offset that is not a number", "offset=abc", offsetNotACountLine},
		{"offset below 0", "offset=-1", offsetNotACountLine},
		{"readable since beside an unreadable limit", "since=2026-10-16T10:00:00Z&limit=abc", limitNotACountLine},
		{"readable limit beside an unreadable offset", "limit=5&offset=x", offsetNotACountLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSecretsAuditQueryFixture()

			status, errLine := f.get(t, tc.query)

			assert.Equal(t, fiber.StatusBadRequest, status)
			assert.Equal(t, tc.want, errLine)
			assert.Empty(t, f.audit.calls, "a refused request does not read the audit log")
		})
	}
}

func TestSecretsHandler_GetAuditLog_PassesTheFiltersItReadsToTheRepository(t *testing.T) {
	tenUTC := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, query string
		since       *time.Time
		limit       int
		offset      int
	}{
		{"no filter", "", nil, 50, 0},
		{"since in UTC with a limit and an offset", "since=2026-10-16T10:00:00Z&limit=7&offset=3", &tenUTC, 7, 3},
		{"since with fractional seconds", "since=2026-10-16T10:00:00.250Z", ptrTime(tenUTC.Add(250 * time.Millisecond)), 50, 0},
		// 10:00 at +02:00 is 08:00 UTC; the `+` is sent encoded, as the SDKs send it.
		{"since with an encoded positive offset", "since=2026-10-16T10%3A00%3A00%2B02%3A00", ptrTime(tenUTC.Add(-2 * time.Hour)), 50, 0},
		{"since with a negative offset", "since=2026-10-16T10:00:00-03:00", ptrTime(tenUTC.Add(3 * time.Hour)), 50, 0},
		{"limit of 0", "limit=0", nil, 0, 0},
		{"empty since, limit and offset", "since=&limit=&offset=", nil, 50, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSecretsAuditQueryFixture()

			status, errLine := f.get(t, tc.query)

			assert.Equal(t, fiber.StatusOK, status)
			assert.Empty(t, errLine)
			require.Len(t, f.audit.calls, 1, "the audit log is read once")
			call := f.audit.calls[0]
			assert.Equal(t, f.agentID, call.agentID)
			if tc.since == nil {
				assert.Nil(t, call.since, "no time filter")
			} else if assert.NotNil(t, call.since, "a time filter") {
				assert.WithinDuration(t, *tc.since, *call.since, 0)
			}
			assert.Equal(t, tc.limit, call.limit)
			assert.Equal(t, tc.offset, call.offset)
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
