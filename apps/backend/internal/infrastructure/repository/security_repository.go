package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a/identity/backend/internal/domain"
)

type SecurityRepository struct {
	db *sql.DB
}

func NewSecurityRepository(db *sql.DB) *SecurityRepository {
	return &SecurityRepository{db: db}
}

// Threats

func (r *SecurityRepository) CreateThreat(threat *domain.Threat) error {
	query := `
		INSERT INTO security_threats (
			id, organization_id, threat_type, severity, title, description,
			source, target_type, target_id, is_blocked, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.Exec(
		query,
		threat.ID,
		threat.OrganizationID,
		threat.ThreatType,
		threat.Severity,
		threat.Title,
		threat.Description,
		threat.Source,
		threat.TargetType,
		threat.TargetID,
		threat.IsBlocked,
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to create threat: %w", err)
	}

	return nil
}

func (r *SecurityRepository) GetThreats(orgID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
	query := `
		SELECT
			st.id, st.organization_id, st.threat_type, st.severity, st.title, st.description,
			st.source, st.target_type, st.target_id, st.is_blocked, st.created_at, st.resolved_at,
			COALESCE(a.display_name, a.name, mcp.name) as target_name
		FROM security_threats st
		LEFT JOIN agents a ON st.target_type = 'agent' AND st.target_id = a.id
		LEFT JOIN mcp_servers mcp ON st.target_type = 'mcp_server' AND st.target_id = mcp.id
		WHERE st.organization_id = $1
		ORDER BY st.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get threats: %w", err)
	}
	defer rows.Close()

	var threats []*domain.Threat
	for rows.Next() {
		threat := &domain.Threat{}
		var targetName sql.NullString
		err := rows.Scan(
			&threat.ID,
			&threat.OrganizationID,
			&threat.ThreatType,
			&threat.Severity,
			&threat.Title,
			&threat.Description,
			&threat.Source,
			&threat.TargetType,
			&threat.TargetID,
			&threat.IsBlocked,
			&threat.CreatedAt,
			&threat.ResolvedAt,
			&targetName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan threat: %w", err)
		}

		// Set target name if available
		if targetName.Valid {
			threat.TargetName = &targetName.String
		}

		threats = append(threats, threat)
	}

	return threats, nil
}

func (r *SecurityRepository) GetThreatByID(id uuid.UUID) (*domain.Threat, error) {
	query := `
		SELECT
			id, organization_id, threat_type, severity, title, description,
			source, target_type, target_id, is_blocked, created_at, resolved_at
		FROM security_threats
		WHERE id = $1
	`

	threat := &domain.Threat{}
	err := r.db.QueryRow(query, id).Scan(
		&threat.ID,
		&threat.OrganizationID,
		&threat.ThreatType,
		&threat.Severity,
		&threat.Title,
		&threat.Description,
		&threat.Source,
		&threat.TargetType,
		&threat.TargetID,
		&threat.IsBlocked,
		&threat.CreatedAt,
		&threat.ResolvedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("threat not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get threat: %w", err)
	}

	return threat, nil
}

