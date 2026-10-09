// Package agentauth is the one shape every agent-signature authenticator follows: read
// only the agent's registered keys, verify the request against them, and only then read
// the agent's status, organization or any other attribute.
//
// The order is the point. An authenticator that read the status first told a caller who
// held nothing but an agent id whether that agent existed and whether it was suspended or
// revoked, because each of those answered with its own refusal. Here the agent row is
// reachable only through Keys, whose methods read key material, and through VerifiedAgent,
// which only a successful verification constructs. An unknown agent, an agent with no
// registered key, a registered key that does not decode to its algorithm's key length and
// a presented key that is not the registered one are one refusal, ErrKeyNotRecognized.
// Which of them it was is attached as a KeyCause for the server's own refusal counters,
// and never reaches the caller.
package agentauth

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const (
	// KeyNotRecognizedMessage is the `error` of the one refusal for the four causes above.
	// It names no cause, so the refusal is the same whichever one it was.
	KeyNotRecognizedMessage = "the signing key is not the registered key of a known agent"
	// KeyNotRecognizedReason is that refusal's `reasonCode`.
	KeyNotRecognizedReason = "agentKeyNotRecognized"
	// SignatureInvalidReason is the `reasonCode` of a signature that does not verify under
	// the registered key.
	SignatureInvalidReason = "signatureInvalid"
	// StatusDeniedReason is the `reasonCode` of a verified agent whose status does not
	// permit authentication.
	StatusDeniedReason = "agentStatusDenied"
	// AssertionNotRecognizedDescription is the token endpoint's `error_description` for the
	// same four causes and for an assertion whose signature does not verify.
	AssertionNotRecognizedDescription = "the assertion is not signed by the registered key of a known agent"
)

var (
	// ErrKeyNotRecognized: the agent is unknown, has no registered key for the algorithm,
	// its registered key does not decode to that algorithm's key length, or the request
	// presented a different key.
	ErrKeyNotRecognized = errors.New(KeyNotRecognizedMessage)
	// ErrEd25519SignatureInvalid: the Ed25519 signature does not verify under the
	// registered key.
	ErrEd25519SignatureInvalid = errors.New("invalid Ed25519 signature")
	// ErrMLDSASignatureInvalid: the ML-DSA signature does not verify under the registered
	// key.
	ErrMLDSASignatureInvalid = errors.New("invalid ML-DSA signature")
)

// KeyCause is which of the causes behind ErrKeyNotRecognized refused a key. The caller
// gets the one refusal whatever the cause; the cause exists for the server's refusal
// counters, which operators read, so a corrupt stored key stays distinguishable from an
// unknown agent or a client presenting the wrong key.
type KeyCause int

const (
	// KeyCauseAgentUnknown: no agent row could be read for the id.
	KeyCauseAgentUnknown KeyCause = iota + 1
	// KeyCauseNoRegisteredKey: the agent has no registered key for the algorithm the
	// request chose, including an ML-DSA key of another level.
	KeyCauseNoRegisteredKey
	// KeyCauseRegisteredKeyMalformed: the registered key does not decode to a key of its
	// algorithm family.
	KeyCauseRegisteredKeyMalformed
	// KeyCausePresentedKeyMismatch: the request presented a key that is not the
	// registered one.
	KeyCausePresentedKeyMismatch
)

// keyNotRecognizedError is ErrKeyNotRecognized with its cause attached. Its text is
// ErrKeyNotRecognized's, so the cause cannot reach a caller through err.Error().
type keyNotRecognizedError struct{ cause KeyCause }

func (e keyNotRecognizedError) Error() string        { return KeyNotRecognizedMessage }
func (e keyNotRecognizedError) Is(target error) bool { return target == ErrKeyNotRecognized }

func keyNotRecognized(cause KeyCause) error {
	return keyNotRecognizedError{cause: cause}
}

// KeyNotRecognizedCause is the cause of an ErrKeyNotRecognized returned by a Keys method.
// ok is false for any other error.
func KeyNotRecognizedCause(err error) (cause KeyCause, ok bool) {
	var e keyNotRecognizedError
	if errors.As(err, &e) {
		return e.cause, true
	}
	return 0, false
}

// KeyNotRecognizedBody is the 401 body for ErrKeyNotRecognized. Every authenticator sends
// it from this one declaration, so the bodies stay byte-equal.
func KeyNotRecognizedBody() map[string]string {
	return map[string]string{"error": KeyNotRecognizedMessage, "reasonCode": KeyNotRecognizedReason}
}

