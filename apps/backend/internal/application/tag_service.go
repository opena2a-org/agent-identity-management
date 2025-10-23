package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// TagService handles business logic for tag management
type TagService struct {
	tagRepo   domain.TagRepository
	agentRepo domain.AgentRepository
	mcpRepo   domain.MCPServerRepository
}

// NewTagService creates a new tag service instance
func NewTagService(
	tagRepo domain.TagRepository,
	agentRepo domain.AgentRepository,
	mcpRepo domain.MCPServerRepository,
) *TagService {
	return &TagService{
		tagRepo:   tagRepo,
		agentRepo: agentRepo,
		mcpRepo:   mcpRepo,
	}
}

// CreateTagInput represents input for creating a new tag
type CreateTagInput struct {
	OrganizationID uuid.UUID
	Key            string
	Value          string
	Category       domain.TagCategory
	Description    string
	Color          string
	CreatedBy      uuid.UUID
}

// UpdateTagInput represents input for updating a tag
type UpdateTagInput struct {
	Key         string
	Value       string
	Category    string
	Description string
	Color       string
	UpdatedBy   uuid.UUID
}

// CreateTag creates a new tag with validation
func (s *TagService) CreateTag(ctx context.Context, input CreateTagInput) (*domain.Tag, error) {
	// Validate input
	if err := s.validateTagInput(input); err != nil {
		return nil, err
	}

	// Create tag
	tag := &domain.Tag{
		OrganizationID: input.OrganizationID,
		Key:            strings.TrimSpace(input.Key),
		Value:          strings.TrimSpace(input.Value),
		Category:       input.Category,
		Description:    input.Description,
		Color:          input.Color,
		CreatedBy:      input.CreatedBy,
	}

	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	return tag, nil
}

// GetTagsByOrganization retrieves all tags for an organization
func (s *TagService) GetTagsByOrganization(ctx context.Context, orgID uuid.UUID, category *domain.TagCategory) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.List(ctx, orgID, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	return tags, nil
}

// UpdateTag updates an existing tag
func (s *TagService) UpdateTag(ctx context.Context, tagID, orgID uuid.UUID, input UpdateTagInput) (*domain.Tag, error) {
	// Get existing tag
	tag, err := s.tagRepo.GetByID(ctx, tagID)
	if err != nil {
		return nil, fmt.Errorf("tag not found: %w", err)
	}

	// Verify tag belongs to organization
	if tag.OrganizationID != orgID {
		return nil, fmt.Errorf("tag does not belong to organization")
	}

	// Update fields if provided
	if input.Key != "" {
		if len(input.Key) > 100 {
			return nil, fmt.Errorf("tag key must be 100 characters or less")
		}
		tag.Key = strings.TrimSpace(input.Key)
	}

	if input.Value != "" {
		if len(input.Value) > 255 {
			return nil, fmt.Errorf("tag value must be 255 characters or less")
		}
		tag.Value = strings.TrimSpace(input.Value)
	}

	if input.Category != "" {
		category := domain.TagCategory(input.Category)
		validCategories := map[domain.TagCategory]bool{
			domain.TagCategoryResourceType:       true,
			domain.TagCategoryEnvironment:        true,
			domain.TagCategoryAgentType:          true,
			domain.TagCategoryDataClassification: true,
			domain.TagCategoryCustom:             true,
		}
		if !validCategories[category] {
			return nil, fmt.Errorf("invalid tag category: %s", input.Category)
		}
		tag.Category = category
	}

	if input.Description != "" {
		tag.Description = input.Description
	}

	if input.Color != "" {
		if !strings.HasPrefix(input.Color, "#") || len(input.Color) != 7 {
			return nil, fmt.Errorf("color must be a valid hex color (e.g., #3B82F6)")
		}
		tag.Color = input.Color
	}

	// Update tag in database
	if err := s.tagRepo.Update(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to update tag: %w", err)
	}

	return tag, nil
}

// DeleteTag deletes a tag (only if not in use)
func (s *TagService) DeleteTag(ctx context.Context, tagID uuid.UUID) error {
	if err := s.tagRepo.Delete(ctx, tagID); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// AddTagsToAgent adds tags to an agent with smart suggestions
func (s *TagService) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	// Verify agent exists
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Verify all tags exist and belong to same organization
	for _, tagID := range tagIDs {
		tag, err := s.tagRepo.GetByID(ctx, tagID)
		if err != nil {
			return fmt.Errorf("tag %s not found: %w", tagID, err)
		}
		if tag.OrganizationID != agent.OrganizationID {
			return fmt.Errorf("tag %s does not belong to agent's organization", tagID)
		}
	}

	// Add tags (database trigger enforces Community Edition 3-tag limit)
	if err := s.tagRepo.AddTagsToAgent(ctx, agentID, tagIDs); err != nil {
		return fmt.Errorf("failed to add tags to agent: %w", err)
	}

	return nil
}

