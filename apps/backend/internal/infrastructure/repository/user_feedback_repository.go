package repository

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// UserFeedbackRepository implements domain.UserFeedbackRepository
type UserFeedbackRepository struct {
	db *sql.DB
}

// NewUserFeedbackRepository creates a new user feedback repository
func NewUserFeedbackRepository(db *sql.DB) *UserFeedbackRepository {
	return &UserFeedbackRepository{db: db}
}

func (r *UserFeedbackRepository) Create(feedback *domain.UserFeedback) error {
	query := `
		INSERT INTO user_feedback (
			id, agent_id, organization_id, user_id, rating, comment, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(query,
		feedback.ID,
		feedback.AgentID,
		feedback.OrganizationID,
		feedback.UserID,
		feedback.Rating,
		feedback.Comment,
		feedback.CreatedAt,
	)
	return err
}

func (r *UserFeedbackRepository) GetByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.UserFeedback, error) {
	query := `
		SELECT id, agent_id, organization_id, user_id, rating, comment, created_at
		FROM user_feedback
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(query, agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedback := make([]*domain.UserFeedback, 0)
	for rows.Next() {
		var f domain.UserFeedback
		if err := rows.Scan(
			&f.ID,
			&f.AgentID,
			&f.OrganizationID,
			&f.UserID,
			&f.Rating,
			&f.Comment,
			&f.CreatedAt,
		); err != nil {
			return nil, err
		}
		feedback = append(feedback, &f)
	}
	return feedback, rows.Err()
}

// GetStats aggregates sentiment buckets in a single query so the trust
// calculator can apply the count-threshold formula cheaply.
func (r *UserFeedbackRepository) GetStats(agentID uuid.UUID) (*domain.UserFeedbackStats, error) {
	query := `
		SELECT
			COUNT(*)                                            AS total,
			COUNT(*) FILTER (WHERE rating >= 4)                 AS positive,
			COUNT(*) FILTER (WHERE rating = 3)                  AS neutral,
			COUNT(*) FILTER (WHERE rating <= 2)                 AS negative,
			COALESCE(AVG(rating), 0)                            AS avg_rating
		FROM user_feedback
		WHERE agent_id = $1
	`
	stats := &domain.UserFeedbackStats{}
	err := r.db.QueryRow(query, agentID).Scan(
		&stats.Total,
		&stats.PositiveCount,
		&stats.NeutralCount,
		&stats.NegativeCount,
		&stats.AverageRating,
	)
	if err != nil {
		return nil, err
	}
	return stats, nil
}
