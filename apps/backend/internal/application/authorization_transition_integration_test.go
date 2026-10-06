//go:build integration

package application

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests drive the capability and agent services against the migrated
// Postgres with a transition recorder set:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration -run Transition ./internal/application/...

// transitionKeys is a record key generated when the test runs. A non-nil
// fail makes every signing request fail, which is how these tests take the
// record path down.
type transitionKeys struct {
	mu      sync.Mutex
	public  ed25519.PublicKey
	private ed25519.PrivateKey
	fail    error
}

func (k *transitionKeys) publicKey() record.PublicKey {
	return record.PublicKey{Alg: record.AlgEd25519, Key: append([]byte(nil), k.public...)}
}

func (k *transitionKeys) PublicKey(context.Context) (record.PublicKey, error) {
	return k.publicKey(), nil
}

func (k *transitionKeys) setFail(err error) {
	k.mu.Lock()
	k.fail = err
	k.mu.Unlock()
}

func (k *transitionKeys) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	k.mu.Lock()
	fail := k.fail
	k.mu.Unlock()
	if fail != nil {
		return "", nil, fail
	}
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return k.publicKey().KeyID(), ed25519.Sign(k.private, message), nil
}

type transitionLogs struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *transitionLogs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *transitionLogs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// transitionTrust returns a fixed score, so the services' follow-on trust
// update runs as it does in production.
type transitionTrust struct{}

func (transitionTrust) Calculate(a *domain.Agent) (*domain.TrustScore, error) {
	return &domain.TrustScore{ID: uuid.New(), AgentID: a.ID, Score: 0.5}, nil
}

func (transitionTrust) CalculateFactors(*domain.Agent) (*domain.TrustScoreFactors, error) {
	return &domain.TrustScoreFactors{}, nil
}

type transitionFixture struct {
	db        *sql.DB
	orgID     uuid.UUID
	userID    uuid.UUID
	agentID   uuid.UUID
	keys      *transitionKeys
	reg       *prometheus.Registry
	logs      *transitionLogs
	chainID   string
	rec       *transition.Recorder
	capSvc    *CapabilityService
	agentSvc  *AgentService
	reqSvc    *CapabilityRequestService
	policySvc *SecurityPolicyService
}

func newTransitionFixture(t *testing.T) *transitionFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping authorization transition test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	ctx := context.Background()

	f := &transitionFixture{db: db, agentID: uuid.New(), logs: &transitionLogs{}, reg: prometheus.NewRegistry()}
	f.orgID, f.userID = seedOrgAndUser(t, db, ctx, "transition")
	// Registered after the seed, so it runs before the seed's own cleanup.
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM audit_records WHERE chain_id IN (SELECT id FROM record_chains WHERE organization_id = $1)`, f.orgID)
		_, _ = db.Exec(`DELETE FROM record_chains WHERE organization_id = $1`, f.orgID)
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE organization_id = $1`, f.orgID)
		_, _ = db.Exec(`DELETE FROM trust_scores WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, f.orgID)
		_, _ = db.Exec(`DELETE FROM capability_requests WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, f.orgID)
		_, _ = db.Exec(`DELETE FROM agent_capabilities WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)`, f.orgID)
		_, _ = db.Exec(`DELETE FROM agents WHERE organization_id = $1`, f.orgID)
	})

	f.insertAgent(t, f.agentID)

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	f.keys = &transitionKeys{public: public, private: private}
	f.chainID = startTestChain(t, db, f.orgID, f.keys)

	metrics, err := store.NewMetrics(f.reg)
	require.NoError(t, err)
	logger := log.New(f.logs, "", 0)
	w, err := store.NewWriter(store.Config{DB: db, Keys: f.keys, Key: f.keys.publicKey(), Metrics: metrics, Logger: logger})
	require.NoError(t, err)
	rec, err := transition.NewRecorder(transition.Config{
		Writer: w, DB: db, Issuer: "urn:uuid:" + uuid.NewString(), Logger: logger,
	})
	require.NoError(t, err)

	masterKey := make([]byte, 32)
	_, err = rand.Read(masterKey)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)

	agentRepo := repository.NewAgentRepository(db)
	capRepo := repository.NewCapabilityRepository(sqlx.NewDb(db, "postgres"))
	trustRepo := repository.NewTrustScoreRepository(db)
	f.capSvc = NewCapabilityService(capRepo, agentRepo, repository.NewAuditLogRepository(db), nil, transitionTrust{}, trustRepo)
	f.capSvc.SetTransitionRecorder(rec)
	f.agentSvc = NewAgentService(agentRepo, transitionTrust{}, trustRepo, vault, nil, nil, capRepo, nil, nil, nil, nil, nil)
	f.agentSvc.SetTransitionRecorder(rec)
	f.rec = rec
	f.reqSvc = NewCapabilityRequestService(repository.NewCapabilityRequestRepository(sqlx.NewDb(db, "postgres")),
		capRepo, agentRepo, repository.NewOrganizationRepository(db))
	f.reqSvc.SetTransitionRecorder(rec)
	f.policySvc = NewSecurityPolicyService(nil, nil, nil)
	f.policySvc.SetAgentRepository(agentRepo)
	f.policySvc.SetTransitionRecorder(rec)
	return f
}

