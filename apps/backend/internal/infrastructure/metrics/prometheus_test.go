package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMethod(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard GET", "GET", "GET"},
		{"standard POST", "POST", "POST"},
		{"standard PUT", "PUT", "PUT"},
		{"standard DELETE", "DELETE", "DELETE"},
		{"standard PATCH", "PATCH", "PATCH"},
		{"standard HEAD", "HEAD", "HEAD"},
		{"standard OPTIONS", "OPTIONS", "OPTIONS"},
		{"garbled GETT", "GETT", "UNKNOWN"},
		{"garbled POS", "POS", "UNKNOWN"},
		{"garbled POSTT", "POSTT", "UNKNOWN"},
		{"lowercase get", "get", "GET"},
		{"mixed case Get", "Get", "GET"},
		{"trailing whitespace", "GET ", "GET"},
		{"leading whitespace", " GET", "GET"},
		{"trailing newline", "GET\n", "GET"},
		{"trailing tab", "GET\t", "GET"},
		{"empty string", "", "UNKNOWN"},
		{"random string", "FOOBAR", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMethod(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "/api/v1/agents", "/api/v1/agents"},
		{"path with UUID", "/api/v1/agents/550e8400-e29b-41d4-a716-446655440000", "/api/v1/agents/:id"},
		{"path with numeric ID", "/api/v1/agents/12345", "/api/v1/agents/:id"},
		{"root path", "/", "/"},
		{"metrics path", "/metrics", "/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"all digits", "12345", true},
		{"single digit", "0", true},
		{"with letters", "123abc", false},
		{"empty string", "", true}, // edge case: empty has no non-digit chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
