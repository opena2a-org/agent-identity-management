package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ComplianceEvidenceRepository implements domain.ComplianceEvidenceRepository
type ComplianceEvidenceRepository struct {
	db *sql.DB
}

// NewComplianceEvidenceRepository creates a new compliance evidence repository
func NewComplianceEvidenceRepository(db *sql.DB) *ComplianceEvidenceRepository {
	return &ComplianceEvidenceRepository{db: db}
}

func (r *ComplianceEvidenceRepository) Create(evidence *domain.ComplianceEvidence) error {
	dataJSON, err := json.Marshal(evidence.Data)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO compliance_evidence (
			id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.Exec(query,
		evidence.ID,
		evidence.OrganizationID,
		string(evidence.Framework),
		evidence.CheckName,
		string(evidence.EvidenceType),
		evidence.Title,
		evidence.Description,
		dataJSON,
		evidence.FileURL,
		evidence.CollectedAt,
		evidence.CollectedBy,
		evidence.IsAutomatic,
		evidence.ValidUntil,
		evidence.CreatedAt,
	)
	return err
}

func (r *ComplianceEvidenceRepository) GetByID(id uuid.UUID) (*domain.ComplianceEvidence, error) {
	query := `
		SELECT id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		FROM compliance_evidence
		WHERE id = $1
	`

	var evidence domain.ComplianceEvidence
	var dataJSON []byte
	var framework, evidenceType string

	err := r.db.QueryRow(query, id).Scan(
		&evidence.ID,
		&evidence.OrganizationID,
		&framework,
		&evidence.CheckName,
		&evidenceType,
		&evidence.Title,
		&evidence.Description,
		&dataJSON,
		&evidence.FileURL,
		&evidence.CollectedAt,
		&evidence.CollectedBy,
		&evidence.IsAutomatic,
		&evidence.ValidUntil,
		&evidence.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	evidence.Framework = domain.ComplianceFramework(framework)
	evidence.EvidenceType = domain.EvidenceType(evidenceType)

	if len(dataJSON) > 0 {
		json.Unmarshal(dataJSON, &evidence.Data)
	}

	return &evidence, nil
}

func (r *ComplianceEvidenceRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.ComplianceEvidence, error) {
	query := `
		SELECT id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		FROM compliance_evidence
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvidenceRows(rows)
}

func (r *ComplianceEvidenceRepository) GetByFramework(orgID uuid.UUID, framework domain.ComplianceFramework) ([]*domain.ComplianceEvidence, error) {
	query := `
		SELECT id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		FROM compliance_evidence
		WHERE organization_id = $1 AND framework = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID, string(framework))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvidenceRows(rows)
}

func (r *ComplianceEvidenceRepository) GetByCheckName(orgID uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error) {
	query := `
		SELECT id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		FROM compliance_evidence
		WHERE organization_id = $1 AND check_name = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID, checkName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvidenceRows(rows)
}

func (r *ComplianceEvidenceRepository) GetRecent(orgID uuid.UUID, since time.Time) ([]*domain.ComplianceEvidence, error) {
	query := `
		SELECT id, organization_id, framework, check_name, evidence_type,
			title, description, data, file_url, collected_at,
			collected_by, is_automatic, valid_until, created_at
		FROM compliance_evidence
		WHERE organization_id = $1 AND collected_at >= $2
		ORDER BY collected_at DESC
	`

	rows, err := r.db.Query(query, orgID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvidenceRows(rows)
}

func (r *ComplianceEvidenceRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM compliance_evidence WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *ComplianceEvidenceRepository) DeleteExpired(orgID uuid.UUID) (int, error) {
	query := `DELETE FROM compliance_evidence WHERE organization_id = $1 AND valid_until < $2`
	result, err := r.db.Exec(query, orgID, time.Now())
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (r *ComplianceEvidenceRepository) scanEvidenceRows(rows *sql.Rows) ([]*domain.ComplianceEvidence, error) {
	var evidences []*domain.ComplianceEvidence

	for rows.Next() {
		var evidence domain.ComplianceEvidence
		var dataJSON []byte
		var framework, evidenceType string

		err := rows.Scan(
			&evidence.ID,
			&evidence.OrganizationID,
			&framework,
			&evidence.CheckName,
			&evidenceType,
			&evidence.Title,
			&evidence.Description,
			&dataJSON,
			&evidence.FileURL,
			&evidence.CollectedAt,
			&evidence.CollectedBy,
			&evidence.IsAutomatic,
			&evidence.ValidUntil,
			&evidence.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		evidence.Framework = domain.ComplianceFramework(framework)
		evidence.EvidenceType = domain.EvidenceType(evidenceType)

		if len(dataJSON) > 0 {
			json.Unmarshal(dataJSON, &evidence.Data)
		}

		evidences = append(evidences, &evidence)
	}

	return evidences, rows.Err()
}

// ComplianceSnapshotRepository implements domain.ComplianceSnapshotRepository
type ComplianceSnapshotRepository struct {
	db *sql.DB
}

// NewComplianceSnapshotRepository creates a new compliance snapshot repository
func NewComplianceSnapshotRepository(db *sql.DB) *ComplianceSnapshotRepository {
	return &ComplianceSnapshotRepository{db: db}
}

func (r *ComplianceSnapshotRepository) Create(snapshot *domain.ComplianceSnapshot) error {
	checkResultsJSON, err := json.Marshal(snapshot.CheckResults)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO compliance_snapshots (
			id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.Exec(query,
		snapshot.ID,
		snapshot.OrganizationID,
		string(snapshot.Framework),
		snapshot.Score,
		snapshot.PassedChecks,
		snapshot.FailedChecks,
		snapshot.TotalChecks,
		checkResultsJSON,
		snapshot.SnapshotDate,
		snapshot.CreatedAt,
	)
	return err
}

func (r *ComplianceSnapshotRepository) GetByID(id uuid.UUID) (*domain.ComplianceSnapshot, error) {
	query := `
		SELECT id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		FROM compliance_snapshots
		WHERE id = $1
	`

	var snapshot domain.ComplianceSnapshot
	var checkResultsJSON []byte
	var framework string

	err := r.db.QueryRow(query, id).Scan(
		&snapshot.ID,
		&snapshot.OrganizationID,
		&framework,
		&snapshot.Score,
		&snapshot.PassedChecks,
		&snapshot.FailedChecks,
		&snapshot.TotalChecks,
		&checkResultsJSON,
		&snapshot.SnapshotDate,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	snapshot.Framework = domain.ComplianceFramework(framework)

	if len(checkResultsJSON) > 0 {
		json.Unmarshal(checkResultsJSON, &snapshot.CheckResults)
	}

	return &snapshot, nil
}

func (r *ComplianceSnapshotRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.ComplianceSnapshot, error) {
	query := `
		SELECT id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		FROM compliance_snapshots
		WHERE organization_id = $1
		ORDER BY snapshot_date DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSnapshotRows(rows)
}

func (r *ComplianceSnapshotRepository) GetByFramework(orgID uuid.UUID, framework domain.ComplianceFramework, limit int) ([]*domain.ComplianceSnapshot, error) {
	query := `
		SELECT id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		FROM compliance_snapshots
		WHERE organization_id = $1 AND framework = $2
		ORDER BY snapshot_date DESC
		LIMIT $3
	`

	rows, err := r.db.Query(query, orgID, string(framework), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSnapshotRows(rows)
}

func (r *ComplianceSnapshotRepository) GetTrending(orgID uuid.UUID, framework domain.ComplianceFramework, startDate, endDate time.Time) ([]*domain.ComplianceSnapshot, error) {
	query := `
		SELECT id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		FROM compliance_snapshots
		WHERE organization_id = $1 AND framework = $2 AND snapshot_date BETWEEN $3 AND $4
		ORDER BY snapshot_date ASC
	`

	rows, err := r.db.Query(query, orgID, string(framework), startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSnapshotRows(rows)
}

func (r *ComplianceSnapshotRepository) GetLatest(orgID uuid.UUID, framework domain.ComplianceFramework) (*domain.ComplianceSnapshot, error) {
	query := `
		SELECT id, organization_id, framework, score, passed_checks,
			failed_checks, total_checks, check_results, snapshot_date, created_at
		FROM compliance_snapshots
		WHERE organization_id = $1 AND framework = $2
		ORDER BY snapshot_date DESC
		LIMIT 1
	`

	var snapshot domain.ComplianceSnapshot
	var checkResultsJSON []byte
	var fw string

	err := r.db.QueryRow(query, orgID, string(framework)).Scan(
		&snapshot.ID,
		&snapshot.OrganizationID,
		&fw,
		&snapshot.Score,
		&snapshot.PassedChecks,
		&snapshot.FailedChecks,
		&snapshot.TotalChecks,
		&checkResultsJSON,
		&snapshot.SnapshotDate,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	snapshot.Framework = domain.ComplianceFramework(fw)

	if len(checkResultsJSON) > 0 {
		json.Unmarshal(checkResultsJSON, &snapshot.CheckResults)
	}

	return &snapshot, nil
}

func (r *ComplianceSnapshotRepository) DeleteOlderThan(orgID uuid.UUID, before time.Time) (int, error) {
	query := `DELETE FROM compliance_snapshots WHERE organization_id = $1 AND snapshot_date < $2`
	result, err := r.db.Exec(query, orgID, before)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (r *ComplianceSnapshotRepository) scanSnapshotRows(rows *sql.Rows) ([]*domain.ComplianceSnapshot, error) {
	var snapshots []*domain.ComplianceSnapshot

	for rows.Next() {
		var snapshot domain.ComplianceSnapshot
		var checkResultsJSON []byte
		var framework string

		err := rows.Scan(
			&snapshot.ID,
			&snapshot.OrganizationID,
			&framework,
			&snapshot.Score,
			&snapshot.PassedChecks,
			&snapshot.FailedChecks,
			&snapshot.TotalChecks,
			&checkResultsJSON,
			&snapshot.SnapshotDate,
			&snapshot.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		snapshot.Framework = domain.ComplianceFramework(framework)

		if len(checkResultsJSON) > 0 {
			json.Unmarshal(checkResultsJSON, &snapshot.CheckResults)
		}

		snapshots = append(snapshots, &snapshot)
	}

	return snapshots, rows.Err()
}
