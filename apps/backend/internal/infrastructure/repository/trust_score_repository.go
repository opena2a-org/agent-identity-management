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
	query := `
		INSERT INTO trust_scores (id, agent_id, score, verification_status, certificate_validity, repository_quality, documentation_score, community_trust, security_audit, update_frequency, age_score, confidence, last_calculated, created_at)
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
		score.Factors.CertificateValidity,
		score.Factors.RepositoryQuality,
		score.Factors.DocumentationScore,
		score.Factors.CommunityTrust,
		score.Factors.SecurityAudit,
		score.Factors.UpdateFrequency,
		score.Factors.AgeScore,
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
	query := `
		SELECT id, agent_id, score, verification_status, certificate_validity, repository_quality, documentation_score, community_trust, security_audit, update_frequency, age_score, confidence, last_calculated, created_at
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
		&score.Factors.CertificateValidity,
		&score.Factors.RepositoryQuality,
		&score.Factors.DocumentationScore,
		&score.Factors.CommunityTrust,
		&score.Factors.SecurityAudit,
		&score.Factors.UpdateFrequency,
		&score.Factors.AgeScore,
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
	query := `
		SELECT id, agent_id, score, verification_status, certificate_validity, repository_quality, documentation_score, community_trust, security_audit, update_frequency, age_score, confidence, last_calculated, created_at
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
			&score.Factors.CertificateValidity,
			&score.Factors.RepositoryQuality,
			&score.Factors.DocumentationScore,
			&score.Factors.CommunityTrust,
			&score.Factors.SecurityAudit,
			&score.Factors.UpdateFrequency,
			&score.Factors.AgeScore,
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
