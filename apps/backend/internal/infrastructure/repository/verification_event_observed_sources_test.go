package repository

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Trust scoring counts only outcomes the server observed. Adding an asserted
// source such as agent_reported or caller_reported to the allowlist fails here
// without a database.
func TestObservedVerificationEventSources_AreExactlyServiceAndSystem(t *testing.T) {
	got := append([]string(nil), observedVerificationEventSources...)
	sort.Strings(got)
	assert.Equal(t, []string{
		string(domain.VerificationEventSourceService),
		string(domain.VerificationEventSourceSystem),
	}, got)

	for _, asserted := range []domain.VerificationEventSource{
		domain.VerificationEventSourceAgentReported,
		domain.VerificationEventSourceCallerReported,
	} {
		assert.NotContains(t, observedVerificationEventSources, string(asserted))
	}
}
