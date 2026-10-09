package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// VerifyA2ARequest reports attestationValid from this check, so an expired
// attestation must read as not current.
func TestCardAttestationCurrent(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }

	for _, tc := range []struct {
		name string
		card *domain.A2AAgentCard
		want bool
	}{
		{"valid and not yet expired", &domain.A2AAgentCard{IsValid: true, AttestationExpiresAt: at(time.Hour)}, true},
		{"expired an hour ago", &domain.A2AAgentCard{IsValid: true, AttestationExpiresAt: at(-time.Hour)}, false},
		{"expires at this instant", &domain.A2AAgentCard{IsValid: true, AttestationExpiresAt: at(0)}, false},
		{"no expiry", &domain.A2AAgentCard{IsValid: true}, false},
		{"card marked invalid", &domain.A2AAgentCard{IsValid: false, AttestationExpiresAt: at(time.Hour)}, false},
		{"no card", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cardAttestationCurrent(tc.card, now))
		})
	}
}
