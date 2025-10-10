package aimsdk

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// IntelligentDetectionConfig holds configuration for intelligent MCP detection
type IntelligentDetectionConfig struct {
	// Detection level
	Level string `json:"level"` // minimal, standard, deep

	// Tier 1 options (static analysis)
	ScanPackages    bool `json:"scan_packages"`
	ScanImports     bool `json:"scan_imports"`
	ScanConfigFiles bool `json:"scan_config_files"`

	// Tier 2 options (runtime hooks)
	HookPackageLoads  bool `json:"hook_package_loads"`
	HookExecCommands  bool `json:"hook_exec_commands"`
	HookNetDialer     bool `json:"hook_net_dialer"`

	// Tier 3 options (deep inspection - requires explicit opt-in)
	EnableASTAnalysis       bool `json:"enable_ast_analysis"`
	EnableDeepDependencyTree bool `json:"enable_deep_dependency_tree"`
	EnableNetworkMonitoring bool `json:"enable_network_monitoring"`

	// Performance options
	CacheTimeout      int  `json:"cache_timeout"` // milliseconds
	WatchForChanges   bool `json:"watch_for_changes"`
	MaxDetectionTimeMs int  `json:"max_detection_time_ms"`
}

// PerformanceMetrics tracks detection performance
type PerformanceMetrics struct {
	DetectionTimeMs    float64 `json:"detection_time_ms"`
	Tier1TimeMs        float64 `json:"tier1_time_ms"`
	Tier2TimeMs        float64 `json:"tier2_time_ms"`
	Tier3TimeMs        float64 `json:"tier3_time_ms,omitempty"`
	CPUOverheadPercent float64 `json:"cpu_overhead_percent"`
	MemoryUsageMb      float64 `json:"memory_usage_mb"`
	CacheHitRate       float64 `json:"cache_hit_rate"`
	MCPsDetected       int     `json:"mcps_detected"`
}

// IntelligentDetectionResult extends DetectionResult with performance metrics
type IntelligentDetectionResult struct {
	MCPs               []MCPCapability           `json:"mcps"`
	DetectedAt         string                    `json:"detected_at"`
	Runtime            map[string]string         `json:"runtime"`
	PerformanceMetrics PerformanceMetrics        `json:"performance_metrics"`
	DetectionConfig    IntelligentDetectionConfig `json:"detection_config"`
}

// Known MCP Go modules for fast lookup
var knownMCPModules = map[string]bool{
	"github.com/modelcontextprotocol/go-sdk":               true,
	"github.com/modelcontextprotocol/server-filesystem":    true,
	"github.com/modelcontextprotocol/server-github":        true,
	"github.com/opena2a/sequential-thinking-mcp":           true,
}

// Detection cache
type detectionCache struct {
	mcps       []MCPCapability
	detectedAt time.Time
	ttl        time.Duration
	mu         sync.RWMutex
}

var globalCache *detectionCache

// Default configuration (Tier 1 + Tier 2 - recommended)
func getDefaultConfig() IntelligentDetectionConfig {
	return IntelligentDetectionConfig{
		Level: "standard",

		// Tier 1 (always enabled)
		ScanPackages:    true,
		ScanImports:     true,
		ScanConfigFiles: true,

		// Tier 2 (enabled in standard mode)
		HookPackageLoads: true,
		HookExecCommands: true,
		HookNetDialer:    true,

		// Tier 3 (disabled by default)
		EnableASTAnalysis:       false,
		EnableDeepDependencyTree: false,
		EnableNetworkMonitoring: false,

		// Performance
		CacheTimeout:      300000, // 5 minutes
		WatchForChanges:   true,
		MaxDetectionTimeMs: 100,
	}
}

