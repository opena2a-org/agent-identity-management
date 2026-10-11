package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
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

// A request whose nonce cannot be recorded could be presented again, so the
// verification returns an error instead of a valid result. Presenting the same
// signed request a second time gets the same answer, not valid: true.
func TestVerifyA2ARequestReturnsAnErrorWhenTheNonceCannotBeRecorded(t *testing.T) {
	id, orgID := uuid.New(), uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(pub)
	grants := &fakeCapabilityReader{grants: []*domain.AgentCapability{grant("api:call")}}
	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

	req := signedA2ARequest(t, id, priv)
	for presentation := 1; presentation <= 2; presentation++ {
		expectAgentRow(agentMock, id, orgID, publicKey)
		nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
			WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).
			WillReturnError(errors.New("a2a_request_nonces: connection reset"))

		result, err := svc.VerifyA2ARequest(context.Background(), req)
		if err == nil {
			t.Errorf("presentation %d: VerifyA2ARequest answered %+v when the nonce could not be recorded, want an error",
				presentation, result)
			continue
		}
		if result != nil {
			t.Errorf("presentation %d: VerifyA2ARequest answered %+v with its error, want no result", presentation, result)
		}
	}
	if err := nonceMock.ExpectationsWereMet(); err != nil {
		t.Errorf("nonce reads and writes: %v", err)
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}

// A nonce the nonce table cannot record is a malformed request: an empty one,
// one longer than the column, and one Postgres refuses as text. It is refused
// with ErrInvalidA2ANonce before the nonce table or the agent is read, not
// recorded and answered valid, and not left to fail the write.
func TestVerifyA2ARequestRefusesANonceThatCannotBeRecorded(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	for name, nonce := range map[string]string{
		"empty":                  "",
		"65 characters":          strings.Repeat("a", MaxA2ANonceLength+1),
		"65 two-byte characters": strings.Repeat("é", MaxA2ANonceLength+1),
		"a NUL character":        "nonce\x00",
		"not UTF-8":              "nonce\xff",
	} {
		t.Run(name, func(t *testing.T) {
			grants := &fakeCapabilityReader{}
			agentDB, agentMock := newSQLMock(t)
			nonceDB, nonceMock := newSQLMock(t)
			svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

			req := signedA2ARequest(t, uuid.New(), priv)
			req.Nonce = nonce
			result, err := svc.VerifyA2ARequest(context.Background(), req)
			if !errors.Is(err, ErrInvalidA2ANonce) {
				t.Fatalf("VerifyA2ARequest answered %+v, %v for nonce %q, want ErrInvalidA2ANonce", result, err, nonce)
			}
			if result != nil {
				t.Errorf("VerifyA2ARequest answered %+v with its error, want no result", result)
			}
			if grants.calls != 0 {
				t.Errorf("the grant table was read %d times for a malformed nonce", grants.calls)
			}
			if err := nonceMock.ExpectationsWereMet(); err != nil {
				t.Errorf("nonce reads: %v", err)
			}
			if err := agentMock.ExpectationsWereMet(); err != nil {
				t.Errorf("agent reads: %v", err)
			}
		})
	}
}

// The limit counts characters, as the VARCHAR(64) column does: a nonce of 64
// two-byte characters is recorded and the request verifies.
func TestVerifyA2ARequestAcceptsANonceOfSixtyFourCharacters(t *testing.T) {
	id, orgID := uuid.New(), uuid.New()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, &fakeCapabilityReader{grants: []*domain.AgentCapability{grant("api:call")}})

	req := signedA2ARequest(t, id, priv)
	req.Nonce = strings.Repeat("é", MaxA2ANonceLength)
	expectAgentRow(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub))
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if !result.Valid {
		t.Fatalf("a request with a 64-character nonce did not verify: %q", result.Error)
	}
	if err := nonceMock.ExpectationsWereMet(); err != nil {
		t.Errorf("nonce reads and writes: %v", err)
	}
}

