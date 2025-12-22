package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type MCPServerCapabilityRepository struct {
	db        *sql.DB
	alertRepo *AlertRepository
}

func NewMCPServerCapabilityRepository(db *sql.DB) *MCPServerCapabilityRepository {
	return &MCPServerCapabilityRepository{db: db}
}

// SetAlertRepository sets the alert repository for creating drift alerts
func (r *MCPServerCapabilityRepository) SetAlertRepository(alertRepo *AlertRepository) {
	r.alertRepo = alertRepo
}

func (r *MCPServerCapabilityRepository) Create(capability *domain.MCPServerCapability) error {
	query := `
		INSERT INTO mcp_server_capabilities (
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		capability.ID,
		capability.MCPServerID,
		capability.Name,
		capability.CapabilityType,
		capability.Description,
		capability.CapabilitySchema,
		capability.DetectedAt,
		capability.LastVerifiedAt,
		capability.IsActive,
		time.Now().UTC(),
		time.Now().UTC(),
	).Scan(&capability.ID, &capability.CreatedAt, &capability.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create mcp server capability: %w", err)
	}

	return nil
}

func (r *MCPServerCapabilityRepository) GetByID(id uuid.UUID) (*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE id = $1
	`

	capability := &domain.MCPServerCapability{}
	var description sql.NullString
	var capabilitySchema []byte
	var lastVerifiedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&capability.ID,
		&capability.MCPServerID,
		&capability.Name,
		&capability.CapabilityType,
		&description,
		&capabilitySchema,
		&capability.DetectedAt,
		&lastVerifiedAt,
		&capability.IsActive,
		&capability.CreatedAt,
		&capability.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server capability not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server capability: %w", err)
	}

	// Convert nullable fields
	if description.Valid {
		capability.Description = description.String
	}
	if capabilitySchema != nil {
		capability.CapabilitySchema = capabilitySchema
	}
	if lastVerifiedAt.Valid {
		capability.LastVerifiedAt = &lastVerifiedAt.Time
	}

	return capability, nil
}

func (r *MCPServerCapabilityRepository) GetByServerID(serverID uuid.UUID) ([]*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE mcp_server_id = $1 AND is_active = true
		ORDER BY capability_type, name
	`

	rows, err := r.db.Query(query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp server capabilities: %w", err)
	}
	defer rows.Close()

	var capabilities []*domain.MCPServerCapability
	for rows.Next() {
		capability := &domain.MCPServerCapability{}
		var description sql.NullString
		var capabilitySchema []byte
		var lastVerifiedAt sql.NullTime

		err := rows.Scan(
			&capability.ID,
			&capability.MCPServerID,
			&capability.Name,
			&capability.CapabilityType,
			&description,
			&capabilitySchema,
			&capability.DetectedAt,
			&lastVerifiedAt,
			&capability.IsActive,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server capability: %w", err)
		}

		// Convert nullable fields
		if description.Valid {
			capability.Description = description.String
		}
		if capabilitySchema != nil {
			capability.CapabilitySchema = capabilitySchema
		}
		if lastVerifiedAt.Valid {
			capability.LastVerifiedAt = &lastVerifiedAt.Time
		}

		capabilities = append(capabilities, capability)
	}

	return capabilities, nil
}

func (r *MCPServerCapabilityRepository) GetByServerIDAndType(serverID uuid.UUID, capType domain.MCPCapabilityType) ([]*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE mcp_server_id = $1 AND capability_type = $2 AND is_active = true
		ORDER BY name
	`

	rows, err := r.db.Query(query, serverID, capType)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp server capabilities by type: %w", err)
	}
	defer rows.Close()

	var capabilities []*domain.MCPServerCapability
	for rows.Next() {
		capability := &domain.MCPServerCapability{}
		var description sql.NullString
		var capabilitySchema []byte
		var lastVerifiedAt sql.NullTime

		err := rows.Scan(
			&capability.ID,
			&capability.MCPServerID,
			&capability.Name,
			&capability.CapabilityType,
			&description,
			&capabilitySchema,
			&capability.DetectedAt,
			&lastVerifiedAt,
			&capability.IsActive,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server capability: %w", err)
		}

		// Convert nullable fields
		if description.Valid {
			capability.Description = description.String
		}
		if capabilitySchema != nil {
			capability.CapabilitySchema = capabilitySchema
		}
		if lastVerifiedAt.Valid {
			capability.LastVerifiedAt = &lastVerifiedAt.Time
		}

		capabilities = append(capabilities, capability)
	}

	return capabilities, nil
}

