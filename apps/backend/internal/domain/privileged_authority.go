package domain

import (
	"crypto/ed25519"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Capability tiers, as stored in privileged_capability_registry.tier.
const (
	CapabilityTierStandard        = "STANDARD"
	CapabilityTierPrivileged      = "PRIVILEGED"
	CapabilityTierSuperPrivileged = "SUPER_PRIVILEGED"
)

// IsPrivilegedClass reports whether a capability tier is in the privileged
// class: the actions only a verified owner signature can authorize. STANDARD
// and PRIVILEGED are outside it. Every other value, including an empty or
// misspelled tier, is inside it, so a bad tier never drops the requirement.
func IsPrivilegedClass(tier string) bool {
	return tier != CapabilityTierStandard && tier != CapabilityTierPrivileged
}

// AuthorityKind names what a caller presents as the reason an action is
// authorized.
type AuthorityKind string

const (
	// AuthorityMessage is any text: an instruction, a relayed message, tool
	// output, or a claim that the owner approved. It never authorizes a
	// privileged action, whatever it says.
	AuthorityMessage AuthorityKind = "message"
	// AuthorityOwnerSignature is a signature by one of the owner's registered
	// keys over a PrivilegedActionStatement for this exact action.
	AuthorityOwnerSignature AuthorityKind = "owner_signature"
)

// OwnerKeyKind names how an owner key was established.
type OwnerKeyKind string

const (
	// OwnerKeyATXCertified is an Ed25519 key certified to the owner by an ATX
	// credential. It signs the statement's SigningBytes directly.
	OwnerKeyATXCertified OwnerKeyKind = "atx_certified"
	// OwnerKeyPasskey is a WebAuthn passkey. A passkey signs an assertion over
	// authenticator data and client data, not the statement itself, and
	// assertion verification is not implemented here, so a passkey signature
	// is refused rather than checked as a raw signature.
	OwnerKeyPasskey OwnerKeyKind = "passkey"
)

// MaxOwnerSignatureLifetime bounds ExpiresAt - IssuedAt on a signed statement,
// matching the approval timeout PAMService.RequestApproval applies.
const MaxOwnerSignatureLifetime = 5 * time.Minute

// ownerSignatureClockSkew is how far IssuedAt may sit ahead of the verifier's
// clock before the statement is refused as issued in the future.
const ownerSignatureClockSkew = 30 * time.Second

// privilegedActionStatementV1 prefixes every signed statement so a signature
// made for another purpose cannot be replayed as an action authorization.
const privilegedActionStatementV1 = "aim-privileged-action-v1"

// PrivilegedAction is the action an agent asks to perform. Tier is the
// capability's tier from the privileged capability registry.
type PrivilegedAction struct {
	AgentID    uuid.UUID
	Capability string
	Tier       string
	Resource   string
	Action     string
}

// OwnerKey is a public key the caller has already established as belonging to
// the agent's owner. Establishing it (passkey registration, ATX certification)
// is the caller's job; this policy only checks signatures against it.
type OwnerKey struct {
	ID        string
	Kind      OwnerKeyKind
	PublicKey ed25519.PublicKey
}

// PrivilegedActionStatement is what the owner signs. It names one action by
// one agent and is valid for a bounded window.
type PrivilegedActionStatement struct {
	AgentID    uuid.UUID
	Capability string
	Resource   string
	Action     string
	Nonce      string
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

// OwnerSignature is a signature over Statement.SigningBytes by the owner key
// named KeyID.
type OwnerSignature struct {
	KeyID     string
	Statement PrivilegedActionStatement
	Signature []byte
}

// ActionAuthority is the evidence presented for an action. Message is kept for
// the audit record only.
type ActionAuthority struct {
	Kind           AuthorityKind
	Message        string
	OwnerSignature *OwnerSignature
}

// PrivilegedDecision is the outcome of the owner-signature gate. Allowed means
// this gate does not stop the action; the capability's other checks still run.
type PrivilegedDecision struct {
	Allowed    bool
	Privileged bool
	OwnerKeyID string
	Reason     string
}

// SigningBytes returns the canonical encoding the owner signs: the version
// line, then one "name:value" line per field, times as Unix seconds. A field
// containing a control character is rejected so no value can add a line.
func (s PrivilegedActionStatement) SigningBytes() ([]byte, error) {
	fields := []struct{ name, value string }{
		{"agentId", s.AgentID.String()},
		{"capability", s.Capability},
		{"resource", s.Resource},
		{"action", s.Action},
		{"nonce", s.Nonce},
		{"issuedAt", strconv.FormatInt(s.IssuedAt.Unix(), 10)},
		{"expiresAt", strconv.FormatInt(s.ExpiresAt.Unix(), 10)},
	}
	var b strings.Builder
	b.WriteString(privilegedActionStatementV1)
	b.WriteByte('\n')
	for _, f := range fields {
		if strings.ContainsFunc(f.value, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return nil, fmt.Errorf("statement field %s contains a control character", f.name)
		}
		b.WriteString(f.name)
		b.WriteByte(':')
		b.WriteString(f.value)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// AuthorizePrivilegedAction decides whether authority is enough for action.
// Outside the privileged class it always allows. Inside it, only an owner
// signature that verifies against one of ownerKeys, over a statement naming
// this exact action and valid at now, allows; a message never does.
//
// The statement's nonce must be non-empty, but recording it so it cannot be
// replayed inside its window needs storage and is the caller's job.
func AuthorizePrivilegedAction(action PrivilegedAction, authority ActionAuthority, ownerKeys []OwnerKey, now time.Time) PrivilegedDecision {
	if !IsPrivilegedClass(action.Tier) {
		return PrivilegedDecision{Allowed: true}
	}
	refuse := func(format string, args ...any) PrivilegedDecision {
		return PrivilegedDecision{Privileged: true, Reason: fmt.Sprintf(format, args...)}
	}

	if authority.Kind != AuthorityOwnerSignature {
		return refuse("capability %s is in the privileged class and requires a verified owner signature; %s authority does not authorize it",
			action.Capability, describeAuthorityKind(authority.Kind))
	}
	sig := authority.OwnerSignature
	if sig == nil || len(sig.Signature) == 0 {
		return refuse("owner signature authority carries no signature")
	}

	st := sig.Statement
	if st.AgentID != action.AgentID || st.Capability != action.Capability ||
		st.Resource != action.Resource || st.Action != action.Action {
		return refuse("owner signature is for a different action than the one requested")
	}
	if st.Nonce == "" {
		return refuse("owner signature statement has no nonce")
	}
	if !st.ExpiresAt.After(st.IssuedAt) || st.ExpiresAt.Sub(st.IssuedAt) > MaxOwnerSignatureLifetime {
		return refuse("owner signature validity window must be positive and at most %s", MaxOwnerSignatureLifetime)
	}
	if st.IssuedAt.After(now.Add(ownerSignatureClockSkew)) {
		return refuse("owner signature is issued in the future")
	}
	if !now.Before(st.ExpiresAt) {
		return refuse("owner signature has expired")
	}

	var key *OwnerKey
	for i := range ownerKeys {
		if ownerKeys[i].ID == sig.KeyID {
			key = &ownerKeys[i]
			break
		}
	}
	if key == nil {
		return refuse("signing key %q is not a registered owner key", sig.KeyID)
	}
	if key.Kind != OwnerKeyATXCertified {
		return refuse("owner key %q of kind %q cannot be verified by this policy", key.ID, key.Kind)
	}
	if len(key.PublicKey) != ed25519.PublicKeySize {
		return refuse("owner key %q is not a valid Ed25519 public key", key.ID)
	}

	msg, err := st.SigningBytes()
	if err != nil {
		return refuse("owner signature statement is malformed: %v", err)
	}
	if !ed25519.Verify(key.PublicKey, msg, sig.Signature) {
		return refuse("owner signature does not verify against owner key %q", key.ID)
	}

	return PrivilegedDecision{Allowed: true, Privileged: true, OwnerKeyID: key.ID}
}

func describeAuthorityKind(kind AuthorityKind) string {
	if kind == "" {
		return "unspecified"
	}
	return string(kind)
}
