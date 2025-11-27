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
// Smart suggestions based on agent type, capabilities, trust score, and MCP connections
func (s *TagService) SuggestTagsForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Get existing tags to avoid suggesting duplicates
	existingTags, err := s.tagRepo.GetAgentTags(ctx, agentID)
	if err != nil {
		existingTags = []*domain.Tag{}
	}
	existingTagKeys := make(map[string]bool)
	for _, t := range existingTags {
		existingTagKeys[t.Key+":"+t.Value] = true
	}

	// Build suggestion criteria based on agent metadata
	suggestions := make([]*domain.Tag, 0)

	// 1. Suggest based on agent type
	agentTypeSuggestions := s.suggestTagsForAgentType(agent.AgentType)

	// 2. Suggest based on capabilities
	capabilitySuggestions := s.suggestTagsForCapabilities(agent.Capabilities)

	// 3. Suggest based on trust score level
	trustSuggestions := s.suggestTagsForTrustScore(agent.TrustScore)

	// 4. Suggest based on MCP server connections
	mcpSuggestions := s.suggestTagsForMCPConnections(agent.TalksTo)

	// 5. Suggest based on status
	statusSuggestions := s.suggestTagsForAgentStatus(agent.Status)

	// Combine all suggestions
	allSuggestions := append(agentTypeSuggestions, capabilitySuggestions...)
	allSuggestions = append(allSuggestions, trustSuggestions...)
	allSuggestions = append(allSuggestions, mcpSuggestions...)
	allSuggestions = append(allSuggestions, statusSuggestions...)

	// Filter out existing tags and find matching tags in organization
	orgTags, err := s.tagRepo.List(ctx, agent.OrganizationID, nil)
	if err != nil {
		return suggestions, nil // Return empty if we can't get org tags
	}

	// Build map of organization tags for quick lookup
	orgTagMap := make(map[string]*domain.Tag)
	for _, t := range orgTags {
		orgTagMap[t.Key+":"+t.Value] = t
	}

	// Match suggestions to existing organization tags
	seenKeys := make(map[string]bool)
	for _, suggestion := range allSuggestions {
		key := suggestion.Key + ":" + suggestion.Value
		// Skip if already applied or already suggested
		if existingTagKeys[key] || seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		// If tag exists in organization, suggest it
		if orgTag, exists := orgTagMap[key]; exists {
			suggestions = append(suggestions, orgTag)
		}
		// Limit suggestions to 5
		if len(suggestions) >= 5 {
			break
		}
	}

	return suggestions, nil
}

// suggestTagsForAgentType suggests tags based on agent type
func (s *TagService) suggestTagsForAgentType(agentType domain.AgentType) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	switch agentType {
	case domain.AgentTypeAI:
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "ai-agent", Category: domain.TagCategoryAgentType})
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "autonomous", Category: domain.TagCategoryAgentType})
	case domain.AgentTypeMCP:
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "mcp-server", Category: domain.TagCategoryAgentType})
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "tool-provider", Category: domain.TagCategoryAgentType})
	}

	return suggestions
}

// suggestTagsForCapabilities suggests tags based on agent capabilities
func (s *TagService) suggestTagsForCapabilities(capabilities []string) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	for _, cap := range capabilities {
		capLower := strings.ToLower(cap)

		// File system capabilities
		if strings.Contains(capLower, "file") || strings.Contains(capLower, "filesystem") {
			suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "filesystem", Category: domain.TagCategoryResourceType})
		}

		// Database capabilities
		if strings.Contains(capLower, "database") || strings.Contains(capLower, "sql") || strings.Contains(capLower, "db") {
			suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "database", Category: domain.TagCategoryResourceType})
		}

		// Network/API capabilities
		if strings.Contains(capLower, "network") || strings.Contains(capLower, "api") || strings.Contains(capLower, "http") {
			suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "network", Category: domain.TagCategoryResourceType})
		}

		// Code execution capabilities
		if strings.Contains(capLower, "code") || strings.Contains(capLower, "execute") || strings.Contains(capLower, "shell") {
			suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "code-execution", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "classification", Value: "high-risk", Category: domain.TagCategoryDataClassification})
		}

		// Memory/state capabilities
		if strings.Contains(capLower, "memory") || strings.Contains(capLower, "state") {
			suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "stateful", Category: domain.TagCategoryResourceType})
		}

		// External service capabilities
		if strings.Contains(capLower, "external") || strings.Contains(capLower, "third-party") || strings.Contains(capLower, "service") {
			suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "external-service", Category: domain.TagCategoryResourceType})
		}
	}

	return suggestions
}

