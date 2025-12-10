# 📊 Trust Scoring Guide - 8-Factor Trust Algorithm

Understanding and optimizing your agent's trust score.

## What is Trust Score?

**Trust Score** is a number between **0.0** (no trust) and **1.0** (complete trust) that represents how trustworthy your agent is.

**Why it matters**:
- ✅ **Risk Management**: Low trust = higher risk
- ✅ **Compliance**: Required for SOC 2, HIPAA audits
- ✅ **Automation**: High-trust agents can run unsupervised
- ✅ **Security**: Detects compromised agents

**Example Trust Scores**:
- **0.95+**: Excellent - Ready for production
- **0.80-0.94**: Good - Minor improvements needed
- **0.70-0.79**: Fair - Address low-scoring factors
- **Below 0.70**: Poor - Requires immediate attention

---

## The 8 Factors

Your agent's trust score is calculated from 8 weighted factors:

```python
Trust Score =
    (0.25 × Verification Status) +
    (0.15 × Uptime & Availability) +
    (0.15 × Action Success Rate) +
    (0.15 × Security Alerts) +
    (0.10 × Compliance Score) +
    (0.10 × Age & History) +
    (0.05 × Drift Detection) +
    (0.05 × User Feedback)
```

### Factor 1: Verification Status (25% weight)

**What it measures**: Percentage of actions successfully verified with Ed25519 signatures

**Calculation**:
```python
verification_status = verified_actions / total_actions
```

**Example**:
- Total actions: 1000
- Verified actions: 1000
- **Score**: 1.0 (100%)

**How to improve**:
- ✅ Always use `agent.verify_capability()` or `@agent.perform_action` decorator
- ✅ Ensure Ed25519 private key is valid
- ✅ Fix authentication errors immediately

**Code Example**:
```python
from aim_sdk import secure

agent = secure("my-agent")

# ✅ GOOD - Capability is automatically verified
@agent.perform_action(capability="data:read", risk_level="low")
def get_data(id: int):
    return {"data": f"Data for {id}"}

# ❌ BAD - Capability not verified (reduces score)
def get_data_unverified(id: int):
    return {"data": f"Data for {id}"}
```

---

### Factor 2: Uptime & Availability (15% weight)

**What it measures**: How often your agent is responsive to verification requests

**Calculation**:
```python
uptime = successful_health_checks / total_health_checks
```

**Example**:
- Health checks: 100
- Successful: 98
- Failed: 2 (network issues)
- **Score**: 0.98 (98%)

**How to improve**:
- ✅ Deploy on reliable infrastructure (Azure, AWS)
- ✅ Implement health check endpoint
- ✅ Monitor with Prometheus/Grafana
- ✅ Set up auto-restart on failure

**Code Example**:
```python
from aim_sdk import secure
from flask import Flask

agent = secure("my-agent")
app = Flask(__name__)

@app.route("/health")
def health():
    """Health check endpoint for monitoring"""
    return {
        "status": "healthy",
        "agent_id": agent.id,
        "trust_score": agent.get_trust_score()
    }

# Uptime monitoring pings /health every minute
# High uptime → high trust score
```

---

### Factor 3: Action Success Rate (15% weight)

**What it measures**: Percentage of actions that complete successfully

**Calculation**:
```python
success_rate = successful_actions / total_actions
```

**Example**:
- Total actions: 500
- Successful: 485
- Failed: 15
- **Score**: 0.97 (97%)

**How to improve**:
- ✅ Implement error handling in agent code
- ✅ Validate inputs before processing
- ✅ Use retries for transient failures
- ✅ Monitor and fix recurring errors

**Code Example**:
```python
from aim_sdk import secure
import requests

agent = secure("weather-agent")

@agent.perform_action(capability="weather:fetch", risk_level="low")
def get_weather(city: str) -> dict:
    """
    Get weather with error handling
    Prevents failed capabilities from lowering trust score
    """
    try:
        response = requests.get(
            "https://api.openweathermap.org/data/2.5/weather",
            params={"q": city, "appid": os.getenv("OPENWEATHER_API_KEY")},
            timeout=5  # Prevent hanging
        )
        response.raise_for_status()
        return response.json()

    except requests.exceptions.Timeout:
        # Handle timeout gracefully
        return {"error": "Timeout", "city": city}

    except requests.exceptions.HTTPError as e:
        # Handle HTTP errors
        return {"error": str(e), "city": city}

# Graceful error handling → high success rate → high trust score
```

