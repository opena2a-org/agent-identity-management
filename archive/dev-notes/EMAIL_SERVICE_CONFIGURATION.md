# 📧 Email Service Configuration Guide

**Status**: Email service is **OPTIONAL** for AIM MVP release.

---

## ⚠️ IMPORTANT: Email is Optional

AIM works perfectly without email configuration. The email service is used for:

- User registration approval notifications
- Password reset emails (if implemented in future)
- Security alert notifications (optional)
- Admin notifications (optional)

**If email is not configured**, AIM will:
- ✅ Still function normally
- ✅ Store all data correctly
- ✅ Allow admins to approve users manually
- ⚠️ Show "unavailable" status in `/api/v1/status` endpoint

---

## 🚀 Quick Start (3 Options)

### Option 1: Console Email (Development - Recommended for MVP)

Prints emails to console logs. Perfect for development and testing.

```bash
# .env
EMAIL_PROVIDER=console
EMAIL_FROM_ADDRESS=info@opena2a.org
EMAIL_FROM_NAME=Agent Identity Management
```

**Pros**:
- ✅ Zero configuration
- ✅ Works immediately
- ✅ See all emails in logs
- ✅ No cost

**Cons**:
- ❌ Emails not actually sent
- ❌ Users won't receive notifications

---

### Option 2: Azure Communication Services (Production)

Enterprise-grade email service with high deliverability.

#### Prerequisites:
1. Azure subscription
2. Azure Communication Services resource created
3. Email domain configured

#### Configuration:
```bash
# .env
EMAIL_PROVIDER=azure
EMAIL_FROM_ADDRESS=DoNotReply@your-domain.azurecomm.net
EMAIL_FROM_NAME=Agent Identity Management
AZURE_EMAIL_CONNECTION_STRING=endpoint=https://your-resource.communication.azure.com/;accesskey=your-access-key
EMAIL_RATE_LIMIT_PER_MINUTE=60
```

#### Setup Steps:

1. **Create Azure Communication Services Resource**:
   ```bash
   # Via Azure Portal
   1. Go to Azure Portal → Create a Resource
   2. Search for "Communication Services"
   3. Create new resource
   4. Note the Connection String from Keys section
   ```

2. **Configure Email Domain**:
   ```bash
   # Via Azure Portal
   1. Go to Communication Services resource
   2. Navigate to "Email" → "Domains"
   3. Add your verified domain
   4. Configure sender addresses
   ```

3. **Test Connection**:
   ```bash
   # Check status endpoint
   curl http://localhost:8080/api/v1/status | jq '.services.email'
   # Should return: "healthy"
   ```

**Pros**:
- ✅ Production-ready
- ✅ High deliverability
- ✅ Azure ecosystem integration
- ✅ Scalable
- ✅ Detailed analytics

**Cons**:
- ❌ Requires Azure subscription
- ❌ Domain verification required
- ❌ Cost (~$0.12 per 1000 emails)

---

### Option 3: SMTP (Gmail, SendGrid, etc.)

Use any SMTP-compatible email service.

#### Configuration:
```bash
# .env
EMAIL_PROVIDER=smtp
EMAIL_FROM_ADDRESS=info@opena2a.org
EMAIL_FROM_NAME=Agent Identity Management
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password  # NOT your regular password!
SMTP_TLS_ENABLED=true
EMAIL_RATE_LIMIT_PER_MINUTE=60
```

#### Gmail Setup:

1. **Enable 2-Factor Authentication** (required)
2. **Create App Password**:
   ```
   1. Go to Google Account → Security
   2. Under "Signing in to Google", select "App Passwords"
   3. Generate password for "Mail"
   4. Use this password in SMTP_PASSWORD
   ```

3. **Test Connection**:
   ```bash
   curl http://localhost:8080/api/v1/status | jq '.services.email'
   # Should return: "healthy"
   ```

**Pros**:
- ✅ Easy to set up
- ✅ Works with any SMTP provider
- ✅ Familiar technology
- ✅ Low cost (Gmail: free tier, SendGrid: free tier)

**Cons**:
- ❌ Gmail rate limits (100-500 emails/day)
- ❌ Less reliable deliverability than Azure
- ❌ App passwords can be security risk if leaked

---

## 📊 Production Status Check

After configuring email, verify it works:

