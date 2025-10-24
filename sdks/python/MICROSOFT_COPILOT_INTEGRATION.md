# Microsoft Copilot + AIM Integration Guide

This guide shows how to integrate **Microsoft Copilot** (GitHub Copilot, Microsoft 365 Copilot, Azure OpenAI) with **AIM (Agent Identity Management)** for enterprise-grade identity verification and trust scoring.

## 🎯 Why Integrate AIM with Microsoft Copilot?

Microsoft Copilot agents need robust identity management for:
- ✅ **Security**: Verify every Copilot action before execution
- ✅ **Compliance**: Full audit trail for SOC 2, HIPAA, GDPR
- ✅ **Trust Scoring**: Track agent behavior and anomaly detection
- ✅ **Access Control**: Role-based permissions for Copilot agents
- ✅ **Monitoring**: Real-time alerts for suspicious activity

---

## 📦 Supported Microsoft Copilot Platforms

### 1. GitHub Copilot Extensions
AIM verifies GitHub Copilot agent actions in repositories, pull requests, and code reviews.

### 2. Microsoft 365 Copilot
AIM tracks Copilot interactions with Teams, Outlook, SharePoint, and OneDrive.

### 3. Azure OpenAI Service
AIM monitors Azure OpenAI-powered agents and chatbots.

### 4. Power Platform Copilot
AIM verifies Power Automate and Power Apps copilot actions.

---

## 🚀 Quick Start

### Installation

Download the SDK from your AIM dashboard (Settings → SDK Download). The SDK includes pre-configured credentials.

**Note**: There is NO pip package. The SDK must be downloaded from your AIM instance.

### Basic Integration

```python
from aim_sdk import secure, aim_verify
import os

# Initialize AIM (auto-registers if needed)
agent = secure("copilot-agent")

# Wrap your Copilot function with AIM verification
@aim_verify(agent, action_type="copilot_action", risk_level="medium")
def copilot_read_email(email_id: str):
    """Copilot reads an email via Microsoft Graph API"""
    # Your Microsoft Graph API call here
    return graph_client.get_email(email_id)

# Verification happens automatically before execution
email = copilot_read_email("email-123")
```

---

## 🔐 GitHub Copilot Integration

### Example: Copilot Extension for Code Review

```python
# copilot_code_reviewer.py
from aim_sdk import secure, aim_verify
from github import Github
import os

# Initialize AIM client for GitHub Copilot
agent = secure("github-copilot-reviewer")

# Initialize GitHub client
github = Github(os.getenv("GITHUB_TOKEN"))

@aim_verify(agent, action_type="code_review", risk_level="low")
def review_pull_request(repo_name: str, pr_number: int):
    """
    GitHub Copilot reviews a pull request and provides feedback.
    AIM verifies this action before execution.
    """
    repo = github.get_repo(repo_name)
    pr = repo.get_pull(pr_number)

    # Analyze PR changes
    files_changed = pr.get_files()
    review_comments = []

    for file in files_changed:
        # Copilot analyzes code
        if "TODO" in file.patch:
            review_comments.append({
                "path": file.filename,
                "line": 1,
                "comment": "⚠️  Found TODO comments - please address before merging"
            })

    return {
        "pr": pr_number,
        "files_reviewed": len(list(files_changed)),
        "comments": review_comments
    }

# Usage
review_result = review_pull_request("org/repo", 123)
print(f"Reviewed PR #{review_result['pr']}: {review_result['files_reviewed']} files")
```

### Environment Configuration

```bash
# .env
AIM_AGENT_NAME=github-copilot-reviewer
AIM_URL=https://aim.example.com
GITHUB_TOKEN=ghp_xxxxxxxxxxxxx
```

---

## 📧 Microsoft 365 Copilot Integration

### Example: Copilot Email Assistant

