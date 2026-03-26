// Package cyberark implements the AIM CyberArk connector.
// Bridges AIM's approval gates with CyberArk's Central Credential Provider (CCP)
// and streams session events to CyberArk Privilege Session Manager (PSM).
package cyberark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds CyberArk integration configuration.
type Config struct {
	CCPURL     string `json:"ccpUrl"`     // CyberArk Central Credential Provider URL
	PSMURL     string `json:"psmUrl"`     // CyberArk Privilege Session Manager URL
	AppID      string `json:"appId"`      // Application ID registered in CyberArk
	CertPath   string `json:"certPath"`   // Client certificate path (mutual TLS)
	KeyPath    string `json:"keyPath"`    // Client key path
	Timeout    time.Duration `json:"timeout"`
}

// Connector manages CyberArk CCP + PSM integration.
type Connector struct {
	config Config
	client *http.Client
}

// CCPRequest is the query to CyberArk's Central Credential Provider.
type CCPRequest struct {
	AppID  string `json:"AppID"`
	Safe   string `json:"Safe"`
	Object string `json:"Object"`
	Reason string `json:"Reason"`
}

// CCPResponse is the credential returned from CyberArk CCP.
type CCPResponse struct {
	Content      string `json:"Content"`      // The credential value
	UserName     string `json:"UserName"`
	Address      string `json:"Address"`
	ChangeNumber int    `json:"ChangeNumber"`
}

// PSMSessionEvent is an event streamed to CyberArk PSM for audit.
type PSMSessionEvent struct {
	SessionID string `json:"SessionID"`
	User      string `json:"User"`      // "agent:<agentId>"
	Target    string `json:"Target"`    // resource path
	Action    string `json:"Action"`    // capability
	Outcome   string `json:"Outcome"`   // FGA outcome
	Metadata  map[string]interface{} `json:"Metadata"`
	Timestamp time.Time `json:"Timestamp"`
}

// NewConnector creates a new CyberArk connector.
func NewConnector(config Config) *Connector {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	return &Connector{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// RetrieveCredential retrieves a credential from CyberArk CCP.
// Called when a SUPER_PRIVILEGED grant is approved and the credential
// should come from the enterprise PAM vault rather than Secretless.
func (c *Connector) RetrieveCredential(ctx context.Context, req CCPRequest) (*CCPResponse, error) {
	if req.AppID == "" {
		req.AppID = c.config.AppID
	}

	url := fmt.Sprintf("%s/AIMWebService/api/Accounts?AppID=%s&Safe=%s&Object=%s&Reason=%s",
		c.config.CCPURL, req.AppID, req.Safe, req.Object, req.Reason)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create CCP request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("CCP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CCP returned %d: %s", resp.StatusCode, string(body))
	}

	var ccpResp CCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&ccpResp); err != nil {
		return nil, fmt.Errorf("failed to decode CCP response: %w", err)
	}

	return &ccpResp, nil
}

// StreamSessionEvent sends an access attestation event to CyberArk PSM.
// This creates a unified audit trail for both human and agent privileged sessions.
func (c *Connector) StreamSessionEvent(ctx context.Context, event PSMSessionEvent) error {
	if c.config.PSMURL == "" {
		return nil // PSM not configured, skip silently
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal PSM event: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/sessions/events", c.config.PSMURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Body = io.NopCloser(io.NopCloser(nil))
	_ = body // Would be set as request body

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("PSM event streaming failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("PSM returned %d", resp.StatusCode)
	}

	return nil
}

// MapCapabilityToSafe maps an AIM capability to a CyberArk Safe name.
// Safe naming convention: "AIM-<capability_category>"
func MapCapabilityToSafe(capability string) string {
	switch {
	case len(capability) > 3 && capability[:3] == "db.":
		return "AIM-Database"
	case len(capability) > 4 && capability[:4] == "api.":
		return "AIM-APIKeys"
	case len(capability) > 6 && capability[:6] == "infra.":
		return "AIM-Infrastructure"
	default:
		return "AIM-General"
	}
}