---

### Factor 4: Security Alerts (15% weight)

**What it measures**: Number and severity of security alerts triggered

**Calculation**:
```python
if critical_alerts > 0:
    security_score = 0.0
elif high_alerts > 0:
    security_score = 0.50
elif medium_alerts > 0:
    security_score = 0.75
else:
    security_score = 1.0
```

**Example**:
- Critical alerts: 0
- High alerts: 0
- Medium alerts: 0
- Low alerts: 2
- **Score**: 1.0 (100%)

**Common Security Alerts**:
| Alert Level | Examples | Impact on Score |
|-------------|----------|-----------------|
| **Critical** | Compromised key, unauthorized access | Score → 0.0 |
| **High** | Unusual action patterns, privilege escalation | Score → 0.5 |
| **Medium** | Rate limit exceeded, suspicious parameters | Score → 0.75 |
| **Low** | Minor anomalies, warnings | No impact |

**How to improve**:
- ✅ Rotate keys regularly (every 90 days)
- ✅ Monitor for unusual activity
- ✅ Implement rate limiting
- ✅ Use least-privilege access
- ✅ Respond to alerts immediately

**Code Example**:
```python
from aim_sdk import secure

agent = secure("database-agent")

# Monitor for security alerts
def check_security_alerts():
    """Check for and respond to security alerts"""
    breakdown = agent.get_trust_score(detailed=True)

    if breakdown["factors"]["security_alerts"] < 0.75:
        # Get recent alerts
        alerts = agent.get_security_alerts(severity="high")

        for alert in alerts:
            print(f"🚨 {alert['severity']}: {alert['message']}")

            # Respond to alert
            if alert['type'] == "unusual_pattern":
                investigate_unusual_activity(alert)
            elif alert['type'] == "failed_verifications":
                rotate_key_immediately(agent)

# Run daily
check_security_alerts()
```

---

### Factor 5: Compliance Score (10% weight)

**What it measures**: Adherence to compliance policies (SOC 2, HIPAA, GDPR)

**Calculation**:
```python
compliance_score = compliant_actions / total_actions_requiring_compliance
```

**Example**:
- Actions requiring compliance logging: 200
- Properly logged: 200
- Missing logs: 0
- **Score**: 1.0 (100%)

**Compliance Requirements**:
| Standard | Requirements | AIM Support |
|----------|--------------|-------------|
| **SOC 2** | Audit trail, access controls | ✅ Automatic |
| **HIPAA** | PHI access logging, encryption | ✅ Automatic |
| **GDPR** | Data access logging, right to be forgotten | ✅ Automatic |

**How to improve**:
- ✅ Enable compliance logging (on by default)
- ✅ Export compliance reports monthly
- ✅ Classify data properly (PII, PHI, etc.)
- ✅ Implement data retention policies

**Code Example**:
```python
from aim_sdk import secure

agent = secure("hr-agent")

# Export compliance report monthly
def generate_monthly_compliance_report():
    """Generate SOC 2 compliance report"""
    report = agent.export_compliance_report(
        report_type="soc2",
        start_date="2025-10-01T00:00:00Z",
        end_date="2025-10-31T23:59:59Z",
        format="pdf"
    )

    # Save report
    with open(f"compliance_report_oct_2025.pdf", "wb") as f:
        f.write(report)

    print("✅ Compliance report generated")
    # High compliance → high trust score
```

---

### Factor 6: Age & History (10% weight)

**What it measures**: How long the agent has been operating successfully

**Calculation**:
```python
days_active = (now - agent.created_at).days

if days_active < 7:
    age_score = 0.30
elif days_active < 30:
    age_score = 0.50
elif days_active < 90:
    age_score = 0.75
else:
    age_score = 1.0
```

**Example**:
- Agent age: 5 days
- **Score**: 0.30 (new agent)

**How it improves over time**:
| Age | Score |
|-----|-------|
| < 7 days | 0.30 |
| 7-30 days | 0.50 |
| 30-90 days | 0.75 |
| 90+ days | 1.00 |

