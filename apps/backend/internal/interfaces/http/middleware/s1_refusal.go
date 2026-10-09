package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/agentauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// s1Refusal is why a signature check refused a request: the counted reason and the
// message the response body carries. The verify helpers return it instead of an
// error so that a refusal cannot reach the caller without a reason attached.
type s1Refusal struct {
	reason  metrics.S1RefusalReason
	message string
}

func s1Refused(reason metrics.S1RefusalReason, format string, args ...interface{}) *s1Refusal {
	return &s1Refusal{reason: reason, message: fmt.Sprintf(format, args...)}
}

// refuseS1 is the one place the signed-request middlewares (Ed25519AgentMiddleware,
// PQCAgentMiddleware) refuse a request. It counts the refusal by reason and calling
// SDK, then writes the response.
//
// Every refusal branch in ed25519_agent_auth.go and pqc_agent_auth.go returns through
// here, and s1_refusal_census_test.go fails if one does not.
func refuseS1(c fiber.Ctx, reason metrics.S1RefusalReason, status int, message string) error {
	metrics.RecordS1Refusal(reason, c.Get("User-Agent"))
	return c.Status(status).JSON(s1RefusalBody(reason, message))
}

// s1RefusalBody is the response body for a refusal counted under reason.
//
// The four reasons agentauth reports as one unrecognised key get agentauth's one body
// whatever message the branch carried: the counter keeps the cause for operators, and
// the caller, who holds at most an agent id, is told nothing about that agent. A
// signature that does not verify and a status refusal carry agentauth's reasonCode
// beside their message. Every other refusal is {"error": message}.
func s1RefusalBody(reason metrics.S1RefusalReason, message string) map[string]string {
	switch reason {
	case metrics.S1ReasonAgentLookupFailed, metrics.S1ReasonNoRegisteredKey,
		metrics.S1ReasonPublicKeyMismatch, metrics.S1ReasonRegisteredKeyMalformed:
		return agentauth.KeyNotRecognizedBody()
	case metrics.S1ReasonSignatureInvalidEd25519, metrics.S1ReasonSignatureInvalidMLDSA:
		return agentauth.SignatureInvalidBody(message)
	case metrics.S1ReasonAgentStatusDenied:
		return map[string]string{"error": message, "reasonCode": agentauth.StatusDeniedReason}
	default:
		return map[string]string{"error": message}
	}
}
