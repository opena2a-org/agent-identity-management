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

func (r *AlertRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM alerts WHERE organization_id = $1`
	var total int
	err := r.db.QueryRow(query, orgID).Scan(&total)
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
