package email

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Per-message tracking suppression
// ===========================

// smtpAPIHeaderLines returns every X-SMTPAPI header line in the header block of
// a message built by buildMessage.
func smtpAPIHeaderLines(t *testing.T, message string) []string {
	t.Helper()

	headerBlock, _, found := strings.Cut(message, "\r\n\r\n")
	require.True(t, found, "message has no header/body separator")

	var lines []string
	for _, line := range strings.Split(headerBlock, "\r\n") {
		name, _, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "X-SMTPAPI") {
			lines = append(lines, line)
		}
	}
	return lines
}

// filterSetting returns filters.<filter>.settings from a decoded X-SMTPAPI value.
func filterSetting(t *testing.T, decoded map[string]interface{}, filter string) map[string]interface{} {
	t.Helper()

	filters, ok := decoded["filters"].(map[string]interface{})
	require.True(t, ok, "X-SMTPAPI has no filters object")
	entry, ok := filters[filter].(map[string]interface{})
	require.True(t, ok, "X-SMTPAPI filters has no %s object", filter)
	settings, ok := entry["settings"].(map[string]interface{})
	require.True(t, ok, "X-SMTPAPI filters.%s has no settings object", filter)
	return settings
}

// assertTrackingSuppressed checks that a built message carries exactly one
// well-formed X-SMTPAPI header that turns off click tracking (HTML and plain
// text), open tracking and Google Analytics link rewriting.
func assertTrackingSuppressed(t *testing.T, message string) {
	t.Helper()

	lines := smtpAPIHeaderLines(t, message)
	require.Len(t, lines, 1, "expected exactly one X-SMTPAPI header")

	line := lines[0]
	// SendGrid limits a header line to 1,000 characters including CRLF.
	assert.Less(t, len(line)+2, 1000, "X-SMTPAPI header line is too long")

	_, value, _ := strings.Cut(line, ":")
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(value)), &decoded), "X-SMTPAPI value is not valid JSON")

	clicktrack := filterSetting(t, decoded, "clicktrack")
	assert.Equal(t, float64(0), clicktrack["enable"], "clicktrack.settings.enable")
	enableText, present := clicktrack["enable_text"]
	require.True(t, present, "clicktrack.settings.enable_text is missing")
	assert.Equal(t, false, enableText, "clicktrack.settings.enable_text")

	opentrack := filterSetting(t, decoded, "opentrack")
	assert.Equal(t, float64(0), opentrack["enable"], "opentrack.settings.enable")

	ganalytics := filterSetting(t, decoded, "ganalytics")
	assert.Equal(t, float64(0), ganalytics["enable"], "ganalytics.settings.enable")
}

func newTrackingTestService(t *testing.T) *SMTPEmailService {
	t.Helper()

	service, err := NewSMTPEmailService(domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
	})
	require.NoError(t, err)
	return service
}

func TestSMTPEmailService_BuildMessage_SuppressesTrackingOnEveryMessage(t *testing.T) {
	service := newTrackingTestService(t)
	from := "Example <noreply@example.com>"
	to := "recipient@example.com"

	templated := []struct {
		name     domain.EmailTemplate
		data     domain.EmailTemplateData
		wantBody string
	}{
		{
			name: domain.TemplateWelcome,
			data: domain.EmailTemplateData{
				UserName:     "Test User",
				DashboardURL: "https://aim.example.com",
			},
			// Checked by name, not by link: a welcome template may carry no link.
			wantBody: "Test User",
		},
		{
			name: domain.TemplateUserApproved,
			data: domain.EmailTemplateData{
				UserName:     "Test User",
				UserEmail:    to,
				DashboardURL: "https://aim.example.com",
				CustomData: map[string]interface{}{
					"LoginURL": "https://aim.example.com/auth/login",
					"Role":     "member",
				},
			},
			wantBody: "/auth/login",
		},
		{
			name: domain.TemplatePasswordReset,
			data: domain.EmailTemplateData{
				UserName:     "Test User",
				DashboardURL: "https://aim.example.com",
				CustomData: map[string]interface{}{
					"ResetLink": "https://aim.example.com/auth/reset-password?token=test-token",
					"ExpiresIn": "24 hours",
				},
			},
			wantBody: "reset-password?token=",
		},
		{
			name: domain.TemplateVerificationReminder,
			data: domain.EmailTemplateData{
				UserName:        "Test User",
				AgentID:         "agent-123",
				AgentName:       "test-agent",
				AgentType:       "ai_agent",
				TrustScore:      0.5,
				VerificationURL: "https://aim.example.com/agents/agent-123",
			},
			wantBody: "https://aim.example.com/agents/agent-123",
		},
	}

	for _, tc := range templated {
		t.Run(string(tc.name), func(t *testing.T) {
			subject, body, err := service.templateRenderer.Render(tc.name, tc.data)
			require.NoError(t, err)
			require.Contains(t, body, tc.wantBody, "rendered template is not the expected mail")

			lowerBody := strings.ToLower(body)
			assert.Equal(t, 0, strings.Count(lowerBody, "<img"), "template carries an image, which can act as an open pixel")
			assert.Equal(t, 0, strings.Count(lowerBody, "url("), "template carries a CSS url(), which can act as an open pixel")

			assertTrackingSuppressed(t, service.buildMessage(from, to, subject, body, true))
		})
	}

	t.Run("raw html body", func(t *testing.T) {
		assertTrackingSuppressed(t, service.buildMessage(from, to, "Subject", "<p><a href=\"https://aim.example.com\">link</a></p>", true))
	})

	t.Run("raw plain text body", func(t *testing.T) {
		assertTrackingSuppressed(t, service.buildMessage(from, to, "Subject", "Visit https://aim.example.com", false))
	})
}

func TestSMTPEmailService_PasswordResetMessage_CarriesTrackingSuppression(t *testing.T) {
	service := newTrackingTestService(t)

	subject, body, err := service.templateRenderer.Render(domain.TemplatePasswordReset, domain.EmailTemplateData{
		UserName:     "Test User",
		DashboardURL: "https://aim.example.com",
		CustomData: map[string]interface{}{
			"ResetLink": "https://aim.example.com/auth/reset-password?token=test-token",
			"ExpiresIn": "24 hours",
		},
	})
	require.NoError(t, err)

	message := service.buildMessage("Example <noreply@example.com>", "recipient@example.com", subject, body, true)

	// Positive control: this is the message that carries the reset credential.
	require.Contains(t, message, "reset-password?token=")
	assertTrackingSuppressed(t, message)
}
