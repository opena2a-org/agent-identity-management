#!/usr/bin/env python3
"""
Data Flow Visibility SDK Integration Tests

Tests the complete data flow tracking workflow:
- DataFlowTracker initialization and configuration
- Classification helpers (column/table inference)
- @sensitive decorator (Tier 1 explicit classification)
- QueryContext for database query tracking
- HTTPInterceptor for automatic flow tracking
- Async/non-blocking event buffering
"""

import os
import sys
import time
import json
import unittest
from unittest.mock import Mock, patch, MagicMock
import threading
from queue import Queue

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from aim_sdk import register_agent
from aim_sdk.data_flow import (
    DataFlowTracker,
    classify_column,
    classify_from_context,
    sensitive,
    track_data_flow,
    QueryContext,
    HTTPInterceptor,
    ClassificationTier,
    ClassificationCategory,
)


class TestClassificationHelpers(unittest.TestCase):
    """Tests for classification helper functions"""

    def test_classify_column_email(self):
        """Email columns should be classified as PII.email"""
        result = classify_column("email")
        self.assertEqual(result["category"], "pii")
        self.assertEqual(result["type"], "email")
        self.assertEqual(result["tier"], ClassificationTier.SCHEMA)
        self.assertGreaterEqual(result["confidence"], 0.9)

    def test_classify_column_email_address(self):
        """email_address column should be classified as PII.email"""
        result = classify_column("email_address")
        self.assertEqual(result["category"], "pii")
        self.assertEqual(result["type"], "email")

    def test_classify_column_ssn(self):
        """SSN columns should be classified as PII.ssn"""
        result = classify_column("ssn")
        self.assertEqual(result["category"], "pii")
        self.assertEqual(result["type"], "ssn")

    def test_classify_column_social_security_number(self):
        """social_security_number should be classified as PII.ssn"""
        result = classify_column("social_security_number")
        self.assertEqual(result["category"], "pii")
        self.assertEqual(result["type"], "ssn")

    def test_classify_column_phone(self):
        """Phone columns should be classified as PII.phone"""
        for col in ["phone", "phone_number", "mobile", "cell_phone"]:
            result = classify_column(col)
            self.assertEqual(result["category"], "pii", f"Failed for {col}")
            self.assertEqual(result["type"], "phone", f"Failed for {col}")

    def test_classify_column_credit_card(self):
        """Credit card columns should be classified as PCI.credit_card"""
        for col in ["credit_card", "card_number", "cc_number", "pan"]:
            result = classify_column(col)
            self.assertEqual(result["category"], "pci", f"Failed for {col}")
            self.assertEqual(result["type"], "credit_card", f"Failed for {col}")

    def test_classify_column_medical_record(self):
        """Medical columns should be classified as PHI"""
        for col in ["medical_record_number", "mrn", "diagnosis", "patient_id"]:
            result = classify_column(col)
            self.assertEqual(result["category"], "phi", f"Failed for {col}")

    def test_classify_column_unknown(self):
        """Unknown columns should return None or low confidence"""
        result = classify_column("some_random_field")
        self.assertIsNone(result)

    def test_classify_from_context_table_column(self):
        """Table.column context should improve classification"""
        result = classify_from_context("customers", "email")
        self.assertEqual(result["category"], "pii")
        self.assertEqual(result["type"], "email")

    def test_classify_from_context_medical_table(self):
        """Medical table context should hint at PHI classification"""
        result = classify_from_context("patient_records", "record_id")
        self.assertEqual(result["category"], "phi")


