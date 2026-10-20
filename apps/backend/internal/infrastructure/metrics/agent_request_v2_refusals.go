package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AgentRequestV2Middleware refuses a request whose signed statement in X-Agent-Request
// (domain.AgentRequestSchemeID) does not authenticate it. That response goes to the
// caller and nowhere else, so a refusal is counted in
//
//   - aim_agent_request_v2_refusals_total{reason, sdk} on /metrics
//
// beside aim_s1_refusals_total, which counts the refusals of the agent signature checks
// that read X-Agent-ID, X-Timestamp and X-Signature. sdk is derived from the User-Agent
// header as for aim_s1_refusals_total. Both labels take values from closed sets: a
// refusal happens before the caller is authenticated, so nothing the caller sent (agent
// id, statement, signature, the raw User-Agent) becomes a label.

// AgentRequestV2RefusalReason is why AgentRequestV2Middleware refused a request. The set
// is closed: one constant per kind of refusal in the middleware, and a refusal added
// there takes an existing constant or adds one here in the same change.
type AgentRequestV2RefusalReason string

const (
	// The server has no configured public origin, so it accepts no statement.
	AgentRequestV2ReasonAudienceNotConfigured AgentRequestV2RefusalReason = "audience_not_configured"
	// The request also carries a header through which another authenticator names a
	// principal: Authorization, X-API-Key, or a header the other agent signature checks
	// read.
	AgentRequestV2ReasonConflictingAuthentication AgentRequestV2RefusalReason = "conflicting_authentication"
	// X-Agent-Request is not sent once, is not unpadded base64url, or does not carry a
	// payload of the format: one JSON object whose members are each defined, present
	// once and in their form.
	AgentRequestV2ReasonStatementMalformed AgentRequestV2RefusalReason = "statement_malformed"
	// The payload has no nonce.
	AgentRequestV2ReasonNonceMissing AgentRequestV2RefusalReason = "nonce_missing"
	// X-Agent-Request-Signature is absent, carries more than two values, a value that is
	// not an algorithm, a colon and an unpadded base64url signature, or two values that
	// are not one Ed25519 and one ML-DSA signature.
	AgentRequestV2ReasonSignatureHeaderMalformed AgentRequestV2RefusalReason = "signature_header_malformed"
	// The statement's audience is not the server's configured public origin.
	AgentRequestV2ReasonAudienceMismatch AgentRequestV2RefusalReason = "audience_mismatch"
	// The statement's method is not the request's method.
	AgentRequestV2ReasonMethodMismatch AgentRequestV2RefusalReason = "method_mismatch"
	// The statement's target is not the request's path and query as received.
	AgentRequestV2ReasonTargetMismatch AgentRequestV2RefusalReason = "target_mismatch"
	// body_sha256 does not match the body as received: absent beside a body, present
	// beside none, or the SHA-256 of other bytes.
	AgentRequestV2ReasonBodyMismatch AgentRequestV2RefusalReason = "body_mismatch"
	// Loading the agent failed: it does not exist, or the lookup itself failed.
	AgentRequestV2ReasonAgentLookupFailed AgentRequestV2RefusalReason = "agent_lookup_failed"
	// The agent has no registered key for an algorithm a signature names, including an
	// ML-DSA key of another level.
	AgentRequestV2ReasonNoRegisteredKey AgentRequestV2RefusalReason = "no_registered_key"
	// The key registered for the agent does not decode to a key of its algorithm family.
	AgentRequestV2ReasonRegisteredKeyMalformed AgentRequestV2RefusalReason = "registered_key_malformed"
	// The Ed25519 signature does not verify over the statement.
	AgentRequestV2ReasonSignatureInvalidEd25519 AgentRequestV2RefusalReason = "signature_invalid_ed25519"
	// The ML-DSA signature does not verify over the statement.
	AgentRequestV2ReasonSignatureInvalidMLDSA AgentRequestV2RefusalReason = "signature_invalid_mldsa"
	// The agent is in hybrid mode and the request carries one signature.
	AgentRequestV2ReasonHybridRequired AgentRequestV2RefusalReason = "hybrid_required"
	// The agent is not in hybrid mode and the request carries two signatures.
	AgentRequestV2ReasonSingleSignatureRequired AgentRequestV2RefusalReason = "single_signature_required"
	// The timestamp is behind the database clock by more than the admission window.
	AgentRequestV2ReasonSkewPast AgentRequestV2RefusalReason = "skew_past"
	// The timestamp is ahead of the database clock by more than the admission window.
	AgentRequestV2ReasonSkewFuture AgentRequestV2RefusalReason = "skew_future"
	// The agent has already used the nonce.
	AgentRequestV2ReasonNonceReused AgentRequestV2RefusalReason = "nonce_reused"
	// The timestamp and nonce could not be checked against the database: no admission
	// store, an admission error, or an admission result the middleware does not know.
	AgentRequestV2ReasonFreshnessCheckUnavailable AgentRequestV2RefusalReason = "freshness_check_unavailable"
	// The agent's status is neither verified nor pending: suspended, revoked or a value
	// the server does not recognise.
	AgentRequestV2ReasonAgentStatusDenied AgentRequestV2RefusalReason = "agent_status_denied"
	// Never passed by the middleware. It is what RecordAgentRequestV2Refusal records for a
	// value outside this set, so a caller mistake cannot turn request data into a label.
	AgentRequestV2ReasonUnclassified AgentRequestV2RefusalReason = "unclassified"
)

