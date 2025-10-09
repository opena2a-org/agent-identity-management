# 🏷️ AIM Tagging Implementation Plan - Community Edition (MVP)

**Date**: October 8, 2025
**Status**: Implementation Plan
**Target**: Community Edition (Free Tier)
**Timeline**: 2 weeks (80 hours)
**Goal**: Basic tagging for agents and MCP servers with 3 tags max

---

## 🎯 MVP Scope (Community Edition)

### What's Included (Free Tier)
- ✅ **Basic tagging** (max 3 tags per asset)
- ✅ **Tag CRUD** (create, read, update, delete tags)
- ✅ **Tag filtering** (filter agents/MCPs by tags in dashboard)
- ✅ **Tag display** (show tags in detail modals and list views)
- ✅ **Smart tag suggestions** (auto-detect from capabilities)
- ✅ **5 predefined tag categories**:
  - Resource Type (filesystem, database, api, etc.)
  - Environment (production, staging, development)
  - Agent Type (customer-facing, autonomous, supervised)
  - Data Classification (public, internal, pii)
  - Custom (user-defined)

### What's NOT Included (Premium Features)
- ❌ Unlimited tags (Community limited to 3 tags)
- ❌ Required tag policies
- ❌ Tag-based RBAC
- ❌ Tag compliance reports
- ❌ Tag analytics dashboard
- ❌ Bulk tag operations
- ❌ Tag hierarchies/taxonomies

---

## 📊 Database Schema

### Migration 022: Add Tags Tables

**File**: `apps/backend/migrations/022_create_tags_tables.up.sql`

```sql
-- ============================================================================
-- Migration 022: Create Tags Tables for Agents and MCP Servers
-- Author: Claude Sonnet 4.5
-- Date: October 8, 2025
-- Description: Basic tagging system for Community Edition (max 3 tags per asset)
-- ============================================================================

-- Tags table (organization-scoped)
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL, -- 'resource_type', 'environment', 'agent_type', 'data_classification', 'custom'
    description TEXT,
    color VARCHAR(7), -- Hex color for UI (e.g., '#3B82F6')
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),

    UNIQUE(organization_id, key, value),
    CHECK (category IN ('resource_type', 'environment', 'agent_type', 'data_classification', 'custom'))
);

-- Indexes for tag lookups
CREATE INDEX idx_tags_org_id ON tags(organization_id);
CREATE INDEX idx_tags_category ON tags(organization_id, category);
CREATE INDEX idx_tags_key_value ON tags(key, value);

-- Comments
COMMENT ON TABLE tags IS 'Organization-scoped tags for categorizing agents and MCP servers';
COMMENT ON COLUMN tags.category IS 'Tag category: resource_type, environment, agent_type, data_classification, custom';
COMMENT ON COLUMN tags.color IS 'Hex color code for UI display (e.g., #3B82F6 for blue)';

-- Agent tags (many-to-many)
CREATE TABLE IF NOT EXISTS agent_tags (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (agent_id, tag_id)
);

-- Indexes for agent tag queries
CREATE INDEX idx_agent_tags_agent_id ON agent_tags(agent_id);
CREATE INDEX idx_agent_tags_tag_id ON agent_tags(tag_id);

COMMENT ON TABLE agent_tags IS 'Many-to-many relationship between agents and tags';

-- MCP server tags (many-to-many)
CREATE TABLE IF NOT EXISTS mcp_server_tags (
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (mcp_server_id, tag_id)
);

-- Indexes for MCP server tag queries
CREATE INDEX idx_mcp_server_tags_mcp_id ON mcp_server_tags(mcp_server_id);
CREATE INDEX idx_mcp_server_tags_tag_id ON mcp_server_tags(tag_id);

COMMENT ON TABLE mcp_server_tags IS 'Many-to-many relationship between MCP servers and tags';

-- Function to enforce Community Edition tag limit (max 3 tags per asset)
CREATE OR REPLACE FUNCTION enforce_community_tag_limit()
RETURNS TRIGGER AS $$
DECLARE
    tag_count INT;
    org_tier VARCHAR(50);
BEGIN
    -- Get organization tier
    SELECT tier INTO org_tier
    FROM organizations
    WHERE id = (
        SELECT organization_id FROM agents WHERE id = NEW.agent_id
        UNION
        SELECT organization_id FROM mcp_servers WHERE id = NEW.mcp_server_id
    );

    -- Only enforce for Community tier
    IF org_tier = 'community' THEN
        -- Count existing tags
        SELECT COUNT(*) INTO tag_count
        FROM (
            SELECT agent_id FROM agent_tags WHERE agent_id = NEW.agent_id
            UNION ALL
            SELECT mcp_server_id FROM mcp_server_tags WHERE mcp_server_id = NEW.mcp_server_id
        ) AS all_tags;

        -- Enforce 3 tag limit
        IF tag_count >= 3 THEN
            RAISE EXCEPTION 'Community Edition limited to 3 tags per asset. Upgrade to Pro for unlimited tags.';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to agent_tags
CREATE TRIGGER enforce_agent_tag_limit
    BEFORE INSERT ON agent_tags
    FOR EACH ROW
    EXECUTE FUNCTION enforce_community_tag_limit();

-- Apply trigger to mcp_server_tags
CREATE TRIGGER enforce_mcp_tag_limit
    BEFORE INSERT ON mcp_server_tags
    FOR EACH ROW
    EXECUTE FUNCTION enforce_community_tag_limit();

-- Insert default tags for all organizations
-- (These are suggestions, users can create their own)
INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'resource_type',
    'filesystem',
    'resource_type',
    'File system operations (read, write, list)',
    '#10B981' -- green
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'resource_type',
    'database',
    'resource_type',
    'Database connections and queries',
    '#3B82F6' -- blue
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'environment',
    'production',
    'environment',
    'Production environment',
    '#EF4444' -- red
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'environment',
    'development',
    'environment',
    'Development environment',
    '#F59E0B' -- yellow
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;
```

