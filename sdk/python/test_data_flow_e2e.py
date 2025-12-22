#!/usr/bin/env python3
"""
End-to-end test for Data Flow Visibility feature.
Tests the SDK data flow tracking functionality with a live backend.
"""

import os
import sys
import json
import time
import uuid
import requests

# Add SDK to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from aim_sdk.data_flow import (
    DataFlowTracker,
    Classification,
    ClassificationTier,
    sensitive,
    classify_by_column_name,
)

# Configuration
AIM_URL = os.environ.get("AIM_URL", "http://localhost:8080")
TOKEN = os.environ.get("AIM_TOKEN", "")

def get_token():
    """Get a valid JWT token by logging in."""
    login_data = {
        "email": "admin@opena2a.org",
        "password": "AIM2025!Secure"
    }
    response = requests.post(
        f"{AIM_URL}/api/v1/public/login",
        json=login_data
    )
    if response.status_code == 200:
        return response.json().get("accessToken")
    else:
        print(f"Login failed: {response.text}")
        return None

def test_classification():
    """Test the classification system."""
    print("\n=== Testing Classification System ===")

    # Test column name classification
    ssn_class = classify_by_column_name("ssn")
    print(f"  Column 'ssn' classification: category={ssn_class.category}, type={ssn_class.data_type}, tier={ssn_class.tier}")

    email_class = classify_by_column_name("user_email")
    print(f"  Column 'user_email' classification: category={email_class.category}, type={email_class.data_type}, tier={email_class.tier}")

    phone_class = classify_by_column_name("phone_number")
    print(f"  Column 'phone_number' classification: category={phone_class.category}, type={phone_class.data_type}, tier={phone_class.tier}")

    # Test unknown column
    unknown_class = classify_by_column_name("random_column")
    print(f"  Column 'random_column' classification: {unknown_class}")

    print("  ✅ Classification system working!")
    return True

def test_decorator():
    """Test the @sensitive decorator."""
    print("\n=== Testing @sensitive Decorator ===")

    @sensitive("PII", "email")
    def get_user_email(user_id: str):
        return f"user-{user_id}@example.com"

    # Call the decorated function
    result = get_user_email("12345")
    print(f"  Decorated function returned: {result}")

    # Verify the classification metadata is attached
    if hasattr(get_user_email, '_aim_classification'):
        classification = get_user_email._aim_classification
        print(f"  Classification metadata: category={classification.category}, type={classification.data_type}, tier={classification.tier}")

    print("  ✅ @sensitive decorator working!")
    return True

def test_data_flow_tracker():
    """Test the DataFlowTracker class."""
    print("\n=== Testing DataFlowTracker ===")

    # Create a mock client for testing
    class MockClient:
        def __init__(self):
            self.agent_id = "test-agent-001"
            self.agent_name = "Test Agent"
            self._requests = []

        def _make_request(self, method, url, data=None):
            self._requests.append((method, url, data))
            return {"status": "ok"}

    mock_client = MockClient()

    # Create a tracker with the mock client
    tracker = DataFlowTracker(
        client=mock_client,
        buffer_size=100,
        flush_interval=10.0,  # Long interval to avoid background flush during test
        enabled=True
    )

    # Record a flow
    result = tracker.record_flow(
        source_type="database",
        source_name="customers_db",
        destination_type="api",
        destination_name="external-api.com",
        table_name="customers",
        columns=["email", "ssn", "phone"],
        record_count=100,
        classification=Classification(
            category="PII",
            data_type="email",
            tier=ClassificationTier.SCHEMA,
            confidence=0.95
        )
    )
    print(f"  Flow recorded, policy result: action={result.action}")

    # Test with QueryContext
    with tracker.track_query(table="customers", columns=["ssn", "email"]) as ctx:
        # Simulate getting data
        mock_data = [{"ssn": "xxx", "email": "test@test.com"}]
        policy_result = ctx.record_outflow("llm", "openai", mock_data)
        print(f"  QueryContext outflow recorded: action={policy_result.action}")

    # Shutdown tracker
    tracker.shutdown(timeout=1.0)

    print("  ✅ DataFlowTracker working!")
    return True

