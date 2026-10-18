package application

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// registryATCRevocationReason is the reason AIM gives the Registry when it asks
// it to revoke an agent's credentials. It is the reason code every entry of
// AIM's own revocation list (GET /api/v1/revocations) carries, so the agent is
// listed with one reason on both lists. It must stay a fixed code: the Registry
// publishes the reason on its unauthenticated revocation list.
const registryATCRevocationReason = "revoked"

// registryATCRevocationAttemptTimeout bounds one revocation request.
const registryATCRevocationAttemptTimeout = 30 * time.Second

// defaultRegistryATCRevocationRetryDelays are the waits before the second and
// later revocation requests, made while the Registry's answer is one a later
// request can change (registryATCRevocationRetryable).
var defaultRegistryATCRevocationRetryDelays = []time.Duration{
	5 * time.Second,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// RegistryATCRevoker revokes at the Registry the Agent Trust Credentials the
// Registry issued for an agent. registry.ATCClient implements it.
type RegistryATCRevoker interface {
	RevokeATC(ctx context.Context, agentID, reason string) error
}

// SetRegistryATCRevoker wires the Registry client RevokeAgent asks to revoke the
// credentials the Registry issued for the agent. When unset, RevokeAgent revokes
// the agent in AIM only.
func (s *AgentService) SetRegistryATCRevoker(revoker RegistryATCRevoker) {
	s.atcRevoker = revoker
}

// SetRegistryATCRevocations wires the server's store of revocation requests.
// When set, RevokeAgent stores its request there and the store's sender sends
// it, so a server stop before the Registry has answered does not end it; the
// revoker set by SetRegistryATCRevoker is then not used.
func (s *AgentService) SetRegistryATCRevocations(q *RegistryATCRevocations) {
	s.atcRevocations = q
}

// requestRegistryATCRevocation asks the Registry to revoke the credentials it
// issued for the agent, and returns without waiting for the Registry's answer:
// the revocation AIM stored stands whatever the Registry answers, and a
// Registry that is slow or unreachable does not hold the request that revoked
// the agent. The request goes through the store SetRegistryATCRevocations
// wired, or is sent from memory when none is wired.
func (s *AgentService) requestRegistryATCRevocation(ctx context.Context, agentID uuid.UUID) {
	if s.atcRevocations != nil {
		s.atcRevocations.Request(ctx, agentID)
		return
	}
	startRegistryATCRevocation(s.atcRevoker, agentID, s.atcRevocationRetryDelays)
}

// startRegistryATCRevocation asks revoker, on its own goroutine, to revoke the
// credentials the Registry issued for agentID, as revokeRegistryATCs does with
// delays (defaultRegistryATCRevocationRetryDelays when nil), and returns at
// once. A nil revoker is sent nothing. The goroutine uses a background context
// because it outlives the request that started it. Nothing of the request is
// stored: a server stop ends it.
func startRegistryATCRevocation(revoker RegistryATCRevoker, agentID uuid.UUID, delays []time.Duration) {
	if revoker == nil {
		return
	}
	if delays == nil {
		delays = defaultRegistryATCRevocationRetryDelays
	}
	go func() {
		_ = revokeRegistryATCs(revoker, agentID.String(), delays)
	}()
}

// revokeRegistryATCs sends the revocation request for agentID, and sends it
// again after each of delays in turn while the answer is one a later request
// can change. It logs each failed request and returns the last one's error, or
// nil once the Registry has revoked the credentials.
func revokeRegistryATCs(revoker RegistryATCRevoker, agentID string, delays []time.Duration) error {
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), registryATCRevocationAttemptTimeout)
		err := revoker.RevokeATC(ctx, agentID, registryATCRevocationReason)
		cancel()
		delay, again := registryATCRevocationNext(agentID, attempt, err, delays)
		if !again {
			return err
		}
		time.Sleep(delay)
	}
}

// registryATCRevocationNext logs the answer err to revocation request number
// attempt for agentID and reports whether another request follows, and after
// which of delays. None follows once the Registry has revoked the credentials,
// after an answer a later request repeats, or after the request that follows
// the last delay. A request that follows a failed one is logged when it
// succeeds.
func registryATCRevocationNext(agentID string, attempt int, err error, delays []time.Duration) (time.Duration, bool) {
	switch {
	case err == nil:
		if attempt > 1 {
			log.Printf("agent %s: the Registry revoked the agent's credentials (attempt %d)", agentID, attempt)
		}
		return 0, false
	case errors.Is(err, registry.ErrTokenNotConfigured):
		log.Printf("agent %s: no Registry credential revocation sent: %v", agentID, err)
		return 0, false
	case !registryATCRevocationRetryable(err) || attempt > len(delays):
		log.Printf("agent %s: the Registry did not revoke the agent's credentials (attempt %d, not retried): %v", agentID, attempt, err)
		return 0, false
	}
	delay := delays[attempt-1]
	log.Printf("agent %s: the Registry did not revoke the agent's credentials (attempt %d, retrying in %s): %v", agentID, attempt, delay, err)
	return delay, true
}

// registryATCRevocationRetryable reports whether a later revocation request can
// succeed where err failed: after a failed connection or a timeout, or an
// answer ATCStatusError.Retryable accepts. A missing token and any other refusal
// repeat.
func registryATCRevocationRetryable(err error) bool {
	if errors.Is(err, registry.ErrTokenNotConfigured) {
		return false
	}
	var statusErr *registry.ATCStatusError
	if errors.As(err, &statusErr) {
		return statusErr.Retryable()
	}
	return true
}
