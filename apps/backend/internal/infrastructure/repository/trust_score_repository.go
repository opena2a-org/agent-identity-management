package repository

import (
	"database/sql"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"time"
)

type TrustScoreRepository struct {
	db *sql.DB
}

func NewTrustScoreRepository(db *sql.DB) *TrustScoreRepository {
	return &TrustScoreRepository{db: db}
}

func (r *TrustScoreRepository) Create(score *domain.TrustScore) error {
	// 8-factor trust scoring system
	query := `
		INSERT INTO trust_scores (
			id, agent_id, score,
			verification_status, uptime, success_rate, security_alerts,
			compliance, age, drift_detection, user_feedback,
			confidence, last_calculated, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	if score.ID == uuid.Nil {
		score.ID = uuid.New()
	}
	if score.CreatedAt.IsZero() {
		score.CreatedAt = time.Now()
	}
	if score.LastCalculated.IsZero() {
		score.LastCalculated = time.Now()
	}

	_, err := r.db.Exec(query,
		score.ID,
		score.AgentID,
		score.Score,
		score.Factors.VerificationStatus,
		score.Factors.Uptime,
		score.Factors.SuccessRate,
		score.Factors.SecurityAlerts,
		score.Factors.Compliance,
		score.Factors.Age,
		score.Factors.DriftDetection,
		score.Factors.UserFeedback,
		score.Confidence,
		score.LastCalculated,
		score.CreatedAt,
	)
	return err
}

func (r *TrustScoreRepository) GetByAgent(agentID uuid.UUID) (*domain.TrustScore, error) {
	return r.GetLatest(agentID)
}

func (r *TrustScoreRepository) GetLatest(agentID uuid.UUID) (*domain.TrustScore, error) {
	// 8-factor trust scoring system
	query := `
		SELECT
			id, agent_id, score,
			verification_status, uptime, success_rate, security_alerts,
			compliance, age, drift_detection, user_feedback,
			confidence, last_calculated, created_at
		FROM trust_scores
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	score := &domain.TrustScore{}
	err := r.db.QueryRow(query, agentID).Scan(
		&score.ID,
		&score.AgentID,
		&score.Score,
		&score.Factors.VerificationStatus,
		&score.Factors.Uptime,
		&score.Factors.SuccessRate,
		&score.Factors.SecurityAlerts,
		&score.Factors.Compliance,
		&score.Factors.Age,
		&score.Factors.DriftDetection,
		&score.Factors.UserFeedback,
		&score.Confidence,
		&score.LastCalculated,
		&score.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return score, err
}

func (r *TrustScoreRepository) GetHistory(agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	// 8-factor trust scoring system
	query := `
		SELECT
			id, agent_id, score,
			verification_status, uptime, success_rate, security_alerts,
			compliance, age, drift_detection, user_feedback,
			confidence, last_calculated, created_at
		FROM trust_scores
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []*domain.TrustScore
	for rows.Next() {
		score := &domain.TrustScore{}
		err := rows.Scan(
			&score.ID,
			&score.AgentID,
			&score.Score,
			&score.Factors.VerificationStatus,
			&score.Factors.Uptime,
			&score.Factors.SuccessRate,
			&score.Factors.SecurityAlerts,
			&score.Factors.Compliance,
			&score.Factors.Age,
			&score.Factors.DriftDetection,
			&score.Factors.UserFeedback,
			&score.Confidence,
			&score.LastCalculated,
			&score.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		scores = append(scores, score)
	}
	return scores, nil
}

// GetHistoryAuditTrail returns trust score audit trail from trust_score_history table
// This provides the full audit trail with who changed it and why (for frontend UI)
func (r *TrustScoreRepository) GetHistoryAuditTrail(agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	query := `
		SELECT
			id, agent_id, organization_id, trust_score, previous_score,
			change_reason, changed_by, recorded_at, created_at
		FROM trust_score_history
		WHERE agent_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.TrustScoreHistoryEntry
	for rows.Next() {
		entry := &domain.TrustScoreHistoryEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.AgentID,
			&entry.OrganizationID,
			&entry.TrustScore,
			&entry.PreviousScore,
			&entry.ChangeReason,
			&entry.ChangedBy,
			&entry.RecordedAt,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
