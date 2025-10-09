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
	MCPServerID      uuid.UUID         `json:"mcp_server_id"`
	Name             string            `json:"name"`        // e.g., "get_weather", "search_code"
	CapabilityType   MCPCapabilityType `json:"type"`        // tool, resource, or prompt
	Description      string            `json:"description"` // Human-readable description
	CapabilitySchema json.RawMessage   `json:"schema"`      // JSON schema for input/output
	DetectedAt       time.Time         `json:"detected_at"`
	LastVerifiedAt   *time.Time        `json:"last_verified_at"`
	IsActive         bool              `json:"is_active"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
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
	ServerID      uuid.UUID `json:"server_id"`
	TotalCount    int       `json:"total_count"`
	ToolCount     int       `json:"tool_count"`
	ResourceCount int       `json:"resource_count"`
	PromptCount   int       `json:"prompt_count"`
}
