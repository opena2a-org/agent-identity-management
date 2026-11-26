package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// These tests pin the rule for verifying an agent at registration, with the
// criterion in agent_registration_verification.go. In an organization in
// monitoring mode, an agent that holds its own key is verified at registration
// on one kind of evidence: its signature over a server-issued, single-use,
// time-bounded challenge, checked against the public key of the same request.
// Anything else leaves it pending, never as an error, and the challenge a
// registration presents is used up whatever the check finds. In strict mode
// nothing changes: the agent stays pending.

// memoryChallengeStore keeps challenges in memory with the semantics of
// RegistrationChallengeRepository: Consume decides first use under one lock.
type memoryChallengeStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.RegistrationChallenge
}

func newMemoryChallengeStore() *memoryChallengeStore {
	return &memoryChallengeStore{rows: map[uuid.UUID]*domain.RegistrationChallenge{}}
}

func (m *memoryChallengeStore) Create(_ context.Context, c *domain.RegistrationChallenge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *c
	m.rows[c.ID] = &copied
	return nil
}

func (m *memoryChallengeStore) Consume(_ context.Context, id uuid.UUID, now time.Time) (*domain.RegistrationChallenge, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[id]
	if !ok {
		return nil, false, domain.ErrRegistrationChallengeNotFound
	}
	firstUse := row.UsedAt == nil
	if firstUse {
		used := now
		row.UsedAt = &used
	}
	copied := *row
	return &copied, firstUse, nil
}

func (m *memoryChallengeStore) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for id, row := range m.rows {
		if !row.ExpiresAt.After(now) {
			delete(m.rows, id)
			n++
		}
	}
	return n, nil
}

func (m *memoryChallengeStore) used(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[id] != nil && m.rows[id].UsedAt != nil
}

// proofAgentRepo keeps the agents a registration stores.
type proofAgentRepo struct {
	domain.AgentRepository
	mu     sync.Mutex
	stored []*domain.Agent
}

func (r *proofAgentRepo) Create(agent *domain.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *agent
	r.stored = append(r.stored, &copied)
	return nil
}

func (r *proofAgentRepo) Update(agent *domain.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *agent
	r.stored = append(r.stored, &copied)
	return nil
}

func (r *proofAgentRepo) last() *domain.Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stored[len(r.stored)-1]
}

type proofTrustCalculator struct {
	domain.TrustScoreCalculator
	score float64
}

func (c proofTrustCalculator) Calculate(*domain.Agent) (*domain.TrustScore, error) {
	return &domain.TrustScore{ID: uuid.New(), Score: c.score}, nil
}

type proofTrustScoreRepo struct{ domain.TrustScoreRepository }

func (proofTrustScoreRepo) Create(*domain.TrustScore) error { return nil }

type proofOrgRepo struct {
	domain.OrganizationRepository
	orgs map[uuid.UUID]*domain.Organization
}

func (r *proofOrgRepo) GetByID(id uuid.UUID) (*domain.Organization, error) {
	if org, ok := r.orgs[id]; ok {
		return org, nil
	}
	return nil, fmt.Errorf("organization %s not found", id)
}

// proofRig is an agent service in one organization, with a challenge service
// whose clock the test sets.
type proofRig struct {
	svc        *AgentService
	challenges *RegistrationChallengeService
	store      *memoryChallengeStore
	agents     *proofAgentRepo
	orgID      uuid.UUID
	userID     uuid.UUID
	clock      *time.Time
}

func newProofRig(t *testing.T, mode domain.EnforcementMode) *proofRig {
	t.Helper()
	rig := &proofRig{orgID: uuid.New(), userID: uuid.New(), store: newMemoryChallengeStore(), agents: &proofAgentRepo{}}
	start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	rig.clock = &start
	rig.challenges = NewRegistrationChallengeService(rig.store)
	rig.challenges.now = func() time.Time { return *rig.clock }
	rig.svc = &AgentService{
		agentRepo:              rig.agents,
		trustCalc:              proofTrustCalculator{score: 0.5},
		trustScoreRepo:         proofTrustScoreRepo{},
		orgRepo:                &proofOrgRepo{orgs: map[uuid.UUID]*domain.Organization{rig.orgID: {ID: rig.orgID, EnforcementMode: mode}}},
		registrationChallenges: rig.challenges,
	}
	return rig
}