// insertAgent adds a verified agent with an Ed25519 key it holds itself to
// the fixture's organization.
func (f *transitionFixture) insertAgent(t *testing.T, id uuid.UUID) {
	t.Helper()
	agentKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = f.db.Exec(`
		INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score,
		                    public_key, key_algorithm, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.5, $4, 'Ed25519', $5, NOW(), NOW())`,
		id, f.orgID, "transition-agent-"+id.String()[:8],
		base64.StdEncoding.EncodeToString(agentKey), f.userID)
	require.NoError(t, err)
}

// startTestChain writes an organization's chain genesis the way the record
// writer's Start does. Start itself refuses while the schema still removes
// audit rows by cascade, which the migrated test database does.
func startTestChain(t *testing.T, db *sql.DB, orgID uuid.UUID, keys *transitionKeys) string {
	t.Helper()
	ctx := context.Background()
	chainID, eventID := uuid.NewString(), uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Microsecond)
	body, err := record.NewGenesis(record.Genesis{
		ChainID:  chainID,
		FirstKey: keys.publicKey(),
		Draft:    record.Draft{EventID: eventID, Timestamp: timestamp},
	})
	require.NoError(t, err)
	rec, err := record.Sign(ctx, body, keys)
	require.NoError(t, err)
	signature, err := base64.StdEncoding.DecodeString(rec.Envelope.Signatures[0].Sig)
	require.NoError(t, err)
	head := body.Head()
	_, err = db.Exec(`INSERT INTO record_chains (id, organization_id, key_id, head_seq, head_hash) VALUES ($1, $2, $3, $4, $5)`,
		chainID, orgID, keys.publicKey().KeyID(), head.Seq, head.Hash)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO audit_records (chain_id, seq, event_id, record_type, recorded_at, record_hash,
		                           payload_type, payload, key_id, signature)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		chainID, head.Seq, eventID, record.TypeChainGenesis, timestamp, head.Hash,
		rec.Envelope.PayloadType, body.Canonical(), rec.Envelope.Signatures[0].KeyID, signature)
	require.NoError(t, err)
	return chainID
}

// transitionRecord is one stored transition, decoded.
type transitionRecord struct {
	Seq      int64
	Trigger  string
	Previous transitionStateJSON
	New      transitionStateJSON
	Actor    string
	Agent    string
	Trace    string
	// Outcome is opena2a.outcome, "" when the record has none.
	Outcome string
}

type transitionStateJSON struct {
	Scope   []string `json:"scope"`
	Opena2a struct {
		GrantedScope []string         `json:"granted_scope"`
		Status       string           `json:"status"`
		Keys         []transition.Key `json:"keys"`
	} `json:"opena2a"`
}

