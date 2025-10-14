package aimsdk

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CapabilityDetector auto-detects agent capabilities
type CapabilityDetector struct {
	// Map Go packages to capabilities
	importToCapability map[string]string
}

// CapabilitiesConfig represents the .aim/capabilities.json config file
type CapabilitiesConfig struct {
	Capabilities []string `json:"capabilities"`
	LastUpdated  string   `json:"last_updated"`
	Version      string   `json:"version"`
}

// CapabilityDetectionResult contains detected capabilities and metadata
type CapabilityDetectionResult struct {
	Capabilities []string          `json:"capabilities"`
	DetectedFrom []string          `json:"detected_from"` // "go.mod", "config", "runtime"
	Metadata     map[string]string `json:"metadata"`
}

// NewCapabilityDetector creates a new capability detector
func NewCapabilityDetector() *CapabilityDetector {
	return &CapabilityDetector{
		importToCapability: map[string]string{
			// File System
			"os":      "read_files",
			"io":      "read_files",
			"io/fs":   "read_files",
			"path":    "read_files",
			"bufio":   "read_files",
			"ioutil":  "read_files",

			// Database
			"database/sql":          "access_database",
			"github.com/lib/pq":     "access_database", // PostgreSQL
			"go.mongodb.org/mongo-driver": "access_database", // MongoDB
			"github.com/go-sql-driver/mysql": "access_database", // MySQL
			"gorm.io/gorm":          "access_database", // GORM
			"github.com/jmoiron/sqlx": "access_database", // sqlx

			// HTTP/API
			"net/http":                    "make_api_calls",
			"net/url":                     "make_api_calls",
			"github.com/gorilla/mux":      "make_api_calls",
			"github.com/gin-gonic/gin":    "make_api_calls",
			"github.com/gofiber/fiber":    "make_api_calls",

			// Code Execution
			"os/exec": "execute_code",

			// Cloud Services
			"github.com/aws/aws-sdk-go":   "access_cloud_services", // AWS SDK
			"cloud.google.com/go":         "access_cloud_services", // Google Cloud
			"github.com/Azure/azure-sdk-for-go": "access_cloud_services", // Azure

			// Web Scraping
			"github.com/PuerkitoBio/goquery": "web_scraping",
			"github.com/gocolly/colly":       "web_scraping",
			"github.com/chromedp/chromedp":   "web_automation",

			// Data Processing
			"encoding/json":     "data_processing",
			"encoding/xml":      "data_processing",
			"encoding/csv":      "data_processing",
			"gopkg.in/yaml.v3":  "data_processing",

			// AI/ML
			"github.com/sashabaranov/go-openai": "ai_model_access",
			"github.com/anthropics/anthropic-sdk-go": "ai_model_access",

			// Email
			"net/smtp": "send_email",
			"net/mail": "read_email",
		},
	}
}

// DetectAll runs all detection methods and returns combined unique capabilities
func (cd *CapabilityDetector) DetectAll() (*CapabilityDetectionResult, error) {
	capabilitiesSet := make(map[string]bool)
	detectedFrom := []string{}
	metadata := make(map[string]string)

	// 1. Detect from go.mod
	modCaps, err := cd.DetectFromGoMod()
	if err == nil && len(modCaps) > 0 {
		for _, cap := range modCaps {
			capabilitiesSet[cap] = true
		}
		detectedFrom = append(detectedFrom, "go.mod")
	}

	// 2. Detect from config file
	configCaps, err := cd.DetectFromConfig()
	if err == nil && len(configCaps) > 0 {
		for _, cap := range configCaps {
			capabilitiesSet[cap] = true
		}
		detectedFrom = append(detectedFrom, "config")
	}

	// 3. Detect from runtime environment
	runtimeCaps := cd.DetectFromRuntime()
	if len(runtimeCaps) > 0 {
		for _, cap := range runtimeCaps {
			capabilitiesSet[cap] = true
		}
		detectedFrom = append(detectedFrom, "runtime")
	}

	// Add metadata
	metadata["go_version"] = runtime.Version()
	metadata["os"] = runtime.GOOS
	metadata["arch"] = runtime.GOARCH

	// Convert map to sorted slice
	capabilities := []string{}
	for cap := range capabilitiesSet {
		capabilities = append(capabilities, cap)
	}

	return &CapabilityDetectionResult{
		Capabilities: capabilities,
		DetectedFrom: detectedFrom,
		Metadata:     metadata,
	}, nil
}

