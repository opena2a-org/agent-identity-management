//go:build integration

package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The README Quick start's refusal step, on the organization a fresh quickstart
// stack creates.
//
// The README shows a decorated db:write call refused for an agent that holds
// db:read. A quickstart stack's organization is the one migration 013 inserts,
// with no enforcement_mode, so it takes the column default from migration 054:
// monitoring. In monitoring mode the capability registration the SDK files
// before the call is auto-granted (RegisterCapability), so the write runs. The
// refusal needs strict mode, and the README names the one step that turns it
// on: Security, then Policies, then Global Enforcement Mode, which is
// PUT /api/v1/admin/enforcement-settings.
//
// That organization also carries no security policy. Migration 015 seeds its
// policies only when the admin user already exists, and on a fresh stack the
// admin is seeded after the migrations run, by the bootstrap binary (measured
// on a database migrated with cmd/migrate and seeded with
// cmd/bootstrap --default: zero security_policies rows). With no
// capability_violation policy the refusal names the built-in default_policy.
//
// This suite drives the real handlers against a real Postgres through that
// sequence: on defaults the mode reads monitoring and the registration is
// granted; after the README's step the mode reads strict, the same
// registration files a pending request, and the agent still holds db:read
// only. TestQuickstartRefusalRoundTrip then sends the signed verification the
// decorator sends and asserts the refusal itself, the reason the README
// quotes, and the audit row that records it.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the
// AIM schema applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestQuickstart.*RoundTrip' ./internal/interfaces/http/handlers/...

func quickstartEnforcementTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping quickstart enforcement integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedQuickstartOrganization inserts an organization in the state a fresh
// quickstart stack's organization is in (enforcement_mode left to the column
// default, no security policy rows), plus one admin user. Rows are removed
// child-first.
func seedQuickstartOrganization(t *testing.T, db *sql.DB, ctx context.Context) (uuid.UUID, uuid.UUID) {
	t.Helper()

	suffix := uuid.New().String()[:8]
	var orgID uuid.UUID
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO organizations (name, domain, plan_type, max_agents, max_users, is_active)
		 VALUES ($1, $2, 'enterprise', 1000, 100, true)
		 RETURNING id`,
		"quickstart-org-"+suffix, "quickstart-"+suffix+".example.com").Scan(&orgID))

	userID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_capabilities WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_requests WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_violations WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM verification_events WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_definitions WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "quickstart-"+suffix+"@example.com", "quickstart-admin", "local-"+suffix)
	require.NoError(t, err)

	return orgID, userID
}

// seedQuickstartAgent inserts a verified agent that holds db:read only, the
// README's my-first-agent.
func seedQuickstartAgent(t *testing.T, db *sql.DB, ctx context.Context, capRepo *repository.CapabilityRepositoryPostgres, orgID, userID uuid.UUID) uuid.UUID {
	t.Helper()

	agentID := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'ai_agent', 'verified', 'unused-for-this-endpoint', 0.5, $6, NOW(), NOW())`,
		agentID, orgID, "my-first-agent-"+agentID.String()[:8], "My First Agent",
		"seeded by the quickstart enforcement suite", userID)
	require.NoError(t, err)

	require.NoError(t, capRepo.CreateCapability(&domain.AgentCapability{
		AgentID:        agentID,
		CapabilityType: "db:read",
		GrantedBy:      &userID,
	}))
	return agentID
}

type quickstartStack struct {
	app     *fiber.App
	capRepo *repository.CapabilityRepositoryPostgres
}

