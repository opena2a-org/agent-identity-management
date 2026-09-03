package application

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCheckAttributes_DenyList(t *testing.T) {
	engine := &FGAEngine{}
	policy := &FGAPolicy{
		DeniedAttributes: []string{"ssn", "credit_card"},
	}

	// Request accessing denied attribute
	req := &FGARequest{
		AgentID:    uuid.New(),
		Capability: "read:users",
		Attributes: []string{"name", "ssn"},
	}
	denied, reason := engine.checkAttributes(req, policy)
	assert.True(t, denied)
	assert.Contains(t, reason, "ssn")

	// Request accessing allowed attribute
	req.Attributes = []string{"name", "email"}
	denied, _ = engine.checkAttributes(req, policy)
	assert.False(t, denied)
}

func TestCheckAttributes_AllowList(t *testing.T) {
	engine := &FGAEngine{}
	policy := &FGAPolicy{
		AllowedAttributes: []string{"name", "email", "role"},
	}

	// Accessing allowed attributes
	req := &FGARequest{
		Attributes: []string{"name", "email"},
	}
	denied, _ := engine.checkAttributes(req, policy)
	assert.False(t, denied)

	// Accessing non-allowed attribute
	req.Attributes = []string{"name", "salary"}
	denied, reason := engine.checkAttributes(req, policy)
	assert.True(t, denied)
	assert.Contains(t, reason, "salary")
}

func TestCheckAttributes_AllowedActions(t *testing.T) {
	engine := &FGAEngine{}
	policy := &FGAPolicy{
		AllowedActions: []string{"read", "list"},
	}

	req := &FGARequest{Action: "read"}
	denied, _ := engine.checkAttributes(req, policy)
	assert.False(t, denied)

	req.Action = "delete"
	denied, reason := engine.checkAttributes(req, policy)
	assert.True(t, denied)
	assert.Contains(t, reason, "delete")
}

func TestCheckAttributes_WildcardPattern(t *testing.T) {
	engine := &FGAEngine{}
	policy := &FGAPolicy{
		DeniedAttributes: []string{"pii.*"},
	}

	req := &FGARequest{
		Attributes: []string{"pii.ssn"},
	}
	denied, _ := engine.checkAttributes(req, policy)
	assert.True(t, denied)

	req.Attributes = []string{"metadata.created"}
	denied, _ = engine.checkAttributes(req, policy)
	assert.False(t, denied)
}

func TestCheckContext_DriftScore(t *testing.T) {
	engine := &FGAEngine{logger: nil}
	engine.logger = nil // will use slog default

	maxDrift := 0.5
	contextRules := map[string]interface{}{
		"maxDriftScore": maxDrift,
	}
	rulesJSON, _ := json.Marshal(contextRules)

	policy := &FGAPolicy{
		ContextRules: rulesJSON,
	}

	// Can't test with real ASC fetch, but test that empty context rules pass
	emptyPolicy := &FGAPolicy{ContextRules: json.RawMessage(`{}`)}
	req := &FGARequest{AgentID: uuid.New()}
	denied, _, _ := engine.checkContext(nil, req, emptyPolicy)
	assert.False(t, denied)

	// Non-empty rules with no ASC data = fail closed by default (AIM-08:
	// pre-AIM-08 this path failed open; contextRules.onUnavailable now
	// decides, with deny as the default).
	denied, _, _ = engine.checkContext(nil, req, policy)
	assert.True(t, denied)
}

func TestCheckChain_EmptyRules(t *testing.T) {
	engine := &FGAEngine{}

	policy := &FGAPolicy{ChainRules: json.RawMessage(`{}`)}
	req := &FGARequest{AgentID: uuid.New()}
	denied, _ := engine.checkChain(nil, req, policy)
	assert.False(t, denied)
}

func TestMatchPattern(t *testing.T) {
	assert.True(t, matchPattern("anything", "*"))
	assert.True(t, matchPattern("pii.ssn", "pii.*"))
	assert.True(t, matchPattern("admin:users", "admin:*"))
	assert.False(t, matchPattern("user:read", "admin:*"))
	assert.True(t, matchPattern("exact", "exact"))
	assert.False(t, matchPattern("different", "exact"))
}

func TestPqArray_Parse(t *testing.T) {
	var arr []string
	scanner := pq_array(&arr)

	// Empty
	err := scanner.Scan("{}")
	assert.NoError(t, err)
	assert.Len(t, arr, 0)

	// Multiple values
	err = scanner.Scan(`{"hello","world"}`)
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, arr)

	// Nil
	err = scanner.Scan(nil)
	assert.NoError(t, err)
	assert.Len(t, arr, 0)
}