// A nonce the nonce table already holds is a replay: the request is not valid,
// and neither the agent nor its grants are read and no nonce is recorded.
func TestVerifyA2ARequestRefusesANonceFoundUsed(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	grants := &fakeCapabilityReader{grants: []*domain.AgentCapability{grant("api:call")}}
	agentDB, agentMock := newSQLMock(t)
	nonceDB, nonceMock := newSQLMock(t)
	svc := a2aServiceForGrants(t, agentDB, nonceDB, grants)

	req := signedA2ARequest(t, uuid.New(), priv)
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	// Registered so a nonce write would match it; it must stay unmatched.
	nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if result.Valid {
		t.Fatalf("a request whose nonce was found used verified as valid with capabilities %v", result.Capabilities)
	}
	if want := "nonce already used (replay attack)"; result.Error != want {
		t.Errorf("verify result error = %q, want %q", result.Error, want)
	}
	if result.Capabilities != nil {
		t.Errorf("the refused result carries capabilities %v", result.Capabilities)
	}
	if grants.calls != 0 {
		t.Errorf("the grant table was read %d times for a replayed nonce", grants.calls)
	}
	if err := nonceMock.ExpectationsWereMet(); err == nil {
		t.Error("the nonce was recorded again for a replayed request")
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}

// An agent row that cannot be read says nothing about the request, so the
// verification returns an error instead of the verdict "agent not found",
// and spends no nonce. An agent that is not stored is still that verdict.
func TestVerifyA2ARequestReturnsAnErrorWhenTheAgentCannotBeRead(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	for name, tc := range map[string]struct {
		answer    func(*sqlmock.ExpectedQuery)
		wantError bool
	}{
		"a failed read": {
			answer:    func(q *sqlmock.ExpectedQuery) { q.WillReturnError(errors.New("agents: connection reset")) },
			wantError: true,
		},
		"no agent": {
			answer: func(q *sqlmock.ExpectedQuery) { q.WillReturnRows(sqlmock.NewRows(agentByIDColumns)) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			agentDB, agentMock := newSQLMock(t)
			nonceDB, nonceMock := newSQLMock(t)
			svc := a2aServiceForGrants(t, agentDB, nonceDB, &fakeCapabilityReader{})

			req := signedA2ARequest(t, uuid.New(), priv)
			nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
				WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			// Registered so a nonce write would match it; it must stay unmatched.
			nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnResult(sqlmock.NewResult(0, 1))
			tc.answer(agentMock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WithArgs(req.AgentID))

			result, err := svc.VerifyA2ARequest(context.Background(), req)
			switch {
			case tc.wantError && err == nil:
				t.Fatalf("VerifyA2ARequest answered %+v when the agent could not be read, want an error", result)
			case tc.wantError && result != nil:
				t.Errorf("VerifyA2ARequest answered %+v with its error, want no result", result)
			case !tc.wantError && err != nil:
				t.Fatalf("VerifyA2ARequest: %v", err)
			case !tc.wantError && (result.Valid || result.Error != "agent not found"):
				t.Errorf("verify result = %+v, want not valid with the error %q", result, "agent not found")
			}
			if err := nonceMock.ExpectationsWereMet(); err == nil {
				t.Error("the nonce was recorded for a request whose agent was not read")
			}
			if err := agentMock.ExpectationsWereMet(); err != nil {
				t.Errorf("agent reads: %v", err)
			}
		})
	}
}

// Two requests carrying one nonce can both find it unused. The nonce's primary
// key lets one of them record it; the other is refused as a request whose nonce
// was found used is, not answered as valid and not reported as a server error.
func TestVerifyA2ARequestRefusesARequestWhoseNonceWasRecordedSinceTheCheck(t *testing.T) {
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
	expectAgentRow(agentMock, id, orgID, base64.StdEncoding.EncodeToString(pub))
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	nonceMock.ExpectExec(`INSERT INTO a2a_request_nonces`).
		WillReturnError(&pq.Error{Code: "23505", Constraint: "a2a_request_nonces_pkey"})

	result, err := svc.VerifyA2ARequest(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyA2ARequest: %v", err)
	}
	if result.Valid {
		t.Fatalf("a request whose nonce was already recorded verified as valid with capabilities %v", result.Capabilities)
	}
	if want := "nonce already used (replay attack)"; result.Error != want {
		t.Errorf("verify result error = %q, want %q", result.Error, want)
	}
	if result.Capabilities != nil {
		t.Errorf("the refused result carries capabilities %v", result.Capabilities)
	}
	if err := nonceMock.ExpectationsWereMet(); err != nil {
		t.Errorf("nonce reads and writes: %v", err)
	}
	if err := agentMock.ExpectationsWereMet(); err != nil {
		t.Errorf("agent reads: %v", err)
	}
}