```bash
# Check email service status
curl http://localhost:8080/api/v1/status | jq '.'
```

**Expected Response (Email Configured)**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "cache": "unavailable",     // Redis is optional
    "email": "healthy"           // ✅ Should be "healthy"
  },
  "timestamp": "2025-10-20T..."
}
```

**Expected Response (Email NOT Configured)**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "cache": "unavailable",
    "email": "unavailable"       // ⚠️ This is OK for MVP
  },
  "timestamp": "2025-10-20T..."
}
```

---

## 🎯 Recommendation for MVP Release

**For open source release, we recommend**:

1. **Set EMAIL_PROVIDER=console** in production .env.example
2. **Document all three options** in README.md
3. **Show "email optional" message** in admin panel
4. **Add clear setup instructions** for production deployments

This allows:
- ✅ Users to test AIM immediately without email setup
- ✅ Production deployments to configure Azure/SMTP if needed
- ✅ Enterprise users to use their preferred email provider
- ✅ MVP to work perfectly without email

---

## 🔧 Troubleshooting

### Email shows "unavailable" but it's configured

**Check**:
1. Environment variables are loaded correctly
2. Connection string is valid
3. Backend logs for error messages

```bash
# Check environment variables
docker exec aim-backend env | grep EMAIL

# Check backend logs
docker logs aim-backend | grep -i email
```

### Emails not being sent (Azure)

**Common Issues**:
1. Domain not verified in Azure
2. Sender address not configured
3. Connection string incorrect
4. Rate limits exceeded

**Solution**:
```bash
# Verify Azure configuration
# 1. Check Azure Portal → Communication Services → Email → Domains
# 2. Ensure domain status is "Verified"
# 3. Check sender addresses are configured
# 4. Test with Azure portal "Try It" feature
```

### SMTP authentication fails

**Common Issues**:
1. Using regular password instead of app password (Gmail)
2. 2FA not enabled (Gmail)
3. "Less secure apps" disabled (older providers)
4. Port blocked by firewall

**Solution**:
```bash
# Test SMTP connection
telnet smtp.gmail.com 587
# Should connect successfully

# Generate new app password (Gmail)
# Google Account → Security → App Passwords
```

---

## 📚 Email Template Customization

Email templates are located in:
```
apps/backend/internal/infrastructure/email/templates/
```

To customize:
1. Edit HTML templates in templates directory
2. Restart backend service
3. Test with console provider first

---

## 🚀 Production Recommendations

### For Enterprise Deployment:

1. **Use Azure Communication Services**:
   - Best deliverability
   - Detailed analytics
   - Scalable to millions of emails
   - Azure ecosystem integration

2. **Configure Custom Domain**:
   - Improves deliverability
   - Professional appearance
   - Reduces spam filtering

3. **Enable Rate Limiting**:
   ```bash
   EMAIL_RATE_LIMIT_PER_MINUTE=60
   ```

4. **Monitor Email Delivery**:
   - Check Azure portal analytics
   - Monitor bounce rates
   - Track delivery success

### For Small Deployments:

1. **Use SMTP with Gmail**:
   - Free tier available
   - Easy setup
   - Good for <100 emails/day

2. **Use SendGrid Free Tier**:
   - 100 emails/day free
   - Better deliverability than Gmail
   - Easy SMTP configuration

---

## 💰 Cost Comparison

| Provider | Free Tier | Production Cost | Setup Time |
|----------|-----------|----------------|------------|
| **Console** | ✅ Free | ✅ Free | 1 minute |
| **Gmail SMTP** | ✅ 500/day | $0 | 5 minutes |
| **SendGrid** | ✅ 100/day | $14.95/month (40K emails) | 10 minutes |
| **Azure** | ❌ None | $0.12/1000 emails | 30 minutes |

---

## ✅ Summary

**For MVP Open Source Release**:
- Email is **OPTIONAL**
- Use `EMAIL_PROVIDER=console` for quick testing
- Document all three options in README
- Users can configure Azure/SMTP in production

**For Production Deployment**:
- Use Azure Communication Services (enterprise)
- Use SMTP with SendGrid (small deployments)
- Monitor delivery rates and adjust as needed

---

**Last Updated**: October 20, 2025
**Status**: Email service is OPTIONAL for MVP release
**Next Steps**: Document in README.md and update status endpoint message
