package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type WebhookRepository struct {
	db *sql.DB
}

func NewWebhookRepository(db *sql.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) Create(webhook *domain.Webhook) error {
	query := `
		INSERT INTO webhooks (
			id, organization_id, name, url, events, secret, is_active,
			timeout_seconds, max_retries, retry_delay_seconds,
			success_count, failure_count, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	events := make([]string, len(webhook.Events))
	for i, e := range webhook.Events {
		events[i] = string(e)
	}

	_, err := r.db.Exec(
		query,
		webhook.ID,
		webhook.OrganizationID,
		webhook.Name,
		webhook.URL,
		pq.Array(events),
		webhook.Secret,
		webhook.IsActive,
		webhook.TimeoutSeconds,
		webhook.MaxRetries,
		webhook.RetryDelaySeconds,
		webhook.SuccessCount,
		webhook.FailureCount,
		webhook.CreatedBy,
		time.Now().UTC(),
		time.Now().UTC(),
	)

	return err
}

func (r *WebhookRepository) GetByID(id uuid.UUID) (*domain.Webhook, error) {
	query := `
		SELECT id, organization_id, name, url, events, secret, is_active,
		       timeout_seconds, max_retries, retry_delay_seconds,
		       last_triggered, success_count, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE id = $1 AND deleted_at IS NULL
	`

	webhook := &domain.Webhook{}
	var events []string

	err := r.db.QueryRow(query, id).Scan(
		&webhook.ID,
		&webhook.OrganizationID,
		&webhook.Name,
		&webhook.URL,
		pq.Array(&events),
		&webhook.Secret,
		&webhook.IsActive,
		&webhook.TimeoutSeconds,
		&webhook.MaxRetries,
		&webhook.RetryDelaySeconds,
		&webhook.LastTriggered,
		&webhook.SuccessCount,
		&webhook.FailureCount,
		&webhook.CreatedBy,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("webhook not found")
	}
	if err != nil {
		return nil, err
	}

	webhook.Events = make([]domain.WebhookEvent, len(events))
	for i, e := range events {
		webhook.Events[i] = domain.WebhookEvent(e)
	}

	return webhook, nil
}

func (r *WebhookRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Webhook, error) {
	query := `
		SELECT id, organization_id, name, url, events, secret, is_active,
		       timeout_seconds, max_retries, retry_delay_seconds,
		       last_triggered, success_count, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*domain.Webhook
	for rows.Next() {
		webhook := &domain.Webhook{}
		var events []string

		err := rows.Scan(
			&webhook.ID,
			&webhook.OrganizationID,
			&webhook.Name,
			&webhook.URL,
			pq.Array(&events),
			&webhook.Secret,
			&webhook.IsActive,
			&webhook.TimeoutSeconds,
			&webhook.MaxRetries,
			&webhook.RetryDelaySeconds,
			&webhook.LastTriggered,
			&webhook.SuccessCount,
			&webhook.FailureCount,
			&webhook.CreatedBy,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		webhook.Events = make([]domain.WebhookEvent, len(events))
		for i, e := range events {
			webhook.Events[i] = domain.WebhookEvent(e)
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

func (r *WebhookRepository) Update(webhook *domain.Webhook) error {
	query := `
		UPDATE webhooks
		SET name = $1, url = $2, events = $3, is_active = $4,
		    timeout_seconds = $5, max_retries = $6, retry_delay_seconds = $7,
		    updated_at = $8
		WHERE id = $9 AND deleted_at IS NULL
	`

	events := make([]string, len(webhook.Events))
	for i, e := range webhook.Events {
		events[i] = string(e)
	}

	_, err := r.db.Exec(
		query,
		webhook.Name,
		webhook.URL,
		pq.Array(events),
		webhook.IsActive,
		webhook.TimeoutSeconds,
		webhook.MaxRetries,
		webhook.RetryDelaySeconds,
		time.Now().UTC(),
		webhook.ID,
	)

	return err
}

func (r *WebhookRepository) Delete(id uuid.UUID) error {
	// Soft delete
	query := `UPDATE webhooks SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *WebhookRepository) RecordDelivery(delivery *domain.WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			id, webhook_id, organization_id, event, payload, status, status_code,
			response_body, error_message, success, attempt_count, duration_ms,
			last_attempt_at, next_retry_at, created_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			status_code = EXCLUDED.status_code,
			response_body = EXCLUDED.response_body,
			error_message = EXCLUDED.error_message,
			success = EXCLUDED.success,
			attempt_count = EXCLUDED.attempt_count,
			duration_ms = EXCLUDED.duration_ms,
			last_attempt_at = EXCLUDED.last_attempt_at,
			next_retry_at = EXCLUDED.next_retry_at,
			completed_at = EXCLUDED.completed_at
	`

	_, err := r.db.Exec(
		query,
		delivery.ID,
		delivery.WebhookID,
		delivery.OrganizationID,
		delivery.Event,
		delivery.Payload,
		string(delivery.Status),
		delivery.StatusCode,
		delivery.ResponseBody,
		delivery.ErrorMessage,
		delivery.Success,
		delivery.AttemptCount,
		delivery.DurationMs,
		delivery.LastAttemptAt,
		delivery.NextRetryAt,
		delivery.CreatedAt,
		delivery.CompletedAt,
	)

	return err
}

func (r *WebhookRepository) GetDeliveries(webhookID uuid.UUID, limit, offset int) ([]*domain.WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, organization_id, event, payload, status, status_code,
		       response_body, error_message, success, attempt_count, duration_ms,
		       last_attempt_at, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, webhookID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*domain.WebhookDelivery
	for rows.Next() {
		delivery := &domain.WebhookDelivery{}
		var status string
		err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.OrganizationID,
			&delivery.Event,
			&delivery.Payload,
			&status,
			&delivery.StatusCode,
			&delivery.ResponseBody,
			&delivery.ErrorMessage,
			&delivery.Success,
			&delivery.AttemptCount,
			&delivery.DurationMs,
			&delivery.LastAttemptAt,
			&delivery.NextRetryAt,
			&delivery.CreatedAt,
			&delivery.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		delivery.Status = domain.WebhookDeliveryStatus(status)
		deliveries = append(deliveries, delivery)
	}

	return deliveries, nil
}

// GetDeliveryByID retrieves a specific delivery by ID
func (r *WebhookRepository) GetDeliveryByID(id uuid.UUID) (*domain.WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, organization_id, event, payload, status, status_code,
		       response_body, error_message, success, attempt_count, duration_ms,
		       last_attempt_at, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE id = $1
	`

	delivery := &domain.WebhookDelivery{}
	var status string
	err := r.db.QueryRow(query, id).Scan(
		&delivery.ID,
		&delivery.WebhookID,
		&delivery.OrganizationID,
		&delivery.Event,
		&delivery.Payload,
		&status,
		&delivery.StatusCode,
		&delivery.ResponseBody,
		&delivery.ErrorMessage,
		&delivery.Success,
		&delivery.AttemptCount,
		&delivery.DurationMs,
		&delivery.LastAttemptAt,
		&delivery.NextRetryAt,
		&delivery.CreatedAt,
		&delivery.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("delivery not found")
	}
	if err != nil {
		return nil, err
	}

	delivery.Status = domain.WebhookDeliveryStatus(status)
	return delivery, nil
}

// GetPendingDeliveries retrieves deliveries that are pending
func (r *WebhookRepository) GetPendingDeliveries(limit int) ([]*domain.WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, organization_id, event, payload, status, status_code,
		       response_body, error_message, success, attempt_count, duration_ms,
		       last_attempt_at, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`

	return r.queryDeliveries(query, limit)
}

// GetRetryableDeliveries retrieves deliveries that are ready for retry
func (r *WebhookRepository) GetRetryableDeliveries(limit int) ([]*domain.WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, organization_id, event, payload, status, status_code,
		       response_body, error_message, success, attempt_count, duration_ms,
		       last_attempt_at, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE status = 'retrying' AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1
	`

	return r.queryDeliveries(query, limit)
}

// queryDeliveries is a helper to query deliveries
func (r *WebhookRepository) queryDeliveries(query string, limit int) ([]*domain.WebhookDelivery, error) {
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*domain.WebhookDelivery
	for rows.Next() {
		delivery := &domain.WebhookDelivery{}
		var status string
		err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.OrganizationID,
			&delivery.Event,
			&delivery.Payload,
			&status,
			&delivery.StatusCode,
			&delivery.ResponseBody,
			&delivery.ErrorMessage,
			&delivery.Success,
			&delivery.AttemptCount,
			&delivery.DurationMs,
			&delivery.LastAttemptAt,
			&delivery.NextRetryAt,
			&delivery.CreatedAt,
			&delivery.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		delivery.Status = domain.WebhookDeliveryStatus(status)
		deliveries = append(deliveries, delivery)
	}

	return deliveries, nil
}

// UpdateDelivery updates a delivery record
func (r *WebhookRepository) UpdateDelivery(delivery *domain.WebhookDelivery) error {
	query := `
		UPDATE webhook_deliveries
		SET status = $1, status_code = $2, response_body = $3, error_message = $4,
		    success = $5, attempt_count = $6, duration_ms = $7, last_attempt_at = $8,
		    next_retry_at = $9, completed_at = $10
		WHERE id = $11
	`

	_, err := r.db.Exec(
		query,
		string(delivery.Status),
		delivery.StatusCode,
		delivery.ResponseBody,
		delivery.ErrorMessage,
		delivery.Success,
		delivery.AttemptCount,
		delivery.DurationMs,
		delivery.LastAttemptAt,
		delivery.NextRetryAt,
		delivery.CompletedAt,
		delivery.ID,
	)

	return err
}

// UpdateWebhookStats updates webhook statistics after a delivery
func (r *WebhookRepository) UpdateWebhookStats(webhookID uuid.UUID, success bool, deliveredAt time.Time) error {
	var query string
	if success {
		query = `
			UPDATE webhooks
			SET last_triggered = $1, last_delivery_status = 'success',
			    success_count = success_count + 1, updated_at = NOW()
			WHERE id = $2
		`
	} else {
		query = `
			UPDATE webhooks
			SET last_triggered = $1, last_delivery_status = 'failed',
			    failure_count = failure_count + 1, updated_at = NOW()
			WHERE id = $2
		`
	}

	_, err := r.db.Exec(query, deliveredAt, webhookID)
	return err
}

// GetActiveByEvent retrieves active webhooks subscribed to a specific event
func (r *WebhookRepository) GetActiveByEvent(orgID uuid.UUID, event domain.WebhookEvent) ([]*domain.Webhook, error) {
	query := `
		SELECT id, organization_id, name, url, events, secret, is_active,
		       timeout_seconds, max_retries, retry_delay_seconds,
		       last_triggered, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE organization_id = $1
		  AND is_active = true
		  AND $2 = ANY(events)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	return r.queryWebhooks(query, orgID, string(event))
}

// GetByEvent retrieves all webhooks subscribed to a specific event (active or not)
func (r *WebhookRepository) GetByEvent(orgID uuid.UUID, event domain.WebhookEvent) ([]*domain.Webhook, error) {
	query := `
		SELECT id, organization_id, name, url, events, secret, is_active,
		       timeout_seconds, max_retries, retry_delay_seconds,
		       last_triggered, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE organization_id = $1
		  AND $2 = ANY(events)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	return r.queryWebhooks(query, orgID, string(event))
}

// queryWebhooks is a helper to query webhooks
func (r *WebhookRepository) queryWebhooks(query string, args ...interface{}) ([]*domain.Webhook, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*domain.Webhook
	for rows.Next() {
		webhook := &domain.Webhook{}
		var events []string

		err := rows.Scan(
			&webhook.ID,
			&webhook.OrganizationID,
			&webhook.Name,
			&webhook.URL,
			pq.Array(&events),
			&webhook.Secret,
			&webhook.IsActive,
			&webhook.TimeoutSeconds,
			&webhook.MaxRetries,
			&webhook.RetryDelaySeconds,
			&webhook.LastTriggered,
			&webhook.FailureCount,
			&webhook.CreatedBy,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		webhook.Events = make([]domain.WebhookEvent, len(events))
		for i, e := range events {
			webhook.Events[i] = domain.WebhookEvent(e)
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

// CountByOrganization counts webhooks for an organization
func (r *WebhookRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM webhooks WHERE organization_id = $1 AND deleted_at IS NULL`
	var count int
	err := r.db.QueryRow(query, orgID).Scan(&count)
	return count, err
}

// DeleteOldDeliveries removes old delivery records
func (r *WebhookRepository) DeleteOldDeliveries(olderThan time.Duration) (int64, error) {
	query := `DELETE FROM webhook_deliveries WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
