#!/usr/bin/env python3
"""
Insurance Claims Processing Agent - Data Flow Visibility Demo

This demo showcases AIM's Data Flow Visibility feature in an enterprise
insurance claims processing context. It demonstrates:

1. Source-based classification (Tier 1-4)
2. Real-time data flow tracking
3. Policy enforcement (PII, PHI, PCI)
4. Violation detection and alerts

For enterprise engineering team demonstration.

SETUP:
  1. pip install aim-sdk
  2. Set AIM_URL environment variable or use downloaded SDK credentials
  3. python claims_agent.py

Watch the AIM dashboard update in real-time as claims are processed!
"""

import sys
import os
import time
import random
import json
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any
from dataclasses import dataclass
from enum import Enum

# Add SDK to path if running locally
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../sdk/python"))

try:
    from aim_sdk import secure
    from aim_sdk.credentials import load_sdk_credentials
    from aim_sdk.data_flow import (
        DataFlowTracker,
        classify_column,
        sensitive,
        track_data_flow,
        QueryContext,
        HTTPInterceptor,
        ClassificationTier,
        ClassificationCategory,
    )
except ImportError:
    print("""
================================================================================
ERROR: Could not import aim_sdk

Please install the SDK:
  pip install aim-sdk

Or download from your AIM dashboard:
  Settings -> SDK Download
================================================================================
""")
    sys.exit(1)


# Configuration
sdk_creds = load_sdk_credentials()
AIM_URL = os.getenv("AIM_URL", sdk_creds.get("aimUrl", "http://localhost:8080") if sdk_creds else "http://localhost:8080")

if "api.aim.opena2a.org" in AIM_URL:
    DASHBOARD_URL = "https://aim.opena2a.org"
elif ":8080" in AIM_URL:
    DASHBOARD_URL = AIM_URL.replace(":8080", ":3000")
else:
    DASHBOARD_URL = AIM_URL.replace("/api", "").rstrip("/")


# =============================================================================
# INSURANCE DOMAIN MODELS
# =============================================================================

class ClaimType(Enum):
    AUTO = "auto"
    HOME = "home"
    HEALTH = "health"
    LIFE = "life"


class ClaimStatus(Enum):
    SUBMITTED = "submitted"
    UNDER_REVIEW = "under_review"
    APPROVED = "approved"
    DENIED = "denied"
    PAID = "paid"


@dataclass
class PolicyHolder:
    """Policyholder with sensitive PII"""
    id: str
    name: str
    ssn: str  # PII.ssn
    email: str  # PII.email
    phone: str  # PII.phone
    address: str  # PII.address
    date_of_birth: str  # PII.dob
    policy_number: str


@dataclass
class MedicalRecord:
    """Medical records for health claims - PHI"""
    patient_id: str
    diagnosis_code: str  # PHI.diagnosis
    treatment_notes: str  # PHI.medical_notes
    provider_npi: str  # PHI.provider_id
    prescription_list: List[str]  # PHI.medication
    lab_results: Dict[str, str]  # PHI.lab_results
    medical_record_number: str  # PHI.mrn


@dataclass
class PaymentInfo:
    """Payment details for claim payouts - PCI"""
    account_holder: str
    bank_routing_number: str  # PCI.routing_number
    bank_account_number: str  # PCI.account_number
    credit_card_number: Optional[str] = None  # PCI.credit_card
    cvv: Optional[str] = None  # PCI.cvv


@dataclass
class Claim:
    """Insurance claim with mixed sensitive data"""
    claim_id: str
    claim_type: ClaimType
    policyholder: PolicyHolder
    incident_date: str
    description: str
    amount_claimed: float
    status: ClaimStatus
    medical_records: Optional[MedicalRecord] = None
    payment_info: Optional[PaymentInfo] = None


# =============================================================================
# SIMULATED DATABASE
# =============================================================================

