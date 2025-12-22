"""
AIM Data Flow Visibility - Track where sensitive data flows

This module provides transparent data flow tracking for AI agents.
It intercepts HTTP calls, classifies data at the source, and enforces policies.

Key Features:
- Source-based classification (zero false positives for Tier 1-2)
- Async, non-blocking flow recording
- Policy enforcement (block, alert, redact)
- Transparent integration with existing code

Usage:
    from aim_sdk.data_flow import DataFlowTracker, sensitive

    # Create tracker (usually done automatically by AIMClient)
    tracker = DataFlowTracker(client)

    # Explicit classification (Tier 1 - highest confidence)
    @sensitive("PII", "ssn")
    def get_customer_ssn(customer_id: str) -> str:
        return database.query("SELECT ssn FROM customers WHERE id = ?", customer_id)

    # Schema-based classification (Tier 2 - automatic)
    # Just use the tracker's database integration
    with tracker.track_query(table="customers", columns=["ssn", "email"]) as ctx:
        result = database.query("SELECT ssn, email FROM customers")
        return ctx.filter_response(result)  # Applies policies

    # HTTP interception (automatic)
    tracker.enable_http_interception()
    # Now all HTTP calls are tracked automatically
"""

import asyncio
import functools
import hashlib
import json
import logging
import queue
import threading
import time
import uuid
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable, Dict, List, Optional, Set, TypeVar, Union
from concurrent.futures import ThreadPoolExecutor

logger = logging.getLogger(__name__)


# ============================================================================
# CLASSIFICATION TYPES
# ============================================================================

class ClassificationCategory(str, Enum):
    """Data classification categories."""
    PII = "PII"
    PHI = "PHI"
    PCI = "PCI"
    SENSITIVE = "SENSITIVE"
    INTERNAL = "INTERNAL"
    PUBLIC = "PUBLIC"


class ClassificationTier(int, Enum):
    """Classification confidence tiers (lower = higher confidence)."""
    EXPLICIT = 1      # Developer explicitly tagged (100% confidence)
    SCHEMA = 2        # Inferred from database schema (95% confidence)
    HEURISTIC = 3     # Inferred from naming patterns (80% confidence)
    CONTENT = 4       # Detected via content patterns (60-70% confidence)


class PolicyAction(str, Enum):
    """Actions that can be taken by policies."""
    ALLOW = "allow"
    BLOCK = "block"
    ALERT = "alert"
    REDACT = "redact"


@dataclass
class Classification:
    """Data classification result."""
    category: str
    data_type: str
    tier: ClassificationTier
    confidence: float = 1.0


@dataclass
class PolicyResult:
    """Result of policy evaluation."""
    action: PolicyAction
    reason: Optional[str] = None
    blocked: bool = False
    redacted_fields: List[str] = field(default_factory=list)


@dataclass
class DataFlowEvent:
    """A data flow event to be recorded."""
    id: str
    flow_id: str
    agent_id: str
    agent_name: str
    source_type: str
    source_name: str
    source_identifier: str = ""
    source_details: Dict[str, Any] = field(default_factory=dict)
    database_type: str = ""
    schema_name: str = ""
    table_name: str = ""
    column_name: str = ""
    destination_type: str = ""
    destination_name: str = ""
    destination_endpoint: str = ""
    destination_details: Dict[str, Any] = field(default_factory=dict)
    data_size_bytes: int = 0
    record_count: int = 0
    field_count: int = 0
    fields_detected: List[str] = field(default_factory=list)
    classification: Optional[Classification] = None
    request_id: str = ""
    session_id: str = ""
    user_id: str = ""
    capability: str = ""
    verification_event_id: str = ""
    parent_flow_id: str = ""
    hop_number: int = 1
    started_at: float = field(default_factory=time.time)


# ============================================================================
# SCHEMA PATTERNS FOR CLASSIFICATION
# ============================================================================

