//go:build integration

package middleware

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// Revocation enforcement on the agent auth middlewares.
//
// Before this suite existed, `AgentService.RevokeAgent` and `EnforceKeyExpiry`
// expressed denial ONLY as a write to `agents.status`, and three of the six auth
// middlewares never read that column. A revoked or suspended agent that still held
// its key material authenticated successfully — on `secrets resolve` among other
// routes in the public backend, and on every SDK route in aim-cloud, which mounts
// Ed25519AgentMiddleware where the public backend mounts the PQC one.
//
// These tests drive the REAL repository against a real Postgres and the REAL fiber
// middleware, with real Ed25519 signatures over the exact message the middleware
// reconstructs. There is no injectable seam on `*application.AgentService`, and a
// faked one would not have caught the defect anyway: the bug was that the status
// column was never read, which only a real row can demonstrate.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the AIM
// schema applied (run migrations first).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAgentRevocation ./internal/interfaces/http/middleware/...

// The statuses under test, and what each must do.
//
// Written as literals, not derived from the domain constants, so this table states
// the contract independently of the code it checks. "deactivated" is not a status
// the system has — it stands for any value a future migration or a hand-run UPDATE
// could put in the column, which `agents.status` permits because it carries no
// CHECK constraint. A deny-list implementation authenticates it; an allow-list
// denies it.
var revocationCases = []struct {
	status     string
	shouldAuth bool
	why        string
}{
	{"verified", true, "the ordinary case — also proves the request is otherwise well-formed"},
	{"pending", true, "registration default; denying it would break enrollment"},
	{"revoked", false, "RevokeAgent writes this and nothing else denied the request"},
	{"suspended", false, "SuspendAgent and EnforceKeyExpiry write this"},
	{"deactivated", false, "unrecognised value must fail closed, not fall through"},
}

func revocationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping revocation enforcement test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedAgentWithStatus creates the organization, user and agent rows an auth request
// needs, with the agent in `status` and holding `publicKeyB64`.
//
// This mirrors the shape of `seedOrgAndUser` in internal/application rather than
// reusing it: that helper is unexported in a different package. The NOT NULL and FK
// requirements it documents are honoured here, and the same rule applies — a future
// column requirement gets fixed in this one helper, not per test.
func seedAgentWithStatus(t *testing.T, db *sql.DB, ctx context.Context, status, publicKeyB64 string) uuid.UUID {
	t.Helper()

	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	suffix := agentID.String()[:8]

	// Child-first, so the foreign keys hold on the way out.
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM api_keys WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "revocation-org-"+suffix, "revocation-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "revocation-"+suffix+"@example.com", "revocation-user", "local-"+suffix)
	require.NoError(t, err)

	// `description` is set explicitly so a green run means the status check ran, rather
	// than the row being unreadable for an unrelated reason.
	//
	// It used to be load-bearing: GetByID scanned this nullable column into a plain
	// string, so a NULL failed the whole lookup and the middleware reported it as
	// "Agent not found" — indistinguishable from a revocation denial. That is fixed
	// (GetByID now scans it through a NullString, covered by
	// TestNullDescriptionIsReadableByEveryAgentReadPath in the repository package), and
	// working around it here is what kept any test from catching it for so long.
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'ai_agent', $6, $7, 0.5, $8, NOW(), NOW())`,
		agentID, orgID, "revocation-agent-"+suffix, "Revocation Agent",
		"seeded by the revocation enforcement suite", status, publicKeyB64, userID)
	require.NoError(t, err,
		"seeding status %q failed — agents.status carries no CHECK constraint, so "+
			"even a value outside the domain constants must insert", status)

	return agentID
}

// signedGet runs one real Ed25519-signed GET through `app` and returns the status
// code and body. The message is the exact one both signature middlewares
// reconstruct: METHOD\nOriginalURL\ntimestamp, with no body.
func signedGet(t *testing.T, app *fiber.App, agentID uuid.UUID, pub ed25519.PublicKey, priv ed25519.PrivateKey) (int, string) {
	t.Helper()

	const path = "/protected"
	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	message := fmt.Sprintf("GET\n%s\n%s", path, timestampStr)

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Agent-ID", agentID.String())
	req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(message))))
	req.Header.Set("X-Timestamp", timestampStr)
	req.Header.Set("X-Public-Key", base64.StdEncoding.EncodeToString(pub))

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

// newSignatureApp mounts one of the two signature middlewares over a real
// DB-backed AgentService. Only agentRepo is exercised — GetAgent delegates
// straight to it — so the remaining dependencies are nil by design.
func newSignatureApp(db *sql.DB, mount func(*application.AgentService) fiber.Handler) *fiber.App {
	svc := application.NewAgentService(
		repository.NewAgentRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	app := fiber.New()
	app.Use(mount(svc))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"reached": true})
	})
	return app
}

// assertSignatureMiddlewareEnforcesStatus is the shared body for both signature
// middlewares. Both directions are asserted for every status: a denial is only
// counted when the body names the status, because a 401 for some unrelated reason
// (bad signature, clock skew, missing key) would otherwise read as enforcement
// that is not there.
func assertSignatureMiddlewareEnforcesStatus(t *testing.T, mount func(*application.AgentService) fiber.Handler) {
	db := revocationTestDB(t)
	ctx := context.Background()

	for _, tc := range revocationCases {
		t.Run(tc.status, func(t *testing.T) {
			pub, priv, err := ed25519.GenerateKey(nil)
			require.NoError(t, err)
			pubB64 := base64.StdEncoding.EncodeToString(pub)

			agentID := seedAgentWithStatus(t, db, ctx, tc.status, pubB64)
			app := newSignatureApp(db, mount)

			code, body := signedGet(t, app, agentID, pub, priv)

			if tc.shouldAuth {
				assert.Equal(t, fiber.StatusOK, code,
					"status %q must authenticate (%s); body: %s", tc.status, tc.why, body)
				assert.Contains(t, body, "reached",
					"the handler must actually run for status %q", tc.status)
				return
			}

			assert.Equal(t, fiber.StatusUnauthorized, code,
				"status %q must be denied (%s); body: %s", tc.status, tc.why, body)
			assert.NotContains(t, body, "reached",
				"the handler ran for status %q — the request was authenticated", tc.status)
			assert.Contains(t, body, tc.status,
				"status %q was denied, but not for being %q — the 401 came from some "+
					"other check, so this asserts nothing about revocation. Body: %s",
				tc.status, tc.status, body)
		})
	}
}

// aim-cloud mounts this one on every SDK route, so it is live in the deployed SaaS
// even though the public backend has no non-test call site for it.
func TestAgentRevocation_Ed25519Middleware(t *testing.T) {
	assertSignatureMiddlewareEnforcesStatus(t, Ed25519AgentMiddleware)
}

// The public backend mounts this one, including on secrets resolve.
func TestAgentRevocation_PQCMiddleware(t *testing.T) {
	assertSignatureMiddlewareEnforcesStatus(t, PQCAgentMiddleware)
}

// seedAPIKey inserts a real api_keys row for `agentID` and returns the plaintext
// key. The hash is computed the way the middleware computes it, so the row is
// reachable by an actual request rather than by a value the test asserts about
// itself.
func seedAPIKey(t *testing.T, db *sql.DB, ctx context.Context, agentID, orgID, userID uuid.UUID) string {
	t.Helper()

	plaintext := "aim_live_revocation_" + uuid.New().String()
	sum := sha256.Sum256([]byte(plaintext))

	_, err := db.ExecContext(ctx,
		`INSERT INTO api_keys (id, organization_id, agent_id, name, key_hash, prefix,
		                       is_active, created_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, TRUE, NOW(), $7)`,
		uuid.New(), orgID, agentID, "revocation-key",
		base64.StdEncoding.EncodeToString(sum[:]), plaintext[:8], userID)
	require.NoError(t, err)

	return plaintext
}

// agentOwners reads back the org and creator of a seeded agent, so the API key row
// can satisfy its own foreign keys without seedAgentWithStatus having to return
// four values for the callers that do not need them.
func agentOwners(t *testing.T, db *sql.DB, ctx context.Context, agentID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var orgID, userID uuid.UUID
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT organization_id, created_by FROM agents WHERE id = $1`, agentID).
		Scan(&orgID, &userID))
	return orgID, userID
}

// APIKeyMiddleware: an API key is bound to exactly one agent by a NOT NULL FK, and
// `is_active` revokes the KEY, not the agent. RevokeAgent never touches the agent's
// keys, so before the join every key issued before a revocation kept working.
func TestAgentRevocation_APIKeyMiddleware(t *testing.T) {
	db := revocationTestDB(t)
	ctx := context.Background()

	for _, tc := range revocationCases {
		t.Run(tc.status, func(t *testing.T) {
			agentID := seedAgentWithStatus(t, db, ctx, tc.status, "")
			orgID, userID := agentOwners(t, db, ctx, agentID)
			key := seedAPIKey(t, db, ctx, agentID, orgID, userID)

			app := fiber.New()
			app.Use(APIKeyMiddleware(db))
			app.Get("/protected", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"reached": true})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("X-API-Key", key)

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tc.shouldAuth {
				assert.Equal(t, fiber.StatusOK, resp.StatusCode,
					"status %q must authenticate (%s); body: %s", tc.status, tc.why, body)
				assert.Contains(t, string(body), "reached")
				return
			}

			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
				"status %q must be denied (%s); body: %s", tc.status, tc.why, body)
			assert.NotContains(t, string(body), "reached",
				"the handler ran with a key belonging to a %q agent", tc.status)
			assert.Contains(t, string(body), tc.status,
				"denied, but not for being %q — the 401 came from some other check "+
					"(the key is active and unexpired, so it should not have). Body: %s",
				tc.status, body)
		})
	}
}