// RemoveTagFromAgent removes a tag from an agent
func (s *TagService) RemoveTagFromAgent(ctx context.Context, agentID, tagID uuid.UUID) error {
	if err := s.tagRepo.RemoveTagFromAgent(ctx, agentID, tagID); err != nil {
		return fmt.Errorf("failed to remove tag from agent: %w", err)
	}
	return nil
}

// GetAgentTags retrieves all tags for an agent
func (s *TagService) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetAgentTags(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tags: %w", err)
	}
	return tags, nil
}

// AddTagsToMCPServer adds tags to an MCP server with smart suggestions
func (s *TagService) AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	// Verify MCP server exists
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return fmt.Errorf("mcp server not found: %w", err)
	}

	// Verify all tags exist and belong to same organization
	for _, tagID := range tagIDs {
		tag, err := s.tagRepo.GetByID(ctx, tagID)
		if err != nil {
			return fmt.Errorf("tag %s not found: %w", tagID, err)
		}
		if tag.OrganizationID != mcpServer.OrganizationID {
			return fmt.Errorf("tag %s does not belong to mcp server's organization", tagID)
		}
	}

	// Add tags (database trigger enforces Community Edition 3-tag limit)
	if err := s.tagRepo.AddTagsToMCPServer(ctx, mcpServerID, tagIDs); err != nil {
		return fmt.Errorf("failed to add tags to mcp server: %w", err)
	}

	return nil
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (s *TagService) RemoveTagFromMCPServer(ctx context.Context, mcpServerID, tagID uuid.UUID) error {
	if err := s.tagRepo.RemoveTagFromMCPServer(ctx, mcpServerID, tagID); err != nil {
		return fmt.Errorf("failed to remove tag from mcp server: %w", err)
	}
	return nil
}

// GetMCPServerTags retrieves all tags for an MCP server
func (s *TagService) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetMCPServerTags(ctx, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server tags: %w", err)
	}
	return tags, nil
}

// SuggestTagsForAgent suggests tags based on agent metadata
// TODO: Implement smart suggestions when capabilities tracking is added
func (s *TagService) SuggestTagsForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	_, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Return empty suggestions for now
	// Future: Analyze agent type, version, metadata for smart suggestions
	return []*domain.Tag{}, nil
}

// SuggestTagsForMCPServer suggests tags based on MCP server metadata
// TODO: Implement smart suggestions when capabilities tracking is added
func (s *TagService) SuggestTagsForMCPServer(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	_, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// Return empty suggestions for now
	// Future: Analyze MCP server type, version, metadata for smart suggestions
	return []*domain.Tag{}, nil
}

// GetPopularTags retrieves the most popular tags by usage count
func (s *TagService) GetPopularTags(ctx context.Context, orgID uuid.UUID, limit int) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetPopularTags(ctx, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular tags: %w", err)
	}
	return tags, nil
}

// SearchTags searches for tags by query string (case-insensitive)
func (s *TagService) SearchTags(ctx context.Context, orgID uuid.UUID, query string, categoryFilter string) ([]*domain.Tag, error) {
	var category *domain.TagCategory
	if categoryFilter != "" {
		cat := domain.TagCategory(categoryFilter)
		category = &cat
	}

	tags, err := s.tagRepo.SearchTags(ctx, orgID, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to search tags: %w", err)
	}
	return tags, nil
}

// validateTagInput validates tag creation input
func (s *TagService) validateTagInput(input CreateTagInput) error {
	if input.Key == "" {
		return fmt.Errorf("tag key is required")
	}
	if input.Value == "" {
		return fmt.Errorf("tag value is required")
	}
	if len(input.Key) > 100 {
		return fmt.Errorf("tag key must be 100 characters or less")
	}
	if len(input.Value) > 255 {
		return fmt.Errorf("tag value must be 255 characters or less")
	}

	// Validate category
	validCategories := map[domain.TagCategory]bool{
		domain.TagCategoryResourceType:       true,
		domain.TagCategoryEnvironment:        true,
		domain.TagCategoryAgentType:          true,
		domain.TagCategoryDataClassification: true,
		domain.TagCategoryCustom:             true,
	}
	if !validCategories[input.Category] {
		return fmt.Errorf("invalid tag category: %s", input.Category)
	}

	// Validate color format (hex color)
	if input.Color != "" {
		if !strings.HasPrefix(input.Color, "#") || len(input.Color) != 7 {
			return fmt.Errorf("color must be a valid hex color (e.g., #3B82F6)")
		}
	}

	return nil
}
