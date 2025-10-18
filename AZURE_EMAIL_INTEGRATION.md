# 📧 Azure Email Integration for AIM

## Overview
Enterprise-grade email integration for Agent Identity Management using Azure Communication Services (ACS) Email with fallback to custom SMTP.

## 🎯 Design Goals

1. **Enterprise-Friendly**: One environment variable config for instant setup
2. **Azure-Native**: Leverage Azure Communication Services by default
3. **SMTP Fallback**: Support any SMTP provider (SendGrid, Mailgun, corporate servers)
4. **Open Source Ready**: No hardcoded Azure dependencies
5. **Cost Effective**: Pay-per-use pricing (~$0.0001 per email)

## 💰 Azure Communication Services Pricing (2025)

### Email Costs
- **Base**: $0.0001 per email recipient
- **Data Transfer**: $0.12 per GB (includes attachments, images)
- **Free Tier**: First 500 emails/month FREE
- **No Limits**: Tested up to 2M emails/hour

### Cost Calculation for 100 Agents
Assuming each agent triggers:
- 1 welcome email on registration
- 1 verification email every 24 hours
- 2 alert emails per month

**Monthly Breakdown**:
- Registration: 100 emails × $0.0001 = $0.01
- Verification: 100 agents × 30 days × $0.0001 = $0.30
- Alerts: 100 agents × 2 alerts × $0.0001 = $0.02
- **Total**: ~$0.33/month (covered by free tier!)

## 🏗️ Architecture

### 1. Email Service Interface (Domain Layer)
```go
// internal/domain/email.go
package domain

type EmailService interface {
    SendEmail(to, subject, body string, isHTML bool) error
    SendTemplatedEmail(template EmailTemplate, data interface{}) error
    SendBulkEmail(recipients []string, subject, body string) error
}

type EmailTemplate string

const (
    TemplateWelcome           EmailTemplate = "welcome"
    TemplateVerification      EmailTemplate = "verification"
    TemplatePasswordReset     EmailTemplate = "password_reset"
    TemplateAgentRegistered   EmailTemplate = "agent_registered"
    TemplateAlertNotification EmailTemplate = "alert_notification"
    TemplateUserApproved      EmailTemplate = "user_approved"
    TemplateUserRejected      EmailTemplate = "user_rejected"
)
```

### 2. Azure Communication Services Implementation
```go
// internal/infrastructure/email/azure_email_service.go
package email

import (
    "context"
    "github.com/Azure/azure-sdk-for-go/sdk/communication/azcommunication"
)

type AzureEmailService struct {
    connectionString string
    fromAddress     string
    client          *azcommunication.EmailClient
}

func NewAzureEmailService(connectionString, fromAddress string) (*AzureEmailService, error) {
    // Initialize Azure Communication Services client
}
```

### 3. SMTP Fallback Implementation
```go
// internal/infrastructure/email/smtp_email_service.go
package email

import "net/smtp"

type SMTPEmailService struct {
    host       string
    port       int
    username   string
    password   string
    fromAddress string
    tlsEnabled bool
}

func NewSMTPEmailService(config SMTPConfig) (*SMTPEmailService, error) {
    // Standard SMTP implementation
}
```

### 4. Email Provider Factory
```go
// internal/infrastructure/email/factory.go
package email

func NewEmailService(config EmailConfig) (domain.EmailService, error) {
    switch config.Provider {
    case "azure":
        return NewAzureEmailService(config.Azure.ConnectionString, config.FromAddress)
    case "smtp":
        return NewSMTPEmailService(config.SMTP)
    default:
        return nil, fmt.Errorf("unsupported email provider: %s", config.Provider)
    }
}
```

## 🔧 Configuration

### Environment Variables (Enterprise-Friendly)

```bash
# Email Provider (azure or smtp)
EMAIL_PROVIDER=azure

# Azure Communication Services (if EMAIL_PROVIDER=azure)
AZURE_EMAIL_CONNECTION_STRING=endpoint=https://xxx.communication.azure.com/;accesskey=xxx
EMAIL_FROM_ADDRESS=noreply@yourdomain.com

# SMTP Configuration (if EMAIL_PROVIDER=smtp)
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USERNAME=apikey
SMTP_PASSWORD=SG.xxxx
SMTP_FROM_ADDRESS=noreply@yourdomain.com
SMTP_TLS_ENABLED=true

# Optional: Email Templates Directory
EMAIL_TEMPLATES_DIR=./templates/email
```

### Docker Compose Update
```yaml
services:
  backend:
    environment:
      - EMAIL_PROVIDER=${EMAIL_PROVIDER:-azure}
      - AZURE_EMAIL_CONNECTION_STRING=${AZURE_EMAIL_CONNECTION_STRING}
      - EMAIL_FROM_ADDRESS=${EMAIL_FROM_ADDRESS:-noreply@aim.opena2a.org}
      - SMTP_HOST=${SMTP_HOST}
      - SMTP_PORT=${SMTP_PORT:-587}
      - SMTP_USERNAME=${SMTP_USERNAME}
      - SMTP_PASSWORD=${SMTP_PASSWORD}
      - SMTP_TLS_ENABLED=${SMTP_TLS_ENABLED:-true}
```

### Kubernetes Secret
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aim-email-config
type: Opaque
stringData:
  EMAIL_PROVIDER: azure
  AZURE_EMAIL_CONNECTION_STRING: endpoint=https://...
  EMAIL_FROM_ADDRESS: noreply@aim.opena2a.org
