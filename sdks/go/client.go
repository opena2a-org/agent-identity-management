package aimsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// Config holds SDK configuration
type Config struct {
	APIURL           string
	APIKey           string
	AgentID          string
	AutoDetect       bool
	DetectionMethods []string // "import", "runtime"
	ReportInterval   time.Duration
}

// Client is the main SDK client
type Client struct {
	config    Config
	reporter  *APIReporter
	detectors []Detector
	stopChan  chan struct{}
	keyPair   *KeyPair // Ed25519 keypair for cryptographic signing
}

// Detector interface for detection methods
type Detector interface {
	Start() error
	Stop()
	GetDetections() []DetectedMCP
}

// NewClient creates a new AIM SDK client
func NewClient(config Config) *Client {
	// Set defaults
	if config.ReportInterval == 0 {
		config.ReportInterval = 10 * time.Second
	}
	if len(config.DetectionMethods) == 0 {
		config.DetectionMethods = []string{"runtime"}
	}

	client := &Client{
		config:    config,
		reporter:  NewAPIReporter(config.APIURL, config.APIKey, config.AgentID),
		detectors: []Detector{},
		stopChan:  make(chan struct{}),
	}

	if config.AutoDetect {
		client.initializeDetectors()
		go client.startPeriodicReporting()
	}

	return client
}

func (c *Client) initializeDetectors() {
	// Go doesn't support runtime import hooks easily
	// For now, rely on manual reporting
	// Future: Could analyze go.mod dependencies at build time
}

func (c *Client) startPeriodicReporting() {
	ticker := time.NewTicker(c.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reportDetections()
		case <-c.stopChan:
			return
		}
	}
}

func (c *Client) reportDetections() {
	var allDetections []DetectedMCP
	for _, detector := range c.detectors {
		allDetections = append(allDetections, detector.GetDetections()...)
	}

	if len(allDetections) == 0 {
		return
	}

	// Convert to DetectionEvents
	events := make([]DetectionEvent, len(allDetections))
	for i, det := range allDetections {
		events[i] = DetectionEvent{
			MCPServer:       det.Name,
			DetectionMethod: det.DetectionMethod,
			Confidence:      det.ConfidenceScore,
			Details:         det.Details,
			SDKVersion:      "1.0.0",
			Timestamp:       time.Now(),
		}
	}

	report := DetectionReportRequest{
		Detections: events,
	}

	if err := c.reporter.Report(context.Background(), report); err != nil {
		// Log error but don't fail
		// fmt.Printf("[AIM SDK] Failed to report detections: %v\n", err)
	}
}

// Detect manually triggers detection
func (c *Client) Detect() []DetectedMCP {
	var allDetections []DetectedMCP
	for _, detector := range c.detectors {
		allDetections = append(allDetections, detector.GetDetections()...)
	}
	return allDetections
}

// ReportMCP manually reports a specific MCP usage
func (c *Client) ReportMCP(ctx context.Context, name string) error {
	report := DetectionReportRequest{
		Detections: []DetectionEvent{
			{
				MCPServer:       name,
				DetectionMethod: "manual",
				Confidence:      100.0,
				SDKVersion:      "1.0.0",
				Timestamp:       time.Now(),
			},
		},
	}

	return c.reporter.Report(ctx, report)
}

// SetKeyPair sets the Ed25519 keypair for cryptographic signing
func (c *Client) SetKeyPair(keyPair *KeyPair) {
	c.keyPair = keyPair
}

// LoadKeyPairFromBase64 loads an Ed25519 keypair from base64-encoded private key string
func (c *Client) LoadKeyPairFromBase64(privateKeyBase64 string) error {
	keyPair, err := NewKeyPairFromBase64(privateKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to load keypair: %w", err)
	}
	c.keyPair = keyPair
	return nil
}

// GetPublicKey returns the client's public key as base64-encoded string
func (c *Client) GetPublicKey() string {
	if c.keyPair == nil {
		return ""
	}
	return c.keyPair.PublicKeyBase64()
}

// RegisterMCP registers an MCP server to this agent's "talks_to" list.
//
// This creates a relationship between the agent and an MCP server,
// indicating that the agent communicates with this MCP server.
//
// Parameters:
//   - ctx: Context for the request
//   - mcpServerId: MCP server ID or name to register
//   - detectionMethod: How the MCP was detected ("manual", "auto_sdk", "auto_config", "cli")
//   - confidence: Detection confidence score (0-100, default: 100 for manual)
//   - metadata: Optional additional context about the detection
//
// Example:
//
//	// Register filesystem MCP server
//	result, err := client.RegisterMCP(
//	    context.Background(),
//	    "filesystem-mcp-server",
//	    "manual",
//	    100.0,
//	    nil,
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Registered %d MCP server(s)\n", result.Added)
func (c *Client) RegisterMCP(
	ctx context.Context,
	mcpServerId string,
	detectionMethod string,
	confidence float64,
	metadata map[string]interface{},
) (*MCPRegistrationResponse, error) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	reqBody := MCPRegistrationRequest{
		MCPServerIDs:     []string{mcpServerId},
		DetectedMethod:   detectionMethod,
		Confidence:       confidence,
		Metadata:         metadata,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/sdk-api/agents/%s/mcp-servers", c.config.APIURL, c.config.AgentID)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))

	resp, err := c.reporter.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP registration failed with status: %d", resp.StatusCode)
	}

	var result MCPRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ReportSDKIntegration reports SDK integration status to AIM dashboard.
