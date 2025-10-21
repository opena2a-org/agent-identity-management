# 📧 Azure Communication Services Email Setup Guide

This guide walks you through setting up Azure Communication Services for email in the Agent Identity Management (AIM) platform.

## 🎯 Overview

Azure Communication Services Email allows you to send enterprise-grade transactional emails (verification, notifications, alerts) at scale with enterprise security and compliance.

### Benefits
- ✅ Enterprise-grade reliability (99.9% SLA)
- ✅ GDPR, SOC 2, HIPAA compliant
- ✅ Pay-as-you-go pricing ($0.025 per 1,000 emails)
- ✅ No third-party dependencies (direct REST API integration)
- ✅ Built-in metrics and monitoring
- ✅ Custom domain support

## 📋 Prerequisites

- Azure subscription (free tier available)
- Azure CLI or access to Azure Portal
- Custom domain (optional, can use Azure-provided domain for testing)
- Verified sender email address

## 🚀 Step-by-Step Setup

### Step 1: Create Email Communication Service Resource

1. **Login to Azure Portal**
   ```bash
   https://portal.azure.com
   ```

2. **Create Email Communication Service**
   - Click "+ Create a resource"
   - Search for "Email Communication Service"
   - Click "Create"

   **Configuration:**
   - **Subscription**: Select your Azure subscription
   - **Resource Group**: Create new or select existing (e.g., `aim-production-rg`)
   - **Email Service Name**: Choose a unique name (e.g., `aim-email-service`)
   - **Region**: Select closest to your users (e.g., `East US`)
   - **Data Location**: Choose based on compliance requirements

   - Click "Review + Create"
   - Click "Create"
   - Wait for deployment (takes 2-3 minutes)

3. **Add Email Domain**

   After deployment completes:
   - Go to your Email Communication Service resource
   - Click "Provision Domains" in left menu
   - Choose one of two options:

   **Option A: Azure Managed Domain (Free, Quick Setup)**
   - Click "+ Add Domain"
   - Select "Azure Managed Domain"
   - Enter subdomain name (e.g., `aim-notifications`)
   - Full domain will be: `aim-notifications.azurecomm.net`
   - Click "Add"
   - Domain is instantly verified and ready

   **Option B: Custom Domain (Professional)**
   - Click "+ Add Domain"
   - Select "Custom Domain"
   - Enter your domain (e.g., `notifications.yourcompany.com`)
   - Add DNS records shown (TXT, MX, CNAME)
   - Click "Verify" after DNS propagation (can take 24-48 hours)

4. **Configure Sender Authentication**
   - In your Email Communication Service
   - Click "MailFrom addresses" under Provision
   - Select your domain
   - Add sender address (e.g., `noreply@aim-notifications.azurecomm.net`)
   - Click "Save"

### Step 2: Create Communication Service Resource

Azure requires a separate Communication Services resource to actually send emails (the Email Communication Service only manages domains).

1. **Create Communication Service**
   - Click "+ Create a resource"
   - Search for "Communication Service"
   - Click "Create"

   **Configuration:**
   - **Subscription**: Same as Email service
   - **Resource Group**: Same as Email service
   - **Communication Service Name**: Choose unique name (e.g., `aim-communication-service`)
   - **Data Location**: Match Email service data location

   - Click "Review + Create"
   - Click "Create"
   - Wait for deployment

2. **Link Email Domain to Communication Service**
   - Go to your **Communication Service** resource (NOT Email service)
   - Click "Domains" in left menu
   - Click "+ Connect Domain"
   - **Email Service Subscription**: Select your subscription
   - **Email Service**: Select your Email Communication Service
   - **Domain**: Select the domain you created earlier
   - Click "Connect"
   - Wait for connection (takes 1-2 minutes)

### Step 3: Get Connection String

1. **Navigate to Communication Service**
   - Go to your **Communication Service** resource
   - Click "Keys" in left menu under Settings