// issue issues a challenge to the rig's organization and user.
func (r *proofRig) issue(t *testing.T) *IssuedRegistrationChallenge {
	t.Helper()
	return r.issueTo(t, r.orgID, r.userID)
}

func (r *proofRig) issueTo(t *testing.T, orgID, userID uuid.UUID) *IssuedRegistrationChallenge {
	t.Helper()
	issued, err := r.challenges.Issue(context.Background(), orgID, userID)
	require.NoError(t, err)
	return issued
}

func newKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return priv, base64.StdEncoding.EncodeToString(pub)
}

// signed is the proof an agent holding priv makes of issued, for the
// registration of publicKey in organization orgID.
func signed(t *testing.T, priv ed25519.PrivateKey, orgID uuid.UUID, issued *IssuedRegistrationChallenge, publicKey string) *PossessionProof {
	t.Helper()
	challenge, err := base64.StdEncoding.DecodeString(issued.Challenge)
	require.NoError(t, err)
	message := RegistrationProofMessage(orgID, issued.ID, challenge, publicKey)
	return &PossessionProof{
		ChallengeID: issued.ID.String(),
		Signature:   base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message)),
	}
}

func (r *proofRig) register(t *testing.T, name, publicKey string, proof *PossessionProof) *domain.Agent {
	t.Helper()
	agent, err := r.svc.CreateAgent(context.Background(), &CreateAgentRequest{
		Name: name, DisplayName: name, Description: "Writes the weekly report",
		AgentType: domain.AgentTypeLangChain, PublicKey: publicKey, PossessionProof: proof,
	}, r.orgID, r.userID, nil, nil, "")
	require.NoError(t, err, "a proof that is not good is not an error")
	require.NotNil(t, agent)
	return agent
}

func requirePending(t *testing.T, agent *domain.Agent, reason string) {
	t.Helper()
	assert.Equal(t, domain.AgentStatusPending, agent.Status)
	assert.Nil(t, agent.VerifiedAt)
	assert.Equal(t, reason, agent.RegistrationPendingReason)
}

func TestASelfKeyedAgentWithAGoodProofIsVerifiedInMonitoringMode(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "self-keyed", pub, signed(t, priv, rig.orgID, issued, pub))

	assert.Equal(t, domain.AgentStatusVerified, agent.Status)
	assert.NotNil(t, agent.VerifiedAt)
	assert.Empty(t, agent.RegistrationPendingReason)
	assert.Nil(t, agent.EncryptedPrivateKey, "the server holds no private key of the agent")
	assert.Equal(t, domain.AgentStatusVerified, rig.agents.last().Status, "the verified status is the one stored")
	assert.True(t, rig.store.used(issued.ID))
}

func TestASelfKeyedAgentWithNoProofStaysPending(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	_, pub := newKey(t)

	agent := rig.register(t, "no-proof", pub, nil)

	requirePending(t, agent, PendingReasonNoProof)
}

func TestAnAgentWhoseKeyTheServerGeneratedIsVerifiedWithoutAProofAsBefore(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	agent := &domain.Agent{Name: "a", DisplayName: "A", Description: "d", TrustScore: 0.5,
		PublicKey: strPtr("k"), EncryptedPrivateKey: strPtr("sealed")}

	decision := rig.svc.decideRegistrationVerification(rig.orgID, agent, possessionProofOutcome{reason: PendingReasonNoProof})

	assert.True(t, decision.Verified)
	assert.True(t, rig.svc.shouldAutoVerifyAgent(agent))
}

