package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// WebhookConfig holds webhook service configuration. The limits on pending
// deliveries and on attempts in flight are read the first time the service
// delivers an event or stops; a limit left at zero takes its default.
type WebhookConfig struct {
	DefaultTimeout     time.Duration
	DefaultMaxRetries  int
	DefaultRetryDelay  time.Duration
	MaxRetryDelay      time.Duration
	RetryBackoffFactor float64
	// DeliveryWorkerCount is the most event delivery attempts in flight at
	// once, across webhooks.
	DeliveryWorkerCount int
	RetryWorkerInterval time.Duration
	// MaxRetries is the most retries an event delivery makes, whatever its
	// webhook stores.
	MaxRetries int
	// MaxTimeout is the longest one attempt waits for its endpoint, whatever
	// its webhook stores.
	MaxTimeout time.Duration
	// MaxPendingDeliveries is the most event deliveries held at once, sending
	// or waiting to retry. A delivery beyond it is not sent and is recorded as
	// abandoned.
	MaxPendingDeliveries int
	// MaxPendingPerWebhook is the most of those deliveries one webhook holds.
	MaxPendingPerWebhook int
	// MaxAttemptsPerWebhook is the most attempts in flight to one webhook at
	// once, so a slow endpoint does not hold every attempt slot.
	MaxAttemptsPerWebhook int
}

// DefaultWebhookConfig returns sensible defaults
func DefaultWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		DefaultTimeout:        30 * time.Second,
		DefaultMaxRetries:     3,
		DefaultRetryDelay:     60 * time.Second,
		MaxRetryDelay:         30 * time.Minute,
		RetryBackoffFactor:    2.0,
		DeliveryWorkerCount:   defaultMaxAttemptsInFlight,
		RetryWorkerInterval:   30 * time.Second,
		MaxRetries:            defaultMaxRetriesCeiling,
		MaxTimeout:            30 * time.Second,
		MaxPendingDeliveries:  defaultMaxPendingDeliveries,
		MaxPendingPerWebhook:  defaultMaxPendingPerWebhook,
		MaxAttemptsPerWebhook: defaultMaxAttemptsPerWebhook,
	}
}

// maxStoredResponseBody is how much of an endpoint's response a delivery keeps.
const maxStoredResponseBody = 1024

type WebhookService struct {
	webhookRepo *repository.WebhookRepository
	config      *WebhookConfig
	stopChan    chan struct{}
	// newHTTPClient builds the client a delivery is sent with. Endpoints are
	// tenant-supplied, so it is the egress client: redirects are not followed and
	// the address of every connection is checked when it is opened.
	newHTTPClient func(timeout time.Duration) *http.Client
	// newRetryTimer starts the wait before a retry.
	newRetryTimer func(d time.Duration) *time.Timer

	boundsOnce     sync.Once
	deliveryBounds *deliveryBounds
	stopOnce       sync.Once
}

func NewWebhookService(webhookRepo *repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		webhookRepo:   webhookRepo,
		config:        DefaultWebhookConfig(),
		stopChan:      make(chan struct{}),
		newHTTPClient: utils.NewEgressClient,
		newRetryTimer: time.NewTimer,
	}
}

// bounds returns the delivery bounds, built from the config the first time an
// event delivery asks for them.
func (s *WebhookService) bounds() *deliveryBounds {
	s.boundsOnce.Do(func() {
		s.deliveryBounds = newDeliveryBounds(s.config)
	})
	return s.deliveryBounds
}

// Stop refuses every later event delivery, ends the deliveries waiting for an
// attempt slot or a retry, and waits up to timeout for the deliveries in hand
// to end. A delivery it ends is recorded as abandoned. An attempt already
// sent runs until its endpoint answers or its timeout ends. Stop returns
// whether every delivery ended within timeout; it may be called more than once.
func (s *WebhookService) Stop(timeout time.Duration) bool {
	drained := s.bounds().stop()
	s.stopOnce.Do(func() { close(s.stopChan) })
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-drained:
		return true
	case <-timer.C:
		return false
	}
}

