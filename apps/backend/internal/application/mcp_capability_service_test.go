package application

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========================================
// MCPCapabilitiesResponse Tests
// ========================================

func TestMCPCapabilitiesResponse_Structure(t *testing.T) {
	resp := MCPCapabilitiesResponse{
		Tools: []MCPTool{
			{
				Name:        "read_file",
				Description: "Read contents of a file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the file",
						},
					},
				},
			},
		},
		Resources: []MCPResource{
			{
				Name:        "documents",
				URI:         "file:///documents",
				Description: "User documents directory",
				MimeTypes:   []string{"text/plain", "application/pdf"},
			},
		},
		Prompts: []MCPPrompt{
			{
				Name:        "summarize",
				Description: "Summarize text content",
				Arguments: []MCPPromptArgument{
					{Name: "text", Description: "Text to summarize", Required: true},
				},
			},
		},
	}

	assert.Len(t, resp.Tools, 1)
	assert.Len(t, resp.Resources, 1)
	assert.Len(t, resp.Prompts, 1)

	assert.Equal(t, "read_file", resp.Tools[0].Name)
	assert.Equal(t, "documents", resp.Resources[0].Name)
	assert.Equal(t, "summarize", resp.Prompts[0].Name)
}

func TestMCPCapabilitiesResponse_JSONSerialization(t *testing.T) {
	resp := MCPCapabilitiesResponse{
		Tools: []MCPTool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
		Resources: []MCPResource{
			{
				Name:        "test_resource",
				URI:         "file:///test",
				Description: "A test resource",
			},
		},
		Prompts: []MCPPrompt{
			{
				Name:        "test_prompt",
				Description: "A test prompt",
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize back
	var restored MCPCapabilitiesResponse
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.Equal(t, len(resp.Tools), len(restored.Tools))
	assert.Equal(t, len(resp.Resources), len(restored.Resources))
	assert.Equal(t, len(resp.Prompts), len(restored.Prompts))
}

func TestMCPCapabilitiesResponse_EmptyResponse(t *testing.T) {
	resp := MCPCapabilitiesResponse{}

	assert.Empty(t, resp.Tools)
	assert.Empty(t, resp.Resources)
	assert.Empty(t, resp.Prompts)

	// Empty response should still serialize fine
	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	var restored MCPCapabilitiesResponse
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)
}

// ========================================
// MCPTool Tests
// ========================================

func TestMCPTool_Structure(t *testing.T) {
	tool := MCPTool{
		Name:        "execute_command",
		Description: "Execute a shell command",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Command to execute",
				},
				"args": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Command arguments",
				},
			},
			"required": []string{"command"},
		},
	}

	assert.Equal(t, "execute_command", tool.Name)
	assert.Equal(t, "Execute a shell command", tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.Equal(t, "object", tool.InputSchema["type"])
}

func TestMCPTool_JSONSerialization(t *testing.T) {
	tool := MCPTool{
		Name:        "test",
		Description: "test description",
		InputSchema: map[string]interface{}{"type": "string"},
	}

	data, err := json.Marshal(tool)
	assert.NoError(t, err)

	var restored MCPTool
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.Equal(t, tool.Name, restored.Name)
	assert.Equal(t, tool.Description, restored.Description)
}

// ========================================
// MCPResource Tests
// ========================================

func TestMCPResource_Structure(t *testing.T) {
	resource := MCPResource{
		Name:        "user_files",
		URI:         "file:///home/user/files",
		Description: "User file storage",
		MimeTypes:   []string{"text/plain", "application/json", "image/png"},
	}

	assert.Equal(t, "user_files", resource.Name)
	assert.Equal(t, "file:///home/user/files", resource.URI)
	assert.Equal(t, "User file storage", resource.Description)
	assert.Len(t, resource.MimeTypes, 3)
	assert.Contains(t, resource.MimeTypes, "text/plain")
	assert.Contains(t, resource.MimeTypes, "application/json")
}

func TestMCPResource_JSONSerialization(t *testing.T) {
	resource := MCPResource{
		Name:        "test_resource",
		URI:         "https://example.com/resource",
		Description: "External resource",
		MimeTypes:   []string{"application/xml"},
	}

	data, err := json.Marshal(resource)
	assert.NoError(t, err)

	var restored MCPResource
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.Equal(t, resource.Name, restored.Name)
	assert.Equal(t, resource.URI, restored.URI)
	assert.Equal(t, resource.MimeTypes, restored.MimeTypes)
}

