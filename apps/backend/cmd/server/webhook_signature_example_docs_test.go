package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Python example docs/API.md gives in § Verifying a delivery is the code
// in cmd/server/testdata/webhook_signature_example.py, which
// TestWebhookReplayGuardRoundTrip (integration tag) runs against deliveries it
// captured, so the documented example is one that verifies a real delivery.
func TestAPIDocsWebhookVerificationExampleIsTheFixture(t *testing.T) {
	doc := aim03ReadRepoFile(t, "docs/API.md")
	fixture := aim03ReadRepoFile(t, "apps/backend/cmd/server/testdata/webhook_signature_example.py")

	_, section, found := strings.Cut(doc, "\n### Verifying a delivery\n")
	require.True(t, found, "docs/API.md has a Verifying a delivery section")
	section, _, _ = strings.Cut(section, "\n---\n")
	_, example, found := strings.Cut(section, "\n```python\n")
	require.True(t, found, "the Verifying a delivery section has a Python example")
	example, _, found = strings.Cut(example, "\n```\n")
	require.True(t, found, "the Python example's code block is closed")

	_, tested, found := strings.Cut(fixture, "# --- docs/API.md example begins ---\n")
	require.True(t, found, "the fixture marks where the example begins")
	tested, _, found = strings.Cut(tested, "\n# --- docs/API.md example ends ---")
	require.True(t, found, "the fixture marks where the example ends")

	assert.Equal(t, tested, example,
		"the Python example in docs/API.md § Verifying a delivery must be the code between the markers of cmd/server/testdata/webhook_signature_example.py")
}