def generate_mock_data():
    """Generate realistic mock insurance data"""
    policyholders = [
        PolicyHolder(
            id="PH-001",
            name="John Smith",
            ssn="123-45-6789",
            email="john.smith@example.com",
            phone="555-123-4567",
            address="123 Main St, Chicago, IL 60601",
            date_of_birth="1985-03-15",
            policy_number="POL-AUTO-2024-001"
        ),
        PolicyHolder(
            id="PH-002",
            name="Sarah Johnson",
            ssn="987-65-4321",
            email="sarah.j@example.com",
            phone="555-987-6543",
            address="456 Oak Ave, Springfield, IL 62701",
            date_of_birth="1978-11-22",
            policy_number="POL-HEALTH-2024-002"
        ),
        PolicyHolder(
            id="PH-003",
            name="Michael Chen",
            ssn="456-78-9012",
            email="m.chen@example.com",
            phone="555-456-7890",
            address="789 Pine Rd, Naperville, IL 60540",
            date_of_birth="1990-07-08",
            policy_number="POL-HOME-2024-003"
        ),
    ]

    medical_records = [
        MedicalRecord(
            patient_id="PH-002",
            diagnosis_code="J06.9",  # Acute upper respiratory infection
            treatment_notes="Patient presents with fever and sore throat. Prescribed antibiotics.",
            provider_npi="1234567890",
            prescription_list=["Amoxicillin 500mg", "Ibuprofen 400mg"],
            lab_results={"WBC": "11.2", "CRP": "8.5"},
            medical_record_number="MRN-2024-00234"
        ),
    ]

    payment_info = [
        PaymentInfo(
            account_holder="John Smith",
            bank_routing_number="071000013",
            bank_account_number="1234567890123",
            credit_card_number="4111111111111111",
            cvv="123"
        ),
    ]

    claims = [
        Claim(
            claim_id="CLM-2024-00001",
            claim_type=ClaimType.AUTO,
            policyholder=policyholders[0],
            incident_date="2024-11-15",
            description="Rear-end collision at intersection",
            amount_claimed=4500.00,
            status=ClaimStatus.UNDER_REVIEW,
            payment_info=payment_info[0]
        ),
        Claim(
            claim_id="CLM-2024-00002",
            claim_type=ClaimType.HEALTH,
            policyholder=policyholders[1],
            incident_date="2024-11-10",
            description="Emergency room visit for respiratory infection",
            amount_claimed=2300.00,
            status=ClaimStatus.SUBMITTED,
            medical_records=medical_records[0]
        ),
        Claim(
            claim_id="CLM-2024-00003",
            claim_type=ClaimType.HOME,
            policyholder=policyholders[2],
            incident_date="2024-11-01",
            description="Water damage from burst pipe",
            amount_claimed=12000.00,
            status=ClaimStatus.APPROVED,
            payment_info=payment_info[0]
        ),
    ]

    return policyholders, medical_records, claims


# =============================================================================
# CLAIMS PROCESSING AGENT
# =============================================================================

