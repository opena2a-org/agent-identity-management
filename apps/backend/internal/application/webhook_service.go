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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// WebhookConfig holds webhook service configuration
type WebhookConfig struct {
	DefaultTimeout       time.Duration
	DefaultMaxRetries    int
	DefaultRetryDelay    time.Duration
	MaxRetryDelay        time.Duration
	RetryBackoffFactor   float64
	DeliveryWorkerCount  int
	RetryWorkerInterval  time.Duration
}

// DefaultWebhookConfig returns sensible defaults
func DefaultWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		DefaultTimeout:       30 * time.Second,
		DefaultMaxRetries:    3,
		DefaultRetryDelay:    60 * time.Second,
		MaxRetryDelay:        30 * time.Minute,
		RetryBackoffFactor:   2.0,
		DeliveryWorkerCount:  5,
		RetryWorkerInterval:  30 * time.Second,
	}
}

type WebhookService struct {
	webhookRepo *repository.WebhookRepository
	config      *WebhookConfig
	stopChan    chan struct{}
}

func NewWebhookService(webhookRepo *repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		webhookRepo: webhookRepo,
		config:      DefaultWebhookConfig(),
		stopChan:    make(chan struct{}),
	}
}

// SetConfig allows overriding default configuration
func (s *WebhookService) SetConfig(config *WebhookConfig) {
	s.config = config
}

// CreateWebhookRequest represents the request to create a webhook
type CreateWebhookRequest struct {
	Name     string                `json:"name" validate:"required"`
	URL      string                `json:"url" validate:"required,url"`
	Events   []domain.WebhookEvent `json:"events" validate:"required"`
	IsActive *bool                 `json:"isActive,omitempty"` // Pointer to distinguish between false and not provided
}