**How to improve**:
- ⏰ **Wait**: Score automatically improves as agent ages
- ✅ Maintain high uptime and success rate
- ✅ Avoid security alerts
- ✅ Run agent continuously

**Note**: New agents start with lower trust scores. This is intentional - trust must be earned over time.

---

### Factor 7: Drift Detection (5% weight)

**What it measures**: Changes in agent behavior patterns

**Calculation**:
```python
if behavioral_change_detected:
    drift_score = 0.0
else:
    drift_score = 1.0
```

**What AIM monitors**:
- Action frequency patterns
- Parameter distributions
- Response times
- Error rates
- New action types

**Example**:
- Normal: 100 API calls/hour
- Suddenly: 10,000 API calls/hour
- **Alert**: Behavioral drift detected
- **Score**: 0.0 (until investigated)

**How to improve**:
- ✅ Gradual changes (not sudden spikes)
- ✅ Document expected behavior changes
- ✅ Acknowledge drift alerts in dashboard
- ✅ Investigate anomalies promptly

**Code Example**:
```python
from aim_sdk import secure

agent = secure("api-agent")

# Monitor for drift
def monitor_behavioral_drift():
    """Check for unexpected behavior changes"""
    breakdown = agent.get_trust_score(detailed=True)

    if breakdown["factors"]["drift_detection"] < 1.0:
        # Get drift details
        drift_events = agent.get_drift_events()

        for event in drift_events:
            print(f"⚠️  Drift detected: {event['type']}")
            print(f"   Baseline: {event['baseline']}")
            print(f"   Current: {event['current']}")
            print(f"   Deviation: {event['deviation']}%")

            # Acknowledge if expected
            if event['type'] == "increased_traffic" and is_expected_traffic_increase():
                agent.acknowledge_drift(event['id'], reason="Black Friday sales")

# Run daily
monitor_behavioral_drift()
```

---

### Factor 8: User Feedback (5% weight)

**What it measures**: Explicit feedback from users about agent performance

**Calculation**:
```python
positive_feedback = thumbs_up_count
negative_feedback = thumbs_down_count

if negative_feedback > 5:
    feedback_score = 0.0
elif negative_feedback > 2:
    feedback_score = 0.50
elif positive_feedback > 10:
    feedback_score = 1.0
else:
    feedback_score = 0.75
```

**Example**:
- Positive feedback: 25
- Negative feedback: 1
- **Score**: 1.0 (100%)

**How to collect feedback**:
```python
from aim_sdk import secure

agent = secure("chatbot-agent")

# Collect user feedback
def submit_user_feedback(agent_id: str, rating: int, comment: str):
    """Submit user feedback for agent"""
    agent.submit_feedback(
        rating=rating,  # 1-5 stars
        comment=comment,
        user_id=current_user.id
    )

# In your chatbot UI
# "Was this helpful? 👍 👎"
# If 👍: submit_user_feedback(agent.id, 5, "Very helpful!")
# If 👎: submit_user_feedback(agent.id, 1, "Incorrect information")
```

---

## Viewing Trust Score

### Simple Trust Score

```python
from aim_sdk import secure

agent = secure("my-agent")

# Get overall trust score
score = agent.get_trust_score()
print(f"Trust Score: {score}")  # 0.95
```

### Detailed Breakdown

```python
from aim_sdk import secure

agent = secure("my-agent")

# Get detailed breakdown
breakdown = agent.get_trust_score(detailed=True)

print(f"Overall Trust Score: {breakdown['overall']}")
print("\nFactor Breakdown:")
for factor, score in breakdown['factors'].items():
    print(f"  {factor}: {score:.2f}")
```

**Output**:
```
Overall Trust Score: 0.95

Factor Breakdown:
  verification_status: 1.00
  uptime: 1.00
  success_rate: 0.98
  security_alerts: 1.00
  compliance: 1.00
  age: 0.75
  drift_detection: 1.00
  user_feedback: 1.00
```

---

## Improving Trust Score

### Step 1: Identify Low-Scoring Factors