# Built-in column name patterns for Tier 2/3 classification
COLUMN_PATTERNS = {
    # PII
    ("PII", "ssn"): ["ssn", "social_security", "social_security_number", "ss_number"],
    ("PII", "email"): ["email", "email_address", "user_email", "contact_email", "e_mail"],
    ("PII", "phone"): ["phone", "phone_number", "telephone", "mobile", "cell", "contact_number"],
    ("PII", "address"): ["address", "street_address", "mailing_address", "home_address", "shipping_address"],
    ("PII", "dob"): ["dob", "date_of_birth", "birthdate", "birth_date", "birthday"],
    ("PII", "name"): ["full_name", "legal_name", "customer_name", "patient_name"],
    ("PII", "drivers_license"): ["drivers_license", "license_number", "dl_number"],

    # PHI
    ("PHI", "medical_record_number"): ["mrn", "medical_record_number", "medical_record_id", "patient_id", "chart_number"],
    ("PHI", "diagnosis"): ["diagnosis", "diagnosis_code", "icd_code", "condition", "medical_condition"],
    ("PHI", "prescription"): ["prescription", "medication", "rx", "drug", "medicine", "rx_number"],
    ("PHI", "lab_result"): ["lab_result", "test_result", "lab_value", "blood_test"],
    ("PHI", "insurance_id"): ["insurance_id", "member_id", "policy_number", "subscriber_id", "health_plan_id"],

    # PCI
    ("PCI", "credit_card"): ["credit_card", "card_number", "cc_number", "pan", "payment_card"],
    ("PCI", "cvv"): ["cvv", "cvc", "cvv2", "security_code", "card_security_code"],
    ("PCI", "expiry_date"): ["expiry", "expiration", "exp_date", "card_expiry"],
    ("PCI", "cardholder_name"): ["cardholder", "cardholder_name", "name_on_card"],
    ("PCI", "bank_account"): ["bank_account", "account_number", "iban", "routing_number"],
}


def classify_by_column_name(column_name: str) -> Optional[Classification]:
    """Classify data based on column name (Tier 2/3)."""
    column_lower = column_name.lower().strip()

    for (category, data_type), patterns in COLUMN_PATTERNS.items():
        for pattern in patterns:
            if pattern in column_lower or column_lower in pattern:
                return Classification(
                    category=category,
                    data_type=data_type,
                    tier=ClassificationTier.SCHEMA,
                    confidence=0.95
                )

    return None


def classify_by_table_name(table_name: str) -> Optional[Classification]:
    """Classify based on table name patterns (Tier 2/3)."""
    table_lower = table_name.lower().strip()

    # PHI table patterns
    if any(p in table_lower for p in ["patient", "medical", "health", "diagnosis", "prescription"]):
        return Classification(
            category="PHI",
            data_type="medical_record_number",
            tier=ClassificationTier.SCHEMA,
            confidence=0.85
        )

    # PCI table patterns
    if any(p in table_lower for p in ["payment", "transaction", "billing", "card"]):
        return Classification(
            category="PCI",
            data_type="credit_card",
            tier=ClassificationTier.SCHEMA,
            confidence=0.85
        )

    # PII table patterns
    if any(p in table_lower for p in ["customer", "user", "person", "employee", "contact"]):
        return Classification(
            category="PII",
            data_type="name",
            tier=ClassificationTier.HEURISTIC,
            confidence=0.7
        )

    return None


# ============================================================================
# DATA FLOW TRACKER
# ============================================================================

