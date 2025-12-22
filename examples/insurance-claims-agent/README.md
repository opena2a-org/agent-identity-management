# Insurance Claims Processing Agent - Data Flow Visibility Demo

This demo showcases AIM's **Data Flow Visibility** feature in an enterprise insurance claims processing context.

## What This Demo Shows

1. **Source-Based Classification (Zero False Positives)**
   - **Tier 1 (Explicit)**: `@sensitive` decorator - 100% confidence
   - **Tier 2 (Schema)**: Column name inference - 95% confidence
   - **Tier 3 (Heuristic)**: Naming patterns - 80% confidence
   - **Tier 4 (Content)**: Regex fallback - 60-70% confidence

2. **Real-Time Data Flow Tracking**
   - Track PII, PHI, PCI as it flows through agents
   - Async, non-blocking recording
   - Zero performance impact on agents

3. **Policy Enforcement**
   - Block sensitive data to unauthorized destinations
   - Alert on potential violations
   - Audit trail for compliance

4. **Enterprise Insurance Scenarios**
   - Auto claims processing
   - Health claims with medical records (PHI)
   - Payment processing (PCI)
   - LLM summarization with data tracking

## Quick Start

```bash
# Install SDK
pip install aim-sdk

# Run demo
python claims_agent.py
```

## Demo Scenarios

| Scenario | Data Type | Classification Tier |
|----------|-----------|---------------------|
| SSN Retrieval | PII.ssn | Tier 1 (Explicit) |
| Medical Diagnosis | PHI.diagnosis | Tier 1 (Explicit) |
| Policyholder Query | PII.* | Tier 2 (Schema) |
| Medical Records Query | PHI.* | Tier 2 (Schema) |
| LLM Summarization | Mixed | Flow Tracking |
| Analytics Pipeline | PII.address (derived) | Flow Tracking |
| Payment Processing | PCI.account_number | Flow Tracking |
| External API | PII.* | Violation Demo |

## Key Code Patterns

### Tier 1: Explicit Classification

```python
@sensitive(category="pii", data_type="ssn")
def get_policyholder_ssn(self, policyholder_id: str) -> str:
    """100% confidence - developer explicitly declares sensitivity"""
    return self.db.query_ssn(policyholder_id)
```

### Tier 2: Schema-Based Classification

```python
def query_policyholder(self, id: str) -> dict:
    """95% confidence - column names auto-classified"""
    with QueryContext(
        tracker,
        table="policyholders",
        columns=["email", "ssn", "phone"],  # Auto-classified
        destination="claims-processor"
    ):
        return self.db.query(id)
```

### Flow Tracking

```python
@track_data_flow(destination="openai-api", operation="summarization")
def summarize_claim(self, claim: Claim) -> str:
    """Tracks data flow to LLM automatically"""
    return self.llm.summarize(claim.description)
```

## Dashboard Integration

After running the demo, check these dashboard views:

- **Data Flows**: `{DASHBOARD_URL}/dashboard/data-flows`
- **Data Sources**: `{DASHBOARD_URL}/dashboard/data-sources`
- **Violations**: `{DASHBOARD_URL}/dashboard/data-flow-violations`
- **Statistics**: `{DASHBOARD_URL}/dashboard/data-flows/statistics`

## Why Source-Based Classification?

Traditional DLP uses regex pattern matching at data exit points:
- High false positive rate (10-30%)
- Misses obfuscated data
- No context about data origin

AIM's source-based approach:
- **Zero false positives** for Tier 1/2
- Data "knows" its sensitivity from origin
- Classification travels with data
- Full audit trail

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Claims Processing Agent                    │
├─────────────────────────────────────────────────────────────┤
│  @sensitive          QueryContext        HTTPInterceptor    │
│  (Tier 1)            (Tier 2)           (Tier 3-4)         │
│      ↓                   ↓                   ↓              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              DataFlowTracker                        │   │
│  │  - Async event buffering                            │   │
│  │  - Non-blocking recording                           │   │
│  │  - Policy evaluation cache                          │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                  │
│                    AIM Backend                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  DataFlowService                                    │   │
│  │  - Classification engine                            │   │
│  │  - Policy cache (5-min TTL)                         │   │
│  │  - Violation detection                              │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## For Enterprise Engineering Teams

This demo is designed to showcase how AIM can help enterprises:

1. **Compliance**: Track PII/PHI/PCI flows for HIPAA, PCI-DSS, CCPA
2. **Risk Reduction**: Block sensitive data from reaching unauthorized destinations
3. **Audit Trail**: Complete visibility into data movement
4. **Zero Overhead**: Async tracking doesn't slow down agents
5. **Zero Configuration**: Schema inference handles most classification automatically