func (s *WebhookService) httpClient(timeout time.Duration) *http.Client {
	if s.newHTTPClient == nil {
		return utils.NewEgressClient(timeout)
	}
	return s.newHTTPClient(timeout)
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
	// Secret is the secret a create stores to sign the webhook's deliveries,
	// MinWebhookSecretLength to MaxWebhookSecretLength characters. When it is
	// empty, CreateWebhook generates one. An update does not take it: the
	// secret changes only through RotateWebhookSecret.
	Secret string `json:"secret,omitempty"`
}

// MinWebhookSecretLength is the fewest characters a caller-chosen webhook
// secret has: an HMAC key at least as long as the SHA-256 output, as RFC 2104
// recommends. MaxWebhookSecretLength is the most the secret column stores.
const (
	MinWebhookSecretLength = 32
	MaxWebhookSecretLength = 255
)

// WebhookRequestError is returned by a webhook create or update the server
// refuses for what the request asks: an event the server does not know, or a
// secret outside the lengths it takes. Reason is said to the caller.
type WebhookRequestError struct {
	Reason string
}

func (e *WebhookRequestError) Error() string { return e.Reason }

// checkWebhookEvents refuses an event that is not one of
// domain.AllWebhookEvents, naming it: a webhook subscribed to it would never
// receive a delivery.
func checkWebhookEvents(events []domain.WebhookEvent) error {
	for _, event := range events {
		if !domain.IsKnownWebhookEvent(event) {
			return &WebhookRequestError{Reason: fmt.Sprintf("unsupported event: %s", event)}
		}
	}
	return nil
}

// checkWebhookSecret refuses a caller-chosen secret shorter than
// MinWebhookSecretLength or longer than MaxWebhookSecretLength characters.
func checkWebhookSecret(secret string) error {
	length := utf8.RuneCountInString(secret)
	if length < MinWebhookSecretLength {
		return &WebhookRequestError{Reason: fmt.Sprintf("secret must be at least %d characters", MinWebhookSecretLength)}
	}
	if length > MaxWebhookSecretLength {
		return &WebhookRequestError{Reason: fmt.Sprintf("secret must be at most %d characters", MaxWebhookSecretLength)}
	}
	return nil
}

// ErrWebhookURLRefused is wrapped by a webhook create or update whose URL the
// server refuses: a URL that does not parse, is not http or https or has no
// host, a host the resolver reports does not exist, or an address the egress
// address policy refuses. The rest of the error says why; it is for the server
// log, not for the caller.
var ErrWebhookURLRefused = errors.New("invalid webhook URL")