func (f *transitionFixture) transitions(t *testing.T) []transitionRecord {
	t.Helper()
	records, err := store.ReadChain(context.Background(), f.db, f.chainID)
	require.NoError(t, err)
	res, err := record.Verify(records, f.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "the chain does not verify: %v", res.Failure)
	var out []transitionRecord
	for _, rec := range records {
		payload, err := base64.StdEncoding.DecodeString(rec.Envelope.Payload)
		require.NoError(t, err)
		var retained struct {
			Type    string `json:"type"`
			Trigger struct {
				Type string `json:"type"`
			} `json:"trigger"`
			Opena2a struct {
				StateSpace string `json:"state_space"`
				Outcome    string `json:"outcome"`
				Chain      struct {
					Seq int64 `json:"seq"`
				} `json:"chain"`
			} `json:"opena2a"`
		}
		require.NoError(t, json.Unmarshal(payload, &retained))
		if retained.Type == record.TypeChainGenesis {
			continue
		}
		require.Equal(t, transition.RecordType, retained.Type)
		require.Equal(t, transition.StateSpaceAgent, retained.Opena2a.StateSpace)
		var tenant struct {
			TraceID  string              `json:"trace_id"`
			Previous transitionStateJSON `json:"previous_state"`
			New      transitionStateJSON `json:"new_state"`
			Opena2a  struct {
				OrganizationID string `json:"organization_id"`
				SubjectAgentID string `json:"subject_agent_id"`
			} `json:"opena2a"`
		}
		require.NoError(t, json.Unmarshal(rec.TenantPart, &tenant))
		require.Equal(t, f.orgID.String(), tenant.Opena2a.OrganizationID)
		var personal struct {
			Actor string `json:"actor"`
		}
		require.NoError(t, json.Unmarshal(rec.PersonalPart, &personal))
		out = append(out, transitionRecord{
			Seq: retained.Opena2a.Chain.Seq, Trigger: retained.Trigger.Type,
			Previous: tenant.Previous, New: tenant.New, Actor: personal.Actor,
			Agent: tenant.Opena2a.SubjectAgentID, Trace: tenant.TraceID, Outcome: retained.Opena2a.Outcome,
		})
	}
	return out
}

