package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// registrationCapabilities is the list the SDK reported when the agent was
// registered, stored on the agent row. It is a declaration: no grant or
// revocation updates it, so no reader may serve it as the agent's capabilities.
const registrationCapabilities = `["legacy:detected","file:read"]`

// agentByIDColumns are the columns AgentRepository.GetByID selects, in order.
var agentByIDColumns = []string{
	"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version",
	"public_key", "encrypted_private_key", "key_algorithm", "certificate_url", "repository_url", "documentation_url",
	"trust_score", "verified_at", "talks_to", "capabilities", "metadata", "created_at", "updated_at", "created_by", "last_active",
	"key_created_at", "key_expires_at", "key_rotation_grace_until", "previous_public_key", "rotation_count",
	"pqc_public_key", "pqc_key_algorithm", "hybrid_mode_enabled", "pqc_key_created_at", "pqc_key_expires_at", "previous_pqc_public_key",
	"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
	"updated_by", "updated_by_name", "updated_by_email",
	"capability_violation_count", "is_compromised", "declared_purpose",
}

// expectAgentRow answers one AgentRepository.GetByID for the agent, with the
// registration-time capability list on the row.
func expectAgentRow(mock sqlmock.Sqlmock, id, orgID uuid.UUID, publicKey string) {
	now := time.Now().UTC()
	row := sqlmock.NewRows(agentByIDColumns).AddRow(
		id.String(), orgID.String(), "billing-agent", "Billing Agent", nil, "ai_agent", "verified", nil,
		publicKey, nil, nil, nil, nil, nil,
		0.8, nil, nil, []byte(registrationCapabilities), nil, now, now, uuid.New().String(), nil,
		nil, nil, nil, nil, nil,
		nil, nil, false, nil, nil, nil,
		"", "", nil, nil,
		nil, "", "",
		int64(0), false, nil,
	)
	mock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WithArgs(id).WillReturnRows(row)
}

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// a2aServiceForGrants builds an A2AService whose agent row carries the
// registration-time list and whose grant table answers with grants. The card,
// skill and trust score tables are a database with no rows, so the card is
// built from the agent alone.
func a2aServiceForGrants(t *testing.T, agentDB, nonceDB *sql.DB, grants activeCapabilityReader) *A2AService {
	t.Helper()
	emptyDB, _ := newSQLMock(t)
	return &A2AService{
		cardRepo:       repository.NewA2AAgentCardRepository(emptyDB),
		skillRepo:      repository.NewA2ASkillRepository(emptyDB),
		trustScoreRepo: repository.NewA2ATrustScoreRepository(emptyDB),
		nonceRepo:      repository.NewA2ARequestNonceRepository(nonceDB),
		agentRepo:      repository.NewAgentRepository(agentDB),
		capabilityRepo: grants,
	}
}

// signedA2ARequest is a request VerifyA2ARequest accepts for the agent holding priv.
func signedA2ARequest(t *testing.T, agentID uuid.UUID, priv ed25519.PrivateKey) VerifyA2ARequestRequest {
	t.Helper()
	hash := sha256.Sum256([]byte("POST /tasks " + agentID.String()))
	return VerifyA2ARequestRequest{
		AgentID:     agentID,
		Timestamp:   time.Now().UTC().Unix(),
		Nonce:       uuid.New().String(),
		Signature:   base64.StdEncoding.EncodeToString(ed25519.Sign(priv, hash[:])),
		RequestHash: hex.EncodeToString(hash[:]),
	}
}

// The agent card's AIM extension and the A2A request verification result serve
// the same capability list as the agent's trust credential: the active grants,
// never the list on the agent row. Grant A and B, revoke A, grant C: the
// credential, the card and the verify result all name exactly B and C.
func TestA2AReaders_ServeTheSameCapabilitiesAsTheCredential(t *testing.T) {
	ctx := context.Background()
	id, orgID := uuid.New(), uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(pub)

	// A revoked and B active from the first grants, C granted after; a honeytoken
	// grant is never served.
	revokedAt := time.Now().UTC()
	a, b, c, decoy := grant("file:read"), grant("api:call"), grant("db:read"), grant("secrets:read")
	a.RevokedAt = &revokedAt
	decoy.Honeytoken = true
	grants := &fakeCapabilityReader{grants: []*domain.AgentCapability{c, decoy, b, a}}
	want := []string{"api:call", "db:read"}

	issuer, client := issuanceServiceForGrants(t, &domain.Agent{
		ID: id, OrganizationID: orgID, Capabilities: []string{"legacy:detected", "file:read"},
	}, grants)
	if _, err := issuer.IssueForAgent(ctx, id); err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	credential := client.gotReq.Capabilities
	if !reflect.DeepEqual(credential, want) {
		t.Fatalf("credential capabilities = %v, want the active grants %v", credential, want)
	}

	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

	expectAgentRow(agentMock, id, orgID, publicKey)
	card, err := svc.GetEnhancedAgentCard(ctx, id)
	if err != nil {
		t.Fatalf("GetEnhancedAgentCard: %v", err)
	}
	if card.AIM == nil {
		t.Fatal("agent card has no AIM extension")
	}
	if !reflect.DeepEqual(card.AIM.Capabilities, credential) {
		t.Errorf("agent card capabilities = %v, want the credential's %v; the row's registration list is %s",
			card.AIM.Capabilities, credential, registrationCapabilities)
	}

	req := signedA2ARequest(t, id, priv)
	expectAgentRow(agentMock, id, orgID, publicKey)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := svc.VerifyA2ARequest(ctx, req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if !result.Valid {
		t.Fatalf("verify result invalid: %q", result.Error)
	}
	if !reflect.DeepEqual(result.Capabilities, credential) {
		t.Errorf("verify result capabilities = %v, want the credential's %v; the row's registration list is %s",
			result.Capabilities, credential, registrationCapabilities)
	}

	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
	if err := nonceMock.ExpectationsWereMet(); err != nil {
		t.Errorf("nonce reads: %v", err)
	}
	if grants.calls != 3 {
		t.Errorf("grant table read %d times for one credential, one card and one verification, want 3", grants.calls)
	}
}

// When the grant table cannot be read, neither reader falls back to the list on
// the agent row: the card fails, and the verification fails without recording
// the caller's nonce, so the same signed request can be retried.
func TestA2AReaders_FailClosedWhenGrantsCannotBeRead(t *testing.T) {
	ctx := context.Background()
	id, orgID := uuid.New(), uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(pub)
	grants := &fakeCapabilityReader{err: errors.New("agent_capabilities: connection reset")}

	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

	expectAgentRow(agentMock, id, orgID, publicKey)
	card, err := svc.GetEnhancedAgentCard(ctx, id)
	if err == nil {
		t.Fatalf("GetEnhancedAgentCard served a card with capabilities %v when the grants could not be read", card.AIM.Capabilities)
	}

	req := signedA2ARequest(t, id, priv)
	expectAgentRow(agentMock, id, orgID, publicKey)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	// Registered so a nonce write would match it; it must stay unmatched.
	nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := svc.VerifyA2ARequest(ctx, req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if result.Valid || result.Capabilities != nil {
		t.Fatalf("verify result valid=%v capabilities=%v when the grants could not be read", result.Valid, result.Capabilities)
	}
	if result.Error != "failed to load capability grants" {
		t.Errorf("verify result error = %q, want %q", result.Error, "failed to load capability grants")
	}
	if err := nonceMock.ExpectationsWereMet(); err == nil {
		t.Error("the nonce was recorded although the verification failed before succeeding")
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}
