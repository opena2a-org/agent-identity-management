package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestBuildAgentDID asserts the canonical did:aip:aim_<uuid> shape and that the
// emitted DID is byte-identical to prefix + the agent UUID, so the agentDid
// carried into an issued credential matches what the did:aip resolver answers.
func TestBuildAgentDID(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	got := BuildAgentDID(id)

	assert.Equal(t, "did:aip:aim_11111111-2222-3333-4444-555555555555", got)
	assert.Equal(t, AgentDIDPrefix+id.String(), got)
	assert.True(t, len(got) > len(AgentDIDPrefix), "DID must carry the UUID after the prefix")
}

// TestBuildAgentDID_DistinctPerAgent guards against a constant/shared-DID bug:
// two different agent UUIDs must map to two different DIDs.
func TestBuildAgentDID_DistinctPerAgent(t *testing.T) {
	a := BuildAgentDID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	b := BuildAgentDID(uuid.MustParse("22222222-2222-2222-2222-222222222222"))

	assert.NotEqual(t, a, b)
}