// suggestTagsForTrustScore suggests tags based on trust score level
func (s *TagService) suggestTagsForTrustScore(trustScore float64) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	if trustScore >= 8.0 {
		suggestions = append(suggestions, &domain.Tag{Key: "trust-level", Value: "high", Category: domain.TagCategoryDataClassification})
		suggestions = append(suggestions, &domain.Tag{Key: "environment", Value: "production", Category: domain.TagCategoryEnvironment})
	} else if trustScore >= 5.0 {
		suggestions = append(suggestions, &domain.Tag{Key: "trust-level", Value: "medium", Category: domain.TagCategoryDataClassification})
		suggestions = append(suggestions, &domain.Tag{Key: "environment", Value: "staging", Category: domain.TagCategoryEnvironment})
	} else {
		suggestions = append(suggestions, &domain.Tag{Key: "trust-level", Value: "low", Category: domain.TagCategoryDataClassification})
		suggestions = append(suggestions, &domain.Tag{Key: "environment", Value: "development", Category: domain.TagCategoryEnvironment})
	}

	return suggestions
}

// suggestTagsForMCPConnections suggests tags based on MCP server connections
func (s *TagService) suggestTagsForMCPConnections(talksTo []string) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	for _, mcp := range talksTo {
		mcpLower := strings.ToLower(mcp)

		// Filesystem MCP
		if strings.Contains(mcpLower, "filesystem") || strings.Contains(mcpLower, "file") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "filesystem", Category: domain.TagCategoryResourceType})
		}

		// Database MCP
		if strings.Contains(mcpLower, "postgres") || strings.Contains(mcpLower, "mysql") || strings.Contains(mcpLower, "sqlite") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "database", Category: domain.TagCategoryResourceType})
		}

		// GitHub MCP
		if strings.Contains(mcpLower, "github") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "github", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "source-control", Category: domain.TagCategoryResourceType})
		}

		// Slack MCP
		if strings.Contains(mcpLower, "slack") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "slack", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "messaging", Category: domain.TagCategoryResourceType})
		}

		// AWS MCP
		if strings.Contains(mcpLower, "aws") || strings.Contains(mcpLower, "amazon") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "aws", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "cloud", Value: "aws", Category: domain.TagCategoryEnvironment})
		}

		// Memory/Knowledge MCP
		if strings.Contains(mcpLower, "memory") || strings.Contains(mcpLower, "knowledge") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "memory", Category: domain.TagCategoryResourceType})
		}

		// Browser MCP
		if strings.Contains(mcpLower, "browser") || strings.Contains(mcpLower, "puppeteer") || strings.Contains(mcpLower, "playwright") {
			suggestions = append(suggestions, &domain.Tag{Key: "mcp", Value: "browser", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "automation", Value: "web", Category: domain.TagCategoryResourceType})
		}
	}

	return suggestions
}

// suggestTagsForAgentStatus suggests tags based on agent status
func (s *TagService) suggestTagsForAgentStatus(status domain.AgentStatus) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	switch status {
	case domain.AgentStatusVerified:
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "verified", Category: domain.TagCategoryDataClassification})
	case domain.AgentStatusPending:
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "pending-review", Category: domain.TagCategoryDataClassification})
	case domain.AgentStatusSuspended:
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "suspended", Category: domain.TagCategoryDataClassification})
		suggestions = append(suggestions, &domain.Tag{Key: "alert", Value: "requires-attention", Category: domain.TagCategoryCustom})
	case domain.AgentStatusRevoked:
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "revoked", Category: domain.TagCategoryDataClassification})
	}

	return suggestions
}

