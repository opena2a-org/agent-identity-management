package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An agent authenticates a request by signing it, and the server accepts several
// signing schemes. The authenticators named beside each scheme below count the
// requests they authenticate in
//
//   - aim_agent_requests_authenticated_total{type, sdk} on /metrics
//
// where type is the scheme the request was signed in and sdk the calling SDK, derived
// from the User-Agent header as for aim_s1_refusals_total. Both take values from closed
// sets. A request is counted where its authenticator accepts it, after the agent's
// status check. A request the authenticator refuses, or hands to another
// authenticator, is not counted.
//
// The series of each scheme for each sdk label are created at start, at 0, so a
// scrape lists a scheme that has authenticated no request.

// AgentRequestScheme is the scheme an authenticated request was signed in. The set is
// closed: one constant per scheme an authenticator below verifies, and an authenticator
// added later takes one of these or adds one here in the same change.
type AgentRequestScheme string

const (
	// The X-Agent-ID, X-Timestamp and X-Signature headers, or X-Signature-Ed25519 and
	// X-Signature-MLDSA, signed over the method, the path and query, the timestamp and
	// the body when there is one. Counted by PQCAgentMiddleware, whichever algorithm
	// verified the request, and by Ed25519AgentMiddleware.
	AgentRequestSchemeV1 AgentRequestScheme = "agent-request-v1"
	// A signed statement in X-Agent-Request. Counted by AgentRequestV2Middleware.
	AgentRequestSchemeV2 = AgentRequestScheme(domain.AgentRequestSchemeID)
	// The X-AIM-Agent-ID, X-AIM-Timestamp and X-AIM-Signature headers of the SDK
	// verification read. Counted by VerificationHandler.GetVerificationSDK.
	AgentRequestSchemeVerificationRead AgentRequestScheme = "verification-read"
	// A verification request body without signedBytes, whose signature member signs its
	// agentId, capability, resource, context and timestamp members. Counted by
	// VerificationHandler.CreateVerification.
	AgentRequestSchemeVerificationBody AgentRequestScheme = "verification-body"
	// A verification request body with signedBytes, a signed action-request statement.
	// Counted by VerificationHandler.CreateVerification.
	AgentRequestSchemeActionRequest = AgentRequestScheme(domain.ActionRequestSchemeID)
	// Never passed by an authenticator. It is what RecordAgentRequestAuthenticated
	// records for a value outside this set, so a caller mistake cannot turn request
	// data into a label.
	AgentRequestSchemeUnclassified AgentRequestScheme = "unclassified"
)

// agentRequestSchemes are the schemes an authenticator passes, in the order their
// series are created.
var agentRequestSchemes = []AgentRequestScheme{
	AgentRequestSchemeV1,
	AgentRequestSchemeV2,
	AgentRequestSchemeVerificationRead,
	AgentRequestSchemeVerificationBody,
	AgentRequestSchemeActionRequest,
}

var agentRequestsAuthenticatedTotal = withAgentRequestSeries(factory.NewCounterVec(
	prometheus.CounterOpts{
		Name: "aim_agent_requests_authenticated_total",
		Help: "Requests authenticated by an agent's signature, by the scheme the request was signed in (type) and calling SDK",
	},
	[]string{"type", "sdk"},
))

// withAgentRequestSeries creates in vec, at 0, the series of every scheme an
// authenticator passes for every sdk label, and returns vec.
func withAgentRequestSeries(vec *prometheus.CounterVec) *prometheus.CounterVec {
	for _, scheme := range agentRequestSchemes {
		for _, known := range s1SDKUserAgentPrefixes {
			vec.WithLabelValues(string(scheme), known.sdk)
		}
		vec.WithLabelValues(string(scheme), s1SDKOther)
	}
	return vec
}

// RecordAgentRequestAuthenticated counts one request an authenticator authenticated.
//
// scheme is the scheme the request was signed in. userAgent is the request's User-Agent
// header; only the sdk label derived from it is kept. A scheme outside the closed set is
// recorded as AgentRequestSchemeUnclassified.
func RecordAgentRequestAuthenticated(scheme AgentRequestScheme, userAgent string) {
	known := false
	for _, s := range agentRequestSchemes {
		if s == scheme {
			known = true
			break
		}
	}
	if !known {
		scheme = AgentRequestSchemeUnclassified
	}
	agentRequestsAuthenticatedTotal.WithLabelValues(string(scheme), s1SDKLabel(userAgent)).Inc()
}
