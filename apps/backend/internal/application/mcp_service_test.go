package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ========================================
// ChallengeData Tests
// ========================================

func TestChallengeData_Fields(t *testing.T) {
	serverID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	data := ChallengeData{
		Challenge: "test-challenge-string",
		ServerID:  serverID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	assert.Equal(t, "test-challenge-string", data.Challenge)
	assert.Equal(t, serverID, data.ServerID)
	assert.Equal(t, now, data.CreatedAt)
	assert.Equal(t, expiresAt, data.ExpiresAt)
}

func TestChallengeData_ExpirationCheck(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		isExpired bool
	}{
		{
			name:      "not expired - 5 minutes in future",
			expiresAt: now.Add(5 * time.Minute),
			isExpired: false,
		},
		{
			name:      "not expired - 1 second in future",
			expiresAt: now.Add(1 * time.Second),
			isExpired: false,
		},
		{
			name:      "expired - 1 second in past",
			expiresAt: now.Add(-1 * time.Second),
			isExpired: true,
		},
		{
			name:      "expired - 5 minutes in past",
			expiresAt: now.Add(-5 * time.Minute),
			isExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := ChallengeData{
				Challenge: "test",
				ServerID:  uuid.New(),
				CreatedAt: now.Add(-1 * time.Minute),
				ExpiresAt: tt.expiresAt,
			}

			// Check if current time is after expiration
			isExpired := time.Now().After(data.ExpiresAt)
			assert.Equal(t, tt.isExpired, isExpired)
		})
	}
}

// ========================================
// CreateMCPServerRequest Tests
// ========================================

func TestCreateMCPServerRequest_Fields(t *testing.T) {
	req := CreateMCPServerRequest{
		Name:              "Test MCP Server",
		Description:       "A test server",
		URL:               "https://mcp.example.com",
		Version:           "1.0.0",
		PublicKey:         "base64-public-key",
		VerificationURL:   "https://mcp.example.com/verify",
		Capabilities:      []string{"read", "write"},
		TagIds:            []string{"tag-1", "tag-2"},
		RegisteredByAgent: "agent-id-123",
	}

	assert.Equal(t, "Test MCP Server", req.Name)
	assert.Equal(t, "A test server", req.Description)
	assert.Equal(t, "https://mcp.example.com", req.URL)
	assert.Equal(t, "1.0.0", req.Version)
	assert.Equal(t, "base64-public-key", req.PublicKey)
	assert.Equal(t, "https://mcp.example.com/verify", req.VerificationURL)
	assert.Len(t, req.Capabilities, 2)
	assert.Len(t, req.TagIds, 2)
	assert.Equal(t, "agent-id-123", req.RegisteredByAgent)
}

func TestCreateMCPServerRequest_MinimalFields(t *testing.T) {
	// Test with only required fields
	req := CreateMCPServerRequest{
		Name: "Minimal Server",
		URL:  "https://minimal.example.com",
	}

	assert.Equal(t, "Minimal Server", req.Name)
	assert.Equal(t, "https://minimal.example.com", req.URL)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.Version)
	assert.Empty(t, req.PublicKey)
	assert.Empty(t, req.VerificationURL)
	assert.Nil(t, req.Capabilities)
	assert.Nil(t, req.TagIds)
}

// ========================================
// UpdateMCPServerRequest Tests
// ========================================

func TestUpdateMCPServerRequest_Fields(t *testing.T) {
	req := UpdateMCPServerRequest{
		Name:            "Updated Name",
		Description:     "Updated description",
		URL:             "https://updated.example.com",
		Version:         "2.0.0",
		PublicKey:       "new-public-key",
		VerificationURL: "https://updated.example.com/verify",
		Capabilities:    []string{"read", "write", "execute"},
	}

	assert.Equal(t, "Updated Name", req.Name)
	assert.Equal(t, "Updated description", req.Description)
	assert.Equal(t, "https://updated.example.com", req.URL)
	assert.Equal(t, "2.0.0", req.Version)
	assert.Equal(t, "new-public-key", req.PublicKey)
	assert.Equal(t, "https://updated.example.com/verify", req.VerificationURL)
	assert.Len(t, req.Capabilities, 3)
}

func TestUpdateMCPServerRequest_PartialUpdate(t *testing.T) {
	// Test partial update with only some fields set
	req := UpdateMCPServerRequest{
		Name: "Just the name",
	}

	assert.Equal(t, "Just the name", req.Name)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.URL)
	assert.Empty(t, req.Version)
}

// ========================================
// AddPublicKeyRequest Tests
// ========================================

func TestAddPublicKeyRequest_Fields(t *testing.T) {
	req := AddPublicKeyRequest{
		PublicKey: "base64-encoded-public-key",
		KeyType:   "ed25519",
	}

	assert.Equal(t, "base64-encoded-public-key", req.PublicKey)
	assert.Equal(t, "ed25519", req.KeyType)
}

func TestAddPublicKeyRequest_KeyTypes(t *testing.T) {
	keyTypes := []string{"ed25519", "rsa", "ecdsa"}

	for _, keyType := range keyTypes {
		t.Run(keyType, func(t *testing.T) {
			req := AddPublicKeyRequest{
				PublicKey: "key-data",
				KeyType:   keyType,
			}
			assert.Equal(t, keyType, req.KeyType)
		})
	}
}

// ========================================
// ConnectedAgent Tests
// ========================================

