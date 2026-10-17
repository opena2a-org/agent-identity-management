package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Microsoft Sentinel Adapter (Azure Monitor Logs Ingestion API)
// ============================================================================

const (
	// DefaultSentinelAuthorityHost is the Microsoft Entra host the token is
	// requested from when SentinelConfig.AuthorityHost is empty.
	DefaultSentinelAuthorityHost = "https://login.microsoftonline.com"

	// sentinelScope asks for a token for the Logs Ingestion API audience.
	sentinelScope = "https://monitor.azure.com/.default"

	// sentinelAPIVersion is the api-version of the upload request.
	sentinelAPIVersion = "2023-01-01"

	// sentinelTokenMargin is how long before a token's expiry it stops
	// being used.
	sentinelTokenMargin = 60 * time.Second

	// maxSentinelAnswer bounds how much of an answer is read. A token answer
	// is longer than maxErrorBody, and a refusal is redacted before it is
	// cut to maxErrorBody.
	maxSentinelAnswer = 64 << 10

	sentinelService = "sentinel"
)

// SentinelConfig holds the Logs Ingestion API and Microsoft Entra application
// configuration.
type SentinelConfig struct {
	Endpoint       string `json:"endpoint"`       // logs ingestion endpoint of the data collection rule or its data collection endpoint
	DCRImmutableID string `json:"dcrImmutableId"` // immutable id of the data collection rule
	StreamName     string `json:"streamName"`     // stream the rule declares for the records
	TenantID       string `json:"tenantId"`       // Microsoft Entra tenant of the application
	ClientID       string `json:"clientId"`       // application (client) id
	ClientSecret   string `json:"-"`              // application client secret
	AuthorityHost  string `json:"authorityHost"`  // token host (default: https://login.microsoftonline.com)
}

// SentinelAdapter delivers events to Microsoft Sentinel through the Azure
// Monitor Logs Ingestion API. It asks Microsoft Entra ID for a token with the
// client credentials grant, keeps it until sentinelTokenMargin before it
// expires, and posts each batch as one JSON array of records to the data
// collection rule's stream.
type SentinelAdapter struct {
	config SentinelConfig
	client *http.Client
	now    func() time.Time

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewSentinelAdapter(config SentinelConfig) *SentinelAdapter {
	if config.AuthorityHost == "" {
		config.AuthorityHost = DefaultSentinelAuthorityHost
	}
	return &SentinelAdapter{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// A redirect is answered as it is: the token request carries
			// the client secret and the upload a token, and neither is
			// sent on to a host the operator did not name.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}
}

func (s *SentinelAdapter) Name() string { return sentinelService }

func (s *SentinelAdapter) Send(ctx context.Context, event *SIEMEvent) error {
	return s.SendBatch(ctx, []*SIEMEvent{event})
}

// sentinelRecord is one record of the upload. Each column name starts with a
// letter, has up to 45 alphanumerics or underscores, and is not a reserved
// column name.
type sentinelRecord struct {
	TimeGenerated string                 `json:"TimeGenerated"`
	EventType     string                 `json:"EventType"`
	Severity      string                 `json:"Severity"`
	AgentID       string                 `json:"AgentId"`
	Capability    string                 `json:"Capability"`
	Outcome       string                 `json:"Outcome"`
	Source        string                 `json:"Source"`
	Details       map[string]interface{} `json:"Details"`
}

func newSentinelRecord(event *SIEMEvent) sentinelRecord {
	source := event.Source
	if source == "" {
		source = "opena2a-aim"
	}
	details := event.Details
	if details == nil {
		details = map[string]interface{}{}
	}
	return sentinelRecord{
		TimeGenerated: event.Timestamp.UTC().Format(time.RFC3339Nano),
		EventType:     string(event.EventType),
		Severity:      event.Severity,
		AgentID:       event.AgentID,
		Capability:    event.Capability,
		Outcome:       event.Outcome,
		Source:        source,
		Details:       details,
	}
}

// SendBatch posts the events as one JSON array of records. Any 2xx answer is
// delivery. A refused token, or a 401 or 403 from the upload, is a
// StatusError that is not retried, and the token is asked for again on the
// next batch.
func (s *SentinelAdapter) SendBatch(ctx context.Context, events []*SIEMEvent) error {
	if len(events) == 0 {
		return nil
	}

	records := make([]sentinelRecord, len(events))
	for i, event := range events {
		records[i] = newSentinelRecord(event)
	}
	body, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to marshal Sentinel records: %w", err)
	}

	token, err := s.accessToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.uploadURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Sentinel upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			s.forgetToken(token)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxSentinelAnswer))
		return &StatusError{Service: sentinelService, StatusCode: resp.StatusCode, Body: s.redact(string(raw), token)}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))
	return nil
}