class ClaimsProcessingAgent:
    """
    AI Agent for processing insurance claims.

    Demonstrates Data Flow Visibility by:
    1. Tracking sensitive data as it flows through processing
    2. Enforcing policies on data destinations
    3. Recording classification confidence levels
    4. Detecting and alerting on policy violations
    """

    def __init__(self, agent, tracker: DataFlowTracker):
        self.agent = agent
        self.tracker = tracker
        self.http_interceptor = HTTPInterceptor(tracker)
        self.policyholders, self.medical_records, self.claims = generate_mock_data()

        # Configure known destinations
        self._configure_destinations()

    def _configure_destinations(self):
        """Configure known external destinations for tracking"""
        # LLM providers
        self.http_interceptor.add_sensitive_destination(
            pattern=r"api\.openai\.com",
            name="OpenAI API"
        )
        self.http_interceptor.add_sensitive_destination(
            pattern=r"api\.anthropic\.com",
            name="Anthropic API"
        )
        # Analytics services
        self.http_interceptor.add_sensitive_destination(
            pattern=r"analytics\.internal\.com",
            name="Internal Analytics"
        )
        # External integrations
        self.http_interceptor.add_sensitive_destination(
            pattern=r"api\.ehicle-history\.com",
            name="Vehicle History API"
        )

    # =========================================================================
    # TIER 1: EXPLICIT CLASSIFICATION (100% CONFIDENCE)
    # =========================================================================

    @sensitive(category="pii", data_type="ssn")
    def get_policyholder_ssn(self, policyholder_id: str) -> str:
        """
        Retrieve policyholder SSN - TIER 1 EXPLICIT CLASSIFICATION

        The @sensitive decorator explicitly marks this data as PII.ssn
        with 100% confidence. This is the gold standard for classification.
        """
        for ph in self.policyholders:
            if ph.id == policyholder_id:
                return ph.ssn
        return None

    @sensitive(category="phi", data_type="diagnosis")
    def get_medical_diagnosis(self, patient_id: str) -> str:
        """
        Retrieve patient diagnosis - TIER 1 EXPLICIT (PHI)
        """
        for record in self.medical_records:
            if record.patient_id == patient_id:
                return record.diagnosis_code
        return None

    @sensitive(category="pci", data_type="account_number")
    def get_payment_account(self, policyholder_id: str) -> str:
        """
        Retrieve bank account number - TIER 1 EXPLICIT (PCI)
        """
        for claim in self.claims:
            if claim.policyholder.id == policyholder_id and claim.payment_info:
                return claim.payment_info.bank_account_number
        return None

    # =========================================================================
    # TIER 2: SCHEMA-BASED CLASSIFICATION (95% CONFIDENCE)
    # =========================================================================

    def query_policyholder_data(self, policyholder_id: str) -> Dict[str, Any]:
        """
        Query policyholder from database - TIER 2 SCHEMA CLASSIFICATION

        Uses QueryContext to track database access. Column names like
        'email', 'ssn', 'phone' are automatically classified based on
        schema patterns with 95% confidence.
        """
        with QueryContext(
            self.tracker,
            table="policyholders",
            columns=["id", "name", "ssn", "email", "phone", "address", "date_of_birth"],
            connection_string="postgres://claims-db.internal/insurance",
            destination="claims-processor"
        ) as ctx:
            # Simulate database query
            for ph in self.policyholders:
                if ph.id == policyholder_id:
                    return {
                        "id": ph.id,
                        "name": ph.name,
                        "ssn": ph.ssn,
                        "email": ph.email,
                        "phone": ph.phone,
                        "address": ph.address,
                        "date_of_birth": ph.date_of_birth,
                        "_classification_tier": 2,
                        "_confidence": 0.95
                    }
        return None

    def query_medical_records(self, patient_id: str) -> Dict[str, Any]:
        """
        Query medical records - TIER 2 SCHEMA CLASSIFICATION (PHI)

        Column names like 'diagnosis_code', 'medical_record_number',
        'prescription_list' are auto-classified as PHI.
        """
        with QueryContext(
            self.tracker,
            table="medical_records",
            columns=["patient_id", "diagnosis_code", "treatment_notes", "prescription_list", "medical_record_number"],
            connection_string="postgres://hipaa-db.internal/medical",
            destination="claims-processor"
        ) as ctx:
            for record in self.medical_records:
                if record.patient_id == patient_id:
                    return {
                        "patient_id": record.patient_id,
                        "diagnosis_code": record.diagnosis_code,
                        "treatment_notes": record.treatment_notes,
                        "prescription_list": record.prescription_list,
                        "medical_record_number": record.medical_record_number,
                        "_classification_tier": 2,
                        "_confidence": 0.95
                    }
        return None

    # =========================================================================
    # DATA FLOW TRACKING DEMONSTRATIONS
    # =========================================================================

    @track_data_flow(destination="openai-api", operation="claim_summarization")
    def summarize_claim_with_llm(self, claim: Claim) -> str:
        """
        Summarize a claim using an LLM - TRACKS DATA FLOW

        The @track_data_flow decorator records that claim data is being
        sent to an external LLM. If the claim contains PII/PHI/PCI,
        this may trigger a policy violation.
        """
        # In reality, this would call an LLM API
        # The decorator tracks the data flow automatically

        summary = f"""
        Claim {claim.claim_id} Summary:
        - Type: {claim.claim_type.value}
        - Amount: ${claim.amount_claimed:,.2f}
        - Status: {claim.status.value}
        - Description: {claim.description}
        """

        # Manually record the flow with detected data types
        self.tracker.record_flow(
            source_type="database",
            source_name="claims.description",
            destination="openai-api",
            data_types=["pii.name", "pii.address"] if "address" in claim.description.lower() else [],
            metadata={
                "operation": "claim_summarization",
                "claim_id": claim.claim_id,
                "model": "gpt-4"
            }
        )

        return summary

    def send_claim_to_analytics(self, claim: Claim) -> Dict[str, Any]:
        """
        Send claim data to analytics service - POTENTIAL VIOLATION

        This demonstrates a data flow that may violate policies if
        PII is included in the analytics payload.
        """
        # Build analytics payload
        payload = {
            "claim_id": claim.claim_id,
            "claim_type": claim.claim_type.value,
            "amount": claim.amount_claimed,
            "region": claim.policyholder.address.split(",")[-1].strip() if "," in claim.policyholder.address else "Unknown",
            "timestamp": datetime.now().isoformat()
        }

        # Track this flow - includes region which is derived from address (PII)
        self.tracker.record_flow(
            source_type="derived",
            source_name="claims.policyholder.address",
            destination="analytics-service",
            data_types=["pii.address"],  # Derived from address
            metadata={
                "operation": "analytics_aggregation",
                "claim_id": claim.claim_id
            }
        )

        return {"status": "sent", "payload_size": len(json.dumps(payload))}

    def process_payment(self, claim: Claim) -> Dict[str, Any]:
        """
        Process claim payment - CRITICAL DATA FLOW (PCI)

        This demonstrates tracking of payment data (PCI) flowing
        to a payment processor. Strict policies should apply.
        """
        if not claim.payment_info:
            return {"status": "error", "message": "No payment info"}

        # Record PCI data flow to payment processor
        self.tracker.record_flow(
            source_type="database",
            source_name="payment_info.bank_account_number",
            destination="payment-processor",
            data_types=["pci.account_number", "pci.routing_number"],
            metadata={
                "operation": "ach_transfer",
                "claim_id": claim.claim_id,
                "amount": claim.amount_claimed
            }
        )

        # Simulate payment processing
        return {
            "status": "processed",
            "confirmation": f"PAY-{random.randint(100000, 999999)}",
            "amount": claim.amount_claimed
        }

    def send_to_external_api(self, claim: Claim, api_name: str = "external-vendor") -> Dict[str, Any]:
        """
        Send claim data to external API - VIOLATION SCENARIO

        This demonstrates sending data to an unauthorized external
        destination, which should trigger a policy violation.
        """
        # This would be blocked/alerted by a well-configured policy
        self.tracker.record_flow(
            source_type="database",
            source_name="claims.*",
            destination=api_name,
            data_types=["pii.ssn", "pii.name", "pii.email"],
            metadata={
                "operation": "external_sync",
                "claim_id": claim.claim_id,
                "warning": "POTENTIAL VIOLATION - External destination"
            }
        )

        return {"status": "sent", "destination": api_name}

    # =========================================================================
    # BATCH OPERATIONS
    # =========================================================================

    def process_daily_claims_batch(self) -> Dict[str, Any]:
        """
        Process a batch of claims - MULTI-FLOW TRACKING

        Demonstrates tracking multiple data flows in a batch operation.
        """
        results = {
            "processed": 0,
            "violations_detected": 0,
            "flows_recorded": 0
        }

        for claim in self.claims:
            # Query claim data (creates flow)
            self.query_policyholder_data(claim.policyholder.id)
            results["flows_recorded"] += 1

            # Summarize with LLM (creates flow)
            self.summarize_claim_with_llm(claim)
            results["flows_recorded"] += 1

            # Send to analytics (creates flow, may violate)
            self.send_claim_to_analytics(claim)
            results["flows_recorded"] += 1

            # Process payment if approved
            if claim.status == ClaimStatus.APPROVED:
                self.process_payment(claim)
                results["flows_recorded"] += 1

            results["processed"] += 1

        return results


