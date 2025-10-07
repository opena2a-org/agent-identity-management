package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a/identity/backend/internal/domain"
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
			id, organization_id, name, url, events, secret, is_active, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
		webhook.CreatedBy,
		time.Now().UTC(),
		time.Now().UTC(),
	)

	return err
}

func (r *WebhookRepository) GetByID(id uuid.UUID) (*domain.Webhook, error) {
	query := `
		SELECT id, organization_id, name, url, events, secret, is_active, last_triggered, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE id = $1
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
		&webhook.LastTriggered,
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
		SELECT id, organization_id, name, url, events, secret, is_active, last_triggered, failure_count, created_by, created_at, updated_at
		FROM webhooks
		WHERE organization_id = $1
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

func (r *WebhookRepository) Update(webhook *domain.Webhook) error {
	query := `
		UPDATE webhooks
		SET name = $1, url = $2, events = $3, is_active = $4, updated_at = $5
		WHERE id = $6
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
		time.Now().UTC(),
		webhook.ID,
	)

	return err
}

func (r *WebhookRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM webhooks WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *WebhookRepository) RecordDelivery(delivery *domain.WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			id, webhook_id, event, payload, status_code, response_body, success, attempt_count, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(
		query,
		delivery.ID,
		delivery.WebhookID,
		delivery.Event,
		delivery.Payload,
		delivery.StatusCode,
		delivery.ResponseBody,
		delivery.Success,
		delivery.AttemptCount,
		time.Now().UTC(),
	)

	return err
}

func (r *WebhookRepository) GetDeliveries(webhookID uuid.UUID, limit, offset int) ([]*domain.WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, event, payload, status_code, response_body, success, attempt_count, created_at
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
		err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.Event,
			&delivery.Payload,
			&delivery.StatusCode,
			&delivery.ResponseBody,
			&delivery.Success,
			&delivery.AttemptCount,
			&delivery.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}

	return deliveries, nil
}
