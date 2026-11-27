package handlers

// POST /api/v1/agents through the server's own handler and agent service, for a
// registration that supplies a publicKey. A key that is not a base64-encoded
// 32-byte Ed25519 key that ed25519key.Acceptable accepts is refused with the
// route's validation error, whether the request carries no proof, a proof for a
// challenge that does not exist, or a proof for an issued challenge whose
// signature is not good. Nothing is stored, and no challenge is used up by the
// refusal.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key/ed25519keytest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// challengeRows keeps the challenges of a test in memory.
type challengeRows struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.RegistrationChallenge
}

func (m *challengeRows) Create(_ context.Context, c *domain.RegistrationChallenge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *c
	m.rows[c.ID] = &copied
	return nil
}

func (m *challengeRows) Consume(_ context.Context, id uuid.UUID, now time.Time) (*domain.RegistrationChallenge, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[id]
	if !ok {
		return nil, false, domain.ErrRegistrationChallengeNotFound
	}
	first := row.UsedAt == nil
	if first {
		row.UsedAt = &now
	}
	copied := *row
	return &copied, first, nil
}

func (m *challengeRows) DeleteExpired(context.Context, time.Time) (int64, error) { return 0, nil }

func (m *challengeRows) used(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[id].UsedAt != nil
}

// postCreateAgentWithKey registers an agent that supplies publicKey and the
// proof the build function returns (nil for none), as a signed-in user of an
// organization in monitoring mode, and returns the status and the answer.
func postCreateAgentWithKey(t *testing.T, repo *registeringAgentRepo, publicKey string,
	build func(rows *challengeRows, svc *application.RegistrationChallengeService, orgID, userID uuid.UUID) any) (int, map[string]any) {
	t.Helper()
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	require.NoError(t, err)
	agentService := application.NewAgentService(repo, failingTrustCalculator{}, nil, vault, nil, nil, nil, nil, nil, nil, &MockOrganizationRepository{}, nil)
	rows := &challengeRows{rows: map[uuid.UUID]*domain.RegistrationChallenge{}}
	challenges := application.NewRegistrationChallengeService(rows)
	agentService.SetRegistrationChallenges(challenges)

	handler := NewAgentHandlerWithInterfaces(agentService, &MockMCPServiceImpl{}, &MockAuditServiceImpl{}, &MockAPIKeyServiceImpl{},
		nil, &MockAlertServiceImpl{}, &MockVerificationEventServiceImpl{}, &MockCapabilityServiceImpl{}, &MockTagServiceImpl{},
		&MockOrganizationRepository{}, &MockMCPAttestationServiceImpl{})
	orgID, userID := uuid.New(), uuid.New()
	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/api/v1/agents", handler.CreateAgent)

	body := map[string]any{"name": "keyed", "displayName": "Keyed", "agentType": "ai_agent", "description": "d", "publicKey": publicKey}
	if build != nil {
		if proof := build(rows, challenges, orgID, userID); proof != nil {
			body["possessionProof"] = proof
		}
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/api/v1/agents", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer map[string]any
	require.NoError(t, json.Unmarshal(out, &answer), "the answer is JSON: %s", out)
	return resp.StatusCode, answer
}

func refusedKeys(t *testing.T) map[string]string {
	t.Helper()
	genuine, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	enc := base64.StdEncoding.EncodeToString
	// The non-canonical class of the key helper's own tests (TestCanonical,
	// "y = 2^255-1"): a y at or above p = 2^255 - 19. It is not small-order, and
	// y mod p = 18 is on the curve, so the canonical check is what refuses it.
	nonCanonical, err := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f")
	require.NoError(t, err)
	keys := map[string]string{
		"not canonical, y = 2^255-1": enc(nonCanonical),
		"off the curve":              enc(ed25519keytest.OffCurve()),
		"31 bytes":                   enc(genuine[:31]),
		"33 bytes":                   enc(append(append([]byte{}, genuine...), 0)),
		"not base64":                 "not base64!",
		"genuine, unpadded":          base64.RawStdEncoding.EncodeToString(genuine),
	}
	for i, small := range ed25519key.SmallOrderEncodings() {
		keys["small order "+string(rune('a'+i))] = enc(small)
	}
	return keys
}

func TestCreateAgent_ARefusedPublicKeyIsAnsweredWithTheValidationErrorAndStoresNothing(t *testing.T) {
	proofs := map[string]func(*challengeRows, *application.RegistrationChallengeService, uuid.UUID, uuid.UUID) any{
		"no proof": nil,
		"a proof for no challenge": func(*challengeRows, *application.RegistrationChallengeService, uuid.UUID, uuid.UUID) any {
			return map[string]string{"challengeId": uuid.NewString(), "signature": base64.StdEncoding.EncodeToString(make([]byte, 64))}
		},
		"a proof whose signature is not good": func(_ *challengeRows, svc *application.RegistrationChallengeService, orgID, userID uuid.UUID) any {
			issued, err := svc.Issue(context.Background(), orgID, userID)
			require.NoError(t, err)
			return map[string]string{"challengeId": issued.ID.String(), "signature": base64.StdEncoding.EncodeToString(make([]byte, 64))}
		},
	}
	for keyName, key := range refusedKeys(t) {
		for proofName, build := range proofs {
			t.Run(keyName+", "+proofName, func(t *testing.T) {
				repo := &registeringAgentRepo{}

				status, answer := postCreateAgentWithKey(t, repo, key, build)

				assert.Equal(t, fiber.StatusBadRequest, status, "answer: %v", answer)
				assert.Equal(t, map[string]any{"error": "publicKey must be a base64-encoded 32-byte Ed25519 public key"}, answer)
				assert.Empty(t, repo.inserted, "nothing is stored")
			})
		}
	}
}

func TestCreateAgent_AGenuinePublicKeyIsStoredAsSent(t *testing.T) {
	genuine, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	key := base64.StdEncoding.EncodeToString(genuine)
	repo := &registeringAgentRepo{}

	status, answer := postCreateAgentWithKey(t, repo, key, nil)

	require.Equal(t, fiber.StatusCreated, status, "answer: %v", answer)
	require.Len(t, repo.inserted, 1)
	require.NotNil(t, repo.inserted[0].PublicKey)
	assert.Equal(t, key, *repo.inserted[0].PublicKey)
	assert.Equal(t, domain.AgentStatusPending, repo.inserted[0].Status)
}
