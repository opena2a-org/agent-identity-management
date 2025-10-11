#!/usr/bin/env python3
"""
Microsoft Copilot + AIM Integration Demo

This demonstrates how AIM integrates with Microsoft Copilot platforms.
Since we don't have live Azure OpenAI resources, this uses simulated
endpoints to show the integration pattern.

In production, replace the simulated functions with actual:
- Azure OpenAI SDK calls
- Microsoft Graph API calls
- GitHub API calls
"""

import sys
import os
from pathlib import Path
import time
from typing import Dict, Any

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient, aim_verify, aim_verify_api_call, aim_verify_external_service

AIM_URL = "http://localhost:8080"


# =============================================================================
# Simulated Microsoft Services (replace with real SDKs in production)
# =============================================================================

class SimulatedAzureOpenAI:
    """Simulates Azure OpenAI API"""

    def __init__(self, api_key: str, endpoint: str):
        self.api_key = api_key
        self.endpoint = endpoint

    def chat_completion(self, messages: list, model: str = "gpt-4") -> Dict[str, Any]:
        """Simulates Azure OpenAI chat completion"""
        user_message = messages[-1]["content"] if messages else "Hello"
        return {
            "choices": [{
                "message": {
                    "content": f"I'm a simulated Azure OpenAI response to: {user_message}"
                }
            }],
            "usage": {
                "total_tokens": 50
            }
        }


class SimulatedGraphClient:
    """Simulates Microsoft Graph API"""

    def __init__(self, tenant_id: str, client_id: str, client_secret: str):
        self.tenant_id = tenant_id
        self.client_id = client_id
        self.client_secret = client_secret

    def send_email(self, to: str, subject: str, body: str) -> Dict[str, Any]:
        """Simulates sending email via Microsoft Graph"""
        return {
            "sent": True,
            "to": to,
            "subject": subject,
            "message_id": "msg-123456"
        }

    def read_email(self, email_id: str) -> Dict[str, Any]:
        """Simulates reading email via Microsoft Graph"""
        return {
            "id": email_id,
            "subject": "Sample Email Subject",
            "from": "sender@example.com",
            "body": "Email body content here..."
        }


class SimulatedGitHubClient:
    """Simulates GitHub API"""

    def __init__(self, token: str):
        self.token = token

    def get_pull_request(self, repo: str, pr_number: int) -> Dict[str, Any]:
        """Simulates fetching a GitHub PR"""
        return {
            "number": pr_number,
            "title": f"Sample PR #{pr_number}",
            "state": "open",
            "files_changed": 5,
            "additions": 120,
            "deletions": 45
        }

    def review_code(self, repo: str, pr_number: int, comments: list) -> Dict[str, Any]:
        """Simulates posting PR review"""
        return {
            "pr": pr_number,
            "comments_posted": len(comments),
            "status": "success"
        }


# =============================================================================
# Test 1: Azure OpenAI Chatbot with AIM
# =============================================================================

