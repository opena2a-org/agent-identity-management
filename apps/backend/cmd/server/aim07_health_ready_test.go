package main

// AIM-07: GET /health/ready body contract, commit stamping, vendored schema.
//
// The readiness checks are injected into newHealthReadyHandler, so every cell
// below runs without a database or Redis. Each response body is validated
// against the vendored contract, apps/backend/contracts/health-ready.schema.json,
// whose sha256 is pinned here so the schema cannot drift silently.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aim07SchemaSHA256 pins apps/backend/contracts/health-ready.schema.json.
// If the schema changes, this pin must be updated deliberately — the file is
// vendored from the fleet-wide contract and must stay byte-identical to the
// copies in opena2a-registry and aim-cloud.
const aim07SchemaSHA256 = "a936972888f9e17632deb4398876eab4b8f790876178781ee8e52a525b2b9b18"

// aim07HealthHandlerSource is the GET /health handler exactly as it exists at
// base (main.go:224-230 at 5405500a); AIM-07.AC4 requires it unchanged byte
// for byte.
const aim07HealthHandlerSource = `	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "agent-identity-management",
			"time":    time.Now().UTC(),
		})
	})`

func aim07Path(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed; cannot locate package directory")
	return filepath.Join(filepath.Dir(thisFile), rel)
}

type aim07Schema struct {
	raw    map[string]any
	sha256 string
}

func aim07LoadSchema(t *testing.T) *aim07Schema {
	t.Helper()
	b, err := os.ReadFile(aim07Path(t, filepath.Join("..", "..", "contracts", "health-ready.schema.json")))
	require.NoError(t, err, "vendored schema must exist")
	sum := sha256.Sum256(b)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(b, &raw), "schema must be valid JSON")
	return &aim07Schema{raw: raw, sha256: hex.EncodeToString(sum[:])}
}

func aim07App(checkDB, checkRedis func(context.Context) error) *fiber.App {
	app := fiber.New()
	app.Get("/health/ready", newHealthReadyHandler(checkDB, checkRedis))
	return app
}

func aim07Get(t *testing.T, app *fiber.App) (int, http.Header, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header, body
}

func aim07Enum(t *testing.T, prop any) []any {
	t.Helper()
	m, ok := prop.(map[string]any)
	require.True(t, ok, "schema property must be an object")
	e, ok := m["enum"].([]any)
	require.True(t, ok, "schema property must carry an enum")
	return e
}

