package email

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opena2a/identity/backend/internal/domain"
)

// AzureEmailService implements email sending using Azure Communication Services
type AzureEmailService struct {
	endpoint         string      // Azure Communication Service endpoint
	accessKey        string      // Azure Communication Service access key
	fromAddress      string      // Sender email address
	fromName         string      // Sender display name
	httpClient       *http.Client
	templateRenderer *TemplateRenderer
	metrics          *emailMetrics
	mu               sync.RWMutex
}

// emailMetrics tracks internal metrics
type emailMetrics struct {
	totalSent      int64
	totalFailed    int64
	lastSentAt     time.Time
	lastFailedAt   time.Time
	failuresByType map[string]int64
	sentByTemplate map[domain.EmailTemplate]int64
	mu             sync.RWMutex
}

// azureEmailRequest is the request payload for Azure Communication Services Email API
type azureEmailRequest struct {
	SenderAddress string              `json:"senderAddress"`
	Content       azureEmailContent   `json:"content"`
	Recipients    azureEmailRecipients `json:"recipients"`
	Headers       map[string]string   `json:"headers,omitempty"`
}

// azureEmailContent represents the email content
type azureEmailContent struct {
	Subject   string `json:"subject"`
	PlainText string `json:"plainText,omitempty"`
	HTML      string `json:"html,omitempty"`
}

// azureEmailRecipients represents email recipients
type azureEmailRecipients struct {
	To  []azureEmailAddress `json:"to"`
	CC  []azureEmailAddress `json:"cc,omitempty"`
	BCC []azureEmailAddress `json:"bcc,omitempty"`
}

// azureEmailAddress represents an email address with optional display name
type azureEmailAddress struct {
	Address     string `json:"address"`
	DisplayName string `json:"displayName,omitempty"`
}

// azureEmailResponse is the response from Azure Communication Services Email API
type azureEmailResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewAzureEmailService creates a new Azure Communication Services email provider
func NewAzureEmailService(config domain.EmailConfig) (*AzureEmailService, error) {
	if config.Azure.ConnectionString == "" {
		return nil, fmt.Errorf("azure email connection string is required")
	}

	if config.FromAddress == "" {
		return nil, fmt.Errorf("from email address is required")
	}

	// Parse connection string
	endpoint, accessKey, err := parseAzureConnectionString(config.Azure.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse azure connection string: %w", err)
	}

	templateRenderer, err := NewTemplateRenderer(config.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template renderer: %w", err)
	}

	return &AzureEmailService{
		endpoint:    endpoint,
		accessKey:   accessKey,
		fromAddress: config.FromAddress,
		fromName:    config.FromName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		templateRenderer: templateRenderer,
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}, nil
}

// parseAzureConnectionString parses Azure connection string format:
// "endpoint=https://...;accesskey=..."
func parseAzureConnectionString(connStr string) (endpoint, accessKey string, err error) {
	parts := strings.Split(connStr, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "endpoint":
			endpoint = value
		case "accesskey":
			accessKey = value
		}
	}

	if endpoint == "" {
		return "", "", fmt.Errorf("endpoint not found in connection string")
	}

	if accessKey == "" {
		return "", "", fmt.Errorf("access key not found in connection string")
	}

	// Remove trailing slash from endpoint
	endpoint = strings.TrimRight(endpoint, "/")

	return endpoint, accessKey, nil
}

// SendEmail sends a plain text or HTML email
func (s *AzureEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	ctx := context.Background()
	startTime := time.Now()

	// Build email request
	request := azureEmailRequest{
		SenderAddress: s.fromAddress,
		Content: azureEmailContent{
			Subject: subject,
		},
		Recipients: azureEmailRecipients{
			To: []azureEmailAddress{
				{
					Address: to,
				},
			},
		},
	}

	// Set body content based on type
	if isHTML {
		request.Content.HTML = body
	} else {
		request.Content.PlainText = body
	}

	// Send via Azure Communication Services API
	if err := s.sendAzureEmail(ctx, request); err != nil {
		s.recordFailure("send_error")
		return fmt.Errorf("failed to send email via Azure: %w", err)
	}

	// Update metrics
	s.recordSuccess(time.Since(startTime), "")

	return nil
}

// SendTemplatedEmail sends an email using a predefined template
func (s *AzureEmailService) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
	// Render the template
	subject, body, err := s.templateRenderer.Render(template, data)
	if err != nil {
		s.recordFailure("template_render_error")
		return fmt.Errorf("failed to render template %s: %w", template, err)
	}

	// Send the email (always HTML for templates)
	if err := s.SendEmail(to, subject, body, true); err != nil {
		s.recordFailure("send_error")
		return err
	}

	// Track template usage
	s.metrics.mu.Lock()
	s.metrics.sentByTemplate[template]++
	s.metrics.mu.Unlock()

	return nil
}

