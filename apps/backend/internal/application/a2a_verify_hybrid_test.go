package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Written out as literals so this file states the contract independently of the
// declarations it checks: the words and reasonCode the other routes that verify
// an Ed25519 signature alone answer an agent in hybrid mode with.
const (
	a2aHybridRequiredError  = "Agent is in hybrid mode and this route verifies an Ed25519 signature alone"
	a2aHybridRequiredReason = "hybridSignatureRequired"
)

// expectHybridAgentRow is expectAgentRowInStatus for an agent in hybrid mode:
// an Ed25519 key, an ML-DSA-65 key and hybrid_mode_enabled true on the row.
func expectHybridAgentRow(mock sqlmock.Sqlmock, id, orgID uuid.UUID, publicKey string, status domain.AgentStatus) {
	now := time.Now().UTC()
	row := sqlmock.NewRows(agentByIDColumns).AddRow(
		id.String(), orgID.String(), "billing-agent", "Billing Agent", nil, "ai_agent", string(status), nil,
		publicKey, nil, "Ed25519", nil, nil, nil,
		0.8, nil, nil, []byte(registrationCapabilities), nil, now, now, uuid.New().String(), nil,
		nil, nil, nil, nil, nil,
		"FAKE-MLDSA-PUBLIC-KEY", "ML-DSA-65", true, now, nil, nil,
		"", "", nil, nil,
		nil, "", "",
		int64(0), false, nil,
	)
	mock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WithArgs(id).WillReturnRows(row)
}

// An agent in hybrid mode authenticates only with an Ed25519 and an ML-DSA
// signature together, and an A2A verification request carries one Ed25519
// signature. A request whose Ed25519 signature verifies is refused with the
// reasonCode the agent-signature middlewares answer, for a verified agent and a
// suspended one alike: the flag is read after the signature and before the
// status. The refusal spends no nonce and reads no grants.
func TestVerifyA2ARequestRefusesAnAgentInHybridModeSigningWithEd25519Alone(t *testing.T) {
	for _, status := range []domain.AgentStatus{domain.AgentStatusVerified, domain.AgentStatusSuspended} {
		t.Run(string(status), func(t *testing.T) {
			id, orgID := uuid.New(), uuid.New()
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatalf("generate key: %v", err)
			}
			grants := &fakeCapabilityReader{grants: []*domain.AgentCapability{grant("api:call")}}
			agentDB, agentMock := newSQLMock(t)
			nonceDB, nonceMock := newSQLMock(t)
			svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

			req := signedA2ARequest(t, id, priv)
			expectHybridAgentRow(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub), status)
			nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
				WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			// Registered so a nonce write would match it; it must stay unmatched.
			nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

			result, err := svc.VerifyA2ARequest(context.Background(), req)
			if err != nil {
				t.Fatalf("VerifyA2ARequest: %v", err)
			}
			if result.Valid {
				t.Fatalf("an Ed25519-only request from a %s agent in hybrid mode verified as valid with capabilities %v",
					status, result.Capabilities)
			}
			if result.Error != a2aHybridRequiredError {
				t.Errorf("verify result error = %q, want %q", result.Error, a2aHybridRequiredError)
			}
			if result.ReasonCode != a2aHybridRequiredReason {
				t.Errorf("verify result reasonCode = %q, want %q", result.ReasonCode, a2aHybridRequiredReason)
			}
			if result.Capabilities != nil {
				t.Errorf("the refused result carries capabilities %v", result.Capabilities)
			}
			if grants.calls != 0 {
				t.Errorf("the grant table was read %d times for a refused request", grants.calls)
			}
			if err := nonceMock.ExpectationsWereMet(); err == nil {
				t.Error("the nonce was recorded for a refused request")
			}
			if err := agentMock.ExpectationsWereMet(); err != nil {
				t.Errorf("agent reads: %v", err)
			}
		})
	}
}

// The hybrid flag is read only after the signature verifies: an agent in hybrid
// mode whose request carries one flipped signature byte gets the signature
// refusal, with no reasonCode, so a caller without the agent's key does not
// learn that the agent is in hybrid mode.
func TestVerifyA2ARequestChecksTheSignatureBeforeTheHybridFlag(t *testing.T) {
	id, orgID := uuid.New(), uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, &fakeCapabilityReader{})

	req := signedA2ARequest(t, id, priv)
	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	signature[0] ^= 0x01
	req.Signature = base64.StdEncoding.EncodeToString(signature)

	expectHybridAgentRow(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub), domain.AgentStatusVerified)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if result.Valid {
		t.Fatal("a request with a flipped signature byte verified as valid")
	}
	if want := "signature verification failed"; result.Error != want {
		t.Errorf("verify result error = %q, want %q", result.Error, want)
	}
	if result.ReasonCode != "" {
		t.Errorf("verify result reasonCode = %q for a signature that does not verify, want none", result.ReasonCode)
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}
