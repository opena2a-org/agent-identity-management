package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONBMap_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected JSONBMap
		wantErr  bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "valid JSON object",
			input:    []byte(`{"key":"value","num":123}`),
			expected: JSONBMap{"key": "value", "num": float64(123)},
			wantErr:  false,
		},
		{
			name:     "nested JSON object",
			input:    []byte(`{"outer":{"inner":"value"}}`),
			expected: JSONBMap{"outer": map[string]interface{}{"inner": "value"}},
			wantErr:  false,
		},
		{
			name:     "invalid type (string)",
			input:    "not bytes",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid JSON",
			input:    []byte(`{invalid json}`),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONBMap
			err := j.Scan(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, j)
			}
		})
	}
}

func TestJSONBMap_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONBMap
		expected string
		wantNil  bool
	}{
		{
			name:    "nil map",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "empty map",
			input:    JSONBMap{},
			expected: "{}",
		},
		{
			name:     "simple map",
			input:    JSONBMap{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "complex map",
			input:    JSONBMap{"str": "hello", "num": 42, "bool": true},
			expected: "", // We'll verify it's valid JSON instead
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.input.Value()
			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, val)
				return
			}

			bytes, ok := val.([]byte)
			require.True(t, ok)

			if tt.expected != "" {
				assert.JSONEq(t, tt.expected, string(bytes))
			} else {
				// Just verify it's valid JSON
				var parsed map[string]interface{}
				err := json.Unmarshal(bytes, &parsed)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCapabilityRequestStatus_Constants(t *testing.T) {
	// Verify status constants have expected values
	assert.Equal(t, CapabilityRequestStatus("pending"), CapabilityRequestStatusPending)
	assert.Equal(t, CapabilityRequestStatus("approved"), CapabilityRequestStatusApproved)
	assert.Equal(t, CapabilityRequestStatus("auto-approved"), CapabilityRequestStatusAutoApproved)
	assert.Equal(t, CapabilityRequestStatus("rejected"), CapabilityRequestStatusRejected)
}

func TestCapabilityRequest_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	requestedBy := uuid.New()
	reviewedBy := uuid.New()
	reviewedAt := now.Add(time.Hour)

	request := CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        agentID,
		CapabilityType: "mcp:tool:execute",
		Reason:         "Need to execute tools for automation",
		Metadata:       JSONBMap{"priority": "high"},
		Status:         CapabilityRequestStatusPending,
		RequestedBy:    requestedBy,
		ReviewedBy:     &reviewedBy,
		RequestedAt:    now,
		ReviewedAt:     &reviewedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, request.ID)
	assert.Equal(t, agentID, request.AgentID)
	assert.Equal(t, "mcp:tool:execute", request.CapabilityType)
	assert.Equal(t, "Need to execute tools for automation", request.Reason)
	assert.Equal(t, CapabilityRequestStatusPending, request.Status)
	assert.Equal(t, requestedBy, request.RequestedBy)
	assert.Equal(t, &reviewedBy, request.ReviewedBy)
	assert.Equal(t, "high", request.Metadata["priority"])
}

func TestCapabilityRequest_PendingToApproved(t *testing.T) {
	now := time.Now()
	reviewedBy := uuid.New()
	reviewedAt := now.Add(time.Hour)

	request := CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		CapabilityType: "mcp:resource:read",
		Reason:         "Need read access to resources",
		Status:         CapabilityRequestStatusPending,
		RequestedBy:    uuid.New(),
		RequestedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Initially pending
	assert.Equal(t, CapabilityRequestStatusPending, request.Status)
	assert.Nil(t, request.ReviewedBy)
	assert.Nil(t, request.ReviewedAt)

	// Simulate approval
	request.Status = CapabilityRequestStatusApproved
	request.ReviewedBy = &reviewedBy
	request.ReviewedAt = &reviewedAt
	request.UpdatedAt = time.Now()

	assert.Equal(t, CapabilityRequestStatusApproved, request.Status)
	assert.Equal(t, &reviewedBy, request.ReviewedBy)
	assert.NotNil(t, request.ReviewedAt)
}

func TestCapabilityRequest_PendingToRejected(t *testing.T) {
	now := time.Now()
	reviewedBy := uuid.New()
	reviewedAt := now.Add(30 * time.Minute)

	request := CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		CapabilityType: "mcp:dangerous:operation",
		Reason:         "Want to perform risky operation",
		Status:         CapabilityRequestStatusPending,
		RequestedBy:    uuid.New(),
		RequestedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Simulate rejection
	request.Status = CapabilityRequestStatusRejected
	request.ReviewedBy = &reviewedBy
	request.ReviewedAt = &reviewedAt
	request.UpdatedAt = time.Now()

	assert.Equal(t, CapabilityRequestStatusRejected, request.Status)
	assert.Equal(t, &reviewedBy, request.ReviewedBy)
	assert.NotNil(t, request.ReviewedAt)
}