class TestSensitiveDecorator(unittest.TestCase):
    """Tests for @sensitive decorator (Tier 1 explicit classification)"""

    def test_sensitive_decorator_basic(self):
        """@sensitive should mark function output as classified"""
        @sensitive(category="pii", data_type="email")
        def get_email():
            return "user@example.com"

        result = get_email()
        self.assertEqual(result, "user@example.com")

    def test_sensitive_decorator_with_tracker(self):
        """@sensitive should record flow when tracker is active"""
        mock_client = Mock()
        mock_client.record_data_flow = Mock(return_value={"id": "flow-123"})

        tracker = DataFlowTracker(mock_client, enabled=True)

        @sensitive(category="pii", data_type="ssn", tracker=tracker)
        def get_ssn():
            return "123-45-6789"

        result = get_ssn()
        self.assertEqual(result, "123-45-6789")

    def test_sensitive_decorator_tier_is_explicit(self):
        """@sensitive should always be Tier 1 (explicit)"""
        @sensitive(category="pci", data_type="credit_card")
        def get_card():
            return "4111111111111111"

        # Check that metadata includes tier=1
        self.assertTrue(hasattr(get_card, "_aim_classification"))
        self.assertEqual(get_card._aim_classification["tier"], ClassificationTier.EXPLICIT)
        self.assertEqual(get_card._aim_classification["confidence"], 1.0)


class TestDataFlowTracker(unittest.TestCase):
    """Tests for DataFlowTracker class"""

    def test_tracker_initialization(self):
        """Tracker should initialize with correct defaults"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client)

        self.assertTrue(tracker.enabled)
        self.assertEqual(tracker._buffer_size, 1000)
        self.assertEqual(tracker._flush_interval, 1.0)

    def test_tracker_disabled(self):
        """Tracker should not record when disabled"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=False)

        tracker.record_flow(
            source_type="database",
            source_name="users.email",
            destination="openai-api",
            data_types=["pii.email"]
        )

        # Should not have queued any events
        self.assertTrue(tracker._event_queue.empty())

    def test_tracker_record_flow(self):
        """Tracker should queue flow events"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)

        # Stop background flush to control testing
        tracker._stop_flush.set()

        tracker.record_flow(
            source_type="database",
            source_name="customers.ssn",
            destination="llm-api",
            data_types=["pii.ssn"]
        )

        self.assertFalse(tracker._event_queue.empty())
        event = tracker._event_queue.get_nowait()
        self.assertEqual(event["sourceType"], "database")
        self.assertEqual(event["sourceName"], "customers.ssn")
        self.assertEqual(event["destination"], "llm-api")

    def test_tracker_non_blocking(self):
        """record_flow should not block the caller"""
        mock_client = Mock()

        # Simulate slow API
        def slow_record(*args, **kwargs):
            time.sleep(2)
            return {"id": "flow-123"}

        mock_client.record_data_flow = slow_record
        tracker = DataFlowTracker(mock_client, enabled=True)

        start = time.time()
        tracker.record_flow(
            source_type="database",
            source_name="test",
            destination="api",
            data_types=["pii.email"]
        )
        elapsed = time.time() - start

        # Should return immediately, not wait for API
        self.assertLess(elapsed, 0.1, "record_flow should be non-blocking")

    def test_tracker_flush(self):
        """Tracker should flush events to API"""
        mock_client = Mock()
        mock_client.record_data_flow_batch = Mock(return_value={"count": 1})

        tracker = DataFlowTracker(mock_client, enabled=True, flush_interval=0.1)
        tracker._stop_flush.set()  # Stop auto-flush

        # Queue an event
        tracker.record_flow(
            source_type="database",
            source_name="test",
            destination="api",
            data_types=["pii.email"]
        )

        # Manual flush
        tracker.flush()

        # Should have called API
        self.assertTrue(
            mock_client.record_data_flow_batch.called or
            mock_client.record_data_flow.called
        )

    def test_tracker_buffer_overflow(self):
        """Tracker should handle buffer overflow gracefully"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True, buffer_size=5)
        tracker._stop_flush.set()

        # Fill buffer beyond capacity
        for i in range(10):
            tracker.record_flow(
                source_type="database",
                source_name=f"field_{i}",
                destination="api",
                data_types=["pii.email"]
            )

        # Should not crash, oldest events may be dropped
        self.assertLessEqual(tracker._event_queue.qsize(), 5)


