package repository

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// IsolationAttestationRepository implements domain.IsolationAttestationRepository.
// Backs trust factor 9 (execution_isolation, 10% weight). Agents self-report
// their runtime isolation posture; the score is computed by domain.ScoreIsolation
// at write time and stored on the attestation.
type IsolationAttestationRepository struct {
	db *sql.DB
}

// NewIsolationAttestationRepository creates a new isolation attestation repository
func NewIsolationAttestationRepository(db *sql.DB) *IsolationAttestationRepository {
	return &IsolationAttestationRepository{db: db}
}

func (r *IsolationAttestationRepository) Create(attestation *domain.IsolationAttestation) error {
	query := `
		INSERT INTO isolation_attestations (
			id, agent_id, sandbox, network, filesystem, process, score, reported_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(query,
		attestation.ID,
		attestation.AgentID,
		string(attestation.Sandbox),
		string(attestation.Network),
		string(attestation.Filesystem),
		string(attestation.Process),
		attestation.Score,
		attestation.ReportedAt,
		attestation.CreatedAt,
	)
	return err
}

func (r *IsolationAttestationRepository) GetLatest(agentID uuid.UUID) (*domain.IsolationAttestation, error) {
	query := `
		SELECT id, agent_id, sandbox, network, filesystem, process, score, reported_at, created_at
		FROM isolation_attestations
		WHERE agent_id = $1
		ORDER BY reported_at DESC
		LIMIT 1
	`
	row := r.db.QueryRow(query, agentID)
	att, err := scanIsolationAttestation(row)
	if err == sql.ErrNoRows {
		return nil, nil // No attestation yet; caller treats nil as "use baseline"
	}
	if err != nil {
		return nil, err
	}
	return att, nil
}

func (r *IsolationAttestationRepository) GetHistory(agentID uuid.UUID, limit int) ([]*domain.IsolationAttestation, error) {
	query := `
		SELECT id, agent_id, sandbox, network, filesystem, process, score, reported_at, created_at
		FROM isolation_attestations
		WHERE agent_id = $1
		ORDER BY reported_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attestations := make([]*domain.IsolationAttestation, 0)
	for rows.Next() {
		att, err := scanIsolationAttestation(rows)
		if err != nil {
			return nil, err
		}
		attestations = append(attestations, att)
	}
	return attestations, rows.Err()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanIsolationAttestation(s rowScanner) (*domain.IsolationAttestation, error) {
	var att domain.IsolationAttestation
	var sandbox, network, filesystem, process string
	err := s.Scan(
		&att.ID,
		&att.AgentID,
		&sandbox,
		&network,
		&filesystem,
		&process,
		&att.Score,
		&att.ReportedAt,
		&att.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	att.Sandbox = domain.SandboxType(sandbox)
	att.Network = domain.NetworkIsolation(network)
	att.Filesystem = domain.FilesystemIsolation(filesystem)
	att.Process = domain.ProcessIsolation(process)
	return &att, nil
}