// SuggestTagsForMCPServer suggests tags based on MCP server metadata
// Smart suggestions based on MCP server name, capabilities, trust score, and status
func (s *TagService) SuggestTagsForMCPServer(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// Get existing tags to avoid suggesting duplicates
	existingTags, err := s.tagRepo.GetMCPServerTags(ctx, mcpServerID)
	if err != nil {
		existingTags = []*domain.Tag{}
	}
	existingTagKeys := make(map[string]bool)
	for _, t := range existingTags {
		existingTagKeys[t.Key+":"+t.Value] = true
	}

	// Build suggestion criteria based on MCP server metadata
	suggestions := make([]*domain.Tag, 0)

	// 1. Suggest based on MCP server name
	nameSuggestions := s.suggestTagsForMCPName(mcpServer.Name)

	// 2. Suggest based on capabilities (tools, prompts, resources)
	capabilitySuggestions := s.suggestTagsForMCPCapabilities(mcpServer.Capabilities)

	// 3. Suggest based on trust score level
	trustSuggestions := s.suggestTagsForTrustScore(mcpServer.TrustScore)

	// 4. Suggest based on verification status
	statusSuggestions := s.suggestTagsForMCPStatus(mcpServer.Status, mcpServer.IsVerified)

	// Combine all suggestions
	allSuggestions := append(nameSuggestions, capabilitySuggestions...)
	allSuggestions = append(allSuggestions, trustSuggestions...)
	allSuggestions = append(allSuggestions, statusSuggestions...)

	// Filter out existing tags and find matching tags in organization
	orgTags, err := s.tagRepo.List(ctx, mcpServer.OrganizationID, nil)
	if err != nil {
		return suggestions, nil // Return empty if we can't get org tags
	}

	// Build map of organization tags for quick lookup
	orgTagMap := make(map[string]*domain.Tag)
	for _, t := range orgTags {
		orgTagMap[t.Key+":"+t.Value] = t
	}

	// Match suggestions to existing organization tags
	seenKeys := make(map[string]bool)
	for _, suggestion := range allSuggestions {
		key := suggestion.Key + ":" + suggestion.Value
		// Skip if already applied or already suggested
		if existingTagKeys[key] || seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		// If tag exists in organization, suggest it
		if orgTag, exists := orgTagMap[key]; exists {
			suggestions = append(suggestions, orgTag)
		}
		// Limit suggestions to 5
		if len(suggestions) >= 5 {
			break
		}
	}

	return suggestions, nil
}