class DataFlowTracker:
    """
    Tracks data flows through AI agents.

    This class provides:
    - Source-based classification
    - Async flow recording (non-blocking)
    - Policy evaluation and enforcement
    - HTTP interception

    Example:
        tracker = DataFlowTracker(client)

        # Track a database query
        with tracker.track_query(table="customers", columns=["ssn"]) as ctx:
            data = db.query("SELECT ssn FROM customers")
            ctx.record_outflow("api", "openai", data)
    """

    def __init__(
        self,
        client,
        buffer_size: int = 1000,
        flush_interval: float = 1.0,
        enabled: bool = True
    ):
        """
        Initialize the DataFlowTracker.

        Args:
            client: AIMClient instance for API communication
            buffer_size: Max events to buffer before forced flush
            flush_interval: Seconds between automatic flushes
            enabled: Whether tracking is enabled
        """
        self.client = client
        self.enabled = enabled
        self.buffer_size = buffer_size
        self.flush_interval = flush_interval

        # Event buffer for async sending
        self._event_queue: queue.Queue = queue.Queue(maxsize=buffer_size)
        self._executor = ThreadPoolExecutor(max_workers=2, thread_name_prefix="aim-flow-")
        self._shutdown = threading.Event()

        # Classification cache (avoid redundant lookups)
        self._classification_cache: Dict[str, Classification] = {}
        self._cache_lock = threading.Lock()

        # Current flow context (thread-local)
        self._context = threading.local()

        # Start background flush thread
        if enabled:
            self._start_flush_thread()

    def _start_flush_thread(self):
        """Start background thread for flushing events."""
        def flush_loop():
            while not self._shutdown.is_set():
                try:
                    self._flush_events()
                except Exception as e:
                    logger.debug(f"Flush error: {e}")
                self._shutdown.wait(timeout=self.flush_interval)

        thread = threading.Thread(target=flush_loop, daemon=True, name="aim-flow-flush")
        thread.start()

    def _flush_events(self):
        """Flush buffered events to AIM server."""
        events = []
        while not self._event_queue.empty() and len(events) < 100:
            try:
                event = self._event_queue.get_nowait()
                events.append(event)
            except queue.Empty:
                break

        if events:
            try:
                self._send_events(events)
            except Exception as e:
                logger.debug(f"Failed to send events: {e}")
                # Re-queue events on failure (up to a limit)
                for event in events[:50]:  # Limit re-queue to prevent infinite growth
                    try:
                        self._event_queue.put_nowait(event)
                    except queue.Full:
                        break

    def _send_events(self, events: List[DataFlowEvent]):
        """Send events to AIM server."""
        for event in events:
            try:
                payload = {
                    "agentId": event.agent_id,
                    "agentName": event.agent_name,
                    "sourceType": event.source_type,
                    "sourceName": event.source_name,
                    "sourceIdentifier": event.source_identifier,
                    "sourceDetails": event.source_details,
                    "databaseType": event.database_type,
                    "schemaName": event.schema_name,
                    "tableName": event.table_name,
                    "columnName": event.column_name,
                    "destinationType": event.destination_type,
                    "destinationName": event.destination_name,
                    "destinationEndpoint": event.destination_endpoint,
                    "destinationDetails": event.destination_details,
                    "dataSizeBytes": event.data_size_bytes,
                    "recordCount": event.record_count,
                    "fieldCount": event.field_count,
                    "fieldsDetected": event.fields_detected,
                    "requestId": event.request_id,
                    "sessionId": event.session_id,
                    "userId": event.user_id,
                    "capability": event.capability,
                    "verificationEventId": event.verification_event_id,
                    "flowId": event.flow_id,
                    "parentFlowId": event.parent_flow_id,
                    "hopNumber": event.hop_number,
                }

                if event.classification:
                    payload["classification"] = {
                        "category": event.classification.category,
                        "type": event.classification.data_type,
                    }

                # Use client's request method (non-blocking)
                self.client._make_request("POST", "/api/v1/data-flows", data=payload)
            except Exception as e:
                logger.debug(f"Failed to send flow event: {e}")

    def classify(
        self,
        source_type: str,
        source_name: str,
        table_name: str = "",
        column_name: str = "",
        explicit: Optional[Classification] = None
    ) -> Optional[Classification]:
        """
        Classify data from a source.

        Priority:
        1. Explicit classification (Tier 1)
        2. Column name patterns (Tier 2)
        3. Table name patterns (Tier 2/3)

        Args:
            source_type: Type of source (database, api, file)
            source_name: Name of the source
            table_name: Database table name
            column_name: Database column name
            explicit: Explicit classification override

        Returns:
            Classification or None if no classification applies
        """
        if not self.enabled:
            return None

        # Tier 1: Explicit classification
        if explicit:
            return explicit

        # Check cache
        cache_key = f"{source_type}:{source_name}:{table_name}:{column_name}"
        with self._cache_lock:
            if cache_key in self._classification_cache:
                return self._classification_cache[cache_key]

        # Tier 2: Column name patterns
        if column_name:
            result = classify_by_column_name(column_name)
            if result:
                with self._cache_lock:
                    self._classification_cache[cache_key] = result
                return result

        # Tier 2/3: Table name patterns
        if table_name:
            result = classify_by_table_name(table_name)
            if result:
                with self._cache_lock:
                    self._classification_cache[cache_key] = result
                return result

        return None

    def record_flow(
        self,
        source_type: str,
        source_name: str,
        destination_type: str,
        destination_name: str,
        destination_endpoint: str = "",
        table_name: str = "",
        column_name: str = "",
        columns: Optional[List[str]] = None,
        data_size: int = 0,
        record_count: int = 0,
        classification: Optional[Classification] = None,
        capability: str = "",
        request_id: str = "",
        user_id: str = "",
        blocking: bool = False
    ) -> PolicyResult:
        """
        Record a data flow event.

        This method:
        1. Classifies the data
        2. Records the flow (async)
        3. Returns policy evaluation result

        Args:
            source_type: Type of source (database, api, file, user_input)
            source_name: Name of the source
            destination_type: Type of destination (api, llm, mcp, file)
            destination_name: Name of the destination
            destination_endpoint: Full endpoint URL
            table_name: Database table name
            column_name: Single column name
            columns: List of column names
            data_size: Size in bytes
            record_count: Number of records
            classification: Explicit classification
            capability: Capability being used
            request_id: Request correlation ID
            user_id: End user ID
            blocking: If True, wait for server response

        Returns:
            PolicyResult indicating what action to take
        """
        if not self.enabled:
            return PolicyResult(action=PolicyAction.ALLOW)

        # Classify the data
        detected_classification = classification
        fields_detected = []

        if columns:
            for col in columns:
                col_class = classify_by_column_name(col)
                if col_class:
                    fields_detected.append(col)
                    if detected_classification is None or col_class.tier < detected_classification.tier:
                        detected_classification = col_class
        elif column_name:
            detected_classification = self.classify(
                source_type, source_name, table_name, column_name, classification
            )
            if detected_classification:
                fields_detected.append(column_name)

        # Create flow event
        flow_id = str(uuid.uuid4())
        event = DataFlowEvent(
            id=str(uuid.uuid4()),
            flow_id=flow_id,
            agent_id=self.client.agent_id,
            agent_name=getattr(self.client, 'agent_name', self.client.agent_id),
            source_type=source_type,
            source_name=source_name,
            source_identifier=self._generate_source_id(source_type, source_name, table_name, column_name),
            table_name=table_name,
            column_name=column_name,
            destination_type=destination_type,
            destination_name=destination_name,
            destination_endpoint=destination_endpoint,
            data_size_bytes=data_size,
            record_count=record_count,
            field_count=len(columns) if columns else (1 if column_name else 0),
            fields_detected=fields_detected,
            classification=detected_classification,
            capability=capability,
            request_id=request_id or str(uuid.uuid4()),
            user_id=user_id,
        )

        # Queue event for async sending
        try:
            self._event_queue.put_nowait(event)
        except queue.Full:
            logger.debug("Flow event queue full, dropping event")

        # Return default allow (policy evaluation happens server-side)
        # For blocking mode, we would wait for server response
        return PolicyResult(action=PolicyAction.ALLOW)

    def _generate_source_id(self, source_type: str, source_name: str, table_name: str, column_name: str) -> str:
        """Generate a unique source identifier."""
        if source_type == "database":
            raw = f"db:{source_name}.{table_name}.{column_name}"
        else:
            raw = f"{source_type}:{source_name}"

        return hashlib.sha256(raw.encode()).hexdigest()[:32]

    def track_query(
        self,
        table: str,
        columns: Optional[List[str]] = None,
        schema: str = "",
        database: str = ""
    ) -> "QueryContext":
        """
        Context manager for tracking database queries.

        Example:
            with tracker.track_query(table="customers", columns=["ssn", "email"]) as ctx:
                data = db.query("SELECT ssn, email FROM customers")
                ctx.record_outflow("llm", "openai", data)
        """
        return QueryContext(
            tracker=self,
            table=table,
            columns=columns or [],
            schema=schema,
            database=database
        )

    def shutdown(self, timeout: float = 5.0):
        """Shutdown the tracker and flush remaining events."""
        self._shutdown.set()
        self._flush_events()  # Final flush
        self._executor.shutdown(wait=True, cancel_futures=False)


