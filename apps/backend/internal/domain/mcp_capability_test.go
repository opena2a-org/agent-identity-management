package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCapabilityType_Constants(t *testing.T) {
	tests := []struct {
		constant MCPCapabilityType
		expected string
	}{
		{MCPCapabilityTypeTool, "tool"},
		{MCPCapabilityTypeResource, "resource"},
		{MCPCapabilityTypePrompt, "prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, MCPCapabilityType(tt.expected), tt.constant)
		})
	}
}

func TestMCPServerCapability_Structure(t *testing.T) {
	now := time.Now()
	lastVerifiedAt := now.Add(-time.Hour)

	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "get_weather",
		CapabilityType: MCPCapabilityTypeTool,
		Description:    "Get current weather for a location",
		CapabilitySchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {"type": "string"},
				"units": {"type": "string", "enum": ["celsius", "fahrenheit"]}
			},
			"required": ["location"]
		}`),
		DetectedAt:     now.Add(-24 * time.Hour),
		LastVerifiedAt: &lastVerifiedAt,
		IsActive:       true,
		CreatedAt:      now.Add(-24 * time.Hour),
		UpdatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, capability.ID)
	assert.Equal(t, "get_weather", capability.Name)
	assert.Equal(t, MCPCapabilityTypeTool, capability.CapabilityType)
	assert.NotEmpty(t, capability.Description)
	assert.NotNil(t, capability.CapabilitySchema)
	assert.True(t, capability.IsActive)
}

func TestMCPServerCapability_Tool(t *testing.T) {
	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "search_code",
		CapabilityType: MCPCapabilityTypeTool,
		Description:    "Search code in repositories",
		CapabilitySchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string"},
				"repo": {"type": "string"}
			}
		}`),
		DetectedAt: time.Now(),
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	assert.Equal(t, MCPCapabilityTypeTool, capability.CapabilityType)
	assert.Contains(t, capability.Name, "search")
}

func TestMCPServerCapability_Resource(t *testing.T) {
	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "file_content",
		CapabilityType: MCPCapabilityTypeResource,
		Description:    "Access file contents from repository",
		CapabilitySchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string"}
			},
			"required": ["path"]
		}`),
		DetectedAt: time.Now(),
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	assert.Equal(t, MCPCapabilityTypeResource, capability.CapabilityType)
}

func TestMCPServerCapability_Prompt(t *testing.T) {
	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "code_review",
		CapabilityType: MCPCapabilityTypePrompt,
		Description:    "Generate a code review prompt",
		CapabilitySchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"code": {"type": "string"},
				"language": {"type": "string"}
			}
		}`),
		DetectedAt: time.Now(),
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	assert.Equal(t, MCPCapabilityTypePrompt, capability.CapabilityType)
}

func TestMCPServerCapability_Inactive(t *testing.T) {
	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "deprecated_tool",
		CapabilityType: MCPCapabilityTypeTool,
		Description:    "This tool is no longer available",
		DetectedAt:     time.Now().Add(-30 * 24 * time.Hour),
		IsActive:       false, // Marked as inactive
		CreatedAt:      time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	assert.False(t, capability.IsActive)
}

func TestMCPServerCapability_NilSchema(t *testing.T) {
	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "simple_tool",
		CapabilityType: MCPCapabilityTypeTool,
		Description:    "A tool with no schema defined",
		DetectedAt:     time.Now(),
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Nil(t, capability.CapabilitySchema)
	assert.Nil(t, capability.LastVerifiedAt)
}

func TestMCPCapabilitySummary_Structure(t *testing.T) {
	summary := MCPCapabilitySummary{
		ServerID:      uuid.New(),
		TotalCount:    15,
		ToolCount:     8,
		ResourceCount: 5,
		PromptCount:   2,
	}

	assert.NotEqual(t, uuid.Nil, summary.ServerID)
	assert.Equal(t, 15, summary.TotalCount)
	assert.Equal(t, 8, summary.ToolCount)
	assert.Equal(t, 5, summary.ResourceCount)
	assert.Equal(t, 2, summary.PromptCount)

	// Verify counts add up
	assert.Equal(t, summary.TotalCount, summary.ToolCount+summary.ResourceCount+summary.PromptCount)
}