// newQuickstartStack mounts the real handlers the README's sequence reaches:
// the admin enforcement settings, the SDK capability registration and the SDK
// verification the decorator sends before the function body runs. The
// middleware stands in for the JWT and agent-signature middleware, which set
// the same locals from the authenticated caller; the verification route is a
// bare mount that checks the agent's Ed25519 signature itself. The services
// behind the verification come from the constructors cmd/server uses, without
// the key vault and the FGA engine, and the security policy repository reads
// the organization's real (empty) policy set.
func newQuickstartStack(db *sql.DB, orgID, userID uuid.UUID) *quickstartStack {
	sqlxDB := sqlx.NewDb(db, "postgres")
	agentRepo := repository.NewAgentRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	capRepo := repository.NewCapabilityRepository(sqlxDB)

	alertRepo := repository.NewAlertRepository(db)
	trustScoreRepo := repository.NewTrustScoreRepository(db)
	trustCalc := application.NewTrustCalculator(trustScoreRepo, repository.NewAPIKeyRepository(db), auditRepo, capRepo, agentRepo, alertRepo)

	capSvc := application.NewCapabilityService(capRepo, agentRepo, auditRepo, alertRepo, trustCalc, trustScoreRepo)
	capReqSvc := application.NewCapabilityRequestService(repository.NewCapabilityRequestRepository(sqlxDB), capRepo, agentRepo, orgRepo)
	capHandler := NewCapabilityHandler(capSvc, capReqSvc, agentRepo, orgRepo)
	adminHandler := NewAdminHandler(nil, application.NewAdminService(userRepo, orgRepo), nil, nil, application.NewAuditService(auditRepo), nil, nil, nil, userRepo)

	verificationEventRepo := repository.NewVerificationEventRepository(db)
	policySvc := application.NewSecurityPolicyService(repository.NewSecurityPolicyRepository(db), alertRepo, auditRepo)
	policySvc.SetVerificationEventRepo(verificationEventRepo)
	verifyTrustCalc := application.NewTrustCalculatorWithVerification(trustScoreRepo, repository.NewAPIKeyRepository(db), auditRepo, capRepo, agentRepo, alertRepo, verificationEventRepo)
	verificationEventSvc := application.NewVerificationEventService(verificationEventRepo, agentRepo, application.NewDriftDetectionService(agentRepo, alertRepo))
	agentSvc := application.NewAgentService(agentRepo, verifyTrustCalc, trustScoreRepo, nil, alertRepo, policySvc, capRepo,
		verificationEventSvc, repository.NewTagRepository(db), userRepo, orgRepo, capReqSvc)
	verificationHandler := NewVerificationHandler(agentSvc, application.NewAuditService(auditRepo),
		application.NewAlertService(alertRepo, agentRepo, db), verifyTrustCalc, verificationEventSvc, orgRepo, nil)

	app := fiber.New()
	app.Post("/api/v1/sdk-api/verifications", verificationHandler.CreateVerification)
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Get("/api/v1/admin/enforcement-settings", adminHandler.GetEnforcementSettings)
	app.Put("/api/v1/admin/enforcement-settings", adminHandler.UpdateEnforcementSettings)
	app.Post("/api/v1/sdk-api/agents/:id/capabilities/register", capHandler.RegisterCapability)

	return &quickstartStack{app: app, capRepo: capRepo}
}

func (s *quickstartStack) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.app.Test(req, fiber.TestConfig{Timeout: 0})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	return resp.StatusCode, out
}

func (s *quickstartStack) enforcementMode(t *testing.T) string {
	t.Helper()
	status, body := s.do(t, "GET", "/api/v1/admin/enforcement-settings", nil)
	require.Equal(t, fiber.StatusOK, status, "body: %v", body)
	mode, _ := body["enforcementMode"].(string)
	return mode
}

func (s *quickstartStack) registerDBWrite(t *testing.T, agentID uuid.UUID) (int, map[string]any) {
	t.Helper()
	return s.do(t, "POST", "/api/v1/sdk-api/agents/"+agentID.String()+"/capabilities/register",
		map[string]any{"capabilityType": "db:write", "description": "Registered capability: db:write", "riskLevel": "medium"})
}

func (s *quickstartStack) activeCapabilities(t *testing.T, agentID uuid.UUID) []string {
	t.Helper()
	caps, err := s.capRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	types := make([]string, 0, len(caps))
	for _, c := range caps {
		types = append(types, c.CapabilityType)
	}
	return types
}

func TestQuickstartEnforcementModeRoundTrip(t *testing.T) {
	db := quickstartEnforcementTestDB(t)
	ctx := context.Background()

	orgID, userID := seedQuickstartOrganization(t, db, ctx)
	stack := newQuickstartStack(db, orgID, userID)

	// Defaults: the state the README now states before the refusal step.
	require.Equal(t, "monitoring", stack.enforcementMode(t),
		"a fresh quickstart organization must start in the mode the README states")

	onDefaults := seedQuickstartAgent(t, db, ctx, stack.capRepo, orgID, userID)
	status, body := stack.registerDBWrite(t, onDefaults)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, "granted", body["status"],
		"on defaults the SDK's registration is auto-granted, so the README's db:write call runs")
	assert.ElementsMatch(t, []string{"db:read", "db:write"}, stack.activeCapabilities(t, onDefaults))

	// The README's one step: Security, Policies, Global Enforcement Mode, Strict.
	status, body = stack.do(t, "PUT", "/api/v1/admin/enforcement-settings", map[string]any{"enforcementMode": "strict"})
	require.Equal(t, fiber.StatusOK, status, "body: %v", body)
	assert.Equal(t, "strict", body["enforcementMode"])

	// The README's verify: the settings read back strict.
	require.Equal(t, "strict", stack.enforcementMode(t))

	// The refusal step: the same registration now waits for an administrator,
	// and the agent still holds db:read only.
	afterStep := seedQuickstartAgent(t, db, ctx, stack.capRepo, orgID, userID)
	status, body = stack.registerDBWrite(t, afterStep)
	require.Equal(t, fiber.StatusAccepted, status, "body: %v", body)
	assert.Equal(t, "pending", body["status"])
	assert.Contains(t, body["message"], "strict mode")
	assert.Equal(t, []string{"db:read"}, stack.activeCapabilities(t, afterStep),
		"after the README's step db:write must not be granted, so the call is refused")
}

