package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type MCPCapabilityService struct {
	capabilityRepo *repository.MCPServerCapabilityRepository
	mcpRepo        *repository.MCPServerRepository
	httpClient     *http.Client
}

// MCPCapabilitiesResponse represents the standard MCP protocol capabilities response
type MCPCapabilitiesResponse struct {
	Tools     []MCPTool     `json:"tools,omitempty"`
	Resources []MCPResource `json:"resources,omitempty"`
	Prompts   []MCPPrompt   `json:"prompts,omitempty"`
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPResource struct {
	Name        string   `json:"name"`
	URI         string   `json:"uri"`
	Description string   `json:"description"`
	MimeTypes   []string `json:"mimeTypes,omitempty"`
}

type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type MCPPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

func NewMCPCapabilityService(
	capabilityRepo *repository.MCPServerCapabilityRepository,
	mcpRepo *repository.MCPServerRepository,
) *MCPCapabilityService {
	return &MCPCapabilityService{
		capabilityRepo: capabilityRepo,
		mcpRepo:        mcpRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 30 second timeout for capability discovery
		},
	}
}

// DetectCapabilities detects and stores capabilities for an MCP server
// ✅ REAL IMPLEMENTATION - Follows MCP Protocol Standard
// Makes HTTP GET request to /.well-known/mcp/capabilities endpoint
func (s *MCPCapabilityService) DetectCapabilities(ctx context.Context, serverID uuid.UUID) error {
	// Get server details
	server, err := s.mcpRepo.GetByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to get MCP server: %w", err)
	}

	// ✅ REAL MCP PROTOCOL CAPABILITY DETECTION
	// Step 1: Construct MCP capabilities endpoint URL
	// Parse the server URL to get base URL without path
	baseURL := server.URL
	// If URL has a path component (e.g., http://localhost:5555/mcp), extract base
	if idx := strings.Index(baseURL, "://"); idx != -1 {
		afterProto := baseURL[idx+3:]
		if slashIdx := strings.Index(afterProto, "/"); slashIdx != -1 {
			// Extract scheme://host:port only
			baseURL = baseURL[:idx+3+slashIdx]
		}
	}
	capabilitiesURL := strings.TrimSuffix(baseURL, "/") + "/.well-known/mcp/capabilities"

	fmt.Printf("🔍 Capability Detection for %s:\n", server.Name)
	fmt.Printf("   Original URL: %s\n", server.URL)
	fmt.Printf("   Base URL: %s\n", baseURL)
	fmt.Printf("   Capabilities URL: %s\n", capabilitiesURL)

	// Step 2: Make HTTP GET request to MCP server
	req, err := http.NewRequestWithContext(ctx, "GET", capabilitiesURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AIM/1.0 (Agent Identity Management)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		fmt.Printf("❌ Failed to fetch capabilities: %v\n", err)
		return fmt.Errorf("failed to fetch capabilities from %s: %w", capabilitiesURL, err)
	}
	defer resp.Body.Close()

	fmt.Printf("   Response Status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Non-200 status: %d\n", resp.StatusCode)
		return fmt.Errorf("MCP server returned non-200 status: %d", resp.StatusCode)
	}

	// Step 3: Parse MCP protocol response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var mcpResp MCPCapabilitiesResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return fmt.Errorf("failed to parse MCP capabilities response: %w", err)
	}

	// Step 4: Convert MCP protocol capabilities to domain objects
	capabilities := []*domain.MCPServerCapability{}

	// Convert tools
	for _, tool := range mcpResp.Tools {
		schemaJSON, _ := json.Marshal(tool.InputSchema)
		capabilities = append(capabilities, &domain.MCPServerCapability{
			ID:               uuid.New(),
			MCPServerID:      serverID,
			Name:             tool.Name,
			CapabilityType:   domain.MCPCapabilityTypeTool,
			Description:      tool.Description,
			CapabilitySchema: schemaJSON,
			DetectedAt:       time.Now().UTC(),
			IsActive:         true,
		})
	}

	// Convert resources
	for _, resource := range mcpResp.Resources {
		schema := map[string]interface{}{
			"uri":       resource.URI,
			"mimeTypes": resource.MimeTypes,
		}
		schemaJSON, _ := json.Marshal(schema)
		capabilities = append(capabilities, &domain.MCPServerCapability{
			ID:               uuid.New(),
			MCPServerID:      serverID,
			Name:             resource.Name,
			CapabilityType:   domain.MCPCapabilityTypeResource,
			Description:      resource.Description,
			CapabilitySchema: schemaJSON,
			DetectedAt:       time.Now().UTC(),
			IsActive:         true,
		})
	}

	// Convert prompts
	for _, prompt := range mcpResp.Prompts {
		schema := map[string]interface{}{
			"arguments": prompt.Arguments,
		}
		schemaJSON, _ := json.Marshal(schema)
		capabilities = append(capabilities, &domain.MCPServerCapability{
			ID:               uuid.New(),
			MCPServerID:      serverID,
			Name:             prompt.Name,
			CapabilityType:   domain.MCPCapabilityTypePrompt,
			Description:      prompt.Description,
			CapabilitySchema: schemaJSON,
			DetectedAt:       time.Now().UTC(),
			IsActive:         true,
		})
	}

	// Step 5: Store detected capabilities in database
	for _, cap := range capabilities {
		if err := s.capabilityRepo.Create(cap); err != nil {
			// Log error but continue with other capabilities
			fmt.Printf("⚠️  Failed to store capability %s: %v\n", cap.Name, err)
			continue
		}

		fmt.Printf("✅ Detected %s capability: %s\n", cap.CapabilityType, cap.Name)
	}

	fmt.Printf("✅ Successfully detected %d real capabilities from MCP server %s\n", len(capabilities), server.Name)
	return nil
}

