package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MCPCapabilityType represents the type of MCP capability
type MCPCapabilityType string

const (
	MCPCapabilityTypeTool     MCPCapabilityType = "tool"
	MCPCapabilityTypeResource MCPCapabilityType = "resource"
	MCPCapabilityTypePrompt   MCPCapabilityType = "prompt"
)

// MCPServerCapability represents an individual capability exposed by an MCP server
type MCPServerCapability struct {
	ID               uuid.UUID         `json:"id"`
	MCPServerID      uuid.UUID         `json:"mcpServerId"`
	Name             string            `json:"name"`        // e.g., "get_weather", "search_code"
	CapabilityType   MCPCapabilityType `json:"type"`        // tool, resource, or prompt
	Description      string            `json:"description"` // Human-readable description
	CapabilitySchema json.RawMessage   `json:"schema"`      // JSON schema for input/output
	DetectedAt       time.Time         `json:"detectedAt"`
	LastVerifiedAt   *time.Time        `json:"lastVerifiedAt"`
	IsActive         bool              `json:"isActive"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// MCPServerCapabilityRepository defines the interface for MCP capability persistence
type MCPServerCapabilityRepository interface {
	Create(capability *MCPServerCapability) error
	GetByID(id uuid.UUID) (*MCPServerCapability, error)
	GetByServerID(serverID uuid.UUID) ([]*MCPServerCapability, error)
	GetByServerIDAndType(serverID uuid.UUID, capType MCPCapabilityType) ([]*MCPServerCapability, error)
	Update(capability *MCPServerCapability) error
	Delete(id uuid.UUID) error
	DeleteByServerID(serverID uuid.UUID) error
}

// MCPCapabilitySummary represents a summary of capabilities by type
type MCPCapabilitySummary struct {
	ServerID      uuid.UUID `json:"serverId"`
	TotalCount    int       `json:"totalCount"`
	ToolCount     int       `json:"toolCount"`
	ResourceCount int       `json:"resourceCount"`
	PromptCount   int       `json:"promptCount"`
}
