# A2A Multi-Agent Collaboration Demo

This demo showcases Agent-to-Agent (A2A) collaboration between a **Research Agent** and an **Analysis Agent**, demonstrating all A2A protocol features across Python and Java SDKs.

## Scenario

The Research Agent discovers and collaborates with the Analysis Agent to:
1. Analyze customer feedback sentiment
2. Detect trends in time-series data
3. Summarize documents

## A2A Features Demonstrated

| Feature | Description | Demo Step |
|---------|-------------|-----------|
| Agent Card Registration | Register agents with skills | Steps 1-2 |
| Intent-Based Discovery | Find agents by natural language intent | Step 3 |
| Trust Score Management | Check A2A and peer trust scores | Step 4 |
| Security Policy Evaluation | Verify requests against policies | Step 5 |
| GDPR Consent Management | Record and check user consent | Step 6 |
| Request Signing | Ed25519 signatures for requests | Step 7 |
| Task Logging | Log and track A2A tasks | Step 7 |
| Skill Attestation | Attest to skill quality | Step 8 |
| Consensus Verification | Check skill consensus status | Step 8 |

## Prerequisites

1. AIM backend running at `http://localhost:8080`, or on the `AIM_BACKEND_PORT` the stack was started with (see [Environment Variables](#environment-variables))
2. SDK built for your language of choice

> **Note**: The SDK handles authentication automatically via bundled credentials. No API key configuration required.

## Quick Start

### Python

```bash
cd python

# Install dependencies
pip install -r requirements.txt

# Run demo
python run_demo.py
```

### Java

```bash
cd java

# Build the SDK first (if not already built)
cd ../../../sdk/java && mvn package -DskipTests && cd ../../examples/a2a-multi-agent-demo/java

# Run demo
mvn exec:java -Dexec.mainClass="org.opena2a.demo.A2AMultiAgentDemo"
```

## Project Structure

```
a2a-multi-agent-demo/
├── README.md
├── shared/
│   ├── agent-cards/
│   │   ├── analysis-agent.json    # Analysis Agent card template
│   │   └── research-agent.json    # Research Agent card template
│   └── sample-data/
│       └── research-data.json     # Sample data for demos
├── python/
│   ├── requirements.txt
│   ├── analysis_agent.py          # Analysis Agent implementation
│   ├── research_agent.py          # Research Agent implementation
│   ├── run_demo.py                # Demo orchestrator
│   └── stack_env.py               # Backend and dashboard URLs, from AIM_BACKEND_PORT and AIM_FRONTEND_PORT
└── java/
    ├── pom.xml
    └── src/main/java/org/opena2a/demo/
        ├── AnalysisAgent.java
        ├── ResearchAgent.java
        └── A2AMultiAgentDemo.java
```

## Demo Flow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Analysis Agent │     │  Research Agent │     │   AIM Platform  │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │ 1. Register           │                       │
         │──────────────────────────────────────────────>│
         │                       │                       │
         │                       │ 2. Register           │
         │                       │──────────────────────>│
         │                       │                       │
         │                       │ 3. Discover by Intent │
         │                       │──────────────────────>│
         │                       │<──────────────────────│
         │                       │                       │
         │                       │ 4. Get Trust Score    │
         │                       │──────────────────────>│
         │                       │                       │
         │                       │ 5. Check Security     │
         │                       │──────────────────────>│
         │                       │                       │
         │                       │ 6. Record Consent     │
         │                       │──────────────────────>│
         │                       │                       │
         │   7. Signed Request   │                       │
         │<──────────────────────│                       │
         │                       │                       │
         │   Return Results      │                       │
         │──────────────────────>│                       │
         │                       │                       │
         │                       │ 8. Attest Skill       │
         │                       │──────────────────────>│
         │                       │                       │
```

## Dashboard Verification

After running the demo, verify results in the AIM Dashboard:

| Tab | What to Check |
|-----|---------------|
| **Overview** | Task stats, trust distribution charts |
| **Agent Cards** | Both agents registered with skills |
| **Consent** | User consent record created |
| **Tasks** | Task with COMPLETED state |
| **Trust** | Peer trust relationship visible |
| **Skills** | Search "sentiment" finds analysis skills |

Access the dashboard at: `http://localhost:3000/dashboard/a2a` (the Python demo prints these links with the port from `AIM_FRONTEND_PORT`)

## Sample Data

The demo uses sample data from `shared/sample-data/research-data.json`:

- **Customer Feedback**: 4 feedback entries for sentiment analysis
- **Time Series Data**: 7 days of sales metrics for trend detection
- **Document Content**: Quarterly report for summarization

## Agent Skills

### Analysis Agent

| Skill | Description |
|-------|-------------|
| `sentiment-analysis` | Analyze text sentiment (positive/negative/neutral) |
| `summarization` | Summarize long documents |
| `trend-detection` | Detect trends in time-series data |

### Research Agent

| Skill | Description |
|-------|-------------|
| `web-research` | Gather information from web sources |
| `document-reading` | Extract content from documents |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AIM_URL` | `http://localhost:$AIM_BACKEND_PORT` (Python), `http://localhost:8080` (Java) | the backend URL (optional); `--aim-url` wins over it |
| `AIM_BACKEND_PORT` | `8080` | the backend, `http://localhost:$AIM_BACKEND_PORT`, unless `AIM_URL` is set (Python) |
| `AIM_FRONTEND_PORT` | `3000` | the dashboard links, `http://localhost:$AIM_FRONTEND_PORT` (Python) |

The Python demo reads `AIM_BACKEND_PORT` and `AIM_FRONTEND_PORT` from the environment first, then from the repository's `.env`, then the default (`python/stack_env.py`). For a stack started with `AIM_BACKEND_PORT=18080 AIM_FRONTEND_PORT=13000`:

```bash
AIM_BACKEND_PORT=18080 AIM_FRONTEND_PORT=13000 python run_demo.py
```

The Java demo reads `AIM_URL` only.

> **Note**: Authentication is handled automatically by the SDK via bundled credentials. No API key environment variable needed.

## Troubleshooting

### Connection Refused
Ensure the AIM backend is running at the configured URL.

### SDK Not Found
Build the SDK first:
```bash
# Python
cd sdk/python && pip install -e .

# Java
cd sdk/java && mvn package -DskipTests
```