func TestCapabilityRequest_AutoApproved(t *testing.T) {
	now := time.Now()

	request := CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		CapabilityType: "mcp:tool:basic",
		Reason:         "Basic tool access needed",
		Status:         CapabilityRequestStatusAutoApproved,
		RequestedBy:    uuid.New(),
		RequestedAt:    now,
		ReviewedAt:     &now, // Auto-approved at request time
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	assert.Equal(t, CapabilityRequestStatusAutoApproved, request.Status)
	// Auto-approved requests may not have a ReviewedBy (system approved)
	assert.Nil(t, request.ReviewedBy)
	assert.NotNil(t, request.ReviewedAt)
}

func TestCapabilityRequestWithDetails_Structure(t *testing.T) {
	now := time.Now()
	reviewedByEmail := "admin@example.com"

	request := CapabilityRequestWithDetails{
		CapabilityRequest: CapabilityRequest{
			ID:             uuid.New(),
			AgentID:        uuid.New(),
			CapabilityType: "mcp:tool:execute",
			Reason:         "Need tool execution",
			Status:         CapabilityRequestStatusApproved,
			RequestedBy:    uuid.New(),
			RequestedAt:    now,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		AgentName:        "test-agent",
		AgentDisplayName: "Test Agent",
		RequestedByEmail: "user@example.com",
		ReviewedByEmail:  &reviewedByEmail,
	}

	assert.Equal(t, "test-agent", request.AgentName)
	assert.Equal(t, "Test Agent", request.AgentDisplayName)
	assert.Equal(t, "user@example.com", request.RequestedByEmail)
	assert.Equal(t, "admin@example.com", *request.ReviewedByEmail)
}

func TestCreateCapabilityRequestInput_Validation(t *testing.T) {
	input := CreateCapabilityRequestInput{
		AgentID:        uuid.New(),
		CapabilityType: "mcp:resource:write",
		Reason:         "Need write access for data persistence",
		Metadata:       JSONBMap{"context": "user-request"},
		RequestedBy:    uuid.New(),
	}

	assert.NotEqual(t, uuid.Nil, input.AgentID)
	assert.NotEmpty(t, input.CapabilityType)
	assert.True(t, len(input.Reason) >= 10, "reason should be at least 10 characters")
	assert.NotNil(t, input.Metadata)
}

func TestCapabilityRequestFilter_Defaults(t *testing.T) {
	filter := CapabilityRequestFilter{}

	assert.Nil(t, filter.Status)
	assert.Nil(t, filter.AgentID)
	assert.Nil(t, filter.OrganizationID)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestCapabilityRequestFilter_WithValues(t *testing.T) {
	status := CapabilityRequestStatusPending
	agentID := uuid.New()
	orgID := uuid.New()

	filter := CapabilityRequestFilter{
		Status:         &status,
		AgentID:        &agentID,
		OrganizationID: &orgID,
		Limit:          20,
		Offset:         40,
	}

	assert.Equal(t, CapabilityRequestStatusPending, *filter.Status)
	assert.Equal(t, agentID, *filter.AgentID)
	assert.Equal(t, orgID, *filter.OrganizationID)
	assert.Equal(t, 20, filter.Limit)
	assert.Equal(t, 40, filter.Offset)
}

func TestJSONBMap_RoundTrip(t *testing.T) {
	// Test that data survives a round trip through Value() and Scan()
	original := JSONBMap{
		"string":  "hello",
		"number":  float64(42),
		"boolean": true,
		"nested":  map[string]interface{}{"inner": "value"},
		"array":   []interface{}{"a", "b", "c"},
	}

	// Convert to driver value
	val, err := original.Value()
	require.NoError(t, err)

	// Scan back
	var restored JSONBMap
	err = restored.Scan(val)
	require.NoError(t, err)

	// Verify values match
	assert.Equal(t, original["string"], restored["string"])
	assert.Equal(t, original["number"], restored["number"])
	assert.Equal(t, original["boolean"], restored["boolean"])
}

func TestCapabilityRequest_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	request := CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		CapabilityType: "mcp:tool:test",
		Reason:         "Testing JSON serialization",
		Metadata:       JSONBMap{"test": true},
		Status:         CapabilityRequestStatusPending,
		RequestedBy:    uuid.New(),
		RequestedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(request)
	require.NoError(t, err)

	// Deserialize back
	var restored CapabilityRequest
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, request.ID, restored.ID)
	assert.Equal(t, request.AgentID, restored.AgentID)
	assert.Equal(t, request.CapabilityType, restored.CapabilityType)
	assert.Equal(t, request.Status, restored.Status)
}
