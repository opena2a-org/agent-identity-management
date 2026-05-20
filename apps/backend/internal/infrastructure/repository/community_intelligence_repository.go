package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// CommunityIntelligenceRepository implements domain.CommunityIntelligenceRepository
type CommunityIntelligenceRepository struct {
	db *sql.DB
}

// NewCommunityIntelligenceRepository creates a new CI repository
func NewCommunityIntelligenceRepository(db *sql.DB) *CommunityIntelligenceRepository {
	return &CommunityIntelligenceRepository{db: db}
}

// SaveReport persists a community intelligence report
func (r *CommunityIntelligenceRepository) SaveReport(report *domain.CommunityIntelligenceReport) error {
	query := `
		INSERT INTO community_intelligence_reports (id, organization_id, report_type, agent_count, data_points, submitted_at, registry_accepted, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	if report.ID == uuid.Nil {
		report.ID = uuid.New()
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}

	dataPointsJSON, err := json.Marshal(report.DataPoints)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(query,
		report.ID,
		report.OrganizationID,
		report.ReportType,
		report.AgentCount,
		dataPointsJSON,
		report.SubmittedAt,
		report.RegistryAccepted,
		report.CreatedAt,
	)
	return err
}

// GetReportsByOrg returns the most recent reports for an organization
func (r *CommunityIntelligenceRepository) GetReportsByOrg(orgID uuid.UUID, limit int) ([]*domain.CommunityIntelligenceReport, error) {
	query := `
		SELECT id, organization_id, report_type, agent_count, data_points, submitted_at, registry_accepted, created_at
		FROM community_intelligence_reports
		WHERE organization_id = $1
		ORDER BY submitted_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanReports(rows)
}

// GetOptedInOrgCount returns the number of organizations with community intelligence enabled
func (r *CommunityIntelligenceRepository) GetOptedInOrgCount() (int, error) {
	query := `SELECT COUNT(*) FROM organizations WHERE community_intelligence_enabled = true AND is_active = true`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// GetAllRecentReports returns all reports submitted since the given time
func (r *CommunityIntelligenceRepository) GetAllRecentReports(since time.Time) ([]*domain.CommunityIntelligenceReport, error) {
	query := `
		SELECT id, organization_id, report_type, agent_count, data_points, submitted_at, registry_accepted, created_at
		FROM community_intelligence_reports
		WHERE submitted_at >= $1
		ORDER BY submitted_at DESC
	`

	rows, err := r.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanReports(rows)
}

func scanReports(rows *sql.Rows) ([]*domain.CommunityIntelligenceReport, error) {
	reports := make([]*domain.CommunityIntelligenceReport, 0)
	for rows.Next() {
		report := &domain.CommunityIntelligenceReport{}
		dataPointsJSON := make([]byte, 0)

		if err := rows.Scan(
			&report.ID,
			&report.OrganizationID,
			&report.ReportType,
			&report.AgentCount,
			&dataPointsJSON,
			&report.SubmittedAt,
			&report.RegistryAccepted,
			&report.CreatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(dataPointsJSON, &report.DataPoints); err != nil {
			return nil, err
		}

		reports = append(reports, report)
	}
	return reports, rows.Err()
}