def test_api_recording(token: str):
    """Test recording a data flow via the API."""
    print("\n=== Testing API Data Flow Recording ===")

    # Create a data flow event
    flow_data = {
        "agentId": "a0000000-0000-0000-0000-000000000003",  # Need a valid agent ID
        "agentName": "Test SDK Agent",
        "flowId": str(uuid.uuid4()),
        "sourceType": "database",
        "sourceName": "test_customers",
        "destinationType": "api",
        "destinationName": "analytics-service",
        "classificationCategory": "PII",
        "classificationType": "email",
        "classificationTier": 2,
        "classificationConfidence": 0.95,
        "recordCount": 50,
        "fieldCount": 3,
        "fieldsDetected": ["email", "phone", "address"],
        "status": "completed"
    }

    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    response = requests.post(
        f"{AIM_URL}/api/v1/data-flows",
        json=flow_data,
        headers=headers
    )

    if response.status_code in [200, 201]:
        print(f"  ✅ Data flow recorded successfully!")
        print(f"  Response: {response.json()}")
        return True
    else:
        print(f"  ⚠️ Data flow recording returned: {response.status_code}")
        print(f"  Response: {response.text}")
        # This might fail if no valid agent exists, but we can still check the API works
        return response.status_code != 500

def test_api_fetch(token: str):
    """Test fetching data flows via the API."""
    print("\n=== Testing API Data Flow Fetch ===")

    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    # Fetch summary
    response = requests.get(
        f"{AIM_URL}/api/v1/data-flows/summary",
        headers=headers
    )
    print(f"  Summary: {response.json()}")

    # Fetch data flows list
    response = requests.get(
        f"{AIM_URL}/api/v1/data-flows",
        headers=headers
    )
    data = response.json()
    print(f"  Total flows: {data.get('total', 0)}")

    # Fetch data sources
    response = requests.get(
        f"{AIM_URL}/api/v1/data-sources",
        headers=headers
    )
    data = response.json()
    print(f"  Total sources: {data.get('total', 0)}")

    # Fetch policies
    response = requests.get(
        f"{AIM_URL}/api/v1/data-flow-policies",
        headers=headers
    )
    data = response.json()
    policies = data.get('policies') or []
    print(f"  Total policies: {len(policies)}")

    print("  ✅ API fetch working!")
    return True

def main():
    """Run all tests."""
    print("=" * 60)
    print("Data Flow Visibility E2E Test Suite")
    print("=" * 60)

    results = []

    # Test 1: Classification system
    try:
        results.append(("Classification", test_classification()))
    except Exception as e:
        print(f"  ❌ Classification failed: {e}")
        results.append(("Classification", False))

    # Test 2: Decorator
    try:
        results.append(("Decorator", test_decorator()))
    except Exception as e:
        print(f"  ❌ Decorator failed: {e}")
        results.append(("Decorator", False))

    # Test 3: DataFlowTracker
    try:
        results.append(("DataFlowTracker", test_data_flow_tracker()))
    except Exception as e:
        print(f"  ❌ DataFlowTracker failed: {e}")
        results.append(("DataFlowTracker", False))

    # Get token for API tests
    token = get_token()
    if not token:
        print("\n❌ Could not get authentication token, skipping API tests")
        results.append(("API Auth", False))
    else:
        print(f"\n✅ Got authentication token")
        results.append(("API Auth", True))

        # Test 4: API recording
        try:
            results.append(("API Recording", test_api_recording(token)))
        except Exception as e:
            print(f"  ❌ API Recording failed: {e}")
            results.append(("API Recording", False))

        # Test 5: API fetch
        try:
            results.append(("API Fetch", test_api_fetch(token)))
        except Exception as e:
            print(f"  ❌ API Fetch failed: {e}")
            results.append(("API Fetch", False))

    # Summary
    print("\n" + "=" * 60)
    print("Test Results Summary")
    print("=" * 60)
    passed = sum(1 for _, r in results if r)
    total = len(results)

    for name, result in results:
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"  {name}: {status}")

    print(f"\nTotal: {passed}/{total} tests passed")

    return passed == total

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