class TestQueryContext(unittest.TestCase):
    """Tests for QueryContext (database query tracking)"""

    def test_query_context_basic(self):
        """QueryContext should track query metadata"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()

        with QueryContext(tracker, table="customers", columns=["email", "name"]) as ctx:
            # Simulate database query
            result = {"email": "test@example.com", "name": "John"}

        # Should have classified columns
        self.assertIn("email", ctx.classified_columns)
        self.assertEqual(ctx.classified_columns["email"]["category"], "pii")

    def test_query_context_with_connection_string(self):
        """QueryContext should include connection metadata"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()

        with QueryContext(
            tracker,
            table="users",
            columns=["email"],
            connection_string="postgres://localhost/mydb"
        ) as ctx:
            pass

        self.assertEqual(ctx.connection_string, "postgres://localhost/mydb")

    def test_query_context_records_flow(self):
        """QueryContext should record flow when exiting"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()

        with QueryContext(
            tracker,
            table="customers",
            columns=["email"],
            destination="openai-api"
        ) as ctx:
            pass

        # Should have queued a flow event
        self.assertFalse(tracker._event_queue.empty())


class TestHTTPInterceptor(unittest.TestCase):
    """Tests for HTTPInterceptor (automatic flow tracking)"""

    def test_interceptor_initialization(self):
        """HTTPInterceptor should initialize correctly"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        interceptor = HTTPInterceptor(tracker)

        self.assertEqual(interceptor.tracker, tracker)
        self.assertEqual(len(interceptor.sensitive_destinations), 0)

    def test_interceptor_add_destination(self):
        """Should be able to add sensitive destinations"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        interceptor = HTTPInterceptor(tracker)

        interceptor.add_sensitive_destination(
            pattern="openai.com",
            name="OpenAI API"
        )

        self.assertEqual(len(interceptor.sensitive_destinations), 1)

    def test_interceptor_matches_destination(self):
        """Interceptor should match destination patterns"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        interceptor = HTTPInterceptor(tracker)

        interceptor.add_sensitive_destination(
            pattern=r"api\.openai\.com",
            name="OpenAI API"
        )

        match = interceptor._match_destination("https://api.openai.com/v1/chat")
        self.assertIsNotNone(match)
        self.assertEqual(match["name"], "OpenAI API")

    def test_interceptor_record_request(self):
        """Interceptor should record outgoing requests"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()
        interceptor = HTTPInterceptor(tracker)

        interceptor.add_sensitive_destination(
            pattern=r"api\.openai\.com",
            name="OpenAI API"
        )

        interceptor.record_request(
            url="https://api.openai.com/v1/chat/completions",
            method="POST",
            body={"messages": [{"content": "Hello"}]},
            detected_data_types=["pii.email"]
        )

        self.assertFalse(tracker._event_queue.empty())


class TestTrackDataFlowDecorator(unittest.TestCase):
    """Tests for @track_data_flow decorator"""

    def test_decorator_basic(self):
        """@track_data_flow should track function execution"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()

        @track_data_flow(tracker, destination="external-api")
        def call_api(data):
            return {"status": "ok"}

        result = call_api({"email": "test@example.com"})
        self.assertEqual(result["status"], "ok")

    def test_decorator_with_classification(self):
        """@track_data_flow should record classification from input"""
        mock_client = Mock()
        tracker = DataFlowTracker(mock_client, enabled=True)
        tracker._stop_flush.set()

        @track_data_flow(
            tracker,
            destination="openai-api",
            input_classifications={"email": "pii.email"}
        )
        def send_to_llm(email):
            return f"Processed: {email}"

        result = send_to_llm("user@example.com")

        # Should have queued flow with classification
        self.assertFalse(tracker._event_queue.empty())