def test_azure_openai_integration():
    """Test 1: Azure OpenAI Copilot with AIM verification"""
    print("\n" + "=" * 70)
    print("TEST 1: Azure OpenAI Copilot Integration")
    print("=" * 70)

    try:
        # Initialize AIM client
        aim_client = AIMClient.auto_register_or_load(
            "azure-openai-copilot",
            AIM_URL
        )
        print(f"✅ AIM client initialized: {aim_client.agent_id}")

        # Initialize simulated Azure OpenAI
        azure_openai = SimulatedAzureOpenAI(
            api_key="simulated-key",
            endpoint="https://example.openai.azure.com"
        )

        # Define Copilot chat function with AIM verification
        @aim_verify(aim_client, action_type="azure_openai_chat", risk_level="medium")
        def copilot_chat(user_message: str) -> str:
            """
            Azure OpenAI Copilot processes user chat.
            AIM verifies this action before calling Azure OpenAI.
            """
            messages = [{"role": "user", "content": user_message}]
            response = azure_openai.chat_completion(messages)
            return response["choices"][0]["message"]["content"]

        # Test the integration
        print("\n🔍 User asks: 'What are the latest sales numbers?'")
        response = copilot_chat("What are the latest sales numbers?")
        print(f"✅ Copilot response: {response}")

        print("\n🎉 TEST 1 PASSED - Azure OpenAI integration works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# =============================================================================
# Test 2: Microsoft 365 Copilot Email Assistant
# =============================================================================

def test_m365_copilot_integration():
    """Test 2: Microsoft 365 Copilot with AIM verification"""
    print("\n" + "=" * 70)
    print("TEST 2: Microsoft 365 Copilot Integration")
    print("=" * 70)

    try:
        # Initialize AIM client
        aim_client = AIMClient.auto_register_or_load(
            "m365-copilot-email",
            AIM_URL
        )
        print(f"✅ AIM client initialized: {aim_client.agent_id}")

        # Initialize simulated Microsoft Graph
        graph_client = SimulatedGraphClient(
            tenant_id="tenant-123",
            client_id="client-456",
            client_secret="secret-789"
        )

        # Define email sending function with HIGH risk level
        @aim_verify_external_service(aim_client, risk_level="high")
        def copilot_send_email(to: str, subject: str, body: str) -> Dict[str, Any]:
            """
            M365 Copilot sends email on behalf of user.
            HIGH risk level - requires AIM verification.
            """
            result = graph_client.send_email(to, subject, body)
            return result

        # Define email reading function with MEDIUM risk level
        @aim_verify_api_call(aim_client, risk_level="medium")
        def copilot_read_email(email_id: str) -> Dict[str, Any]:
            """
            M365 Copilot reads user email.
            MEDIUM risk level - requires AIM verification.
            """
            email = graph_client.read_email(email_id)
            return email

        # Test email reading
        print("\n🔍 Copilot reads email 'email-123'...")
        email = copilot_read_email("email-123")
        print(f"✅ Email read: {email['subject']}")

        # Test email sending (high risk)
        print("\n🔍 Copilot sends email to 'colleague@example.com'...")
        result = copilot_send_email(
            to="colleague@example.com",
            subject="Meeting Summary",
            body="Here's the summary from today's meeting..."
        )
        print(f"✅ Email sent: {result['message_id']}")

        print("\n🎉 TEST 2 PASSED - M365 Copilot integration works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# =============================================================================
# Test 3: GitHub Copilot Code Review
# =============================================================================

def test_github_copilot_integration():
    """Test 3: GitHub Copilot with AIM verification"""
    print("\n" + "=" * 70)
    print("TEST 3: GitHub Copilot Integration")
    print("=" * 70)

    try:
        # Initialize AIM client
        aim_client = AIMClient.auto_register_or_load(
            "github-copilot-reviewer",
            AIM_URL
        )
        print(f"✅ AIM client initialized: {aim_client.agent_id}")

        # Initialize simulated GitHub client
        github = SimulatedGitHubClient(token="ghp_simulated_token")

        # Define PR review function
        @aim_verify(aim_client, action_type="code_review", risk_level="low")
        def copilot_review_pr(repo: str, pr_number: int) -> Dict[str, Any]:
            """
            GitHub Copilot reviews pull request.
            LOW risk level - informational only.
            """
            # Get PR details
            pr = github.get_pull_request(repo, pr_number)

            # Simulate code analysis
            review_comments = []
            if pr["additions"] > 100:
                review_comments.append({
                    "line": 1,
                    "comment": "⚠️  Large PR (100+ lines) - consider breaking into smaller PRs"
                })

            # Post review
            github.review_code(repo, pr_number, review_comments)

            return {
                "pr": pr_number,
                "title": pr["title"],
                "files_changed": pr["files_changed"],
                "comments": len(review_comments)
            }

        # Test the integration
        print("\n🔍 Copilot reviews PR #123 in org/repo...")
        review = copilot_review_pr("org/repo", 123)
        print(f"✅ PR reviewed: {review['title']}")
        print(f"   Files changed: {review['files_changed']}")
        print(f"   Comments posted: {review['comments']}")

        print("\n🎉 TEST 3 PASSED - GitHub Copilot integration works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# =============================================================================
# Test 4: Environment Variable Auto-Configuration
# =============================================================================

def test_env_var_configuration():
    """Test 4: Environment variable auto-configuration"""
    print("\n" + "=" * 70)
    print("TEST 4: Environment Variable Auto-Configuration")
    print("=" * 70)

    try:
        # Set environment variables
        os.environ["AIM_AGENT_NAME"] = "env-configured-copilot"
        os.environ["AIM_URL"] = AIM_URL
        os.environ["AIM_AUTO_REGISTER"] = "true"

        print("✅ Environment configured:")
        print(f"   AIM_AGENT_NAME={os.environ['AIM_AGENT_NAME']}")
        print(f"   AIM_URL={os.environ['AIM_URL']}")

        # Define function with auto-init (no explicit client!)
        @aim_verify(auto_init=True, action_type="copilot_action")
        def auto_configured_copilot_function():
            """
            This function auto-initializes AIM from environment variables.
            No explicit AIMClient needed!
            """
            return {"status": "success", "message": "Auto-configured!"}

        # Call the function - client auto-initializes
        print("\n🔍 Calling auto-configured function...")
        result = auto_configured_copilot_function()
        print(f"✅ Function executed: {result['message']}")

        print("\n🎉 TEST 4 PASSED - Environment auto-configuration works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# =============================================================================
# Main Test Runner
# =============================================================================

def main():
    """Run all Microsoft Copilot integration tests"""
    print("=" * 70)
    print("Microsoft Copilot + AIM Integration Tests")
    print("=" * 70)
    print(f"AIM Server: {AIM_URL}")
    print()
    print("📝 NOTE: This demo uses simulated Microsoft services.")
    print("   In production, replace with actual Azure OpenAI SDK,")
    print("   Microsoft Graph API, and GitHub API calls.")
    print()

    results = []

    # Test 1: Azure OpenAI
    test1_passed = test_azure_openai_integration()
    results.append(("Azure OpenAI Integration", test1_passed))

    # Test 2: Microsoft 365
    test2_passed = test_m365_copilot_integration()
    results.append(("Microsoft 365 Integration", test2_passed))

    # Test 3: GitHub Copilot
    test3_passed = test_github_copilot_integration()
    results.append(("GitHub Copilot Integration", test3_passed))

    # Test 4: Environment configuration
    test4_passed = test_env_var_configuration()
    results.append(("Environment Configuration", test4_passed))

    # Summary
    print("\n" + "=" * 70)
    print("TEST SUMMARY")
    print("=" * 70)

    passed = sum(1 for _, result in results if result)
    total = len(results)

    for test_name, result in results:
        status = "✅ PASSED" if result else "❌ FAILED"
        print(f"{status}: {test_name}")

    print(f"\nTotal: {passed}/{total} tests passed")

    if passed == total:
        print("\n🎉 ALL TESTS PASSED - Microsoft Copilot integration ready!")
        print("\n📚 Next Steps:")
        print("   1. Get Azure OpenAI API key")
        print("   2. Get Microsoft Graph API credentials")
        print("   3. Get GitHub personal access token")
        print("   4. Replace simulated clients with real SDK calls")
        print("   5. Deploy to production!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