// IntelligentAutoDetectMCPs performs intelligent MCP detection with performance monitoring
func IntelligentAutoDetectMCPs(config *IntelligentDetectionConfig) (*IntelligentDetectionResult, error) {
	startTime := time.Now()

	// Use default config if not provided
	cfg := getDefaultConfig()

	// If user provided a config with only Level set (all other fields are zero),
	// use defaults. Otherwise, merge their explicit values.
	if config != nil {
		// Check if this is a "level-only" config (all bools are false = zero value)
		isLevelOnly := !config.ScanPackages &&
			!config.ScanImports &&
			!config.ScanConfigFiles &&
			!config.HookPackageLoads &&
			!config.HookExecCommands &&
			!config.HookNetDialer &&
			!config.EnableASTAnalysis &&
			!config.EnableDeepDependencyTree &&
			!config.EnableNetworkMonitoring &&
			!config.WatchForChanges &&
			config.CacheTimeout == 0 &&
			config.MaxDetectionTimeMs == 0

		if config.Level != "" {
			cfg.Level = config.Level
		}

		// Apply level presets
		if cfg.Level == "minimal" {
			cfg.HookPackageLoads = false
			cfg.HookExecCommands = false
			cfg.HookNetDialer = false
		} else if cfg.Level == "deep" {
			cfg.EnableASTAnalysis = true
			cfg.EnableDeepDependencyTree = true
		}

		// If NOT a level-only config, apply user overrides
		if !isLevelOnly {
			cfg.ScanPackages = config.ScanPackages
			cfg.ScanImports = config.ScanImports
			cfg.ScanConfigFiles = config.ScanConfigFiles
			cfg.HookPackageLoads = config.HookPackageLoads
			cfg.HookExecCommands = config.HookExecCommands
			cfg.HookNetDialer = config.HookNetDialer
			cfg.EnableASTAnalysis = config.EnableASTAnalysis
			cfg.EnableDeepDependencyTree = config.EnableDeepDependencyTree
			cfg.EnableNetworkMonitoring = config.EnableNetworkMonitoring
			cfg.WatchForChanges = config.WatchForChanges
			if config.CacheTimeout > 0 {
				cfg.CacheTimeout = config.CacheTimeout
			}
			if config.MaxDetectionTimeMs > 0 {
				cfg.MaxDetectionTimeMs = config.MaxDetectionTimeMs
			}
		}
	}

	// Check cache first
	if globalCache != nil {
		if cached := getCachedResult(&cfg); cached != nil {
			return cached, nil
		}
	}

	// Initialize with empty slice (never nil)
	mcps := make([]MCPCapability, 0)
	var tier1Time, tier2Time, tier3Time time.Duration

	// === TIER 1: Static Detection ===
	tier1Start := time.Now()

	// 1. Scan go.mod for MCP packages
	if cfg.ScanPackages {
		packageMCPs, err := detectFromGoMod()
		if err == nil {
			mcps = append(mcps, packageMCPs...)
		}
	}

	// 2. Scan import statements in Go files
	if cfg.ScanImports {
		importMCPs, err := detectFromGoImports()
		if err == nil {
			mcps = append(mcps, importMCPs...)
		}
	}

	// 3. Scan config files (backward compatibility)
	if cfg.ScanConfigFiles {
		configMCPs, err := detectFromConfigFilesIntelligent()
		if err == nil {
			mcps = append(mcps, configMCPs...)
		}
	}

	tier1Time = time.Since(tier1Start)

	// === TIER 2: Runtime Hooks ===
	if cfg.Level != "minimal" {
		tier2Start := time.Now()

		if cfg.HookExecCommands {
			// Note: In Go, we can't easily hook os/exec at runtime like JavaScript
			// Instead, we detect from process table or existing exec calls
			execMCPs := detectFromProcessTable()
			mcps = append(mcps, execMCPs...)
		}

		tier2Time = time.Since(tier2Start)
	}

	// === TIER 3: Deep Inspection (opt-in only) ===
	if cfg.Level == "deep" {
		tier3Start := time.Now()

		if cfg.EnableASTAnalysis {
			fmt.Println("[AIM SDK] ⚠️  AST analysis enabled - may add ~50ms per file")
			// TODO: Implement AST parsing with go/ast package
		}

		if cfg.EnableDeepDependencyTree {
			fmt.Println("[AIM SDK] ⚠️  Deep dependency analysis enabled - may add ~500ms")
			// TODO: Implement deep module graph analysis
		}

		if cfg.EnableNetworkMonitoring {
			fmt.Println(
				"[AIM SDK] ⚠️  Network monitoring enabled - This may add 2-5% CPU overhead. " +
				"Make sure you have user consent for traffic monitoring.",
			)
			// TODO: Implement network traffic monitoring
		}

		tier3Time = time.Since(tier3Start)
	}

	// Deduplicate MCPs by name
	uniqueMCPs := deduplicateMCPsIntelligent(mcps)

	// Update cache
	updateCache(uniqueMCPs, time.Duration(cfg.CacheTimeout)*time.Millisecond)

	// Calculate performance metrics
	totalTime := time.Since(startTime)
	metrics := PerformanceMetrics{
		DetectionTimeMs:    float64(totalTime.Milliseconds()),
		Tier1TimeMs:        float64(tier1Time.Milliseconds()),
		Tier2TimeMs:        float64(tier2Time.Milliseconds()),
		Tier3TimeMs:        float64(tier3Time.Milliseconds()),
		CPUOverheadPercent: estimateCPUOverheadGo(&cfg),
		MemoryUsageMb:      getMemoryUsageGo(),
		CacheHitRate:       0.0,
		MCPsDetected:       len(uniqueMCPs),
	}

	// Warn if detection is slow
	if totalTime.Milliseconds() > int64(cfg.MaxDetectionTimeMs) {
		fmt.Printf(
			"[AIM SDK] ⚠️  MCP detection took %dms (expected <%dms). "+
			"Consider using 'minimal' mode for faster startup.\n",
			totalTime.Milliseconds(), cfg.MaxDetectionTimeMs,
		)
	}

	return &IntelligentDetectionResult{
		MCPs:               uniqueMCPs,
		DetectedAt:         time.Now().UTC().Format(time.RFC3339),
		Runtime:            collectRuntimeInfo(),
		PerformanceMetrics: metrics,
		DetectionConfig:    cfg,
	}, nil
}

