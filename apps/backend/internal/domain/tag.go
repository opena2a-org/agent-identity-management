package domain

import (
	"time"

	"github.com/google/uuid"
)

// TagCategory represents the type of tag
type TagCategory string

const (
	TagCategoryResourceType       TagCategory = "resource_type"
	TagCategoryEnvironment        TagCategory = "environment"
	TagCategoryAgentType          TagCategory = "agent_type"
	TagCategoryDataClassification TagCategory = "data_classification"
	TagCategoryCustom             TagCategory = "custom"
)

// Tag represents a tag that can be applied to agents or MCP servers
type Tag struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	Key            string      `json:"key"`
	Value          string      `json:"value"`
	Category       TagCategory `json:"category"`
	Description    string      `json:"description"`
	Color          string      `json:"color"` // Hex color (e.g., "#3B82F6")
	CreatedAt      time.Time   `json:"created_at"`
	CreatedBy      uuid.UUID   `json:"created_by"`
}

// AgentTag represents the relationship between an agent and a tag
type AgentTag struct {
	AgentID   uuid.UUID `json:"agent_id"`
	TagID     uuid.UUID `json:"tag_id"`
	AppliedAt time.Time `json:"applied_at"`
	AppliedBy uuid.UUID `json:"applied_by"`
}

// MCPServerTag represents the relationship between an MCP server and a tag
type MCPServerTag struct {
	MCPServerID uuid.UUID `json:"mcp_server_id"`
	TagID       uuid.UUID `json:"tag_id"`
	AppliedAt   time.Time `json:"applied_at"`
	AppliedBy   uuid.UUID `json:"applied_by"`
}

// TagRepository defines the interface for tag data access
type TagRepository interface {
	// Tag CRUD
	Create(tag *Tag) error
	GetByID(id uuid.UUID) (*Tag, error)
	GetByOrganization(orgID uuid.UUID) ([]*Tag, error)
	GetByCategory(orgID uuid.UUID, category TagCategory) ([]*Tag, error)
	Delete(id uuid.UUID) error

	// Agent tag operations
	AddTagsToAgent(agentID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error
	RemoveTagFromAgent(agentID uuid.UUID, tagID uuid.UUID) error
	GetAgentTags(agentID uuid.UUID) ([]*Tag, error)
	GetAgentsByTag(tagID uuid.UUID) ([]uuid.UUID, error)

	// MCP server tag operations
	AddTagsToMCPServer(mcpServerID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error
	RemoveTagFromMCPServer(mcpServerID uuid.UUID, tagID uuid.UUID) error
	GetMCPServerTags(mcpServerID uuid.UUID) ([]*Tag, error)
	GetMCPServersByTag(tagID uuid.UUID) ([]uuid.UUID, error)
}
