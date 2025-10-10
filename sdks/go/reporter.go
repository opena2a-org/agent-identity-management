package aimsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// APIReporter reports detections to AIM API
type APIReporter struct {
	apiURL     string
	apiKey     string
	agentID    string
	httpClient *http.Client
	lastReport map[string]time.Time // MCP name -> last reported time
}

// NewAPIReporter creates a new API reporter
func NewAPIReporter(apiURL, apiKey, agentID string) *APIReporter {
	return &APIReporter{
		apiURL:     apiURL,
		apiKey:     apiKey,
		agentID:    agentID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		lastReport: make(map[string]time.Time),
	}
}

// Report sends detection report to AIM API
func (r *APIReporter) Report(ctx context.Context, report DetectionReportRequest) error {
	// Deduplicate: Only report if not reported in last 60 seconds
	now := time.Now()
	var newDetections []DetectionEvent

	for _, detection := range report.Detections {
		lastReported, exists := r.lastReport[detection.MCPServer]
		if !exists || now.Sub(lastReported) > 60*time.Second {
			newDetections = append(newDetections, detection)
		}
	}

	if len(newDetections) == 0 {
		return nil
	}

	report.Detections = newDetections

	// Marshal request body
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/v1/detection/agents/%s/report", r.apiURL, r.agentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

	// Send request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var response DetectionReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Update last report timestamps
	for _, detection := range newDetections {
		r.lastReport[detection.MCPServer] = now
	}

	return nil
}