2. **Copy Connection String**
   - You'll see:
     - Endpoint URL (e.g., `https://aim-communication-service.communication.azure.com/`)
     - Primary access key
     - **Connection string** (this is what AIM needs)

   - Copy the entire "Connection string" (looks like):
     ```
     endpoint=https://aim-communication-service.communication.azure.com/;accesskey=ABC123xyz...==
     ```

### Step 4: Configure AIM

1. **Update `.env` file**

   ```bash
   # Email Provider Configuration
   EMAIL_PROVIDER=azure

   # Azure Communication Services
   AZURE_EMAIL_CONNECTION_STRING=endpoint=https://aim-communication-service.communication.azure.com/;accesskey=YOUR_ACCESS_KEY_HERE

   # Sender Configuration
   EMAIL_FROM_ADDRESS=noreply@aim-notifications.azurecomm.net
   EMAIL_FROM_NAME=Agent Identity Management

   # Optional: Email Templates
   EMAIL_TEMPLATES_DIR=apps/backend/internal/infrastructure/email/templates
   EMAIL_RATE_LIMIT_PER_MINUTE=60

   # Optional: Polling (for delivery status tracking)
   AZURE_EMAIL_POLLING_ENABLED=false
   ```

2. **Restart AIM Backend**
   ```bash
   # If using Docker Compose
   docker compose restart backend

   # If running directly
   cd apps/backend
   go run cmd/server/main.go
   ```

3. **Verify Configuration**

   Check backend logs for successful email service initialization:
   ```
   INFO: Email service initialized successfully (provider: azure)
   INFO: Email from address: noreply@aim-notifications.azurecomm.net
   ```

## 🧪 Testing Email Configuration

### Test 1: Manual Test via API

```bash
# Register a new user (triggers verification email)
curl -X POST http://localhost:8080/api/v1/public/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "your-real-email@example.com",
    "name": "Test User",
    "password": "SecurePassword123!"
  }'
```

You should receive a verification email within 1-2 minutes.

### Test 2: Check Azure Portal

1. Go to your Communication Service resource
2. Click "Metrics" in left menu
3. Add metric: "Email Messages Sent"
4. You should see email count increase

### Test 3: Check Backend Logs

```bash
# View Docker logs
docker compose logs backend | grep -i email

# Expected output:
# [INFO] Sending email to your-real-email@example.com
# [INFO] Email sent successfully via Azure Communication Services
```

## 📊 Monitoring & Metrics

### Azure Portal Metrics

Available metrics in Azure Communication Services:
- **Email Messages Sent**: Total emails queued
- **Email Messages Delivered**: Successfully delivered
- **Email Messages Failed**: Delivery failures
- **Email Messages Bounced**: Bounce rate

### AIM Internal Metrics

Access via AIM API:
```bash
GET /api/v1/admin/metrics/email
Authorization: Bearer <admin-token>
```

Response:
```json
{
  "totalSent": 1247,
  "totalFailed": 3,
  "successRate": 99.76,
  "lastSentAt": "2025-10-19T14:30:00Z",
  "sentByTemplate": {
    "verification": 523,
    "approval": 412,
    "rejection": 8
  },
  "failuresByType": {
    "invalid_recipient": 2,
    "send_error": 1
  }
}
```

## 💰 Pricing

Azure Communication Services Email pricing (as of October 2025):

| Volume | Price per 1,000 emails |
|--------|------------------------|
| First 100/month | **FREE** |
| 100 - 100,000/month | $0.025 |
| 100,000 - 1M/month | $0.020 |
| 1M+/month | Contact Azure sales |

**Example Costs:**
- 1,000 emails/month: **$0.02**
- 10,000 emails/month: **$0.25**
- 50,000 emails/month: **$1.25**
- 100,000 emails/month: **$2.50**

**Included Free:**
- Domain management
- Sender authentication
- Delivery tracking
- Bounce handling
- Metrics and monitoring

## 🔒 Security Best Practices

