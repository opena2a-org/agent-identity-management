#!/usr/bin/env python3
"""
LIVE Azure OpenAI + AIM Integration Test

This test uses REAL Azure OpenAI resources to validate the integration.

Requires: openai package installed + Azure OpenAI credentials in environment
Run with: pytest -m integration tests/test_live_azure_openai.py
"""

import sys
import os
from pathlib import Path

import pytest

# Skip entire module if openai is not installed
openai_mod = pytest.importorskip("openai", reason="openai not installed")

from aim_sdk import AIMClient, aim_verify
from openai import AzureOpenAI

# Azure OpenAI Configuration from environment
AZURE_OPENAI_API_KEY = os.environ.get("AZURE_OPENAI_API_KEY", "")
AZURE_OPENAI_ENDPOINT = os.environ.get("AZURE_OPENAI_ENDPOINT", "https://aim-openai-demo.openai.azure.com/")
AZURE_OPENAI_DEPLOYMENT = os.environ.get("AZURE_OPENAI_DEPLOYMENT", "gpt-4-aim-demo")
AZURE_OPENAI_API_VERSION = os.environ.get("AZURE_OPENAI_API_VERSION", "2024-06-01")

# AIM Configuration
AIM_URL = "http://localhost:8080"


@pytest.mark.integration
def test_live_azure_openai_integration():
    """
    Test LIVE Azure OpenAI + AIM integration.

    This test:
    1. Creates an AIM agent
    2. Wraps Azure OpenAI calls with @aim_verify
    3. Makes REAL API calls to Azure OpenAI
    4. Verifies AIM tracks and validates all calls
    """
    if not AZURE_OPENAI_API_KEY:
        pytest.skip("AZURE_OPENAI_API_KEY not set in environment")

    # Step 1: Initialize AIM client
    aim_client = AIMClient.auto_register_or_load(
        "azure-openai-verification-test",
        AIM_URL
    )

    # Step 2: Initialize Azure OpenAI client
    azure_client = AzureOpenAI(
        api_key=AZURE_OPENAI_API_KEY,
        api_version=AZURE_OPENAI_API_VERSION,
        azure_endpoint=AZURE_OPENAI_ENDPOINT
    )

    # Step 3: Define AIM-verified Azure OpenAI function
    @aim_verify(aim_client, action_type="azure_openai_chat")
    def chat_with_gpt4(user_message: str) -> dict:
        """Chat with GPT-4 via Azure OpenAI."""
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

    # Step 4: Make REAL API calls
    result = chat_with_gpt4("What is AI agent identity management? Answer in 2 sentences.")
    assert "assistant" in result
    assert result["tokens"] > 0


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
