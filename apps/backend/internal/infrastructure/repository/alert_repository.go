package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// alertExecer is what insertAlert needs: *sql.DB or *sql.Tx.
type alertExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (r *AlertRepository) Create(alert *domain.Alert) error {
	return insertAlert(r.db, alert)
}

func insertAlert(exec alertExecer, alert *domain.Alert) error {
	query := `
		INSERT INTO alerts (id, organization_id, alert_type, severity, title, description, resource_type, resource_id, audit_id, agent_name, source_ip, metadata, is_acknowledged, created_at, dedupe_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}

	// Convert metadata to JSON
	metadataJSON := make([]byte, 0)
	var err error
	if alert.Metadata != nil {
		metadataJSON, err = json.Marshal(alert.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Handle empty source IP
	var sourceIP *string
	if alert.SourceIP != "" {
		sourceIP = &alert.SourceIP
	}

	// An alert without a dedupe key stores NULL and never coalesces.
	var dedupeKey *string
	if alert.DedupeKey != "" {
		dedupeKey = &alert.DedupeKey
	}

	_, err = exec.Exec(query,
		alert.ID,
		alert.OrganizationID,
		alert.AlertType,
		alert.Severity,
		alert.Title,
		alert.Description,
		alert.ResourceType,
		alert.ResourceID,
		alert.AuditID,
		alert.AgentName,
		sourceIP,
		metadataJSON,
		alert.IsAcknowledged,
		alert.CreatedAt,
		dedupeKey,
	)
	if err != nil {
		return err
	}
	// The row's occurrence_count starts at 1.
	alert.OccurrenceCount = 1
	return nil
}

func (r *AlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts
		WHERE id = $1
	`

	alert := &domain.Alert{}
	metadataJSON := make([]byte, 0)
	var agentName sql.NullString
	var sourceIP sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&alert.ID,
		&alert.OrganizationID,
		&alert.AlertType,
		&alert.Severity,
		&alert.Title,
		&alert.Description,
		&alert.ResourceType,
		&alert.ResourceID,
		&alert.AuditID,
		&agentName,
		&sourceIP,
		&metadataJSON,
		&alert.IsAcknowledged,
		&alert.AcknowledgedBy,
		&alert.AcknowledgedAt,
		&alert.CreatedAt,
		&alert.OccurrenceCount,
		&alert.LastSeenAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("alert not found")
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable agent name
	if agentName.Valid {
		alert.AgentName = agentName.String
	}

	// Handle nullable source IP
	if sourceIP.Valid {
		alert.SourceIP = sourceIP.String
	}

	// Parse metadata JSON
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &alert.Metadata); err != nil {
			alert.Metadata = make(map[string]interface{})
		}
	}

	return alert, nil
}

func (r *AlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// GetByOrganizationFiltered retrieves alerts with optional status filtering
func (r *AlertRepository) GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*domain.Alert, error) {
	var query string
	var args []interface{}

	baseSelect := `SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts`

	if status == "acknowledged" {
		query = baseSelect + `
			WHERE organization_id = $1 AND is_acknowledged = true
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	} else if status == "unacknowledged" {
		query = baseSelect + `
			WHERE organization_id = $1 AND is_acknowledged = false
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	} else {
		// Return all alerts (no status filter)
		query = baseSelect + `
			WHERE organization_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

func (r *AlertRepository) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts
		WHERE organization_id = $1 AND is_acknowledged = false
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

func (r *AlertRepository) Acknowledge(id, userID uuid.UUID) error {
	query := `
		UPDATE alerts
		SET is_acknowledged = true, acknowledged_by = $1, acknowledged_at = $2
		WHERE id = $3
	`

	now := time.Now()
	_, err := r.db.Exec(query, userID, now, id)
	return err
}

func (r *AlertRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM alerts WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// CreateCoalesced counts a repeat on the newest unacknowledged alert in the
// alert's organization with the same dedupe key created at or after since, or
// inserts the alert when there is none, and reports whether it inserted.
//
// The lookup and the insert run in one transaction that first takes a
// transaction-scoped advisory lock on the organization and key. Without it,
// two first occurrences arriving together each find no open alert and each
// insert one. Under READ COMMITTED every statement reads what was committed
// when it starts, so the lookup after the lock sees the alert the previous
// holder inserted.
func (r *AlertRepository) CreateCoalesced(alert *domain.Alert, since, seenAt time.Time) (bool, error) {
	if alert.DedupeKey == "" {
		return false, errors.New("CreateCoalesced: alert has no dedupe key")
	}

	tx, err := r.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`SELECT pg_advisory_xact_lock(hashtext('alert_dedupe'), hashtext($1))`,
		alert.OrganizationID.String()+":"+alert.DedupeKey,
	); err != nil {
		return false, fmt.Errorf("failed to lock alert dedupe key: %w", err)
	}

	var openID uuid.UUID
	err = tx.QueryRow(`
		SELECT id
		FROM alerts
		WHERE organization_id = $1 AND dedupe_key = $2 AND is_acknowledged = false AND created_at >= $3
		ORDER BY created_at DESC
		LIMIT 1
	`, alert.OrganizationID, alert.DedupeKey, since).Scan(&openID)
	switch {
	case err == nil:
		if _, err := tx.Exec(`
			UPDATE alerts
			SET occurrence_count = occurrence_count + 1, last_seen_at = $1
			WHERE id = $2
		`, seenAt, openID); err != nil {
			return false, err
		}
		return false, tx.Commit()
	case !errors.Is(err, sql.ErrNoRows):
		return false, err
	}

	if err := insertAlert(tx, alert); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// BulkAcknowledge updates all alerts for an org in one query
func (r *AlertRepository) BulkAcknowledge(orgID uuid.UUID, userID uuid.UUID) (int, error) {
	query := `
		UPDATE alerts
		SET is_acknowledged = true, acknowledged_by = $1, acknowledged_at = $2
		WHERE organization_id = $3 AND is_acknowledged = false
	`

	result, err := r.db.Exec(query, userID, time.Now(), orgID)
	if err != nil {
		return 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rows), nil
}

func (r *AlertRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM alerts WHERE organization_id = $1`
	var total int
	err := r.db.QueryRow(query, orgID).Scan(&total)
	return total, err
}

// CountByOrganizationFiltered counts alerts with optional status filtering
func (r *AlertRepository) CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error) {
	var query string
	var args []interface{}

	if status == "acknowledged" {
		query = `SELECT COUNT(*) FROM alerts WHERE organization_id = $1 AND is_acknowledged = true`
		args = []interface{}{orgID}
	} else if status == "unacknowledged" {
		query = `SELECT COUNT(*) FROM alerts WHERE organization_id = $1 AND is_acknowledged = false`
		args = []interface{}{orgID}
	} else {
		// Count all alerts (no status filter)
		query = `SELECT COUNT(*) FROM alerts WHERE organization_id = $1`
		args = []interface{}{orgID}
	}

	var total int
	err := r.db.QueryRow(query, args...).Scan(&total)
	return total, err
}

