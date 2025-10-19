# 📧 Email Provider System

AIM supports multiple email providers to give enterprises flexibility in how they send emails (verification, notifications, alerts).

## Supported Providers

### ✅ Fully Implemented
1. **SMTP** - Standard SMTP server (supports TLS)
   - Works with Gmail, Outlook, Office 365, private mail servers
   - Most universal option

2. **Console** - Development only
   - Prints emails to console instead of sending
   - Perfect for local development

### 🚧 Community Contributions Welcome
3. **Azure Communication Services** - Microsoft Azure email service
4. **AWS SES** - Amazon Simple Email Service
5. **SendGrid** - SendGrid email API
6. **Resend** - Modern email API

## Configuration

### Environment Variables

```bash
# Email Provider Selection
EMAIL_PROVIDER=smtp  # Options: smtp, azure, aws_ses, sendgrid, resend, console

# From Address (All Providers)
EMAIL_FROM_ADDRESS=noreply@aim.example.com
EMAIL_FROM_NAME=AIM Platform

# SMTP Configuration (if EMAIL_PROVIDER=smtp)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@aim.example.com
SMTP_PASSWORD=your-smtp-password
SMTP_TLS=true

# Azure Communication Services (if EMAIL_PROVIDER=azure)
AZURE_EMAIL_CONNECTION_STRING=endpoint=https://...;accesskey=...

# AWS SES (if EMAIL_PROVIDER=aws_ses)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# SendGrid (if EMAIL_PROVIDER=sendgrid)
SENDGRID_API_KEY=SG.xxxxxxxxxxxxxxxxxxxxx

# Resend (if EMAIL_PROVIDER=resend)
RESEND_API_KEY=re_xxxxxxxxxxxxxxxxxxxxx
```

## Provider-Specific Setup Guides

### SMTP (Gmail Example)

1. **Enable 2FA** on your Gmail account
2. **Generate App Password**:
   - Go to Google Account settings
   - Security → 2-Step Verification → App passwords
   - Generate password for "Mail"
3. **Configure AIM**:
   ```bash
   EMAIL_PROVIDER=smtp
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   SMTP_USER=your-email@gmail.com
   SMTP_PASSWORD=your-app-password  # 16-character password from step 2
   SMTP_TLS=true
   EMAIL_FROM_ADDRESS=your-email@gmail.com
   EMAIL_FROM_NAME=AIM Platform
   ```

### SMTP (Office 365 Example)

```bash
EMAIL_PROVIDER=smtp
SMTP_HOST=smtp.office365.com
SMTP_PORT=587
SMTP_USER=noreply@yourcompany.com
SMTP_PASSWORD=your-password
SMTP_TLS=true
EMAIL_FROM_ADDRESS=noreply@yourcompany.com
EMAIL_FROM_NAME=AIM Platform
```

### SMTP (Private Server Example)

```bash
EMAIL_PROVIDER=smtp
SMTP_HOST=mail.yourcompany.com
SMTP_PORT=587
SMTP_USER=aim@yourcompany.com
SMTP_PASSWORD=your-password
SMTP_TLS=true
EMAIL_FROM_ADDRESS=aim@yourcompany.com
EMAIL_FROM_NAME=AIM Platform
```

### Azure Communication Services

1. **Create Azure Communication Service** in Azure Portal
2. **Get Connection String** from Keys section
3. **Configure AIM**:
   ```bash
   EMAIL_PROVIDER=azure
   AZURE_EMAIL_CONNECTION_STRING=endpoint=https://...;accesskey=...
   EMAIL_FROM_ADDRESS=DoNotReply@your-domain.azurecomm.net
   EMAIL_FROM_NAME=AIM Platform
   ```

**Note**: Azure provider requires implementation (contributions welcome!)

### AWS SES

1. **Verify Domain** in AWS SES Console
2. **Move out of Sandbox** (production use)
3. **Create IAM User** with SES permissions
4. **Configure AIM**:
   ```bash
   EMAIL_PROVIDER=aws_ses
   AWS_REGION=us-east-1
   AWS_ACCESS_KEY_ID=your-access-key
   AWS_SECRET_ACCESS_KEY=your-secret-key
   EMAIL_FROM_ADDRESS=noreply@your-verified-domain.com
   EMAIL_FROM_NAME=AIM Platform
   ```

**Note**: AWS SES provider requires implementation (contributions welcome!)

### SendGrid