func TestMCPResource_EmptyMimeTypes(t *testing.T) {
	resource := MCPResource{
		Name:        "generic_resource",
		URI:         "file:///data",
		Description: "Generic resource without specific MIME types",
		// MimeTypes intentionally empty
	}

	assert.Empty(t, resource.MimeTypes)
}

// ========================================
// MCPPrompt Tests
// ========================================

func TestMCPPrompt_Structure(t *testing.T) {
	prompt := MCPPrompt{
		Name:        "generate_summary",
		Description: "Generate a summary of the provided content",
		Arguments: []MCPPromptArgument{
			{
				Name:        "content",
				Description: "The content to summarize",
				Required:    true,
			},
			{
				Name:        "max_length",
				Description: "Maximum length of summary in words",
				Required:    false,
			},
		},
	}

	assert.Equal(t, "generate_summary", prompt.Name)
	assert.Equal(t, "Generate a summary of the provided content", prompt.Description)
	assert.Len(t, prompt.Arguments, 2)
	assert.True(t, prompt.Arguments[0].Required)
	assert.False(t, prompt.Arguments[1].Required)
}

func TestMCPPrompt_JSONSerialization(t *testing.T) {
	prompt := MCPPrompt{
		Name:        "test_prompt",
		Description: "Test prompt description",
		Arguments: []MCPPromptArgument{
			{Name: "arg1", Description: "First argument", Required: true},
		},
	}

	data, err := json.Marshal(prompt)
	assert.NoError(t, err)

	var restored MCPPrompt
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)

	assert.Equal(t, prompt.Name, restored.Name)
	assert.Equal(t, len(prompt.Arguments), len(restored.Arguments))
}

func TestMCPPrompt_NoArguments(t *testing.T) {
	prompt := MCPPrompt{
		Name:        "simple_prompt",
		Description: "A prompt with no arguments",
		// Arguments intentionally empty
	}

	assert.Empty(t, prompt.Arguments)
}

// ========================================
// MCPPromptArgument Tests
// ========================================

func TestMCPPromptArgument_Structure(t *testing.T) {
	arg := MCPPromptArgument{
		Name:        "query",
		Description: "Search query string",
		Required:    true,
	}

	assert.Equal(t, "query", arg.Name)
	assert.Equal(t, "Search query string", arg.Description)
	assert.True(t, arg.Required)
}

func TestMCPPromptArgument_Optional(t *testing.T) {
	arg := MCPPromptArgument{
		Name:        "limit",
		Description: "Maximum number of results",
		Required:    false,
	}

	assert.False(t, arg.Required)
}

// ========================================
// URL Parsing Logic Tests
// ========================================

func TestURLParsing_BaseURLExtraction(t *testing.T) {
	tests := []struct {
		name          string
		originalURL   string
		expectedBase  string
	}{
		{
			name:         "simple URL without path",
			originalURL:  "http://localhost:5555",
			expectedBase: "http://localhost:5555",
		},
		{
			name:         "URL with path",
			originalURL:  "http://localhost:5555/mcp",
			expectedBase: "http://localhost:5555",
		},
		{
			name:         "URL with deeper path",
			originalURL:  "http://localhost:5555/api/v1/mcp",
			expectedBase: "http://localhost:5555",
		},
		{
			name:         "HTTPS URL",
			originalURL:  "https://mcp.example.com/server",
			expectedBase: "https://mcp.example.com",
		},
		{
			name:         "URL with port and path",
			originalURL:  "http://192.168.1.100:8080/mcp/v2",
			expectedBase: "http://192.168.1.100:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the same logic as DetectCapabilities
			baseURL := tt.originalURL
			if idx := strings.Index(baseURL, "://"); idx != -1 {
				afterProto := baseURL[idx+3:]
				if slashIdx := strings.Index(afterProto, "/"); slashIdx != -1 {
					baseURL = baseURL[:idx+3+slashIdx]
				}
			}

			assert.Equal(t, tt.expectedBase, baseURL)
		})
	}
}

func TestCapabilitiesURLConstruction(t *testing.T) {
	tests := []struct {
		name                   string
		baseURL                string
		expectedCapabilitiesURL string
	}{
		{
			name:                    "with trailing slash",
			baseURL:                 "http://localhost:5555/",
			expectedCapabilitiesURL: "http://localhost:5555/.well-known/mcp/capabilities",
		},
		{
			name:                    "without trailing slash",
			baseURL:                 "http://localhost:5555",
			expectedCapabilitiesURL: "http://localhost:5555/.well-known/mcp/capabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilitiesURL := strings.TrimSuffix(tt.baseURL, "/") + "/.well-known/mcp/capabilities"
			assert.Equal(t, tt.expectedCapabilitiesURL, capabilitiesURL)
		})
	}
}

