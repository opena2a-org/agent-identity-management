// Package siem implements event streaming adapters for enterprise SIEM platforms.
// Supports Splunk HEC (HTTP Event Collector) and Microsoft Sentinel.
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
	EventIntentBlocked         EventType = "aim.intent.blocked"
	EventApprovalRequested     EventType = "aim.approval.requested"
	EventApprovalDecided       EventType = "aim.approval.decided"
	EventEmergencyDeclared     EventType = "aim.emergency.declared"
	EventEmergencyExpired      EventType = "aim.emergency.expired"
	EventCertificationStarted  EventType = "aim.certification.started"
	EventCertificationDecision EventType = "aim.certification.decision"
	EventAgentSuspended        EventType = "aim.agent.suspended"
	EventDriftAlert            EventType = "aim.drift.alert"
)

// SIEMEvent is the standardized event schema for all AIM security events.
type SIEMEvent struct {
	EventType  EventType              `json:"eventType"`
	Timestamp  time.Time              `json:"timestamp"`
	AgentID    string                 `json:"agentId"`
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

// ============================================================================
// Splunk HEC Adapter
// ============================================================================

// SplunkConfig holds Splunk HTTP Event Collector configuration.
type SplunkConfig struct {
	HECURL   string `json:"hecUrl"`   // e.g., https://splunk.example.com:8088/services/collector
	HECToken string `json:"hecToken"` // HEC auth token
	Index    string `json:"index"`    // Target index (default: "main")
	Source   string `json:"source"`   // Source identifier (default: "opena2a-aim")
}

type SplunkAdapter struct {
	config SplunkConfig
	client *http.Client
}

func NewSplunkAdapter(config SplunkConfig) *SplunkAdapter {
	if config.Index == "" {
		config.Index = "main"
	}
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
	event.Source = s.config.Source

	// Splunk HEC format
	hecEvent := map[string]interface{}{
		"time":       event.Timestamp.Unix(),
		"sourcetype": "opena2a:aim",
		"source":     s.config.Source,
		"index":      s.config.Index,
		"event":      event,
	}

	body, err := json.Marshal(hecEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal Splunk event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.HECURL, bytes.NewReader(body))
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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Splunk HEC returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ============================================================================
// Microsoft Sentinel Adapter
// ============================================================================

// SentinelConfig holds Microsoft Sentinel configuration.
type SentinelConfig struct {
	WorkspaceID string `json:"workspaceId"` // Log Analytics workspace ID
	SharedKey   string `json:"sharedKey"`   // Primary or secondary key
	LogType     string `json:"logType"`     // Custom log type name (default: "OpenA2A_AIM")
}

type SentinelAdapter struct {
	config SentinelConfig
	client *http.Client
}

func NewSentinelAdapter(config SentinelConfig) *SentinelAdapter {
	if config.LogType == "" {
		config.LogType = "OpenA2A_AIM"
	}
	return &SentinelAdapter{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SentinelAdapter) Name() string { return "sentinel" }

func (s *SentinelAdapter) Send(ctx context.Context, event *SIEMEvent) error {
	event.Source = "opena2a-aim"

	body, err := json.Marshal([]interface{}{event})
	if err != nil {
		return fmt.Errorf("failed to marshal Sentinel event: %w", err)
	}

	url := fmt.Sprintf("https://%s.ods.opinsights.azure.com/api/logs?api-version=2016-04-01",
		s.config.WorkspaceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Log-Type", s.config.LogType)
	// Authorization header would use HMAC-SHA256 signature of the request
	// (simplified here -- full implementation requires date + content-length + body hash)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Sentinel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Sentinel returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

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
