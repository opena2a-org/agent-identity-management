package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	aimcrypto "github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// PUT /a2a/cards/:id replaces the agent's card document the way
// POST /agents/:id/card registers one: the stored card_hash becomes the hash of
// the new document and a new attestation is issued. A body with no document
// is refused with 400 naming the field-level members it carried.
//
// The handler runs on the real card service and repositories over sqlmock.

// capturedArg matches any SQL argument and keeps it.
type capturedArg struct{ value *driver.Value }

func (a capturedArg) Match(v driver.Value) bool {
	*a.value = v
	return true
}

// cardReplaceAuditSink keeps the audit rows written.
type cardReplaceAuditSink struct {
	domain.AuditLogRepository
	rows []*domain.AuditLog
}

func (s *cardReplaceAuditSink) Create(entry *domain.AuditLog) error {
	s.rows = append(s.rows, entry)
	return nil
}

type cardReplaceFixture struct {
	app     *fiber.App
	mock    sqlmock.Sqlmock
	audit   *cardReplaceAuditSink
	orgID   uuid.UUID
	agentID uuid.UUID
}

// newCardReplaceFixture mounts PUT /a2a/cards/:id for a caller in the agent's
// organization. principal sets the caller's identity locals.
func newCardReplaceFixture(t *testing.T, principal func(c fiber.Ctx, agentID uuid.UUID)) *cardReplaceFixture {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	master := make([]byte, 32)
	_, err = rand.Read(master)
	require.NoError(t, err)
	kv, err := aimcrypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	ring, err := aimcrypto.LoadSigningKeyRing(kv, func(string) string { return "" })
	require.NoError(t, err)

	f := &cardReplaceFixture{mock: mock, audit: &cardReplaceAuditSink{}, orgID: uuid.New(), agentID: uuid.New()}
	svc := application.NewA2AService(
		repository.NewA2AAgentCardRepository(db),
		repository.NewA2ASkillRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		repository.NewAgentRepository(db),
		nil,
		nil,
		ring.Key(aimcrypto.PurposeCardAttestation),
	)
	h := &A2AHandler{
		a2aService: svc,
		a2aReader:  svc,
		agentService: &MockAgentServiceImpl{GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: f.orgID}, nil
		}},
		auditService: application.NewAuditService(f.audit),
	}
	f.app = fiber.New()
	f.app.Put("/a2a/cards/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", f.orgID)
		principal(c, f.agentID)
		return h.UpdateAgentCard(c)
	})
	return f
}

func asUser(userID uuid.UUID) func(fiber.Ctx, uuid.UUID) {
	return func(c fiber.Ctx, _ uuid.UUID) { c.Locals("user_id", userID) }
}

// asTheAgent authenticates the caller as the agent whose card it replaces,
// with no user.
func asTheAgent(c fiber.Ctx, agentID uuid.UUID) { c.Locals("agent_id", agentID) }

// expectAgentRead answers the card service's agent lookup.
func (f *cardReplaceFixture) expectAgentRead() {
	now := time.Now().UTC()
	f.mock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version",
		"public_key", "encrypted_private_key", "key_algorithm", "certificate_url", "repository_url", "documentation_url",
		"trust_score", "verified_at", "talks_to", "capabilities", "metadata", "created_at", "updated_at", "created_by", "last_active",
		"key_created_at", "key_expires_at", "key_rotation_grace_until", "previous_public_key", "rotation_count",
		"pqc_public_key", "pqc_key_algorithm", "hybrid_mode_enabled", "pqc_key_created_at", "pqc_key_expires_at", "previous_pqc_public_key",
		"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
		"updated_by", "updated_by_name", "updated_by_email",
		"capability_violation_count", "is_compromised", "declared_purpose",
	}).AddRow(
		f.agentID.String(), f.orgID.String(), "card-agent", "Card Agent", nil, string(domain.AgentTypeAI), string(domain.AgentStatusVerified), nil,
		nil, nil, "Ed25519", nil, nil, nil,
		0.0, nil, nil, nil, nil, now, now, uuid.New().String(), nil,
		nil, nil, nil, nil, 0,
		nil, nil, false, nil, nil, nil,
		"", "", nil, nil,
		nil, "", "",
		0, false, nil,
	))
}

// expectCardUpsert answers the card upsert and keeps the card_data and
// card_hash it writes.
func (f *cardReplaceFixture) expectCardUpsert(cardData, cardHash *driver.Value) {
	args := make([]driver.Value, 18)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[3] = capturedArg{cardData}
	args[4] = capturedArg{cardHash}
	f.mock.ExpectQuery(`INSERT INTO a2a_agent_cards`).WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "fetch_count"}).
			AddRow(uuid.New().String(), time.Now().UTC(), time.Now().UTC(), 1))
}