// SignatureInvalidBody is the 401 body for a signature that does not verify, with the
// authenticator's own message.
func SignatureInvalidBody(message string) map[string]string {
	return map[string]string{"error": message, "reasonCode": SignatureInvalidReason}
}

// StatusDeniedBody is the 401 body for a verified agent whose status does not permit
// authentication. It names the status: only a caller holding the agent's key reaches it.
func StatusDeniedBody(status domain.AgentStatus) map[string]string {
	return map[string]string{"error": domain.AgentStatusDeniedMessage(status), "reasonCode": StatusDeniedReason}
}

// Source reads one agent row by id. *application.AgentService satisfies it.
type Source interface {
	GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
}

type rowReader interface {
	GetByID(id uuid.UUID) (*domain.Agent, error)
}

type repositorySource struct{ r rowReader }

func (s repositorySource) GetAgent(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	return s.r.GetByID(id)
}

// FromRepository is the Source for an authenticator that holds a repository rather than
// the agent service.
func FromRepository(r rowReader) Source {
	return repositorySource{r: r}
}

// Keys is an agent's registered key material: the only part of the agent row an
// authenticator reads before a signature has verified. An unknown agent yields a Keys
// that holds no key, so it is refused exactly as an agent with no registered key is.
type Keys struct {
	agentID uuid.UUID
	agent   *domain.Agent
}

// KeySet reads agentID's registered keys. A failed read is an unknown agent.
func KeySet(ctx context.Context, src Source, agentID uuid.UUID) Keys {
	agent, err := src.GetAgent(ctx, agentID)
	if err != nil || agent == nil {
		return Keys{agentID: agentID}
	}
	return Keys{agentID: agentID, agent: agent}
}

// Known reports whether the agent exists. Only legacy verification creation reads it, to
// keep its unknown-agent 404 until that route converges on the signed-request scheme;
// every other authenticator answers an unknown agent with ErrKeyNotRecognized.
func (k Keys) Known() bool {
	return k.agent != nil
}

// VerifyEd25519 verifies signature over message under the registered Ed25519 key.
// presented is the public key the request carried, base64 as sent, or "" for a scheme
// that does not carry one; it is compared with the registered key on decoded bytes.
func (k Keys) VerifyEd25519(presented string, message, signature []byte) (VerifiedAgent, error) {
	key, err := k.ed25519Key()
	if err != nil {
		return VerifiedAgent{}, err
	}
	if !presentedMatches(presented, key) {
		return VerifiedAgent{}, keyNotRecognized(KeyCausePresentedKeyMismatch)
	}
	if !ed25519.Verify(key, message, signature) {
		return VerifiedAgent{}, ErrEd25519SignatureInvalid
	}
	return VerifiedAgent{agent: k.agent}, nil
}

// VerifyMLDSA verifies signature over message under the registered ML-DSA key, read at
// alg's key length. presented is as for VerifyEd25519.
func (k Keys) VerifyMLDSA(alg pqc.Algorithm, presented string, message, signature []byte) (VerifiedAgent, error) {
	key, err := k.mldsaKey(alg)
	if err != nil {
		return VerifiedAgent{}, err
	}
	if !presentedMatches(presented, key) {
		return VerifiedAgent{}, keyNotRecognized(KeyCausePresentedKeyMismatch)
	}
	if err := pqc.VerifyMLDSA(alg, key, message, signature); err != nil {
		return VerifiedAgent{}, fmt.Errorf("%w: %v", ErrMLDSASignatureInvalid, err)
	}
	return VerifiedAgent{agent: k.agent}, nil
}

// VerifyHybrid verifies both signatures of a hybrid algorithm; both must verify. Both
// keys are checked before either signature, so a request that names the wrong key for
// either half is refused as unrecognised whichever half it is.
func (k Keys) VerifyHybrid(alg pqc.Algorithm, presentedEd25519, presentedMLDSA string, message, ed25519Sig, mldsaSig []byte) (VerifiedAgent, error) {
	variant := pqc.GetMLDSAVariant(alg)
	edKey, err := k.ed25519Key()
	if err != nil {
		return VerifiedAgent{}, err
	}
	if !presentedMatches(presentedEd25519, edKey) {
		return VerifiedAgent{}, keyNotRecognized(KeyCausePresentedKeyMismatch)
	}
	mlKey, err := k.mldsaKey(variant)
	if err != nil {
		return VerifiedAgent{}, err
	}
	if !presentedMatches(presentedMLDSA, mlKey) {
		return VerifiedAgent{}, keyNotRecognized(KeyCausePresentedKeyMismatch)
	}
	if !ed25519.Verify(edKey, message, ed25519Sig) {
		return VerifiedAgent{}, ErrEd25519SignatureInvalid
	}
	if err := pqc.VerifyMLDSA(variant, mlKey, message, mldsaSig); err != nil {
		return VerifiedAgent{}, fmt.Errorf("%w: %v", ErrMLDSASignatureInvalid, err)
	}
	return VerifiedAgent{agent: k.agent}, nil
}

