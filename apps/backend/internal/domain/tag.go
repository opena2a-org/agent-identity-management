package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TagCategory represents the category of a tag
type TagCategory string

const (
	TagCategoryResourceType      TagCategory = "resource_type"
	TagCategoryEnvironment       TagCategory = "environment"
	TagCategoryAgentType         TagCategory = "agent_type"
	TagCategoryDataClassification TagCategory = "data_classification"
	TagCategoryCustom            TagCategory = "custom"
)

// Tag represents a label that can be attached to agents and MCP servers
type Tag struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organizationId"`
	Key            string      `json:"key"`
	Value          string      `json:"value"`
	Category       TagCategory `json:"category"`
	Description    string      `json:"description"`
	Color          string      `json:"color"`
	CreatedAt      time.Time   `json:"createdAt"`
	CreatedBy      uuid.UUID   `json:"createdBy"`
}

// CreateTagInput represents input for creating a new tag
type CreateTagInput struct {
	Key         string      `json:"key" validate:"required,min=1,max=50"`
	Value       string      `json:"value" validate:"required,min=1,max=100"`
	Category    TagCategory `json:"category" validate:"required,oneof=resource_type environment agent_type data_classification custom"`
	Description string      `json:"description" validate:"max=500"`
	Color       string      `json:"color" validate:"omitempty,hexcolor"`
}

// TagRepository defines the interface for tag data access
type TagRepository interface {
	// Tag CRUD
	Create(ctx context.Context, tag *Tag) error
	GetByID(ctx context.Context, id uuid.UUID) (*Tag, error)
	Update(ctx context.Context, tag *Tag) error
	List(ctx context.Context, organizationID uuid.UUID, category *TagCategory) ([]*Tag, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Tag Search & Discovery
	GetPopularTags(ctx context.Context, organizationID uuid.UUID, limit int) ([]*Tag, error)
	SearchTags(ctx context.Context, organizationID uuid.UUID, query string, category *TagCategory) ([]*Tag, error)

	// Agent Tag Relationships
	AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID) error
	RemoveTagFromAgent(ctx context.Context, agentID uuid.UUID, tagID uuid.UUID) error
	GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*Tag, error)

	// MCP Server Tag Relationships
	AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID) error
	RemoveTagFromMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagID uuid.UUID) error
	GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*Tag, error)
}
