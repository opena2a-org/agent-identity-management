package email

import (
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewTemplateRenderer Tests
// ===========================

func TestNewTemplateRenderer_Success(t *testing.T) {
	renderer, err := NewTemplateRenderer("")

	require.NoError(t, err)
	assert.NotNil(t, renderer)
	assert.NotEmpty(t, renderer.templates)
}

func TestNewTemplateRenderer_WithNonExistentCustomDir(t *testing.T) {
	// Should succeed with warning but use embedded templates
	renderer, err := NewTemplateRenderer("/nonexistent/path")

	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestNewTemplateRenderer_WithExistingDir(t *testing.T) {
	// Using current directory as a valid path
	renderer, err := NewTemplateRenderer(".")

	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

// ===========================
// Render Tests
// ===========================

func TestTemplateRenderer_Render_Welcome(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":     "Test User",
		"DashboardURL": "https://example.com/dashboard",
	}

	subject, body, err := renderer.Render(domain.TemplateWelcome, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_UserApproved(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":     "Approved User",
		"DashboardURL": "https://example.com/dashboard",
	}

	subject, body, err := renderer.Render(domain.TemplateUserApproved, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_UserRejected(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName": "Rejected User",
	}

	subject, body, err := renderer.Render(domain.TemplateUserRejected, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_PasswordReset(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName": "Reset User",
		"ResetURL": "https://example.com/reset?token=abc123",
	}

	subject, body, err := renderer.Render(domain.TemplatePasswordReset, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_AgentRegistered(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":  "Agent Owner",
		"AgentName": "My Agent",
	}

	subject, body, err := renderer.Render(domain.TemplateAgentRegistered, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_AgentVerified(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":  "Agent Owner",
		"AgentName": "Verified Agent",
	}

	subject, body, err := renderer.Render(domain.TemplateAgentVerified, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_AlertCritical(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":    "Admin",
		"AlertTitle":  "Critical Alert",
		"AlertDetail": "Something is wrong",
	}

	subject, body, err := renderer.Render(domain.TemplateAlertCritical, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, subject, "Critical")
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_AlertWarning(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":   "Admin",
		"AlertTitle": "Warning Alert",
	}

	subject, body, err := renderer.Render(domain.TemplateAlertWarning, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, subject, "Warning")
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_AlertInfo(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":   "Admin",
		"AlertTitle": "Info Alert",
	}

	subject, body, err := renderer.Render(domain.TemplateAlertInfo, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, subject, "Info")
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_MCPServerRegistered(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":   "Server Owner",
		"ServerName": "My MCP Server",
	}

	subject, body, err := renderer.Render(domain.TemplateMCPServerRegistered, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_APIKeyCreated(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName": "Key Owner",
		"KeyName":  "My API Key",
	}

	subject, body, err := renderer.Render(domain.TemplateAPIKeyCreated, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_APIKeyExpiring(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":   "Key Owner",
		"KeyName":    "Expiring Key",
		"ExpiryDate": "2025-01-01",
	}

	subject, body, err := renderer.Render(domain.TemplateAPIKeyExpiring, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_APIKeyRevoked(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName": "Key Owner",
		"KeyName":  "Revoked Key",
	}

	subject, body, err := renderer.Render(domain.TemplateAPIKeyRevoked, data)

	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.NotEmpty(t, body)
}

func TestTemplateRenderer_Render_InvalidTemplate(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	_, _, err = renderer.Render(domain.EmailTemplate("nonexistent"), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

// ===========================
// getDefaultSubject Tests
// ===========================

func TestTemplateRenderer_GetDefaultSubject_KnownTemplates(t *testing.T) {
	renderer := &TemplateRenderer{}

	testCases := []struct {
		template        domain.EmailTemplate
		expectedContain string
	}{
		{domain.TemplateWelcome, "Welcome"},
		{domain.TemplateUserApproved, "approved"},
		{domain.TemplatePasswordReset, "password"},
		{domain.TemplateAlertCritical, "Critical"},
		{domain.TemplateAlertWarning, "Warning"},
		{domain.TemplateAlertInfo, "Info"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.template), func(t *testing.T) {
			subject := renderer.getDefaultSubject(tc.template)
			assert.Contains(t, subject, tc.expectedContain)
		})
	}
}

func TestTemplateRenderer_GetDefaultSubject_UnknownTemplate(t *testing.T) {
	renderer := &TemplateRenderer{}

	subject := renderer.getDefaultSubject(domain.EmailTemplate("unknown"))

	assert.Contains(t, subject, "Notification")
}

// ===========================
// getDefaultTemplate Tests
// ===========================

func TestTemplateRenderer_GetDefaultTemplate_ContainsBasicStructure(t *testing.T) {
	renderer := &TemplateRenderer{}

	template := renderer.getDefaultTemplate(domain.TemplateWelcome)

	assert.Contains(t, template, "<!DOCTYPE html>")
	assert.Contains(t, template, "<html>")
	assert.Contains(t, template, "</html>")
	assert.Contains(t, template, "Agent Identity Management")
	assert.Contains(t, template, "{{.UserName}}")
	assert.Contains(t, template, "{{.DashboardURL}}")
}

// ===========================
// Thread Safety Tests
// ===========================

func TestTemplateRenderer_ConcurrentRender(t *testing.T) {
	renderer, err := NewTemplateRenderer("")
	require.NoError(t, err)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			data := map[string]interface{}{
				"UserName":     "Test User",
				"DashboardURL": "https://example.com",
			}
			_, _, err := renderer.Render(domain.TemplateWelcome, data)
			assert.NoError(t, err)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
