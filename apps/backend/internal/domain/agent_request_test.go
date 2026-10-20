package domain

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentRequestConstants(t *testing.T) {
	assert.Equal(t, "agent-request-v2", AgentRequestSchemeID)
	assert.Equal(t, "application/vnd.opena2a.agent-request.v2+json", AgentRequestPayloadType)
	assert.Equal(t, 45, len(AgentRequestPayloadType))
	assert.Equal(t, "X-Agent-Request", AgentRequestHeader)
	assert.Equal(t, "X-Agent-Request-Signature", AgentRequestSignatureHeader)

	// The signed bytes of the two statement formats part at the payload type, which
	// follows "DSSEv1 " and its length in both.
	assert.NotEqual(t, ActionRequestPayloadType, AgentRequestPayloadType)
	assert.NotEqual(t, ActionRequestSchemeID, AgentRequestSchemeID)
}

// The format's public name and its payload type each have one declaration in
// the backend's Go source; every message interpolates them.
func TestAgentRequestNamesAreDeclaredOnce(t *testing.T) {
	names := 0
	types := 0
	for path, src := range backendGoSources(t) {
		if n := strings.Count(src, "agent-request-v2"); n > 0 {
			names += n
			assert.Equal(t, filepath.Join("internal", "domain", "agent_request.go"), path)
		}
		if n := strings.Count(src, "application/vnd.opena2a.agent-request.v2+json"); n > 0 {
			types += n
			assert.Equal(t, filepath.Join("internal", "domain", "agent_request.go"), path)
		}
	}
	assert.Equal(t, 1, names)
	assert.Equal(t, 1, types)
}
