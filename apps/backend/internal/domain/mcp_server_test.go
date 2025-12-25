package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServerStatus_Constants(t *testing.T) {
	tests := []struct {
		constant MCPServerStatus
		expected string
	}{
		{MCPServerStatusPending, "pending"},
		{MCPServerStatusVerified, "verified"},
		{MCPServerStatusSuspended, "suspended"},
		{MCPServerStatusRevoked, "revoked"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, MCPServerStatus(tt.expected), tt.constant)
		})
	}
}

func TestMCPServer_Structure(t *testing.T) {
	now := time.Now()
	lastVerifiedAt := now.Add(-time.Hour)
	registeredByAgent := uuid.New()
	sdkTokenID := uuid.New()
	lastAttestedAt := now.Add(-30 * time.Minute)

	server := MCPServer{
		ID:              uuid.New(),
		OrganizationID:  uuid.New(),
		Name:            "github-mcp",
		Description:     "MCP server for GitHub API integration",
		URL:             "https://mcp.github.com/api",
		Version:         "1.0.0",
		PublicKey:       "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
		Status:          MCPServerStatusVerified,
		IsVerified:      true,
		LastVerifiedAt:  &lastVerifiedAt,
		VerificationURL: "https://mcp.github.com/.well-known/mcp-verification",
		Capabilities:    []string{"tools", "prompts", "resources"},
		TrustScore:      0.95,
		VerificationCount: 10,
		RegisteredByAgent: &registeredByAgent,
		CreatedBy:       uuid.New(),
		CreatedByName:   "John Doe",
		CreatedByEmail:  "john@example.com",
		CreatedBySDKTokenID: &sdkTokenID,
		CreatedAt:       now.Add(-30 * 24 * time.Hour),
		UpdatedAt:       now,
		Tags:            []Tag{{ID: uuid.New(), Key: "env", Value: "production"}},
		VerificationMethod: "agent_attestation",
		AttestationCount:   5,
		ConfidenceScore:    85.0,
		LastAttestedAt:     &lastAttestedAt,
		AttestedBy:         []string{"agent-1", "agent-2"},
		ConnectedAgentsCount: 10,
		CapabilitiesCount:    15,
	}

	assert.NotEqual(t, uuid.Nil, server.ID)
	assert.Equal(t, "github-mcp", server.Name)
	assert.Equal(t, MCPServerStatusVerified, server.Status)
	assert.True(t, server.IsVerified)
	assert.Len(t, server.Capabilities, 3)
	assert.Equal(t, 0.95, server.TrustScore)
	assert.Equal(t, 85.0, server.ConfidenceScore)
	assert.Equal(t, 5, server.AttestationCount)
	assert.Len(t, server.Tags, 1)
}

func TestMCPServer_Pending(t *testing.T) {
	server := MCPServer{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "new-mcp",
		Description:    "Newly registered MCP server",
		URL:            "https://new.mcp.com/api",
		Status:         MCPServerStatusPending,
		IsVerified:     false,
		Capabilities:   []string{"tools"},
		TrustScore:     0.0,
		CreatedBy:      uuid.New(),
		CreatedByName:  "Jane Doe",
		CreatedByEmail: "jane@example.com",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, MCPServerStatusPending, server.Status)
	assert.False(t, server.IsVerified)
	assert.Nil(t, server.LastVerifiedAt)
	assert.Equal(t, 0.0, server.TrustScore)
}

func TestMCPServer_Suspended(t *testing.T) {
	server := MCPServer{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "suspicious-mcp",
		URL:            "https://suspicious.mcp.com/api",
		Status:         MCPServerStatusSuspended,
		IsVerified:     false,
		TrustScore:     0.3,
		CreatedBy:      uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, MCPServerStatusSuspended, server.Status)
	assert.False(t, server.IsVerified)
}

func TestMCPServer_Revoked(t *testing.T) {
	server := MCPServer{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "revoked-mcp",
		URL:            "https://revoked.mcp.com/api",
		Status:         MCPServerStatusRevoked,
		IsVerified:     false,
		TrustScore:     0.0,
		CreatedBy:      uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, MCPServerStatusRevoked, server.Status)
	assert.Equal(t, 0.0, server.TrustScore)
}

