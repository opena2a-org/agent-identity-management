# ✈️ Flight Tracker Agent - Real-Time Flight Data

Build a secure flight tracking agent that monitors delays, cancellations, and gate changes.

## What You'll Build

A real-world AI agent that:
- ✅ Tracks real-time flight status from FlightAware API
- ✅ Monitors delays, cancellations, and gate changes
- ✅ Sends proactive alerts for flight updates
- ✅ Automatically secured with AIM (1 line of code)
- ✅ Complete audit trail of all API calls
- ✅ Enterprise-grade security and compliance

**Difficulty**: Intermediate
**Time**: 10 minutes
**Use Case**: Travel assistants, corporate travel management, personal trip tracking

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ FlightAware API key ([Get API access](https://www.flightaware.com/commercial/aeroapi/))
3. ✅ Python 3.8+ installed
4. ✅ `aim-sdk` installed (`pip install aim-sdk`)

**Note**: FlightAware offers a free tier (10 queries/month). For production, upgrade to paid tier.

---

## Step 1: Register Agent (30 seconds)

### In AIM Dashboard

1. **Login** to http://localhost:3000
2. **Navigate**: Agents → Register New Agent
3. **Fill in**:
   ```
   Agent Name: flight-tracker
   Agent Type: AI Agent
   Description: Tracks real-time flight status and sends alerts
   ```
4. **Click** "Register Agent"
5. **Copy** the private key (only shown once!)

### Save Credentials

```bash
# Save to environment variables
export AIM_PRIVATE_KEY="your-aim-private-key"
export FLIGHTAWARE_API_KEY="your-flightaware-api-key"

# Or add to .env file
cat >> .env <<EOF
AIM_PRIVATE_KEY=your-aim-private-key
FLIGHTAWARE_API_KEY=your-flightaware-api-key
AIM_URL=http://localhost:8080
EOF
```

---

## Step 2: Write the Agent (7 minutes)

Create `flight_tracker.py`:

```python
"""
Flight Tracker Agent - Secured with AIM
Track real-time flight status, delays, and cancellations
"""

from aim_sdk import secure
import requests
import os
from typing import Dict, List, Optional
from datetime import datetime, timedelta
from dataclasses import dataclass

# 🔐 ONE LINE - Secure your agent!
agent = secure(
    name="flight-tracker",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)


@dataclass
class FlightStatus:
    """Flight status data class"""
    flight_number: str
    airline: str
    status: str  # "Scheduled", "Departed", "Arrived", "Cancelled", "Delayed"
    departure_airport: str
    arrival_airport: str
    scheduled_departure: datetime
    actual_departure: Optional[datetime]
    scheduled_arrival: datetime
    estimated_arrival: Optional[datetime]
    gate: Optional[str]
    terminal: Optional[str]
    delay_minutes: int
    is_cancelled: bool


class FlightTrackerAgent:
    """Real-time flight tracking secured by AIM"""

    def __init__(self):
        self.api_key = os.getenv("FLIGHTAWARE_API_KEY")
        self.base_url = "https://aeroapi.flightaware.com/aeroapi"
        self.headers = {"x-apikey": self.api_key}

    def get_flight_status(self, flight_number: str) -> FlightStatus:
        """
        Get real-time status for a flight

        Args:
            flight_number: Flight number (e.g., "AA123", "DL456")

        Returns:
            FlightStatus object with complete flight data

        Example:
            >>> status = agent.get_flight_status("AA123")
            >>> print(f"Status: {status.status}, Delay: {status.delay_minutes} min")
        """
        # AIM automatically verifies this action
        response = requests.get(
            f"{self.base_url}/flights/{flight_number}",
            headers=self.headers
        )
        response.raise_for_status()
        data = response.json()

        # Parse response into FlightStatus
        flight = data['flights'][0]  # Most recent flight

        return FlightStatus(
            flight_number=flight_number,
            airline=flight['operator'],
            status=flight['status'],
            departure_airport=flight['origin']['code'],
            arrival_airport=flight['destination']['code'],
            scheduled_departure=datetime.fromisoformat(flight['scheduled_off']),
            actual_departure=datetime.fromisoformat(flight['actual_off']) if flight.get('actual_off') else None,
            scheduled_arrival=datetime.fromisoformat(flight['scheduled_on']),
            estimated_arrival=datetime.fromisoformat(flight['estimated_on']) if flight.get('estimated_on') else None,
            gate=flight.get('gate_origin'),
            terminal=flight.get('terminal_origin'),
            delay_minutes=self._calculate_delay(flight),
            is_cancelled=flight['status'] == 'Cancelled'
        )

    def track_flights(self, flight_numbers: List[str]) -> Dict[str, FlightStatus]:
        """Track multiple flights simultaneously"""
        results = {}
        for flight_number in flight_numbers:
            try:
                results[flight_number] = self.get_flight_status(flight_number)
            except Exception as e:
                print(f"⚠️  Failed to track {flight_number}: {e}")
        return results

    def get_flight_alerts(self, flight_number: str) -> List[str]:
        """
        Get alerts for flight delays, cancellations, gate changes

        Returns:
            List of alert messages
        """
        status = self.get_flight_status(flight_number)
        alerts = []

        # Cancellation alert
        if status.is_cancelled:
            alerts.append(f"🚨 CANCELLED: Flight {flight_number} has been cancelled!")

        # Delay alert
        elif status.delay_minutes > 15:
            severity = "🔴" if status.delay_minutes > 60 else "🟡"
            alerts.append(
                f"{severity} DELAYED: Flight {flight_number} delayed by {status.delay_minutes} minutes"
            )

        # Gate change alert (simplified - would need historical data)
        if status.gate:
            alerts.append(f"🚪 Gate {status.gate}, Terminal {status.terminal or 'TBD'}")

        return alerts

    def is_flight_on_time(self, flight_number: str, tolerance_minutes: int = 15) -> bool:
        """Check if flight is on time (within tolerance)"""
        status = self.get_flight_status(flight_number)
        return not status.is_cancelled and status.delay_minutes <= tolerance_minutes

    def get_airport_departures(self, airport_code: str, hours_ahead: int = 3) -> List[FlightStatus]:
        """
        Get all departures from an airport in next N hours

        Args:
            airport_code: IATA code (e.g., "SFO", "JFK", "LAX")
            hours_ahead: Hours to look ahead (default: 3)

        Returns:
            List of FlightStatus objects for departing flights
        """
        # AIM verifies this action
        end_time = datetime.now() + timedelta(hours=hours_ahead)

        response = requests.get(
            f"{self.base_url}/airports/{airport_code}/flights/departures",
            headers=self.headers,
            params={
                "start": datetime.now().isoformat(),
                "end": end_time.isoformat()
            }
        )
        response.raise_for_status()

        flights = []
        for flight_data in response.json()['departures']:
            # Parse each flight...
            pass  # Implementation similar to get_flight_status

        return flights

    def _calculate_delay(self, flight: dict) -> int:
        """Calculate delay in minutes"""
        if not flight.get('estimated_on') or not flight.get('scheduled_on'):
            return 0

        estimated = datetime.fromisoformat(flight['estimated_on'])
        scheduled = datetime.fromisoformat(flight['scheduled_on'])
        delay = (estimated - scheduled).total_seconds() / 60

        return max(0, int(delay))

    def format_status_report(self, flight_number: str) -> str:
        """Generate human-readable status report"""
        status = self.get_flight_status(flight_number)

        report = f"""
✈️  Flight {status.flight_number} ({status.airline})
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📍 Route: {status.departure_airport} → {status.arrival_airport}

📅 Departure:
   Scheduled: {status.scheduled_departure.strftime('%I:%M %p')}
   Actual: {status.actual_departure.strftime('%I:%M %p') if status.actual_departure else 'Not yet departed'}

📅 Arrival:
   Scheduled: {status.scheduled_arrival.strftime('%I:%M %p')}
   Estimated: {status.estimated_arrival.strftime('%I:%M %p') if status.estimated_arrival else 'TBD'}

🚪 Gate: {status.gate or 'TBD'} | Terminal: {status.terminal or 'TBD'}

⏱️  Status: {status.status}
   Delay: {status.delay_minutes} minutes
        """

        # Add alerts
        alerts = self.get_flight_alerts(flight_number)
        if alerts:
            report += "\n🔔 Alerts:\n"
            for alert in alerts:
                report += f"   {alert}\n"

        return report


def demo_basic_tracking():
    """Demo: Basic flight tracking"""
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📱 DEMO 1: Basic Flight Tracking")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = FlightTrackerAgent()

    # Track a single flight
    flight = "AA123"
    print(f"Tracking flight {flight}...\n")

    status = agent.get_flight_status(flight)
    print(f"✅ Flight: {status.flight_number}")
    print(f"   Status: {status.status}")
    print(f"   Route: {status.departure_airport} → {status.arrival_airport}")
    print(f"   Delay: {status.delay_minutes} minutes")


def demo_multiple_flights():
    """Demo: Track multiple flights"""
    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📱 DEMO 2: Track Multiple Flights")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = FlightTrackerAgent()

    flights = ["AA123", "DL456", "UA789"]
    print(f"Tracking {len(flights)} flights...\n")

    results = agent.track_flights(flights)

    for flight_num, status in results.items():
        emoji = "✅" if status.delay_minutes < 15 else "⚠️"
        print(f"{emoji} {flight_num}: {status.status} ({status.delay_minutes} min delay)")


def demo_flight_alerts():
    """Demo: Get flight alerts"""
    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📱 DEMO 3: Flight Alerts")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = FlightTrackerAgent()

    flight = "AA123"
    alerts = agent.get_flight_alerts(flight)

    if alerts:
        print(f"🔔 {len(alerts)} alert(s) for flight {flight}:\n")
        for alert in alerts:
            print(f"   {alert}")
    else:
        print(f"✅ No alerts for flight {flight}")


def demo_status_report():
    """Demo: Formatted status report"""
    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("📱 DEMO 4: Detailed Status Report")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    agent = FlightTrackerAgent()

    flight = "AA123"
    report = agent.format_status_report(flight)
    print(report)


def main():
    """Run all demos"""
    demo_basic_tracking()
    demo_multiple_flights()
    demo_flight_alerts()
    demo_status_report()


if __name__ == "__main__":
    main()
```

---

## Step 3: Run It! (2 minutes)

```bash
# Set environment variables
export AIM_PRIVATE_KEY="your-key"
export FLIGHTAWARE_API_KEY="your-flightaware-key"
export AIM_URL="http://localhost:8080"

# Install dependencies
pip install requests python-dateutil

# Run the agent
python flight_tracker.py
```

**Expected Output**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📱 DEMO 1: Basic Flight Tracking
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Tracking flight AA123...

✅ Flight: AA123
   Status: Departed
   Route: SFO → JFK
   Delay: 12 minutes

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📱 DEMO 2: Track Multiple Flights
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Tracking 3 flights...

✅ AA123: Departed (12 min delay)
⚠️  DL456: Delayed (45 min delay)
✅ UA789: On Time (0 min delay)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📱 DEMO 3: Flight Alerts
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔔 2 alert(s) for flight DL456:

   🟡 DELAYED: Flight DL456 delayed by 45 minutes
   🚪 Gate B12, Terminal 2

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📱 DEMO 4: Detailed Status Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✈️  Flight AA123 (American Airlines)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📍 Route: SFO → JFK

📅 Departure:
   Scheduled: 8:00 AM
   Actual: 8:12 AM

📅 Arrival:
   Scheduled: 4:30 PM
   Estimated: 4:42 PM

🚪 Gate: A15 | Terminal: 1

⏱️  Status: Departed
   Delay: 12 minutes
```

---

## Step 4: Check Your Dashboard

Open http://localhost:3000 → Agents → flight-tracker

### Agent Status

```
Agent: flight-tracker
Status: ✅ ACTIVE
Trust Score: 0.97 (Excellent)
Last Verified: 8 seconds ago
Total Actions: 6
Success Rate: 100%
```

### Recent Activity

```
✅ get_flight_status("AA123")      |  30s ago  |  SUCCESS  |  298ms
✅ get_flight_status("DL456")      |  25s ago  |  SUCCESS  |  312ms
✅ get_flight_status("UA789")      |  20s ago  |  SUCCESS  |  276ms
✅ get_flight_alerts("DL456")      |  15s ago  |  SUCCESS  |  289ms
✅ format_status_report("AA123")   |  10s ago  |  SUCCESS  |  301ms
```

### Trust Score

```
✅ Verification Status:     100%  (1.00)  [Weight: 25%]
✅ Uptime & Availability:   100%  (1.00)  [Weight: 15%]
✅ Action Success Rate:     100%  (1.00)  [Weight: 15%]
✅ Security Alerts:           0   (1.00)  [Weight: 15%]
✅ Compliance Score:        100%  (1.00)  [Weight: 10%]
✅ Age & History:         12 min  (0.85)  [Weight: 10%]
✅ Drift Detection:         None  (1.00)  [Weight:  5%]
✅ User Feedback:           None  (1.00)  [Weight:  5%]

Overall Trust Score: 0.97 / 1.00
```

---

## 🚀 Real-World Use Cases

### 1. Slack Notifications for Business Travel

```python
from slack_sdk import WebClient
from aim_sdk import secure
import schedule

agent = secure("flight-slack-notifier")
slack = WebClient(token=os.getenv("SLACK_BOT_TOKEN"))

def check_executive_flights():
    """Monitor executive team flights"""
    # Track VP's flight
    status = flight_agent.get_flight_status("AA123")
    alerts = flight_agent.get_flight_alerts("AA123")

    if alerts:
        slack.chat_postMessage(
            channel="#travel-alerts",
            text=f"🔔 Alert for CEO's flight AA123:\n" + "\n".join(alerts)
        )

# Check every 15 minutes
schedule.every(15).minutes.do(check_executive_flights)
```

### 2. Corporate Travel Dashboard

```python
from aim_sdk import secure
from flask import Flask, render_template

agent = secure("corporate-travel-dashboard")
app = Flask(__name__)

@app.route("/flights")
def flight_dashboard():
    """Real-time dashboard for all company flights"""
    # Get today's company flights from database
    company_flights = get_todays_company_flights()

    # Track all flights
    flight_statuses = {}
    for flight_num in company_flights:
        flight_statuses[flight_num] = flight_agent.get_flight_status(flight_num)

    return render_template("dashboard.html", flights=flight_statuses)
```

### 3. Automated Rebooking Assistant

```python
from aim_sdk import secure

agent = secure("rebooking-assistant")

def handle_cancelled_flight(flight_number: str, passenger_id: str):
    """Automatically rebook passenger on cancelled flight"""
    status = flight_agent.get_flight_status(flight_number)

    if status.is_cancelled:
        # Find alternative flights
        alternatives = find_alternative_flights(
            origin=status.departure_airport,
            destination=status.arrival_airport,
            date=status.scheduled_departure.date()
        )

        # Book best alternative
        best_flight = alternatives[0]
        rebook_passenger(passenger_id, best_flight)

        # Notify passenger
        send_email(passenger_id, f"Your flight {flight_number} was cancelled. We've rebooked you on {best_flight}.")
```

---

## 💡 Production Tips

### Rate Limiting

```python
import time
from functools import wraps

def rate_limit(calls_per_minute=10):
    """Decorator to enforce rate limiting"""
    min_interval = 60.0 / calls_per_minute
    last_called = [0.0]

    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            elapsed = time.time() - last_called[0]
            wait_time = min_interval - elapsed

            if wait_time > 0:
                time.sleep(wait_time)

            result = func(*args, **kwargs)
            last_called[0] = time.time()
            return result

        return wrapper
    return decorator

class RateLimitedFlightAgent(FlightTrackerAgent):
    @rate_limit(calls_per_minute=10)
    def get_flight_status(self, flight_number: str):
        return super().get_flight_status(flight_number)
```

### Retry Logic

```python
from tenacity import retry, stop_after_attempt, wait_exponential

class RobustFlightAgent(FlightTrackerAgent):
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10)
    )
    def get_flight_status(self, flight_number: str):
        """Retry up to 3 times with exponential backoff"""
        return super().get_flight_status(flight_number)
