package domain

import "testing"

// The four sources are written out as string literals, so the test states the stored
// vocabulary independently of the constants it checks.
func TestVerificationEventSourceIsKnown(t *testing.T) {
	for _, source := range []string{"service", "system", "caller_reported", "agent_reported"} {
		if !VerificationEventSource(source).IsKnown() {
			t.Errorf("%q is a source a writer may record", source)
		}
	}
	for _, source := range []string{"", "Service", "caller-reported", "agent", "unknown"} {
		if VerificationEventSource(source).IsKnown() {
			t.Errorf("%q must not be accepted as a source", source)
		}
	}
}