// seedQuickstartSigningAgent is seedQuickstartAgent with a real Ed25519 public
// key on the agent row, the keypair secure() generates and the decorator signs
// with.
func seedQuickstartSigningAgent(t *testing.T, db *sql.DB, ctx context.Context, capRepo *repository.CapabilityRepositoryPostgres, orgID, userID uuid.UUID) (uuid.UUID, ed25519.PrivateKey) {
	t.Helper()

	agentID := seedQuickstartAgent(t, db, ctx, capRepo, orgID, userID)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE agents SET public_key = $1 WHERE id = $2`,
		base64.StdEncoding.EncodeToString(pub), agentID)
	require.NoError(t, err)
	return agentID, priv
}

// verifyDBWrite sends the verification the Python SDK sends for the README's
// @agent.perform_action(capability="db:write") call: no resource, an empty
// context, signed over json.dumps(sort_keys=True, separators=(', ', ': ')) of
// the action_type, agent_id, context, resource and timestamp fields.
func (s *quickstartStack) verifyDBWrite(t *testing.T, agentID uuid.UUID, priv ed25519.PrivateKey) (int, map[string]any) {
	t.Helper()

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
	message := fmt.Sprintf(`{"action_type": "db:write", "agent_id": "%s", "context": {}, "resource": null, "timestamp": "%s"}`,
		agentID, timestamp)
	return s.do(t, "POST", "/api/v1/sdk-api/verifications", map[string]any{
		"agentId":    agentID.String(),
		"capability": "db:write",
		"resource":   nil,
		"context":    map[string]any{},
		"timestamp":  timestamp,
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(message))),
		"publicKey":  base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
	})
}

// quickstartRefusalReason is the reason the server returns for the README's
// refused db:write on a fresh quickstart organization in strict mode: it has
// no capability_violation policy, so the built-in default_policy decides.
const quickstartRefusalReason = "Capability violation blocked by security policy 'default_policy': " +
	"Agent does not have permission for capability 'db:write' (allowed: [db:read])"

func TestQuickstartRefusalRoundTrip(t *testing.T) {
	db := quickstartEnforcementTestDB(t)
	ctx := context.Background()

	orgID, userID := seedQuickstartOrganization(t, db, ctx)
	stack := newQuickstartStack(db, orgID, userID)

	var policies int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM security_policies WHERE organization_id = $1`, orgID).Scan(&policies))
	require.Zero(t, policies, "a fresh quickstart organization carries no security policy")

	// Defaults: the decorator registers db:write, then verifies it. The
	// registration is auto-granted, so the verification approves and the
	// README's db:write call runs.
	require.Equal(t, "monitoring", stack.enforcementMode(t))
	onDefaults, onDefaultsKey := seedQuickstartSigningAgent(t, db, ctx, stack.capRepo, orgID, userID)
	status, body := stack.registerDBWrite(t, onDefaults)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	status, body = stack.verifyDBWrite(t, onDefaults, onDefaultsKey)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, "auto-approved", body["status"],
		"on defaults the README's db:write call runs; the README must say so before the call")
	assert.Equal(t, "monitoring", body["enforcementMode"])
	assert.Empty(t, body["denialReason"])

	// The README's one step: Global Enforcement Mode, Strict.
	status, body = stack.do(t, "PUT", "/api/v1/admin/enforcement-settings", map[string]any{"enforcementMode": "strict"})
	require.Equal(t, fiber.StatusOK, status, "body: %v", body)
	require.Equal(t, "strict", stack.enforcementMode(t))

	// The refusal step: the registration waits for an administrator and the
	// verification refuses the call before the function body would run.
	afterStep, afterStepKey := seedQuickstartSigningAgent(t, db, ctx, stack.capRepo, orgID, userID)
	status, body = stack.registerDBWrite(t, afterStep)
	require.Equal(t, fiber.StatusAccepted, status, "body: %v", body)
	status, body = stack.verifyDBWrite(t, afterStep, afterStepKey)
	require.Equal(t, fiber.StatusForbidden, status, "body: %v", body)
	assert.Equal(t, "denied", body["status"])
	assert.Equal(t, "strict", body["enforcementMode"])
	assert.Equal(t, quickstartRefusalReason, body["denialReason"])

	// The README's audit claim: the agent's audit log records the denial with
	// the same reason.
	var auditedReason string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT metadata->>'denialReason' FROM audit_logs WHERE agent_id = $1 AND action = 'db:write'`,
		afterStep).Scan(&auditedReason))
	assert.Equal(t, quickstartRefusalReason, auditedReason)

	// The README quotes the refusal the SDK prints, which wraps this reason. A
	// reader on quickstart defaults sees default_policy as the policy name, so
	// the README must name it.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "README.md"))
	require.NoError(t, err)
	readme := string(raw)
	assert.True(t, strings.Contains(readme,
		"': Agent does not have permission for capability 'db:write' (allowed: [db:read])."),
		"README.md: the quoted refusal must carry the reason the server returns")
	assert.True(t, strings.Contains(readme, "`default_policy`"),
		"README.md: the quick start must name the policy a quickstart-default stack reports, `default_policy`")
}