```python
from aim_sdk import secure

agent = secure("my-agent")

# Get breakdown
breakdown = agent.get_trust_score(detailed=True)

# Find low-scoring factors
low_factors = {
    factor: score
    for factor, score in breakdown['factors'].items()
    if score < 0.75
}

if low_factors:
    print("⚠️  Low-scoring factors:")
    for factor, score in low_factors.items():
        print(f"  - {factor}: {score:.2f}")
else:
    print("✅ All factors above 0.75!")
```

### Step 2: Address Each Factor

```python
from aim_sdk import secure

agent = secure("my-agent")

def improve_trust_score():
    """Systematic trust score improvement"""
    breakdown = agent.get_trust_score(detailed=True)

    # Factor 1: Verification Status
    if breakdown['factors']['verification_status'] < 0.95:
        print("❌ Low verification status")
        print("   → Add @agent.perform_action decorators to all functions")

    # Factor 2: Uptime
    if breakdown['factors']['uptime'] < 0.95:
        print("❌ Low uptime")
        print("   → Deploy on more reliable infrastructure")
        print("   → Implement auto-restart on failure")

    # Factor 3: Success Rate
    if breakdown['factors']['success_rate'] < 0.95:
        print("❌ Low success rate")
        print("   → Review recent failed actions")
        logs = agent.get_audit_logs(limit=100)
        failed = [log for log in logs if not log['success']]
        print(f"   → {len(failed)} failed actions in last 100")

    # Factor 4: Security Alerts
    if breakdown['factors']['security_alerts'] < 1.0:
        print("❌ Security alerts detected")
        alerts = agent.get_security_alerts()
        print(f"   → {len(alerts)} active alerts")
        print("   → Investigate and resolve immediately")

    # Factor 5: Compliance
    if breakdown['factors']['compliance'] < 1.0:
        print("❌ Compliance issues detected")
        print("   → Ensure all sensitive actions are logged")

    # Factor 6: Age
    if breakdown['factors']['age'] < 0.75:
        days_active = (datetime.now() - agent.created_at).days
        print(f"⏰ New agent ({days_active} days old)")
        print("   → Trust score will improve over time")

    # Factor 7: Drift Detection
    if breakdown['factors']['drift_detection'] < 1.0:
        print("❌ Behavioral drift detected")
        print("   → Review drift events in dashboard")
        print("   → Acknowledge expected changes")

    # Factor 8: User Feedback
    if breakdown['factors']['user_feedback'] < 0.75:
        print("❌ Negative user feedback")
        print("   → Review user complaints")
        print("   → Improve agent responses")

# Run weekly
improve_trust_score()
```

---

## Trust Score Thresholds

### Production Readiness

```python
from aim_sdk import secure

agent = secure("production-agent")

def check_production_readiness():
    """Verify agent is ready for production"""
    score = agent.get_trust_score()

    if score >= 0.95:
        print("✅ PRODUCTION READY - Excellent trust score")
        return True
    elif score >= 0.80:
        print("⚠️  CAUTION - Good trust score, minor improvements recommended")
        return True
    elif score >= 0.70:
        print("❌ NOT READY - Fair trust score, address issues before production")
        return False
    else:
        print("❌ NOT READY - Poor trust score, requires immediate attention")
        return False

# Check before deployment
if check_production_readiness():
    deploy_to_production()
else:
    print("Fix trust score issues first")
```

### Automated Actions by Trust Score

```python
from aim_sdk import secure

agent = secure("my-agent")

@agent.perform_action(capability="sensitive:execute", risk_level="high")
def sensitive_action(data):
    """Sensitive capability with trust score check"""
    score = agent.get_trust_score()

    if score >= 0.90:
        # High trust - execute immediately
        return execute_action(data)
    elif score >= 0.75:
        # Medium trust - require approval
        return await_approval(data)
    else:
        # Low trust - block execution
        raise PermissionError("Trust score too low for this action")
```

---

## Monitoring Trust Score Over Time

### Track Historical Trends