1. **Create SendGrid Account**
2. **Generate API Key** (Settings → API Keys)
3. **Verify Sender Identity** (Settings → Sender Authentication)
4. **Configure AIM**:
   ```bash
   EMAIL_PROVIDER=sendgrid
   SENDGRID_API_KEY=SG.xxxxxxxxxxxxxxxxxxxxx
   EMAIL_FROM_ADDRESS=noreply@your-verified-domain.com
   EMAIL_FROM_NAME=AIM Platform
   ```

**Note**: SendGrid provider requires implementation (contributions welcome!)

### Resend

1. **Create Resend Account** at https://resend.com
2. **Add Domain** and verify
3. **Generate API Key**
4. **Configure AIM**:
   ```bash
   EMAIL_PROVIDER=resend
   RESEND_API_KEY=re_xxxxxxxxxxxxxxxxxxxxx
   EMAIL_FROM_ADDRESS=noreply@your-verified-domain.com
   EMAIL_FROM_NAME=AIM Platform
   ```

**Note**: Resend provider requires implementation (contributions welcome!)

### Console (Development Only)

Perfect for local development - prints emails to console instead of sending.

```bash
EMAIL_PROVIDER=console
EMAIL_FROM_ADDRESS=dev@localhost
EMAIL_FROM_NAME=AIM Dev
```

## Usage in Code

```go
import (
    "github.com/opena2a/identity/backend/internal/infrastructure/email"
)

// Initialize provider from config
provider, err := email.NewEmailProvider(emailConfig)
if err != nil {
    // Handle error
}

// Send email
err = provider.SendEmail(ctx, email.EmailParams{
    To:       []string{"user@example.com"},
    From:     "noreply@aim.example.com",
    Subject:  "Verify Your Email",
    TextBody: "Click here to verify: https://...",
    HTMLBody: "<p>Click here to verify: <a href='https://...'>Verify Email</a></p>",
})
```

## Email Types Sent by AIM

1. **Email Verification** - Confirm email ownership during registration
2. **Registration Approval** - Notify user when admin approves their account
3. **Registration Rejection** - Notify user if admin rejects their account
4. **Password Reset** - Send password reset link (future)
5. **Security Alerts** - Notify about suspicious activity (future)
6. **Agent Verification** - Alert when agent verification status changes (future)

## Testing Email Setup

### Test with Console Provider

1. Set `EMAIL_PROVIDER=console`
2. Register a test user
3. Check console output for verification email

### Test with Real Provider

1. Configure your provider (SMTP recommended for testing)
2. Register with a real email address you control
3. Check inbox for verification email
4. Click verification link to confirm

## Troubleshooting

### SMTP Authentication Failed
- **Gmail**: Make sure you're using app password, not account password
- **Office 365**: Check if account has 2FA enabled
- **Private server**: Verify credentials and TLS settings

### Emails Not Received
- Check spam/junk folder
- Verify SPF/DKIM/DMARC records for your domain
- Check provider dashboard for delivery status

### Connection Timeout
- Verify SMTP host and port are correct
- Check firewall rules allow outbound connections
- Try alternative ports (587, 465, 25)

## Contributing a Provider

Want to implement Azure, AWS SES, SendGrid, or Resend? Here's how:

1. **Implement the `EmailProvider` interface**:
   ```go
   type EmailProvider interface {
       SendEmail(ctx context.Context, params EmailParams) error
       ValidateConfig() error
       GetProviderName() string
   }
   ```

2. **Add to `NewEmailProvider` factory** in `provider.go`

3. **Add environment variables** to config

4. **Update this README** with setup instructions

5. **Add tests** in `*_provider_test.go`

6. **Submit PR** to https://github.com/opena2a-org/agent-identity-management

## Security Best Practices

1. **Never hardcode credentials** - Always use environment variables
2. **Use TLS** for SMTP connections in production
3. **Rotate API keys** regularly
4. **Monitor email bounce rates** and delivery metrics
5. **Verify sender domains** to prevent spoofing
6. **Rate limit** email sending to prevent abuse

## Cost Comparison

| Provider | Free Tier | Pricing | Best For |
|----------|-----------|---------|----------|
| SMTP (Gmail) | 500/day | Free | Small teams, testing |
| SMTP (Office 365) | Included | Part of Office 365 | Enterprises with O365 |
| AWS SES | 62,000/month | $0.10/1000 | AWS-based deployments |
| SendGrid | 100/day | $15/month (40k emails) | Startups, medium teams |
| Resend | 3,000/month | $20/month (50k emails) | Modern dev teams |
| Azure | 100/day | $0.10/1000 | Azure-based deployments |

## Support

- **Documentation**: https://docs.aim.opena2a.org
- **Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discussions**: https://github.com/opena2a-org/agent-identity-management/discussions

---

**Last Updated**: October 19, 2025
**Maintainer**: OpenA2A Community