func (f *cardReplaceFixture) expectSkillUpserts(n int) {
	for i := 0; i < n; i++ {
		f.mock.ExpectQuery(`INSERT INTO a2a_skills`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
				AddRow(uuid.New().String(), time.Now().UTC(), time.Now().UTC()))
	}
}

func (f *cardReplaceFixture) put(t *testing.T, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("PUT", "/a2a/cards/"+f.agentID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	return resp.StatusCode, out
}

func cardHashOf(doc string) string {
	sum := sha256.Sum256([]byte(doc))
	return hex.EncodeToString(sum[:])
}

func putCardData(t *testing.T, f *cardReplaceFixture, doc string, skills int) (int, map[string]any, driver.Value, driver.Value) {
	t.Helper()
	var data, hash driver.Value
	f.expectAgentRead()
	f.expectCardUpsert(&data, &hash)
	f.expectSkillUpserts(skills)
	body, err := json.Marshal(map[string]json.RawMessage{"cardData": json.RawMessage(doc)})
	require.NoError(t, err)
	status, out := f.put(t, string(body))
	return status, out, data, hash
}

func TestUpdateAgentCard_CardDataReplacesTheDocumentAndChangesItsHash(t *testing.T) {
	userID := uuid.New()
	f := newCardReplaceFixture(t, asUser(userID))
	first := `{"name":"report-agent","version":"1.0.0","skills":[{"id":"summarize","name":"Summarize"}]}`
	second := `{"name":"report-agent","version":"1.1.0","skills":[{"id":"summarize","name":"Summarize"},{"id":"translate","name":"Translate"}]}`

	status, out, data, firstHash := putCardData(t, f, first, 1)
	require.Equal(t, fiber.StatusOK, status, out)
	assert.Equal(t, first, string(data.([]byte)), "the stored card is the document sent")
	assert.Equal(t, cardHashOf(first), firstHash)

	status, out, data, secondHash := putCardData(t, f, second, 2)
	require.Equal(t, fiber.StatusOK, status, out)
	assert.Equal(t, second, string(data.([]byte)))
	assert.Equal(t, cardHashOf(second), secondHash)
	assert.NotEqual(t, firstHash, secondHash, "replacing the document changes cardHash")

	assert.Equal(t, f.agentID.String(), out["agentId"])
	assert.Equal(t, "inline://"+f.agentID.String(), out["cardUrl"])
	assert.Equal(t, []any{"summarize", "translate"}, out["skills"])
	assert.NotEmpty(t, out["attestationExpires"], "a new attestation is issued")
	require.NoError(t, f.mock.ExpectationsWereMet())

	require.Len(t, f.audit.rows, 2)
	assert.Equal(t, domain.AuditActionUpdate, f.audit.rows[1].Action)
	require.NotNil(t, f.audit.rows[1].UserID)
	assert.Equal(t, userID, *f.audit.rows[1].UserID, "the actor is the authenticated user")
}

func TestUpdateAgentCard_AnAgentCallerReplacesItsOwnCardWithNoUser(t *testing.T) {
	f := newCardReplaceFixture(t, asTheAgent)
	status, out, _, hash := putCardData(t, f, `{"name":"report-agent","version":"2.0.0"}`, 0)
	require.Equal(t, fiber.StatusOK, status, out)
	assert.Equal(t, cardHashOf(`{"name":"report-agent","version":"2.0.0"}`), hash)
	require.NoError(t, f.mock.ExpectationsWereMet())

	require.Len(t, f.audit.rows, 1)
	assert.Nil(t, f.audit.rows[0].UserID, "an agent caller has no user")
	require.NotNil(t, f.audit.rows[0].AgentID)
	assert.Equal(t, f.agentID, *f.audit.rows[0].AgentID, "the actor is the calling agent")
}

func TestUpdateAgentCard_FieldMembersWithoutADocumentAre400NamingThem(t *testing.T) {
	f := newCardReplaceFixture(t, asUser(uuid.New()))
	status, out := f.put(t, `{"name":"x","description":"y","version":"z","endpoint":"e","skills":[{"id":"s"}]}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, map[string]any{
		"error": "cardUrl or cardData is required: this route replaces the agent card document and does not edit fields; " +
			"it does not read name, description, version, endpoint, skills",
		"unreadMembers": []any{"name", "description", "version", "endpoint", "skills"},
	}, out)

	status, out = f.put(t, `{"name":"x","skills":["s"]}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, []any{"name", "skills"}, out["unreadMembers"], "only the members the body carried are named")

	status, out = f.put(t, `{}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, map[string]any{
		"error": "cardUrl or cardData is required: this route replaces the agent card document and does not edit fields",
	}, out)

	require.NoError(t, f.mock.ExpectationsWereMet(), "nothing is read or written")
	assert.Empty(t, f.audit.rows)
}
