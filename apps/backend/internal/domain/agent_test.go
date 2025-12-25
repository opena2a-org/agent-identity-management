package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAgentTypeConstants(t *testing.T) {
	// LLM Provider-based agents
	assert.Equal(t, AgentType("claude"), AgentTypeClaude)
	assert.Equal(t, AgentType("gpt"), AgentTypeGPT)
	assert.Equal(t, AgentType("gemini"), AgentTypeGemini)
	assert.Equal(t, AgentType("llama"), AgentTypeLlama)
	assert.Equal(t, AgentType("mistral"), AgentTypeMistral)
	assert.Equal(t, AgentType("cohere"), AgentTypeCohere)

	// Framework-based agents
	assert.Equal(t, AgentType("langchain"), AgentTypeLangChain)
	assert.Equal(t, AgentType("llamaindex"), AgentTypeLlamaIndex)
	assert.Equal(t, AgentType("autogen"), AgentTypeAutoGen)
	assert.Equal(t, AgentType("crewai"), AgentTypeCrewAI)
	assert.Equal(t, AgentType("langgraph"), AgentTypeLangGraph)
	assert.Equal(t, AgentType("haystack"), AgentTypeHaystack)
	assert.Equal(t, AgentType("semantic_kernel"), AgentTypeSemanticKernel)

	// Copilot/Assistant types
	assert.Equal(t, AgentType("copilot"), AgentTypeCopilot)
	assert.Equal(t, AgentType("assistant"), AgentTypeAssistant)
	assert.Equal(t, AgentType("chatbot"), AgentTypeChatbot)

	// Autonomous agents
	assert.Equal(t, AgentType("autogpt"), AgentTypeAutoGPT)
	assert.Equal(t, AgentType("babyagi"), AgentTypeBabyAGI)

	// Generic types
	assert.Equal(t, AgentType("custom"), AgentTypeCustom)
	assert.Equal(t, AgentType("ai_agent"), AgentTypeAI)
}

func TestAgentStatusConstants(t *testing.T) {
	assert.Equal(t, AgentStatus("pending"), AgentStatusPending)
	assert.Equal(t, AgentStatus("verified"), AgentStatusVerified)
	assert.Equal(t, AgentStatus("suspended"), AgentStatusSuspended)
	assert.Equal(t, AgentStatus("revoked"), AgentStatusRevoked)
}

func TestAgentStruct(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	orgID := uuid.New()
	createdBy := uuid.New()
	publicKey := "test-public-key"

	agent := Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		DisplayName:    "Test Agent",
		Description:    "A test agent for unit testing",
		AgentType:      AgentTypeClaude,
		Status:         AgentStatusVerified,
		Version:        "1.0.0",
		PublicKey:      &publicKey,
		TrustScore:     0.85,
		Capabilities:   []string{"file:read", "api:call"},
		TalksTo:        []string{"mcp-server-1", "mcp-server-2"},
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
		CreatedByName:  "Test User",
		CreatedByEmail: "test@example.com",
	}

	assert.Equal(t, agentID, agent.ID)
	assert.Equal(t, orgID, agent.OrganizationID)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "Test Agent", agent.DisplayName)
	assert.Equal(t, AgentTypeClaude, agent.AgentType)
	assert.Equal(t, AgentStatusVerified, agent.Status)
	assert.Equal(t, 0.85, agent.TrustScore)
	assert.Len(t, agent.Capabilities, 2)
	assert.Len(t, agent.TalksTo, 2)
	assert.Contains(t, agent.Capabilities, "file:read")
	assert.Contains(t, agent.TalksTo, "mcp-server-1")
}

func TestAgentWithMetadata(t *testing.T) {
	agent := Agent{
		ID:        uuid.New(),
		Name:      "agent-with-metadata",
		AgentType: AgentTypeCustom,
		Metadata: map[string]interface{}{
			"model":      "gpt-4",
			"department": "engineering",
			"owner":      "team-a",
			"version":    1.0,
		},
	}

	assert.NotNil(t, agent.Metadata)
	assert.Equal(t, "gpt-4", agent.Metadata["model"])
	assert.Equal(t, "engineering", agent.Metadata["department"])
	assert.Equal(t, "team-a", agent.Metadata["owner"])
}

func TestAgentWithTags(t *testing.T) {
	agent := Agent{
		ID:   uuid.New(),
		Name: "agent-with-tags",
		Tags: []Tag{
			{ID: uuid.New(), Key: "environment", Value: "production", Color: "#00FF00"},
			{ID: uuid.New(), Key: "priority", Value: "critical", Color: "#FF0000"},
		},
	}

	assert.Len(t, agent.Tags, 2)
	assert.Equal(t, "environment", agent.Tags[0].Key)
	assert.Equal(t, "production", agent.Tags[0].Value)
	assert.Equal(t, "priority", agent.Tags[1].Key)
	assert.Equal(t, "critical", agent.Tags[1].Value)
}

func TestAgentKeyRotationFields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(90 * 24 * time.Hour) // 90 days
	graceUntil := now.Add(7 * 24 * time.Hour) // 7 days grace
	previousKey := "previous-public-key"

	agent := Agent{
		ID:                    uuid.New(),
		Name:                  "agent-with-rotation",
		KeyCreatedAt:          &now,
		KeyExpiresAt:          &expiresAt,
		KeyRotationGraceUntil: &graceUntil,
		PreviousPublicKey:     &previousKey,
		RotationCount:         3,
	}

	assert.NotNil(t, agent.KeyCreatedAt)
	assert.NotNil(t, agent.KeyExpiresAt)
	assert.NotNil(t, agent.KeyRotationGraceUntil)
	assert.Equal(t, 3, agent.RotationCount)
}

func TestAgentCompromisedFlag(t *testing.T) {
	agent := Agent{
		ID:            uuid.New(),
		Name:          "compromised-agent",
		IsCompromised: true,
	}

	assert.True(t, agent.IsCompromised)
}

func TestAgentCapabilityViolationCount(t *testing.T) {
	agent := Agent{
		ID:                       uuid.New(),
		Name:                     "violating-agent",
		CapabilityViolationCount: 5,
	}

	assert.Equal(t, 5, agent.CapabilityViolationCount)
}

func TestAgentTypeStringConversion(t *testing.T) {
	agentType := AgentTypeClaude
	str := string(agentType)
	assert.Equal(t, "claude", str)

	// Convert back
	converted := AgentType(str)
	assert.Equal(t, AgentTypeClaude, converted)
}

func TestAgentStatusStringConversion(t *testing.T) {
	status := AgentStatusVerified
	str := string(status)
	assert.Equal(t, "verified", str)

	// Convert back
	converted := AgentStatus(str)
	assert.Equal(t, AgentStatusVerified, converted)
}

func TestAgentWithNilOptionalFields(t *testing.T) {
	agent := Agent{
		ID:        uuid.New(),
		Name:      "minimal-agent",
		AgentType: AgentTypeCustom,
		Status:    AgentStatusPending,
	}

	// Verify nil optional fields don't cause issues
	assert.Nil(t, agent.PublicKey)
	assert.Nil(t, agent.VerifiedAt)
	assert.Nil(t, agent.LastCapabilityCheckAt)
	assert.Nil(t, agent.KeyCreatedAt)
	assert.Nil(t, agent.KeyExpiresAt)
	assert.Nil(t, agent.LastActive)
	assert.Nil(t, agent.UpdatedBy)
}
