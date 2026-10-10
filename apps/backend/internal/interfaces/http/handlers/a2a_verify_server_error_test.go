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