```python
from aim_sdk import secure
import matplotlib.pyplot as plt

agent = secure("my-agent")

# Get historical trust scores
history = agent.get_trust_score_history(days=30)

# Plot trend
dates = [h['date'] for h in history]
scores = [h['score'] for h in history]

plt.plot(dates, scores)
plt.xlabel("Date")
plt.ylabel("Trust Score")
plt.title("Trust Score Trend (Last 30 Days)")
plt.ylim(0, 1)
plt.axhline(y=0.95, color='g', linestyle='--', label='Excellent (0.95)')
plt.axhline(y=0.80, color='y', linestyle='--', label='Good (0.80)')
plt.axhline(y=0.70, color='r', linestyle='--', label='Fair (0.70)')
plt.legend()
plt.show()
```

### Set Up Alerts

```python
from aim_sdk import secure

agent = secure("my-agent")

def monitor_trust_score():
    """Alert when trust score drops"""
    score = agent.get_trust_score()

    if score < 0.70:
        send_critical_alert(f"🚨 Trust score critically low: {score}")
    elif score < 0.80:
        send_warning_alert(f"⚠️  Trust score below threshold: {score}")

# Run every hour
schedule.every().hour.do(monitor_trust_score)
```

---

## Examples

### Complete Trust Score Monitoring

```python
from aim_sdk import secure
from datetime import datetime
import json

agent = secure("production-agent")

def comprehensive_trust_check():
    """Complete trust score analysis"""
    print("=" * 60)
    print(f"Trust Score Report - {datetime.now()}")
    print("=" * 60)

    # Get detailed breakdown
    breakdown = agent.get_trust_score(detailed=True)

    # Overall score
    overall = breakdown['overall']
    print(f"\n📊 Overall Trust Score: {overall:.3f}")

    if overall >= 0.95:
        status = "✅ EXCELLENT"
    elif overall >= 0.80:
        status = "✅ GOOD"
    elif overall >= 0.70:
        status = "⚠️  FAIR"
    else:
        status = "❌ POOR"

    print(f"   Status: {status}")

    # Factor breakdown
    print("\n📋 Factor Breakdown:")
    print("-" * 60)

    factors = breakdown['factors']
    for factor, score in factors.items():
        weight = {
            'verification_status': 0.25,
            'uptime': 0.15,
            'success_rate': 0.15,
            'security_alerts': 0.15,
            'compliance': 0.10,
            'age': 0.10,
            'drift_detection': 0.05,
            'user_feedback': 0.05
        }[factor]

        contribution = score * weight

        # Status emoji
        if score >= 0.95:
            emoji = "✅"
        elif score >= 0.75:
            emoji = "⚠️ "
        else:
            emoji = "❌"

        print(f"{emoji} {factor:25} {score:5.2f}  (Weight: {weight:5.2%}, Contrib: {contribution:.3f})")

    # Recommendations
    print("\n💡 Recommendations:")
    print("-" * 60)

    if factors['verification_status'] < 0.95:
        print("  • Add verification to all actions")

    if factors['success_rate'] < 0.95:
        print("  • Improve error handling")

    if factors['security_alerts'] < 1.0:
        print("  • Resolve active security alerts")

    if factors['age'] < 0.75:
        print("  • Continue running agent (trust improves with age)")

    if all(v >= 0.95 for v in factors.values()):
        print("  ✅ All factors excellent! No improvements needed.")

    print("\n" + "=" * 60)

# Run daily report
comprehensive_trust_check()
```

---

## Best Practices

### 1. Monitor Trust Score Daily

```python
# ✅ GOOD - Check trust score every day
def daily_trust_check():
    score = agent.get_trust_score()
    if score < 0.80:
        send_alert(f"Trust score: {score}")

schedule.every().day.at("09:00").do(daily_trust_check)
```

### 2. Fix Low Factors Immediately

```python
# ✅ GOOD - Respond to low factors promptly
breakdown = agent.get_trust_score(detailed=True)

for factor, score in breakdown['factors'].items():
    if score < 0.70:
        investigate_and_fix_immediately(factor, score)
```

### 3. Track Trends Over Time

```python
# ✅ GOOD - Monitor trends, not just current score
history = agent.get_trust_score_history(days=7)

# Check if trending down
if is_trending_down(history):
    investigate_cause()
```

---

## Next Steps

- **[Python SDK Guide →](./python.md)** - Complete SDK reference
- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography
- **[Auto-Detection Guide →](./auto-detection.md)** - MCP server discovery

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