// checkWebhookURL checks a webhook URL with ValidateExternalURL. A lookup that
// failed for any reason but the name not existing (a timeout, a resolver that
// did not answer) says nothing about the URL, so it is not a refusal.
func checkWebhookURL(rawURL string) error {
	err := utils.ValidateExternalURL(rawURL)
	if err == nil {
		return nil
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsNotFound {
		return fmt.Errorf("webhook URL host lookup failed: %w", err)
	}
	return fmt.Errorf("%w: %w", ErrWebhookURLRefused, err)
}

// CreateWebhook creates a new webhook subscription. Its deliveries are signed
// with req.Secret when the caller chose one, and with a generated secret
// otherwise; the returned webhook holds that secret.
func (s *WebhookService) CreateWebhook(ctx context.Context, req *CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
	if err := checkWebhookEvents(req.Events); err != nil {
		return nil, err
	}
	secret := req.Secret
	if secret != "" {
		if err := checkWebhookSecret(secret); err != nil {
			return nil, err
		}
	}

	// SECURITY: Validate URL to prevent SSRF attacks
	if err := checkWebhookURL(req.URL); err != nil {
		return nil, err
	}

	if secret == "" {
		generated, err := generateSecret()
		if err != nil {
			return nil, err
		}
		secret = generated
	}

	webhook := &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           req.Name,
		URL:            req.URL,
		Events:         req.Events,
		Secret:         secret,
		IsActive:       req.IsActive == nil || *req.IsActive,
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

// UpdateWebhook updates an existing webhook. It does not change the secret,
// and refuses a request that carries one.
func (s *WebhookService) UpdateWebhook(ctx context.Context, id uuid.UUID, req *CreateWebhookRequest) (*domain.Webhook, error) {
	if req.Secret != "" {
		return nil, &WebhookRequestError{Reason: "unknown field: secret"}
	}
	if err := checkWebhookEvents(req.Events); err != nil {
		return nil, err
	}

	// SECURITY: Validate URL to prevent SSRF attacks
	if err := checkWebhookURL(req.URL); err != nil {
		return nil, err
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

// RotateWebhookSecret replaces the secret of the organization's webhook id
// with a newly generated one and returns it. Every attempt of a delivery is
// signed with the secret the webhook had when the delivery was queued, so a
// delivery queued after the rotation carries the new signature, and one queued
// before it, retries included, keeps the old one.
func (s *WebhookService) RotateWebhookSecret(ctx context.Context, id, orgID uuid.UUID) (string, error) {
	secret, err := generateSecret()
	if err != nil {
		return "", err
	}
	if err := s.webhookRepo.UpdateSecret(id, orgID, secret); err != nil {
		return "", err
	}
	return secret, nil
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

	// Send HTTP request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}

	start := time.Now()
	setDeliveryHeaders(req, webhook, event, jsonData, start)

	// SECURITY: The webhook URL was checked when the webhook was saved. The
	// egress client judges the address again when it connects and does not
	// follow redirects, so neither a changed DNS answer nor a 3xx from the
	// endpoint sends the delivery to an address that check refuses.
	client := s.httpClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		// An event delivery that gets no answer is recorded with the error,
		// so a test delivery that gets no answer is recorded too, as failed.
		failedAt := time.Now().UTC()
		s.recordTestDelivery(&domain.WebhookDelivery{
			ID:             uuid.New(),
			WebhookID:      webhook.ID,
			OrganizationID: webhook.OrganizationID,
			Event:          domain.WebhookEvent(event),
			Payload:        string(jsonData),
			Status:         domain.DeliveryStatusFailed,
			ErrorMessage:   err.Error(),
			AttemptCount:   1,
			DurationMs:     failedAt.Sub(start).Milliseconds(),
			LastAttemptAt:  &failedAt,
			CreatedAt:      start.UTC(),
			CompletedAt:    &failedAt,
		})
		return 0, err
	}
	defer resp.Body.Close()

	// Read response (limited, as for every delivery)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxStoredResponseBody))
	completedAt := time.Now().UTC()

	// Record delivery
	delivery := &domain.WebhookDelivery{
		ID:             uuid.New(),
		WebhookID:      webhook.ID,
		OrganizationID: webhook.OrganizationID,
		Event:          domain.WebhookEvent(event),
		Payload:        string(jsonData),
		Status:         domain.DeliveryStatusSuccess,
		StatusCode:     resp.StatusCode,
		ResponseBody:   string(body),
		Success:        resp.StatusCode >= 200 && resp.StatusCode < 300,
		AttemptCount:   1,
		DurationMs:     completedAt.Sub(start).Milliseconds(),
		LastAttemptAt:  &completedAt,
		CreatedAt:      start.UTC(),
		CompletedAt:    &completedAt,
	}
	if !delivery.Success {
		delivery.Status = domain.DeliveryStatusFailed
		delivery.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	s.recordTestDelivery(delivery)

	if !delivery.Success {
		return resp.StatusCode, fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// recordTestDelivery stores the record of a test delivery. A record that
// cannot be stored leaves the test's result unchanged and is written to the
// server log. A service built without a repository, as a test that only
// watches the connection builds it, stores nothing.
func (s *WebhookService) recordTestDelivery(delivery *domain.WebhookDelivery) {
	if s.webhookRepo == nil {
		return
	}
	if err := s.webhookRepo.RecordDelivery(delivery); err != nil {
		log.Printf("webhook %s: failed to record test delivery %s: %v", delivery.WebhookID, delivery.ID, err)
	}
}

// sendWebhook sends a webhook payload
func (s *WebhookService) sendWebhook(webhook *domain.Webhook, event string, payload interface{}) error {
	_, err := s.sendWebhookWithResult(webhook, event, payload)
	return err
}

// Helper functions

// setDeliveryHeaders sets the headers of a webhook delivery attempt of body
// sent at sentAt, signed with the webhook's secret (see signDelivery). A test
// delivery and an event delivery send the same set, so a receiver that routes
// or filters on them treats both alike.
func setDeliveryHeaders(req *http.Request, webhook *domain.Webhook, event string, body []byte, sentAt time.Time) {
	timestamp, signature := signDelivery(body, webhook.Secret, sentAt)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Timestamp", timestamp)
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", event)
	req.Header.Set("X-Webhook-ID", webhook.ID.String())
	req.Header.Set("User-Agent", "AIM-Webhook/1.0")
}

// webhookSignatureScheme names the signing scheme of X-Webhook-Signature, so
// a later scheme can be sent beside it under its own name.
const webhookSignatureScheme = "v1"

// signDelivery returns the X-Webhook-Timestamp and X-Webhook-Signature values
// of an attempt of body sent at sentAt: the time in Unix seconds, and "v1=" and
// the hex HMAC-SHA256 under secret of "<timestamp>.<body>". Each attempt is
// signed when it is sent, so a receiver that refuses an old timestamp and an
// event id it already processed refuses a replayed attempt.
func signDelivery(body []byte, secret string, sentAt time.Time) (timestamp, signature string) {
	timestamp = strconv.FormatInt(sentAt.Unix(), 10)
	signed := make([]byte, 0, len(timestamp)+1+len(body))
	signed = append(signed, timestamp...)
	signed = append(signed, '.')
	signed = append(signed, body...)
	return timestamp, webhookSignatureScheme + "=" + createSignature(signed, secret)
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createSignature returns the hex HMAC-SHA256 of message under secret.
func createSignature(message []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
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
		s.dispatch(webhook, payload)
	}

	return nil
}

// dispatch starts the delivery of payload to webhook on a goroutine of its own
// when the delivery bounds admit it. A delivery they refuse is not sent: it is
// recorded as abandoned with the reason, which is also written to the server log.
func (s *WebhookService) dispatch(webhook *domain.Webhook, payload *domain.WebhookPayload) {
	bounds := s.bounds()
	if admitted, reason := bounds.admit(webhook.ID); !admitted {
		log.Printf("webhook %s: %s delivery %s %s", webhook.ID, payload.Event, payload.ID, reason)
		now := time.Now().UTC()
		delivery := &domain.WebhookDelivery{
			ID:             uuid.New(),
			WebhookID:      webhook.ID,
			OrganizationID: webhook.OrganizationID,
			Event:          payload.Event,
			Status:         domain.DeliveryStatusAbandoned,
			ErrorMessage:   reason,
			CreatedAt:      now,
			CompletedAt:    &now,
		}
		if jsonData, err := json.Marshal(payload); err == nil {
			delivery.Payload = string(jsonData)
		}
		s.recordDelivery(delivery)
		return
	}
	go func() {
		defer bounds.done(webhook.ID)
		s.deliverWithRetry(webhook, payload)
	}()
}

// retryBudget returns the retries, the first retry delay and the attempt
// timeout of a delivery to webhook: the webhook's own values, or the
// defaults where it stores none, capped by MaxRetries, MaxRetryDelay and
// MaxTimeout.
func (s *WebhookService) retryBudget(webhook *domain.Webhook) (maxRetries int, retryDelay, timeout time.Duration) {
	maxRetries = webhook.MaxRetries
	if maxRetries <= 0 {
		maxRetries = s.config.DefaultMaxRetries
	}
	if ceiling := positiveOr(s.config.MaxRetries, defaultMaxRetriesCeiling); maxRetries > ceiling {
		maxRetries = ceiling
	}

	retryDelay = time.Duration(webhook.RetryDelaySeconds) * time.Second
	if retryDelay <= 0 {
		retryDelay = s.config.DefaultRetryDelay
	}
	if s.config.MaxRetryDelay > 0 && retryDelay > s.config.MaxRetryDelay {
		retryDelay = s.config.MaxRetryDelay
	}

	timeout = time.Duration(webhook.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = s.config.DefaultTimeout
	}
	maxTimeout := s.config.MaxTimeout
	if maxTimeout <= 0 {
		maxTimeout = DefaultWebhookConfig().MaxTimeout
	}
	if timeout <= 0 || timeout > maxTimeout {
		timeout = maxTimeout
	}
	return maxRetries, retryDelay, timeout
}

// waitForRetry waits d before a retry. It returns false when the service
// stops first.
func (s *WebhookService) waitForRetry(d time.Duration) bool {
	newTimer := s.newRetryTimer
	if newTimer == nil {
		newTimer = time.NewTimer
	}
	timer := newTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-s.stopChan:
		return false
	}
}

// recordDelivery stores the record of an event delivery. A record that cannot
// be stored is written to the server log. A service built without a
// repository stores nothing.
func (s *WebhookService) recordDelivery(delivery *domain.WebhookDelivery) {
	if s.webhookRepo == nil {
		return
	}
	if err := s.webhookRepo.RecordDelivery(delivery); err != nil {
		log.Printf("webhook %s: failed to record delivery %s: %v", delivery.WebhookID, delivery.ID, err)
	}
}

// abandonOnStop records a delivery the service stopped before its next
// attempt as abandoned, keeping the error of the attempt before it.
func (s *WebhookService) abandonOnStop(delivery *domain.WebhookDelivery) {
	const stopped = "not sent: the server stopped before the attempt"
	if delivery.ErrorMessage == "" {
		delivery.ErrorMessage = stopped
	} else {
		delivery.ErrorMessage += "; " + stopped
	}
	delivery.Status = domain.DeliveryStatusAbandoned
	delivery.Success = false
	delivery.NextRetryAt = nil
	completedAt := time.Now().UTC()
	delivery.CompletedAt = &completedAt
	s.recordDelivery(delivery)
}

// deliverWithRetry delivers payload to webhook, retrying a failed attempt with
// exponential backoff within the budget retryBudget returns. Each attempt
// waits for an attempt slot (see deliveryBounds); the wait before a retry
// holds none, and lasts the delay recorded in the delivery's nextRetryAt.
func (s *WebhookService) deliverWithRetry(webhook *domain.Webhook, payload *domain.WebhookPayload) {
	maxRetries, retryDelay, timeout := s.retryBudget(webhook)

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
		s.recordDelivery(delivery)
		return
	}
	delivery.Payload = string(jsonData)

	// Attempt delivery with retries
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		release, ok := s.bounds().acquireAttempt(webhook.ID, s.stopChan)
		if !ok {
			s.abandonOnStop(delivery)
			return
		}
		delivery.AttemptCount = attempt
		now := time.Now().UTC()
		delivery.LastAttemptAt = &now

		statusCode, responseBody, duration, deliveryErr := s.attemptDelivery(webhook, payload.Event, jsonData, timeout)
		release()
		delivery.StatusCode = statusCode
		delivery.ResponseBody = responseBody
		delivery.DurationMs = duration.Milliseconds()

		if deliveryErr == nil && statusCode >= 200 && statusCode < 300 {
			// Success
			delivery.Status = domain.DeliveryStatusSuccess
			delivery.Success = true
			completedAt := time.Now().UTC()
			delivery.CompletedAt = &completedAt
			s.recordDelivery(delivery)
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
			s.recordDelivery(delivery)

			// Wait the delay just recorded, then back off for the next one
			if !s.waitForRetry(retryDelay) {
				s.abandonOnStop(delivery)
				return
			}
			retryDelay = time.Duration(float64(retryDelay) * s.config.RetryBackoffFactor)
			if s.config.MaxRetryDelay > 0 && retryDelay > s.config.MaxRetryDelay {
				retryDelay = s.config.MaxRetryDelay
			}
		}
	}

	// All retries exhausted
	delivery.Status = domain.DeliveryStatusAbandoned
	delivery.Success = false
	delivery.NextRetryAt = nil
	completedAt := time.Now().UTC()
	delivery.CompletedAt = &completedAt
	s.recordDelivery(delivery)
	s.webhookRepo.UpdateWebhookStats(webhook.ID, false, completedAt)
}

// attemptDelivery makes a single delivery attempt of the payload for event. The
// X-Webhook-Event header names that event, which need not be the first event the
// webhook subscribes to.
func (s *WebhookService) attemptDelivery(webhook *domain.Webhook, event domain.WebhookEvent, payload []byte, timeout time.Duration) (int, string, time.Duration, error) {
	start := time.Now()

	// Create request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return 0, "", time.Since(start), err
	}

	// Each attempt, a retry included, is signed with the time it is sent.
	setDeliveryHeaders(req, webhook, string(event), payload, start)

	// Send request. SECURITY: see sendWebhookWithResult for why delivery
	// goes through the egress client.
	client := s.httpClient(timeout)
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return 0, "", duration, err
	}
	defer resp.Body.Close()

	// Read response (limit to 1KB to prevent memory issues)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxStoredResponseBody))

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
	s.dispatch(webhook, &payload)

	return nil
}
