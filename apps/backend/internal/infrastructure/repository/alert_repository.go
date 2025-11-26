package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

func (r *AlertRepository) Create(alert *domain.Alert) error {
	query := `
		INSERT INTO alerts (id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}

	_, err := r.db.Exec(query,
		alert.ID,
		alert.OrganizationID,
		alert.AlertType,
		alert.Severity,
		alert.Title,
		alert.Description,
		alert.ResourceType,
		alert.ResourceID,
		alert.IsAcknowledged,
		alert.CreatedAt,
	)
	return err
}

func (r *AlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
		FROM alerts
		WHERE id = $1
	`

	alert := &domain.Alert{}
	err := r.db.QueryRow(query, id).Scan(
		&alert.ID,
		&alert.OrganizationID,
		&alert.AlertType,
		&alert.Severity,
		&alert.Title,
		&alert.Description,
		&alert.ResourceType,
		&alert.ResourceID,
		&alert.IsAcknowledged,
		&alert.AcknowledgedBy,
		&alert.AcknowledgedAt,
		&alert.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("alert not found")
	}
	return alert, err
}

func (r *AlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	query := `
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
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

	if status == "acknowledged" {
		query = `
			SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
			FROM alerts
			WHERE organization_id = $1 AND is_acknowledged = true
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	} else if status == "unacknowledged" {
		query = `
			SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
			FROM alerts
			WHERE organization_id = $1 AND is_acknowledged = false
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{orgID, limit, offset}
	} else {
		// Return all alerts (no status filter)
		query = `
			SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
			FROM alerts
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
		SELECT id, organization_id, alert_type, severity, title, description, resource_type, resource_id, is_acknowledged, acknowledged_by, acknowledged_at, created_at
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

func (r *AlertRepository) scanAlerts(rows *sql.Rows) ([]*domain.Alert, error) {
	var alerts []*domain.Alert

	for rows.Next() {
		alert := &domain.Alert{}
		err := rows.Scan(
			&alert.ID,
			&alert.OrganizationID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Title,
			&alert.Description,
			&alert.ResourceType,
			&alert.ResourceID,
			&alert.IsAcknowledged,
			&alert.AcknowledgedBy,
			&alert.AcknowledgedAt,
			&alert.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}
