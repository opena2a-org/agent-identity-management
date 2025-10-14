package aimsdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCapabilityDetector(t *testing.T) {
	detector := NewCapabilityDetector()

	if detector == nil {
		t.Fatal("NewCapabilityDetector() returned nil")
	}

	if detector.importToCapability == nil {
		t.Fatal("importToCapability map is nil")
	}

	// Check that some key mappings exist
	if _, exists := detector.importToCapability["os"]; !exists {
		t.Error("Expected 'os' package mapping to exist")
	}

	if _, exists := detector.importToCapability["database/sql"]; !exists {
		t.Error("Expected 'database/sql' package mapping to exist")
	}
}

func TestCapabilityDetectFromGoMod(t *testing.T) {
	detector := NewCapabilityDetector()

	// This test will work if there's a go.mod in the project
	capabilities, err := detector.DetectFromGoMod()

	if err != nil {
		// It's okay if go.mod doesn't exist in test environment
		t.Logf("go.mod not found (expected in some environments): %v", err)
		return
	}

	t.Logf("Detected capabilities from go.mod: %v", capabilities)

	// If we found a go.mod, we should have some capabilities
	if len(capabilities) == 0 {
		t.Log("Warning: No capabilities detected from go.mod")
	}
}

func TestDetectFromConfig(t *testing.T) {
	detector := NewCapabilityDetector()

	// Create a temporary config file
	tmpDir := t.TempDir()
	aimDir := filepath.Join(tmpDir, ".aim")
	if err := os.MkdirAll(aimDir, 0700); err != nil {
		t.Fatalf("Failed to create .aim directory: %v", err)
	}

	configPath := filepath.Join(aimDir, "capabilities.json")
	config := CapabilitiesConfig{
		Capabilities: []string{"read_files", "write_files", "access_database"},
		LastUpdated:  "2025-10-10T00:00:00Z",
		Version:      "1.0.0",
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Change to temp directory so detector finds our config
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Detect from config
	capabilities, err := detector.DetectFromConfig()
	if err != nil {
		t.Fatalf("DetectFromConfig() failed: %v", err)
	}

	// Verify capabilities
	if len(capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(capabilities))
	}

	expectedCaps := map[string]bool{
		"read_files":      true,
		"write_files":     true,
		"access_database": true,
	}

	for _, cap := range capabilities {
		if !expectedCaps[cap] {
			t.Errorf("Unexpected capability: %s", cap)
		}
	}
}

func TestDetectFromRuntime(t *testing.T) {
	detector := NewCapabilityDetector()

	capabilities := detector.DetectFromRuntime()

	// Should at least detect network access
	found := false
	for _, cap := range capabilities {
		if cap == "network_access" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'network_access' to be detected from runtime")
	}

	t.Logf("Runtime capabilities: %v", capabilities)
}

func TestDetectAll(t *testing.T) {
	detector := NewCapabilityDetector()

	result, err := detector.DetectAll()
	if err != nil {
		t.Fatalf("DetectAll() failed: %v", err)
	}

	if result == nil {
		t.Fatal("DetectAll() returned nil result")
	}

	// Should have some capabilities (at least from runtime)
	if len(result.Capabilities) == 0 {
		t.Log("Warning: No capabilities detected")
	}

	// Should have detected from runtime at minimum
	if len(result.DetectedFrom) == 0 {
		t.Error("Expected at least one detection source")
	}

	// Should have metadata
	if len(result.Metadata) == 0 {
		t.Error("Expected metadata to be populated")
	}

	// Check metadata fields
	if result.Metadata["go_version"] == "" {
		t.Error("Expected go_version in metadata")
	}

	if result.Metadata["os"] == "" {
		t.Error("Expected os in metadata")
	}

	if result.Metadata["arch"] == "" {
		t.Error("Expected arch in metadata")
	}

	t.Logf("Detection result: %+v", result)
}

func TestAutoDetectCapabilities(t *testing.T) {
	capabilities, err := AutoDetectCapabilities()
	if err != nil {
		t.Fatalf("AutoDetectCapabilities() failed: %v", err)
	}

	// Should return at least some capabilities
	if len(capabilities) == 0 {
		t.Log("Warning: No capabilities auto-detected")
	}

	t.Logf("Auto-detected capabilities: %v", capabilities)
}

func TestSaveCapabilitiesConfig(t *testing.T) {
	// Create temporary home directory
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	capabilities := []string{"read_files", "write_files", "execute_code"}

	err := SaveCapabilitiesConfig(capabilities)
	if err != nil {
		t.Fatalf("SaveCapabilitiesConfig() failed: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".aim", "capabilities.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Read and verify contents
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config CapabilitiesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	// Verify capabilities
	if len(config.Capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(config.Capabilities))
	}

	if config.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", config.Version)
	}
}

func TestImportToCapabilityMappings(t *testing.T) {
	detector := NewCapabilityDetector()

	// Test specific mappings
	testCases := []struct {
		packageName string
		expected    string
	}{
		{"os", "read_files"},
		{"database/sql", "access_database"},
		{"net/http", "make_api_calls"},
		{"os/exec", "execute_code"},
		{"net/smtp", "send_email"},
	}

	for _, tc := range testCases {
		if capability, exists := detector.importToCapability[tc.packageName]; exists {
			if capability != tc.expected {
				t.Errorf("Package %s: expected capability %s, got %s", tc.packageName, tc.expected, capability)
			}
		} else {
			t.Errorf("Package %s: expected mapping to exist", tc.packageName)
		}
	}
}