// Tier 1: Detect MCPs from go.mod file
func detectFromGoMod() ([]MCPCapability, error) {
	// Initialize with empty slice (never nil)
	mcps := make([]MCPCapability, 0)

	cwd, err := os.Getwd()
	if err != nil {
		return mcps, err
	}

	goModPath := filepath.Join(cwd, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return mcps, nil
	}

	file, err := os.Open(goModPath)
	if err != nil {
		return mcps, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines, comments, and block delimiters
		if line == "" || strings.HasPrefix(line, "//") || line == "(" || line == ")" {
			continue
		}

		// Skip module/go directive lines
		if strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "go ") {
			continue
		}

		// Parse require statements (both inline and block form)
		// Inline: require github.com/foo/bar v1.0.0
		// Block:  github.com/foo/bar v1.0.0 (inside require block)
		if strings.HasPrefix(line, "require") || strings.Contains(line, "/") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				// Get module name (skip "require" keyword if present)
				moduleName := parts[0]
				if moduleName == "require" && len(parts) >= 2 {
					moduleName = parts[1]
				}

				// Check if it's an MCP module
				if isMCPModule(moduleName) {
					mcps = append(mcps, MCPCapability{
						Name:         moduleName,
						Type:         "module",
						Command:      "go",
						Args:         []string{"run", moduleName},
						DetectedFrom: goModPath,
						Capabilities: inferCapabilitiesFromModuleName(moduleName),
					})
				}
			}
		}
	}

	return mcps, scanner.Err()
}

// Tier 1: Detect MCPs from Go import statements
func detectFromGoImports() ([]MCPCapability, error) {
	// Initialize with empty slice (never nil)
	mcps := make([]MCPCapability, 0)

	cwd, err := os.Getwd()
	if err != nil {
		return mcps, err
	}

	// Common entry points
	entryPoints := []string{"main.go", "cmd/main.go", "cmd/server/main.go"}

	for _, entry := range entryPoints {
		filePath := filepath.Join(cwd, entry)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// Match import statements
			if strings.Contains(line, "import") && strings.Contains(line, `"`) {
				start := strings.Index(line, `"`)
				end := strings.LastIndex(line, `"`)
				if start != -1 && end != -1 && start < end {
					moduleName := line[start+1 : end]

					if isMCPModule(moduleName) {
						mcps = append(mcps, MCPCapability{
							Name:         moduleName,
							Type:         "import",
							Command:      "go",
							Args:         []string{},
							DetectedFrom: filePath,
							Capabilities: inferCapabilitiesFromModuleName(moduleName),
						})
					}
				}
			}
		}
	}

	return mcps, nil
}

