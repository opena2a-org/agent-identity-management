package aimsdk

import (
	"context"
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