### 1. Protect Access Keys
```bash
# NEVER commit connection strings to git
echo ".env" >> .gitignore

# Use Azure Key Vault in production
az keyvault secret set \
  --vault-name aim-vault \
  --name email-connection-string \
  --value "endpoint=https://...;accesskey=..."

# Reference in app
AZURE_EMAIL_CONNECTION_STRING=@Microsoft.KeyVault(SecretUri=https://aim-vault.vault.azure.net/secrets/email-connection-string/)
```

### 2. Rotate Keys Regularly
```bash
# In Azure Portal:
# Communication Service → Keys → Regenerate key
# Copy new connection string
# Update AIM .env
# Restart services
```

### 3. Use Azure Managed Identities (Production)
```bash
# Instead of access keys, use managed identity
az communication identity create \
  --resource-group aim-production-rg \
  --communication-service aim-communication-service
```

### 4. Enable Azure Monitor Alerts
Set up alerts for:
- High failure rate (>5%)
- Unusual sending volume
- Access key usage from unexpected locations

## 🐛 Troubleshooting

### Issue: "Connection string is invalid"
**Solution**: Verify connection string format:
```
endpoint=https://...;accesskey=...
```
- Must have both `endpoint` and `accesskey`
- No spaces around `=` or `;`
- Endpoint must use HTTPS

### Issue: "Sender address not verified"
**Solution**:
1. Go to Email Communication Service
2. Check "MailFrom addresses"
3. Ensure your sender email is added and verified
4. Wait 5 minutes after adding

### Issue: "Domain not linked"
**Solution**:
1. Go to **Communication Service** (not Email service)
2. Click "Domains"
3. Verify your email domain is connected
4. If not, click "+ Connect Domain"

### Issue: Emails not being delivered
**Checks**:
1. **Azure Portal**: Check metrics for delivery status
2. **Spam folder**: Check recipient spam/junk folder
3. **Email logs**: `docker compose logs backend | grep email`
4. **Domain DNS**: Verify SPF, DKIM, DMARC records

### Issue: "Rate limit exceeded"
**Solution**:
```bash
# Default limit: 60 emails/minute
# Increase in .env:
EMAIL_RATE_LIMIT_PER_MINUTE=120

# Or contact Azure to increase quota
```

## 📚 Additional Resources

### Official Documentation
- [Azure Communication Services Email Overview](https://learn.microsoft.com/en-us/azure/communication-services/concepts/email/email-overview)
- [Send Email Quickstart](https://learn.microsoft.com/en-us/azure/communication-services/quickstarts/email/send-email)
- [REST API Reference](https://learn.microsoft.com/en-us/rest/api/communication/email/)

### AIM-Specific Docs
- [Email Provider System README](apps/backend/internal/infrastructure/email/README.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)
- [Environment Variables Guide](.env.example)

### Support
- **Azure Support**: https://portal.azure.com → Support + troubleshooting
- **AIM GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **AIM Discussions**: https://github.com/opena2a-org/agent-identity-management/discussions

## ✅ Production Deployment Checklist

Before deploying to production:

- [ ] Custom domain configured (not Azure-provided domain)
- [ ] SPF, DKIM, DMARC DNS records added
- [ ] Sender address verified
- [ ] Connection string stored in Azure Key Vault (not .env)
- [ ] Managed identity enabled (no access keys)
- [ ] Azure Monitor alerts configured
- [ ] Rate limits configured appropriately
- [ ] Email templates tested
- [ ] Bounce handling configured
- [ ] Unsubscribe links added to marketing emails (if applicable)
- [ ] Privacy policy link added to emails
- [ ] GDPR compliance verified
- [ ] Backup email provider configured (optional)

## 🎓 Next Steps

1. **Test Email Flow**: Send a test registration email
2. **Monitor Metrics**: Check Azure portal for delivery stats
3. **Customize Templates**: Edit email templates in `apps/backend/internal/infrastructure/email/templates/`
4. **Configure Alerts**: Set up Azure Monitor alerts for failures
5. **Plan Scaling**: Review pricing and adjust quotas if needed

---

**Last Updated**: October 19, 2025
**AIM Version**: v1.0.0
**Azure API Version**: 2023-03-31