// GetCapabilities retrieves all capabilities for an MCP server
func (s *MCPCapabilityService) GetCapabilities(ctx context.Context, serverID uuid.UUID) ([]*domain.MCPServerCapability, error) {
	return s.capabilityRepo.GetByServerID(serverID)
}

// GetCapabilitiesByType retrieves capabilities by type
func (s *MCPCapabilityService) GetCapabilitiesByType(ctx context.Context, serverID uuid.UUID, capType domain.MCPCapabilityType) ([]*domain.MCPServerCapability, error) {
	return s.capabilityRepo.GetByServerIDAndType(serverID, capType)
}

// ===== LEGACY SIMULATED METHODS (NO LONGER USED) =====
// The methods below were used for MVP simulation and are kept for reference only.
// Real capability detection now uses the MCP protocol standard.

// generateSampleCapabilities (DEPRECATED - DO NOT USE)
func (s *MCPCapabilityService) generateSampleCapabilities(server *domain.MCPServer) []*domain.MCPServerCapability {
	capabilities := []*domain.MCPServerCapability{}

	// Generate sample tools based on server URL patterns
	if containsAny(server.URL, []string{"openai", "gpt", "ai"}) {
		capabilities = append(capabilities,
			s.createToolCapability("generate_text", "Generate text using AI models", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt":      map[string]string{"type": "string", "description": "Text prompt"},
					"max_tokens":  map[string]string{"type": "integer", "description": "Maximum tokens to generate"},
					"temperature": map[string]string{"type": "number", "description": "Sampling temperature"},
				},
				"required": []string{"prompt"},
			}),
			s.createToolCapability("analyze_sentiment", "Analyze sentiment of text", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]string{"type": "string", "description": "Text to analyze"},
				},
				"required": []string{"text"},
			}),
		)

		capabilities = append(capabilities,
			s.createResourceCapability("models", "/models", "List available AI models", []string{"application/json"}),
		)

		capabilities = append(capabilities,
			s.createPromptCapability("code_review", "Review code for best practices and potential issues", []string{"code", "language"}),
		)
	}

	if containsAny(server.URL, []string{"github", "git", "code"}) {
		capabilities = append(capabilities,
			s.createToolCapability("search_code", "Search code repositories", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":      map[string]string{"type": "string", "description": "Search query"},
					"repository": map[string]string{"type": "string", "description": "Repository name"},
				},
				"required": []string{"query"},
			}),
			s.createToolCapability("create_pr", "Create a pull request", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":  map[string]string{"type": "string", "description": "PR title"},
					"body":   map[string]string{"type": "string", "description": "PR description"},
					"branch": map[string]string{"type": "string", "description": "Source branch"},
				},
				"required": []string{"title", "branch"},
			}),
		)

		capabilities = append(capabilities,
			s.createResourceCapability("repositories", "/repos/{owner}/{repo}", "Access repository data", []string{"application/json"}),
			s.createResourceCapability("issues", "/repos/{owner}/{repo}/issues", "Access issue data", []string{"application/json"}),
		)
	}

	if containsAny(server.URL, []string{"database", "postgres", "mysql", "sql"}) {
		capabilities = append(capabilities,
			s.createToolCapability("execute_query", "Execute SQL query", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]string{"type": "string", "description": "SQL query to execute"},
				},
				"required": []string{"query"},
			}),
		)

		capabilities = append(capabilities,
			s.createResourceCapability("tables", "/tables", "List database tables", []string{"application/json"}),
		)
	}

	// Default capabilities for any server
	if len(capabilities) == 0 {
		capabilities = append(capabilities,
			s.createToolCapability("health_check", "Check server health status", map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
			s.createResourceCapability("server_info", "/info", "Get server information", []string{"application/json"}),
		)
	}

	return capabilities
}

func (s *MCPCapabilityService) createToolCapability(name, description string, schema interface{}) *domain.MCPServerCapability {
	schemaJSON, _ := json.Marshal(schema)
	return &domain.MCPServerCapability{
		Name:             name,
		CapabilityType:   domain.MCPCapabilityTypeTool,
		Description:      description,
		CapabilitySchema: schemaJSON,
	}
}

func (s *MCPCapabilityService) createResourceCapability(name, uri, description string, mimeTypes []string) *domain.MCPServerCapability {
	schema := map[string]interface{}{
		"uri":       uri,
		"mimeTypes": mimeTypes,
	}
	schemaJSON, _ := json.Marshal(schema)
	return &domain.MCPServerCapability{
		Name:             name,
		CapabilityType:   domain.MCPCapabilityTypeResource,
		Description:      description,
		CapabilitySchema: schemaJSON,
	}
}

func (s *MCPCapabilityService) createPromptCapability(name, description string, arguments []string) *domain.MCPServerCapability {
	schema := map[string]interface{}{
		"arguments": arguments,
	}
	schemaJSON, _ := json.Marshal(schema)
	return &domain.MCPServerCapability{
		Name:             name,
		CapabilityType:   domain.MCPCapabilityTypePrompt,
		Description:      description,
		CapabilitySchema: schemaJSON,
	}
}

// Helper function to check if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
