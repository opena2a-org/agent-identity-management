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

	// Always return empty slice, not nil
	if tags == nil {
		tags = []*domain.Tag{}
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

	// Always return empty slice, not nil
	if tags == nil {
		tags = []*domain.Tag{}
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