class TestClassificationTiers(unittest.TestCase):
    """Tests for classification tier constants"""

    def test_tier_values(self):
        """Tier values should follow expected hierarchy"""
        self.assertEqual(ClassificationTier.EXPLICIT, 1)
        self.assertEqual(ClassificationTier.SCHEMA, 2)
        self.assertEqual(ClassificationTier.HEURISTIC, 3)
        self.assertEqual(ClassificationTier.CONTENT, 4)

    def test_category_values(self):
        """Category values should be defined"""
        self.assertEqual(ClassificationCategory.PII, "pii")
        self.assertEqual(ClassificationCategory.PHI, "phi")
        self.assertEqual(ClassificationCategory.PCI, "pci")


class TestDataFlowLiveIntegration(unittest.TestCase):
    """Live integration tests (require running AIM backend)"""

    @classmethod
    def setUpClass(cls):
        """Setup: Register test agent"""
        cls.aim_url = os.getenv("AIM_URL", "http://localhost:8080")

        # Check if backend is running
        import requests
        try:
            resp = requests.get(f"{cls.aim_url}/health", timeout=2)
            if resp.status_code != 200:
                raise unittest.SkipTest("AIM backend not running")

            data = resp.json()
            if data.get("service") != "agent-identity-management":
                raise unittest.SkipTest("AIM backend not running (different service)")
        except requests.exceptions.RequestException:
            raise unittest.SkipTest("AIM backend not reachable")

        # Register test agent
        try:
            cls.agent = register_agent(
                f"data-flow-test-{int(time.time())}",
                aim_url=cls.aim_url,
                description="Data flow integration test agent"
            )
        except Exception as e:
            raise unittest.SkipTest(f"Could not register test agent: {e}")

    def test_live_record_flow(self):
        """Live: Record a data flow event"""
        tracker = DataFlowTracker(self.agent, enabled=True)

        tracker.record_flow(
            source_type="database",
            source_name="customers.email",
            destination="openai-api",
            data_types=["pii.email"],
            metadata={"test": True}
        )

        # Flush to ensure event is sent
        tracker.flush()

        # Give backend time to process
        time.sleep(0.5)

        # Verify flow was recorded (would need API call to verify)
        # For now, just ensure no exceptions

    def test_live_sensitive_decorator(self):
        """Live: Use @sensitive decorator"""
        tracker = DataFlowTracker(self.agent, enabled=True)

        @sensitive(category="pii", data_type="ssn", tracker=tracker)
        def get_customer_ssn(customer_id):
            return "123-45-6789"

        result = get_customer_ssn("cust-001")
        self.assertEqual(result, "123-45-6789")

        tracker.flush()

    def test_live_query_context(self):
        """Live: Use QueryContext for database tracking"""
        tracker = DataFlowTracker(self.agent, enabled=True)

        with QueryContext(
            tracker,
            table="customers",
            columns=["email", "ssn", "name"],
            destination="analytics-service"
        ) as ctx:
            # Simulate query
            results = [
                {"email": "a@example.com", "ssn": "111-22-3333", "name": "Alice"},
                {"email": "b@example.com", "ssn": "444-55-6666", "name": "Bob"},
            ]

        tracker.flush()

        # Verify classifications were detected
        self.assertIn("email", ctx.classified_columns)
        self.assertIn("ssn", ctx.classified_columns)

    def test_live_http_interceptor(self):
        """Live: Use HTTPInterceptor for request tracking"""
        tracker = DataFlowTracker(self.agent, enabled=True)
        interceptor = HTTPInterceptor(tracker)

        interceptor.add_sensitive_destination(
            pattern=r"api\.openai\.com",
            name="OpenAI API"
        )

        # Simulate intercepted request
        interceptor.record_request(
            url="https://api.openai.com/v1/chat/completions",
            method="POST",
            body={"messages": [{"role": "user", "content": "Hello"}]},
            detected_data_types=["pii.email"]
        )

        tracker.flush()


if __name__ == "__main__":
    unittest.main(verbosity=2)