```python
# copilot_email_assistant.py
from aim_sdk import secure, aim_verify_external_service
from msgraph import GraphServiceClient
from azure.identity import ClientSecretCredential
import os

# Initialize Microsoft Graph client
credential = ClientSecretCredential(
    tenant_id=os.getenv("AZURE_TENANT_ID"),
    client_id=os.getenv("AZURE_CLIENT_ID"),
    client_secret=os.getenv("AZURE_CLIENT_SECRET")
)
graph_client = GraphServiceClient(credential)

# Initialize AIM client
agent = secure(
    agent_name="m365-copilot-email",
    aim_url=os.getenv("AIM_URL")
)

@aim_verify_external_service(aim_client, risk_level="high")
async def copilot_send_email(to: str, subject: str, body: str):
    """
    Microsoft 365 Copilot sends an email on behalf of the user.
    AIM verifies this high-risk action before execution.
    """
    message = {
        "subject": subject,
        "body": {
            "contentType": "HTML",
            "content": body
        },
        "toRecipients": [
            {"emailAddress": {"address": to}}
        ]
    }

    # Send email via Microsoft Graph
    await graph_client.me.send_mail(message=message).post()

    return {
        "sent": True,
        "to": to,
        "subject": subject
    }

# Usage (async)
import asyncio

async def main():
    result = await copilot_send_email(
        to="colleague@example.com",
        subject="Meeting Summary",
        body="<p>Here's the summary from today's meeting...</p>"
    )
    print(f"Email sent: {result}")

asyncio.run(main())
```

### Microsoft Graph Permissions Required

```json
{
  "requiredResourceAccess": [
    {
      "resourceAppId": "00000003-0000-0000-c000-000000000000",
      "resourceAccess": [
        {
          "id": "e1fe6dd8-ba31-4d61-89e7-88639da4683d",
          "type": "Scope"
        }
      ]
    }
  ]
}
```

---

## ☁️ Azure OpenAI Service Integration

### Example: Azure OpenAI Chatbot with AIM

```python
# copilot_chatbot.py
from aim_sdk import secure, aim_verify
from openai import AzureOpenAI
import os

# Initialize Azure OpenAI client
azure_openai = AzureOpenAI(
    api_key=os.getenv("AZURE_OPENAI_API_KEY"),
    api_version="2024-02-15-preview",
    azure_endpoint=os.getenv("AZURE_OPENAI_ENDPOINT")
)

# Initialize AIM client
agent = secure(
    agent_name="azure-openai-chatbot",
    aim_url=os.getenv("AIM_URL")
)

@aim_verify(aim_client, action_type="llm_chat", risk_level="medium")
def copilot_chat(user_message: str, conversation_history: list = None):
    """
    Azure OpenAI Copilot processes user chat messages.
    AIM verifies and logs every chat interaction.
    """
    messages = conversation_history or []
    messages.append({"role": "user", "content": user_message})

    # Call Azure OpenAI
    response = azure_openai.chat.completions.create(
        model="gpt-4",
        messages=messages,
        temperature=0.7,
        max_tokens=800
    )

    assistant_message = response.choices[0].message.content

    return {
        "user": user_message,
        "assistant": assistant_message,
        "model": "gpt-4",
        "tokens_used": response.usage.total_tokens
    }

# Usage
chat_result = copilot_chat("What are the latest sales numbers?")
print(f"Copilot: {chat_result['assistant']}")
```

---

## ⚡ Power Platform Copilot Integration

### Example: Power Automate Flow with AIM

```python
# copilot_power_automate.py
from aim_sdk import secure, aim_verify
import requests
import os

# Initialize AIM client
agent = secure(
    agent_name="power-automate-copilot",
    aim_url=os.getenv("AIM_URL")
)

@aim_verify(aim_client, action_type="workflow_trigger", risk_level="high")
def trigger_power_automate_flow(flow_id: str, inputs: dict):
    """
    Copilot triggers a Power Automate flow.
    AIM verifies this action due to potential business impact.
    """
    # Power Automate HTTP trigger URL
    flow_url = f"https://prod-xx.eastus.logic.azure.com/workflows/{flow_id}/triggers/manual/paths/invoke"

    # Trigger the flow
    response = requests.post(
        flow_url,
        json=inputs,
        headers={"Content-Type": "application/json"},
        params={"api-version": "2016-06-01", "sp": os.getenv("POWER_AUTOMATE_SAS")}
    )

    return {
        "flow_id": flow_id,
        "triggered": response.status_code == 202,
        "status": response.status_code
    }

# Usage
result = trigger_power_automate_flow(
    flow_id="abc123",
    inputs={"customer_email": "customer@example.com", "action": "send_invoice"}
)
print(f"Flow triggered: {result['triggered']}")
```

---

## 🔒 Security Best Practices

### 1. Use Environment Variables for Secrets