func (s *SentinelAdapter) uploadURL() string {
	return strings.TrimRight(s.config.Endpoint, "/") +
		"/dataCollectionRules/" + url.PathEscape(s.config.DCRImmutableID) +
		"/streams/" + url.PathEscape(s.config.StreamName) +
		"?api-version=" + sentinelAPIVersion
}

func (s *SentinelAdapter) tokenURL() string {
	return strings.TrimRight(s.config.AuthorityHost, "/") + "/" + url.PathEscape(s.config.TenantID) + "/oauth2/v2.0/token"
}

// accessToken returns the cached token, or asks for a new one with the client
// credentials grant.
func (s *SentinelAdapter) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.now().Before(s.tokenExpiry) {
		return s.token, nil
	}
	s.token, s.tokenExpiry = "", time.Time{}

	form := url.Values{
		"client_id":     {s.config.ClientID},
		"client_secret": {s.config.ClientSecret},
		"scope":         {sentinelScope},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	requested := s.now()
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Sentinel token request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSentinelAnswer))
	if err != nil {
		return "", fmt.Errorf("Sentinel token answer could not be read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", &StatusError{
			Service:    sentinelService + " token request",
			StatusCode: resp.StatusCode,
			Body:       s.redact(tokenRefusal(raw), ""),
		}
	}

	var answer struct {
		TokenType   string      `json:"token_type"`
		ExpiresIn   json.Number `json:"expires_in"`
		AccessToken string      `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || answer.AccessToken == "" || !strings.EqualFold(answer.TokenType, "Bearer") {
		return "", fmt.Errorf("Sentinel token answer %d carries no bearer access token", resp.StatusCode)
	}
	expiresIn, err := answer.ExpiresIn.Int64()
	if err != nil || expiresIn <= 0 {
		return "", fmt.Errorf("Sentinel token answer %d carries no expires_in", resp.StatusCode)
	}

	s.token = answer.AccessToken
	s.tokenExpiry = requested.Add(time.Duration(expiresIn)*time.Second - sentinelTokenMargin)
	return s.token, nil
}

// forgetToken drops token from the cache, unless another has replaced it.
func (s *SentinelAdapter) forgetToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == token {
		s.token, s.tokenExpiry = "", time.Time{}
	}
}

// tokenRefusal reads the error and error_description of a refused token
// request, or keeps the answer as it is when it is not that JSON object.
func tokenRefusal(raw []byte) string {
	var refusal struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(raw, &refusal) == nil && refusal.Error != "" {
		if refusal.Description == "" {
			return refusal.Error
		}
		return refusal.Error + ": " + refusal.Description
	}
	return string(raw)
}

// redact replaces the client secret and token in an endpoint's answer, so an
// answer that echoes either never reaches an error or a log line, and then
// keeps the first maxErrorBody bytes, as a StatusError does.
func (s *SentinelAdapter) redact(answer, token string) string {
	for _, secret := range []string{s.config.ClientSecret, token} {
		if secret != "" {
			answer = strings.ReplaceAll(answer, secret, "[redacted]")
		}
	}
	if len(answer) > maxErrorBody {
		answer = answer[:maxErrorBody]
	}
	return answer
}