var agentRequestV2RefusalReasons = map[AgentRequestV2RefusalReason]bool{
	AgentRequestV2ReasonAudienceNotConfigured:     true,
	AgentRequestV2ReasonConflictingAuthentication: true,
	AgentRequestV2ReasonStatementMalformed:        true,
	AgentRequestV2ReasonNonceMissing:              true,
	AgentRequestV2ReasonSignatureHeaderMalformed:  true,
	AgentRequestV2ReasonAudienceMismatch:          true,
	AgentRequestV2ReasonMethodMismatch:            true,
	AgentRequestV2ReasonTargetMismatch:            true,
	AgentRequestV2ReasonBodyMismatch:              true,
	AgentRequestV2ReasonAgentLookupFailed:         true,
	AgentRequestV2ReasonNoRegisteredKey:           true,
	AgentRequestV2ReasonRegisteredKeyMalformed:    true,
	AgentRequestV2ReasonSignatureInvalidEd25519:   true,
	AgentRequestV2ReasonSignatureInvalidMLDSA:     true,
	AgentRequestV2ReasonHybridRequired:            true,
	AgentRequestV2ReasonSingleSignatureRequired:   true,
	AgentRequestV2ReasonSkewPast:                  true,
	AgentRequestV2ReasonSkewFuture:                true,
	AgentRequestV2ReasonNonceReused:               true,
	AgentRequestV2ReasonFreshnessCheckUnavailable: true,
	AgentRequestV2ReasonAgentStatusDenied:         true,
	AgentRequestV2ReasonUnclassified:              true,
}

var agentRequestV2RefusalsTotal = factory.NewCounterVec(
	prometheus.CounterOpts{
		Name: "aim_agent_request_v2_refusals_total",
		Help: "Requests carrying a signed " + domain.AgentRequestSchemeID +
			" statement that the server refused, by reason and calling SDK",
	},
	[]string{"reason", "sdk"},
)

// RecordAgentRequestV2Refusal counts one request AgentRequestV2Middleware refused.
//
// reason is the refusal's kind. userAgent is the request's User-Agent header; only the
// sdk label derived from it is kept. A reason outside the closed set is recorded as
// AgentRequestV2ReasonUnclassified.
func RecordAgentRequestV2Refusal(reason AgentRequestV2RefusalReason, userAgent string) {
	if !agentRequestV2RefusalReasons[reason] {
		reason = AgentRequestV2ReasonUnclassified
	}
	agentRequestV2RefusalsTotal.WithLabelValues(string(reason), s1SDKLabel(userAgent)).Inc()
}
