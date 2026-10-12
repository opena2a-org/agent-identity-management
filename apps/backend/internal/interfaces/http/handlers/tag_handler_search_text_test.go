package handlers

// GET /api/v1/tags/search by the query that reaches the database: the
// handler, the tag service and the tag repository are the server's own, over
// a sqlmock connection that answers only the statement each case expects.

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// tagSearchColumns are the columns the tag search selects, in order.
var tagSearchColumns = []string{
	"id", "organization_id", "key", "value", "category", "description", "color", "created_at", "created_by",
}

// searchTagsOverSQL sends one GET to the tag search handler, wired to the
// real service and repository over mock's connection, as a member of orgID,
// and returns the status and the raw answer.
func searchTagsOverSQL(t *testing.T, orgID uuid.UUID, target string, expect func(mock sqlmock.Sqlmock)) (int, []byte) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	expect(mock)

	tagService := application.NewTagService(repository.NewTagRepository(db), nil, nil)
	handler := NewTagHandler(tagService, nil, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Get("/tags/search", handler.SearchTags)

	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet(), "every expected statement reached the database")
	return resp.StatusCode, raw
}

// A q or category value that is not valid UTF-8 or holds a NUL character is
// answered 400 with the name of its parameter, and no statement reaches the
// database: PostgreSQL refuses such a value as a text parameter.
func TestSearchTags_ValueThatIsNotUTF8TextIsABadRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		param string
	}{
		{name: "q is a NUL character", query: "?q=%00", param: "q"},
		{name: "q holds a NUL character", query: "?q=ab%00c", param: "q"},
		{name: "q is a byte that is not UTF-8", query: "?q=%ff", param: "q"},
		{name: "q is a truncated UTF-8 sequence", query: "?q=%e2%82", param: "q"},
		{name: "category is a NUL character", query: "?q=abc&category=%00", param: "category"},
		{name: "category is a byte that is not UTF-8", query: "?q=abc&category=%c3%28", param: "category"},
		{name: "q is named before category", query: "?q=%ff&category=%00", param: "q"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, raw := searchTagsOverSQL(t, uuid.New(), "/tags/search"+tc.query, func(sqlmock.Sqlmock) {})

			assert.Equal(t, fiber.StatusBadRequest, status)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &body), "the answer is JSON: %s", raw)
			assert.Equal(t, tc.param+" must be UTF-8 text without a NUL character", body["error"])
		})
	}
}

// A q and a category that are UTF-8 text, inside or outside ASCII, reach the
// search statement as its parameters and the answer is 200 with the tags it
// returns.
func TestSearchTags_TextValuesReachTheSearchStatement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		query    string
		category string
		pattern  string
	}{
		{name: "q", query: "?q=abc", pattern: "%abc%"},
		{name: "q outside ASCII", query: "?q=%e2%82%ac", pattern: "%\xe2\x82\xac%"},
		{name: "q and category", query: "?q=abc&category=environment", category: "environment", pattern: "%abc%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orgID := uuid.New()
			now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

			status, raw := searchTagsOverSQL(t, orgID, "/tags/search"+tc.query, func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(tagSearchColumns).
					AddRow(uuid.New().String(), orgID.String(), "env", "abc", "environment", "", "", now, uuid.New().String())
				if tc.category == "" {
					mock.ExpectQuery(regexp.QuoteMeta("AND (key ILIKE $2 OR value ILIKE $2 OR description ILIKE $2)")).
						WithArgs(orgID, tc.pattern).
						WillReturnRows(rows)
					return
				}
				mock.ExpectQuery(regexp.QuoteMeta("AND category = $2")).
					WithArgs(orgID, tc.category, tc.pattern).
					WillReturnRows(rows)
			})

			assert.Equal(t, fiber.StatusOK, status, "answer: %s", raw)
			var tags []map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &tags), "the answer is a JSON list: %s", raw)
			require.Len(t, tags, 1)
			assert.Equal(t, "env", tags[0]["key"])
		})
	}
}
