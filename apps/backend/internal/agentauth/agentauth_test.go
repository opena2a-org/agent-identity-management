package agentauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type mapSource map[uuid.UUID]*domain.Agent

func (m mapSource) GetAgent(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	if a, ok := m[id]; ok {
		return a, nil
	}
	return nil, errors.New("agent not found")
}

func encoded(raw []byte) *string {
	s := base64.StdEncoding.EncodeToString(raw)
	return &s
}

func TestRefusalBodiesAreTheContractedBytes(t *testing.T) {
	for _, tc := range []struct {
		body any
		want string
	}{
		{KeyNotRecognizedBody(), `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`},
		{StatusDeniedBody(domain.AgentStatusRevoked), `{"error":"Agent is not permitted to authenticate (status: revoked)","reasonCode":"agentStatusDenied"}`},
		{SignatureInvalidBody("Invalid signature"), `{"error":"Invalid signature","reasonCode":"signatureInvalid"}`},
	} {
		got, err := json.Marshal(tc.body)
		require.NoError(t, err)
		require.Equal(t, tc.want, string(got))
	}
}

func TestVerifyEd25519(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	otherPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusSuspended, PublicKey: encoded(pub)}
	src := mapSource{agent.ID: agent}
	msg := []byte("message")
	sig := ed25519.Sign(priv, msg)

	keys := KeySet(context.Background(), src, agent.ID)
	require.True(t, keys.Known())

	v, err := keys.VerifyEd25519(*encoded(pub), msg, sig)
	require.NoError(t, err)
	require.Same(t, agent, LoadVerifiedAgent(v))

	// No presented key: verified against the registered key alone.
	_, err = keys.VerifyEd25519("", msg, sig)
	require.NoError(t, err)

	for name, presented := range map[string]string{
		"a different key": *encoded(otherPub),
		"not base64":      "!!",
		"a truncated key": *encoded(pub[:16]),
	} {
		_, err := keys.VerifyEd25519(presented, msg, sig)
		require.ErrorIs(t, err, ErrKeyNotRecognized, name)
	}

	bad := append([]byte(nil), sig...)
	bad[0] ^= 1
	_, err = keys.VerifyEd25519("", msg, bad)
	require.ErrorIs(t, err, ErrEd25519SignatureInvalid)

	unknown := KeySet(context.Background(), src, uuid.New())
	require.False(t, unknown.Known())
	_, err = unknown.VerifyEd25519("", msg, sig)
	require.ErrorIs(t, err, ErrKeyNotRecognized)
}

// A registered ML-DSA key of another level is not this algorithm's key: refused, but not
// logged as corrupt, because the request chose the level.
func TestVerifyMLDSA_AKeyOfAnotherLevelIsRefusedWithoutALogLine(t *testing.T) {
	kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusVerified, PQCPublicKey: encoded(kp.PublicKey)}
	keys := KeySet(context.Background(), mapSource{agent.ID: agent}, agent.ID)
	msg := []byte("message")
	sig, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, kp.PrivateKey, msg)
	require.NoError(t, err)

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	_, err = keys.VerifyMLDSA(pqc.AlgorithmMLDSA44, "", msg, sig)
	require.ErrorIs(t, err, ErrKeyNotRecognized)
	require.Empty(t, buf.String())

	_, err = keys.VerifyMLDSA(pqc.AlgorithmMLDSA65, "", msg, sig)
	require.NoError(t, err)
}

func TestRequiresHybridIsReadFromTheVerifiedAgent(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusVerified, PublicKey: encoded(pub), HybridModeEnabled: true}
	msg := []byte("message")
	v, err := KeySet(context.Background(), mapSource{agent.ID: agent}, agent.ID).VerifyEd25519("", msg, ed25519.Sign(priv, msg))
	require.NoError(t, err)
	require.True(t, RequiresHybrid(v))

	// The zero value carries no agent: it is not a verification.
	require.False(t, RequiresHybrid(VerifiedAgent{}))
	require.Nil(t, LoadVerifiedAgent(VerifiedAgent{}))
}

// Every unrecognised key is the one ErrKeyNotRecognized, with the one text, and carries
// which cause it was for the server's refusal counters. A signature that does not verify
// carries no cause.
func TestKeyNotRecognizedCarriesItsCause(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	otherPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	msg := []byte("message")
	sig := ed25519.Sign(priv, msg)

	registered := &domain.Agent{ID: uuid.New(), PublicKey: encoded(pub), PQCPublicKey: encoded(kp.PublicKey)}
	noKey := &domain.Agent{ID: uuid.New()}
	corrupt := &domain.Agent{ID: uuid.New(), PublicKey: encoded(pub[:16]), PQCPublicKey: encoded([]byte("not an ML-DSA key"))}
	src := mapSource{registered.ID: registered, noKey.ID: noKey, corrupt.ID: corrupt}
	keys := func(id uuid.UUID) Keys { return KeySet(context.Background(), src, id) }

	prev := log.Writer()
	log.SetOutput(&bytes.Buffer{})
	defer log.SetOutput(prev)

	for _, tc := range []struct {
		name string
		err  error
		want KeyCause
	}{
		{"unknown agent, Ed25519", second(keys(uuid.New()).VerifyEd25519("", msg, sig)), KeyCauseAgentUnknown},
		{"unknown agent, ML-DSA", second(keys(uuid.New()).VerifyMLDSA(pqc.AlgorithmMLDSA65, "", msg, sig)), KeyCauseAgentUnknown},
		{"no registered Ed25519 key", second(keys(noKey.ID).VerifyEd25519("", msg, sig)), KeyCauseNoRegisteredKey},
		{"no registered ML-DSA key", second(keys(noKey.ID).VerifyMLDSA(pqc.AlgorithmMLDSA65, "", msg, sig)), KeyCauseNoRegisteredKey},
		{"an ML-DSA key of another level", second(keys(registered.ID).VerifyMLDSA(pqc.AlgorithmMLDSA44, "", msg, sig)), KeyCauseNoRegisteredKey},
		{"corrupt Ed25519 key", second(keys(corrupt.ID).VerifyEd25519("", msg, sig)), KeyCauseRegisteredKeyMalformed},
		{"corrupt ML-DSA key", second(keys(corrupt.ID).VerifyMLDSA(pqc.AlgorithmMLDSA65, "", msg, sig)), KeyCauseRegisteredKeyMalformed},
		{"a different Ed25519 key presented", second(keys(registered.ID).VerifyEd25519(*encoded(otherPub), msg, sig)), KeyCausePresentedKeyMismatch},
		{"a hybrid request presenting a different Ed25519 key", second(keys(registered.ID).VerifyHybrid(
			pqc.AlgorithmHybridEd25519MLDSA65, *encoded(otherPub), "", msg, sig, sig)), KeyCausePresentedKeyMismatch},
	} {
		require.ErrorIs(t, tc.err, ErrKeyNotRecognized, tc.name)
		require.Equal(t, KeyNotRecognizedMessage, tc.err.Error(), "%s: the text names no cause", tc.name)
		cause, ok := KeyNotRecognizedCause(tc.err)
		require.True(t, ok, tc.name)
		require.Equal(t, tc.want, cause, tc.name)
	}

	bad := append([]byte(nil), sig...)
	bad[0] ^= 1
	_, err = keys(registered.ID).VerifyEd25519("", msg, bad)
	require.ErrorIs(t, err, ErrEd25519SignatureInvalid)
	_, ok := KeyNotRecognizedCause(err)
	require.False(t, ok, "a signature that does not verify is not an unrecognised key")
}

func second(_ VerifiedAgent, err error) error { return err }
