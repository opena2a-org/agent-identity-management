//go:build integration

package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// A signed action-request statement through the real handler, services and
// admission store, on both mounts of the route.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestActionRequestRoundTrip ./internal/interfaces/http/handlers/...

func newActionRequestStack(t *testing.T, db *sql.DB, orgID uuid.UUID) *fiber.App {
	t.Helper()
	sqlxDB := sqlx.NewDb(db, "postgres")
	agentRepo := repository.NewAgentRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	capRepo := repository.NewCapabilityRepository(sqlxDB)
	alertRepo := repository.NewAlertRepository(db)
	trustScoreRepo := repository.NewTrustScoreRepository(db)
	verificationEventRepo := repository.NewVerificationEventRepository(db)

	policySvc := application.NewSecurityPolicyService(repository.NewSecurityPolicyRepository(db), alertRepo, auditRepo)
	policySvc.SetVerificationEventRepo(verificationEventRepo)
	trustCalc := application.NewTrustCalculatorWithVerification(trustScoreRepo, repository.NewAPIKeyRepository(db), auditRepo, capRepo, agentRepo, alertRepo, verificationEventRepo)
	verificationEventSvc := application.NewVerificationEventService(verificationEventRepo, agentRepo, application.NewDriftDetectionService(agentRepo, alertRepo))
	capReqSvc := application.NewCapabilityRequestService(repository.NewCapabilityRequestRepository(sqlxDB), capRepo, agentRepo, orgRepo)
	agentSvc := application.NewAgentService(agentRepo, trustCalc, trustScoreRepo, nil, alertRepo, policySvc, capRepo,
		verificationEventSvc, repository.NewTagRepository(db), repository.NewUserRepository(db), orgRepo, capReqSvc)

	nonces := application.NewActionRequestNonceService(repository.NewAgentRequestNonceRepository(db))
	_, err := nonces.Purge(context.Background())
	require.NoError(t, err)

	handler := NewVerificationHandler(agentSvc, application.NewAuditService(auditRepo),
		application.NewAlertService(alertRepo, agentRepo, db), trustCalc, verificationEventSvc, orgRepo, nil).
		WithActionRequestNonces(nonces)

	app := fiber.New()
	app.Post(sdkVerificationsPath, handler.CreateVerification)
	jwt := app.Group(jwtVerificationsPath)
	jwt.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	})
	jwt.Post("/", handler.CreateVerification)
	return app
}

func postStatement(t *testing.T, app *fiber.App, path string, body []byte) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

type agentRecordCounts struct{ events, audits, alerts, nonces int }

func countAgentRecords(t *testing.T, db *sql.DB, agentID uuid.UUID) agentRecordCounts {
	t.Helper()
	var c agentRecordCounts
	require.NoError(t, db.QueryRow(`SELECT
		(SELECT count(*) FROM verification_events WHERE agent_id = $1),
		(SELECT count(*) FROM audit_logs WHERE agent_id = $1),
		(SELECT count(*) FROM alerts WHERE resource_id = $1),
		(SELECT count(*) FROM agent_request_nonces WHERE agent_id = $1)`, agentID).
		Scan(&c.events, &c.audits, &c.alerts, &c.nonces))
	return c
}

func TestActionRequestRoundTrip_AcceptedOnOneMountAndRefusedOnTheOther(t *testing.T) {
	db := quickstartEnforcementTestDB(t)
	ctx := context.Background()
	orgID, userID := seedQuickstartOrganization(t, db, ctx)
	capRepo := repository.NewCapabilityRepository(sqlx.NewDb(db, "postgres"))
	agentID, priv := seedQuickstartSigningAgent(t, db, ctx, capRepo, orgID, userID)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM agent_request_nonces WHERE agent_id = $1`, agentID) })
	app := newActionRequestStack(t, db, orgID)

	signer := &statementSigner{agentID: agentID, pub: priv.Public().(ed25519.PublicKey), priv: priv}
	body := signer.statement(fmt.Sprintf(
		`{"action_type":"db:read","agent_id":"%s","resource":null,"context":{"rows":9007199254740993},"timestamp":"%s","nonce":"%s"}`,
		agentID, time.Now().UTC().Format(time.RFC3339Nano), newTestNonce(t)))

	before := countAgentRecords(t, db, agentID)
	status, out := postStatement(t, app, sdkVerificationsPath, body)
	require.Equal(t, 201, status, out)
	var resp VerificationResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Contains(t, []string{"approved", "auto-approved"}, resp.Status, "a granted capability")

	afterFirst := countAgentRecords(t, db, agentID)
	assert.Equal(t, before.events+1, afterFirst.events)
	assert.Equal(t, before.nonces+1, afterFirst.nonces)

	var recordedContext string
	require.NoError(t, db.QueryRow(`SELECT metadata->>'context' FROM verification_events WHERE id = $1`,
		uuid.MustParse(resp.ID)).Scan(&recordedContext))
	assert.JSONEq(t, `{"rows":9007199254740993}`, recordedContext)
	assert.Contains(t, recordedContext, "9007199254740993", "recorded with the signed digits")

	status, out = postStatement(t, app, jwtVerificationsPath, body)
	assert.Equal(t, 401, status)
	assert.JSONEq(t, refusalBody("nonceReused", "this nonce was already used by this agent"), out)
	assert.Equal(t, afterFirst, countAgentRecords(t, db, agentID), "the replay wrote nothing")

	// The original form still verifies for the same agent.
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
	message := fmt.Sprintf(`{"action_type": "db:read", "agent_id": "%s", "context": {}, "resource": null, "timestamp": "%s"}`,
		agentID, timestamp)
	legacy, _ := json.Marshal(map[string]any{
		"agentId": agentID.String(), "capability": "db:read", "resource": nil, "context": map[string]any{},
		"timestamp": timestamp,
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(message))),
		"publicKey": base64.StdEncoding.EncodeToString(signer.pub),
	})
	status, out = postStatement(t, app, sdkVerificationsPath, legacy)
	assert.Equal(t, 201, status, out)
	assert.Equal(t, afterFirst.nonces, countAgentRecords(t, db, agentID).nonces)

	// A statement stale by the database clock writes nothing.
	stale := signer.statement(fmt.Sprintf(
		`{"action_type":"db:read","agent_id":"%s","resource":null,"timestamp":"%s","nonce":"%s"}`,
		agentID, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), newTestNonce(t)))
	beforeStale := countAgentRecords(t, db, agentID)
	status, out = postStatement(t, app, sdkVerificationsPath, stale)
	assert.Equal(t, 401, status)
	assert.JSONEq(t, refusalBody("timestampOutsideWindow",
		"request timestamp is more than 30 seconds behind the AIM database clock"), out)
	assert.Equal(t, beforeStale, countAgentRecords(t, db, agentID))
}