func TestMCPCapabilitySummary_Empty(t *testing.T) {
	summary := MCPCapabilitySummary{
		ServerID:      uuid.New(),
		TotalCount:    0,
		ToolCount:     0,
		ResourceCount: 0,
		PromptCount:   0,
	}

	assert.Equal(t, 0, summary.TotalCount)
	assert.Equal(t, 0, summary.ToolCount)
	assert.Equal(t, 0, summary.ResourceCount)
	assert.Equal(t, 0, summary.PromptCount)
}

func TestMCPCapabilitySummary_ToolsOnly(t *testing.T) {
	summary := MCPCapabilitySummary{
		ServerID:      uuid.New(),
		TotalCount:    10,
		ToolCount:     10,
		ResourceCount: 0,
		PromptCount:   0,
	}

	assert.Equal(t, summary.TotalCount, summary.ToolCount)
	assert.Equal(t, 0, summary.ResourceCount)
	assert.Equal(t, 0, summary.PromptCount)
}

func TestMCPServerCapability_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	capability := MCPServerCapability{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		Name:           "test_tool",
		CapabilityType: MCPCapabilityTypeTool,
		Description:    "Test tool for serialization",
		CapabilitySchema: json.RawMessage(`{"type":"object"}`),
		DetectedAt:     now,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(capability)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPServerCapability
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, capability.ID, restored.ID)
	assert.Equal(t, capability.Name, restored.Name)
	assert.Equal(t, capability.CapabilityType, restored.CapabilityType)
	assert.Equal(t, capability.IsActive, restored.IsActive)
}

func TestMCPServerCapability_SchemaValidation(t *testing.T) {
	validSchemas := []string{
		`{"type":"object"}`,
		`{"type":"object","properties":{}}`,
		`{"type":"object","properties":{"name":{"type":"string"}}}`,
		`{"type":"array","items":{"type":"string"}}`,
	}

	for i, schema := range validSchemas {
		capability := MCPServerCapability{
			ID:               uuid.New(),
			MCPServerID:      uuid.New(),
			Name:             "test_tool",
			CapabilityType:   MCPCapabilityTypeTool,
			CapabilitySchema: json.RawMessage(schema),
			DetectedAt:       time.Now(),
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		// Verify schema is valid JSON
		var parsed interface{}
		err := json.Unmarshal(capability.CapabilitySchema, &parsed)
		assert.NoError(t, err, "Schema %d should be valid JSON", i)
	}
}

func TestMCPCapabilitySummary_JSONSerialization(t *testing.T) {
	summary := MCPCapabilitySummary{
		ServerID:      uuid.New(),
		TotalCount:    20,
		ToolCount:     12,
		ResourceCount: 5,
		PromptCount:   3,
	}

	// Serialize to JSON
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPCapabilitySummary
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, summary.ServerID, restored.ServerID)
	assert.Equal(t, summary.TotalCount, restored.TotalCount)
	assert.Equal(t, summary.ToolCount, restored.ToolCount)
	assert.Equal(t, summary.ResourceCount, restored.ResourceCount)
	assert.Equal(t, summary.PromptCount, restored.PromptCount)
}

func TestMCPServerCapability_AllTypes(t *testing.T) {
	types := []MCPCapabilityType{
		MCPCapabilityTypeTool,
		MCPCapabilityTypeResource,
		MCPCapabilityTypePrompt,
	}

	for _, capType := range types {
		t.Run(string(capType), func(t *testing.T) {
			capability := MCPServerCapability{
				ID:             uuid.New(),
				MCPServerID:    uuid.New(),
				Name:           "test_" + string(capType),
				CapabilityType: capType,
				Description:    "Test capability of type " + string(capType),
				DetectedAt:     time.Now(),
				IsActive:       true,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			assert.Equal(t, capType, capability.CapabilityType)
			assert.NotEmpty(t, string(capType))
		})
	}
}
