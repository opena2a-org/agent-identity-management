// Package siem implements event streaming adapters for enterprise SIEM platforms.
// Supports Splunk HEC (HTTP Event Collector) and Microsoft Sentinel through
// the Azure Monitor Logs Ingestion API.
package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EventType categorizes AIM security events for SIEM routing.
type EventType string

const (
	EventCapabilityGranted     EventType = "aim.capability.granted"
	EventCapabilityDenied      EventType = "aim.capability.denied"
	EventFGADenied             EventType = "aim.fga.denied"
	EventSignInFailed          EventType = "aim.auth.signin_failed"
	EventIntentBlocked         EventType = "aim.intent.blocked"
	EventApprovalRequested     EventType = "aim.approval.requested"
	EventApprovalDecided       EventType = "aim.approval.decided"
	EventEmergencyDeclared     EventType = "aim.emergency.declared"
	EventEmergencyExpired      EventType = "aim.emergency.expired"
	EventCertificationStarted  EventType = "aim.certification.started"
	EventCertificationDecision EventType = "aim.certification.decision"
	EventAgentSuspended        EventType = "aim.agent.suspended"
	EventDriftAlert            EventType = "aim.drift.alert"
	EventAlertCreated          EventType = "aim.alert.created"
)

// SIEMEvent is the standardized event schema for all AIM security events.
type SIEMEvent struct {
	EventType  EventType              `json:"eventType"`
	Timestamp  time.Time              `json:"timestamp"`
	AgentID    string                 `json:"agentId,omitempty"`
	Capability string                 `json:"capability,omitempty"`
	Outcome    string                 `json:"outcome,omitempty"`
	Severity   string                 `json:"severity"` // info, low, medium, high, critical
	Details    map[string]interface{} `json:"details,omitempty"`
	Source     string                 `json:"source"` // "opena2a-aim"
}

// Adapter sends events to a SIEM platform.
type Adapter interface {
	Send(ctx context.Context, event *SIEMEvent) error
	Name() string
}

// BatchAdapter is an Adapter that delivers several events in one request.
// The Forwarder delivers through it.
type BatchAdapter interface {
	Adapter
	SendBatch(ctx context.Context, events []*SIEMEvent) error
}

// StatusError is a SIEM endpoint's refusal of a request: an HTTP status of
// 400 or above, a redirect, or a 2xx token answer that carries no usable
// token. Body is the start of the endpoint's answer, or a description of it.
type StatusError struct {
	Service    string
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s returned %d: %s", e.Service, e.StatusCode, e.Body)
}

// Retryable reports whether the same request can succeed later: a timeout
// (408), a rate limit (429) or a server-side failure (5xx). Any other refusal
// (a bad token, a malformed event, an index the token may not write) repeats.
func (e *StatusError) Retryable() bool {
	return e.StatusCode == http.StatusRequestTimeout ||
		e.StatusCode == http.StatusTooManyRequests ||
		e.StatusCode >= 500
}

// maxErrorBody bounds how much of an endpoint's error answer a StatusError keeps.
const maxErrorBody = 512

func statusError(service string, resp *http.Response) *StatusError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return &StatusError{Service: service, StatusCode: resp.StatusCode, Body: string(body)}
}

// ============================================================================
// Splunk HEC Adapter
// ============================================================================

// SplunkConfig holds Splunk HTTP Event Collector configuration.
type SplunkConfig struct {
	HECURL   string `json:"hecUrl"`   // e.g., https://splunk.example.com:8088/services/collector/event
	HECToken string `json:"hecToken"` // HEC auth token
	Index    string `json:"index"`    // Target index (default: none, so the token's default index applies)
	Source   string `json:"source"`   // Source identifier (default: "opena2a-aim")
}

type SplunkAdapter struct {
	config SplunkConfig
	client *http.Client
}

func NewSplunkAdapter(config SplunkConfig) *SplunkAdapter {
	if config.Source == "" {
		config.Source = "opena2a-aim"
	}
	return &SplunkAdapter{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SplunkAdapter) Name() string { return "splunk" }

func (s *SplunkAdapter) Send(ctx context.Context, event *SIEMEvent) error {
	return s.SendBatch(ctx, []*SIEMEvent{event})
}

// SendBatch posts the events to the collector in one request: one HEC event
// object per event, one after another, which is the HEC batch format. The
// events themselves are not modified.
func (s *SplunkAdapter) SendBatch(ctx context.Context, events []*SIEMEvent) error {
	if len(events) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, event := range events {
		e := *event
		e.Source = s.config.Source
		hecEvent := map[string]interface{}{
			"time":       float64(e.Timestamp.UnixMilli()) / 1000,
			"sourcetype": "opena2a:aim",
			"source":     s.config.Source,
			"event":      &e,
		}
		if s.config.Index != "" {
			hecEvent["index"] = s.config.Index
		}
		line, err := json.Marshal(hecEvent)
		if err != nil {
			return fmt.Errorf("failed to marshal Splunk event: %w", err)
		}
		body.Write(line)
		body.WriteByte('\n')
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.HECURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Splunk "+s.config.HECToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Splunk HEC request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return statusError("Splunk HEC", resp)
	}
	// Read the short acknowledgement so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))

	return nil
}

// The Microsoft Sentinel adapter is in sentinel.go.

// ============================================================================
// Multi-Adapter (fan-out to multiple SIEMs)
// ============================================================================

// MultiAdapter sends events to all configured SIEM adapters.
type MultiAdapter struct {
	adapters []Adapter
}

func NewMultiAdapter(adapters ...Adapter) *MultiAdapter {
	return &MultiAdapter{adapters: adapters}
}

func (m *MultiAdapter) Send(ctx context.Context, event *SIEMEvent) error {
	var lastErr error
	for _, adapter := range m.adapters {
		if err := adapter.Send(ctx, event); err != nil {
			lastErr = fmt.Errorf("%s: %w", adapter.Name(), err)
		}
	}
	return lastErr
}

func (m *MultiAdapter) Name() string { return "multi" }
