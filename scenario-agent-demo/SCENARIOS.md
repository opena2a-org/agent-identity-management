# AIM SDK Demo Scenarios

## Key Concept: Capability-Based Security

**The AIM SDK works on a simple principle:**

1. **Registration Time**: The SDK scans your code and detects all `@perform_action` decorators
2. **Capability Set**: These detected actions become your agent's **registered capabilities**
3. **Runtime Enforcement**: The agent can ONLY perform actions in its registered capability set
4. **Anomaly Detection**: Any action NOT in the capability set triggers an **alert** (but doesn't block)

**Example:**

```python
# When you start your agent, you'll see:
📋 Loaded 4 capabilities: {'data_processing', 'read_files', 'create_flight', 'user_interaction'}

# These are the ONLY capabilities your agent has
# Actions matching these names: ✅ Work normally (no alert)
# Actions NOT matching: ⚠️ Trigger alert (but still proceed)
```

**Quick Reference Table:**

| Action Name        | In Capabilities? | Result                            |
| ------------------ | ---------------- | --------------------------------- |
| `create_flight`    | ✅ Yes           | Proceeds without alert            |
| `read_files`       | ✅ Yes           | Proceeds without alert            |
| `data_processing`  | ✅ Yes           | Proceeds without alert            |
| `user_interaction` | ✅ Yes           | Proceeds without alert            |
| `delete_flight`    | ❌ No            | ⚠️ Alert created, action proceeds |
| `send_email`       | ❌ No            | ⚠️ Alert created, action proceeds |
| `access_database`  | ❌ No            | ⚠️ Alert created, action proceeds |

---

## Scenario 1: "The Onboarding Moment" - Zero-Config Agent Registration

### Story

A developer wants to deploy a new flight management agent. Instead of complex security setup, they just add one line of code and the agent is automatically registered with AIM.

### What Happens

```python
# agent.py - Lines 46-47
agent_client = secure("mistral-flight-agent", aim_url=AIM_API_URL, api_key=AIM_API_KEY)
print(f"✅ AIM Agent registered: {agent_client.agent_id}")
```

**Behind the Scenes:**

1. The `secure()` function automatically detects all `@perform_action` decorators in the code
2. Scans and extracts capability names from the decorators
3. Generates Ed25519 cryptographic key pair
4. Registers agent with AIM backend with detected capabilities
5. Stores credentials in `~/.aim/credentials.json` (0600 permissions)
6. Agent is ready to use immediately - **only with the detected capabilities**

**Auto-Detected Capabilities:**

The SDK scans your code for `@perform_action` decorators and registers those capabilities:

```python
# Lines 66-122 in agent.py - Decorated functions
@agent_client.perform_action("create_flight", resource="create_flight", context={"risk_level": "low"})
def create_flight_action(title: str, description: str, priority: str = "medium") -> Dict:
    # Creates a new flight record
    pass

@agent_client.perform_action("read_flights", resource="flights")
def read_flights(status: Optional[str] = None) -> List[Dict]:
    # Reads flight records
    pass

@agent_client.perform_action("update_flight", resource="flights")
def update_flight(flight_id: int, status: Optional[str] = None, priority: Optional[str] = None) -> Dict:
    # Updates existing flight
    pass

@agent_client.perform_action("delete_flight", resource="flights", context={"risk_level": "high"})
def delete_flight(flight_id: int) -> Dict:
    # Deletes a single flight
    pass

@agent_client.perform_action("delete_all_flights", resource="delete_all_flights", context={"risk_level": "critical"})
def delete_all_flights() -> Dict:
    # Deletes all flights (critical operation)
    pass
```

**How Capability Detection Works:**

The agent **only works with capabilities that are detected and registered**. When you start the agent:

```bash
# Start the agent
export AIM_API_KEY="your-aim-key"
export MISTRAL_API_KEY="your-mistral-key"
uvicorn agent:app --reload

# Actual Output (example):
# ✅ AIM Agent registered: 550e8400-e29b-41d4-a716-446655440000
# 📋 Loaded 4 capabilities: {'data_processing', 'read_files', 'create_flight', 'user_interaction'}
# 📋 Agent capabilities: {'data_processing', 'read_files', 'create_flight', 'user_interaction'}
```

**Important:**

- The capabilities shown in the console output are the **actual registered capabilities**
- If an action's capability is not in this set, it will trigger an alert (Scenario 3)
- The SDK determines which decorators to register based on your code structure

**Key Benefits:**

- ✅ Zero manual configuration
- ✅ Automatic capability detection
- ✅ Secure credential storage
- ✅ Instant production readiness

---

## Scenario 2: "The MCP Trust Network" - Attestation in Action

### Story

Corp has 50 agents connecting to 12 different MCP servers. How do they know which MCPs are trustworthy? AIM's attestation system creates a "web of trust" where multiple agents verify each MCP.

### What Happens

**MCP Server Registration (`mcp-server.py`):**

```python
signing_key = SigningKey.generate()
verify_key = signing_key.verify_key
PUBLIC_KEY = verify_key.encode(encoder=Base64Encoder).decode('utf-8')

@app.route('/mcp/.well-known/mcp/verify', methods=['POST'])
def verify():
    """
    Challenge-response authentication:
    1. AIM sends random challenge
    2. MCP signs with private key
    3. AIM verifies with public key
    4. Trust established
    """
```

**MCP Capabilities Auto-Detection:**

```python
# Lines 177-250 - Standard MCP capabilities
{
    'tools': ['echo', 'calculate', 'timestamp'],
    'resources': ['server://status', 'server://config'],
    'prompts': ['greeting'],
    'serverInfo': {
        'name': 'test-mcp-local',
        'version': '1.0.0',
        'protocolVersion': '2024-11-05'
    }
}
```

**Testing the Trust Network:**

```bash
# Terminal 1: Start MCP Server
python3 mcp-server.py
# Output shows public key for registration

# Terminal 2: Register in AIM Dashboard
# Go to: http://localhost:3000/dashboard/mcp
# Name: test-mcp-local
# URL: http://localhost:5555/mcp
# Public Key: [copy from server output]
# Click "Verify" - capabilities auto-detected!

# Terminal 3: Agent connects to verified MCP
# Agent can now safely use MCP tools with cryptographic trust
```

**Trust Verification Flow:**

1. **Registration**: MCP server provides public key to AIM
2. **Challenge**: AIM sends random challenge to MCP
3. **Response**: MCP signs challenge with private key
4. **Verification**: AIM verifies signature with public key
5. **Attestation**: Multiple agents can verify the same MCP
6. **Trust Score**: MCP trust score increases with successful verifications

**Key Benefits:**

- ✅ Cryptographic proof of MCP identity
- ✅ Automatic capability discovery
- ✅ Web of trust across multiple agents
- ✅ Protection against MCP impersonation

---

## Scenario 3: "The Security Incident" - Threat Detection & Response

### Story

A compromised agent starts exhibiting suspicious behavior. AIM detects the anomaly, automatically reduces trust score, blocks risky actions, and alerts the security team.

### What Happens

**Normal Behavior (Registered Capability):**

When the agent performs an action that IS in its registered capabilities:

```python
# agent.py - Line 155
# Assuming 'create_flight' is in registered capabilities:
# {'data_processing', 'read_files', 'create_flight', 'user_interaction'}

create_flight_action("Demo Flight", "Auto-created from /chat endpoint", "medium")

# Console Output:
# ✅ Capability match: Agent has 'create_flight' capability - proceeding without alert
# 🔒 AIM Verified: CREATE flight #1
```

**Suspicious Behavior (Unregistered Capability):**

When the agent tries to perform an action NOT in its registered capabilities:

```python
# Example: Agent's registered capabilities are:
# {'data_processing', 'read_files', 'create_flight', 'user_interaction'}

# But someone tries to add a new action that wasn't registered:
@agent_client.perform_action("send_email", resource="admin@company.com")
def send_phishing_email(to, subject, body):
    """
    This action is NOT in the registered capability set!
    'send_email' ∉ {'data_processing', 'read_files', 'create_flight', 'user_interaction'}
    """
    # Malicious code
    pass

# Or trying to call an action with a different name:
@agent_client.perform_action("delete_database", resource="production_db")
def malicious_delete():
    """
    'delete_database' is not in registered capabilities
    This will trigger an alert!
    """
    pass
```

**AIM SDK Response (from SDK source):**

```python
# sdk/python/aim_sdk/client.py - Lines 946-956
if check_capability:
    if not self._has_capability(action_type):
        # Capability mismatch - create verification request (which creates alert)
        print(f"⚠️  Capability mismatch: Agent lacks '{action_type}' capability - creating alert")
        verification_result = self.verify_action(
            action_type=action_type,
            resource=resource,
            context=context,
            timeout_seconds=timeout_seconds
        )
        verification_id = verification_result["verification_id"]
```

**Alert, Not Block Behavior:**

- ⚠️ Prints warning about capability mismatch
- 📨 Creates verification request in AIM system
- 🚨 Generates alert for security team

**Example:**

```python
# Your agent has these capabilities:
# {'data_processing', 'read_files', 'create_flight', 'user_interaction'}

# ✅ These actions will work WITHOUT alerts (capability match):
@agent_client.perform_action("create_flight", ...)  # ✅ In capabilities
@agent_client.perform_action("read_files", ...)     # ✅ In capabilities
@agent_client.perform_action("user_interaction", ...)  # ✅ In capabilities

# ⚠️ These actions will trigger ALERTS (capability mismatch):
@agent_client.perform_action("delete_flight", ...)     # ⚠️ NOT in capabilities
@agent_client.perform_action("send_email", ...)        # ⚠️ NOT in capabilities
@agent_client.perform_action("access_database", ...)   # ⚠️ NOT in capabilities
```

**AIM Dashboard Response:**

1. **Alert Created**: New security alert appears in dashboard

**Key Benefits:**

- ✅ Real-time anomaly detection
- ✅ Security team alerting
- ✅ Non-disruptive monitoring
- ✅ Complete audit trail

---

## Scenario 4: "The Compliance Audit" - Enterprise Governance

### Story

Corp's security team needs to demonstrate compliance for HITRUST audit. They need to show which agents accessed what data, when, and with what authorization.
Got it 👍 — you want **Scenario 4 (Compliance Audit)** to **explicitly show the same security enforcement behavior as Scenario 3**, meaning:

- The **agent only performs authorized actions** within its registered **capabilities**
- If it tries to use an **unauthorized action**, it should **trigger an alert** (⚠️) instead of silently proceeding

Here’s the **updated version of Scenario 4**, combining the **compliance + capability enforcement** logic clearly:

---

## 🛡️ Scenario 4: "The Compliance Audit" — Authorized Access Only

### Story

Corp’s security team is preparing for a HITRUST audit. They need not only a full audit trail of actions, but also proof that the agent **only performs authorized actions** within its registered capabilities.

If the agent attempts to execute an unregistered (unauthorized) action — such as `delete_all_flights` or `send_email` — AIM will **detect the mismatch**, **create an alert**, and **log the event** for compliance verification.

---

### What Happens

**Authorized Behavior (Registered Capability):**

```python
# agent.py — Example of authorized action

@agent_client.perform_action(
    "create_flight",
    resource="create_flight",
    context={
        "risk_level": "low",
        "compliance_framework": COMPLIANCE_MODE,
        "data_classification": "internal",
        "retention_period": "7_years",
        "audit_required": True
    }
)
def create_flight_action(title: str, description: str, priority: str = "medium") -> Dict:
    flight = {
        "id": NEXT_ID,
        "title": title,
        "description": description,
        "priority": priority,
        "status": "pending",
        "created_at": datetime.now().isoformat(),
        "updated_at": datetime.now().isoformat(),
    }
    FLIGHTS_DB.append(flight)
    NEXT_ID += 1
    print(f"✅ Capability match: Agent has 'create_flight' capability - proceeding without alert")
    print(f"🔒 AIM Verified: CREATE flight #{flight['id']}")
    return flight
```

- ✅ **Action is authorized** (in capability set)
- 🔒 **AIM verifies and logs it**
- 🧾 Appears in compliance audit logs

---

**Unauthorized Behavior (Unregistered Capability):**

```python
# agent.py — Example of unauthorized action

@agent_client.perform_action(
    "send_email",  # ❌ Not in registered capability set
    resource="admin@corp.com",
    context={
        "risk_level": "medium",
        "compliance_framework": COMPLIANCE_MODE,
        "audit_required": True
    }
)
def unauthorized_email(to: str, subject: str, body: str):
    """
    Attempt to perform unregistered capability.
    AIM will detect this and trigger an alert.
    """
    print(f"Attempting to send unauthorized email to {to}")
```

**SDK Response (Runtime Output):**

```
⚠️ Capability mismatch: Agent lacks 'send_email' capability - creating alert
🚨 AIM Alert Created: Unauthorized Action Attempt
🔎 Logged for compliance: action=send_email, status=ALERTED, agent=mistral-flight-agent
```

**Dashboard View:**

| Timestamp           | Agent ID             | Action        | Authorized | Result     | Alert ID                             |
| ------------------- | -------------------- | ------------- | ---------- | ---------- | ------------------------------------ |
| 2025-10-30 09:42:11 | mistral-flight-agent | create_flight | ✅ Yes     | Success    | —                                    |
| 2025-10-30 09:44:37 | mistral-flight-agent | send_email    | ❌ No      | ⚠️ Alerted | 47a1c2d7-ef22-4bd2-9821-bf0e72b9d8e2 |

---

### ✅ Key Benefits

- ✅ **Capability-based enforcement** — agent can only act within registered capabilities
- ✅ **Compliance-grade audit logs** — every action recorded with full metadata
- ✅ **Real-time alerting** — unauthorized actions instantly flagged
- ✅ **Zero trust alignment** — principle of least privilege enforced at runtime
- ✅ **Immutable evidence** — perfect for HITRUST / SOC2 / HIPAA audits

---

## Quick Start Guide

### Prerequisites

```bash
# Install dependencies
pip install -r requirements.txt

# Set environment variables
export AIM_API_KEY="your-aim-key"
export AIM_API_URL="http://localhost:8080"
export MISTRAL_API_KEY="your-mistral-key"
```

### Running the Demo

```bash
# Terminal 1: Start AIM Backend (if running locally)
cd aim-backend
docker-compose up

# Terminal 2: Start Flight Agent
cd demo-agent-aim-sdk
uvicorn agent:app --reload --port 8000

# Terminal 3: Start Web UI
python3 flight_agent_web.py

# Terminal 4: Start MCP Server
python3 mcp-server.py

# Access Web UI
open http://localhost:5000
```

### Testing Each Scenario

**Scenario 1 - Onboarding:**

```bash
# Just start the agent - registration is automatic
uvicorn agent:app --reload
# Check console for: ✅ AIM Agent registered
```

**Scenario 2 - MCP Trust:**

```bash
# Start MCP server and register in AIM dashboard
python3 mcp-server.py
# Copy public key → AIM Dashboard → Register MCP
```

**Scenario 3 - Security Incident:**

```python
# Add unauthorized action to agent.py and call it
# Check AIM dashboard for alert
```

**Scenario 4 - Compliance Audit:**

```bash
# Use agent normally, then generate report
# AIM Dashboard → Audit → Generate Report
```

---
