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

	if err := s.tagRepo.Create(tag); err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	return tag, nil
}

// GetTagsByOrganization retrieves all tags for an organization
func (s *TagService) GetTagsByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	return tags, nil
}

// GetTagsByCategory retrieves tags by category for an organization
func (s *TagService) GetTagsByCategory(ctx context.Context, orgID uuid.UUID, category domain.TagCategory) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetByCategory(orgID, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags by category: %w", err)
	}
	return tags, nil
}

// DeleteTag deletes a tag (only if not in use)
func (s *TagService) DeleteTag(ctx context.Context, tagID uuid.UUID) error {
	if err := s.tagRepo.Delete(tagID); err != nil {
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
		tag, err := s.tagRepo.GetByID(tagID)
		if err != nil {
			return fmt.Errorf("tag %s not found: %w", tagID, err)
		}
		if tag.OrganizationID != agent.OrganizationID {
			return fmt.Errorf("tag %s does not belong to agent's organization", tagID)
		}
	}

	// Add tags (database trigger enforces Community Edition 3-tag limit)
	if err := s.tagRepo.AddTagsToAgent(agentID, tagIDs, appliedBy); err != nil {
		return fmt.Errorf("failed to add tags to agent: %w", err)
	}

	return nil
}

// RemoveTagFromAgent removes a tag from an agent
func (s *TagService) RemoveTagFromAgent(ctx context.Context, agentID, tagID uuid.UUID) error {
	if err := s.tagRepo.RemoveTagFromAgent(agentID, tagID); err != nil {
		return fmt.Errorf("failed to remove tag from agent: %w", err)
	}
	return nil
}

// GetAgentTags retrieves all tags for an agent
func (s *TagService) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetAgentTags(agentID)
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
		tag, err := s.tagRepo.GetByID(tagID)
		if err != nil {
			return fmt.Errorf("tag %s not found: %w", tagID, err)
		}
		if tag.OrganizationID != mcpServer.OrganizationID {
			return fmt.Errorf("tag %s does not belong to mcp server's organization", tagID)
		}
	}

	// Add tags (database trigger enforces Community Edition 3-tag limit)
	if err := s.tagRepo.AddTagsToMCPServer(mcpServerID, tagIDs, appliedBy); err != nil {
		return fmt.Errorf("failed to add tags to mcp server: %w", err)
	}

	return nil
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (s *TagService) RemoveTagFromMCPServer(ctx context.Context, mcpServerID, tagID uuid.UUID) error {
	if err := s.tagRepo.RemoveTagFromMCPServer(mcpServerID, tagID); err != nil {
		return fmt.Errorf("failed to remove tag from mcp server: %w", err)
	}
	return nil
}

// GetMCPServerTags retrieves all tags for an MCP server
func (s *TagService) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	tags, err := s.tagRepo.GetMCPServerTags(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server tags: %w", err)
	}
	return tags, nil
}

// SuggestTagsForAgent suggests tags based on agent capabilities
func (s *TagService) SuggestTagsForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Get all existing tags for organization
	allTags, err := s.tagRepo.GetByOrganization(agent.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization tags: %w", err)
	}

	// Detect resource types from capabilities
	suggestions := s.detectResourceTypesFromCapabilities(agent.Capabilities, allTags)

	return suggestions, nil
}

// SuggestTagsForMCPServer suggests tags based on MCP server capabilities
func (s *TagService) SuggestTagsForMCPServer(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// Get all existing tags for organization
	allTags, err := s.tagRepo.GetByOrganization(mcpServer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization tags: %w", err)
	}

	// Detect resource types from capabilities
	suggestions := s.detectResourceTypesFromCapabilities(mcpServer.Capabilities, allTags)

	return suggestions, nil
}

// detectResourceTypesFromCapabilities analyzes capabilities and suggests resource type tags
func (s *TagService) detectResourceTypesFromCapabilities(capabilities []string, allTags []*domain.Tag) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)
	suggestedValues := make(map[string]bool)

	// Capability patterns to tag mappings
	patterns := map[string]string{
		"file":       "filesystem",
		"read_file":  "filesystem",
		"write_file": "filesystem",
		"database":   "database",
		"db":         "database",
		"sql":        "database",
		"api":        "api",
		"http":       "api",
		"rest":       "api",
		"network":    "network",
		"socket":     "network",
		"ai":         "ai_model",
		"ml":         "ai_model",
		"model":      "ai_model",
		"crypto":     "cryptography",
		"encrypt":    "cryptography",
		"hash":       "cryptography",
		"auth":       "authentication",
		"login":      "authentication",
	}

	// Analyze capabilities
	for _, capability := range capabilities {
		capLower := strings.ToLower(capability)
		for pattern, tagValue := range patterns {
			if strings.Contains(capLower, pattern) {
				// Avoid duplicates
				if suggestedValues[tagValue] {
					continue
				}
				suggestedValues[tagValue] = true

				// Find matching tag in organization's tags
				for _, tag := range allTags {
					if tag.Category == domain.TagCategoryResourceType && tag.Value == tagValue {
						suggestions = append(suggestions, tag)
						break
					}
				}
			}
		}
	}

	return suggestions
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