# =============================================================================
# DEMO RUNNER
# =============================================================================

def print_box(title: str, content: str, width: int = 78):
    """Print content in a formatted box"""
    print("=" * width)
    print(f"  {title}")
    print("=" * width)
    if content:
        print(content)
    print("=" * width)


def print_result(success: bool, title: str, details: Dict = None, error: str = None):
    """Print action result"""
    icon = "OK" if success else "!!"
    status = "SUCCESS" if success else "BLOCKED"

    print()
    print(f"  [{icon}] {status}: {title}")
    if details:
        for key, value in details.items():
            print(f"      {key}: {value}")
    if error:
        print(f"      Reason: {error}")
    print()


def run_demo():
    """Run the insurance claims agent demo"""

    print(f"""
================================================================================
       INSURANCE CLAIMS AGENT - Data Flow Visibility Demo
================================================================================

This demo showcases AIM's Data Flow Visibility feature for enterprise
insurance claims processing.

You'll see:
  1. Source-based classification (Tier 1-4)
  2. Real-time data flow tracking
  3. Policy enforcement for PII, PHI, PCI
  4. Violation detection and alerts

Dashboard: {DASHBOARD_URL}/dashboard
API:       {AIM_URL}

================================================================================
""")

    # Register agent
    print("Registering claims processing agent...")
    try:
        agent = secure(
            "insurance-claims-agent",
            capabilities=[
                "claims:read", "claims:process", "claims:approve",
                "pii:read", "phi:read", "pci:read",
                "llm:call", "analytics:write"
            ]
        )
        print(f"  Agent ID: {agent.agent_id}")
        print(f"  Status: Registered")
    except Exception as e:
        print(f"  ERROR: {e}")
        print("  Make sure AIM backend is running and SDK is configured.")
        sys.exit(1)

    # Initialize tracker
    tracker = DataFlowTracker(agent, enabled=True, flush_interval=0.5)
    claims_agent = ClaimsProcessingAgent(agent, tracker)

    print()
    print("Agent ready. Choose a demo scenario:\n")

    while True:
        print(f"""
--------------------------------------------------------------------------------
                           DEMO SCENARIOS
--------------------------------------------------------------------------------

  CLASSIFICATION TIERS:
    1. Tier 1 Demo - Explicit classification (@sensitive decorator)
    2. Tier 2 Demo - Schema-based classification (QueryContext)

  DATA FLOW TRACKING:
    3. LLM Summarization - Send claim to OpenAI (tracked flow)
    4. Analytics Pipeline - Send to analytics (PII derivation)
    5. Payment Processing - Process ACH payment (PCI flow)

  POLICY ENFORCEMENT:
    6. Violation Demo - Send to unauthorized external API
    7. Batch Processing - Process all claims (multi-flow)

  FULL DEMO:
    8. Run Complete Demo - All scenarios in sequence

  0. Exit

  Dashboard: {DASHBOARD_URL}/dashboard/data-flows
--------------------------------------------------------------------------------
""")

        choice = input("  Enter choice: ").strip()

        if choice == "0":
            print(f"\n  Check your dashboard: {DASHBOARD_URL}/dashboard/data-flows\n")
            tracker.flush()
            break

        elif choice == "1":
            print_box("TIER 1: EXPLICIT CLASSIFICATION", """
The @sensitive decorator marks data with 100% confidence classification.
This is the gold standard - developers explicitly declare data sensitivity.
""")

            print("  Retrieving policyholder SSN (Tier 1 - PII.ssn)...")
            ssn = claims_agent.get_policyholder_ssn("PH-001")
            print(f"  Result: {ssn[:3]}****** (masked)")
            print(f"  Classification: PII.ssn, Tier 1, Confidence 100%")

            print("\n  Retrieving medical diagnosis (Tier 1 - PHI.diagnosis)...")
            diagnosis = claims_agent.get_medical_diagnosis("PH-002")
            print(f"  Result: {diagnosis}")
            print(f"  Classification: PHI.diagnosis, Tier 1, Confidence 100%")

            print("\n  Retrieving bank account (Tier 1 - PCI.account_number)...")
            account = claims_agent.get_payment_account("PH-001")
            print(f"  Result: ****{account[-4:]}")
            print(f"  Classification: PCI.account_number, Tier 1, Confidence 100%")

            print_result(True, "Tier 1 classification complete", {
                "Flows recorded": 3,
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-flows"
            })

        elif choice == "2":
            print_box("TIER 2: SCHEMA-BASED CLASSIFICATION", """
QueryContext automatically classifies data based on column/table names.
Columns like 'email', 'ssn', 'phone' are inferred with 95% confidence.
""")

            print("  Querying policyholder data...")
            data = claims_agent.query_policyholder_data("PH-001")
            if data:
                print(f"  Retrieved columns: {list(data.keys())[:5]}...")
                print(f"  Classification tier: {data.get('_classification_tier', 2)}")
                print(f"  Confidence: {data.get('_confidence', 0.95) * 100}%")

            print("\n  Querying medical records...")
            records = claims_agent.query_medical_records("PH-002")
            if records:
                print(f"  Retrieved columns: {list(records.keys())[:5]}...")
                print(f"  Classification: PHI (inferred from column names)")
                print(f"  Confidence: 95%")

            print_result(True, "Tier 2 classification complete", {
                "Auto-classified columns": "ssn, email, phone, diagnosis_code, medical_record_number",
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-sources"
            })

        elif choice == "3":
            print_box("LLM SUMMARIZATION - Data Flow Tracking", """
When claims data is sent to an LLM for summarization, AIM tracks the flow.
This helps ensure PII/PHI doesn't inadvertently reach external APIs.
""")

            claim = claims_agent.claims[0]  # Auto claim
            print(f"  Processing claim: {claim.claim_id}")
            print(f"  Sending to OpenAI for summarization...")

            summary = claims_agent.summarize_claim_with_llm(claim)
            print(f"  Summary generated (flow tracked)")

            print_result(True, "LLM flow recorded", {
                "Destination": "openai-api",
                "Data types detected": "pii.name, pii.address (if present)",
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-flows"
            })

        elif choice == "4":
            print_box("ANALYTICS PIPELINE - PII Derivation", """
Even derived data can contain PII. When we extract region from address,
the flow is tracked to ensure analytics doesn't leak sensitive info.
""")

            claim = claims_agent.claims[0]
            print(f"  Sending claim {claim.claim_id} to analytics...")

            result = claims_agent.send_claim_to_analytics(claim)
            print(f"  Status: {result['status']}")
            print(f"  Payload size: {result['payload_size']} bytes")

            print_result(True, "Analytics flow recorded", {
                "Source": "claims.policyholder.address",
                "Destination": "analytics-service",
                "Classification": "pii.address (derived)",
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-flows"
            })

        elif choice == "5":
            print_box("PAYMENT PROCESSING - PCI Data Flow", """
Payment processing involves the most sensitive data (PCI).
AIM tracks bank account numbers, routing numbers to payment processors.
""")

            # Find approved claim with payment info
            claim = claims_agent.claims[2]  # Home claim (approved)
            print(f"  Processing payment for claim: {claim.claim_id}")
            print(f"  Amount: ${claim.amount_claimed:,.2f}")

            result = claims_agent.process_payment(claim)
            print(f"  Status: {result['status']}")
            print(f"  Confirmation: {result.get('confirmation', 'N/A')}")

            print_result(True, "PCI flow recorded", {
                "Data types": "pci.account_number, pci.routing_number",
                "Destination": "payment-processor",
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-flows"
            })

        elif choice == "6":
            print_box("VIOLATION DEMO - Unauthorized External API", """
This simulates sending sensitive data to an UNAUTHORIZED destination.
A properly configured policy would block or alert on this flow.
""")

            claim = claims_agent.claims[0]
            print(f"  Attempting to send claim {claim.claim_id} to external-vendor...")
            print(f"  (This should trigger a policy violation)")

            result = claims_agent.send_to_external_api(claim, "unknown-external-api")

            print_result(False, "POTENTIAL VIOLATION DETECTED", {
                "Flow": "claims.* -> unknown-external-api",
                "Data types": "pii.ssn, pii.name, pii.email",
                "Action": "Alert/Block based on policy",
                "Check violations": f"{DASHBOARD_URL}/dashboard/data-flow-violations"
            }, error="Data sent to unauthorized destination")

        elif choice == "7":
            print_box("BATCH PROCESSING - Multi-Flow Tracking", """
Batch processing creates multiple data flows. AIM tracks all of them
for complete visibility into your data pipeline.
""")

            print("  Processing daily claims batch...")
            result = claims_agent.process_daily_claims_batch()

            print(f"\n  Batch Results:")
            print(f"    Claims processed: {result['processed']}")
            print(f"    Flows recorded: {result['flows_recorded']}")

            print_result(True, "Batch processing complete", {
                "Total flows": result['flows_recorded'],
                "Check dashboard": f"{DASHBOARD_URL}/dashboard/data-flows"
            })

        elif choice == "8":
            print_box("COMPLETE DEMO - All Scenarios", """
Running all demo scenarios in sequence...
Watch the dashboard update in real-time!
""")

            scenarios = [
                ("Tier 1 - SSN retrieval", lambda: claims_agent.get_policyholder_ssn("PH-001")),
                ("Tier 1 - Medical diagnosis", lambda: claims_agent.get_medical_diagnosis("PH-002")),
                ("Tier 2 - Policyholder query", lambda: claims_agent.query_policyholder_data("PH-001")),
                ("Tier 2 - Medical records", lambda: claims_agent.query_medical_records("PH-002")),
                ("LLM summarization", lambda: claims_agent.summarize_claim_with_llm(claims_agent.claims[0])),
                ("Analytics pipeline", lambda: claims_agent.send_claim_to_analytics(claims_agent.claims[0])),
                ("Payment processing", lambda: claims_agent.process_payment(claims_agent.claims[2])),
                ("External API (violation)", lambda: claims_agent.send_to_external_api(claims_agent.claims[0], "unknown-api")),
            ]

            for name, fn in scenarios:
                print(f"  Running: {name}...", end=" ")
                try:
                    fn()
                    print("OK")
                except Exception as e:
                    print(f"Error: {e}")
                time.sleep(0.3)

            # Flush all events
            tracker.flush()

            print_result(True, "Complete demo finished", {
                "Scenarios run": len(scenarios),
                "Check flows": f"{DASHBOARD_URL}/dashboard/data-flows",
                "Check violations": f"{DASHBOARD_URL}/dashboard/data-flow-violations",
                "Check statistics": f"{DASHBOARD_URL}/dashboard/data-flows/statistics"
            })

        else:
            print("  Invalid choice. Please try again.")

        # Flush after each action
        tracker.flush()
        input("\n  Press Enter to continue...")


if __name__ == "__main__":
    run_demo()