// Tier 1: Detect MCPs from config files (backward compatibility)
func detectFromConfigFilesIntelligent() ([]MCPCapability, error) {
	result, err := AutoDetectMCPs()
	if err != nil {
		return []MCPCapability{}, err
	}
	return result.MCPs, nil
}

// Tier 2: Detect MCPs from running processes
func detectFromProcessTable() []MCPCapability {
	// Note: This is a simplified implementation
	// In production, you'd use ps or /proc on Linux, tasklist on Windows
	// Initialize with empty slice (never nil)
	mcps := make([]MCPCapability, 0)

	// TODO: Implement process table scanning
	// For now, return empty to avoid runtime overhead

	return mcps
}

// Check if Go module is a known MCP module
func isMCPModule(name string) bool {
	if knownMCPModules[name] {
		return true
	}

	nameLower := strings.ToLower(name)
	mcpPatterns := []string{
		"modelcontextprotocol",
		"mcp-server",
		"sequential-thinking",
	}

	for _, pattern := range mcpPatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	return false
}

// Infer capabilities from Go module name
func inferCapabilitiesFromModuleName(name string) []string {
	capabilities := []string{}
	nameLower := strings.ToLower(name)

	patterns := map[string][]string{
		"filesystem": {"filesystem", "fs", "file"},
		"database":   {"sqlite", "postgres", "mysql", "mongodb", "db"},
		"github":     {"github", "git"},
		"sequential": {"sequential", "thinking"},
	}

	for cap, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(nameLower, keyword) {
				capabilities = append(capabilities, cap)
				break
			}
		}
	}

	return capabilities
}

// Deduplicate MCPs by name
func deduplicateMCPsIntelligent(mcps []MCPCapability) []MCPCapability {
	seen := make(map[string]bool)
	unique := []MCPCapability{}

	for _, mcp := range mcps {
		if !seen[mcp.Name] {
			seen[mcp.Name] = true
			unique = append(unique, mcp)
		}
	}

	return unique
}

// Get cached result if still valid
func getCachedResult(cfg *IntelligentDetectionConfig) *IntelligentDetectionResult {
	if globalCache == nil {
		return nil
	}

	globalCache.mu.RLock()
	defer globalCache.mu.RUnlock()

	// Check if cache is valid (not expired and has data)
	if time.Since(globalCache.detectedAt) < globalCache.ttl && globalCache.mcps != nil {
		return &IntelligentDetectionResult{
			MCPs:       globalCache.mcps,
			DetectedAt: globalCache.detectedAt.Format(time.RFC3339),
			Runtime:    collectRuntimeInfo(),
			PerformanceMetrics: PerformanceMetrics{
				DetectionTimeMs:    0,
				Tier1TimeMs:        0,
				Tier2TimeMs:        0,
				CPUOverheadPercent: 0,
				MemoryUsageMb:      getMemoryUsageGo(),
				CacheHitRate:       1.0,
				MCPsDetected:       len(globalCache.mcps),
			},
			DetectionConfig: *cfg,
		}
	}

	return nil
}

// Update detection cache
func updateCache(mcps []MCPCapability, ttl time.Duration) {
	if globalCache == nil {
		globalCache = &detectionCache{}
	}

	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	globalCache.mcps = mcps
	globalCache.detectedAt = time.Now()
	globalCache.ttl = ttl
}

// Estimate CPU overhead based on configuration
func estimateCPUOverheadGo(config *IntelligentDetectionConfig) float64 {
	var overhead float64

	if config.HookPackageLoads {
		overhead += 0.03
	}
	if config.HookExecCommands {
		overhead += 0.03
	}
	if config.HookNetDialer {
		overhead += 0.04
	}
	if config.EnableASTAnalysis {
		overhead += 0.5
	}
	if config.EnableDeepDependencyTree {
		overhead += 1.0
	}
	if config.EnableNetworkMonitoring {
		overhead += 3.0
	}

	return overhead
}

// Get current memory usage in MB
func getMemoryUsageGo() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// InvalidateDetectionCache clears the detection cache
func InvalidateDetectionCache() {
	if globalCache != nil {
		globalCache.mu.Lock()
		globalCache.mcps = nil
		globalCache.detectedAt = time.Time{} // Reset to zero time so cache is invalid
		globalCache.mu.Unlock()
	}
}
