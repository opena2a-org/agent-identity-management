# Quick Start: Auto-Capability Detection

## 🚀 5-Minute Setup

### Installation
```bash
pip install aim-sdk
```

### Basic Usage

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import auto_detect_capabilities

# 1. Initialize AIM client
aim_client = AIMClient.auto_register_or_load(
    agent_name="my-agent",
    aim_url="http://localhost:8080"
)

# 2. Auto-detect capabilities
result = auto_detect_capabilities(
    aim_client=aim_client,
    agent_id=aim_client.agent_id,
    auto_detect_from_mcp=True
)

# 3. Check results
print(f"Risk Level: {result['risk_assessment']['risk_level']}")
print(f"Risk Score: {result['risk_assessment']['overall_risk_score']}")
print(f"Trust Impact: {result['risk_assessment']['trust_score_impact']}")

# 4. Handle alerts
for alert in result['risk_assessment']['alerts']:
    if alert['severity'] in ['HIGH', 'CRITICAL']:
        print(f"⚠️ {alert['severity']}: {alert['message']}")
```

## 📋 Common Use Cases

### Use Case 1: Report Specific Capabilities

```python
from aim_sdk.integrations.mcp import auto_detect_capabilities

capabilities = [
    {
        "capability_type": "file_read",
        "capability_scope": {"paths": ["/etc/hosts"], "permissions": "read"},
        "risk_level": "MEDIUM",
        "detected_via": "static_analysis"
    }
]

result = auto_detect_capabilities(
    aim_client=aim_client,
    agent_id=agent_id,
    detected_capabilities=capabilities
)
```

### Use Case 2: Get All Capabilities

```python
from aim_sdk.integrations.mcp import get_agent_capabilities

capabilities = get_agent_capabilities(
    aim_client=aim_client,
    agent_id=agent_id
)

print(f"Total capabilities: {capabilities['total']}")
```

### Use Case 3: Scheduled Detection

```python
import schedule
from aim_sdk.integrations.mcp import auto_detect_capabilities

def detect_capabilities():
    result = auto_detect_capabilities(
        aim_client=aim_client,
        agent_id=agent_id,
        auto_detect_from_mcp=True
    )

    if result['risk_assessment']['risk_level'] in ['HIGH', 'CRITICAL']:
        send_security_alert(result)

# Run every hour
schedule.every().hour.do(detect_capabilities)
```

## 🎯 Backend Endpoint

**Endpoint**: `POST /api/v1/detection/agents/:id/capabilities/report`

**Handler**: `DetectionHandler.ReportCapabilities` (line 161 in detection_handler.go)

**Service**: `CapabilityService.AutoDetectCapabilities` (line 307 in capability_service.go)

## 📊 Risk Levels

- **CRITICAL** (85-100): Code execution, credential write → Trust impact: -15 to -10
- **HIGH** (65-84): Database write, file delete → Trust impact: -10 to -5
- **MEDIUM** (35-64): File read, network access → Trust impact: -5 to 0
- **LOW** (0-34): Read-only access → Trust impact: 0 to +5

## 🔧 Error Handling

```python
try:
    result = auto_detect_capabilities(
        aim_client=aim_client,
        agent_id=agent_id
    )
except ValueError as e:
    print(f"Validation error: {e}")
except PermissionError as e:
    print(f"Permission denied: {e}")
except Exception as e:
    print(f"API error: {e}")
```

## 📚 Full Documentation

See [AUTO_CAPABILITY_DETECTION.md](./docs/AUTO_CAPABILITY_DETECTION.md) for:
- Complete API reference
- Advanced usage examples
- Best practices
- Troubleshooting guide
- Integration with trust scoring

## ✅ Implementation Checklist

- [x] Backend endpoint discovered: `POST /api/v1/detection/agents/:id/capabilities/report`
- [x] SDK function created: `auto_detect_capabilities()`
- [x] Helper function created: `get_agent_capabilities()`
- [x] Module exports updated: `aim_sdk/integrations/mcp/__init__.py`
- [x] Comprehensive documentation written
- [x] Test script created
- [x] Code follows existing SDK style
- [x] Imports verified working
- [x] Function signatures validated

## 🎉 Ready to Use!

The auto-capability detection feature is fully implemented and ready for production use.
