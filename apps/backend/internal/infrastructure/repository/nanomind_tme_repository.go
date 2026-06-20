package repository

import (
	"database/sql"

	"github.com/google/uuid"
)

// NanoMindTMERepository reads the latest NanoMind Threat Model Evaluation (TME)
// score per agent from the nanomind_tme_evaluations table (migration 090).
//
// It satisfies application.NanoMindTMEProvider structurally (GetLatestTMEScore),
// so it can be wired into the trust calculator without an import cycle. The TME
// score (0-1, higher = safer) blends 30% into the security alerts factor.
type NanoMindTMERepository struct {
	db *sql.DB
}

// NewNanoMindTMERepository creates a new NanoMind TME repository
func NewNanoMindTMERepository(db *sql.DB) *NanoMindTMERepository {
	return &NanoMindTMERepository{db: db}
}

// GetLatestTMEScore returns the latest TME score for an agent (0-1).
// Returns -1 (with nil error) when no evaluation exists, signalling the trust
// calculator to leave the security alerts factor untouched.
func (r *NanoMindTMERepository) GetLatestTMEScore(agentID uuid.UUID) (float64, error) {
	query := `
		SELECT tme_score
		FROM nanomind_tme_evaluations
		WHERE agent_id = $1
		ORDER BY evaluated_at DESC
		LIMIT 1
	`
	var score float64
	err := r.db.QueryRow(query, agentID).Scan(&score)
	if err == sql.ErrNoRows {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return score, nil
}
