package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ErrPublicKeyNotAccepted is returned wherever an Ed25519 public key is accepted
// for storing and is not a base64-encoded 32-byte key that ed25519key.Acceptable
// accepts. Nothing is stored; a route answers it with its validation error.
var ErrPublicKeyNotAccepted = errors.New("publicKey must be a base64-encoded 32-byte Ed25519 public key")

// This file holds the criterion for verifying an agent at registration, and
// nothing else decides it: the proof of possession of the agent's key, the
// conditions an agent has to meet, and the status they give.
//
// In an organization in monitoring mode, an agent that holds its own key is
// verified at registration on one kind of evidence: a signature by its private
// key over a server-issued, single-use, time-bounded challenge, checked against
// the public key of the same request before the status is decided. It is never
// verified on the bare public key, on the SDK token or session, on a
// User-Agent, on the SDK's own check that a key matches, or on a signature over
// bytes the client chose. In strict mode an agent stays pending.

// registrationProofContext starts the bytes an agent signs. It names the
// protocol and its version, so a signature made for another purpose is not a
// proof of possession.
const registrationProofContext = "aim-agent-registration-v1\n"

// autoVerifyMinimumTrustScore is the trust score below which an agent is not
// verified at registration.
const autoVerifyMinimumTrustScore = 0.3

// The reasons a registration's answer gives for an agent that stays pending.
// None of them holds a secret or a challenge.
const (
	PendingReasonStrictMode       = "strict mode: the agent is verified by an administrator"
	PendingReasonNoProof          = "no possession proof"
	PendingReasonProofExpired     = "possession proof expired"
	PendingReasonProofUsed        = "possession proof already used"
	PendingReasonProofNotAccepted = "possession proof not accepted"
	PendingReasonProofNotChecked  = "possession proof could not be checked"
	PendingReasonModeUnknown      = "enforcement mode could not be read"
	PendingReasonMissingKeys      = "missing cryptographic keys"
	PendingReasonTrustScoreTooLow = "trust score below the minimum for automatic verification"
	PendingReasonMissingMetadata  = "missing required metadata"
)

// PossessionProof is the optional possessionProof member of the registration
// body: the id of a challenge issued by POST /api/v1/agents/registration-challenges
// and the agent's signature over RegistrationProofMessage, in standard padded
// base64.
type PossessionProof struct {
	ChallengeID string `json:"challengeId"`
	Signature   string `json:"signature"`
}

