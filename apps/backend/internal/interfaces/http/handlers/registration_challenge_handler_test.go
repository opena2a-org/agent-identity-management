package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// POST /api/v1/agents/registration-challenges answers a challenge id and a
// 32-byte challenge in base64, valid for five minutes and bound to the caller's
// organization and to the caller. A registration whose agent stays pending says
// why in the pendingReason member of its answer.

type storedChallenges struct {
	domain.RegistrationChallengeRepository
	rows []*domain.RegistrationChallenge
}

func (s *storedChallenges) Create(_ context.Context, c *domain.RegistrationChallenge) error {
	s.rows = append(s.rows, c)
	return nil
}

func TestTheChallengeRouteAnswersAChallengeBoundToItsAsker(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	store := &storedChallenges{}
	handler := NewRegistrationChallengeHandler(application.NewRegistrationChallengeService(store))
	app := createTestAppWithAuth(&AgentHandler{}, orgID, userID)
	app.Post("/api/v1/agents/registration-challenges", handler.Issue)

	before := time.Now()
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/agents/registration-challenges", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, 201, resp.StatusCode, string(raw))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var answer struct {
		ChallengeID string    `json:"challengeId"`
		Challenge   string    `json:"challenge"`
		ExpiresAt   time.Time `json:"expiresAt"`
	}
	require.NoError(t, json.Unmarshal(raw, &answer))
	challenge, err := base64.StdEncoding.DecodeString(answer.Challenge)
	require.NoError(t, err)
	assert.Len(t, challenge, 32)
	assert.WithinDuration(t, before.Add(5*time.Minute), answer.ExpiresAt, 5*time.Second)
	require.Len(t, store.rows, 1)
	assert.Equal(t, answer.ChallengeID, store.rows[0].ID.String())
	assert.Equal(t, orgID, store.rows[0].OrganizationID)
	assert.Equal(t, userID, store.rows[0].PrincipalID)
	assert.Equal(t, challenge, store.rows[0].Challenge)
}

func TestARegistrationThatStaysPendingSaysWhyInOneField(t *testing.T) {
	publicKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cases := map[string]struct {
		extra string
		want  string
	}{
		"no proof": {extra: ``, want: application.PendingReasonNoProof},
		"a proof the server cannot accept": {
			extra: `,"possessionProof":{"challengeId":"` + uuid.NewString() + `","signature":"c2ln"}`,
			want:  application.PendingReasonProofNotAccepted,
		},
		"a proof that is not an object": {extra: `,"possessionProof":"text"`, want: application.PendingReasonProofNotAccepted},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			status, answer := postCreateAgent(t, &registeringAgentRepo{},
				`{"name":"pending-agent","displayName":"Pending","description":"d","agentType":"ai_agent","publicKey":"`+publicKey+`"`+tc.extra+`}`)

			assert.Equal(t, 201, status, "a proof that is not good is not an error")
			assert.Equal(t, "pending", answer["status"])
			assert.Equal(t, tc.want, answer["pendingReason"])
		})
	}
}