```

## 📧 Email Templates

### Welcome Email
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Welcome to AIM</title>
</head>
<body>
    <h1>Welcome to Agent Identity Management</h1>
    <p>Hi {{.UserName}},</p>
    <p>Your account has been approved! You can now start registering agents.</p>
    <p><a href="{{.DashboardURL}}">Go to Dashboard</a></p>
</body>
</html>
```

### Verification Email
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Agent Verification Required</title>
</head>
<body>
    <h1>Agent Verification Required</h1>
    <p>Agent <strong>{{.AgentName}}</strong> needs re-verification.</p>
    <p>Trust Score: {{.TrustScore}}/10</p>
    <p><a href="{{.VerifyURL}}">Verify Now</a></p>
</body>
</html>
```

## 🚀 Usage in Application

### User Registration Approval
```go
// internal/application/auth_service.go
func (s *AuthService) ApproveUser(ctx context.Context, userID uuid.UUID) error {
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return err
    }

    // Update user status
    user.Status = domain.UserStatusActive
    user.ApprovedAt = time.Now()
    if err := s.userRepo.Update(user); err != nil {
        return err
    }

    // Send welcome email
    templateData := struct {
        UserName     string
        DashboardURL string
    }{
        UserName:     user.Name,
        DashboardURL: s.config.BaseURL + "/dashboard",
    }

    return s.emailService.SendTemplatedEmail(
        domain.TemplateWelcome,
        user.Email,
        templateData,
    )
}
```

### Agent Verification Alerts
```go
// internal/application/agent_service.go
func (s *AgentService) SendVerificationReminder(ctx context.Context, agentID uuid.UUID) error {
    agent, err := s.agentRepo.GetByID(agentID)
    if err != nil {
        return err
    }

    user, err := s.userRepo.GetByID(agent.CreatedBy)
    if err != nil {
        return err
    }

    templateData := struct {
        AgentName   string
        TrustScore  float64
        VerifyURL   string
    }{
        AgentName:  agent.Name,
        TrustScore: agent.TrustScore,
        VerifyURL:  fmt.Sprintf("%s/agents/%s/verify", s.config.BaseURL, agent.ID),
    }

    return s.emailService.SendTemplatedEmail(
        domain.TemplateVerification,
        user.Email,
        templateData,
    )
}
```

## 🔒 Security Considerations

1. **Connection String Protection**: Never log or expose in responses
2. **Rate Limiting**: Implement per-user email rate limits
3. **Content Validation**: Sanitize all email content
4. **SPF/DKIM/DMARC**: Configure for Azure domain
5. **Unsubscribe Links**: Required for compliance

## 📊 Monitoring & Alerts

### Metrics to Track
- `email_sent_total` (counter by template, status)
- `email_send_duration_seconds` (histogram)
- `email_failures_total` (counter by error type)

### Dashboard Queries
```promql
# Email success rate
rate(email_sent_total{status="success"}[5m])
/
rate(email_sent_total[5m])

# Failed emails by template
sum(rate(email_sent_total{status="failure"}[5m])) by (template)
```

## 🧪 Testing Strategy

### Unit Tests
```go
func TestAzureEmailService_SendEmail(t *testing.T) {
    mockClient := &mockAzureClient{}
    service := &AzureEmailService{client: mockClient}

    err := service.SendEmail("test@example.com", "Test", "Body", false)
    assert.NoError(t, err)
    assert.Equal(t, 1, mockClient.SendCallCount)
}
```

### Integration Tests
```go
func TestEmailIntegration_WelcomeFlow(t *testing.T) {
    // Use real Azure connection or mock SMTP server
    emailService := setupEmailService(t)

    err := emailService.SendTemplatedEmail(
        domain.TemplateWelcome,
        testEmail,
        testData,
    )

    assert.NoError(t, err)
}
```

## 📈 Migration Plan

### Phase 1: Email Service Abstraction (Week 1)
- [ ] Create `internal/domain/email.go` interface
- [ ] Implement Azure Communication Services provider
- [ ] Implement SMTP fallback provider
- [ ] Add email service factory
- [ ] Write unit tests

### Phase 2: Template System (Week 1)
- [ ] Create HTML email templates
- [ ] Implement template rendering with Go templates
- [ ] Add template validation
- [ ] Test all templates

### Phase 3: Integration (Week 2)
- [ ] Update `auth_service.go` for user approval emails
- [ ] Update `agent_service.go` for verification emails
- [ ] Add alert notification emails
- [ ] Add password reset emails

### Phase 4: Infrastructure (Week 2)
- [ ] Update Docker Compose configuration
- [ ] Update Kubernetes manifests
- [ ] Create Terraform/Bicep for Azure resources
- [ ] Update documentation

### Phase 5: Testing & Deployment (Week 3)
- [ ] Integration tests with real Azure account
- [ ] Load testing (100 concurrent emails)
- [ ] Deploy to Azure demo environment
- [ ] Monitor email delivery rates

## 🎯 Success Criteria

- ✅ Support both Azure ACS and custom SMTP
- ✅ One-line environment variable setup
- ✅ <500ms email send latency (p95)
- ✅ >99.5% delivery rate
- ✅ HTML templates for all notification types
- ✅ Comprehensive test coverage (>90%)
- ✅ Production-ready monitoring

## 📚 Resources

- [Azure Communication Services Email Docs](https://learn.microsoft.com/en-us/azure/communication-services/concepts/email/email-overview)
- [Azure Email Go SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/communication/azcommunication)
- [SMTP Go Package](https://pkg.go.dev/net/smtp)