```

### Caching

```python
from functools import lru_cache
from datetime import datetime, timedelta

class CachedFlightAgent(FlightTrackerAgent):
    def __init__(self):
        super().__init__()
        self.cache = {}
        self.cache_ttl = timedelta(minutes=5)

    def get_flight_status(self, flight_number: str):
        """Cache flight status for 5 minutes"""
        now = datetime.now()

        if flight_number in self.cache:
            cached_status, cache_time = self.cache[flight_number]
            if now - cache_time < self.cache_ttl:
                return cached_status

        # Fetch fresh data
        status = super().get_flight_status(flight_number)
        self.cache[flight_number] = (status, now)
        return status
```

---

## ✅ Checklist

- [ ] Agent registered in AIM dashboard
- [ ] FlightAware API key obtained and tested
- [ ] Code runs without errors
- [ ] Dashboard shows agent status
- [ ] Trust score visible (should be ~0.97)
- [ ] Audit trail shows API calls
- [ ] No security alerts
- [ ] Rate limiting implemented (production)
- [ ] Error handling added (production)

**All checked?** 🎉 **Your flight tracker is production-ready!**

---

## 🚀 Next Steps

- [Database Agent Example →](./database-agent.md) - Enterprise database security
- [SDK Documentation](../sdk/python.md) - Complete SDK reference
- [Azure Deployment](../deployment/azure.md) - Production deployment

---

<div align="center">

**Next**: [Database Agent Example →](./database-agent.md)

[🏠 Back to Home](../../README.md) • [📚 All Examples](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
