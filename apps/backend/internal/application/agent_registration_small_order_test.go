package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key/ed25519keytest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// forgedSmallOrderSignature is the signature (R = the identity point, S = 0)
// that plain ed25519.Verify accepts under a small-order key for a message whose
// hash is a multiple of the key's order.
func forgedSmallOrderSignature() []byte {
	sig := make([]byte, ed25519.SignatureSize)
	sig[0] = 1
	return sig
}

// chooseChallengeForForgery sets the bytes of the stored challenge so that
// plain ed25519.Verify accepts the forged signature over the proof message of
// publicKey, and returns the proof carrying that signature.
func (r *proofRig) chooseChallengeForForgery(t *testing.T, issued *IssuedRegistrationChallenge, key []byte, publicKey string) *PossessionProof {
	t.Helper()
	sig := forgedSmallOrderSignature()
	for i := 0; i < 10000; i++ {
		challenge := make([]byte, 32)
		challenge[0], challenge[1] = byte(i), byte(i>>8)
		message := RegistrationProofMessage(r.orgID, issued.ID, challenge, publicKey)
		if ed25519.Verify(ed25519.PublicKey(key), message, sig) {
			r.store.mu.Lock()
			r.store.rows[issued.ID].Challenge = challenge
			r.store.mu.Unlock()
			return &PossessionProof{
				ChallengeID: issued.ID.String(),
				Signature:   base64.StdEncoding.EncodeToString(sig),
			}
		}
	}
	t.Fatalf("no challenge found for which the forged signature verifies under %x", key)
	return nil
}

func TestAForgedProofForASmallOrderKeyLeavesTheAgentPending(t *testing.T) {
	encodings := ed25519key.SmallOrderEncodings()
	require.Len(t, encodings, 14)
	for _, key := range encodings {
		key := key
		t.Run(base64.StdEncoding.EncodeToString(key), func(t *testing.T) {
			rig := newProofRig(t, domain.EnforcementModeMonitoring)
			publicKey := base64.StdEncoding.EncodeToString(key)
			issued := rig.issue(t)
			proof := rig.chooseChallengeForForgery(t, issued, key, publicKey)

			agent := rig.register(t, "forged", publicKey, proof)

			requirePending(t, agent, PendingReasonProofNotAccepted)
			assert.True(t, rig.store.used(issued.ID), "the challenge is used up")
		})
	}
}

// An encoding with a canonical y that has no matching x is not a point. The
// proof presented with it is not accepted, whatever the signature, and the
// agent registered with it stays pending.
func TestAProofForAnOffCurveKeyIsNotAcceptedAndLeavesTheAgentPending(t *testing.T) {
	key := ed25519keytest.OffCurve()
	require.False(t, ed25519key.Acceptable(key), "the helper refuses the off-curve key")
	publicKey := base64.StdEncoding.EncodeToString(key)

	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	issued := rig.issue(t)
	proof := &PossessionProof{
		ChallengeID: issued.ID.String(),
		Signature:   base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)),
	}

	outcome := rig.challenges.CheckPossessionProof(context.Background(), rig.orgID, rig.userID, publicKey, proof)

	assert.Equal(t, PendingReasonProofNotAccepted, outcome.reason)
	assert.Empty(t, outcome.provenKey)
	assert.True(t, rig.store.used(issued.ID), "the challenge is used up")
	agent := &domain.Agent{Name: "a", DisplayName: "A", Description: "d", TrustScore: 0.5, PublicKey: &publicKey}
	decision := rig.svc.decideRegistrationVerification(rig.orgID, agent, outcome)
	assert.False(t, decision.Verified)
	assert.Equal(t, PendingReasonProofNotAccepted, decision.PendingReason)
}

func TestAGenuineKeyWithAGenuineProofIsStillVerified(t *testing.T) {
	rig := newProofRig(t, domain.EnforcementModeMonitoring)
	priv, pub := newKey(t)
	issued := rig.issue(t)

	agent := rig.register(t, "genuine", pub, signed(t, priv, rig.orgID, issued, pub))

	assert.Equal(t, domain.AgentStatusVerified, agent.Status)
	assert.Empty(t, agent.RegistrationPendingReason)
}

// replacementRepo serves one agent and records every update.
type replacementRepo struct {
	domain.AgentRepository
	agent   *domain.Agent
	updates int
}

func (r *replacementRepo) GetByID(uuid.UUID) (*domain.Agent, error) {
	copied := *r.agent
	return &copied, nil
}

func (r *replacementRepo) Update(*domain.Agent) error {
	r.updates++
	return nil
}

func TestKeyReplacementRefusesTheFourteenSmallOrderEncodingsAndStoresNothing(t *testing.T) {
	_, current := newKey(t)
	for _, key := range ed25519key.SmallOrderEncodings() {
		key := key
		t.Run(base64.StdEncoding.EncodeToString(key), func(t *testing.T) {
			repo := &replacementRepo{agent: &domain.Agent{ID: uuid.New(), PublicKey: &current}}
			svc := &AgentService{agentRepo: repo}

			err := svc.UpdateAgentPublicKey(context.Background(), repo.agent.ID, base64.StdEncoding.EncodeToString(key), "jwt")

			assert.ErrorIs(t, err, ErrPublicKeyNotAccepted)
			assert.Zero(t, repo.updates, "nothing is stored")
			assert.Equal(t, current, *repo.agent.PublicKey)
		})
	}
}

func TestKeyReplacementRefusesAnOffCurveKeyAndStoresNothing(t *testing.T) {
	_, current := newKey(t)
	repo := &replacementRepo{agent: &domain.Agent{ID: uuid.New(), PublicKey: &current}}
	svc := &AgentService{agentRepo: repo}
	offCurve := base64.StdEncoding.EncodeToString(ed25519keytest.OffCurve())

	err := svc.UpdateAgentPublicKey(context.Background(), repo.agent.ID, offCurve, "jwt")

	assert.ErrorIs(t, err, ErrPublicKeyNotAccepted)
	assert.Zero(t, repo.updates, "nothing is stored")
	assert.Equal(t, current, *repo.agent.PublicKey)
}

func TestKeyReplacementDecodesAndLengthChecksTheKeyAsRegistrationDoes(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	_, current := newKey(t)
	for name, key := range map[string]string{
		"not base64":         "not base64!",
		"31 bytes":           base64.StdEncoding.EncodeToString(pub[:31]),
		"33 bytes":           base64.StdEncoding.EncodeToString(append(append([]byte{}, pub...), 0)),
		"non-canonical text": base64.StdEncoding.EncodeToString(pub)[:43] + "B=",
		"unpadded":           base64.RawStdEncoding.EncodeToString(pub),
	} {
		t.Run(name, func(t *testing.T) {
			repo := &replacementRepo{agent: &domain.Agent{ID: uuid.New(), PublicKey: &current}}
			svc := &AgentService{agentRepo: repo}

			err := svc.UpdateAgentPublicKey(context.Background(), repo.agent.ID, key, "jwt")

			assert.ErrorIs(t, err, ErrPublicKeyNotAccepted)
			assert.Zero(t, repo.updates)
		})
	}
}

func TestKeyReplacementStoresAGenuineKey(t *testing.T) {
	_, current := newKey(t)
	_, fresh := newKey(t)
	repo := &replacementRepo{agent: &domain.Agent{ID: uuid.New(), PublicKey: &current}}
	svc := &AgentService{agentRepo: repo}

	require.NoError(t, svc.UpdateAgentPublicKey(context.Background(), repo.agent.ID, fresh, "jwt"))
	assert.Equal(t, 1, repo.updates)
}