// The other two fields APIKeyMiddleware is responsible for. `is_active` had only a
// sqlmock-backed negative test and `expires_at` had none at all, so the enforcement
// matrix for this middleware was one-third proven. Asserted here against real rows,
// alongside a positive control, so a middleware that dropped either check would fail.
func TestAgentRevocation_APIKeyMiddlewareEnforcesKeyFields(t *testing.T) {
	db := revocationTestDB(t)
	ctx := context.Background()

	// .UTC() is load-bearing, and its absence found a real defect worth naming here.
	// `api_keys.expires_at` is TIMESTAMP, not TIMESTAMPTZ. lib/pq sends a local-zone
	// time.Time as its wall clock, Postgres drops the offset, and the value reads back
	// labelled UTC — so a time written from a UTC-6 process comes back six hours early,
	// and one written from UTC+2 comes back two hours LATE, i.e. the key outlives its
	// stated expiry. `time.Now().Add(time.Hour)` here failed the positive control for
	// exactly that reason. Writing UTC sidesteps it; the column is still wrong, and
	// api_key_service.CreateAPIKey writes a local time. Tracked separately — fixing it
	// is a migration on a production table, not part of a revocation change.
	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(1 * time.Hour)

	for _, tc := range []struct {
		name       string
		active     bool
		expiresAt  *time.Time
		wantStatus int
		wantErr    string
	}{
		{"control-active-unexpired", true, nil, fiber.StatusOK, ""},
		{"control-active-future-expiry", true, &future, fiber.StatusOK, ""},
		{"inactive", false, nil, fiber.StatusUnauthorized, "inactive"},
		{"expired", true, &past, fiber.StatusUnauthorized, "expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentID := seedAgentWithStatus(t, db, ctx, "verified", "")
			orgID, userID := agentOwners(t, db, ctx, agentID)
			key := seedAPIKey(t, db, ctx, agentID, orgID, userID)

			_, err := db.ExecContext(ctx,
				`UPDATE api_keys SET is_active = $1, expires_at = $2 WHERE agent_id = $3`,
				tc.active, tc.expiresAt, agentID)
			require.NoError(t, err)

			app := fiber.New()
			app.Use(APIKeyMiddleware(db))
			app.Get("/protected", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"reached": true})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("X-API-Key", key)

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatus, resp.StatusCode, "body: %s", body)
			if tc.wantErr == "" {
				assert.Contains(t, string(body), "reached")
				return
			}
			assert.NotContains(t, string(body), "reached")
			// The reason is asserted, not just the code: the agent here is
			// `verified`, so a 401 naming a status would mean the wrong check fired.
			assert.Contains(t, string(body), tc.wantErr,
				"denied, but not for being %s", tc.wantErr)
		})
	}
}

// OptionalAPIKeyMiddleware is optional-auth: a denied agent must continue WITHOUT
// auth context rather than 401, matching how it already handles an inactive key.
// The assertion is on the context it sets, not on the status code — a 200 here
// means nothing on its own, since the middleware always calls Next().
func TestAgentRevocation_OptionalAPIKeyMiddleware(t *testing.T) {
	db := revocationTestDB(t)
	ctx := context.Background()

	for _, tc := range revocationCases {
		t.Run(tc.status, func(t *testing.T) {
			agentID := seedAgentWithStatus(t, db, ctx, tc.status, "")
			orgID, userID := agentOwners(t, db, ctx, agentID)
			key := seedAPIKey(t, db, ctx, agentID, orgID, userID)

			var sawAuthMethod string
			var sawAgentID any

			app := fiber.New()
			app.Use(OptionalAPIKeyMiddleware(db))
			app.Get("/protected", func(c fiber.Ctx) error {
				sawAuthMethod, _ = c.Locals("auth_method").(string)
				sawAgentID = c.Locals("agent_id")
				return c.JSON(fiber.Map{"reached": true})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("X-API-Key", key)

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, fiber.StatusOK, resp.StatusCode,
				"optional auth always reaches the handler")

			if tc.shouldAuth {
				assert.Equal(t, "api_key", sawAuthMethod,
					"status %q must still receive auth context (%s)", tc.status, tc.why)
				assert.Equal(t, agentID, sawAgentID)
				return
			}

			assert.Empty(t, sawAuthMethod,
				"status %q was given auth context — downstream handlers will treat the "+
					"request as an authenticated %q agent (%s)", tc.status, tc.status, tc.why)
			assert.Nil(t, sawAgentID,
				"agent_id was set for a %q agent", tc.status)
		})
	}
}