//
// This updates the Detection tab to show that the AIM SDK is installed
// and integrated with the agent, enabling auto-detection features.
//
// Parameters:
//   - ctx: Context for the request
//   - sdkVersion: SDK version string (e.g., "aim-sdk-go@1.0.0")
//   - platform: Platform/language (e.g., "go", "golang")
//   - capabilities: Optional list of SDK capabilities enabled
//
// Example:
//
//	// Report SDK integration
//	result, err := client.ReportSDKIntegration(
//	    context.Background(),
//	    "aim-sdk-go@1.0.0",
//	    "go",
//	    []string{"auto_detect_mcps", "capability_detection"},
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("SDK integration reported: %s\n", result.Message)
func (c *Client) ReportSDKIntegration(
	ctx context.Context,
	sdkVersion string,
	platform string,
	capabilities []string,
) (*DetectionReportResponse, error) {
	if capabilities == nil {
		capabilities = []string{}
	}

	// Create SDK integration detection event
	detectionEvent := DetectionEvent{
		MCPServer:       "aim-sdk-integration",
		DetectionMethod: "sdk_integration",
		Confidence:      100.0,
		Details: map[string]interface{}{
			"platform":     platform,
			"capabilities": capabilities,
			"integrated":   true,
		},
		SDKVersion: sdkVersion,
		Timestamp:  time.Now(),
	}

	report := DetectionReportRequest{
		Detections: []DetectionEvent{detectionEvent},
	}

	bodyBytes, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/sdk-api/agents/%s/detection/report", c.config.APIURL, c.config.AgentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))

	resp, err := c.reporter.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SDK integration report failed with status: %d", resp.StatusCode)
	}

	var result DetectionReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ReportCapabilities reports detected agent capabilities to the backend.
//
// This populates the Capabilities tab in the AIM dashboard with the agent's
// detected or manually specified capabilities.
//
// Parameters:
//   - ctx: Context for the request
//   - capabilities: List of capability strings (e.g., "read_files", "access_database")
//
// Example:
//
//	// Auto-detect and report capabilities
//	caps, err := AutoDetectCapabilities()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, err := client.ReportCapabilities(context.Background(), caps)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Reported %d capabilities\n", len(caps))
func (c *Client) ReportCapabilities(
	ctx context.Context,
	capabilities []string,
) error {
	if capabilities == nil || len(capabilities) == 0 {
		return fmt.Errorf("capabilities cannot be empty")
	}

	// Grant each capability to the agent
	// Note: Using the POST /agents/{id}/capabilities endpoint
	for _, capability := range capabilities {
		reqBody := GrantCapabilityRequest{
			CapabilityType: capability,
			Scope:          make(map[string]interface{}),
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		url := fmt.Sprintf("%s/api/v1/sdk-api/agents/%s/capabilities", c.config.APIURL, c.config.AgentID)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))

		resp, err := c.reporter.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		// Accept 201 (Created) or 409 (Conflict - already exists) as success
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("capability reporting failed with status: %d", resp.StatusCode)
		}
	}

	return nil
}

// AutoDetectAndReportCapabilities is a convenience method that auto-detects capabilities
// and reports them to the backend in one call.
//
// Example:
//
//	// Auto-detect and report in one call
//	caps, err := client.AutoDetectAndReportCapabilities(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Auto-detected and reported %d capabilities: %v\n", len(caps), caps)
func (c *Client) AutoDetectAndReportCapabilities(ctx context.Context) ([]string, error) {
	// Auto-detect capabilities
	caps, err := AutoDetectCapabilities()
	if err != nil {
		return nil, fmt.Errorf("capability auto-detection failed: %w", err)
	}

	if len(caps) == 0 {
		return []string{}, nil
	}

	// Report to backend
	if err := c.ReportCapabilities(ctx, caps); err != nil {
		return nil, fmt.Errorf("capability reporting failed: %w", err)
	}

	return caps, nil
}

// GetRuntimeInfo returns information about the Go runtime
func (c *Client) GetRuntimeInfo() map[string]interface{} {
	return map[string]interface{}{
		"runtime":    "go",
		"goVersion":  runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"numCPU":     runtime.NumCPU(),
		"goroutines": runtime.NumGoroutine(),
	}
}

// Close cleans up resources
func (c *Client) Close() {
	close(c.stopChan)
	for _, detector := range c.detectors {
		detector.Stop()
	}
}
