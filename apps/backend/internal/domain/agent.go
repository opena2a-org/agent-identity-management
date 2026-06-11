package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AgentType represents the type of AI agent
type AgentType string

const (
	// LLM Provider-based agents
	AgentTypeClaude  AgentType = "claude"
	AgentTypeGPT     AgentType = "gpt"
	AgentTypeGemini  AgentType = "gemini"
	AgentTypeLlama   AgentType = "llama"
	AgentTypeMistral AgentType = "mistral"
	AgentTypeCohere  AgentType = "cohere"

	// Framework-based agents
	AgentTypeLangChain   AgentType = "langchain"
	AgentTypeLlamaIndex  AgentType = "llamaindex"
	AgentTypeAutoGen     AgentType = "autogen"
	AgentTypeCrewAI      AgentType = "crewai"
	AgentTypeLangGraph   AgentType = "langgraph"
	AgentTypeHaystack    AgentType = "haystack"
	AgentTypeSemanticKernel AgentType = "semantic_kernel"

	// Copilot/Assistant types
	AgentTypeCopilot   AgentType = "copilot"
	AgentTypeAssistant AgentType = "assistant"
	AgentTypeChatbot   AgentType = "chatbot"

	// Autonomous agents
	AgentTypeAutoGPT AgentType = "autogpt"
	AgentTypeBabyAGI AgentType = "babyagi"

	// Generic types
	AgentTypeCustom AgentType = "custom"

	// Legacy support
	AgentTypeAI AgentType = "ai_agent"
)

// AgentStatus represents the verification status
type AgentStatus string

const (
	AgentStatusPending   AgentStatus = "pending"
	AgentStatusVerified  AgentStatus = "verified"
	AgentStatusSuspended AgentStatus = "suspended"
	AgentStatusRevoked   AgentStatus = "revoked"
)

// Agent represents an AI agent or MCP server
type Agent struct {
	ID                       uuid.UUID   `json:"id"`
	OrganizationID           uuid.UUID   `json:"organizationId"`
	Name                     string      `json:"name"`
	DisplayName              string      `json:"displayName"`
	Description              string      `json:"description"`
	AgentType                AgentType   `json:"agentType"`
	Status                   AgentStatus `json:"status"`
	Version                  string      `json:"version"`
	PublicKey                *string     `json:"publicKey"`
	EncryptedPrivateKey      *string     `json:"-"` // Stored encrypted, never exposed in API
	KeyAlgorithm             string      `json:"keyAlgorithm"`
	CertificateURL           string      `json:"certificateUrl"`
	RepositoryURL            string      `json:"repositoryUrl"`
	DocumentationURL         string      `json:"documentationUrl"`
	TrustScore               float64     `json:"trustScore"`
	VerifiedAt               *time.Time  `json:"verifiedAt"`
	LastCapabilityCheckAt    *time.Time  `json:"lastCapabilityCheckAt"`
	CapabilityViolationCount int         `json:"capabilityViolationCount"`
	IsCompromised            bool        `json:"isCompromised"`
	// Capability-based access control (simple MVP)
	TalksTo                  []string    `json:"talksTo"` // List of MCP server names/IDs this agent can communicate with
	Capabilities             []string    `json:"capabilities"` // Agent capabilities (e.g., ["file:read", "api:call"])
	// Key rotation support
	KeyCreatedAt             *time.Time  `json:"keyCreatedAt"`
	KeyExpiresAt             *time.Time  `json:"keyExpiresAt"`
	KeyRotationGraceUntil    *time.Time  `json:"keyRotationGraceUntil,omitempty"`
	PreviousPublicKey        *string     `json:"-"` // Not exposed in API, used for grace period verification
	RotationCount            int         `json:"rotationCount"`
	// Post-Quantum Cryptography (PQC) support
	PQCPublicKey             *string     `json:"pqcPublicKey,omitempty"`
	PQCKeyAlgorithm          *string     `json:"pqcKeyAlgorithm,omitempty"` // ML-DSA-44, ML-DSA-65, ML-DSA-87
	HybridModeEnabled        bool        `json:"hybridModeEnabled"`
	PQCKeyCreatedAt          *time.Time  `json:"pqcKeyCreatedAt,omitempty"`
	PQCKeyExpiresAt          *time.Time  `json:"pqcKeyExpiresAt,omitempty"`
	PreviousPQCPublicKey     *string     `json:"-"` // Not exposed in API, used for grace period verification
	CreatedAt                time.Time   `json:"createdAt"`
	UpdatedAt                time.Time   `json:"updatedAt"`
	CreatedBy                uuid.UUID   `json:"createdBy"`
	CreatedByName            string      `json:"createdByName"`            // Denormalized for display
	CreatedByEmail           string      `json:"createdByEmail"`           // Denormalized for display
	CreatedBySDKTokenID      *uuid.UUID  `json:"createdBySdkTokenId,omitempty"` // SDK token used to create this agent
	CreatedByAPIKeyID        *uuid.UUID  `json:"createdByApiKeyId,omitempty"`   // API key used to create this agent
	UpdatedBy                *uuid.UUID  `json:"updatedBy,omitempty"`      // User who last updated this agent
	UpdatedByName            string      `json:"updatedByName,omitempty"`  // Denormalized for display
	UpdatedByEmail           string      `json:"updatedByEmail,omitempty"` // Denormalized for display
	// Tags applied to this agent (populated by join)
	Tags                     []Tag                  `json:"tags"`
	// Track when agent last performed an action (updated on every verify-action call)
	LastActive               *time.Time             `json:"lastActive"`
	// Track heartbeat for liveness monitoring
	LastHeartbeat            *time.Time             `json:"lastHeartbeat"`
	// Custom metadata for the agent (model, department, owner, etc.)
	Metadata                 map[string]interface{} `json:"metadata,omitempty"`
	// DeclaredPurpose is the publisher's optional structured declaration of what
	// the agent is for (atx-spec core.md §1.5). Identity/attestation + offline
	// detection signal only; never an authorization input. Nil = not declared.
	DeclaredPurpose *DeclaredPurpose `json:"declaredPurpose,omitempty"`
}

// AgentRepository defines the interface for agent persistence
type AgentRepository interface {
	Create(agent *Agent) error
	GetByID(id uuid.UUID) (*Agent, error)
	GetByName(orgID uuid.UUID, name string) (*Agent, error)
	GetByOrganization(orgID uuid.UUID) ([]*Agent, error)
	GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*Agent, int, error)
	Update(agent *Agent) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]*Agent, error)
	UpdateTrustScore(id uuid.UUID, newScore float64) error
	MarkAsCompromised(id uuid.UUID) error
	UpdateLastActive(ctx context.Context, agentID uuid.UUID) error
	GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*Agent, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*Agent, error)
}