// CreateWebhook creates a new webhook subscription
func (s *WebhookService) CreateWebhook(ctx context.Context, req *CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
	// SECURITY: Validate URL to prevent SSRF attacks
	if err := utils.ValidateExternalURL(req.URL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

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

// UpdateWebhook updates an existing webhook
func (s *WebhookService) UpdateWebhook(ctx context.Context, id uuid.UUID, req *CreateWebhookRequest) (*domain.Webhook, error) {
	// SECURITY: Validate URL to prevent SSRF attacks
	if err := utils.ValidateExternalURL(req.URL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Get existing webhook
	webhook, err := s.webhookRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields
	webhook.Name = req.Name
	webhook.URL = req.URL
	webhook.Events = req.Events

	// Update IsActive if provided
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
	}

	webhook.UpdatedAt = time.Now().UTC()

	// Save changes
	if err := s.webhookRepo.Update(webhook); err != nil {
		return nil, err
	}

	return webhook, nil
}

// WebhookTestResult contains the result of a webhook test
type WebhookTestResult struct {
	Success      bool
	StatusCode   int
	ErrorMessage string
}

// TestWebhook sends a test payload to a webhook
func (s *WebhookService) TestWebhook(ctx context.Context, id uuid.UUID) (*WebhookTestResult, error) {
	webhook, err := s.webhookRepo.GetByID(id)
	if err != nil {
		return nil, err
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

	// Send webhook and capture result
	statusCode, deliveryErr := s.sendWebhookWithResult(webhook, "webhook.test", payload)

	result := &WebhookTestResult{
		Success:    statusCode >= 200 && statusCode < 300,
		StatusCode: statusCode,
	}

	if deliveryErr != nil {
		result.ErrorMessage = deliveryErr.Error()
	}

	return result, nil
}

// sendWebhookWithResult sends a webhook payload and returns status code and error
func (s *WebhookService) sendWebhookWithResult(webhook *domain.Webhook, event string, payload interface{}) (int, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	// Create signature
	signature := createSignature(jsonData, webhook.Secret)

	// Send HTTP request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", event)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
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
		return resp.StatusCode, fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// sendWebhook sends a webhook payload
func (s *WebhookService) sendWebhook(webhook *domain.Webhook, event string, payload interface{}) error {
	_, err := s.sendWebhookWithResult(webhook, event, payload)
	return err
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

// TriggerEvent triggers webhooks for a specific event
func (s *WebhookService) TriggerEvent(ctx context.Context, orgID uuid.UUID, event domain.WebhookEvent, data map[string]interface{}) error {
	// Get all active webhooks subscribed to this event
	webhooks, err := s.webhookRepo.GetByOrganization(orgID)
	if err != nil {
		return err
	}

	// Filter webhooks that are active and subscribed to this event
	matchingWebhooks := make([]*domain.Webhook, 0)
	for _, wh := range webhooks {
		if !wh.IsActive {
			continue
		}
		for _, e := range wh.Events {
			if e == event {
				matchingWebhooks = append(matchingWebhooks, wh)
				break
			}
		}
	}

	if len(matchingWebhooks) == 0 {
		return nil // No webhooks to trigger
	}

	// Create payload
	payload := &domain.WebhookPayload{
		ID:             uuid.New().String(),
		Event:          event,
		Timestamp:      time.Now().UTC(),
		OrganizationID: orgID.String(),
		Data:           data,
	}

	// Send to each webhook asynchronously
	for _, webhook := range matchingWebhooks {
		go s.deliverWithRetry(webhook, payload)
	}

	return nil
}

// deliverWithRetry attempts to deliver a webhook with retry logic
func (s *WebhookService) deliverWithRetry(webhook *domain.Webhook, payload *domain.WebhookPayload) {
	maxRetries := webhook.MaxRetries
	if maxRetries == 0 {
		maxRetries = s.config.DefaultMaxRetries
	}

	retryDelay := time.Duration(webhook.RetryDelaySeconds) * time.Second
	if retryDelay == 0 {
		retryDelay = s.config.DefaultRetryDelay
	}

	timeout := time.Duration(webhook.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = s.config.DefaultTimeout
	}

	// Create initial delivery record
	delivery := &domain.WebhookDelivery{
		ID:             uuid.New(),
		WebhookID:      webhook.ID,
		OrganizationID: webhook.OrganizationID,
		Event:          payload.Event,
		Status:         domain.DeliveryStatusPending,
		AttemptCount:   0,
		CreatedAt:      time.Now().UTC(),
	}

	// Marshal payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		delivery.Status = domain.DeliveryStatusFailed
		delivery.ErrorMessage = fmt.Sprintf("failed to marshal payload: %v", err)
		s.webhookRepo.RecordDelivery(delivery)
		return
	}
	delivery.Payload = string(jsonData)

	// Attempt delivery with retries
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		delivery.AttemptCount = attempt
		now := time.Now().UTC()
		delivery.LastAttemptAt = &now

		statusCode, responseBody, duration, deliveryErr := s.attemptDelivery(webhook, jsonData, timeout)
		delivery.StatusCode = statusCode
		delivery.ResponseBody = responseBody
		delivery.DurationMs = duration.Milliseconds()

		if deliveryErr == nil && statusCode >= 200 && statusCode < 300 {
			// Success
			delivery.Status = domain.DeliveryStatusSuccess
			delivery.Success = true
			completedAt := time.Now().UTC()
			delivery.CompletedAt = &completedAt
			s.webhookRepo.RecordDelivery(delivery)
			s.webhookRepo.UpdateWebhookStats(webhook.ID, true, completedAt)
			return
		}

		// Failed
		if deliveryErr != nil {
			delivery.ErrorMessage = deliveryErr.Error()
		} else {
			delivery.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
		}

		if attempt <= maxRetries {
			// Will retry
			delivery.Status = domain.DeliveryStatusRetrying
			nextRetry := time.Now().Add(retryDelay)
			delivery.NextRetryAt = &nextRetry

			// Exponential backoff
			retryDelay = time.Duration(float64(retryDelay) * s.config.RetryBackoffFactor)
			if retryDelay > s.config.MaxRetryDelay {
				retryDelay = s.config.MaxRetryDelay
			}

			s.webhookRepo.RecordDelivery(delivery)

			// Wait before retry
			time.Sleep(retryDelay)
		}
	}

	// All retries exhausted
	delivery.Status = domain.DeliveryStatusAbandoned
	delivery.Success = false
	completedAt := time.Now().UTC()
	delivery.CompletedAt = &completedAt
	s.webhookRepo.RecordDelivery(delivery)
	s.webhookRepo.UpdateWebhookStats(webhook.ID, false, completedAt)
}

// attemptDelivery makes a single delivery attempt
func (s *WebhookService) attemptDelivery(webhook *domain.Webhook, payload []byte, timeout time.Duration) (int, string, time.Duration, error) {
	start := time.Now()

	// Create signature
	signature := createSignature(payload, webhook.Secret)

	// Create request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return 0, "", time.Since(start), err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", string(webhook.Events[0])) // Primary event
	req.Header.Set("X-Webhook-ID", webhook.ID.String())
	req.Header.Set("User-Agent", "AIM-Webhook/1.0")

	// Send request
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return 0, "", duration, err
	}
	defer resp.Body.Close()

	// Read response (limit to 1KB to prevent memory issues)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return resp.StatusCode, string(body), duration, nil
}

// GetDeliveries retrieves delivery history for a webhook
func (s *WebhookService) GetDeliveries(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]*domain.WebhookDelivery, error) {
	return s.webhookRepo.GetDeliveries(webhookID, limit, offset)
}

// ReplayDelivery replays a specific delivery
func (s *WebhookService) ReplayDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := s.webhookRepo.GetDeliveryByID(deliveryID)
	if err != nil {
		return err
	}

	webhook, err := s.webhookRepo.GetByID(delivery.WebhookID)
	if err != nil {
		return err
	}

	// Parse the original payload
	var payload domain.WebhookPayload
	if err := json.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
		return fmt.Errorf("failed to parse original payload: %w", err)
	}

	// Update delivery ID to make it unique
	payload.ID = uuid.New().String()
	payload.Timestamp = time.Now().UTC()

	// Redeliver
	go s.deliverWithRetry(webhook, &payload)

	return nil
}