// UnmarshalJSON never fails: a member that is not an object with these two
// strings is a proof that is not accepted, and the registration goes on with
// the agent pending, as it does for any proof that is not good.
func (p *PossessionProof) UnmarshalJSON(data []byte) error {
	var raw struct {
		ChallengeID string `json:"challengeId"`
		Signature   string `json:"signature"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		*p = PossessionProof{}
		return nil
	}
	*p = PossessionProof(raw)
	return nil
}

// RegistrationProofMessage returns the bytes an agent signs to prove that it
// holds the key it registers:
//
//	"aim-agent-registration-v1\n" + organizationId + "\n" + challengeId + "\n" + challenge + "\n" + publicKey
//
// organizationId and challengeId are the lowercase hyphenated UUID text the API
// carries. challenge is the 32 challenge bytes in standard padded base64, as
// the challenge route answers them, and publicKey is the publicKey text of the
// registration body, as it is sent: standard padded base64 of the 32-byte
// Ed25519 key.
func RegistrationProofMessage(organizationID, challengeID uuid.UUID, challenge []byte, publicKey string) []byte {
	return []byte(registrationProofContext +
		organizationID.String() + "\n" +
		challengeID.String() + "\n" +
		base64.StdEncoding.EncodeToString(challenge) + "\n" +
		publicKey)
}

// possessionProofOutcome is what the check of a registration's proof found.
type possessionProofOutcome struct {
	// presented is whether the registration carried a possessionProof.
	presented bool
	// provenKey is the public key text the signature was verified against, set
	// only when the proof is good.
	provenKey string
	// reason says why the proof is not good; empty when it is.
	reason string
}

// provesKeyOf reports whether the proof is good for the public key the agent is
// stored with.
func (o possessionProofOutcome) provesKeyOf(agent *domain.Agent) bool {
	return o.provenKey != "" && agent.PublicKey != nil && *agent.PublicKey == o.provenKey
}

// CheckPossessionProof checks a registration's proof against the public key of
// the same registration. The challenge is used up first, whatever the check
// finds; the proof is good only when the challenge exists, was not used before,
// has not expired, was issued to this organization and this user, and the
// signature over RegistrationProofMessage verifies under publicKey. It reports
// the outcome and never an error: a proof that is not good leaves the agent
// pending.
func (s *RegistrationChallengeService) CheckPossessionProof(ctx context.Context, orgID, principalID uuid.UUID, publicKey string, proof *PossessionProof) possessionProofOutcome {
	if proof == nil {
		return possessionProofOutcome{reason: PendingReasonNoProof}
	}
	refused := func(reason string) possessionProofOutcome {
		return possessionProofOutcome{presented: true, reason: reason}
	}

	challengeID, err := uuid.Parse(proof.ChallengeID)
	if err != nil {
		return refused(PendingReasonProofNotAccepted)
	}

	now := s.now()
	challenge, firstUse, err := s.repo.Consume(ctx, challengeID, now)
	switch {
	case errors.Is(err, domain.ErrRegistrationChallengeNotFound):
		return refused(PendingReasonProofNotAccepted)
	case err != nil:
		fmt.Printf("Warning: registration challenge %s could not be used: %v\n", challengeID, err)
		return refused(PendingReasonProofNotChecked)
	}
	if !firstUse {
		return refused(PendingReasonProofUsed)
	}
	if !now.Before(challenge.ExpiresAt) {
		return refused(PendingReasonProofExpired)
	}
	if challenge.OrganizationID != orgID || challenge.PrincipalID != principalID {
		return refused(PendingReasonProofNotAccepted)
	}

	key, err := ed25519key.Decode(publicKey, true)
	if err != nil {
		return refused(PendingReasonProofNotAccepted)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(proof.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return refused(PendingReasonProofNotAccepted)
	}
	message := RegistrationProofMessage(orgID, challenge.ID, challenge.Challenge, publicKey)
	if !ed25519.Verify(key, message, signature) {
		return refused(PendingReasonProofNotAccepted)
	}
	return possessionProofOutcome{presented: true, provenKey: publicKey}
}

// checkRegistrationProof checks the proof of a registration. A registration
// without a proof has nothing to check.
func (s *AgentService) checkRegistrationProof(ctx context.Context, orgID, userID uuid.UUID, req *CreateAgentRequest) possessionProofOutcome {
	if req.PossessionProof == nil {
		return possessionProofOutcome{reason: PendingReasonNoProof}
	}
	if s.registrationChallenges == nil {
		return possessionProofOutcome{presented: true, reason: PendingReasonProofNotAccepted}
	}
	return s.registrationChallenges.CheckPossessionProof(ctx, orgID, userID, req.PublicKey, req.PossessionProof)
}

// registrationVerification is the status a registration gives the agent: either
// verified, or pending with the reason.
type registrationVerification struct {
	Verified      bool
	PendingReason string
}

// decideRegistrationVerification decides whether an agent is verified at
// registration. A strict-mode organization keeps every agent pending. A proof
// that was presented and is not good keeps the agent pending. An agent that
// holds its own key is verified only by a good proof, in an organization read as
// being in monitoring mode, and then meets the conditions of every agent
// (autoVerificationCriteria).
func (s *AgentService) decideRegistrationVerification(orgID uuid.UUID, agent *domain.Agent, proof possessionProofOutcome) registrationVerification {
	if s.inStrictMode(orgID) {
		return registrationVerification{PendingReason: PendingReasonStrictMode}
	}
	if proof.presented && proof.reason != "" {
		return registrationVerification{PendingReason: proof.reason}
	}
	keyHeld := proof.provesKeyOf(agent)
	if keyHeld && !s.inMonitoringMode(orgID) {
		return registrationVerification{PendingReason: PendingReasonModeUnknown}
	}
	if agent.EncryptedPrivateKey == nil && !keyHeld {
		reason := proof.reason
		if reason == "" {
			reason = PendingReasonNoProof
		}
		return registrationVerification{PendingReason: reason}
	}
	if ok, reason := autoVerificationCriteria(agent, keyHeld); !ok {
		return registrationVerification{PendingReason: reason}
	}
	return registrationVerification{Verified: true}
}

// inMonitoringMode reports whether the organization is read as being in
// monitoring mode. Unlike inStrictMode, an organization that cannot be read is
// not in monitoring mode.
func (s *AgentService) inMonitoringMode(orgID uuid.UUID) bool {
	if s.orgRepo == nil {
		return false
	}
	org, err := s.orgRepo.GetByID(orgID)
	return err == nil && org != nil && org.EnforcementMode == domain.EnforcementModeMonitoring
}

// shouldAutoVerifyAgent determines if an agent meets criteria for automatic
// verification, for an agent whose key is not proven by a possession proof:
//  1. Has valid cryptographic keys (public + encrypted private key)
//  2. Trust score >= 0.3 (30% minimum threshold)
//  3. Has required metadata (name, description, type)
func (s *AgentService) shouldAutoVerifyAgent(agent *domain.Agent) bool {
	ok, _ := autoVerificationCriteria(agent, false)
	return ok
}

// autoVerificationCriteria holds the conditions an agent meets to be verified
// at registration, and the reason for the first it does not meet. keyHeld is
// true only when the agent's key was proven by a good possession proof of the
// same registration; the agent's public key is then taken in place of a
// private key held by the server (condition 1).
func autoVerificationCriteria(agent *domain.Agent, keyHeld bool) (bool, string) {
	// ✅ Check 1: Must have cryptographic keys
	if agent.PublicKey == nil || (agent.EncryptedPrivateKey == nil && !keyHeld) {
		fmt.Printf("Agent %s cannot be auto-verified: missing cryptographic keys\n", agent.Name)
		return false, PendingReasonMissingKeys
	}

	// ✅ Check 2: Trust score must be >= 0.3 (30%)
	if agent.TrustScore < autoVerifyMinimumTrustScore {
		fmt.Printf("Agent %s cannot be auto-verified: trust score too low (%.2f < 0.3)\n", agent.Name, agent.TrustScore)
		return false, PendingReasonTrustScoreTooLow
	}

	// ✅ Check 3: Must have required metadata
	if agent.Name == "" || agent.DisplayName == "" || agent.Description == "" {
		fmt.Printf("Agent %s cannot be auto-verified: missing required metadata\n", agent.Name)
		return false, PendingReasonMissingMetadata
	}

	// ✅ All checks passed - agent qualifies for auto-verification
	return true, ""
}

// SetRegistrationChallenges wires the service that issues and uses up the
// challenges of possession proofs. When unset, a registration's proof is not
// accepted and the agent stays pending.
func (s *AgentService) SetRegistrationChallenges(challenges *RegistrationChallengeService) {
	s.registrationChallenges = challenges
}
