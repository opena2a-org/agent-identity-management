package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// MCPTrustScoreRepository implements domain.MCPTrustScoreRepository
type MCPTrustScoreRepository struct {
	db *sql.DB
}

// NewMCPTrustScoreRepository creates a new MCP trust score repository
func NewMCPTrustScoreRepository(db *sql.DB) *MCPTrustScoreRepository {
	return &MCPTrustScoreRepository{db: db}
}

// Create stores a new MCP trust score
func (r *MCPTrustScoreRepository) Create(score *domain.MCPTrustScore) error {
	query := `
		INSERT INTO mcp_trust_scores (
			id, mcp_server_id, score, factors, confidence, last_calculated, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	// Marshal factors to JSON
	factorsJSON, err := json.Marshal(score.Factors)
	if err != nil {
		return fmt.Errorf("failed to marshal factors: %w", err)
	}

	err = r.db.QueryRow(
		query,
		score.ID,
		score.MCPServerID,
		score.Score,
		factorsJSON,
		score.Confidence,
		score.LastCalculated,
		time.Now().UTC(),
	).Scan(&score.ID, &score.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create MCP trust score: %w", err)
	}

	return nil
}

// GetByMCPServer retrieves trust score by MCP server ID
func (r *MCPTrustScoreRepository) GetByMCPServer(mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	return r.GetLatest(mcpServerID)
}

// GetLatest retrieves the latest trust score for an MCP server
func (r *MCPTrustScoreRepository) GetLatest(mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	query := `
		SELECT id, mcp_server_id, score, factors, confidence, last_calculated, created_at
		FROM mcp_trust_scores
		WHERE mcp_server_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	score := &domain.MCPTrustScore{}
	var factorsJSON []byte

	err := r.db.QueryRow(query, mcpServerID).Scan(
		&score.ID,
		&score.MCPServerID,
		&score.Score,
		&factorsJSON,
		&score.Confidence,
		&score.LastCalculated,
		&score.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("MCP trust score not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP trust score: %w", err)
	}

	// Unmarshal factors
	if err := json.Unmarshal(factorsJSON, &score.Factors); err != nil {
		return nil, fmt.Errorf("failed to unmarshal factors: %w", err)
	}

	return score, nil
}

// GetHistory retrieves trust score history for an MCP server
func (r *MCPTrustScoreRepository) GetHistory(mcpServerID uuid.UUID, limit int) ([]*domain.MCPTrustScore, error) {
	query := `
		SELECT id, mcp_server_id, score, factors, confidence, last_calculated, created_at
		FROM mcp_trust_scores
		WHERE mcp_server_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, mcpServerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP trust score history: %w", err)
	}
	defer rows.Close()

	var scores []*domain.MCPTrustScore
	for rows.Next() {
		score := &domain.MCPTrustScore{}
		var factorsJSON []byte

		err := rows.Scan(
			&score.ID,
			&score.MCPServerID,
			&score.Score,
			&factorsJSON,
			&score.Confidence,
			&score.LastCalculated,
			&score.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan MCP trust score: %w", err)
		}

		// Unmarshal factors
		if err := json.Unmarshal(factorsJSON, &score.Factors); err != nil {
			return nil, fmt.Errorf("failed to unmarshal factors: %w", err)
		}

		scores = append(scores, score)
	}

	return scores, nil
}

// GetHistoryAuditTrail returns audit trail entries for MCP trust score changes
func (r *MCPTrustScoreRepository) GetHistoryAuditTrail(mcpServerID uuid.UUID, limit int) ([]*domain.MCPTrustScoreHistoryEntry, error) {
	query := `
		SELECT
			id, mcp_server_id, organization_id, trust_score, previous_score,
			change_reason, changed_by, recorded_at, created_at
		FROM mcp_trust_score_history
		WHERE mcp_server_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, mcpServerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP trust score history audit trail: %w", err)
	}
	defer rows.Close()

	var entries []*domain.MCPTrustScoreHistoryEntry
	for rows.Next() {
		entry := &domain.MCPTrustScoreHistoryEntry{}
		var previousScore sql.NullFloat64
		var changedBy sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.MCPServerID,
			&entry.OrganizationID,
			&entry.TrustScore,
			&previousScore,
			&entry.ChangeReason,
			&changedBy,
			&entry.RecordedAt,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan MCP trust score history entry: %w", err)
		}

		// Convert nullable fields
		if previousScore.Valid {
			entry.PreviousScore = &previousScore.Float64
		}
		if changedBy.Valid {
			userID, _ := uuid.Parse(changedBy.String)
			entry.ChangedBy = &userID
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
