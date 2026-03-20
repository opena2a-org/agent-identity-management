package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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

	// ============================================
	// PRIMARY METRICS - Actions Blocked (from capability_violations)
	// ============================================
	var totalViolations, blockedViolations, blockedToday int
	var lastBlockedAt *string
	r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_blocked THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_blocked AND created_at >= CURRENT_DATE THEN 1 ELSE 0 END), 0),
			(SELECT TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			 FROM capability_violations
			 WHERE is_blocked = true
			 ORDER BY created_at DESC
			 LIMIT 1)
		FROM capability_violations
		WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
	`, orgID).Scan(&totalViolations, &blockedViolations, &blockedToday, &lastBlockedAt)

	metrics.ActionsBlocked = blockedViolations
	metrics.ActionsBlockedToday = blockedToday
	if lastBlockedAt != nil {
		metrics.LastIncidentAt = *lastBlockedAt
	}

	// Legacy: map to old fields for backward compatibility
	metrics.BlockedThreats = blockedViolations
	metrics.TotalThreats = totalViolations
	metrics.ActiveThreats = totalViolations - blockedViolations

	// ============================================
	// AGENTS METRICS
	// ============================================
	var agentsTotal, agentsActive, agentsTrusted int
	var avgTrustScore float64
	r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status IN ('active', 'verified') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN trust_score >= 0.8 THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(trust_score), 0)
		FROM agents
		WHERE organization_id = $1
	`, orgID).Scan(&agentsTotal, &agentsActive, &agentsTrusted, &avgTrustScore)

	metrics.AgentsMonitored = agentsActive
	metrics.AgentsTrusted = agentsTrusted
	metrics.AverageTrustScore = avgTrustScore
	if agentsActive > 0 {
		metrics.TrustPercentage = int(float64(agentsTrusted) / float64(agentsActive) * 100)
	}

	// ============================================
	// MCP SERVER METRICS
	// ============================================
	var mcpTotal, mcpVerified int
	r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_verified THEN 1 ELSE 0 END), 0)
		FROM mcp_servers
		WHERE organization_id = $1
	`, orgID).Scan(&mcpTotal, &mcpVerified)

	metrics.MCPServersTotal = mcpTotal
	metrics.MCPServersVerified = mcpVerified
	if mcpTotal > 0 {
		metrics.MCPTrustPercentage = int(float64(mcpVerified) / float64(mcpTotal) * 100)
	}

	// ============================================
	// ACTIONS TODAY (from verification_events - actual agent operations)
	// ============================================
	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM verification_events
		WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
			AND created_at >= CURRENT_DATE
	`, orgID).Scan(&metrics.ActionsToday)

	// ============================================
	// REQUIRES ATTENTION (pending requests + unacknowledged alerts)
	// ============================================
	var pendingRequests, unacknowledgedAlerts int
	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM capability_requests
		WHERE organization_id = $1 AND status = 'pending'
	`, orgID).Scan(&pendingRequests)

	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM alerts
		WHERE organization_id = $1 AND is_acknowledged = false
	`, orgID).Scan(&unacknowledgedAlerts)

	metrics.RequiresAttention = pendingRequests + unacknowledgedAlerts

	// ============================================
	// ANOMALIES AND HIGH SEVERITY
	// ============================================
	r.db.QueryRow(`
		SELECT COUNT(*)
		FROM security_anomalies
		WHERE organization_id = $1
	`, orgID).Scan(&metrics.TotalAnomalies)

	r.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM alerts WHERE organization_id = $1 AND severity IN ('high', 'critical')
			UNION ALL
			SELECT 1 FROM security_anomalies WHERE organization_id = $1 AND severity = 'critical'
			UNION ALL
			SELECT 1 FROM capability_violations WHERE severity IN ('high', 'critical')
				AND agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
		) as high_severity
	`, orgID).Scan(&metrics.HighSeverityCount)

	// ============================================
	// SECURITY SCORE CALCULATION
	// ============================================
	// Smart scoring that reflects actual security posture:
	//
	// 1. Trust Health (40 points): Average trust score of all agents
	//    - High trust = agents behaving well
	//
	// 2. Fleet Coverage (25 points): % of agents verified/active with good trust
	//    - More trusted agents = better security posture
	//
	// 3. Threat Response (25 points): How well we block unauthorized actions
	//    - More blocked violations = AIM is protecting the organization
	//    - Zero violations = full points (nothing bad attempted)
	//
	// 4. Operational Health (10 points): Pending attention items managed
	//    - Fewer pending items relative to fleet size = better
	//    - Normalized to fleet size (large orgs have more alerts)

	// If no agents exist, score is 0 — an empty fleet is not a secure fleet
	if agentsTotal == 0 {
		metrics.SecurityScore = 0
		metrics.SecurityGrade = "—"
		metrics.SecurityStatus = "No Agents"
	} else {
	// 1. Trust Health (40 points)
	trustComponent := metrics.AverageTrustScore * 40

	// 2. Fleet Coverage (25 points)
	// % of agents that are trusted (trust > 80%)
	trustedRatio := float64(agentsTrusted) / float64(agentsTotal)
	fleetComponent := trustedRatio * 25

	// 3. Threat Response (25 points)
	var threatComponent float64 = 25 // Full points if no violations (nothing bad attempted)
	if totalViolations > 0 {
		// Blocking violations is GOOD - we want high block rate
		// 100% blocked = full points, 0% blocked = 0 points
		blockRate := float64(blockedViolations) / float64(totalViolations)
		threatComponent = blockRate * 25
	}

	// 4. Operational Health (10 points)
	// Scale penalty based on fleet size - large orgs naturally have more alerts
	// Normalize pending items to fleet size
	pendingRatio := float64(metrics.RequiresAttention) / float64(agentsTotal)
	if pendingRatio > 1.0 {
		pendingRatio = 1.0
	}
	// Invert: fewer pending = higher score
	opsComponent := (1.0 - pendingRatio) * 10

	// Calculate final score
	score := int(trustComponent + fleetComponent + threatComponent + opsComponent)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	metrics.SecurityScore = score

	// Determine grade and status
	switch {
	case score >= 90:
		metrics.SecurityGrade = "A"
		metrics.SecurityStatus = "Secure"
	case score >= 80:
		metrics.SecurityGrade = "B"
		metrics.SecurityStatus = "Good"
	case score >= 70:
		metrics.SecurityGrade = "C"
		metrics.SecurityStatus = "Needs Attention"
	case score >= 60:
		metrics.SecurityGrade = "D"
		metrics.SecurityStatus = "At Risk"
	default:
		metrics.SecurityGrade = "F"
		metrics.SecurityStatus = "Critical"
	}
	} // end else (agentsTotal > 0)

	// ============================================
	// PROTECTION TIMELINE (last 30 days)
	// ============================================
	timelineRows, err := r.db.Query(`
		WITH dates AS (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '29 days',
				CURRENT_DATE,
				'1 day'::interval
			)::date as date
		),
		actions_by_date AS (
			SELECT DATE(created_at) as date, COUNT(*) as count
			FROM verification_events
			WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
				AND created_at >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY DATE(created_at)
		),
		blocked_by_date AS (
			SELECT DATE(created_at) as date, COUNT(*) as count
			FROM capability_violations
			WHERE is_blocked = true
				AND agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
				AND created_at >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY DATE(created_at)
		)
		SELECT
			TO_CHAR(d.date, 'Mon DD') as date,
			COALESCE(a.count, 0) as actions,
			COALESCE(b.count, 0) as blocked
		FROM dates d
		LEFT JOIN actions_by_date a ON d.date = a.date
		LEFT JOIN blocked_by_date b ON d.date = b.date
		ORDER BY d.date ASC
	`, orgID)
	if err == nil {
		defer timelineRows.Close()
		for timelineRows.Next() {
			var timeline domain.ProtectionTimelineData
			if err := timelineRows.Scan(&timeline.Date, &timeline.Actions, &timeline.Blocked); err == nil {
				metrics.ProtectionTimeline = append(metrics.ProtectionTimeline, timeline)
			}
		}
	}

	// ============================================
	// RISK BY CATEGORY (group violations by capability namespace)
	// ============================================
	categoryRows, err := r.db.Query(`
		SELECT
			COALESCE(
				CASE
					WHEN attempted_capability LIKE 'admin:%' THEN 'Admin'
					WHEN attempted_capability LIKE 'file:%' THEN 'File System'
					WHEN attempted_capability LIKE 'db:%' OR attempted_capability LIKE 'database:%' THEN 'Database'
					WHEN attempted_capability LIKE 'network:%' THEN 'Network'
					WHEN attempted_capability LIKE 'secret:%' OR attempted_capability LIKE 'credential:%' THEN 'Secrets'
					WHEN attempted_capability LIKE 'payment:%' OR attempted_capability LIKE 'financial:%' THEN 'Financial'
					WHEN attempted_capability LIKE 'notification:%' OR attempted_capability LIKE 'email:%' THEN 'Notifications'
					WHEN attempted_capability LIKE 'user:%' THEN 'User Management'
					WHEN attempted_capability LIKE 'api:%' OR attempted_capability LIKE 'http:%' THEN 'External API'
					WHEN attempted_capability LIKE 'data:%' OR attempted_capability LIKE 'export:%' THEN 'Data Export'
					WHEN attempted_capability LIKE 'critical:%' THEN 'Critical Operations'
					ELSE INITCAP(SPLIT_PART(attempted_capability, ':', 1))
				END,
				'Other'
			) as category,
			COUNT(*) as blocked
		FROM capability_violations
		WHERE is_blocked = true
			AND agent_id IN (SELECT id FROM agents WHERE organization_id = $1)
		GROUP BY category
		ORDER BY blocked DESC
		LIMIT 8
	`, orgID)
	if err == nil {
		defer categoryRows.Close()
		for categoryRows.Next() {
			var cat domain.RiskByCategoryData
			if err := categoryRows.Scan(&cat.Category, &cat.Blocked); err == nil {
				// Determine risk level based on count
				switch {
				case cat.Blocked >= 10:
					cat.RiskLevel = "high"
				case cat.Blocked >= 5:
					cat.RiskLevel = "medium"
				case cat.Blocked >= 1:
					cat.RiskLevel = "low"
				default:
					cat.RiskLevel = "secure"
				}
				metrics.RiskByCategory = append(metrics.RiskByCategory, cat)
			}
		}
	}

	// ============================================
	// RECENT BLOCKED ACTIONS
	// ============================================
	blockedRows, err := r.db.Query(`
		SELECT
			cv.id::text,
			cv.agent_id::text,
			COALESCE(a.name, a.display_name, 'Unknown Agent') as agent_name,
			cv.attempted_capability,
			COALESCE(cv.request_metadata::text, '{}'),
			cv.trust_score_impact,
			TO_CHAR(cv.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM capability_violations cv
		LEFT JOIN agents a ON cv.agent_id = a.id
		WHERE cv.is_blocked = true
			AND a.organization_id = $1
		ORDER BY cv.created_at DESC
		LIMIT 10
	`, orgID)
	if err == nil {
		defer blockedRows.Close()
		for blockedRows.Next() {
			var action domain.BlockedActionData
			var metadata string
			if err := blockedRows.Scan(&action.ID, &action.AgentID, &action.AgentName,
				&action.AttemptedCapability, &metadata, &action.TrustImpact, &action.CreatedAt); err == nil {
				action.Details = metadata
				metrics.RecentBlockedActions = append(metrics.RecentBlockedActions, action)
			}
		}
	}

	// ============================================
	// LEGACY: Threat trend from alerts (for backward compatibility)
	// ============================================
	trendRows, err := r.db.Query(`
		SELECT
			TO_CHAR(DATE(created_at), 'Mon DD') as date,
			COUNT(*) as count
		FROM alerts
		WHERE organization_id = $1
			AND created_at >= NOW() - INTERVAL '30 days'
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

	// ============================================
	// LEGACY: Severity distribution from alerts
	// ============================================
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
				WHEN 'warning' THEN 5
				WHEN 'info' THEN 6
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

// UpdateSecurityScan updates a security scan with results
func (r *SecurityRepository) UpdateSecurityScan(scanID uuid.UUID, threatsFound, anomaliesFound, vulnerabilitiesFound int, securityScore float64, status string, completedAt *time.Time) error {
	query := `
		UPDATE security_scans
		SET threats_found = $1, anomalies_found = $2, vulnerabilities_found = $3,
			security_score = $4, status = $5, completed_at = $6
		WHERE id = $7
	`

	_, err := r.db.Exec(
		query,
		threatsFound,
		anomaliesFound,
		vulnerabilitiesFound,
		securityScore,
		status,
		completedAt,
		scanID,
	)

	if err != nil {
		return fmt.Errorf("failed to update security scan: %w", err)
	}

	return nil
}