**File**: `apps/backend/migrations/022_create_tags_tables.down.sql`

```sql
-- Rollback migration 022

-- Drop triggers
DROP TRIGGER IF EXISTS enforce_mcp_tag_limit ON mcp_server_tags;
DROP TRIGGER IF EXISTS enforce_agent_tag_limit ON agent_tags;

-- Drop function
DROP FUNCTION IF EXISTS enforce_community_tag_limit();

-- Drop tables (cascade removes indexes and constraints)
DROP TABLE IF EXISTS mcp_server_tags CASCADE;
DROP TABLE IF EXISTS agent_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
```

---

## 🏗️ Backend Implementation

### Phase 1: Domain Models (Day 1)

**File**: `apps/backend/internal/domain/tag.go`

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// TagCategory represents the type of tag
type TagCategory string

const (
	TagCategoryResourceType      TagCategory = "resource_type"
	TagCategoryEnvironment       TagCategory = "environment"
	TagCategoryAgentType         TagCategory = "agent_type"
	TagCategoryDataClassification TagCategory = "data_classification"
	TagCategoryCustom            TagCategory = "custom"
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
```

**Update existing domain models**:

**File**: `apps/backend/internal/domain/agent.go` (add tags field)

```go
type Agent struct {
	// ... existing fields

	// Tags applied to this agent (populated by join)
	Tags []Tag `json:"tags"` // NEW: Agent tags
}
```

**File**: `apps/backend/internal/domain/mcp_server.go` (add tags field)

```go
type MCPServer struct {
	// ... existing fields

	// Tags applied to this MCP server (populated by join)
	Tags []Tag `json:"tags"` // NEW: MCP server tags
}
```

### Phase 2: Repository Layer (Day 2-3)

**File**: `apps/backend/internal/infrastructure/repository/tag_repository.go`

```go
package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type TagRepository struct {
	db *sql.DB
}

func NewTagRepository(db *sql.DB) *TagRepository {
	return &TagRepository{db: db}
}

// Create creates a new tag
func (r *TagRepository) Create(tag *domain.Tag) error {
	query := `
		INSERT INTO tags (id, organization_id, key, value, category, description, color, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	tag.ID = uuid.New()
	tag.CreatedAt = time.Now()

	_, err := r.db.Exec(query,
		tag.ID,
		tag.OrganizationID,
		tag.Key,
		tag.Value,
		tag.Category,
		tag.Description,
		tag.Color,
		tag.CreatedAt,
		tag.CreatedBy,
	)

	return err
}

// GetByID retrieves a tag by ID
func (r *TagRepository) GetByID(id uuid.UUID) (*domain.Tag, error) {
	query := `
		SELECT id, organization_id, key, value, category, description, color, created_at, created_by
		FROM tags
		WHERE id = $1
	`

	tag := &domain.Tag{}
	err := r.db.QueryRow(query, id).Scan(
		&tag.ID,
		&tag.OrganizationID,
		&tag.Key,
		&tag.Value,
		&tag.Category,
		&tag.Description,
		&tag.Color,
		&tag.CreatedAt,
		&tag.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tag not found")
	}

	return tag, err
}

// GetByOrganization retrieves all tags for an organization
func (r *TagRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Tag, error) {
	query := `
		SELECT id, organization_id, key, value, category, description, color, created_at, created_by
		FROM tags
		WHERE organization_id = $1
		ORDER BY category, key, value
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.Tag
	for rows.Next() {
		tag := &domain.Tag{}
		err := rows.Scan(
			&tag.ID,
			&tag.OrganizationID,
			&tag.Key,
			&tag.Value,
			&tag.Category,
			&tag.Description,
			&tag.Color,
			&tag.CreatedAt,
			&tag.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetByCategory retrieves tags by category for an organization
func (r *TagRepository) GetByCategory(orgID uuid.UUID, category domain.TagCategory) ([]*domain.Tag, error) {
	query := `
		SELECT id, organization_id, key, value, category, description, color, created_at, created_by
		FROM tags
		WHERE organization_id = $1 AND category = $2
		ORDER BY key, value
	`

	rows, err := r.db.Query(query, orgID, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.Tag
	for rows.Next() {
		tag := &domain.Tag{}
		err := rows.Scan(
			&tag.ID,
			&tag.OrganizationID,
			&tag.Key,
			&tag.Value,
			&tag.Category,
			&tag.Description,
			&tag.Color,
			&tag.CreatedAt,
			&tag.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// Delete deletes a tag (only if not in use)
func (r *TagRepository) Delete(id uuid.UUID) error {
	// Check if tag is in use
	var count int
	checkQuery := `
		SELECT COUNT(*) FROM (
			SELECT agent_id FROM agent_tags WHERE tag_id = $1
			UNION ALL
			SELECT mcp_server_id FROM mcp_server_tags WHERE tag_id = $1
		) AS usage
	`
	err := r.db.QueryRow(checkQuery, id).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("cannot delete tag: in use by %d assets", count)
	}

	// Delete tag
	query := `DELETE FROM tags WHERE id = $1`
	_, err = r.db.Exec(query, id)
	return err
}

// AddTagsToAgent adds tags to an agent
func (r *TagRepository) AddTagsToAgent(agentID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO agent_tags (agent_id, tag_id, applied_at, applied_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (agent_id, tag_id) DO NOTHING
	`

	now := time.Now()
	for _, tagID := range tagIDs {
		_, err := tx.Exec(query, agentID, tagID, now, appliedBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RemoveTagFromAgent removes a tag from an agent
func (r *TagRepository) RemoveTagFromAgent(agentID uuid.UUID, tagID uuid.UUID) error {
	query := `DELETE FROM agent_tags WHERE agent_id = $1 AND tag_id = $2`
	_, err := r.db.Exec(query, agentID, tagID)
	return err
}

// GetAgentTags retrieves all tags for an agent
func (r *TagRepository) GetAgentTags(agentID uuid.UUID) ([]*domain.Tag, error) {
	query := `
		SELECT t.id, t.organization_id, t.key, t.value, t.category, t.description, t.color, t.created_at, t.created_by
		FROM tags t
		INNER JOIN agent_tags at ON t.id = at.tag_id
		WHERE at.agent_id = $1
		ORDER BY t.category, t.key, t.value
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.Tag
	for rows.Next() {
		tag := &domain.Tag{}
		err := rows.Scan(
			&tag.ID,
			&tag.OrganizationID,
			&tag.Key,
			&tag.Value,
			&tag.Category,
			&tag.Description,
			&tag.Color,
			&tag.CreatedAt,
			&tag.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	// Always return empty slice, not nil
	if tags == nil {
		tags = []*domain.Tag{}
	}

	return tags, nil
}

// GetAgentsByTag retrieves agent IDs that have a specific tag
func (r *TagRepository) GetAgentsByTag(tagID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_id FROM agent_tags WHERE tag_id = $1`

	rows, err := r.db.Query(query, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agentIDs []uuid.UUID
	for rows.Next() {
		var agentID uuid.UUID
		if err := rows.Scan(&agentID); err != nil {
			return nil, err
		}
		agentIDs = append(agentIDs, agentID)
	}

	return agentIDs, nil
}

// AddTagsToMCPServer adds tags to an MCP server
func (r *TagRepository) AddTagsToMCPServer(mcpServerID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO mcp_server_tags (mcp_server_id, tag_id, applied_at, applied_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (mcp_server_id, tag_id) DO NOTHING
	`

	now := time.Now()
	for _, tagID := range tagIDs {
		_, err := tx.Exec(query, mcpServerID, tagID, now, appliedBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (r *TagRepository) RemoveTagFromMCPServer(mcpServerID uuid.UUID, tagID uuid.UUID) error {
	query := `DELETE FROM mcp_server_tags WHERE mcp_server_id = $1 AND tag_id = $2`
	_, err := r.db.Exec(query, mcpServerID, tagID)
	return err
}

// GetMCPServerTags retrieves all tags for an MCP server
func (r *TagRepository) GetMCPServerTags(mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	query := `
		SELECT t.id, t.organization_id, t.key, t.value, t.category, t.description, t.color, t.created_at, t.created_by
		FROM tags t
		INNER JOIN mcp_server_tags mst ON t.id = mst.tag_id
		WHERE mst.mcp_server_id = $1
		ORDER BY t.category, t.key, t.value
	`

	rows, err := r.db.Query(query, mcpServerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.Tag
	for rows.Next() {
		tag := &domain.Tag{}
		err := rows.Scan(
			&tag.ID,
			&tag.OrganizationID,
			&tag.Key,
			&tag.Value,
			&tag.Category,
			&tag.Description,
			&tag.Color,
			&tag.CreatedAt,
			&tag.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	// Always return empty slice, not nil
	if tags == nil {
		tags = []*domain.Tag{}
	}

	return tags, nil
}

// GetMCPServersByTag retrieves MCP server IDs that have a specific tag
func (r *TagRepository) GetMCPServersByTag(tagID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT mcp_server_id FROM mcp_server_tags WHERE tag_id = $1`

	rows, err := r.db.Query(query, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mcpServerIDs []uuid.UUID
	for rows.Next() {
		var mcpServerID uuid.UUID
		if err := rows.Scan(&mcpServerID); err != nil {
			return nil, err
		}
		mcpServerIDs = append(mcpServerIDs, mcpServerID)
	}

	return mcpServerIDs, nil
}
```

### Phase 3: Application Layer (Day 4)

**File**: `apps/backend/internal/application/tag_service.go`

```go
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type TagService struct {
	tagRepo domain.TagRepository
}

func NewTagService(tagRepo domain.TagRepository) *TagService {
	return &TagService{
		tagRepo: tagRepo,
	}
}

// CreateTag creates a new tag
func (s *TagService) CreateTag(ctx context.Context, orgID uuid.UUID, key, value, category, description, color string, createdBy uuid.UUID) (*domain.Tag, error) {
	// Validate category
	validCategories := map[string]bool{
		"resource_type":      true,
		"environment":        true,
		"agent_type":         true,
		"data_classification": true,
		"custom":             true,
	}

	if !validCategories[category] {
		return nil, fmt.Errorf("invalid category: %s", category)
	}

	// Normalize key and value
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.ToLower(strings.TrimSpace(value))

	tag := &domain.Tag{
		OrganizationID: orgID,
		Key:            key,
		Value:          value,
		Category:       domain.TagCategory(category),
		Description:    description,
		Color:          color,
		CreatedBy:      createdBy,
	}

	err := s.tagRepo.Create(tag)
	return tag, err
}

// GetOrganizationTags retrieves all tags for an organization
func (s *TagService) GetOrganizationTags(ctx context.Context, orgID uuid.UUID) ([]*domain.Tag, error) {
	return s.tagRepo.GetByOrganization(orgID)
}

// GetTagsByCategory retrieves tags by category
func (s *TagService) GetTagsByCategory(ctx context.Context, orgID uuid.UUID, category string) ([]*domain.Tag, error) {
	return s.tagRepo.GetByCategory(orgID, domain.TagCategory(category))
}

// DeleteTag deletes a tag
func (s *TagService) DeleteTag(ctx context.Context, tagID uuid.UUID) error {
	return s.tagRepo.Delete(tagID)
}

// AddTagsToAgent adds tags to an agent
func (s *TagService) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	return s.tagRepo.AddTagsToAgent(agentID, tagIDs, appliedBy)
}

// RemoveTagFromAgent removes a tag from an agent
func (s *TagService) RemoveTagFromAgent(ctx context.Context, agentID uuid.UUID, tagID uuid.UUID) error {
	return s.tagRepo.RemoveTagFromAgent(agentID, tagID)
}

// GetAgentTags retrieves all tags for an agent
func (s *TagService) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	return s.tagRepo.GetAgentTags(agentID)
}

// AddTagsToMCPServer adds tags to an MCP server
func (s *TagService) AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID, appliedBy uuid.UUID) error {
	return s.tagRepo.AddTagsToMCPServer(mcpServerID, tagIDs, appliedBy)
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (s *TagService) RemoveTagFromMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagID uuid.UUID) error {
	return s.tagRepo.RemoveTagFromMCPServer(mcpServerID, tagID)
}

// GetMCPServerTags retrieves all tags for an MCP server
func (s *TagService) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	return s.tagRepo.GetMCPServerTags(mcpServerID)
}

// SuggestTagsFromCapabilities suggests tags based on agent/MCP capabilities
func (s *TagService) SuggestTagsFromCapabilities(ctx context.Context, orgID uuid.UUID, capabilities []string) ([]*domain.Tag, error) {
	suggestedTagValues := detectResourceType(capabilities)

	var suggestions []*domain.Tag
	for _, value := range suggestedTagValues {
		// Find existing tag with this value
		tags, err := s.tagRepo.GetByCategory(orgID, domain.TagCategoryResourceType)
		if err != nil {
			continue
		}

		for _, tag := range tags {
			if tag.Value == value {
				suggestions = append(suggestions, tag)
				break
			}
		}
	}

	return suggestions, nil
}

// Helper function to detect resource type from capabilities
func detectResourceType(capabilities []string) []string {
	detectionRules := map[string][]string{
		"filesystem": {"read_file", "write_file", "list_directory", "delete_file", "file", "directory"},
		"database":   {"query", "execute_sql", "transaction", "connection", "database", "sql"},
		"api":        {"http_request", "rest_call", "graphql_query", "api", "http"},
		"cloud":      {"aws_", "azure_", "gcp_", "s3_", "lambda_"},
		"security":   {"encrypt", "decrypt", "sign", "verify", "vault", "secret"},
	}

	detectedTags := make(map[string]bool)

	for tagValue, keywords := range detectionRules {
		for _, capability := range capabilities {
			capLower := strings.ToLower(capability)
			for _, keyword := range keywords {
				if strings.Contains(capLower, keyword) {
					detectedTags[tagValue] = true
					break
				}
			}
		}
	}

	// Convert map to slice
	var result []string
	for tag := range detectedTags {
		result = append(result, tag)
	}

	return result
}
```

### Phase 4: HTTP Handlers (Day 5-6)

**File**: `apps/backend/internal/interfaces/http/handlers/tag_handler.go`

```go
package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/interfaces/http/middleware"
)

type TagHandler struct {
	tagService *application.TagService
}

func NewTagHandler(tagService *application.TagService) *TagHandler {
	return &TagHandler{
		tagService: tagService,
	}
}

// RegisterRoutes registers tag routes
func (h *TagHandler) RegisterRoutes(app *fiber.App) {
	tags := app.Group("/api/v1/tags")
	tags.Use(middleware.AuthRequired())

	// Tag CRUD
	tags.Post("/", h.CreateTag)
	tags.Get("/", h.ListTags)
	tags.Get("/:id", h.GetTag)
	tags.Delete("/:id", h.DeleteTag)

	// Agent tag operations
	tags.Post("/agents/:agentId", h.AddTagsToAgent)
	tags.Delete("/agents/:agentId/:tagId", h.RemoveTagFromAgent)
	tags.Get("/agents/:agentId", h.GetAgentTags)

	// MCP server tag operations
	tags.Post("/mcp-servers/:mcpId", h.AddTagsToMCPServer)
	tags.Delete("/mcp-servers/:mcpId/:tagId", h.RemoveTagFromMCPServer)
	tags.Get("/mcp-servers/:mcpId", h.GetMCPServerTags)

	// Suggestions
	tags.Post("/suggest", h.SuggestTags)
}

// CreateTag creates a new tag
func (h *TagHandler) CreateTag(c fiber.Ctx) error {
	var req struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get org ID from context
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	tag, err := h.tagService.CreateTag(c.Context(), orgID, req.Key, req.Value, req.Category, req.Description, req.Color, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(tag)
}

// ListTags lists all tags for an organization
func (h *TagHandler) ListTags(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	category := c.Query("category")
	var tags interface{}
	var err error

	if category != "" {
		tags, err = h.tagService.GetTagsByCategory(c.Context(), orgID, category)
	} else {
		tags, err = h.tagService.GetOrganizationTags(c.Context(), orgID)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tags)
}

// GetTag gets a tag by ID
func (h *TagHandler) GetTag(c fiber.Ctx) error {
	// Implementation similar to above
	return c.SendStatus(fiber.StatusNotImplemented)
}

// DeleteTag deletes a tag
func (h *TagHandler) DeleteTag(c fiber.Ctx) error {
	tagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid tag ID",
		})
	}

	if err := h.tagService.DeleteTag(c.Context(), tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// AddTagsToAgent adds tags to an agent
func (h *TagHandler) AddTagsToAgent(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("agentId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		TagIDs []string `json:"tag_ids"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse tag IDs
	tagIDs := make([]uuid.UUID, len(req.TagIDs))
	for i, idStr := range req.TagIDs {
		tagID, err := uuid.Parse(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid tag ID format",
			})
		}
		tagIDs[i] = tagID
	}

	userID := c.Locals("user_id").(uuid.UUID)

	if err := h.tagService.AddTagsToAgent(c.Context(), agentID, tagIDs, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveTagFromAgent removes a tag from an agent
func (h *TagHandler) RemoveTagFromAgent(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("agentId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	tagID, err := uuid.Parse(c.Params("tagId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid tag ID",
		})
	}

	if err := h.tagService.RemoveTagFromAgent(c.Context(), agentID, tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetAgentTags gets all tags for an agent
func (h *TagHandler) GetAgentTags(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("agentId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	tags, err := h.tagService.GetAgentTags(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tags)
}

// AddTagsToMCPServer adds tags to an MCP server
func (h *TagHandler) AddTagsToMCPServer(c fiber.Ctx) error {
	mcpID, err := uuid.Parse(c.Params("mcpId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	var req struct {
		TagIDs []string `json:"tag_ids"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse tag IDs
	tagIDs := make([]uuid.UUID, len(req.TagIDs))
	for i, idStr := range req.TagIDs {
		tagID, err := uuid.Parse(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid tag ID format",
			})
		}
		tagIDs[i] = tagID
	}

	userID := c.Locals("user_id").(uuid.UUID)

	if err := h.tagService.AddTagsToMCPServer(c.Context(), mcpID, tagIDs, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (h *TagHandler) RemoveTagFromMCPServer(c fiber.Ctx) error {
	mcpID, err := uuid.Parse(c.Params("mcpId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	tagID, err := uuid.Parse(c.Params("tagId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid tag ID",
		})
	}

	if err := h.tagService.RemoveTagFromMCPServer(c.Context(), mcpID, tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetMCPServerTags gets all tags for an MCP server
func (h *TagHandler) GetMCPServerTags(c fiber.Ctx) error {
	mcpID, err := uuid.Parse(c.Params("mcpId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	tags, err := h.tagService.GetMCPServerTags(c.Context(), mcpID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tags)
}

// SuggestTags suggests tags based on capabilities
func (h *TagHandler) SuggestTags(c fiber.Ctx) error {
	var req struct {
		Capabilities []string `json:"capabilities"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	orgID := c.Locals("organization_id").(uuid.UUID)

	suggestions, err := h.tagService.SuggestTagsFromCapabilities(c.Context(), orgID, req.Capabilities)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(suggestions)
}
```

---

## 🎨 Frontend Implementation

### Phase 5: TypeScript Types (Day 7)

**File**: `apps/web/lib/api.ts` (add Tag interface)

```typescript
export type TagCategory =
  | 'resource_type'
  | 'environment'
  | 'agent_type'
  | 'data_classification'
  | 'custom';

export interface Tag {
  id: string;
  organization_id: string;
  key: string;
  value: string;
  category: TagCategory;
  description: string;
  color: string; // Hex color (e.g., "#3B82F6")
  created_at: string;
  created_by: string;
}

export interface Agent {
  // ... existing fields
  tags: Tag[]; // NEW: Agent tags
}

export interface MCPServer {
  // ... existing fields
  tags: Tag[]; // NEW: MCP server tags
}

// Tag API functions
export async function getTags(category?: TagCategory): Promise<Tag[]> {
  const url = category
    ? `/api/v1/tags?category=${category}`
    : '/api/v1/tags';

  const response = await fetch(url, {
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch tags');
  }

  return response.json();
}

export async function createTag(data: {
  key: string;
  value: string;
  category: TagCategory;
  description: string;
  color: string;
}): Promise<Tag> {
  const response = await fetch('/api/v1/tags', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw new Error('Failed to create tag');
  }

  return response.json();
}

export async function addTagsToAgent(agentId: string, tagIds: string[]): Promise<void> {
  const response = await fetch(`/api/v1/tags/agents/${agentId}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
    body: JSON.stringify({ tag_ids: tagIds }),
  });

  if (!response.ok) {
    throw new Error('Failed to add tags to agent');
  }
}

export async function removeTagFromAgent(agentId: string, tagId: string): Promise<void> {
  const response = await fetch(`/api/v1/tags/agents/${agentId}/${tagId}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
  });

  if (!response.ok) {
    throw new Error('Failed to remove tag from agent');
  }
}

export async function addTagsToMCPServer(mcpId: string, tagIds: string[]): Promise<void> {
  const response = await fetch(`/api/v1/tags/mcp-servers/${mcpId}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
    body: JSON.stringify({ tag_ids: tagIds }),
  });

  if (!response.ok) {
    throw new Error('Failed to add tags to MCP server');
  }
}

export async function removeTagFromMCPServer(mcpId: string, tagId: string): Promise<void> {
  const response = await fetch(`/api/v1/tags/mcp-servers/${mcpId}/${tagId}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
  });

  if (!response.ok) {
    throw new Error('Failed to remove tag from MCP server');
  }
}

export async function suggestTags(capabilities: string[]): Promise<Tag[]> {
  const response = await fetch('/api/v1/tags/suggest', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
    },
    body: JSON.stringify({ capabilities }),
  });

  if (!response.ok) {
    throw new Error('Failed to get tag suggestions');
  }

  return response.json();
}
```

### Phase 6: Tag Components (Day 8-9)

**File**: `apps/web/components/tag-chip.tsx`

```typescript
'use client';

import { X } from 'lucide-react';
import { Tag } from '@/lib/api';

interface TagChipProps {
  tag: Tag;
  onRemove?: () => void;
  size?: 'sm' | 'md' | 'lg';
}

export function TagChip({ tag, onRemove, size = 'md' }: TagChipProps) {
  const sizeClasses = {
    sm: 'px-2 py-0.5 text-xs',
    md: 'px-3 py-1 text-sm',
    lg: 'px-4 py-1.5 text-base',
  };

  return (
    <span
      className={`inline-flex items-center rounded-full font-medium ${sizeClasses[size]}`}
      style={{
        backgroundColor: `${tag.color}20`, // 20% opacity
        color: tag.color,
        border: `1px solid ${tag.color}40`,
      }}
    >
      {tag.value}
      {onRemove && (
        <button
          onClick={onRemove}
          className="ml-1 hover:bg-black/10 rounded-full p-0.5 transition-colors"
          aria-label={`Remove ${tag.value} tag`}
        >
          <X className="h-3 w-3" />
        </button>
      )}
    </span>
  );
}
```

**File**: `apps/web/components/tag-selector.tsx`

```typescript
'use client';

import { useState, useEffect } from 'react';
import { Tag, TagCategory, getTags } from '@/lib/api';
import { TagChip } from './tag-chip';
import { Plus } from 'lucide-react';

interface TagSelectorProps {
  selectedTags: Tag[];
  onTagsChange: (tags: Tag[]) => void;
  maxTags?: number; // Community limit: 3
  suggestedTags?: Tag[];
}

export function TagSelector({
  selectedTags,
  onTagsChange,
  maxTags = 3,
  suggestedTags = [],
}: TagSelectorProps) {
  const [availableTags, setAvailableTags] = useState<Tag[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [selectedCategory, setSelectedCategory] = useState<TagCategory | 'all'>('all');

  useEffect(() => {
    loadTags();
  }, [selectedCategory]);

  const loadTags = async () => {
    try {
      const tags = selectedCategory === 'all'
        ? await getTags()
        : await getTags(selectedCategory);
      setAvailableTags(tags);
    } catch (error) {
      console.error('Failed to load tags:', error);
    }
  };

  const handleAddTag = (tag: Tag) => {
    if (selectedTags.length >= maxTags) {
      alert(`Community Edition limited to ${maxTags} tags. Upgrade to Pro for unlimited tags.`);
      return;
    }

    if (!selectedTags.find(t => t.id === tag.id)) {
      onTagsChange([...selectedTags, tag]);
    }
    setIsOpen(false);
  };

  const handleRemoveTag = (tagId: string) => {
    onTagsChange(selectedTags.filter(t => t.id !== tagId));
  };

  const categories: Array<{ value: TagCategory | 'all'; label: string }> = [
    { value: 'all', label: 'All Categories' },
    { value: 'resource_type', label: 'Resource Type' },
    { value: 'environment', label: 'Environment' },
    { value: 'agent_type', label: 'Agent Type' },
    { value: 'data_classification', label: 'Data Classification' },
    { value: 'custom', label: 'Custom' },
  ];

  return (
    <div className="space-y-3">
      {/* Selected Tags */}
      <div className="flex flex-wrap gap-2">
        {selectedTags.map(tag => (
          <TagChip
            key={tag.id}
            tag={tag}
            onRemove={() => handleRemoveTag(tag.id)}
          />
        ))}

        {/* Add Tag Button */}
        {selectedTags.length < maxTags && (
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="inline-flex items-center gap-1 px-3 py-1 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-full hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          >
            <Plus className="h-3 w-3" />
            Add Tag
          </button>
        )}
      </div>

      {/* Tag Limit Warning */}
      {selectedTags.length >= maxTags && (
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Community Edition limit reached ({maxTags} tags).{' '}
          <a href="/pricing" className="text-blue-600 dark:text-blue-400 underline">
            Upgrade to Pro
          </a>{' '}
          for unlimited tags.
        </p>
      )}

      {/* Suggested Tags */}
      {suggestedTags.length > 0 && selectedTags.length < maxTags && (
        <div className="space-y-2">
          <p className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Suggested based on capabilities:
          </p>
          <div className="flex flex-wrap gap-2">
            {suggestedTags
              .filter(tag => !selectedTags.find(t => t.id === tag.id))
              .map(tag => (
                <button
                  key={tag.id}
                  onClick={() => handleAddTag(tag)}
                  className="inline-flex items-center gap-1"
                >
                  <TagChip tag={tag} />
                  <Plus className="h-3 w-3 text-gray-500" />
                </button>
              ))}
          </div>
        </div>
      )}

      {/* Tag Picker Dropdown */}
      {isOpen && (
        <div className="border border-gray-300 dark:border-gray-600 rounded-lg p-4 bg-white dark:bg-gray-800 shadow-lg">
          {/* Category Filter */}
          <div className="mb-3">
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Category
            </label>
            <select
              value={selectedCategory}
              onChange={(e) => setSelectedCategory(e.target.value as TagCategory | 'all')}
              className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            >
              {categories.map(cat => (
                <option key={cat.value} value={cat.value}>
                  {cat.label}
                </option>
              ))}
            </select>
          </div>

          {/* Available Tags */}
          <div className="max-h-48 overflow-y-auto space-y-2">
            {availableTags
              .filter(tag => !selectedTags.find(t => t.id === tag.id))
              .map(tag => (
                <button
                  key={tag.id}
                  onClick={() => handleAddTag(tag)}
                  className="w-full text-left px-3 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors flex items-center justify-between"
                >
                  <div>
                    <TagChip tag={tag} />
                    {tag.description && (
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        {tag.description}
                      </p>
                    )}
                  </div>
                </button>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
```

### Phase 7: Update Existing Modals (Day 10)

**File**: `apps/web/components/modals/agent-detail-modal.tsx` (update to show tags)

```typescript
// Add after Capabilities section:

{/* Tags */}
<div>
  <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
    Tags
  </h3>
  {agent.tags && agent.tags.length > 0 ? (
    <div className="flex flex-wrap gap-2">
      {agent.tags.map((tag) => (
        <TagChip key={tag.id} tag={tag} />
      ))}
    </div>
  ) : (
    <p className="text-sm text-gray-500 dark:text-gray-400 italic">
      No tags applied
    </p>
  )}
</div>
```

Similar update for `mcp-detail-modal.tsx`.

---

## 📋 Implementation Checklist

### Week 1: Backend Foundation
- [ ] **Day 1**: Database migration (022)
  - [ ] Create tags table
  - [ ] Create agent_tags table
  - [ ] Create mcp_server_tags table
  - [ ] Create tag limit trigger
  - [ ] Apply migration
  - [ ] Test with sample data

- [ ] **Day 2**: Domain models
  - [ ] Create tag.go domain model
  - [ ] Update agent.go (add tags field)
  - [ ] Update mcp_server.go (add tags field)
  - [ ] Write unit tests

- [ ] **Day 3**: Repository layer
  - [ ] Implement tag_repository.go
  - [ ] All CRUD operations
  - [ ] Agent tag operations
  - [ ] MCP server tag operations
  - [ ] Write integration tests

- [ ] **Day 4**: Application layer
  - [ ] Implement tag_service.go
  - [ ] Tag suggestion logic
  - [ ] Validation logic
  - [ ] Write unit tests

- [ ] **Day 5-6**: HTTP handlers
  - [ ] Implement tag_handler.go
  - [ ] All API endpoints (12 total)
  - [ ] Update main.go to register routes
  - [ ] Test all endpoints with Postman/curl

### Week 2: Frontend & Integration
- [ ] **Day 7**: TypeScript types
  - [ ] Add Tag interface to api.ts
  - [ ] Add tags field to Agent
  - [ ] Add tags field to MCPServer
  - [ ] Implement API functions

- [ ] **Day 8-9**: UI components
  - [ ] Create TagChip component
  - [ ] Create TagSelector component
  - [ ] Test components in Storybook

- [ ] **Day 10**: Update existing modals
  - [ ] Update agent-detail-modal.tsx
  - [ ] Update mcp-detail-modal.tsx
  - [ ] Update agent registration form
  - [ ] Update MCP registration form

- [ ] **Day 11**: Dashboard filtering
  - [ ] Add tag filter to agents dashboard
  - [ ] Add tag filter to MCP servers dashboard
  - [ ] Test filtering works correctly

- [ ] **Day 12**: Testing & Polish
  - [ ] E2E tests with Chrome DevTools MCP
  - [ ] Test 3-tag limit enforcement
  - [ ] Test tag suggestions
  - [ ] Fix any bugs
  - [ ] Update documentation

---

## 🧪 Testing Plan

### Unit Tests
```go
// apps/backend/internal/application/tag_service_test.go
func TestSuggestTagsFromCapabilities(t *testing.T) {
    capabilities := []string{"read_file", "write_file", "list_directory"}
    suggestions := detectResourceType(capabilities)

    assert.Contains(t, suggestions, "filesystem")
    assert.NotContains(t, suggestions, "database")
}
```

### Integration Tests
```go
// apps/backend/tests/tag_integration_test.go
func TestTagCommunityLimit(t *testing.T) {
    // Create 3 tags
    tag1 := createTag(t, "environment", "production")
    tag2 := createTag(t, "resource_type", "filesystem")
    tag3 := createTag(t, "agent_type", "autonomous")

    // Add all 3 to agent
    addTagsToAgent(t, agentID, []uuid.UUID{tag1.ID, tag2.ID, tag3.ID})

    // Try to add 4th tag - should fail
    tag4 := createTag(t, "custom", "test")
    err := addTagsToAgent(t, agentID, []uuid.UUID{tag4.ID})

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "limited to 3 tags")
}
```

### E2E Tests (Chrome DevTools MCP)
```typescript
// Test agent registration with tags
1. Navigate to /agents/new
2. Fill in agent details
3. Click "Add Tag"
4. Select "production" tag
5. Add 2 more tags
6. Try to add 4th tag - verify error message
7. Submit form
8. Verify agent created with 3 tags
```

---

## 📊 Success Metrics

### Technical Metrics
- ✅ All 12 API endpoints working
- ✅ 100% test coverage (unit + integration)
- ✅ <100ms API response time
- ✅ 3-tag limit enforced correctly
- ✅ Tag suggestions ≥80% accurate

### User Experience Metrics
- ✅ Tag selection takes <10 seconds
- ✅ Tag filtering works instantly
- ✅ UI shows tag limit clearly
- ✅ Zero confusion about tag categories

### Business Metrics
- ✅ Community users use average 2.5 tags per asset
- ✅ 15% of Community users hit 3-tag limit (upgrade opportunity)
- ✅ Tag filtering used in 40%+ of dashboard sessions

---

## 🚀 Future Enhancements (Pro/Enterprise)

After Community MVP is stable, build Premium features:

1. **Pro Tier** ($49/month)
   - Unlimited tags
   - Tag compliance reports
   - Tag analytics

2. **Enterprise Tier** ($5K/month)
   - Required tag policies
   - Tag-based RBAC
   - Bulk tag operations
   - Tag hierarchies
   - Custom validation rules

---

**Built by**: Claude Sonnet 4.5
**Timeline**: 2 weeks (80 hours)
**Target**: Community Edition MVP
**License**: Apache 2.0
**Project**: OpenA2A Agent Identity Management