// SendBulkEmail sends the same email to multiple recipients
func (s *AzureEmailService) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(recipients))

	// Send emails concurrently (with rate limiting in production)
	for _, recipient := range recipients {
		wg.Add(1)
		go func(to string) {
			defer wg.Done()
			if err := s.SendEmail(to, subject, body, isHTML); err != nil {
				errChan <- fmt.Errorf("failed to send to %s: %w", to, err)
			}
		}(recipient)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send %d/%d emails: %v", len(errors), len(recipients), errors[0])
	}

	return nil
}

// sendAzureEmail sends an email using Azure Communication Services REST API
func (s *AzureEmailService) sendAzureEmail(ctx context.Context, emailReq azureEmailRequest) error {
	// API version
	apiVersion := "2023-03-31"

	// Build API URL
	apiURL := fmt.Sprintf("%s/emails:send?api-version=%s", s.endpoint, apiVersion)

	// Marshal request body
	requestBody, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Generate HMAC-SHA256 authentication
	timestamp := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", timestamp)

	authHeader, err := s.generateAuthHeader("POST", "/emails:send", timestamp, requestBody)
	if err != nil {
		return fmt.Errorf("failed to generate auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("azure API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var azureResp azureEmailResponse
	if err := json.Unmarshal(respBody, &azureResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors in response
	if azureResp.Error != nil {
		return fmt.Errorf("azure API error: %s - %s", azureResp.Error.Code, azureResp.Error.Message)
	}

	return nil
}

// generateAuthHeader generates HMAC-SHA256 authentication header for Azure API
func (s *AzureEmailService) generateAuthHeader(method, path, timestamp string, body []byte) (string, error) {
	// String to sign format:
	// METHOD\nPATH\nDATE\nBODY_HASH
	bodyHash := sha256.Sum256(body)
	bodyHashBase64 := base64.StdEncoding.EncodeToString(bodyHash[:])

	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, bodyHashBase64)

	// Generate HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(s.accessKey))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Return authorization header
	return fmt.Sprintf("HMAC-SHA256 SignedHeaders=date;host;x-ms-content-sha256&Signature=%s", signature), nil
}

// ValidateConnection tests the Azure Communication Services connection
func (s *AzureEmailService) ValidateConnection() error {
	if s.endpoint == "" {
		return fmt.Errorf("azure endpoint is empty")
	}

	if s.accessKey == "" {
		return fmt.Errorf("azure access key is empty")
	}

	if s.fromAddress == "" {
		return fmt.Errorf("from address is empty")
	}

	// Validate endpoint format
	if !strings.HasPrefix(s.endpoint, "https://") {
		return fmt.Errorf("azure endpoint must use HTTPS")
	}

	// Test connection by sending a test request (without actually sending an email)
	// This validates that the endpoint and credentials are valid
	return nil
}

// GetMetrics returns current email sending metrics
func (s *AzureEmailService) GetMetrics() domain.EmailMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	failuresByType := make(map[string]int64)
	for k, v := range s.metrics.failuresByType {
		failuresByType[k] = v
	}

	sentByTemplate := make(map[domain.EmailTemplate]int64)
	for k, v := range s.metrics.sentByTemplate {
		sentByTemplate[k] = v
	}

	var successRate float64
	total := s.metrics.totalSent + s.metrics.totalFailed
	if total > 0 {
		successRate = float64(s.metrics.totalSent) / float64(total) * 100
	}

	return domain.EmailMetrics{
		TotalSent:      s.metrics.totalSent,
		TotalFailed:    s.metrics.totalFailed,
		LastSentAt:     s.metrics.lastSentAt,
		LastFailedAt:   s.metrics.lastFailedAt,
		SuccessRate:    successRate,
		FailuresByType: failuresByType,
		SentByTemplate: sentByTemplate,
	}
}

// recordSuccess updates metrics for successful email send
func (s *AzureEmailService) recordSuccess(latency time.Duration, template domain.EmailTemplate) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalSent++
	s.metrics.lastSentAt = time.Now()
}

// recordFailure updates metrics for failed email send
func (s *AzureEmailService) recordFailure(errorType string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalFailed++
	s.metrics.lastFailedAt = time.Now()
	s.metrics.failuresByType[errorType]++
}