func (f *transitionFixture) failures(t *testing.T, class store.Class, reason store.Reason) float64 {
	t.Helper()
	families, err := f.reg.Gather()
	require.NoError(t, err)
	for _, fam := range families {
		if fam.GetName() != "aim_record_write_failures_total" {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["class"] == string(class) && labels["reason"] == string(reason) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// Every grant, capability revocation, suspension, reactivation, credential
// rotation and agent revocation writes exactly one transition record with the
// trigger of its path and the agent's state before and after. Replaying the
// chain rebuilds the agent's state in the tables, and each previous_state
// equals the new_state before it.
func TestTransitionTriggersOnEveryLifecyclePathReplayToTheTables(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	require.Len(t, before.Keys, 1)
	originalKey := before.Keys[0]
	assert.True(t, strings.HasPrefix(originalKey.ID, "did:key:z6Mk"), originalKey.ID)
	assert.Equal(t, transition.KeyCustodyExternal, originalKey.Custody)

	read, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	autoCtx := transition.WithActor(transition.WithTrigger(ctx, transition.TriggerRequestAutoApproved), transition.Agent(f.agentID))
	_, err = f.capSvc.GrantCapability(autoCtx, f.agentID, "api:call", nil, nil, "")
	require.NoError(t, err)
	require.NoError(t, f.capSvc.RevokeCapability(ctx, read.ID, &f.userID))
	require.NoError(t, f.agentSvc.SuspendAgent(userCtx, f.agentID))
	require.NoError(t, f.agentSvc.ReactivateAgent(userCtx, f.agentID))
	newPublic, _, err := f.agentSvc.RotateCredentials(userCtx, f.agentID)
	require.NoError(t, err)
	require.NoError(t, f.agentSvc.RevokeAgent(userCtx, f.agentID))

	got := f.transitions(t)
	require.Len(t, got, 7, "one record per change")
	user, agent := "user:"+f.userID.String(), "agent:"+f.agentID.String()
	want := []struct {
		trigger, actor, status string
		granted, scope         []string
	}{
		{"direct_grant", user, "verified", []string{"files:read"}, []string{"files:read"}},
		{"request_auto_approved", agent, "verified", []string{"api:call", "files:read"}, []string{"api:call", "files:read"}},
		{"revocation", user, "verified", []string{"api:call"}, []string{"api:call"}},
		{"agent_suspended", user, "suspended", []string{"api:call"}, []string{}},
		{"agent_reactivated", user, "verified", []string{"api:call"}, []string{"api:call"}},
		{"key_rotated", user, "verified", []string{"api:call"}, []string{"api:call"}},
		{"agent_revoked", user, "revoked", []string{"api:call"}, []string{}},
	}
	for i, w := range want {
		r := got[i]
		assert.Equal(t, int64(i+1), r.Seq)
		assert.Equal(t, w.trigger, r.Trigger, "seq %d", r.Seq)
		assert.Equal(t, w.actor, r.Actor, "seq %d", r.Seq)
		assert.Equal(t, f.agentID.String(), r.Agent, "seq %d", r.Seq)
		assert.Len(t, r.Trace, 32, "seq %d", r.Seq)
		assert.Equal(t, w.status, r.New.Opena2a.Status, "seq %d", r.Seq)
		assert.Equal(t, w.granted, r.New.Opena2a.GrantedScope, "seq %d", r.Seq)
		assert.Equal(t, w.scope, r.New.Scope, "seq %d", r.Seq)
		if i == 0 {
			assert.Empty(t, r.Previous.Opena2a.GrantedScope)
			assert.Equal(t, "verified", r.Previous.Opena2a.Status)
		} else {
			assert.Equal(t, got[i-1].New, r.Previous, "seq %d: previous_state is not the new_state before it", r.Seq)
		}
	}

	// The rotation keeps the scope and changes the keys: the new key is the
	// server's, and the key it replaced stays as the previous key.
	rotated := got[5]
	assert.Equal(t, rotated.Previous.Scope, rotated.New.Scope)
	require.Len(t, rotated.New.Opena2a.Keys, 2)
	current, previous := rotated.New.Opena2a.Keys[0], rotated.New.Opena2a.Keys[1]
	assert.Equal(t, transition.KeyRoleCurrent, current.Role)
	assert.Equal(t, transition.KeyCustodyServer, current.Custody)
	rawNew, err := base64.StdEncoding.DecodeString(newPublic)
	require.NoError(t, err)
	assert.NotEqual(t, originalKey.ID, current.ID)
	assert.True(t, strings.HasPrefix(current.ID, "did:key:z6Mk") && len(rawNew) == ed25519.PublicKeySize)
	assert.Equal(t, transition.KeyRolePrevious, previous.Role)
	assert.Equal(t, originalKey.ID, previous.ID)
	assert.Nil(t, previous.GraceUntil, "the rotation sets no grace deadline, and the record says so")

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7}, replayed.Seqs[f.agentID])
	tables, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.True(t, transition.Equal(tables, replayed.States[f.agentID]))
	assert.Equal(t, "revoked", tables.Status)
}

// A change made without a record is found: by the comparison with the tables
// while it is the newest change, and by the continuity check once a recorded
// change follows it.
func TestTransitionTriggerReplayFindsAChangeWithoutARecord(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)

	// A path that writes no record.
	_, err = f.db.Exec(`UPDATE agents SET status = 'suspended' WHERE id = $1`, f.agentID)
	require.NoError(t, err)
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	var mismatch *transition.MismatchError
	require.True(t, errors.As(err, &mismatch), "want *MismatchError, got %v", err)
	assert.Equal(t, f.agentID, mismatch.AgentID)

	require.NoError(t, f.agentSvc.ReactivateAgent(userCtx, f.agentID))
	_, err = transition.Replay(ctx, f.db, f.orgID, f.keys.publicKey())
	var broken *transition.ContinuityError
	require.True(t, errors.As(err, &broken), "want *ContinuityError, got %v", err)
	assert.Equal(t, f.agentID, broken.AgentID)
	assert.Equal(t, int64(2), broken.Seq)
	assert.Equal(t, int64(1), broken.PreviousSeq)
}

