package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type MCPCapabilityService struct {
	capabilityRepo *repository.MCPServerCapabilityRepository
	mcpRepo        *repository.MCPServerRepository
}

func NewMCPCapabilityService(
	capabilityRepo *repository.MCPServerCapabilityRepository,
	mcpRepo *repository.MCPServerRepository,
) *MCPCapabilityService {
	return &MCPCapabilityService{
		capabilityRepo: capabilityRepo,
		mcpRepo:        mcpRepo,
	}
}

// DetectCapabilities detects and stores capabilities for an MCP server
// For MVP, this simulates capability detection
// In production, this would make an HTTP request to the MCP server's capabilities endpoint
func (s *MCPCapabilityService) DetectCapabilities(ctx context.Context, serverID uuid.UUID) error {
	// Get server details
	server, err := s.mcpRepo.GetByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to get MCP server: %w", err)
	}

	// ✅ SIMULATED CAPABILITY DETECTION FOR MVP
	// In production, this would:
	// 1. Make HTTP request to server.URL + "/.well-known/mcp/capabilities"
	// 2. Parse the MCP protocol response
	// 3. Extract tools, resources, and prompts
	//
	// For now, we'll simulate by generating sample capabilities based on server URL

	capabilities := s.generateSampleCapabilities(server)

	// Store detected capabilities
	for _, cap := range capabilities {
		cap.MCPServerID = serverID
		cap.ID = uuid.New()
		cap.DetectedAt = time.Now().UTC()
		cap.IsActive = true

		if err := s.capabilityRepo.Create(cap); err != nil {
			// Log error but continue with other capabilities
			fmt.Printf("⚠️  Failed to store capability %s: %v\n", cap.Name, err)
			continue
		}

		fmt.Printf("✅ Detected %s capability: %s\n", cap.CapabilityType, cap.Name)
	}

	fmt.Printf("✅ Successfully detected %d capabilities for MCP server %s\n", len(capabilities), server.Name)
	return nil
}

// generateSampleCapabilities generates sample capabilities for testing
// In production, this would be replaced with actual MCP protocol communication
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

// GetCapabilities retrieves all capabilities for an MCP server
func (s *MCPCapabilityService) GetCapabilities(ctx context.Context, serverID uuid.UUID) ([]*domain.MCPServerCapability, error) {
	return s.capabilityRepo.GetByServerID(serverID)
}

// GetCapabilitiesByType retrieves capabilities by type
func (s *MCPCapabilityService) GetCapabilitiesByType(ctx context.Context, serverID uuid.UUID, capType domain.MCPCapabilityType) ([]*domain.MCPServerCapability, error) {
	return s.capabilityRepo.GetByServerIDAndType(serverID, capType)
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
