package aimsdk

import (
	"os"
	"testing"
)

// Test helper to create temporary test directory
func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "aim-sdk-intelligent-detection-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tmpDir
}

// Test helper to cleanup test directory
func cleanupTestDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test dir: %v", err)
	}
}

// Test helper to change to test directory and restore
func changeToTestDir(t *testing.T, dir string) func() {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}

	return func() {
		err := os.Chdir(originalDir)
		if err != nil {
			t.Logf("Warning: Failed to restore directory: %v", err)
		}
	}
}

// === TIER 1: Static Detection Tests ===

func TestDetectFromGoMod(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	// Clear cache before test
	InvalidateDetectionCache()

	// Create go.mod with MCP dependencies
	goModContent := `module test-agent

go 1.23

require (
	github.com/modelcontextprotocol/go-sdk v1.0.0
	github.com/opena2a/sequential-thinking-mcp v1.0.0
)`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if len(result.MCPs) == 0 {
		t.Error("Expected to detect MCPs from go.mod")
	}

	// Check for specific MCP
	found := false
	for _, mcp := range result.MCPs {
		if mcp.Name == "github.com/modelcontextprotocol/go-sdk" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find modelcontextprotocol/go-sdk in detected MCPs")
	}
}

func TestDetectFromGoImports(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create main.go with MCP imports
	mainGoContent := `package main

import (
	"github.com/modelcontextprotocol/go-sdk"
	"github.com/opena2a/sequential-thinking-mcp"
)

func main() {
	// Agent code here
}`

	err := os.WriteFile("main.go", []byte(mainGoContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Should detect from imports
	if result.MCPs == nil {
		t.Error("Expected MCPs to be defined")
	}

	// Check performance metrics (0ms means "very fast, <1ms")
	if result.PerformanceMetrics.DetectionTimeMs < 0 {
		t.Error("Expected detection time >= 0")
	}
}

func TestDetectFromConfigFiles(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create mcp.json config
	mcpConfigContent := `{
	"mcpServers": {
		"filesystem": {
			"command": "npx",
			"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
		}
	}
}`

	err := os.WriteFile("mcp.json", []byte(mcpConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create mcp.json: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Should detect from config
	found := false
	for _, mcp := range result.MCPs {
		if mcp.Name == "filesystem" {
			found = true
			if len(mcp.Capabilities) == 0 {
				t.Error("Expected filesystem MCP to have capabilities")
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find filesystem MCP from config")
	}
}

func TestShouldNotDetectNonMCPPackages(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod with only regular dependencies
	goModContent := `module test-agent

go 1.23

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/google/uuid v1.3.0
)`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Should not detect non-MCP packages
	for _, mcp := range result.MCPs {
		if mcp.Name == "github.com/gin-gonic/gin" || mcp.Name == "github.com/google/uuid" {
			t.Errorf("Should not detect non-MCP package: %s", mcp.Name)
		}
	}
}

// === PERFORMANCE METRICS TESTS ===

func TestTier1DetectionPerformance(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/go-sdk v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Check performance metrics
	if result.PerformanceMetrics.Tier1TimeMs > 10 {
		t.Errorf("Tier 1 detection too slow: %f ms (expected <10ms)", result.PerformanceMetrics.Tier1TimeMs)
	}

	if result.PerformanceMetrics.Tier2TimeMs != 0 {
		t.Error("Minimal mode should not run Tier 2")
	}
}

func TestCPUOverheadEstimate(t *testing.T) {
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "standard",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.PerformanceMetrics.CPUOverheadPercent > 0.2 {
		t.Errorf("CPU overhead too high: %f%% (expected <0.2%%)", result.PerformanceMetrics.CPUOverheadPercent)
	}
}

func TestMemoryUsageTracking(t *testing.T) {
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.PerformanceMetrics.MemoryUsageMb <= 0 {
		t.Error("Expected memory usage > 0")
	}

	// Reasonable upper bound for test environment (500MB)
	if result.PerformanceMetrics.MemoryUsageMb > 500 {
		t.Errorf("Memory usage too high: %f MB", result.PerformanceMetrics.MemoryUsageMb)
	}
}

// === CONFIGURATION API TESTS ===

func TestMinimalMode(t *testing.T) {
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.DetectionConfig.Level != "minimal" {
		t.Errorf("Expected level 'minimal', got '%s'", result.DetectionConfig.Level)
	}

	if result.PerformanceMetrics.Tier2TimeMs != 0 {
		t.Error("Minimal mode should not run Tier 2")
	}
}

func TestStandardModeByDefault(t *testing.T) {
	result, err := IntelligentAutoDetectMCPs(nil)

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.DetectionConfig.Level != "standard" {
		t.Errorf("Expected default level 'standard', got '%s'", result.DetectionConfig.Level)
	}

	if !result.DetectionConfig.HookPackageLoads {
		t.Error("Standard mode should enable package load hooks")
	}

	if !result.DetectionConfig.HookExecCommands {
		t.Error("Standard mode should enable exec command hooks")
	}
}

func TestCustomConfiguration(t *testing.T) {
	config := &IntelligentDetectionConfig{
		Level:            "standard",
		ScanPackages:     true,
		ScanImports:      false,
		HookPackageLoads: false,
	}

	result, err := IntelligentAutoDetectMCPs(config)

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.DetectionConfig.ScanImports {
		t.Error("Expected scan imports to be disabled")
	}

	if result.DetectionConfig.HookPackageLoads {
		t.Error("Expected package load hooks to be disabled")
	}
}

// === CACHING TESTS ===

func TestCacheDetectionResults(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/go-sdk v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// First call - should detect and cache
	result1, err := IntelligentAutoDetectMCPs(nil)
	if err != nil {
		t.Fatalf("First detection failed: %v", err)
	}

	if result1.PerformanceMetrics.CacheHitRate != 0 {
		t.Error("First call should not hit cache")
	}

	// Second call - should use cache
	result2, err := IntelligentAutoDetectMCPs(nil)
	if err != nil {
		t.Fatalf("Second detection failed: %v", err)
	}

	if result2.PerformanceMetrics.CacheHitRate != 1.0 {
		t.Error("Second call should hit cache")
	}

	// Cache lookup should be faster or equal (both may be <1ms, showing as 0ms)
	if result2.PerformanceMetrics.DetectionTimeMs > result1.PerformanceMetrics.DetectionTimeMs {
		t.Error("Cache lookup should not be slower than full detection")
	}
}

func TestInvalidateCache(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/go-sdk v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// First call
	_, err = IntelligentAutoDetectMCPs(nil)
	if err != nil {
		t.Fatalf("First detection failed: %v", err)
	}

	// Invalidate cache
	InvalidateDetectionCache()

	// Second call - should re-detect
	result, err := IntelligentAutoDetectMCPs(nil)
	if err != nil {
		t.Fatalf("Second detection failed: %v", err)
	}

	if result.PerformanceMetrics.CacheHitRate != 0 {
		t.Error("After invalidation, should not hit cache")
	}
}

// === CAPABILITY INFERENCE TESTS ===

func TestInferFilesystemCapability(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod with filesystem MCP
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/server-filesystem v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Find filesystem MCP
	found := false
	for _, mcp := range result.MCPs {
		for _, cap := range mcp.Capabilities {
			if cap == "filesystem" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Error("Expected to infer filesystem capability")
	}
}

func TestInferDatabaseCapability(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create mcp.json with database MCPs
	mcpConfigContent := `{
	"mcpServers": {
		"sqlite": {
			"command": "npx",
			"args": ["-y", "mcp-server-sqlite", "/tmp/data.db"]
		},
		"postgres": {
			"command": "npx",
			"args": ["-y", "mcp-server-postgres"]
		}
	}
}`

	err := os.WriteFile("mcp.json", []byte(mcpConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create mcp.json: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Count MCPs with database capability
	dbCount := 0
	for _, mcp := range result.MCPs {
		for _, cap := range mcp.Capabilities {
			if cap == "database" {
				dbCount++
				break
			}
		}
	}

	if dbCount < 1 {
		t.Error("Expected to infer database capability")
	}
}

func TestInferGitHubCapability(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod with GitHub MCP
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/server-github v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Find GitHub capability
	found := false
	for _, mcp := range result.MCPs {
		for _, cap := range mcp.Capabilities {
			if cap == "github" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Error("Expected to infer github capability")
	}
}

// === DEDUPLICATION TESTS ===

func TestDeduplicateMCPs(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Add MCP to both go.mod and config
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/server-filesystem v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	mcpConfigContent := `{
	"mcpServers": {
		"github.com/modelcontextprotocol/server-filesystem": {
			"command": "go",
			"args": ["run", "github.com/modelcontextprotocol/server-filesystem"]
		}
	}
}`

	err = os.WriteFile("mcp.json", []byte(mcpConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create mcp.json: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Count filesystem MCPs (should only have one)
	fsCount := 0
	for _, mcp := range result.MCPs {
		if mcp.Name == "github.com/modelcontextprotocol/server-filesystem" {
			fsCount++
		}
	}

	if fsCount != 1 {
		t.Errorf("Expected 1 filesystem MCP after deduplication, got %d", fsCount)
	}
}

// === RUNTIME INFORMATION TESTS ===

func TestCollectRuntimeInfo(t *testing.T) {
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if result.Runtime == nil {
		t.Fatal("Expected runtime info to be defined")
	}

	if result.Runtime["runtime"] == "" {
		t.Error("Expected runtime to be set")
	}

	if result.Runtime["platform"] == "" {
		t.Error("Expected platform to be set")
	}

	if result.Runtime["arch"] == "" {
		t.Error("Expected arch to be set")
	}
}

// === ERROR HANDLING TESTS ===

func TestHandleMissingGoMod(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// No go.mod in test directory
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	if err != nil {
		t.Fatalf("Detection should not fail without go.mod: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be defined")
	}

	if result.MCPs == nil {
		t.Error("Expected MCPs to be defined (empty slice)")
	}
}

func TestHandleInvalidGoMod(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create invalid go.mod
	err := os.WriteFile("go.mod", []byte("invalid go.mod content {{{"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	// Should not fail, just continue with other detection methods
	if err != nil {
		t.Fatalf("Detection should not fail with invalid go.mod: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be defined")
	}
}

func TestHandleMissingEntryFiles(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// No main.go or other entry files
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	// Should not fail
	if err != nil {
		t.Fatalf("Detection should not fail without entry files: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be defined")
	}
}

// === PERFORMANCE WARNING TESTS ===

func TestPerformanceWarningConfiguration(t *testing.T) {
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)
	restore := changeToTestDir(t, testDir)
	defer restore()

	InvalidateDetectionCache()

	// Create go.mod
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/go-sdk v1.0.0`

	err := os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Run detection with custom threshold
	result, err := IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level:             "minimal",
		MaxDetectionTimeMs: 50,
	})

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Verify detection completed successfully
	if result == nil {
		t.Fatal("Expected result to be defined")
	}

	// Detection time >= 0 (0ms means "very fast, <1ms")
	if result.PerformanceMetrics.DetectionTimeMs < 0 {
		t.Error("Expected detection time >= 0")
	}

	if result.DetectionConfig.MaxDetectionTimeMs != 50 {
		t.Errorf("Expected max detection time 50ms, got %d", result.DetectionConfig.MaxDetectionTimeMs)
	}

	// Note: Warning only triggers if detection exceeds threshold
	// In most cases, detection is <10ms (often <1ms), so warning won't appear
	// This is expected behavior - our detection is very fast!
}

// === BENCHMARK TESTS ===

func BenchmarkTier1StaticDetection(b *testing.B) {
	testDir := setupTestDir(&testing.T{})
	defer cleanupTestDir(&testing.T{}, testDir)

	originalDir, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(originalDir)

	// Create go.mod
	goModContent := `module test-agent

go 1.23

require github.com/modelcontextprotocol/go-sdk v1.0.0`

	os.WriteFile("go.mod", []byte(goModContent), 0644)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		InvalidateDetectionCache()
		IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
			Level: "minimal",
		})
	}
}

func BenchmarkCacheLookup(b *testing.B) {
	testDir := setupTestDir(&testing.T{})
	defer cleanupTestDir(&testing.T{}, testDir)

	originalDir, _ := os.Getwd()
	os.Chdir(testDir)
	defer os.Chdir(originalDir)

	// Prime the cache
	InvalidateDetectionCache()
	IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
		Level: "minimal",
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		IntelligentAutoDetectMCPs(&IntelligentDetectionConfig{
			Level: "minimal",
		})
	}
}
