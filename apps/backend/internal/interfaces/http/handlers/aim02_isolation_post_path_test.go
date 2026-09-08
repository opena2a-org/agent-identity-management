package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-02 — the HTTP half of "no write path can produce a verified isolation
// attestation".
//
// The unverified ceiling in calculateExecutionIsolation is only worth anything
// if `verified` is unreachable from a request. This test drives the real POST
// path — fiber route, real handler, real application.TrustCalculator ingest,
// real repository semantics for the verified column — with a body that asks for
// verification outright, and inspects the row that comes out the other end.
// The other two layers are covered next to the code they constrain:
// TestAIM02_NoVerifiedWritePath (ingest) and TestAIM02_CreateNeverWritesVerified
// (SQL statement).

// aim02CapturingRepo records what the ingest path persists. Create mirrors the
// SQL INSERT, which writes the literal FALSE for verified rather than binding
// the struct field, so the fake cannot be more permissive than the real table.
type aim02CapturingRepo struct {
	rows []*domain.IsolationAttestation
}

func (r *aim02CapturingRepo) Create(a *domain.IsolationAttestation) error {
	stored := *a
	stored.Verified = false
	stored.VerifiedBy = nil
	stored.VerifiedAt = nil
	r.rows = append(r.rows, &stored)
	return nil
}

func (r *aim02CapturingRepo) GetLatest(agentID uuid.UUID) (*domain.IsolationAttestation, error) {
	if len(r.rows) == 0 {
		return nil, nil
	}
	return r.rows[len(r.rows)-1], nil
}

func (r *aim02CapturingRepo) GetHistory(agentID uuid.UUID, limit int) ([]*domain.IsolationAttestation, error) {
	return r.rows, nil
}

func TestAIM02_PostPathCannotProduceVerifiedRow(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()

	repo := &aim02CapturingRepo{}
	// The genuine ingest, not a stand-in: the handler's write reaches
	// application.RecordIsolationAttestation exactly as it does in production.
	calculator := &application.TrustCalculator{}
	calculator.SetIsolationRepo(repo)

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "test-agent"}, nil
		},
	}
	trustMock := &MockTrustCalculatorServicerImpl{
		RecordIsolationAttestationFunc: func(ctx context.Context, aID uuid.UUID, s domain.SandboxType, n domain.NetworkIsolation, f domain.FilesystemIsolation, p domain.ProcessIsolation) (*domain.IsolationAttestation, error) {
			return calculator.RecordIsolationAttestation(ctx, aID, s, n, f, p)
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := agentAuthIsolationApp(handler.SubmitIsolationAttestation, orgID, agentID)

	t.Run("AIM-02.AC3 a POST demanding verification still writes an unverified row", func(t *testing.T) {
		// Every shape the body could take to reach the column: the snake_case
		// column names, the camelCase JSON tags on the domain struct, and a
		// nested object in case a future binder flattened one.
		body := `{
			"sandbox":"firecracker","network":"airgap","filesystem":"readonly","process":"full",
			"verified":true,"verifiedBy":"attacker","verifiedAt":"2026-08-29T00:00:00Z",
			"verified_by":"attacker","verified_at":"2026-08-29T00:00:00Z",
			"attestation":{"verified":true},"score":1.0
		}`

		resp, err := app.Test(submitIsolationRequest(agentID, body))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, fiber.StatusCreated, resp.StatusCode,
			"the posture is valid, so the request succeeds — it is the verification that must be dropped")

		require.Len(t, repo.rows, 1)
		row := repo.rows[0]
		assert.False(t, row.Verified,
			"a self-report that asks to be trusted is still a self-report")
		assert.Nil(t, row.VerifiedBy)
		assert.Nil(t, row.VerifiedAt)

		// The score is computed from the posture, so the body's "score":1.0 is
		// ignored too — it happens to coincide here because the claimed posture
		// really does score 1.0. What matters is that the READ side clips it.
		assert.InDelta(t, 1.0, row.Score, 1e-9,
			"the stored row keeps the honest posture score; the ceiling is applied on read")
	})

	t.Run("AIM-02.AC3 the created response exposes no verification the caller could set", func(t *testing.T) {
		resp, err := app.Test(submitIsolationRequest(agentID,
			`{"sandbox":"docker","network":"namespace","filesystem":"readonly","process":"seccomp","verified":true}`))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		// Nothing in the 201 body echoes the caller's claim back as fact. An
		// echoed `verified: true` would be a lie an SDK could cache and show an
		// operator even though the stored row says otherwise.
		for _, k := range []string{"verified", "verifiedBy", "verifiedAt", "verified_by", "verified_at"} {
			assert.NotContains(t, result, k,
				"the attestation response must not carry a verification field the caller supplied")
		}
	})

	t.Run("AIM-02.AC3 a verified row is superseded by the next self-report", func(t *testing.T) {
		// Simulate the only future in which a verified row exists at all (a
		// Phase-2 verifier wrote one) and confirm the POST path does not let it
		// carry forward: the newest row is the one the scorer reads, and the
		// POST path can only ever append an unverified one.
		verifier := "tee-attestation"
		repo.rows = append(repo.rows, &domain.IsolationAttestation{
			ID: uuid.New(), AgentID: agentID, Score: 1.0, Verified: true, VerifiedBy: &verifier,
		})

		resp, err := app.Test(submitIsolationRequest(agentID,
			`{"sandbox":"firecracker","network":"airgap","filesystem":"readonly","process":"full"}`))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, fiber.StatusCreated, resp.StatusCode)

		latest, err := repo.GetLatest(agentID)
		require.NoError(t, err)
		assert.False(t, latest.Verified,
			"re-attesting must start unverified — verification is bound to a row, not to the agent")
		assert.Nil(t, latest.VerifiedBy)
	})
}
