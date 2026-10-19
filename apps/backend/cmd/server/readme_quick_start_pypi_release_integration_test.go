//go:build integration

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// pypiQuickStartSignInSection is the docs/quick-start.md section the README's
// Quick start links for a self-hosted AIM, and pypiQuickStartCopy is the
// command in it that gives the release on PyPI its sign-in: the archive's
// sdk_credentials.json copied to ~/.aim/sdk_credentials.json, the first file
// that release reads a sign-in from.
const pypiQuickStartSignInSection = "### Sign in from your dashboard"

var pypiQuickStartCopy = regexp.MustCompile(`(?m)^mkdir -p ~/\.aim && cp (\S+) ~/\.aim/sdk_credentials\.json$`)

// The root README's Quick start on a self-hosted AIM, sent through this
// server's route table as aim-sdk 2.0.3, the release on PyPI, sends it.
//
// That release has no device sign-in: its `aim-sdk login --url` opens
// <url>/auth/login and exchanges a code at /api/v1/auth/token, and this server
// registers neither. It signs in with the sign-in the dashboard's SDK
// download carries once that file is at ~/.aim/sdk_credentials.json, which
// is the step docs/quick-start.md gives under "Sign in from your dashboard".
// Each request below is one that release sends, with its headers, and with
// its body byte for byte where it signs the body:
//
//   - secure("my-first-agent", capabilities=["db:read"]) presents the refresh
//     token, asks whether the agent exists, and registers it with a key pair it
//     generated, as the signed-in user;
//   - an administrator verifies the agent under Agents;
//   - @agent.perform_action(capability="db:read") registers the capability with
//     an Ed25519-signed request, then sends the signed verification, which is
//     approved and the function body runs;
//   - after Global Enforcement Mode is set to Strict, the decorated db:write
//     call is refused with the reason the README quotes.
//
// The server is built as main builds it: initServices, initHandlers and
// mountAPIRoutes over this database, with no Redis.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration -p 4 \
//	  -run TestReadmeQuickStartAsThePyPIReleaseSendsIt ./cmd/server/
func TestReadmeQuickStartAsThePyPIReleaseSendsIt(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping the README Quick start integration test")
	}

	// The step the README's Quick start links: the archive member it copies.
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "quick-start.md"))
	require.NoError(t, err)
	section := markdownSection(t, string(doc), pypiQuickStartSignInSection)
	copied := pypiQuickStartCopy.FindStringSubmatch(section)
	require.NotNil(t, copied, "docs/quick-start.md gives the release on PyPI no sign-in under %q: no line copies an "+
		"archive's sdk_credentials.json to ~/.aim/sdk_credentials.json", pypiQuickStartSignInSection)
	member := copied[1]

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	// The quickstart server's configuration: no Registry bridge and no Redis,
	// secrets of its own, and the SDK download built from this repository's
	// sdk/ tree, as the server image builds it from the copy it carries.
	withEnvironment(t, "", false)
	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "sdk"))
	require.NoError(t, err)
	cfg := loadConfigWithoutRegistryVariables(t, map[string]string{
		"JWT_SECRET":          hex.EncodeToString(randomTestBytes(t, 32)),
		"KEYVAULT_MASTER_KEY": base64.StdEncoding.EncodeToString(randomTestBytes(t, 32)),
		"REDIS_HOST":          "",
		"SDK_BASE_DIR":        sdkDir,
	})

	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	orgID, userID := uuid.New(), uuid.New()
	email := "readme-pypi-" + suffix + "@example.invalid"
	password := "Pw-" + uuid.NewString()
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
		orgID, "readme-pypi-"+suffix, "readme-pypi-"+suffix+".invalid")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
		VALUES ($1, $2, $3, 'readme quick start admin', 'local', $4, 'admin', $5, 'active')`,
		userID, orgID, email, userID.String(), hash)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM agent_capabilities WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`,
			`DELETE FROM capability_requests WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`,
			`DELETE FROM trust_scores WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`,
			`DELETE FROM capability_violations WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`,
			`DELETE FROM verification_events WHERE organization_id = $1`,
			`DELETE FROM alerts WHERE organization_id = $1`,
			`DELETE FROM capability_definitions WHERE organization_id = $1`,
			`DELETE FROM audit_logs WHERE organization_id = $1`,
			`DELETE FROM agents WHERE organization_id = $1`,
			`DELETE FROM sdk_tokens WHERE organization_id = $1`,
			`DELETE FROM users WHERE organization_id = $1`,
			`DELETE FROM organizations WHERE id = $1`,
		} {
			if _, err := db.ExecContext(ctx, statement, orgID); err != nil {
				t.Logf("cleanup %q: %v", statement, err)
			}
		}
	})

	repos, oauthRepo := initRepositories(db)
	jwtService := auth.NewJWTService()
	jwtService.SetSessionStateReader(repos.User)
	services, keyVault := initServices(cfg, db, repos, nil, oauthRepo, jwtService, nil)
	t.Cleanup(func() {
		services.AlertChannel.Stop()
		_ = services.FGA.Shutdown(ctx)
	})
	h := initHandlers(services, repos, jwtService, keyVault, cfg, db)
	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, services, jwtService, repos.SDKToken, db)

	send := func(method, target string, headers map[string]string, body string) (int, []byte) {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, target, reader)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		// A password check takes longer than app.Test's default second under -race.
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, raw
	}
	decode := func(raw []byte) map[string]any {
		t.Helper()
		var out map[string]any
		require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
		return out
	}
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		require.NoError(t, err)
		return string(raw)
	}

	// The dashboard: the administrator signs in and clicks "Download Python
	// SDK"; the docs' step copies the archive member to ~/.aim.
	status, raw := send(fiber.MethodPost, "/api/v1/auth/login/local", map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":%s,"password":%s}`, quoted(email), quoted(password)))
	require.Equal(t, fiber.StatusOK, status, "dashboard sign-in: %s", raw)
	dashboard, _ := decode(raw)["accessToken"].(string)
	require.NotEmpty(t, dashboard)
	admin := map[string]string{"Authorization": "Bearer " + dashboard, "Content-Type": "application/json"}

	status, archive := send(fiber.MethodGet, "/api/v1/sdk/download?sdk=python", map[string]string{"Authorization": "Bearer " + dashboard}, "")
	require.Equal(t, fiber.StatusOK, status, "SDK download: %s", archive)
	zipped, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	var signIn map[string]any
	for _, f := range zipped.File {
		if f.Name != member {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		contents, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		signIn = decode(contents)
	}
	require.NotNil(t, signIn, "the SDK download holds no %s, the file docs/quick-start.md copies", member)
	// The members the release on PyPI reads from it: the server URL, the
	// refresh token it presents, and the token id it sends as X-SDK-Token.
	for _, name := range []string{"aimUrl", "refreshToken", "sdkTokenId"} {
		value, _ := signIn[name].(string)
		require.NotEmpty(t, value, "the sign-in %s carries no %s", member, name)
	}
	sdkTokenID := signIn["sdkTokenId"].(string)

	// secure(): the refresh, as OAuthTokenManager sends it, with no other header.
	status, raw = send(fiber.MethodPost, "/api/v1/auth/refresh", map[string]string{"Content-Type": "application/json"},
		`{"refreshToken": `+quoted(signIn["refreshToken"].(string))+`}`)
	require.Equal(t, fiber.StatusOK, status, "the refresh of the downloaded sign-in: %s", raw)
	refreshed := decode(raw)
	access, _ := refreshed["accessToken"].(string)
	require.NotEmpty(t, access, "the release on PyPI reads accessToken from the refresh")
	rotated, _ := refreshed["refreshToken"].(string)
	require.NotEmpty(t, rotated, "the release on PyPI saves refreshToken from the refresh")
	user := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + access, "X-SDK-Token": sdkTokenID}

	// It connects to an agent of that name when this answers 200; on a fresh
	// organization it must not.
	status, raw = send(fiber.MethodGet, "/api/v1/sdk-api/agents/my-first-agent", user, "")
	require.NotEqual(t, fiber.StatusOK, status, "a fresh organization reports an agent named my-first-agent: %s", raw)

	// The registration, with the key pair the release on PyPI generates.
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)
	status, raw = send(fiber.MethodPost, "/api/v1/agents", user, `{"name": "my-first-agent", "displayName": "my-first-agent", `+
		`"description": "Agent my-first-agent registered via AIM SDK", "agentType": "ai_agent", "version": "1.0.0", `+
		`"communityIntelligenceOptIn": false, "capabilities": ["db:read"], "publicKey": `+quoted(publicKeyB64)+`}`)
	require.Contains(t, []int{fiber.StatusOK, fiber.StatusCreated}, status, "the registration: %s", raw)
	created := decode(raw)
	agentID, _ := created["id"].(string)
	require.NotEmpty(t, agentID, "the release on PyPI reads the agent's id from the registration: %s", raw)
	assert.Equal(t, "pending", created["status"], "the README shows the agent pending")
	// It keeps the key the server answers with, and signs with the one it generated.
	for _, name := range []string{"public_key", "publicKey"} {
		if answered, ok := created[name]; ok {
			assert.Equal(t, publicKeyB64, answered, "the registration answers a %s other than the one sent", name)
		}
	}
	var rowOrg uuid.UUID
	var rowKey sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `SELECT organization_id, public_key FROM agents WHERE id = $1`, agentID).Scan(&rowOrg, &rowKey))
	assert.Equal(t, orgID, rowOrg, "the agent is registered in the signed-in user's organization")
	assert.Equal(t, publicKeyB64, rowKey.String, "the agent's stored key is the one the release on PyPI signs with")

	// Signed as AIMClient._make_request signs: method, path, timestamp and the
	// body as json.dumps(sort_keys=True) writes it, in the X- headers.
	signed := func(path, body string) map[string]string {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := ed25519.Sign(privateKey, []byte(strings.Join([]string{fiber.MethodPost, path, timestamp, body}, "\n")))
		return map[string]string{
			"Content-Type": "application/json",
			"X-Agent-ID":   agentID,
			"X-Signature":  base64.StdEncoding.EncodeToString(signature),
			"X-Timestamp":  timestamp,
			"X-Public-Key": publicKeyB64,
		}
	}
	// What @agent.perform_action sends before the function body runs: the
	// capability registration it files, then the verification, signed in its
	// body over the action_type, agent_id, context, resource and timestamp.
	decorated := func(capability, function, risk string) (int, []byte, int, map[string]any) {
		t.Helper()
		path := "/api/v1/sdk-api/agents/" + agentID + "/capabilities/register"
		body := fmt.Sprintf(`{"capabilityType": %s, "description": %s, "riskLevel": %s}`,
			quoted(capability), quoted("Auto-registered by @perform_action decorator for "+function), quoted(risk))
		registered, registration := send(fiber.MethodPost, path, signed(path, body), body)

		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
		message := fmt.Sprintf(`{"action_type": %s, "agent_id": %s, "context": {}, "resource": null, "timestamp": %s}`,
			quoted(capability), quoted(agentID), quoted(timestamp))
		verification, err := json.Marshal(map[string]any{
			"agentId":    agentID,
			"capability": capability,
			"resource":   nil,
			"context":    map[string]any{},
			"timestamp":  timestamp,
			"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(message))),
			"publicKey":  publicKeyB64,
		})
		require.NoError(t, err)
		verified, answer := send(fiber.MethodPost, "/api/v1/sdk-api/verifications",
			map[string]string{"Content-Type": "application/json", "User-Agent": "AIM-Python-SDK/2.0.3"}, string(verification))
		return registered, registration, verified, decode(answer)
	}

	// The administrator verifies the agent under Agents.
	status, raw = send(fiber.MethodPost, "/api/v1/agents/"+agentID+"/verify", admin, "")
	require.Equal(t, fiber.StatusOK, status, "the administrator's verification: %s", raw)

	// get_customer(42): db:read was granted at registration, which the release
	// on PyPI counts as registered on a 409, and the call is approved.
	registered, registration, verified, answer := decorated("db:read", "get_customer", "low")
	assert.Contains(t, []int{fiber.StatusOK, fiber.StatusCreated, fiber.StatusConflict}, registered,
		"the signed db:read registration: %s", registration)
	require.Equal(t, fiber.StatusCreated, verified, "the db:read verification: %v", answer)
	assert.Empty(t, answer["denialReason"], "the db:read call is refused: %v", answer)

	// The README's strict-mode step, then delete_customer(42): the
	// registration waits for an administrator and the call is refused.
	status, raw = send(fiber.MethodPut, "/api/v1/admin/enforcement-settings", admin, `{"enforcementMode": "strict"}`)
	require.Equal(t, fiber.StatusOK, status, "strict mode: %s", raw)
	registered, registration, verified, answer = decorated("db:write", "delete_customer", "medium")
	assert.Equal(t, fiber.StatusAccepted, registered, "the signed db:write registration: %s", registration)
	require.Equal(t, fiber.StatusForbidden, verified, "the db:write verification: %v", answer)
	assert.Equal(t, "denied", answer["status"])
	reason, _ := answer["denialReason"].(string)
	assert.True(t, strings.HasSuffix(reason, ": Agent does not have permission for capability 'db:write' (allowed: [db:read])"),
		"the refusal does not carry the reason the README quotes: %q", reason)
}

// markdownSection returns the lines under heading up to the next heading of
// the same or a higher level; lines inside fenced code are never headings.
func markdownSection(t *testing.T, doc, heading string) string {
	t.Helper()
	level := strings.IndexFunc(heading, func(r rune) bool { return r != '#' })
	lines := strings.Split(doc, "\n")
	start, inFence := -1, false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if start < 0 {
			if strings.TrimSpace(line) == heading {
				start = i + 1
			}
			continue
		}
		if hashes := strings.IndexFunc(line, func(r rune) bool { return r != '#' }); hashes > 0 && hashes <= level && strings.HasPrefix(line[hashes:], " ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	require.GreaterOrEqual(t, start, 0, "docs/quick-start.md has no heading %q", heading)
	return strings.Join(lines[start:], "\n")
}

func randomTestBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}
