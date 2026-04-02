package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// RemediationRepository implements domain.RemediationRepository
type RemediationRepository struct {
	db *sql.DB
}

// NewRemediationRepository creates a new remediation repository
func NewRemediationRepository(db *sql.DB) *RemediationRepository {
	return &RemediationRepository{db: db}
}

// Create persists a new remediation record
func (r *RemediationRepository) Create(record *domain.RemediationRecord) error {
	query := `
		INSERT INTO remediation_records (id, finding_id, source, source_event_id, agent_identifier,
			initial_score, initial_severity, remediation_applied, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}

	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = r.db.Exec(query,
		record.ID,
		record.FindingID,
		record.Source,
		record.SourceEventID,
		record.AgentIdentifier,
		record.InitialScore,
		record.InitialSeverity,
		record.RemediationApplied,
		record.Status,
		metadataJSON,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

// GetByID returns a remediation record by its UUID
func (r *RemediationRepository) GetByID(id uuid.UUID) (*domain.RemediationRecord, error) {
	query := `
		SELECT id, finding_id, source, source_event_id, agent_identifier,
			initial_score, initial_severity, remediation_applied, remediation_scan_id,
			rescan_score, improvement_delta, remediation_duration_hours, status, metadata,
			created_at, updated_at
		FROM remediation_records
		WHERE id = $1
	`

	record := &domain.RemediationRecord{}
	var metadataJSON []byte
	var sourceEventID, agentIdentifier, initialSeverity, remediationScanID sql.NullString
	var initialScore, rescanScore, improvementDelta sql.NullInt32
	var remediationDurationH sql.NullFloat64

	err := r.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.FindingID,
		&record.Source,
		&sourceEventID,
		&agentIdentifier,
		&initialScore,
		&initialSeverity,
		&record.RemediationApplied,
		&remediationScanID,
		&rescanScore,
		&improvementDelta,
		&remediationDurationH,
		&record.Status,
		&metadataJSON,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	record.SourceEventID = sourceEventID.String
	record.AgentIdentifier = agentIdentifier.String
	record.InitialSeverity = initialSeverity.String
	record.RemediationScanID = remediationScanID.String

	if initialScore.Valid {
		v := int(initialScore.Int32)
		record.InitialScore = &v
	}
	if rescanScore.Valid {
		v := int(rescanScore.Int32)
		record.RescanScore = &v
	}
	if improvementDelta.Valid {
		v := int(improvementDelta.Int32)
		record.ImprovementDelta = &v
	}
	if remediationDurationH.Valid {
		record.RemediationDurationH = &remediationDurationH.Float64
	}

	if metadataJSON != nil {
		_ = json.Unmarshal(metadataJSON, &record.Metadata)
	}

	return record, nil
}

// GetByFindingAndAgent returns the most recent non-remediated record for a finding+agent pair
func (r *RemediationRepository) GetByFindingAndAgent(findingID, agentIdentifier string) (*domain.RemediationRecord, error) {
	query := `
		SELECT id FROM remediation_records
		WHERE finding_id = $1 AND agent_identifier = $2 AND status != 'remediated'
		ORDER BY created_at DESC
		LIMIT 1
	`

	var id uuid.UUID
	err := r.db.QueryRow(query, findingID, agentIdentifier).Scan(&id)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// UpdateRemediation marks a record as remediated with rescan results
func (r *RemediationRepository) UpdateRemediation(id uuid.UUID, scanID string, rescanScore int) error {
	query := `
		UPDATE remediation_records
		SET remediation_applied = TRUE,
			remediation_scan_id = $2,
			rescan_score = $3,
			improvement_delta = $3 - COALESCE(initial_score, 0),
			remediation_duration_hours = EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600.0,
			status = 'remediated',
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(query, id, scanID, rescanScore)
	return err
}

// GetStats returns aggregate remediation statistics
func (r *RemediationRepository) GetStats() (*domain.RemediationStats, error) {
	stats := &domain.RemediationStats{
		BySource:   make(map[string]int),
		BySeverity: make(map[string]int),
	}

	// Status counts
	statusQuery := `
		SELECT status, COUNT(*) FROM remediation_records GROUP BY status
	`
	rows, err := r.db.Query(statusQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.TotalFindings += count
		switch domain.RemediationStatus(status) {
		case domain.RemediationStatusOpen:
			stats.Open = count
		case domain.RemediationStatusInProgress:
			stats.InProgress = count
		case domain.RemediationStatusRemediated:
			stats.Remediated = count
		case domain.RemediationStatusWontFix:
			stats.WontFix = count
		}
	}

	if stats.TotalFindings > 0 {
		stats.RemediationRate = float64(stats.Remediated) / float64(stats.TotalFindings) * 100
	}

	// Average remediation time
	avgQuery := `
		SELECT COALESCE(AVG(remediation_duration_hours), 0)
		FROM remediation_records
		WHERE status = 'remediated' AND remediation_duration_hours IS NOT NULL
	`
	_ = r.db.QueryRow(avgQuery).Scan(&stats.AvgRemediationHours)

	// By source
	sourceQuery := `SELECT source, COUNT(*) FROM remediation_records GROUP BY source`
	sourceRows, err := r.db.Query(sourceQuery)
	if err == nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var source string
			var count int
			if err := sourceRows.Scan(&source, &count); err == nil {
				stats.BySource[source] = count
			}
		}
	}

	// By severity
	sevQuery := `SELECT COALESCE(initial_severity, 'unknown'), COUNT(*) FROM remediation_records GROUP BY initial_severity`
	sevRows, err := r.db.Query(sevQuery)
	if err == nil {
		defer sevRows.Close()
		for sevRows.Next() {
			var severity string
			var count int
			if err := sevRows.Scan(&severity, &count); err == nil {
				stats.BySeverity[severity] = count
			}
		}
	}

	return stats, nil
}

// ListRecent returns the most recent remediation records
func (r *RemediationRepository) ListRecent(limit int) ([]*domain.RemediationRecord, error) {
	query := `
		SELECT id FROM remediation_records
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*domain.RemediationRecord
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		record, err := r.GetByID(id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}