func (r *MCPServerCapabilityRepository) Update(capability *domain.MCPServerCapability) error {
	query := `
		UPDATE mcp_server_capabilities
		SET
			name = $1,
			description = $2,
			capability_schema = $3,
			last_verified_at = $4,
			is_active = $5,
			updated_at = $6
		WHERE id = $7
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		capability.Name,
		capability.Description,
		capability.CapabilitySchema,
		capability.LastVerifiedAt,
		capability.IsActive,
		time.Now().UTC(),
		capability.ID,
	).Scan(&capability.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update mcp server capability: %w", err)
	}

	return nil
}

func (r *MCPServerCapabilityRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM mcp_server_capabilities WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete mcp server capability: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server capability not found")
	}

	return nil
}

func (r *MCPServerCapabilityRepository) DeleteByServerID(serverID uuid.UUID) error {
	query := `DELETE FROM mcp_server_capabilities WHERE mcp_server_id = $1`

	_, err := r.db.Exec(query, serverID)
	if err != nil {
		return fmt.Errorf("failed to delete capabilities for server: %w", err)
	}

	return nil
}

// CapabilityDriftAlert represents a detected change in MCP server capabilities
type CapabilityDriftAlert struct {
	ID                 string    `json:"id"`
	MCPServerID        string    `json:"mcpServerId"`
	MCPServerName      string    `json:"mcpServerName"`
	DriftType          string    `json:"driftType"` // "added", "removed", "modified"
	Severity           string    `json:"severity"`  // "low", "medium", "high"
	CapabilityName     string    `json:"capabilityName"`
	CapabilityType     string    `json:"capabilityType"`
	Description        string    `json:"description"`
	DetectedAt         time.Time `json:"detectedAt"`
	PreviousVerifiedAt time.Time `json:"previousVerifiedAt,omitempty"`
	IsAcknowledged     bool      `json:"isAcknowledged"`
}

// CapabilityDriftStats provides aggregate drift statistics
type CapabilityDriftStats struct {
	TotalAlerts        int `json:"totalAlerts"`
	AddedCapabilities  int `json:"addedCapabilities"`
	RemovedCapabilities int `json:"removedCapabilities"`
	HighSeverity       int `json:"highSeverity"`
	MediumSeverity     int `json:"mediumSeverity"`
	LowSeverity        int `json:"lowSeverity"`
	UnacknowledgedCount int `json:"unacknowledgedCount"`
}

// GetCapabilityDriftAlerts detects capability changes by comparing current vs recently verified capabilities
// Returns alerts for capabilities that were added (never seen before) or removed/stale (not verified recently)
// Also creates real alerts in the alerts table for the security dashboard
func (r *MCPServerCapabilityRepository) GetCapabilityDriftAlerts(orgID uuid.UUID, days int) ([]CapabilityDriftAlert, *CapabilityDriftStats, error) {
	var alerts []CapabilityDriftAlert
	stats := &CapabilityDriftStats{}

	// Query 1: Stale capabilities (not verified in last N days)
	staleQuery := `
		SELECT c.id, c.mcp_server_id, m.name, c.name, c.capability_type,
		       c.detected_at, c.last_verified_at
		FROM mcp_server_capabilities c
		JOIN mcp_servers m ON c.mcp_server_id = m.id
		WHERE m.organization_id = $1
		  AND c.is_active = true
		  AND c.last_verified_at IS NOT NULL
		  AND c.last_verified_at < NOW() - ($2 || ' days')::interval
		ORDER BY c.last_verified_at ASC
	`

	rows, err := r.db.Query(staleQuery, orgID, days)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query stale capabilities: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var capID, mcpServerID uuid.UUID
		var mcpServerName, capName, capType string
		var detectedAt sql.NullTime
		var lastVerifiedAt sql.NullTime

		if err := rows.Scan(&capID, &mcpServerID, &mcpServerName, &capName, &capType, &detectedAt, &lastVerifiedAt); err != nil {
			continue
		}

		severity := "low"
		if capType == "tool" {
			severity = "high"
		} else if capType == "resource" {
			severity = "medium"
		}

		// Use detected_at if valid, otherwise fall back to last_verified_at
		alertTime := time.Now()
		if detectedAt.Valid && !detectedAt.Time.IsZero() {
			alertTime = detectedAt.Time
		} else if lastVerifiedAt.Valid {
			alertTime = lastVerifiedAt.Time
		}

		alert := CapabilityDriftAlert{
			ID:             capID.String(),
			MCPServerID:    mcpServerID.String(),
			MCPServerName:  mcpServerName,
			CapabilityName: capName,
			CapabilityType: capType,
			DriftType:      "stale",
			Severity:       severity,
			Description:    fmt.Sprintf("Capability '%s' on %s has not been verified in %d+ days", capName, mcpServerName, days),
			DetectedAt:     alertTime,
		}
		if lastVerifiedAt.Valid {
			alert.PreviousVerifiedAt = lastVerifiedAt.Time
		}
		alerts = append(alerts, alert)

		switch severity {
		case "high":
			stats.HighSeverity++
		case "medium":
			stats.MediumSeverity++
		default:
			stats.LowSeverity++
		}
	}

	// Query 2: New capabilities (detected recently but never verified)
	addedQuery := `
		SELECT c.id, c.mcp_server_id, m.name, c.name, c.capability_type,
		       c.detected_at
		FROM mcp_server_capabilities c
		JOIN mcp_servers m ON c.mcp_server_id = m.id
		WHERE m.organization_id = $1
		  AND c.is_active = true
		  AND c.last_verified_at IS NULL
		  AND c.detected_at >= NOW() - ($2 || ' days')::interval
		ORDER BY c.detected_at DESC
	`

	rows2, err := r.db.Query(addedQuery, orgID, days)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query added capabilities: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var capID, mcpServerID uuid.UUID
		var mcpServerName, capName, capType string
		var detectedAt sql.NullTime

		if err := rows2.Scan(&capID, &mcpServerID, &mcpServerName, &capName, &capType, &detectedAt); err != nil {
			continue
		}

		// Use detected_at if valid, otherwise use current time
		alertTime := time.Now()
		if detectedAt.Valid && !detectedAt.Time.IsZero() {
			alertTime = detectedAt.Time
		}

		alert := CapabilityDriftAlert{
			ID:             capID.String(),
			MCPServerID:    mcpServerID.String(),
			MCPServerName:  mcpServerName,
			CapabilityName: capName,
			CapabilityType: capType,
			DriftType:      "added",
			Severity:       "medium",
			Description:    fmt.Sprintf("New %s capability '%s' detected on %s but not verified", capType, capName, mcpServerName),
			DetectedAt:     alertTime,
		}
		alerts = append(alerts, alert)
		stats.AddedCapabilities++
		stats.MediumSeverity++
	}

	// Query 3: Removed capabilities (marked inactive recently)
	removedQuery := `
		SELECT c.id, c.mcp_server_id, m.name, c.name, c.capability_type,
		       c.updated_at, c.last_verified_at
		FROM mcp_server_capabilities c
		JOIN mcp_servers m ON c.mcp_server_id = m.id
		WHERE m.organization_id = $1
		  AND c.is_active = false
		  AND c.updated_at >= NOW() - ($2 || ' days')::interval
		ORDER BY c.updated_at DESC
	`

	rows3, err := r.db.Query(removedQuery, orgID, days)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query removed capabilities: %w", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var capID, mcpServerID uuid.UUID
		var mcpServerName, capName, capType string
		var updatedAt time.Time
		var lastVerifiedAt sql.NullTime

		if err := rows3.Scan(&capID, &mcpServerID, &mcpServerName, &capName, &capType, &updatedAt, &lastVerifiedAt); err != nil {
			continue
		}

		alert := CapabilityDriftAlert{
			ID:             capID.String(),
			MCPServerID:    mcpServerID.String(),
			MCPServerName:  mcpServerName,
			CapabilityName: capName,
			CapabilityType: capType,
			DriftType:      "removed",
			Severity:       "high",
			Description:    fmt.Sprintf("Capability '%s' was removed or disabled on %s", capName, mcpServerName),
			DetectedAt:     updatedAt,
		}
		if lastVerifiedAt.Valid {
			alert.PreviousVerifiedAt = lastVerifiedAt.Time
		}
		alerts = append(alerts, alert)
		stats.RemovedCapabilities++
		stats.HighSeverity++
	}

	stats.TotalAlerts = len(alerts)

	// Create real alerts in the alerts table for new drift events
	if r.alertRepo != nil {
		r.createRealAlertsForDrift(orgID, alerts)
	}

	return alerts, stats, nil
}

// createRealAlertsForDrift creates alerts in the alerts table for drift events that don't already have alerts
func (r *MCPServerCapabilityRepository) createRealAlertsForDrift(orgID uuid.UUID, driftAlerts []CapabilityDriftAlert) {
	for _, drift := range driftAlerts {
		// Check if alert already exists for this capability drift
		// Use capability ID + drift type as unique identifier
		existingQuery := `
			SELECT COUNT(*) FROM alerts
			WHERE organization_id = $1
			  AND alert_type = $2
			  AND metadata->>'capabilityId' = $3
			  AND metadata->>'driftType' = $4
			  AND is_acknowledged = false
		`
		var count int
		err := r.db.QueryRow(existingQuery, orgID, domain.AlertMCPCapabilityDrift, drift.ID, drift.DriftType).Scan(&count)
		if err != nil || count > 0 {
			// Alert already exists or error checking, skip
			continue
		}

		// Map severity string to domain severity
		var severity domain.AlertSeverity
		switch drift.Severity {
		case "high":
			severity = domain.AlertSeverityHigh
		case "medium":
			severity = domain.AlertSeverityWarning
		default:
			severity = domain.AlertSeverityInfo
		}

		// Build alert title based on drift type
		var title string
		switch drift.DriftType {
		case "added":
			title = fmt.Sprintf("New MCP Capability Detected: %s", drift.CapabilityName)
		case "removed":
			title = fmt.Sprintf("MCP Capability Removed: %s", drift.CapabilityName)
		case "stale":
			title = fmt.Sprintf("MCP Capability Not Verified: %s", drift.CapabilityName)
		default:
			title = fmt.Sprintf("MCP Capability Drift: %s", drift.CapabilityName)
		}

		mcpServerID, _ := uuid.Parse(drift.MCPServerID)

		alert := &domain.Alert{
			ID:             uuid.New(),
			OrganizationID: orgID,
			AlertType:      domain.AlertMCPCapabilityDrift,
			Severity:       severity,
			Title:          title,
			Description:    drift.Description,
			ResourceType:   "mcp_server",
			ResourceID:     mcpServerID,
			AgentName:      drift.MCPServerName,
			IsAcknowledged: false,
			CreatedAt:      time.Now(),
			Metadata: map[string]interface{}{
				"capabilityId":   drift.ID,
				"capabilityName": drift.CapabilityName,
				"capabilityType": drift.CapabilityType,
				"driftType":      drift.DriftType,
				"mcpServerId":    drift.MCPServerID,
				"mcpServerName":  drift.MCPServerName,
			},
		}

		if err := r.alertRepo.Create(alert); err != nil {
			// Log but don't fail - alerts are supplementary
			fmt.Printf("Failed to create drift alert: %v\n", err)
		}
	}
}

// UpsertFromAttestation syncs capabilities discovered during attestation
// Creates new capabilities if they don't exist, or updates last_verified_at if they do
func (r *MCPServerCapabilityRepository) UpsertFromAttestation(serverID uuid.UUID, capabilityNames []string) error {
	if len(capabilityNames) == 0 {
		return nil
	}

	now := time.Now().UTC()

	for _, name := range capabilityNames {
		// Skip generic capability types (these aren't real tool names)
		if name == "tools" || name == "resources" || name == "prompts" {
			continue
		}

		// Use upsert (ON CONFLICT) to either insert new or update existing
		// Unique constraint is on (mcp_server_id, name, capability_type)
		query := `
			INSERT INTO mcp_server_capabilities (
				id, mcp_server_id, name, capability_type, description,
				detected_at, last_verified_at, is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (mcp_server_id, name, capability_type) DO UPDATE SET
				last_verified_at = EXCLUDED.last_verified_at,
				is_active = true,
				updated_at = EXCLUDED.updated_at
		`

		_, err := r.db.Exec(
			query,
			uuid.New(),                                    // id
			serverID,                                      // mcp_server_id
			name,                                          // name (e.g., "brave_web_search")
			domain.MCPCapabilityTypeTool,                  // capability_type (assume tools for attestations)
			fmt.Sprintf("Verified via agent attestation"), // description
			now,            // detected_at
			now,            // last_verified_at
			true,           // is_active
			now,            // created_at
			now,            // updated_at
		)

		if err != nil {
			return fmt.Errorf("failed to upsert capability %s: %w", name, err)
		}
	}

	return nil
}