func TestAnExpiredChallengeLeavesTheAgentPendingAndIsUsedUp(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)
	proof := signed(t, priv, rig.orgID, issued, pub)
	*rig.clock = rig.clock.Add(domain.RegistrationChallengeLifetime)

	agent := rig.register(t, "expired", pub, proof)

	requirePending(t, agent, PendingReasonProofExpired)
	assert.True(t, rig.store.used(issued.ID))
}

// slowConsumeStore moves the clock while it uses the challenge up, as a store
// that is slow to answer does.
type slowConsumeStore struct {
	*memoryChallengeStore
	clock *time.Time
	delay time.Duration
}

func (s slowConsumeStore) Consume(ctx context.Context, id uuid.UUID, now time.Time) (*domain.RegistrationChallenge, bool, error) {
	challenge, firstUse, err := s.memoryChallengeStore.Consume(ctx, id, now)
	*s.clock = s.clock.Add(s.delay)
	return challenge, firstUse, err
}

func TestAChallengeThatExpiresWhileItIsBeingUsedLeavesTheAgentPendingAsExpired(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	rig.challenges.repo = slowConsumeStore{memoryChallengeStore: rig.store, clock: rig.clock, delay: time.Second}
	priv, pub := newKey(t)
	issued := rig.issue(t)
	proof := signed(t, priv, rig.orgID, issued, pub)
	// One second before the expiry when the registration starts.
	*rig.clock = issued.ExpiresAt.Add(-time.Second)

	agent := rig.register(t, "expires-during-use", pub, proof)

	requirePending(t, agent, PendingReasonProofExpired)
	assert.True(t, rig.store.used(issued.ID))
}

func TestAReusedChallengeLeavesTheSecondAgentPending(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)
	proof := signed(t, priv, rig.orgID, issued, pub)

	first := rig.register(t, "first", pub, proof)
	second := rig.register(t, "second", pub, proof)

	assert.Equal(t, domain.AgentStatusVerified, first.Status)
	requirePending(t, second, PendingReasonProofUsed)
}

func TestAChallengeIssuedToAnotherOrganizationLeavesTheAgentPendingAndIsUsedUp(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	otherOrg := uuid.New()
	issued := rig.issueTo(t, otherOrg, rig.userID)
	// Signed for the organization it was issued to, and signed for the one it
	// is presented in: neither is accepted.
	for _, signedFor := range []uuid.UUID{otherOrg, rig.orgID} {
		rig.store.mu.Lock()
		rig.store.rows[issued.ID].UsedAt = nil
		rig.store.mu.Unlock()

		agent := rig.register(t, "foreign-org-"+signedFor.String()[:8], pub, signed(t, priv, signedFor, issued, pub))

		requirePending(t, agent, PendingReasonProofNotAccepted)
		assert.True(t, rig.store.used(issued.ID))
	}
}