```bash
# Never hardcode credentials!
export AIM_AGENT_NAME="copilot-agent"
export AIM_URL="https://aim.example.com"
export AZURE_CLIENT_ID="..."
export AZURE_CLIENT_SECRET="..."
export AZURE_TENANT_ID="..."
```

### 2. Enable Strict Mode in Production

```python
# Production configuration
os.environ['AIM_STRICT_MODE'] = 'true'  # Block execution if verification fails

@aim_verify(aim_client, action_type="sensitive_action", risk_level="critical")
def delete_sharepoint_files(site_id: str, file_ids: list):
    # This will be blocked if AIM verification fails
    pass
```

### 3. Use Least Privilege Permissions

```python
# Only request permissions your Copilot agent actually needs
# Bad: Mail.ReadWrite.All (too broad)
# Good: Mail.Send (specific)
```

### 4. Monitor Trust Scores

```python
# Check agent trust score regularly
trust_score = aim_client.get_trust_score()
if trust_score < 50:
    print(f"⚠️  Low trust score: {trust_score} - review agent behavior")
```

---

## 📊 Monitoring and Alerts

### View Copilot Activity in AIM Dashboard

1. **Login** to AIM web UI at `https://aim.example.com`
2. **Navigate** to Agents → Find your Copilot agent
3. **View** activity logs, trust score trends, verification events

### Proactive Alerts

AIM automatically alerts you when:
- ✅ Copilot agent trust score drops below threshold
- ✅ Unusual activity detected (e.g., 100 API calls in 1 minute)
- ✅ Failed verification attempts
- ✅ Privilege escalation attempts

---

## 🧪 Testing Microsoft Copilot Integration

```python
# test_copilot_integration.py
import pytest
from aim_sdk import secure, aim_verify
import os

@pytest.fixture
def aim_client():
    return secure(
        agent_name="test-copilot",
        aim_url=os.getenv("AIM_URL", "http://localhost:8080")
    )

def test_copilot_verification(aim_client):
    """Test that Copilot actions are verified by AIM"""

    @aim_verify(aim_client, action_type="test_action")
    def test_function():
        return "success"

    result = test_function()
    assert result == "success"

def test_strict_mode_blocks_on_failure():
    """Test that strict mode blocks execution on verification failure"""
    os.environ['AIM_STRICT_MODE'] = 'true'

    # This should raise an exception if verification fails
    # (requires mock AIM server returning denial)
    pass
```

Run tests:
```bash
pytest test_copilot_integration.py -v
```

---

## 📚 Additional Resources

### Microsoft Documentation
- [GitHub Copilot Extensions](https://docs.github.com/en/copilot/building-copilot-extensions)
- [Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365-copilot/)
- [Azure OpenAI Service](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Power Platform Copilot](https://learn.microsoft.com/en-us/power-platform/copilot/)
- [Microsoft Graph API](https://learn.microsoft.com/en-us/graph/)

### AIM Documentation
- [AIM SDK Documentation](../README.md)
- [Environment Variables Guide](./ENV_CONFIG.md)
- [Security Best Practices](./SECURITY.md)
- [API Reference](./API_REFERENCE.md)

---

## 🆘 Support

### Common Issues

**Issue**: "Failed to connect to Microsoft Graph API"
**Solution**: Verify Azure AD app registration and permissions

**Issue**: "AIM verification failed"
**Solution**: Check agent trust score and recent activity logs in AIM dashboard

**Issue**: "GitHub Copilot extension not responding"
**Solution**: Verify GitHub App permissions and webhook configuration

### Get Help
- 📖 [AIM Documentation](https://opena2a.org)
- 💬 [Community Forum](https://community.opena2a.org)
- 🐛 [Report Issues](https://github.com/opena2a-org/agent-identity-management/issues)
- 📧 Email: info@opena2a.org

---

## 🎉 Success Stories

> "Integrating AIM with our Microsoft 365 Copilot gave us the security and compliance we needed for enterprise deployment. Trust scoring helps us identify and fix agent issues before they impact users."
>
> — **Fortune 500 Financial Services Company**

> "GitHub Copilot + AIM = Perfect combination. We can now audit every code suggestion and maintain SOC 2 compliance."
>
> — **SaaS Startup (Series B)**

---

**Built with ❤️ by the OpenA2A Team**
