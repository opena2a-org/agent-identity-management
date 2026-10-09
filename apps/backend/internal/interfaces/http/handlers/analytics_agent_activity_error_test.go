package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A failed activity query answers the empty fallback with a fixed note; the
// database error's text, which can name tables, hosts and SQL, stays out of
// the body.
func TestAnalyticsHandler_GetAgentActivity_QueryErrorTextIsNotInTheBody(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const dbError = `pq: relation "verification_events" does not exist (host db-internal.example:5432)`
	mock.ExpectQuery(`WITH unified_activities AS`).WillReturnError(errors.New(dbError))

	handler := &AnalyticsHandler{db: db}
	app := fiber.New()
	app.Get("/analytics/agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentActivity(c)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/analytics/agents", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.NotContains(t, string(raw), "verification_events")
	assert.NotContains(t, string(raw), "db-internal.example")
	var body struct {
		Note string `json:"note"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "Activity data unavailable: the activity query failed", body.Note)
	require.NoError(t, mock.ExpectationsWereMet())
}