func TestAChallengeIssuedToAnotherPrincipalLeavesTheAgentPendingAndIsUsedUp(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issueTo(t, rig.orgID, uuid.New())

	agent := rig.register(t, "foreign-principal", pub, signed(t, priv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonProofNotAccepted)
	assert.True(t, rig.store.used(issued.ID))
}

func TestASignatureByAnotherKeyLeavesTheAgentPendingAndIsUsedUp(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	_, pub := newKey(t)
	otherPriv, _ := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "other-key", pub, signed(t, otherPriv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonProofNotAccepted)
	assert.True(t, rig.store.used(issued.ID))
}

func TestASignatureForAnotherPublicKeyIsNotCheckedAgainstThatKey(t *testing.T) {
	// The agent signs for the key it holds but registers another: the proof is
	// checked against the public key of the same request.
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	_, otherPub := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "swapped-key", otherPub, signed(t, priv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonProofNotAccepted)
	assert.True(t, rig.store.used(issued.ID))
}

func TestASignatureOverOtherBytesLeavesTheAgentPendingAndIsUsedUp(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)

	cases := map[string]func(issued *IssuedRegistrationChallenge, challenge []byte) []byte{
		"the bare challenge": func(_ *IssuedRegistrationChallenge, challenge []byte) []byte { return challenge },
		"bytes the client chose": func(*IssuedRegistrationChallenge, []byte) []byte {
			return []byte("a message the client chose")
		},
		"the message without its protocol line": func(issued *IssuedRegistrationChallenge, challenge []byte) []byte {
			return []byte(rig.orgID.String() + "\n" + issued.ID.String() + "\n" + issued.Challenge + "\n" + pub)
		},
		"the message of another version": func(issued *IssuedRegistrationChallenge, challenge []byte) []byte {
			return []byte("aim-agent-registration-v2\n" + rig.orgID.String() + "\n" + issued.ID.String() + "\n" + issued.Challenge + "\n" + pub)
		},
		"the message without the public key": func(issued *IssuedRegistrationChallenge, challenge []byte) []byte {
			return []byte(registrationProofContext + rig.orgID.String() + "\n" + issued.ID.String() + "\n" + issued.Challenge + "\n")
		},
	}
	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			issued := rig.issue(t)
			challenge, err := base64.StdEncoding.DecodeString(issued.Challenge)
			require.NoError(t, err)
			proof := &PossessionProof{
				ChallengeID: issued.ID.String(),
				Signature:   base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message(issued, challenge))),
			}

			agent := rig.register(t, "other-bytes-"+uuid.NewString()[:8], pub, proof)

			requirePending(t, agent, PendingReasonProofNotAccepted)
			assert.True(t, rig.store.used(issued.ID))
		})
	}
}

func TestAProofThatIsNotWellFormedLeavesTheAgentPending(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)

	cases := map[string]func(good *PossessionProof) *PossessionProof{
		"no challenge id": func(good *PossessionProof) *PossessionProof { return &PossessionProof{Signature: good.Signature} },
		"a challenge id of no row": func(good *PossessionProof) *PossessionProof {
			return &PossessionProof{ChallengeID: uuid.NewString(), Signature: good.Signature}
		},
		"a challenge id that is not a UUID": func(good *PossessionProof) *PossessionProof {
			return &PossessionProof{ChallengeID: "nope", Signature: good.Signature}
		},
		"no signature": func(good *PossessionProof) *PossessionProof { return &PossessionProof{ChallengeID: good.ChallengeID} },
		"a short signature": func(good *PossessionProof) *PossessionProof {
			return &PossessionProof{ChallengeID: good.ChallengeID, Signature: base64.StdEncoding.EncodeToString([]byte("short"))}
		},
	}
	for name, malform := range cases {
		t.Run(name, func(t *testing.T) {
			issued := rig.issue(t)
			agent := rig.register(t, "malformed-"+uuid.NewString()[:8], pub, malform(signed(t, priv, rig.orgID, issued, pub)))
			requirePending(t, agent, PendingReasonProofNotAccepted)
		})
	}

	t.Run("a public key that is not 32 bytes", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("short key"))
		issued := rig.issue(t)
		agent := rig.register(t, "short-key", short, signed(t, priv, rig.orgID, issued, short))
		requirePending(t, agent, PendingReasonProofNotAccepted)
	})
}

