package handlers

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /a2a/verify answers 500, not 401, when the verification cannot read
// what it needs: a failed database read is not an authentication failure, and
// a caller told 401 would treat a good signature as a bad one.
func TestA2AVerifyAnswersServerErrorWhenTheNonceCannotBeChecked(t *testing.T) {
	nonceDB, nonceMock, err := sqlmock.New()
	require.NoError(t, err)
	defer nonceDB.Close()
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WillReturnError(errors.New("a2a_request_nonces: connection reset"))

	svc := application.NewA2AService(
		nil, nil, nil, nil, nil, nil,
		repository.NewA2ARequestNonceRepository(nonceDB),
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	app := fiber.New()
	app.Post("/a2a/verify", NewA2AHandler(svc, nil, nil).VerifyRequest)

	body, err := json.Marshal(application.VerifyA2ARequestRequest{
		AgentID:     uuid.New(),
		Timestamp:   time.Now().UTC().Unix(),
		Nonce:       uuid.New().String(),
		Signature:   "c2lnbmF0dXJl",
		RequestHash: strings.Repeat("ab", 32),
	})
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/a2a/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, got)
	assert.NoError(t, nonceMock.ExpectationsWereMet())
}

// postVerify posts a verification request with nonce, for an agent that
// signed nothing, to a handler on svc.
func postVerify(t *testing.T, svc *application.A2AService, agentID uuid.UUID, nonce string) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Post("/a2a/verify", NewA2AHandler(svc, nil, nil).VerifyRequest)
	body, err := json.Marshal(application.VerifyA2ARequestRequest{
		AgentID:     agentID,
		Timestamp:   time.Now().UTC().Unix(),
		Nonce:       nonce,
		Signature:   "c2lnbmF0dXJl",
		RequestHash: strings.Repeat("ab", 32),
	})
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/a2a/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return resp.StatusCode, got
}

// POST /a2a/verify answers 400 with valid: false for a body it cannot read as
// a verification request, as the 401 does: a caller that branches on valid
// finds it in both of the handler's 400 answers, as in its 401.
func TestA2AVerifyAnswersValidFalseForABodyItCannotRead(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":              `{"nonce": `,
		"agentId is not a UUID": `{"agentId": "not-a-uuid", "nonce": "n-1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			nonceDB, nonceMock, err := sqlmock.New()
			require.NoError(t, err)
			defer nonceDB.Close()
			svc := application.NewA2AService(
				nil, nil, nil, nil, nil, nil,
				repository.NewA2ARequestNonceRepository(nonceDB),
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			app := fiber.New()
			app.Post("/a2a/verify", NewA2AHandler(svc, nil, nil).VerifyRequest)
			req := httptest.NewRequest("POST", "/a2a/verify", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
			var got map[string]any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			assert.Equal(t, map[string]any{"valid": false, "error": "Invalid request body"}, got)
			assert.NoError(t, nonceMock.ExpectationsWereMet())
		})
	}
}

// POST /a2a/verify answers 400 for a nonce the nonce table cannot record, an
// empty one or one longer than 64 characters, before anything is read: not
// 500 for the failed write, and not valid: true for a nonce recorded empty.
// The answer carries valid: false, as the 401 does.
func TestA2AVerifyAnswersBadRequestForANonceThatCannotBeRecorded(t *testing.T) {
	for name, nonce := range map[string]string{
		"empty":         "",
		"65 characters": strings.Repeat("a", 65),
	} {
		t.Run(name, func(t *testing.T) {
			nonceDB, nonceMock, err := sqlmock.New()
			require.NoError(t, err)
			defer nonceDB.Close()
			svc := application.NewA2AService(
				nil, nil, nil, nil, nil, nil,
				repository.NewA2ARequestNonceRepository(nonceDB),
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)

			status, got := postVerify(t, svc, uuid.New(), nonce)
			assert.Equal(t, fiber.StatusBadRequest, status)
			assert.Equal(t, map[string]any{"valid": false, "error": application.ErrInvalidA2ANonce.Error()}, got)
			assert.NoError(t, nonceMock.ExpectationsWereMet())
		})
	}
}

// POST /a2a/verify answers 500, not 401 "agent not found", when the agent row
// cannot be read.
func TestA2AVerifyAnswersServerErrorWhenTheAgentCannotBeRead(t *testing.T) {
	nonceDB, nonceMock, err := sqlmock.New()
	require.NoError(t, err)
	defer nonceDB.Close()
	agentDB, agentMock, err := sqlmock.New()
	require.NoError(t, err)
	defer agentDB.Close()
	agentID := uuid.New()
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	agentMock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WithArgs(agentID).
		WillReturnError(errors.New("agents: connection reset"))

	svc := application.NewA2AService(
		nil, nil, nil, nil, nil, nil,
		repository.NewA2ARequestNonceRepository(nonceDB),
		nil, nil, nil, nil, nil,
		repository.NewAgentRepository(agentDB),
		nil, nil, nil,
	)

	status, got := postVerify(t, svc, agentID, uuid.New().String())
	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, got)
	assert.NoError(t, nonceMock.ExpectationsWereMet())
	assert.NoError(t, agentMock.ExpectationsWereMet())
}
