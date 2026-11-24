package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// With an audit queue, the verification route answers without waiting for the
// audit store, and every decision it made (approved, denied, and refused for
// the agent's status) is still stored once the queue is closed, as main closes
// it at shutdown, even when the store fails some writes first.

// queuedDecisionStore keeps the audit rows the queue writes. Its first
// failWrites writes fail, and it counts the writes it was asked for.
type queuedDecisionStore struct {
	domain.AuditLogRepository

	mu         sync.Mutex
	rows       map[uuid.UUID]*domain.AuditLog
	writes     int
	failWrites int
}

func (s *queuedDecisionStore) CreateBatch(logs []*domain.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.failWrites > 0 {
		s.failWrites--
		return errors.New("connection reset by peer")
	}
	for _, entry := range logs {
		s.rows[entry.ID] = entry
	}
	return nil
}

func (s *queuedDecisionStore) Create(entry *domain.AuditLog) error {
	return s.CreateBatch([]*domain.AuditLog{entry})
}

func (s *queuedDecisionStore) snapshot() (map[uuid.UUID]*domain.AuditLog, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[uuid.UUID]*domain.AuditLog, len(s.rows))
	for id, row := range s.rows {
		out[id] = row
	}
	return out, s.writes
}

func TestCreateVerification_WithAnAuditQueueEveryDecisionIsStored(t *testing.T) {
	prevLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prevLog) })

	activePub, activePriv := mustKeypair(t)
	active := refusalAgent(domain.AgentStatusVerified, activePub)
	active.CreatedBy = uuid.New()
	suspendedPub, suspendedPriv := mustKeypair(t)
	suspended := suspendedAgentWithCreator(suspendedPub)

	// Decisions alternate: approved, then denied.
	var mu sync.Mutex
	calls := 0
	svc := &MockAgentServiceForVerificationImpl{}
	lookup := agentsByID(active, suspended)
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return lookup(id) }
	svc.VerifyCapabilityFunc = func(context.Context, uuid.UUID, string, string, map[string]interface{}, string) (bool, string, uuid.UUID, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls%2 == 1 {
			return true, "", uuid.New(), nil
		}
		return false, "Agent does not have permission for capability 'read_file'", uuid.New(), nil
	}
	svc.HasCapabilityNoAlertFunc = func(context.Context, uuid.UUID, string, string) (bool, error) { return true, nil }
	audit := &MockAuditServiceForVerificationImpl{LogFunc: func(context.Context, *domain.AuditLog) error {
		t.Error("a decision's audit entry was written through the audit service, not the queue")
		return nil
	}}
	orgs := &MockOrganizationRepositoryerImpl{GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
		return &domain.Organization{ID: id, EnforcementMode: domain.EnforcementModeStrict}, nil
	}}

	// The first batch and the first entry written on its own fail.
	store := &queuedDecisionStore{rows: map[uuid.UUID]*domain.AuditLog{}, failWrites: 2}
	// An hour's interval: nothing is written while the requests are answered.
	queue := application.NewAuditQueue(store, application.AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour})
	h := NewVerificationHandlerWithInterfaces(svc, audit, &MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{}, orgs).WithAuditQueue(queue)
	app := fiber.New()
	app.Post("/api/v1/sdk-api/verifications", h.CreateVerification)

	const decisions, refusals = 10, 3
	answered := map[string]int{}
	for i := 0; i < decisions; i++ {
		code, body := createVerificationCase{agentID: active.ID, signer: activePriv, presented: activePub}.do(t, app)
		var resp VerificationResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp), body)
		if i%2 == 0 {
			require.Equal(t, fiber.StatusCreated, code, body)
			require.Equal(t, "approved", resp.Status)
		} else {
			require.Equal(t, fiber.StatusForbidden, code, body)
			require.Equal(t, "denied", resp.Status)
		}
		answered[resp.Status]++
	}
	for i := 0; i < refusals; i++ {
		code, body := createVerificationCase{agentID: suspended.ID, signer: suspendedPriv, presented: suspendedPub}.do(t, app)
		require.Equal(t, fiber.StatusForbidden, code, body)
	}

	// Every request was answered and no write was made yet: the audit write
	// is off the request path.
	rows, writes := store.snapshot()
	require.Empty(t, rows)
	require.Zero(t, writes)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, queue.Close(ctx))

	rows, _ = store.snapshot()
	require.Len(t, rows, decisions+refusals, "every decision and every refusal is stored")
	stored := map[string]int{}
	refused := 0
	for _, row := range rows {
		require.NotNil(t, row.AgentID)
		if row.Action == verificationRefusedAction {
			assert.Equal(t, suspended.ID, *row.AgentID)
			refused++
			continue
		}
		assert.Equal(t, active.ID, *row.AgentID)
		assert.Equal(t, active.OrganizationID, row.OrganizationID)
		approved, _ := row.Metadata["autoApproved"].(bool)
		if approved {
			stored["approved"]++
			assert.NotContains(t, row.Metadata, "denialReason")
		} else {
			stored["denied"]++
			assert.Equal(t, "Agent does not have permission for capability 'read_file'", row.Metadata["denialReason"])
		}
	}
	assert.Equal(t, refusals, refused)
	assert.Equal(t, answered, stored, "each answered decision is stored with the outcome the caller was given")
	assert.EqualValues(t, decisions+refusals, queue.Written())
	assert.Zero(t, queue.Lost())
}

func TestCreateVerification_WithoutAnAuditQueueTheDecisionIsWrittenThroughTheAuditService(t *testing.T) {
	pub, priv := mustKeypair(t)
	suspended := suspendedAgentWithCreator(pub)
	ra := newRefusalAuditApp(domain.EnforcementModeStrict, nil, nil, suspended)

	code, body := createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub}.do(t, ra.app)
	require.Equal(t, fiber.StatusForbidden, code, body)
	require.Len(t, ra.logged, 1)
}