// CountBySeverity counts alerts grouped by severity level with optional status filtering
func (r *AlertRepository) CountBySeverity(orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	var query string
	var args []interface{}

	if status == "acknowledged" {
		query = `
			SELECT
				COUNT(*) FILTER (WHERE severity = 'critical') AS critical_count,
				COUNT(*) FILTER (WHERE severity = 'high') AS high_count,
				COUNT(*) FILTER (WHERE severity = 'warning') AS warning_count,
				COUNT(*) FILTER (WHERE severity = 'info') AS info_count
			FROM alerts
			WHERE organization_id = $1 AND is_acknowledged = true
		`
		args = []interface{}{orgID}
	} else if status == "unacknowledged" {
		query = `
			SELECT
				COUNT(*) FILTER (WHERE severity = 'critical') AS critical_count,
				COUNT(*) FILTER (WHERE severity = 'high') AS high_count,
				COUNT(*) FILTER (WHERE severity = 'warning') AS warning_count,
				COUNT(*) FILTER (WHERE severity = 'info') AS info_count
			FROM alerts
			WHERE organization_id = $1 AND is_acknowledged = false
		`
		args = []interface{}{orgID}
	} else {
		query = `
			SELECT
				COUNT(*) FILTER (WHERE severity = 'critical') AS critical_count,
				COUNT(*) FILTER (WHERE severity = 'high') AS high_count,
				COUNT(*) FILTER (WHERE severity = 'warning') AS warning_count,
				COUNT(*) FILTER (WHERE severity = 'info') AS info_count
			FROM alerts
			WHERE organization_id = $1
		`
		args = []interface{}{orgID}
	}

	err = r.db.QueryRow(query, args...).Scan(&critical, &high, &warning, &info)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return critical, high, warning, info, nil
}

func (r *AlertRepository) GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts
		WHERE resource_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, resourceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

func (r *AlertRepository) GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id,
		       audit_id, agent_name, source_ip, COALESCE(metadata, '{}'), is_acknowledged, acknowledged_by, acknowledged_at, created_at,
		       occurrence_count, last_seen_at
		FROM alerts
		WHERE resource_id = $1 AND is_acknowledged = false
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

func (r *AlertRepository) scanAlerts(rows *sql.Rows) ([]*domain.Alert, error) {
	alerts := make([]*domain.Alert, 0)

	for rows.Next() {
		alert := &domain.Alert{}
		metadataJSON := make([]byte, 0)
		var agentName sql.NullString
		var sourceIP sql.NullString
		err := rows.Scan(
			&alert.ID,
			&alert.OrganizationID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Title,
			&alert.Description,
			&alert.ResourceType,
			&alert.ResourceID,
			&alert.AuditID,
			&agentName,
			&sourceIP,
			&metadataJSON,
			&alert.IsAcknowledged,
			&alert.AcknowledgedBy,
			&alert.AcknowledgedAt,
			&alert.CreatedAt,
			&alert.OccurrenceCount,
			&alert.LastSeenAt,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable agent name
		if agentName.Valid {
			alert.AgentName = agentName.String
		}

		// Handle nullable source IP
		if sourceIP.Valid {
			alert.SourceIP = sourceIP.String
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &alert.Metadata); err != nil {
				alert.Metadata = make(map[string]interface{})
			}
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}
