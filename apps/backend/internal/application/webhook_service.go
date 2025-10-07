package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type WebhookService struct {
	webhookRepo *repository.WebhookRepository
}

func NewWebhookService(webhookRepo *repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		webhookRepo: webhookRepo,
	}
}

// CreateWebhookRequest represents the request to create a webhook
type CreateWebhookRequest struct {
	Name   string                 `json:"name" validate:"required"`
	URL    string                 `json:"url" validate:"required,url"`
	Events []domain.WebhookEvent  `json:"events" validate:"required"`
}

// CreateWebhook creates a new webhook subscription
func (s *WebhookService) CreateWebhook(ctx context.Context, req *CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
	// Generate secret for webhook signature
	secret, err := generateSecret()
	if err != nil {
		return nil, err
	}

	webhook := &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           req.Name,
		URL:            req.URL,
		Events:         req.Events,
		Secret:         secret,
		IsActive:       true,
		FailureCount:   0,
		CreatedBy:      userID,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.webhookRepo.Create(webhook); err != nil {
		return nil, err
	}

	return webhook, nil
}

// ListWebhooks lists all webhooks for an organization
func (s *WebhookService) ListWebhooks(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error) {
	return s.webhookRepo.GetByOrganization(orgID)
}

// GetWebhook retrieves a webhook by ID
func (s *WebhookService) GetWebhook(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	return s.webhookRepo.GetByID(id)
}

// DeleteWebhook deletes a webhook
func (s *WebhookService) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	return s.webhookRepo.Delete(id)
}

// TestWebhook sends a test payload to a webhook
func (s *WebhookService) TestWebhook(ctx context.Context, id uuid.UUID) error {
	webhook, err := s.webhookRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Create test payload
	payload := map[string]interface{}{
		"event":      "webhook.test",
		"webhook_id": webhook.ID.String(),
		"timestamp":  time.Now().UTC(),
		"data": map[string]string{
			"message": "This is a test webhook delivery",
		},
	}

	return s.sendWebhook(webhook, "webhook.test", payload)
}

// sendWebhook sends a webhook payload
func (s *WebhookService) sendWebhook(webhook *domain.Webhook, event string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create signature
	signature := createSignature(jsonData, webhook.Secret)

	// Send HTTP request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", event)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)

	// Record delivery
	delivery := &domain.WebhookDelivery{
		ID:           uuid.New(),
		WebhookID:    webhook.ID,
		Event:        domain.WebhookEvent(event),
		Payload:      string(jsonData),
		StatusCode:   resp.StatusCode,
		ResponseBody: string(body),
		Success:      resp.StatusCode >= 200 && resp.StatusCode < 300,
		AttemptCount: 1,
		CreatedAt:    time.Now().UTC(),
	}

	s.webhookRepo.RecordDelivery(delivery)

	if !delivery.Success {
		return fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}

	return nil
}

// Helper functions

func generateSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