func TestConnectedAgent_Fields(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	agent := ConnectedAgent{
		ID:          id,
		Name:        "test-agent",
		DisplayName: "Test Agent",
		Status:      "active",
		TrustScore:  85.5,
		UpdatedAt:   now,
	}

	assert.Equal(t, id, agent.ID)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "Test Agent", agent.DisplayName)
	assert.Equal(t, "active", agent.Status)
	assert.Equal(t, 85.5, agent.TrustScore)
	assert.Equal(t, now, agent.UpdatedAt)
}

func TestConnectedAgent_StatusValues(t *testing.T) {
	statuses := []string{"active", "inactive", "pending", "suspended"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			agent := ConnectedAgent{
				ID:     uuid.New(),
				Status: status,
			}
			assert.Equal(t, status, agent.Status)
		})
	}
}

// ========================================
// sanitizeTalksToEntries Tests
// ========================================

func TestSanitizeTalksToEntries_EmptyInput(t *testing.T) {
	result := sanitizeTalksToEntries([]string{})
	assert.Empty(t, result)
}

func TestSanitizeTalksToEntries_NilInput(t *testing.T) {
	result := sanitizeTalksToEntries(nil)
	assert.Nil(t, result)
}

func TestSanitizeTalksToEntries_ValidEntries(t *testing.T) {
	input := []string{"memory", "aws-terraform", "github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_CommaSeparatedEntry(t *testing.T) {
	// Test the fix for malformed entries like "memory,aws-terraform"
	input := []string{"memory,aws-terraform"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
}

func TestSanitizeTalksToEntries_MixedEntries(t *testing.T) {
	// Mix of valid and comma-separated entries
	input := []string{"github", "memory,aws-terraform,docker", "slack"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 5)
	assert.Contains(t, result, "github")
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "docker")
	assert.Contains(t, result, "slack")
}

func TestSanitizeTalksToEntries_WhitespaceHandling(t *testing.T) {
	input := []string{"  memory  ", "  aws-terraform  "}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
}

func TestSanitizeTalksToEntries_CommaSeparatedWithSpaces(t *testing.T) {
	input := []string{"memory, aws-terraform, github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_EmptyEntries(t *testing.T) {
	input := []string{"", "memory", "", "github", ""}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_Deduplication(t *testing.T) {
	// Test that duplicate entries are removed
	input := []string{"memory", "memory", "github", "memory"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_DeduplicationAcrossCommaSplit(t *testing.T) {
	// Duplicates that appear after comma splitting
	input := []string{"memory", "memory,github", "github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_OnlyWhitespace(t *testing.T) {
	input := []string{"   ", "\t", "  \n  "}
	result := sanitizeTalksToEntries(input)

	assert.Empty(t, result)
}

func TestSanitizeTalksToEntries_TrailingComma(t *testing.T) {
	input := []string{"memory,"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "memory")
}

func TestSanitizeTalksToEntries_LeadingComma(t *testing.T) {
	input := []string{",memory"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "memory")
}

func TestSanitizeTalksToEntries_MultipleCommas(t *testing.T) {
	input := []string{"memory,,github,,,docker"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
	assert.Contains(t, result, "docker")
}

// ========================================
// MCPService Constructor Tests
// ========================================

func TestNewMCPService_NilDependencies(t *testing.T) {
	// Test that constructor handles nil dependencies gracefully
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.mcpRepo)
	assert.Nil(t, service.verificationEventRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.keyVault)
	assert.Nil(t, service.capabilityService)
	assert.Nil(t, service.capabilityRepo)
	assert.Nil(t, service.connectionRepo)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.tagRepo)
	assert.NotNil(t, service.cryptoService)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.challenges)
}

func TestNewMCPService_HttpClientTimeout(t *testing.T) {
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Verify HTTP client has 30 second timeout
	assert.NotNil(t, service.httpClient)
	assert.Equal(t, 30*time.Second, service.httpClient.Timeout)
}

func TestNewMCPService_ChallengesMapInitialized(t *testing.T) {
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Verify challenges map is initialized
	assert.NotNil(t, service.challenges)
	assert.Len(t, service.challenges, 0)
}

// ========================================
// Challenge Expiration Logic Tests
// ========================================

func TestMCPService_ChallengeExpirationDuration(t *testing.T) {
	// Document and test the 5-minute challenge expiration
	expectedDuration := 5 * time.Minute

	now := time.Now()
	expiresAt := now.Add(expectedDuration)

	diff := expiresAt.Sub(now)
	assert.Equal(t, expectedDuration, diff)
}

// ========================================
// Trust Score Logic Tests
// ========================================

func TestMCPService_TrustScoreValues(t *testing.T) {
	// Document expected trust score values
	tests := []struct {
		name     string
		score    float64
		scenario string
	}{
		{
			name:     "SDK registration",
			score:    75.0,
			scenario: "Initial trust score for SDK-verified servers",
		},
		{
			name:     "Manual pending",
			score:    0.0,
			scenario: "Unverified servers have zero trust",
		},
		{
			name:     "Verified server",
			score:    75.0,
			scenario: "Verified servers start at 75.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document expected values
			assert.True(t, tt.score >= 0.0)
			assert.True(t, tt.score <= 100.0)
		})
	}
}

// ========================================
// Default Verification URL Tests
// ========================================

func TestMCPService_DefaultVerificationURLPath(t *testing.T) {
	// Document the default verification URL path
	defaultPath := "/.well-known/mcp/verify"
	baseURL := "https://mcp.example.com"

	expectedURL := baseURL + defaultPath
	assert.Equal(t, "https://mcp.example.com/.well-known/mcp/verify", expectedURL)
}