// aim07Validate checks a response body against the vendored schema: exact
// top-level key set (additionalProperties: false + the seven required keys),
// service and status enums, the commit pattern, RFC 3339 checkedAt, and the
// closed shape of each dependency entry. Constraints are read from the schema
// file itself, so the test exercises the committed contract, not a copy.
func aim07Validate(t *testing.T, s *aim07Schema, rawBody []byte) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rawBody, &body), "body must be JSON: %s", rawBody)

	require.Equal(t, false, s.raw["additionalProperties"], "schema must close the top level")
	props, ok := s.raw["properties"].(map[string]any)
	require.True(t, ok)
	for key := range body {
		_, known := props[key]
		assert.True(t, known, "unexpected top-level key %q", key)
	}
	required, ok := s.raw["required"].([]any)
	require.True(t, ok)
	require.Len(t, required, 7, "schema must require the seven top-level keys")
	for _, key := range required {
		_, present := body[key.(string)]
		assert.True(t, present, "missing required top-level key %q", key)
	}

	_, isBool := body["ready"].(bool)
	assert.True(t, isBool, "ready must be a boolean, got %T", body["ready"])
	_, isBool = body["degraded"].(bool)
	assert.True(t, isBool, "degraded must be a boolean, got %T", body["degraded"])

	assert.Contains(t, aim07Enum(t, props["service"]), body["service"], "service must be in the schema enum")

	commitProp, ok := props["commit"].(map[string]any)
	require.True(t, ok)
	commitRe := regexp.MustCompile(commitProp["pattern"].(string))
	switch v := body["commit"].(type) {
	case nil:
	case string:
		assert.True(t, commitRe.MatchString(v), "commit %q must match %s", v, commitRe)
	default:
		t.Errorf("commit must be a string or null, got %T", v)
	}

	switch v := body["version"].(type) {
	case nil, string:
	default:
		t.Errorf("version must be a string or null, got %T", v)
	}

	checkedAt, ok := body["checkedAt"].(string)
	require.True(t, ok, "checkedAt must be a string")
	parsed, err := time.Parse(time.RFC3339, checkedAt)
	assert.NoError(t, err, "checkedAt must be RFC 3339")
	assert.True(t, strings.HasSuffix(checkedAt, "Z") || parsed.Location() == time.UTC, "checkedAt must be UTC: %q", checkedAt)

	depsProp, ok := props["dependencies"].(map[string]any)
	require.True(t, ok)
	entrySchema, ok := depsProp["additionalProperties"].(map[string]any)
	require.True(t, ok, "dependency entries must have a schema")
	require.Equal(t, false, entrySchema["additionalProperties"], "schema must close each dependency entry")
	entryProps, ok := entrySchema["properties"].(map[string]any)
	require.True(t, ok)
	statusEnum := aim07Enum(t, entryProps["status"])
	entryRequired, ok := entrySchema["required"].([]any)
	require.True(t, ok)

	deps, ok := body["dependencies"].(map[string]any)
	require.True(t, ok, "dependencies must be an object")
	for name, rawEntry := range deps {
		entry, ok := rawEntry.(map[string]any)
		require.True(t, ok, "dependency %q must be an object", name)
		for key := range entry {
			_, known := entryProps[key]
			assert.True(t, known, "dependency %q: unexpected key %q", name, key)
		}
		for _, key := range entryRequired {
			_, present := entry[key.(string)]
			assert.True(t, present, "dependency %q: missing required key %q", name, key)
		}
		assert.Contains(t, statusEnum, entry["status"], "dependency %q status must be in the schema enum", name)
		_, isBool := entry["required"].(bool)
		assert.True(t, isBool, "dependency %q: required must be a boolean", name)
		if v, present := entry["latencyMs"]; present {
			latency, isNum := v.(float64)
			assert.True(t, isNum && latency >= 0, "dependency %q: latencyMs must be a non-negative number, got %v", name, v)
		}
		if v, present := entry["reason"]; present {
			_, isStr := v.(string)
			assert.True(t, isStr, "dependency %q: reason must be a string", name)
		}
	}
}

func aim07DepStatus(t *testing.T, body []byte, name string) (status string, requiredFlag bool) {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	deps, ok := m["dependencies"].(map[string]any)
	require.True(t, ok)
	entry, ok := deps[name].(map[string]any)
	require.True(t, ok, "dependency %q missing", name)
	status, _ = entry["status"].(string)
	requiredFlag, _ = entry["required"].(bool)
	return status, requiredFlag
}

func aim07TopLevel(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	return m
}

var aim07CheckOK = func(context.Context) error { return nil }

func TestAIM07HealthReadyContract(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("AIM-07.AC3 vendored schema sha256 matches the pinned literal", func(t *testing.T) {
		assert.Equal(t, aim07SchemaSHA256, schema.sha256,
			"contracts/health-ready.schema.json changed; the vendored contract must stay byte-identical to the fleet copies")
	})

	t.Run("AIM-07.AC1 database ok, redis notConfigured -> 200 ready, not degraded", func(t *testing.T) {
		status, header, body := aim07Get(t, aim07App(aim07CheckOK, nil))
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "no-store", header.Get("Cache-Control"))
		aim07Validate(t, schema, body)

		m := aim07TopLevel(t, body)
		assert.Equal(t, true, m["ready"])
		assert.Equal(t, false, m["degraded"])
		assert.Equal(t, "agent-identity-management", m["service"])
		dbStatus, dbRequired := aim07DepStatus(t, body, "database")
		assert.Equal(t, "ok", dbStatus)
		assert.True(t, dbRequired)
		redisStatus, redisRequired := aim07DepStatus(t, body, "redis")
		assert.Equal(t, "notConfigured", redisStatus)
		assert.False(t, redisRequired)
	})

	t.Run("AIM-07.AC1 database unavailable -> 503 not ready", func(t *testing.T) {
		checkDB := func(context.Context) error { return errors.New("ping failed") }
		status, header, body := aim07Get(t, aim07App(checkDB, nil))
		require.Equal(t, http.StatusServiceUnavailable, status)
		assert.Equal(t, "no-store", header.Get("Cache-Control"))
		aim07Validate(t, schema, body)

		m := aim07TopLevel(t, body)
		assert.Equal(t, false, m["ready"])
		dbStatus, _ := aim07DepStatus(t, body, "database")
		assert.Equal(t, "unavailable", dbStatus)
	})

	t.Run("AIM-07.AC1 database ok, redis unavailable -> 200 degraded", func(t *testing.T) {
		checkRedis := func(context.Context) error { return errors.New("redis ping failed") }
		status, header, body := aim07Get(t, aim07App(aim07CheckOK, checkRedis))
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "no-store", header.Get("Cache-Control"))
		aim07Validate(t, schema, body)

		m := aim07TopLevel(t, body)
		assert.Equal(t, true, m["ready"])
		assert.Equal(t, true, m["degraded"])
		redisStatus, _ := aim07DepStatus(t, body, "redis")
		assert.Equal(t, "unavailable", redisStatus)
	})

	t.Run("AIM-07.AC1 database check never returns -> 503 within 5.5s", func(t *testing.T) {
		hang := func(context.Context) error { select {} } // ignores its context, never returns
		start := time.Now()
		status, header, body := aim07Get(t, aim07App(hang, nil))
		elapsed := time.Since(start)

		require.Equal(t, http.StatusServiceUnavailable, status)
		assert.Less(t, elapsed, 5500*time.Millisecond,
			"handler must answer within its own deadline even when a check hangs")
		assert.Equal(t, "no-store", header.Get("Cache-Control"))
		aim07Validate(t, schema, body)

		m := aim07TopLevel(t, body)
		assert.Equal(t, false, m["ready"])
		dbStatus, _ := aim07DepStatus(t, body, "database")
		assert.Equal(t, "unavailable", dbStatus)
	})
}