// With record writes failing, a widening is refused and changes nothing,
// while a narrowing commits without its record, is counted and writes a
// SECURITY line that names the agent and not the user who acted.
func TestTransitionTriggerOfAReductionCommitsWhenTheRecordPathFails(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	granted, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	var rotationsBefore int
	require.NoError(t, f.db.QueryRow(`SELECT rotation_count FROM agents WHERE id = $1`, f.agentID).Scan(&rotationsBefore))

	f.keys.setFail(errors.New("record key unavailable"))

	_, _, err = f.agentSvc.RotateCredentials(userCtx, f.agentID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "api:call", nil, &f.userID, "")
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	unchanged, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.True(t, transition.Equal(before, unchanged), "a refused widening changed the agent")
	var rotations int
	require.NoError(t, f.db.QueryRow(`SELECT rotation_count FROM agents WHERE id = $1`, f.agentID).Scan(&rotations))
	assert.Equal(t, rotationsBefore, rotations)

	require.NoError(t, f.agentSvc.SuspendAgent(userCtx, f.agentID))
	require.NoError(t, f.capSvc.RevokeCapability(ctx, granted.ID, &f.userID))
	after, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.Equal(t, "suspended", after.Status)
	assert.Empty(t, after.GrantedScope)

	assert.Equal(t, float64(2), f.failures(t, store.ClassExpansion, store.ReasonSigner))
	assert.Equal(t, float64(2), f.failures(t, store.ClassReduction, store.ReasonSigner))
	require.Len(t, f.transitions(t), 1, "only the grant made before the fault has a record")

	var lines []string
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.HasPrefix(line, "SECURITY "+transition.EventTransitionUnrecorded) {
			lines = append(lines, line)
		}
	}
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "trigger=agent_suspended")
	assert.Contains(t, lines[1], "trigger=revocation")
	for _, line := range lines {
		assert.Contains(t, line, "organization="+f.orgID.String())
		assert.Contains(t, line, "subject="+f.agentID.String())
		assert.NotContains(t, line, f.userID.String(), "the line names the user who acted")
	}
}

// The fault is held in the database: with the organization's append lock
// held past the lock timeout, a suspension commits without its record and a
// rotation is refused; with the agent's row held past it, a revocation waits
// for the row and commits.
func TestTransitionTriggerOfAReductionCommitsPastAHeldLock(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = holder.Exec(`SELECT id FROM record_chains WHERE organization_id = $1 FOR UPDATE`, f.orgID)
	require.NoError(t, err)
	_, _, err = f.agentSvc.RotateCredentials(userCtx, f.agentID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	require.NoError(t, f.agentSvc.SuspendAgent(userCtx, f.agentID))
	require.NoError(t, holder.Rollback())
	assert.Equal(t, float64(1), f.failures(t, store.ClassExpansion, store.ReasonLockTimeout))
	assert.Equal(t, float64(1), f.failures(t, store.ClassReduction, store.ReasonLockTimeout))

	holder, err = f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = holder.Exec(`SELECT id FROM agents WHERE id = $1 FOR UPDATE`, f.agentID)
	require.NoError(t, err)
	released := make(chan struct{})
	go func() {
		time.Sleep(store.LockTimeout + 500*time.Millisecond)
		_ = holder.Rollback()
		close(released)
	}()
	require.NoError(t, f.agentSvc.RevokeAgent(userCtx, f.agentID))
	<-released

	state, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.Equal(t, "revoked", state.Status)
	assert.Empty(t, f.transitions(t), "neither reduction has a record")
	assert.Equal(t, 2, strings.Count(f.logs.String(), "SECURITY "+transition.EventTransitionUnrecorded))
}