func TestAPossessionProofThatIsNotAnObjectIsAProofNotAcceptedAndNotARejectedBody(t *testing.T) {
	for _, body := range []string{`"text"`, `42`, `[1]`, `{"challengeId":7}`, `{}`} {
		var req CreateAgentRequest
		require.NoError(t, json.Unmarshal([]byte(`{"name":"n","possessionProof":`+body+`}`), &req), body)
		require.NotNil(t, req.PossessionProof, body)
		assert.Empty(t, req.PossessionProof.ChallengeID, body)
	}
	var req CreateAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"n","possessionProof":{"challengeId":"c","signature":"s"}}`), &req))
	assert.Equal(t, &PossessionProof{ChallengeID: "c", Signature: "s"}, req.PossessionProof)
}

func TestAChallengeIdIsKeptAndUsedWhateverTheSignatureMemberHolds(t *testing.T) {
	for _, signatureJSON := range []string{`7`, `[1,2]`, `{"a":1}`, `true`, `null`} {
		t.Run(signatureJSON, func(t *testing.T) {
			rig := newProofRig(t, domain.EnforcementModeMonitoring)
			priv, pub := newKey(t)
			issued := rig.issue(t)
			good := signed(t, priv, rig.orgID, issued, pub)

			var req CreateAgentRequest
			body := `{"name":"n","possessionProof":{"challengeId":"` + issued.ID.String() + `","signature":` + signatureJSON + `}}`
			require.NoError(t, json.Unmarshal([]byte(body), &req))
			require.NotNil(t, req.PossessionProof)
			assert.Equal(t, issued.ID.String(), req.PossessionProof.ChallengeID)
			assert.Empty(t, req.PossessionProof.Signature)

			first := rig.register(t, "wrong-type-signature", pub, req.PossessionProof)
			requirePending(t, first, PendingReasonProofNotAccepted)
			assert.True(t, rig.store.used(issued.ID), "a challenge that is named is used")

			second := rig.register(t, "good-signature-after", pub, good)
			requirePending(t, second, PendingReasonProofUsed)
		})
	}

	t.Run("a signature is kept when the challenge id has the wrong type", func(t *testing.T) {
		var proof PossessionProof
		require.NoError(t, json.Unmarshal([]byte(`{"challengeId":7,"signature":"abc"}`), &proof))
		assert.Equal(t, PossessionProof{Signature: "abc"}, proof)
	})
}

func TestStrictModeKeepsASelfKeyedAgentPendingWithAGoodProof(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeStrict)
	priv, pub := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "strict", pub, signed(t, priv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonStrictMode)
	assert.Equal(t, domain.AgentStatusPending, rig.agents.last().Status)
}

func TestAnOrganizationWhoseModeCannotBeReadKeepsASelfKeyedAgentPendingWithAGoodProof(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	rig.svc.orgRepo = &proofOrgRepo{orgs: map[uuid.UUID]*domain.Organization{}}
	priv, pub := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "mode-unknown", pub, signed(t, priv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonModeUnknown)
}

func TestWithoutChallengesWiredAProofIsNotAccepted(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)
	rig.svc.registrationChallenges = nil

	agent := rig.register(t, "unwired", pub, signed(t, priv, rig.orgID, issued, pub))

	requirePending(t, agent, PendingReasonProofNotAccepted)
}

func TestAGoodProofDoesNotWaiveTheOtherConditionsForAutomaticVerification(t *testing.T) {
	priv, pub := newKey(t)

	t.Run("a trust score below the minimum", func(t *testing.T) {
		rig := newProofRig(t, domain.EnforcementModeMonitoring)
		rig.svc.trustCalc = proofTrustCalculator{score: 0.29}
		issued := rig.issue(t)
		agent := rig.register(t, "low-trust", pub, signed(t, priv, rig.orgID, issued, pub))
		requirePending(t, agent, PendingReasonTrustScoreTooLow)
	})

	t.Run("a description left out", func(t *testing.T) {
		rig := newProofRig(t, domain.EnforcementModeMonitoring)
		issued := rig.issue(t)
		agent, err := rig.svc.CreateAgent(context.Background(), &CreateAgentRequest{
			Name: "no-description", DisplayName: "No description", AgentType: domain.AgentTypeLangChain,
			PublicKey: pub, PossessionProof: signed(t, priv, rig.orgID, issued, pub),
		}, rig.orgID, rig.userID, nil, nil, "")
		require.NoError(t, err)
		requirePending(t, agent, PendingReasonMissingMetadata)
	})
}

func TestTheChallengeIsSingleUseUnderTwoConcurrentRegistrations(t *testing.T) {
	for round := 0; round < 20; round++ {
		rig := newProofRig(t, domain.EnforcementModeMonitoring)
		priv, pub := newKey(t)
		issued := rig.issue(t)
		proof := signed(t, priv, rig.orgID, issued, pub)

		var wg sync.WaitGroup
		start := make(chan struct{})
		results := make([]*domain.Agent, 2)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				agent, err := rig.svc.CreateAgent(context.Background(), &CreateAgentRequest{
					Name: fmt.Sprintf("concurrent-%d", i), DisplayName: "Concurrent", Description: "Writes the weekly report",
					AgentType: domain.AgentTypeLangChain, PublicKey: pub, PossessionProof: proof,
				}, rig.orgID, rig.userID, nil, nil, "")
				if err == nil {
					results[i] = agent
				}
			}(i)
		}
		close(start)
		wg.Wait()

		verified := 0
		for _, agent := range results {
			require.NotNil(t, agent)
			if agent.Status == domain.AgentStatusVerified {
				verified++
			} else {
				assert.Equal(t, PendingReasonProofUsed, agent.RegistrationPendingReason)
			}
		}
		assert.Equal(t, 1, verified, "exactly one registration of the two is verified")
	}
}

func TestAChallengeIsFor32BytesValidFiveMinutesAndBoundToItsAsker(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)

	issued := rig.issue(t)

	raw, err := base64.StdEncoding.DecodeString(issued.Challenge)
	require.NoError(t, err)
	assert.Len(t, raw, 32)
	assert.Equal(t, rig.clock.Add(5*time.Minute), issued.ExpiresAt)
	row := rig.store.rows[issued.ID]
	require.NotNil(t, row)
	assert.Equal(t, rig.orgID, row.OrganizationID)
	assert.Equal(t, rig.userID, row.PrincipalID)
	assert.Equal(t, raw, row.Challenge)
	assert.Nil(t, row.UsedAt)
	other := rig.issue(t)
	assert.NotEqual(t, issued.Challenge, other.Challenge)
	assert.NotEqual(t, issued.ID, other.ID)
}

func TestTheReasonAnAgentStaysPendingHoldsNoChallengeAndNoSecret(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)
	proof := signed(t, priv, rig.orgID, issued, pub)
	*rig.clock = rig.clock.Add(time.Hour)

	agent := rig.register(t, "reason", pub, proof)

	for _, secret := range []string{issued.Challenge, proof.Signature, proof.ChallengeID, pub} {
		assert.NotContains(t, agent.RegistrationPendingReason, secret)
	}
}

// The signed bytes are pinned by a fixed vector: the organization, challenge id,
// challenge bytes 0x00..0x1f in standard padded base64, and the public key of the
// Ed25519 key whose seed is 0x20..0x3f, as the API carries them. A change to any
// of the bytes breaks every SDK that signs them.
func TestTheSignedBytesOfAPossessionProofAreAFixedVector(t *testing.T) {
	orgID := uuid.MustParse("0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d")
	challengeID := uuid.MustParse("5d6e7f80-9a1b-4c2d-8e3f-4a5b6c7d8e9f")
	challenge := make([]byte, 32)
	seed := make([]byte, 32)
	for i := range challenge {
		challenge[i] = byte(i)
		seed[i] = byte(0x20 + i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	publicKey := base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	require.Equal(t, "Kay64UG8yvCyLhqU000LxzYeUm0L/hLIl5S8kyKWbdc=", publicKey)

	message := RegistrationProofMessage(orgID, challengeID, challenge, publicKey)

	wantMessage := "aim-agent-registration-v1\n" +
		"0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d\n" +
		"5d6e7f80-9a1b-4c2d-8e3f-4a5b6c7d8e9f\n" +
		"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=\n" +
		publicKey
	assert.Equal(t, wantMessage, string(message))
	assert.Equal(t, "THHeUErKz6CNEl6UcxnlSUgcnvZ9d9tmanZO1IFZ9kr/lgRfCG1x29CmIxTYpw/JRFTUIwffrlauzMIfllFZCQ==", base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message)))
}
