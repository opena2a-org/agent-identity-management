package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dashboard's developers page renders apps/web/lib/api-documentation.ts and
// offers its example bodies to copy. These tests hold the POST /api/v1/api-keys
// entry to CreateAPIKey: the documented example is posted to the handler,
// mounted behind a sign-in as the route is, and the documented request and
// response fields are compared with what the handler reads and returns.

const apiKeysPath = "/api/v1/api-keys"

// dashboardEndpointEntry returns the text of the one entry whose method and
// path are the given ones, up to the next entry's method line.
func dashboardEndpointEntry(t *testing.T, doc, method, apiPath string) string {
	t.Helper()

	head := regexp.MustCompile(fmt.Sprintf(`method: %q,\s*path: %q,`, method, apiPath))
	found := head.FindAllStringIndex(doc, -1)
	require.Len(t, found, 1, "api-documentation.ts must document %s %s exactly once", method, apiPath)

	entry := doc[found[0][1]:]
	if next := strings.Index(entry, "method: \""); next >= 0 {
		entry = entry[:next]
	}
	return entry
}

// postAPIKeyAsSignedIn posts body to CreateAPIKey with a sign-in's access token
// and the agent lookup answering an agent of the caller's organization.
func postAPIKeyAsSignedIn(t *testing.T, body string) (int, map[string]any) {
	t.Helper()

	f := newMintFixture(t, &rotationStore{}, false)
	var calls []string
	app := fiber.New()
	app.Post("/api-keys", f.acting(f.family, apiKeyHandlerFor(f, &calls).CreateAPIKey))

	status, raw := doRequest(t, app, "POST", "/api-keys", body)
	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &answer), "the answer is a JSON object")
	return status, answer
}

// assertExpiresInNinetyDays checks that expiresAt is 90 calendar days after a
// moment between before and after.
func assertExpiresInNinetyDays(t *testing.T, answer map[string]any, before, after time.Time) {
	t.Helper()

	raw, ok := answer["expiresAt"].(string)
	require.True(t, ok, "expiresAt is a timestamp string, got %v", answer["expiresAt"])
	expiresAt, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err)
	assert.False(t, expiresAt.Before(before.AddDate(0, 0, 90)), "expiresAt %s is 90 days after creation", raw)
	assert.False(t, expiresAt.After(after.AddDate(0, 0, 90)), "expiresAt %s is 90 days after creation", raw)
}

func TestDashboardAPIReference_CreateAPIKeyMatchesTheHandler(t *testing.T) {
	doc := readBackendFile(t, "../web/lib/api-documentation.ts")
	entry := dashboardEndpointEntry(t, doc, "POST", apiKeysPath)
	label := "POST " + apiKeysPath

	requestAt := strings.Index(entry, "requestSchema:")
	require.GreaterOrEqual(t, requestAt, 0, "%s has no requestSchema", label)
	documented := documentedRequestIn(t, entry[requestAt:], label)

	responseAt := strings.Index(entry, "responseSchema:")
	require.GreaterOrEqual(t, responseAt, 0, "%s has no responseSchema", label)
	responseFields, _ := docSchemaProperties(t, entry[responseAt:], label)

	t.Run("the request lists the two fields the handler requires, and only those", func(t *testing.T) {
		// CreateAPIKey also binds expiresAt but does not apply it, so the
		// reference does not offer it.
		want := []string{"agentId", "name"}
		assert.Equal(t, want, documented.properties)
		assert.Equal(t, want, documented.required)
		assert.Equal(t, want, documented.exampleKeys)
	})

	t.Run("the example body creates a key and the answer carries the documented fields", func(t *testing.T) {
		before := time.Now()
		status, answer := postAPIKeyAsSignedIn(t, documented.example)
		after := time.Now()

		require.Equal(t, fiber.StatusCreated, status, "the example body is refused: %v", answer["error"])
		assert.Equal(t, sortedKeys(answer), responseFields,
			"the documented response fields are the ones the handler returns")
		assertExpiresInNinetyDays(t, answer, before, after)
	})

	t.Run("an expiresAt in the body does not change the 90-day expiry", func(t *testing.T) {
		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(documented.example), &body))
		body["expiresAt"] = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		withExpiry, err := json.Marshal(body)
		require.NoError(t, err)

		before := time.Now()
		status, answer := postAPIKeyAsSignedIn(t, string(withExpiry))
		after := time.Now()

		require.Equal(t, fiber.StatusCreated, status, "refused: %v", answer["error"])
		assertExpiresInNinetyDays(t, answer, before, after)
	})
}