func (r *SecurityRepository) BlockThreat(id uuid.UUID) error {
	query := `UPDATE security_threats SET is_blocked = true WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *SecurityRepository) ResolveThreat(id uuid.UUID) error {
	query := `UPDATE security_threats SET resolved_at = $1 WHERE id = $2`
	_, err := r.db.Exec(query, time.Now().UTC(), id)
	return err
}

// Anomalies

func (r *SecurityRepository) CreateAnomaly(anomaly *domain.Anomaly) error {
	query := `
		INSERT INTO security_anomalies (
			id, organization_id, anomaly_type, severity, title, description,
			resource_type, resource_id, confidence, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.Exec(
		query,
		anomaly.ID,
		anomaly.OrganizationID,
		anomaly.AnomalyType,
		anomaly.Severity,
		anomaly.Title,
		anomaly.Description,
		anomaly.ResourceType,
		anomaly.ResourceID,
		anomaly.Confidence,
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to create anomaly: %w", err)
	}

	return nil
}

func (r *SecurityRepository) GetAnomalies(orgID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
	query := `
		SELECT
			id, organization_id, anomaly_type, severity, title, description,
			resource_type, resource_id, confidence, created_at
		FROM security_anomalies
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies: %w", err)
	}
	defer rows.Close()

	var anomalies []*domain.Anomaly
	for rows.Next() {
		anomaly := &domain.Anomaly{}
		err := rows.Scan(
			&anomaly.ID,
			&anomaly.OrganizationID,
			&anomaly.AnomalyType,
			&anomaly.Severity,
			&anomaly.Title,
			&anomaly.Description,
			&anomaly.ResourceType,
			&anomaly.ResourceID,
			&anomaly.Confidence,
			&anomaly.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan anomaly: %w", err)
		}
		anomalies = append(anomalies, anomaly)
	}

	return anomalies, nil
}

func (r *SecurityRepository) GetAnomalyByID(id uuid.UUID) (*domain.Anomaly, error) {
	query := `
		SELECT
			id, organization_id, anomaly_type, severity, title, description,
			resource_type, resource_id, confidence, created_at
		FROM security_anomalies
		WHERE id = $1
	`

	anomaly := &domain.Anomaly{}
	err := r.db.QueryRow(query, id).Scan(
		&anomaly.ID,
		&anomaly.OrganizationID,
		&anomaly.AnomalyType,
		&anomaly.Severity,
		&anomaly.Title,
		&anomaly.Description,
		&anomaly.ResourceType,
		&anomaly.ResourceID,
		&anomaly.Confidence,
		&anomaly.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("anomaly not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get anomaly: %w", err)
	}

	return anomaly, nil
}

// Incidents

func (r *SecurityRepository) CreateIncident(incident *domain.SecurityIncident) error {
	query := `
		INSERT INTO security_incidents (
			id, organization_id, incident_type, status, severity, title, description,
			affected_resources, assigned_to, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.Exec(
		query,
		incident.ID,
		incident.OrganizationID,
		incident.IncidentType,
		incident.Status,
		incident.Severity,
		incident.Title,
		incident.Description,
		pq.Array(incident.AffectedResources),
		incident.AssignedTo,
		time.Now().UTC(),
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to create incident: %w", err)
	}

	return nil
}

func (r *SecurityRepository) GetIncidents(orgID uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT
				id, organization_id, incident_type, status, severity, title, description,
				affected_resources, assigned_to, created_at, updated_at, resolved_at, resolved_by, resolution_notes
			FROM security_incidents
			WHERE organization_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{orgID, status, limit, offset}
	} else {
		query = `
			SELECT
				id, organization_id, incident_type, status, severity, title, description,
				affected_resources, assigned_to, created_at, updated_at, resolved_at, resolved_by, resolution_notes
			FROM security_incidents
			WHERE organization_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get incidents: %w", err)
	}
	defer rows.Close()

	var incidents []*domain.SecurityIncident
	for rows.Next() {
		incident := &domain.SecurityIncident{}
		var affectedResources []string
		var assignedTo, resolvedBy, resolutionNotes sql.NullString
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&incident.ID,
			&incident.OrganizationID,
			&incident.IncidentType,
			&incident.Status,
			&incident.Severity,
			&incident.Title,
			&incident.Description,
			pq.Array(&affectedResources),
			&assignedTo,
			&incident.CreatedAt,
			&incident.UpdatedAt,
			&resolvedAt,
			&resolvedBy,
			&resolutionNotes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan incident: %w", err)
		}

		incident.AffectedResources = affectedResources

		// Handle nullable fields
		if assignedTo.Valid {
			uid, _ := uuid.Parse(assignedTo.String)
			incident.AssignedTo = &uid
		}
		if resolvedBy.Valid {
			uid, _ := uuid.Parse(resolvedBy.String)
			incident.ResolvedBy = &uid
		}
		if resolvedAt.Valid {
			incident.ResolvedAt = &resolvedAt.Time
		}
		if resolutionNotes.Valid {
			incident.ResolutionNotes = resolutionNotes.String
		}

		incidents = append(incidents, incident)
	}

	return incidents, nil
}

func (r *SecurityRepository) GetIncidentByID(id uuid.UUID) (*domain.SecurityIncident, error) {
	query := `
		SELECT
			id, organization_id, incident_type, status, severity, title, description,
			affected_resources, assigned_to, created_at, updated_at, resolved_at, resolved_by, resolution_notes
		FROM security_incidents
		WHERE id = $1
	`

	incident := &domain.SecurityIncident{}
	var affectedResources []string
	var assignedTo, resolvedBy, resolutionNotes sql.NullString
	var resolvedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&incident.ID,
		&incident.OrganizationID,
		&incident.IncidentType,
		&incident.Status,
		&incident.Severity,
		&incident.Title,
		&incident.Description,
		pq.Array(&affectedResources),
		&assignedTo,
		&incident.CreatedAt,
		&incident.UpdatedAt,
		&resolvedAt,
		&resolvedBy,
		&resolutionNotes,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("incident not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get incident: %w", err)
	}

	incident.AffectedResources = affectedResources

	// Handle nullable fields
	if assignedTo.Valid {
		uid, _ := uuid.Parse(assignedTo.String)
		incident.AssignedTo = &uid
	}
	if resolvedBy.Valid {
		uid, _ := uuid.Parse(resolvedBy.String)
		incident.ResolvedBy = &uid
	}
	if resolvedAt.Valid {
		incident.ResolvedAt = &resolvedAt.Time
	}
	if resolutionNotes.Valid {
		incident.ResolutionNotes = resolutionNotes.String
	}

	return incident, nil
}

func (r *SecurityRepository) UpdateIncidentStatus(id uuid.UUID, status domain.IncidentStatus, resolvedBy *uuid.UUID, notes string) error {
	var query string
	var args []interface{}

	if status == domain.IncidentStatusResolved {
		query = `
			UPDATE security_incidents
			SET status = $1, resolved_at = $2, resolved_by = $3, resolution_notes = $4, updated_at = $5
			WHERE id = $6
		`
		args = []interface{}{status, time.Now().UTC(), resolvedBy, notes, time.Now().UTC(), id}
	} else {
		query = `
			UPDATE security_incidents
			SET status = $1, updated_at = $2
			WHERE id = $3
		`
		args = []interface{}{status, time.Now().UTC(), id}
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update incident status: %w", err)
	}

	return nil
}

// Metrics

func (r *SecurityRepository) GetSecurityMetrics(orgID uuid.UUID) (*domain.SecurityMetrics, error) {
	metrics := &domain.SecurityMetrics{}

	// Count threats from alerts table
	r.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_acknowledged THEN 1 ELSE 0 END), 0)
		FROM alerts
		WHERE organization_id = $1
	`, orgID).Scan(&metrics.TotalThreats, &metrics.BlockedThreats)

	metrics.ActiveThreats = metrics.TotalThreats - metrics.BlockedThreats

	// Count anomalies
	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM security_anomalies
		WHERE organization_id = $1
	`, orgID).Scan(&metrics.TotalAnomalies)

	// Count high severity items from alerts table
	r.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM alerts WHERE organization_id = $1 AND severity = 'high'
			UNION ALL
			SELECT 1 FROM security_anomalies WHERE organization_id = $1 AND severity = 'critical'
		) as high_severity
	`, orgID).Scan(&metrics.HighSeverityCount)

	// Count open incidents
	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM security_incidents
		WHERE organization_id = $1 AND status IN ('open', 'investigating')
	`, orgID).Scan(&metrics.OpenIncidents)

	// Get average trust score
	r.db.QueryRow(`
		SELECT COALESCE(AVG(trust_score), 0)
		FROM agents
		WHERE organization_id = $1
	`, orgID).Scan(&metrics.AverageTrustScore)

	// Calculate security score (simple formula)
	metrics.SecurityScore = 100.0
	if metrics.TotalThreats > 0 {
		metrics.SecurityScore -= float64(metrics.ActiveThreats) / float64(metrics.TotalThreats) * 30
	}
	if metrics.HighSeverityCount > 0 {
		metrics.SecurityScore -= float64(metrics.HighSeverityCount) * 10
	}
	if metrics.OpenIncidents > 0 {
		metrics.SecurityScore -= float64(metrics.OpenIncidents) * 5
	}

	if metrics.SecurityScore < 0 {
		metrics.SecurityScore = 0
	}

	// Get threat trend (last 7 days) from alerts table
	trendRows, err := r.db.Query(`
		SELECT
			TO_CHAR(DATE(created_at), 'Mon DD') as date,
			COUNT(*) as count
		FROM alerts
		WHERE organization_id = $1
			AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, orgID)
	if err == nil {
		defer trendRows.Close()
		for trendRows.Next() {
			var trend domain.ThreatTrendData
			if err := trendRows.Scan(&trend.Date, &trend.Count); err == nil {
				metrics.ThreatTrend = append(metrics.ThreatTrend, trend)
			}
		}
	}

	// Get severity distribution from alerts table
	sevRows, err := r.db.Query(`
		SELECT
			INITCAP(severity::TEXT) as severity,
			COUNT(*) as count
		FROM alerts
		WHERE organization_id = $1
		GROUP BY severity
		ORDER BY
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
			END
	`, orgID)
	if err == nil {
		defer sevRows.Close()
		for sevRows.Next() {
			var sev domain.SeverityDistribution
			if err := sevRows.Scan(&sev.Severity, &sev.Count); err == nil {
				metrics.SeverityDistribution = append(metrics.SeverityDistribution, sev)
			}
		}
	}

	return metrics, nil
}