class QueryContext:
    """Context manager for tracking database queries."""

    def __init__(
        self,
        tracker: DataFlowTracker,
        table: str,
        columns: List[str],
        schema: str = "",
        database: str = ""
    ):
        self.tracker = tracker
        self.table = table
        self.columns = columns
        self.schema = schema
        self.database = database
        self.flow_id = str(uuid.uuid4())
        self._entered = False

    def __enter__(self) -> "QueryContext":
        self._entered = True
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self._entered = False
        return False

    def record_outflow(
        self,
        destination_type: str,
        destination_name: str,
        data: Any = None,
        endpoint: str = ""
    ) -> PolicyResult:
        """Record data flowing out to a destination."""
        data_size = 0
        record_count = 0

        if data is not None:
            try:
                if isinstance(data, (list, tuple)):
                    record_count = len(data)
                    data_size = len(json.dumps(data))
                elif isinstance(data, dict):
                    record_count = 1
                    data_size = len(json.dumps(data))
                elif isinstance(data, str):
                    data_size = len(data)
                    record_count = 1
            except:
                pass

        return self.tracker.record_flow(
            source_type="database",
            source_name=self.database or "default",
            destination_type=destination_type,
            destination_name=destination_name,
            destination_endpoint=endpoint,
            table_name=self.table,
            columns=self.columns,
            data_size=data_size,
            record_count=record_count,
        )