// ed25519Key is the registered Ed25519 key, decoded, or ErrKeyNotRecognized with its
// cause. A stored value that does not decode to 32 bytes is a corrupt row rather than a
// bad request, so it is logged once, without the key.
func (k Keys) ed25519Key() (ed25519.PublicKey, error) {
	if k.agent == nil {
		return nil, keyNotRecognized(KeyCauseAgentUnknown)
	}
	if k.agent.PublicKey == nil || *k.agent.PublicKey == "" {
		return nil, keyNotRecognized(KeyCauseNoRegisteredKey)
	}
	raw, err := base64.StdEncoding.DecodeString(*k.agent.PublicKey)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		logMalformedKey(k.agentID, "Ed25519")
		return nil, keyNotRecognized(KeyCauseRegisteredKeyMalformed)
	}
	return ed25519.PublicKey(raw), nil
}

// mldsaKey is the registered ML-DSA key, decoded at alg's key length, or
// ErrKeyNotRecognized with its cause. A key of another ML-DSA level is not this
// algorithm's key and is refused without a log line, since the request chose the level;
// a value that is no ML-DSA key at all is a corrupt row.
func (k Keys) mldsaKey(alg pqc.Algorithm) ([]byte, error) {
	if k.agent == nil {
		return nil, keyNotRecognized(KeyCauseAgentUnknown)
	}
	if k.agent.PQCPublicKey == nil || *k.agent.PQCPublicKey == "" {
		return nil, keyNotRecognized(KeyCauseNoRegisteredKey)
	}
	size, err := pqc.GetExpectedPublicKeySize(alg)
	if err != nil || !pqc.IsPQCAlgorithm(alg) {
		return nil, keyNotRecognized(KeyCauseNoRegisteredKey)
	}
	raw, err := base64.StdEncoding.DecodeString(*k.agent.PQCPublicKey)
	if err != nil || !isMLDSAKeyLength(len(raw)) {
		logMalformedKey(k.agentID, "ML-DSA")
		return nil, keyNotRecognized(KeyCauseRegisteredKeyMalformed)
	}
	if len(raw) != size {
		return nil, keyNotRecognized(KeyCauseNoRegisteredKey)
	}
	return raw, nil
}

func isMLDSAKeyLength(n int) bool {
	return n == pqc.MLDSA44PublicKeySize || n == pqc.MLDSA65PublicKeySize || n == pqc.MLDSA87PublicKeySize
}

// presentedMatches reports whether the presented key decodes to registered. A scheme that
// carries no key presents "", which matches: the signature is then verified against the
// registered key alone.
func presentedMatches(presented string, registered []byte) bool {
	if presented == "" {
		return true
	}
	raw, err := base64.StdEncoding.DecodeString(presented)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(raw, registered) == 1
}

// logMalformedKey is the operator's signal that a stored key is corrupt. The caller gets
// ErrKeyNotRecognized like any other unrecognised key, and the refusal counters carry no
// agent id, so this line is the only place that names the agent whose row is corrupt. It
// names the agent and the algorithm, never the key.
func logMalformedKey(agentID uuid.UUID, algorithm string) {
	log.Printf("agent authentication: agent %s has a registered %s public key that does not decode to a valid key; refusing its requests until the key is re-registered", agentID, algorithm)
}

// VerifiedAgent is an agent whose request signature verified under its registered key.
// Only the Verify methods of Keys construct one, so holding one is the proof that the
// signature verified; its fields are unexported for that reason.
type VerifiedAgent struct {
	agent *domain.Agent
}

// LoadVerifiedAgent is the agent row of a verified agent: the only way an authenticator
// reads an agent's status, organization or other attributes.
func LoadVerifiedAgent(v VerifiedAgent) *domain.Agent {
	return v.agent
}

// RequiresHybrid reports whether the verified agent has hybrid mode turned on: the hybrid
// requirement is an attribute, so it is read only after verification, like the status.
func RequiresHybrid(v VerifiedAgent) bool {
	return v.agent != nil && v.agent.HybridModeEnabled
}