func TestAIM07CommitStamping(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("AIM-07.AC2 40-hex buildCommit is echoed verbatim", func(t *testing.T) {
		prev := buildCommit
		t.Cleanup(func() { buildCommit = prev })
		buildCommit = "0123456789abcdef0123456789abcdef01234567"

		status, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))
		require.Equal(t, http.StatusOK, status)
		aim07Validate(t, schema, body)
		assert.Equal(t, "0123456789abcdef0123456789abcdef01234567", aim07TopLevel(t, body)["commit"])
	})

	t.Run("AIM-07.AC2 empty buildCommit serialises as JSON null", func(t *testing.T) {
		prev := buildCommit
		t.Cleanup(func() { buildCommit = prev })
		buildCommit = ""

		_, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))
		aim07Validate(t, schema, body)
		assert.Contains(t, string(body), `"commit":null`)
	})

	t.Run("AIM-07.AC2 Dockerfile.backend declares ARG GIT_COMMIT and stamps main.buildCommit", func(t *testing.T) {
		b, err := os.ReadFile(aim07Path(t, filepath.Join("..", "..", "infrastructure", "docker", "Dockerfile.backend")))
		require.NoError(t, err)
		src := string(b)

		argIdx := strings.Index(src, "ARG GIT_COMMIT")
		ldflagIdx := strings.Index(src, "-X main.buildCommit=")
		require.GreaterOrEqual(t, argIdx, 0, "Dockerfile.backend must declare ARG GIT_COMMIT")
		require.GreaterOrEqual(t, ldflagIdx, 0, "Dockerfile.backend must stamp -X main.buildCommit=")
		assert.Less(t, argIdx, ldflagIdx, "ARG GIT_COMMIT must precede the build step that uses it")
	})
}

func TestAIM07NegativeGuarantees(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("AIM-07.AC4 reason is a fixed string, driver error text never leaks", func(t *testing.T) {
		driverErr := errors.New("dial tcp db.internal:5432: connection refused")
		status, _, body := aim07Get(t, aim07App(func(context.Context) error { return driverErr }, nil))
		require.Equal(t, http.StatusServiceUnavailable, status)
		aim07Validate(t, schema, body)

		raw := string(body)
		assert.NotContains(t, raw, "db.internal")
		assert.NotContains(t, raw, "5432")
		assert.NotContains(t, raw, "connection refused")
	})

	t.Run("AIM-07.AC4 GET /health is unchanged byte for byte and no /health/live route exists", func(t *testing.T) {
		b, err := os.ReadFile(aim07Path(t, "main.go"))
		require.NoError(t, err)
		src := string(b)

		assert.Contains(t, src, aim07HealthHandlerSource,
			"the GET /health handler must remain exactly as at base 5405500a")
		assert.NotContains(t, src, "/health/live", "no /health/live route may be added")
	})
}
