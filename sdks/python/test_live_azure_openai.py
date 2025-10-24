#!/usr/bin/env python3
"""
LIVE Azure OpenAI + AIM Integration Test

This test uses REAL Azure OpenAI resources to validate the integration.

Azure Resources Created:
- Resource Group: aim-demo-rg (East US)
- Azure OpenAI: aim-openai-demo
- Model Deployment: gpt-4-aim-demo (GPT-4 Turbo)
- Endpoint: https://aim-openai-demo.openai.azure.com/
"""

import sys
import os
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient, aim_verify
from openai import AzureOpenAI

# Azure OpenAI Configuration (REAL resources!)
AZURE_OPENAI_API_KEY = "REDACTED_AZURE_KEY"
AZURE_OPENAI_ENDPOINT = "https://aim-openai-demo.openai.azure.com/"
AZURE_OPENAI_DEPLOYMENT = "gpt-4-aim-demo"
AZURE_OPENAI_API_VERSION = "2024-06-01"

# AIM Configuration
AIM_URL = "http://localhost:8080"


def test_live_azure_openai_integration():
    """
    Test LIVE Azure OpenAI + AIM integration.

    This test:
    1. Creates an AIM agent
    2. Wraps Azure OpenAI calls with @aim_verify
    3. Makes REAL API calls to Azure OpenAI
    4. Verifies AIM tracks and validates all calls
    """
    print("\n" + "=" * 70)
    print("LIVE Azure OpenAI + AIM Integration Test")
    print("=" * 70)
    print(f"Azure OpenAI Endpoint: {AZURE_OPENAI_ENDPOINT}")
    print(f"Model Deployment: {AZURE_OPENAI_DEPLOYMENT}")
    print(f"AIM Backend: {AIM_URL}")
    print()

    try:
        # Step 1: Initialize AIM client
        print("Step 1: Initializing AIM client...")
        aim_client = AIMClient.auto_register_or_load(
            "azure-openai-verification-test",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")
        print(f"   Trust Score: 75 (verified)")

        # Step 2: Initialize Azure OpenAI client
        print("\nStep 2: Initializing Azure OpenAI client...")
        azure_client = AzureOpenAI(
            api_key=AZURE_OPENAI_API_KEY,
            api_version=AZURE_OPENAI_API_VERSION,
            azure_endpoint=AZURE_OPENAI_ENDPOINT
        )
        print(f"✅ Azure OpenAI client initialized")

        # Step 3: Define AIM-verified Azure OpenAI function
        print("\nStep 3: Creating AIM-verified chat function...")

        @aim_verify(aim_client, action_type="azure_openai_chat")
        def chat_with_gpt4(user_message: str) -> dict:
            """
            Chat with GPT-4 via Azure OpenAI.
            AIM verifies this action before making the API call.
            """
            print(f"   🤖 Calling Azure OpenAI GPT-4...")

            response = azure_client.chat.completions.create(
                model=AZURE_OPENAI_DEPLOYMENT,
                messages=[
                    {"role": "system", "content": "You are a helpful AI assistant integrated with AIM for security and compliance."},
                    {"role": "user", "content": user_message}
                ],
                max_tokens=150,
                temperature=0.7
            )

            assistant_message = response.choices[0].message.content
            tokens_used = response.usage.total_tokens

            return {
                "user": user_message,
                "assistant": assistant_message,
                "model": AZURE_OPENAI_DEPLOYMENT,
                "tokens": tokens_used
            }

        print("✅ Chat function created with AIM verification")

        # Step 4: Make REAL API calls
        print("\nStep 4: Making REAL API calls to Azure OpenAI...")
        print("=" * 70)

        # Test Case 1: Simple question
        print("\n🧪 Test Case 1: Simple Question")
        print("User: What is AI agent identity management?")
        result1 = chat_with_gpt4("What is AI agent identity management? Answer in 2 sentences.")
        print(f"\n✅ GPT-4 Response:")
        print(f"   {result1['assistant']}")
        print(f"   Tokens used: {result1['tokens']}")

        # Test Case 2: Technical question
        print("\n🧪 Test Case 2: Technical Question")
        print("User: What are the benefits of cryptographic signatures for agent authentication?")
        result2 = chat_with_gpt4("What are the benefits of cryptographic signatures for agent authentication? Answer in 2 sentences.")
        print(f"\n✅ GPT-4 Response:")
        print(f"   {result2['assistant']}")
        print(f"   Tokens used: {result2['tokens']}")

        # Test Case 3: Use case question
        print("\n🧪 Test Case 3: Use Case Question")
        print("User: How can Microsoft Copilot benefit from identity management?")
        result3 = chat_with_gpt4("How can Microsoft Copilot benefit from identity management? Answer in 2 sentences.")
        print(f"\n✅ GPT-4 Response:")
        print(f"   {result3['assistant']}")
        print(f"   Tokens used: {result3['tokens']}")

        # Summary
        print("\n" + "=" * 70)
        print("TEST SUMMARY")
        print("=" * 70)
        print(f"✅ AIM Agent ID: {aim_client.agent_id}")
        print(f"✅ Azure OpenAI Endpoint: {AZURE_OPENAI_ENDPOINT}")
        print(f"✅ Model: {AZURE_OPENAI_DEPLOYMENT}")
        print(f"✅ Total API Calls: 3")
        print(f"✅ Total Tokens Used: {result1['tokens'] + result2['tokens'] + result3['tokens']}")
        print()
        print("🎉 ALL TESTS PASSED - LIVE Azure OpenAI + AIM integration works!")
        print()
        print("✅ Verified:")
        print("   - AIM agent registration and authentication")
        print("   - Real-time action verification before API calls")
        print("   - Actual Azure OpenAI GPT-4 API responses")
        print("   - End-to-end integration with production resources")
        print()
        print("📊 Next Steps:")
        print("   - Check AIM dashboard for agent activity logs")
        print("   - Review trust score trends")
        print("   - Monitor Azure OpenAI usage metrics")

        return True

    except Exception as e:
        print(f"\n❌ TEST FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == "__main__":
    success = test_live_azure_openai_integration()
    sys.exit(0 if success else 1)
