package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A request signed by an agent that is not permitted to authenticate is not
// valid, however good the signature: a suspended or revoked agent, and a status
// outside the four known ones, are refused by the same allow-list the auth
// middlewares apply. The refusal spends no nonce and reads no grants.
func TestVerifyA2ARequestRefusesAnAgentNotPermittedToAuthenticate(t *testing.T) {
	for _, status := range []domain.AgentStatus{
		domain.AgentStatusSuspended,
		domain.AgentStatusRevoked,
		domain.AgentStatus("disabled"),
	} {
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
			expectAgentRowInStatus(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub), status)
			nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
				WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			// Registered so a nonce write would match it; it must stay unmatched.
			nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

			result, err := svc.VerifyA2ARequest(context.Background(), req)
			if err != nil {
				t.Fatalf("VerifyA2ARequest: %v", err)
			}
			if result.Valid {
				t.Fatalf("a request signed by a %s agent verified as valid with capabilities %v", status, result.Capabilities)
			}
			if want := domain.AgentStatusDeniedMessage(status); result.Error != want {
				t.Errorf("verify result error = %q, want %q", result.Error, want)
			}
			if result.Capabilities != nil {
				t.Errorf("the refused result carries capabilities %v", result.Capabilities)
			}
			if grants.calls != 0 {
				t.Errorf("the grant table was read %d times for a refused agent", grants.calls)
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

// The status is read only after the signature verifies: a suspended agent's
// request with one flipped signature byte is refused for its signature, so a
// caller without the agent's key does not learn that the agent is suspended.
func TestVerifyA2ARequestChecksTheSignatureBeforeTheStatus(t *testing.T) {
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
	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	signature[0] ^= 0x01
	req.Signature = base64.StdEncoding.EncodeToString(signature)

	expectAgentRowInStatus(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub), domain.AgentStatusSuspended)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if result.Valid {
		t.Fatalf("a request with a flipped signature byte verified as valid with capabilities %v", result.Capabilities)
	}
	if want := "signature verification failed"; result.Error != want {
		t.Errorf("verify result error = %q, want %q", result.Error, want)
	}
	if grants.calls != 0 {
		t.Errorf("the grant table was read %d times for a request whose signature failed", grants.calls)
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}

// A nonce table that cannot be read says nothing about the request, so the
// verification returns an error instead of an invalid result.
func TestVerifyA2ARequestReturnsAnErrorWhenTheNonceCannotBeChecked(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, &fakeCapabilityReader{})

	req := signedA2ARequest(t, uuid.New(), priv)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnError(errors.New("a2a_request_nonces: connection reset"))

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err == nil {
		t.Fatalf("VerifyA2ARequest answered %+v when the nonce could not be checked, want an error", result)
	}
	if result != nil {
		t.Errorf("VerifyA2ARequest answered %+v with its error, want no result", result)
	}
	if err := nonceMock.ExpectationsWereMet(); err != nil {
		t.Errorf("nonce reads: %v", err)
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}