// ========================================
// Complex Scenario Tests
// ========================================

func TestMCPCapabilitiesResponse_RealWorldExample(t *testing.T) {
	// Simulates a real MCP server response (e.g., filesystem server)
	resp := MCPCapabilitiesResponse{
		Tools: []MCPTool{
			{
				Name:        "read_file",
				Description: "Read the complete contents of a file from the file system.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Absolute or relative path to the file",
						},
					},
					"required": []string{"path"},
				},
			},
			{
				Name:        "write_file",
				Description: "Write content to a file.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Content to write",
						},
					},
					"required": []string{"path", "content"},
				},
			},
			{
				Name:        "list_directory",
				Description: "List contents of a directory.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
				},
			},
		},
		Resources: []MCPResource{
			{
				Name:        "current_directory",
				URI:         "file://.",
				Description: "Access to current working directory",
				MimeTypes:   []string{"text/plain", "application/json"},
			},
		},
		Prompts: []MCPPrompt{}, // Filesystem server typically has no prompts
	}

	// Verify structure
	assert.Len(t, resp.Tools, 3)
	assert.Len(t, resp.Resources, 1)
	assert.Empty(t, resp.Prompts)

	// Verify tool names
	toolNames := make([]string, len(resp.Tools))
	for i, tool := range resp.Tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "read_file")
	assert.Contains(t, toolNames, "write_file")
	assert.Contains(t, toolNames, "list_directory")

	// Test JSON round-trip
	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	var restored MCPCapabilitiesResponse
	err = json.Unmarshal(data, &restored)
	assert.NoError(t, err)
	assert.Equal(t, len(resp.Tools), len(restored.Tools))
}

// ========================================
// String Helper Function Tests
// ========================================

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		substrings []string
		expected   bool
	}{
		{
			name:       "contains first substring",
			input:      "read_file",
			substrings: []string{"read", "write", "delete"},
			expected:   true,
		},
		{
			name:       "contains middle substring",
			input:      "write_data",
			substrings: []string{"read", "write", "delete"},
			expected:   true,
		},
		{
			name:       "contains last substring",
			input:      "delete_record",
			substrings: []string{"read", "write", "delete"},
			expected:   true,
		},
		{
			name:       "contains none",
			input:      "execute_command",
			substrings: []string{"read", "write", "delete"},
			expected:   false,
		},
		{
			name:       "empty substrings slice",
			input:      "some_string",
			substrings: []string{},
			expected:   false,
		},
		{
			name:       "empty input string",
			input:      "",
			substrings: []string{"read", "write"},
			expected:   false,
		},
		{
			name:       "exact match",
			input:      "read",
			substrings: []string{"read"},
			expected:   true,
		},
		{
			name:       "case sensitive - no match",
			input:      "READ_FILE",
			substrings: []string{"read", "write"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.input, tt.substrings)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "substring at start",
			s:        "read_file",
			substr:   "read",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "file_read",
			substr:   "read",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "do_read_now",
			substr:   "read",
			expected: true,
		},
		{
			name:     "exact match",
			s:        "read",
			substr:   "read",
			expected: true,
		},
		{
			name:     "no match",
			s:        "write_file",
			substr:   "read",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "read",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "something",
			substr:   "",
			expected: true,
		},
		{
			name:     "both empty",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "abc",
			substr:   "abcdef",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "found at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "found at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "found in middle",
			s:        "hello world today",
			substr:   "world",
			expected: true,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "xyz",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "single char found",
			s:        "abc",
			substr:   "b",
			expected: true,
		},
		{
			name:     "single char not found",
			s:        "abc",
			substr:   "x",
			expected: false,
		},
		{
			name:     "same string",
			s:        "exact",
			substr:   "exact",
			expected: true,
		},
		{
			name:     "overlapping pattern",
			s:        "aaa",
			substr:   "aa",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// Constructor Tests
// ========================================

func TestNewMCPCapabilityService(t *testing.T) {
	service := NewMCPCapabilityService(nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.mcpRepo)
	assert.Nil(t, service.capabilityRepo)
}