// suggestTagsForMCPName suggests tags based on MCP server name
func (s *TagService) suggestTagsForMCPName(name string) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)
	nameLower := strings.ToLower(name)

	// Filesystem MCP
	if strings.Contains(nameLower, "filesystem") || strings.Contains(nameLower, "file") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "filesystem", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "local-storage", Category: domain.TagCategoryResourceType})
	}

	// Database MCPs
	if strings.Contains(nameLower, "postgres") || strings.Contains(nameLower, "mysql") || strings.Contains(nameLower, "sqlite") || strings.Contains(nameLower, "database") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "database", Category: domain.TagCategoryResourceType})
	}

	// GitHub MCP
	if strings.Contains(nameLower, "github") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "source-control", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "github", Category: domain.TagCategoryResourceType})
	}

	// Slack MCP
	if strings.Contains(nameLower, "slack") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "messaging", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "slack", Category: domain.TagCategoryResourceType})
	}

	// AWS/Cloud MCPs
	if strings.Contains(nameLower, "aws") || strings.Contains(nameLower, "amazon") {
		suggestions = append(suggestions, &domain.Tag{Key: "cloud", Value: "aws", Category: domain.TagCategoryEnvironment})
	}
	if strings.Contains(nameLower, "gcp") || strings.Contains(nameLower, "google") {
		suggestions = append(suggestions, &domain.Tag{Key: "cloud", Value: "gcp", Category: domain.TagCategoryEnvironment})
	}
	if strings.Contains(nameLower, "azure") || strings.Contains(nameLower, "microsoft") {
		suggestions = append(suggestions, &domain.Tag{Key: "cloud", Value: "azure", Category: domain.TagCategoryEnvironment})
	}

	// Memory/Knowledge MCPs
	if strings.Contains(nameLower, "memory") || strings.Contains(nameLower, "knowledge") || strings.Contains(nameLower, "rag") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "memory", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "resource", Value: "knowledge-base", Category: domain.TagCategoryResourceType})
	}

	// Browser automation MCPs
	if strings.Contains(nameLower, "browser") || strings.Contains(nameLower, "puppeteer") || strings.Contains(nameLower, "playwright") || strings.Contains(nameLower, "selenium") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "browser-automation", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "automation", Value: "web", Category: domain.TagCategoryResourceType})
	}

	// Search MCPs
	if strings.Contains(nameLower, "search") || strings.Contains(nameLower, "brave") || strings.Contains(nameLower, "google") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "search", Category: domain.TagCategoryResourceType})
	}

	// Email MCPs
	if strings.Contains(nameLower, "email") || strings.Contains(nameLower, "gmail") || strings.Contains(nameLower, "smtp") {
		suggestions = append(suggestions, &domain.Tag{Key: "type", Value: "email", Category: domain.TagCategoryResourceType})
		suggestions = append(suggestions, &domain.Tag{Key: "integration", Value: "email", Category: domain.TagCategoryResourceType})
	}

	return suggestions
}

// suggestTagsForMCPCapabilities suggests tags based on MCP server capabilities
func (s *TagService) suggestTagsForMCPCapabilities(capabilities []string) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	for _, cap := range capabilities {
		capLower := strings.ToLower(cap)

		// Tools capability
		if capLower == "tools" {
			suggestions = append(suggestions, &domain.Tag{Key: "capability", Value: "tools", Category: domain.TagCategoryResourceType})
		}

		// Prompts capability
		if capLower == "prompts" {
			suggestions = append(suggestions, &domain.Tag{Key: "capability", Value: "prompts", Category: domain.TagCategoryResourceType})
		}

		// Resources capability
		if capLower == "resources" {
			suggestions = append(suggestions, &domain.Tag{Key: "capability", Value: "resources", Category: domain.TagCategoryResourceType})
		}

		// Sampling capability (advanced)
		if capLower == "sampling" {
			suggestions = append(suggestions, &domain.Tag{Key: "capability", Value: "sampling", Category: domain.TagCategoryResourceType})
			suggestions = append(suggestions, &domain.Tag{Key: "classification", Value: "advanced", Category: domain.TagCategoryDataClassification})
		}
	}

	return suggestions
}

// suggestTagsForMCPStatus suggests tags based on MCP server status
func (s *TagService) suggestTagsForMCPStatus(status domain.MCPServerStatus, isVerified bool) []*domain.Tag {
	suggestions := make([]*domain.Tag, 0)

	if isVerified {
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "verified", Category: domain.TagCategoryDataClassification})
	} else {
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "unverified", Category: domain.TagCategoryDataClassification})
	}

	switch status {
	case domain.MCPServerStatusVerified:
		suggestions = append(suggestions, &domain.Tag{Key: "availability", Value: "active", Category: domain.TagCategoryCustom})
	case domain.MCPServerStatusSuspended:
		suggestions = append(suggestions, &domain.Tag{Key: "availability", Value: "suspended", Category: domain.TagCategoryCustom})
		suggestions = append(suggestions, &domain.Tag{Key: "alert", Value: "requires-attention", Category: domain.TagCategoryCustom})
	case domain.MCPServerStatusRevoked:
		suggestions = append(suggestions, &domain.Tag{Key: "availability", Value: "revoked", Category: domain.TagCategoryCustom})
	case domain.MCPServerStatusPending:
		suggestions = append(suggestions, &domain.Tag{Key: "status", Value: "pending-review", Category: domain.TagCategoryDataClassification})
	}

	return suggestions
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