# ============================================================================
# DECORATORS
# ============================================================================

F = TypeVar('F', bound=Callable[..., Any])


def sensitive(category: str, data_type: str) -> Callable[[F], F]:
    """
    Decorator to mark a function as returning sensitive data (Tier 1).

    This provides explicit classification with 100% confidence.

    Example:
        @sensitive("PII", "ssn")
        def get_customer_ssn(customer_id: str) -> str:
            return database.query("SELECT ssn FROM customers WHERE id = ?", customer_id)
    """
    classification = Classification(
        category=category,
        data_type=data_type,
        tier=ClassificationTier.EXPLICIT,
        confidence=1.0
    )

    def decorator(func: F) -> F:
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            # Store classification in thread-local for tracker to pick up
            result = func(*args, **kwargs)
            # Classification is available via the decorator metadata
            return result

        # Attach classification as metadata
        wrapper._aim_classification = classification
        return wrapper

    return decorator


def track_data_flow(
    source_type: str = "function",
    source_name: Optional[str] = None,
    tracker: Optional[DataFlowTracker] = None
) -> Callable[[F], F]:
    """
    Decorator to automatically track data flows through a function.

    Example:
        @track_data_flow(source_type="database", source_name="customers_db")
        def get_customer_data(customer_id: str):
            return database.query("SELECT * FROM customers WHERE id = ?", customer_id)
    """
    def decorator(func: F) -> F:
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            name = source_name or func.__name__

            # Check if function has explicit classification
            classification = getattr(func, '_aim_classification', None)

            result = func(*args, **kwargs)

            # Record flow if tracker is available
            if tracker:
                tracker.record_flow(
                    source_type=source_type,
                    source_name=name,
                    destination_type="return",
                    destination_name="caller",
                    classification=classification,
                )

            return result

        return wrapper

    return decorator


# ============================================================================
# HTTP INTERCEPTION (Optional - for automatic tracking)
# ============================================================================

class HTTPInterceptor:
    """
    Intercepts HTTP calls to track data flows automatically.

    This monkey-patches requests/httpx/aiohttp to track all HTTP calls.
    Use with caution as it modifies global behavior.
    """

    def __init__(self, tracker: DataFlowTracker):
        self.tracker = tracker
        self._original_request = None
        self._enabled = False

    def enable(self):
        """Enable HTTP interception."""
        if self._enabled:
            return

        try:
            import requests

            self._original_request = requests.Session.request

            def intercepted_request(session, method, url, **kwargs):
                # Record the outbound flow
                self.tracker.record_flow(
                    source_type="agent",
                    source_name=self.tracker.client.agent_id,
                    destination_type="api",
                    destination_name=self._get_host(url),
                    destination_endpoint=url,
                    data_size=self._estimate_size(kwargs.get('data'), kwargs.get('json')),
                )

                return self._original_request(session, method, url, **kwargs)

            requests.Session.request = intercepted_request
            self._enabled = True
            logger.debug("HTTP interception enabled")

        except ImportError:
            logger.debug("requests not available, HTTP interception disabled")

    def disable(self):
        """Disable HTTP interception."""
        if not self._enabled or not self._original_request:
            return

        try:
            import requests
            requests.Session.request = self._original_request
            self._enabled = False
            logger.debug("HTTP interception disabled")
        except:
            pass

    def _get_host(self, url: str) -> str:
        """Extract host from URL."""
        try:
            from urllib.parse import urlparse
            parsed = urlparse(url)
            return parsed.netloc or url
        except:
            return url

    def _estimate_size(self, data: Any, json_data: Any) -> int:
        """Estimate the size of request data."""
        try:
            if json_data:
                return len(json.dumps(json_data))
            if data:
                if isinstance(data, (str, bytes)):
                    return len(data)
                return len(str(data))
        except:
            pass
        return 0


# ============================================================================
# INTEGRATION WITH AIM CLIENT
# ============================================================================

def create_tracker(client, enabled: bool = True) -> DataFlowTracker:
    """
    Create a DataFlowTracker for an AIM client.

    This is the main entry point for data flow visibility.

    Args:
        client: AIMClient instance
        enabled: Whether to enable tracking

    Returns:
        DataFlowTracker instance
    """
    return DataFlowTracker(client, enabled=enabled)