// Scans

func (r *SecurityRepository) CreateSecurityScan(scan *domain.SecurityScanResult) error {
	query := `
		INSERT INTO security_scans (
			id, organization_id, scan_type, status, threats_found, anomalies_found,
			vulnerabilities_found, security_score, started_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(
		query,
		scan.ScanID,
		scan.OrganizationID,
		scan.ScanType,
		scan.Status,
		scan.ThreatsFound,
		scan.AnomaliesFound,
		scan.VulnerabilitiesFound,
		scan.SecurityScore,
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to create security scan: %w", err)
	}

	return nil
}

// CountOpenIncidents returns the count of open and investigating security incidents
func (r *SecurityRepository) CountOpenIncidents(orgID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM security_incidents
		WHERE organization_id = $1 AND status IN ('open', 'investigating')
	`, orgID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count open incidents: %w", err)
	}

	return count, nil
}

func (r *SecurityRepository) GetSecurityScan(scanID uuid.UUID) (*domain.SecurityScanResult, error) {
	query := `
		SELECT
			id, organization_id, scan_type, status, threats_found, anomalies_found,
			vulnerabilities_found, security_score, started_at, completed_at
		FROM security_scans
		WHERE id = $1
	`

	scan := &domain.SecurityScanResult{}
	err := r.db.QueryRow(query, scanID).Scan(
		&scan.ScanID,
		&scan.OrganizationID,
		&scan.ScanType,
		&scan.Status,
		&scan.ThreatsFound,
		&scan.AnomaliesFound,
		&scan.VulnerabilitiesFound,
		&scan.SecurityScore,
		&scan.StartedAt,
		&scan.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("security scan not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get security scan: %w", err)
	}

	return scan, nil
}