func TestMCPServer_RegisteredByAgent(t *testing.T) {
	agentID := uuid.New()

	server := MCPServer{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "agent-registered-mcp",
		URL:               "https://agent.mcp.com/api",
		Status:            MCPServerStatusVerified,
		IsVerified:        true,
		RegisteredByAgent: &agentID,
		VerificationMethod: "agent_attestation",
		CreatedBy:         uuid.New(),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	assert.NotNil(t, server.RegisteredByAgent)
	assert.Equal(t, &agentID, server.RegisteredByAgent)
	assert.Equal(t, "agent_attestation", server.VerificationMethod)
}

func TestMCPServer_CreatedByAPIKey(t *testing.T) {
	apiKeyID := uuid.New()

	server := MCPServer{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		Name:             "api-created-mcp",
		URL:              "https://api.mcp.com/api",
		Status:           MCPServerStatusPending,
		CreatedBy:        uuid.New(),
		CreatedByName:    "API User",
		CreatedByEmail:   "api@example.com",
		CreatedByAPIKeyID: &apiKeyID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.NotNil(t, server.CreatedByAPIKeyID)
	assert.Nil(t, server.CreatedBySDKTokenID)
}

func TestMCPServer_WithTags(t *testing.T) {
	server := MCPServer{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "tagged-mcp",
		URL:            "https://tagged.mcp.com/api",
		Status:         MCPServerStatusVerified,
		IsVerified:     true,
		Tags: []Tag{
			{ID: uuid.New(), Key: "env", Value: "production"},
			{ID: uuid.New(), Key: "priority", Value: "critical"},
			{ID: uuid.New(), Key: "provider", Value: "github"},
		},
		CreatedBy: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.Len(t, server.Tags, 3)
	assert.Equal(t, "env", server.Tags[0].Key)
	assert.Equal(t, "production", server.Tags[0].Value)
}

func TestMCPServerVerificationStatus_Structure(t *testing.T) {
	now := time.Now()
	lastVerifiedAt := now.Add(-time.Hour)

	status := MCPServerVerificationStatus{
		ServerID:       uuid.New(),
		IsVerified:     true,
		LastVerifiedAt: &lastVerifiedAt,
		TrustScore:     0.92,
		PublicKeyCount: 2,
		Status:         MCPServerStatusVerified,
	}

	assert.NotEqual(t, uuid.Nil, status.ServerID)
	assert.True(t, status.IsVerified)
	assert.NotNil(t, status.LastVerifiedAt)
	assert.Equal(t, 0.92, status.TrustScore)
	assert.Equal(t, 2, status.PublicKeyCount)
	assert.Equal(t, MCPServerStatusVerified, status.Status)
}

func TestMCPServerVerificationStatus_Unverified(t *testing.T) {
	status := MCPServerVerificationStatus{
		ServerID:       uuid.New(),
		IsVerified:     false,
		LastVerifiedAt: nil,
		TrustScore:     0.0,
		PublicKeyCount: 0,
		Status:         MCPServerStatusPending,
	}

	assert.False(t, status.IsVerified)
	assert.Nil(t, status.LastVerifiedAt)
	assert.Equal(t, 0.0, status.TrustScore)
}

func TestMCPServer_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	server := MCPServer{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		Name:             "test-mcp",
		Description:      "Test MCP server",
		URL:              "https://test.mcp.com/api",
		Version:          "1.0.0",
		Status:           MCPServerStatusVerified,
		IsVerified:       true,
		Capabilities:     []string{"tools", "resources"},
		TrustScore:       0.85,
		CreatedBy:        uuid.New(),
		CreatedByName:    "Test User",
		CreatedByEmail:   "test@example.com",
		CreatedAt:        now,
		UpdatedAt:        now,
		ConfidenceScore:  75.0,
		AttestationCount: 3,
	}

	// Serialize to JSON
	data, err := json.Marshal(server)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPServer
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, server.ID, restored.ID)
	assert.Equal(t, server.Name, restored.Name)
	assert.Equal(t, server.Status, restored.Status)
	assert.Equal(t, server.IsVerified, restored.IsVerified)
	assert.Equal(t, server.TrustScore, restored.TrustScore)
	assert.Equal(t, server.Capabilities, restored.Capabilities)
}

func TestMCPServerVerificationStatus_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	status := MCPServerVerificationStatus{
		ServerID:       uuid.New(),
		IsVerified:     true,
		LastVerifiedAt: &now,
		TrustScore:     0.90,
		PublicKeyCount: 1,
		Status:         MCPServerStatusVerified,
	}

	// Serialize to JSON
	data, err := json.Marshal(status)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPServerVerificationStatus
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, status.ServerID, restored.ServerID)
	assert.Equal(t, status.IsVerified, restored.IsVerified)
	assert.Equal(t, status.TrustScore, restored.TrustScore)
	assert.Equal(t, status.Status, restored.Status)
}

func TestMCPServer_AllStatuses(t *testing.T) {
	statuses := []MCPServerStatus{
		MCPServerStatusPending,
		MCPServerStatusVerified,
		MCPServerStatusSuspended,
		MCPServerStatusRevoked,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			server := MCPServer{
				ID:             uuid.New(),
				OrganizationID: uuid.New(),
				Name:           "status-test",
				URL:            "https://test.com",
				Status:         status,
				CreatedBy:      uuid.New(),
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			assert.Equal(t, status, server.Status)
			assert.NotEmpty(t, string(status))
		})
	}
}
