package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// TagRepository implements domain.TagRepository
type TagRepository struct {
	db *sql.DB
}

// NewTagRepository creates a new tag repository
func NewTagRepository(db *sql.DB) *TagRepository {
	return &TagRepository{db: db}
}

// Create creates a new tag
func (r *TagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	query := `
		INSERT INTO tags (organization_id, key, value, category, description, color, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(
		ctx,
		query,
		tag.OrganizationID,
		tag.Key,
		tag.Value,
		tag.Category,
		tag.Description,
		tag.Color,
		tag.CreatedBy,
	).Scan(&tag.ID, &tag.CreatedAt)
}

// GetByID retrieves a tag by ID
func (r *TagRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	query := `
		SELECT id, organization_id, key, value, category, description, color, created_at, created_by
		FROM tags
		WHERE id = $1
	`

	tag := &domain.Tag{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
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
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}

	return tag, nil
}

// Update updates an existing tag
func (r *TagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	query := `
		UPDATE tags
		SET key = $1, value = $2, category = $3, description = $4, color = $5
		WHERE id = $6
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		tag.Key,
		tag.Value,
		tag.Category,
		tag.Description,
		tag.Color,
		tag.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}

	return nil
}

// List retrieves all tags for an organization, optionally filtered by category
func (r *TagRepository) List(ctx context.Context, organizationID uuid.UUID, category *domain.TagCategory) ([]*domain.Tag, error) {
	var query string
	var args []interface{}

	if category != nil {
		query = `
			SELECT id, organization_id, key, value, category, description, color, created_at, created_by
			FROM tags
			WHERE organization_id = $1 AND category = $2
			ORDER BY category, key, value
		`
		args = []interface{}{organizationID, *category}
	} else {
		query = `
			SELECT id, organization_id, key, value, category, description, color, created_at, created_by
			FROM tags
			WHERE organization_id = $1
			ORDER BY category, key, value
		`
		args = []interface{}{organizationID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.Tag, 0)
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
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// Delete deletes a tag
func (r *TagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tags WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tag not found")
	}

	return nil
}

// AddTagsToAgent adds tags to an agent
func (r *TagRepository) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO agent_tags (agent_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (agent_id, tag_id) DO NOTHING
	`

	for _, tagID := range tagIDs {
		_, err := tx.ExecContext(ctx, query, agentID, tagID)
		if err != nil {
			return fmt.Errorf("failed to add tag %s to agent: %w", tagID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RemoveTagFromAgent removes a tag from an agent
func (r *TagRepository) RemoveTagFromAgent(ctx context.Context, agentID uuid.UUID, tagID uuid.UUID) error {
	query := `DELETE FROM agent_tags WHERE agent_id = $1 AND tag_id = $2`
	result, err := r.db.ExecContext(ctx, query, agentID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from agent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tag not found on agent")
	}

	return nil
}

// GetAgentTags retrieves all tags for an agent
func (r *TagRepository) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	query := `
		SELECT t.id, t.organization_id, t.key, t.value, t.category, t.description, t.color, t.created_at, t.created_by
		FROM tags t
		INNER JOIN agent_tags at ON t.id = at.tag_id
		WHERE at.agent_id = $1
		ORDER BY t.category, t.key, t.value
	`

	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.Tag, 0)
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
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// AddTagsToMCPServer adds tags to an MCP server
func (r *TagRepository) AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO mcp_server_tags (mcp_server_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (mcp_server_id, tag_id) DO NOTHING
	`

	for _, tagID := range tagIDs {
		_, err := tx.ExecContext(ctx, query, mcpServerID, tagID)
		if err != nil {
			return fmt.Errorf("failed to add tag %s to mcp server: %w", tagID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RemoveTagFromMCPServer removes a tag from an MCP server
func (r *TagRepository) RemoveTagFromMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagID uuid.UUID) error {
	query := `DELETE FROM mcp_server_tags WHERE mcp_server_id = $1 AND tag_id = $2`
	result, err := r.db.ExecContext(ctx, query, mcpServerID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from mcp server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("tag not found on mcp server")
	}

	return nil
}

// GetMCPServerTags retrieves all tags for an MCP server
func (r *TagRepository) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	query := `
		SELECT t.id, t.organization_id, t.key, t.value, t.category, t.description, t.color, t.created_at, t.created_by
		FROM tags t
		INNER JOIN mcp_server_tags mst ON t.id = mst.tag_id
		WHERE mst.mcp_server_id = $1
		ORDER BY t.category, t.key, t.value
	`

	rows, err := r.db.QueryContext(ctx, query, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.Tag, 0)
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
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetPopularTags retrieves the most popular tags by usage count
func (r *TagRepository) GetPopularTags(ctx context.Context, organizationID uuid.UUID, limit int) ([]*domain.Tag, error) {
	query := `
		SELECT t.id, t.organization_id, t.key, t.value, t.category, t.description, t.color, t.created_at, t.created_by,
		       COALESCE(agent_count, 0) + COALESCE(mcp_count, 0) as usage_count
		FROM tags t
		LEFT JOIN (
			SELECT tag_id, COUNT(*) as agent_count
			FROM agent_tags
			GROUP BY tag_id
		) at ON t.id = at.tag_id
		LEFT JOIN (
			SELECT tag_id, COUNT(*) as mcp_count
			FROM mcp_server_tags
			GROUP BY tag_id
		) mst ON t.id = mst.tag_id
		WHERE t.organization_id = $1
		ORDER BY usage_count DESC, t.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.Tag, 0)
	for rows.Next() {
		tag := &domain.Tag{}
		var usageCount int
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
			&usageCount, // We select it but don't store it in the tag struct
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// SearchTags searches for tags by query string (case-insensitive)
func (r *TagRepository) SearchTags(ctx context.Context, organizationID uuid.UUID, query string, category *domain.TagCategory) ([]*domain.Tag, error) {
	var sqlQuery string
	var args []interface{}

	if category != nil {
		sqlQuery = `
			SELECT id, organization_id, key, value, category, description, color, created_at, created_by
			FROM tags
			WHERE organization_id = $1
			  AND category = $2
			  AND (key ILIKE $3 OR value ILIKE $3 OR description ILIKE $3)
			ORDER BY key, value
		`
		searchPattern := "%" + query + "%"
		args = []interface{}{organizationID, *category, searchPattern}
	} else {
		sqlQuery = `
			SELECT id, organization_id, key, value, category, description, color, created_at, created_by
			FROM tags
			WHERE organization_id = $1
			  AND (key ILIKE $2 OR value ILIKE $2 OR description ILIKE $2)
			ORDER BY key, value
		`
		searchPattern := "%" + query + "%"
		args = []interface{}{organizationID, searchPattern}
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.Tag, 0)
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
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}