// DetectFromGoMod detects capabilities from go.mod file
func (cd *CapabilityDetector) DetectFromGoMod() ([]string, error) {
	capabilities := make(map[string]bool)

	// Find go.mod file
	goModPath, err := cd.findGoMod()
	if err != nil {
		return []string{}, err
	}

	// Read go.mod file
	file, err := os.Open(goModPath)
	if err != nil {
		return []string{}, err
	}
	defer file.Close()

	// Scan for require statements
	scanner := bufio.NewScanner(file)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for require block start
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		// Check for require block end
		if inRequireBlock && strings.HasPrefix(line, ")") {
			inRequireBlock = false
			continue
		}

		// Parse require line
		if inRequireBlock || strings.HasPrefix(line, "require ") {
			// Extract package name
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				packageName := parts[0]
				if packageName == "require" {
					packageName = parts[1]
				}

				// Check if package maps to a capability
				if capability, exists := cd.importToCapability[packageName]; exists {
					capabilities[capability] = true
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return []string{}, err
	}

	// Convert map to slice
	result := []string{}
	for cap := range capabilities {
		result = append(result, cap)
	}

	return result, nil
}

// DetectFromConfig reads capabilities from .aim/capabilities.json
func (cd *CapabilityDetector) DetectFromConfig() ([]string, error) {
	configPath := cd.getCapabilitiesConfigPath()
	if configPath == "" {
		return []string{}, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return []string{}, err
	}

	// Parse JSON
	var config CapabilitiesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return []string{}, err
	}

	return config.Capabilities, nil
}

// DetectFromRuntime detects capabilities from runtime environment
func (cd *CapabilityDetector) DetectFromRuntime() []string {
	capabilities := []string{}

	// Check if running with elevated permissions (sudo/admin)
	if cd.hasElevatedPermissions() {
		capabilities = append(capabilities, "elevated_permissions")
	}

	// Check network access
	if cd.hasNetworkAccess() {
		capabilities = append(capabilities, "network_access")
	}

	return capabilities
}

// findGoMod searches for go.mod file in current directory and parents
func (cd *CapabilityDetector) findGoMod() (string, error) {
	// Start from current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Search up the directory tree
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root directory
			break
		}
		currentDir = parentDir
	}

	return "", os.ErrNotExist
}

// getCapabilitiesConfigPath gets path to .aim/capabilities.json
func (cd *CapabilityDetector) getCapabilitiesConfigPath() string {
	// Check project-local config first
	localConfig := filepath.Join(".aim", "capabilities.json")
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}

	// Check home directory config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	homeConfig := filepath.Join(homeDir, ".aim", "capabilities.json")
	if _, err := os.Stat(homeConfig); err == nil {
		return homeConfig
	}

	return ""
}

// hasElevatedPermissions checks if running with elevated permissions
func (cd *CapabilityDetector) hasElevatedPermissions() bool {
	// On Unix-like systems, check if effective UID is 0 (root)
	if runtime.GOOS != "windows" {
		return os.Geteuid() == 0
	}
	// On Windows, this would require more complex checks
	return false
}

// hasNetworkAccess checks if network access is available
func (cd *CapabilityDetector) hasNetworkAccess() bool {
	// Simple check - assumes network is available
	// Could be enhanced with actual network checks
	return true
}

// AutoDetectCapabilities is a convenience function for quick capability detection
func AutoDetectCapabilities() ([]string, error) {
	detector := NewCapabilityDetector()
	result, err := detector.DetectAll()
	if err != nil {
		return []string{}, err
	}
	return result.Capabilities, nil
}

// SaveCapabilitiesConfig saves capabilities to .aim/capabilities.json
func SaveCapabilitiesConfig(capabilities []string) error {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Create .aim directory
	aimDir := filepath.Join(homeDir, ".aim")
	if err := os.MkdirAll(aimDir, 0700); err != nil {
		return err
	}

	// Create config
	config := CapabilitiesConfig{
		Capabilities: capabilities,
		LastUpdated:  "",
		Version:      "1.0.0",
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	configPath := filepath.Join(aimDir, "capabilities.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return err
	}

	return nil
}
