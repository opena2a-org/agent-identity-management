package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sync"

	"github.com/opena2a/identity/backend/internal/domain"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

// TemplateRenderer renders email templates
type TemplateRenderer struct {
	templates map[domain.EmailTemplate]*emailTemplate
	mu        sync.RWMutex
}

// emailTemplate holds both subject and body templates
type emailTemplate struct {
	subject *template.Template
	body    *template.Template
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(customTemplateDir string) (*TemplateRenderer, error) {
	renderer := &TemplateRenderer{
		templates: make(map[domain.EmailTemplate]*emailTemplate),
	}

	// Load embedded templates by default
	if err := renderer.loadEmbeddedTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load embedded templates: %w", err)
	}

	// Override with custom templates if directory is provided
	if customTemplateDir != "" {
		if err := renderer.loadCustomTemplates(customTemplateDir); err != nil {
			// Log warning but don't fail - embedded templates are sufficient
			fmt.Printf("[WARN] Failed to load custom templates from %s: %v\n", customTemplateDir, err)
		}
	}

	return renderer, nil
}

// loadEmbeddedTemplates loads templates from embedded filesystem
func (r *TemplateRenderer) loadEmbeddedTemplates() error {
	templateNames := []domain.EmailTemplate{
		domain.TemplateWelcome,
		domain.TemplateUserApproved,
		domain.TemplateUserRejected,
		domain.TemplatePasswordReset,
		domain.TemplateAgentRegistered,
		domain.TemplateAgentVerified,
		domain.TemplateVerificationReminder,
		domain.TemplateVerificationFailed,
		domain.TemplateAlertCritical,
		domain.TemplateAlertWarning,
		domain.TemplateAlertInfo,
		domain.TemplateMCPServerRegistered,
		domain.TemplateMCPServerExpiring,
		domain.TemplateAPIKeyCreated,
		domain.TemplateAPIKeyExpiring,
		domain.TemplateAPIKeyRevoked,
	}

	for _, name := range templateNames {
		bodyPath := fmt.Sprintf("templates/%s.html", name)
		subjectPath := fmt.Sprintf("templates/%s.subject.txt", name)

		// Load body template
		bodyContent, err := embeddedTemplates.ReadFile(bodyPath)
		if err != nil {
			// Create placeholder template if not found
			bodyContent = []byte(r.getDefaultTemplate(name))
		}

		bodyTmpl, err := template.New(string(name)).Parse(string(bodyContent))
		if err != nil {
			return fmt.Errorf("failed to parse body template %s: %w", name, err)
		}

		// Load subject template
		subjectContent, err := embeddedTemplates.ReadFile(subjectPath)
		if err != nil {
			// Use default subject if not found
			subjectContent = []byte(r.getDefaultSubject(name))
		}

		subjectTmpl, err := template.New(string(name) + "_subject").Parse(string(subjectContent))
		if err != nil {
			return fmt.Errorf("failed to parse subject template %s: %w", name, err)
		}

		r.templates[name] = &emailTemplate{
			subject: subjectTmpl,
			body:    bodyTmpl,
		}
	}

	return nil
}

// loadCustomTemplates loads templates from filesystem directory
func (r *TemplateRenderer) loadCustomTemplates(dir string) error {
	// This would load templates from a custom directory
	// For MVP, we'll just use embedded templates
	return nil
}

// Render renders a template with the given data
func (r *TemplateRenderer) Render(templateName domain.EmailTemplate, data interface{}) (subject, body string, err error) {
	r.mu.RLock()
	tmpl, ok := r.templates[templateName]
	r.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("template not found: %s", templateName)
	}

	// Render subject
	var subjectBuf bytes.Buffer
	if err := tmpl.subject.Execute(&subjectBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render subject: %w", err)
	}
	subject = subjectBuf.String()

	// Render body
	var bodyBuf bytes.Buffer
	if err := tmpl.body.Execute(&bodyBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render body: %w", err)
	}
	body = bodyBuf.String()

	return subject, body, nil
}

// getDefaultTemplate returns a simple default template if file doesn't exist
func (r *TemplateRenderer) getDefaultTemplate(name domain.EmailTemplate) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #0066cc; color: white; padding: 20px; text-align: center; }
        .content { background: #f9f9f9; padding: 30px; }
        .button { display: inline-block; padding: 12px 24px; background: #0066cc; color: white; text-decoration: none; border-radius: 4px; margin: 20px 0; }
        .footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Agent Identity Management</h1>
        </div>
        <div class="content">
            <p>This is a default template for: %s</p>
            <p>{{.UserName}}</p>
            {{if .DashboardURL}}
            <a href="{{.DashboardURL}}" class="button">Go to Dashboard</a>
            {{end}}
        </div>
        <div class="footer">
            <p>© 2025 OpenA2A</p>
        </div>
    </div>
</body>
</html>`, name, name)
}

// getDefaultSubject returns a simple default subject if file doesn't exist
func (r *TemplateRenderer) getDefaultSubject(name domain.EmailTemplate) string {
	subjects := map[domain.EmailTemplate]string{
		domain.TemplateWelcome:              "Welcome to Agent Identity Management",
		domain.TemplateUserApproved:         "Your account has been approved",
		domain.TemplateUserRejected:         "Account registration update",
		domain.TemplatePasswordReset:        "Reset your password",
		domain.TemplateAgentRegistered:      "Agent registered successfully",
		domain.TemplateAgentVerified:        "Agent verified successfully",
		domain.TemplateVerificationReminder: "Agent verification required",
		domain.TemplateVerificationFailed:   "Agent verification failed",
		domain.TemplateAlertCritical:        "🚨 Critical Alert",
		domain.TemplateAlertWarning:         "⚠️ Warning Alert",
		domain.TemplateAlertInfo:            "ℹ️ Information Alert",
		domain.TemplateMCPServerRegistered:  "MCP Server registered successfully",
		domain.TemplateMCPServerExpiring:    "MCP Server certificate expiring soon",
		domain.TemplateAPIKeyCreated:        "New API key created",
		domain.TemplateAPIKeyExpiring:       "API key expiring soon",
		domain.TemplateAPIKeyRevoked:        "API key revoked",
	}

	if subject, ok := subjects[name]; ok {
		return subject
	}

	return fmt.Sprintf("Notification: %s", name)
}
