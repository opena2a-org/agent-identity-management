package aimsdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// MCPCapability represents a detected MCP server with its capabilities
type MCPCapability struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	DetectedFrom string            `json:"detected_from"`
	Capabilities []string          `json:"capabilities"`
}

// DetectionResult holds all detected MCP capabilities
type DetectionResult struct {
	MCPs       []MCPCapability    `json:"mcps"`
	DetectedAt string             `json:"detected_at"`
	Runtime    map[string]string  `json:"runtime"`
}

// AutoDetectMCPs scans for MCP server configurations and capabilities
// Searches in standard locations for MCP configuration files
func AutoDetectMCPs() (*DetectionResult, error) {
	result := &DetectionResult{
		MCPs:       []MCPCapability{},
		DetectedAt: time.Now().UTC().Format(time.RFC3339),
		Runtime:    collectRuntimeInfo(),
	}

	// Find MCP config files in standard locations
	configPaths := findMCPConfigs()

	for _, path := range configPaths {
		mcps, err := parseMCPConfig(path)
		if err != nil {
			// Log warning but continue with other configs
			continue
		}

		for _, mcp := range mcps {
			mcp.DetectedFrom = path
			mcp.Capabilities = probeMCPCapabilities(mcp)
			result.MCPs = append(result.MCPs, mcp)
		}
	}

	return result, nil
}

// findMCPConfigs searches for MCP configuration files in standard locations
func findMCPConfigs() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	// Standard MCP config file locations
	locations := []string{}

	if homeDir != "" {
		locations = append(locations,
			filepath.Join(homeDir, ".config", "mcp", "servers.json"),
			filepath.Join(homeDir, ".mcp", "config.json"),
			filepath.Join(homeDir, ".config", "claude", "mcp", "servers.json"),
		)
	}

	if cwd != "" {
		locations = append(locations,
			filepath.Join(cwd, "mcp.json"),
			filepath.Join(cwd, ".mcp", "servers.json"),
			filepath.Join(cwd, "mcp", "servers.json"),
		)
	}

	// Filter to only existing files
	var found []string
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			found = append(found, loc)
		}
	}

	return found
}

// parseMCPConfig parses an MCP configuration file
func parseMCPConfig(path string) ([]MCPCapability, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			Type    string            `json:"type"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert to MCPCapability slice
	var mcps []MCPCapability
	for name, server := range config.MCPServers {
		mcps = append(mcps, MCPCapability{
			Name:    name,
			Type:    server.Type,
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
		})
	}

	return mcps, nil
}

// probeMCPCapabilities attempts to detect MCP server capabilities
// based on command, name, and common patterns
func probeMCPCapabilities(mcp MCPCapability) []string {
	capabilities := []string{}

	// Define capability keywords
	checks := map[string][]string{
		"filesystem": {"npx", "filesystem", "fs", "@modelcontextprotocol/server-filesystem", "file"},
		"database":   {"sqlite", "postgres", "postgresql", "mysql", "mongodb", "db"},
		"web":        {"puppeteer", "playwright", "fetch", "browser", "http"},
		"memory":     {"memory", "redis", "cache", "qdrant", "vector"},
		"github":     {"github", "git"},
		"sequential": {"sequential", "thinking"},
		"brave":      {"brave", "search"},
	}

	// Check command and name for capability keywords
	commandLower := strings.ToLower(mcp.Command)
	nameLower := strings.ToLower(mcp.Name)

	for capType, keywords := range checks {
		for _, keyword := range keywords {
			if strings.Contains(commandLower, keyword) || strings.Contains(nameLower, keyword) {
				capabilities = append(capabilities, capType)
				break
			}
		}
	}

	// Check args for additional hints
	for _, arg := range mcp.Args {
		argLower := strings.ToLower(arg)
		for capType, keywords := range checks {
			for _, keyword := range keywords {
				if strings.Contains(argLower, keyword) {
					// Only add if not already present
					if !contains(capabilities, capType) {
						capabilities = append(capabilities, capType)
					}
					break
				}
			}
		}
	}

	return capabilities
}

// collectRuntimeInfo collects information about the Go runtime
func collectRuntimeInfo() map[string]string {
	return map[string]string{
		"runtime":     "go",
		"go_version":  runtime.Version(),
		"platform":    runtime.GOOS,
		"arch":        runtime.GOARCH,
		"num_cpu":     fmt.Sprintf("%d", runtime.NumCPU()),
		"num_goroutines": fmt.Sprintf("%d", runtime.NumGoroutine()),
	}
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// AutoDetectAndReport is a convenience method that auto-detects MCPs
// and reports them to the AIM backend using the provided client
func AutoDetectAndReport(client *Client) error {
	detection, err := AutoDetectMCPs()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Report each detected MCP
	for _, mcp := range detection.MCPs {
		if err := client.ReportMCP(nil, mcp.Name); err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to report %s: %v\n", mcp.Name, err)
		}
	}

	return nil
}
