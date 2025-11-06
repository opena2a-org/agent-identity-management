# 🌤️ Weather Agent Example - The Simplest Agent

Build a secure weather agent in **3 minutes**.

## What You'll Build

A simple AI agent that:
- ✅ Fetches real-time weather data from OpenWeatherMap API
- ✅ Automatically secured with AIM (1 line of code)
- ✅ Complete audit trail of all API calls
- ✅ Real-time trust scoring
- ✅ Security monitoring and alerts

**Difficulty**: Beginner
**Time**: 3 minutes
**Use Case**: Perfect for learning AIM basics

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ OpenWeatherMap API key ([Get free key](https://openweathermap.org/api))
3. ✅ Python 3.8+ installed
4. ✅ AIM SDK downloaded from dashboard ([Download Instructions](../quick-start.md#step-3-download-aim-sdk-and-install-dependencies-30-seconds))
   - **NO pip install available** - must download from dashboard
   - Dependencies: `pip install keyring PyNaCl requests cryptography`

---

## Step 1: Register Agent (30 seconds)

### In AIM Dashboard

1. **Login** to http://localhost:3000
2. **Navigate**: Agents → Register New Agent
3. **Fill in**:
   ```
   Agent Name: weather-agent
   Agent Type: AI Agent
   Description: Fetches weather data from OpenWeatherMap API
   ```
4. **Click** "Register Agent"
5. **Copy** the private key shown (only shown once!)

### Save Private Key

```bash
# Save to environment variable
export AIM_PRIVATE_KEY="your-private-key-here"

# Or add to .env file
echo "AIM_PRIVATE_KEY=your-private-key-here" >> .env
echo "OPENWEATHER_API_KEY=your-openweather-api-key" >> .env
```

---

## Step 2: Write the Agent (2 minutes)

Create `weather_agent.py`:

```python
"""
Weather Agent - Secured with AIM
Get real-time weather data for any city
"""

from aim_sdk import secure
import requests
import os
from typing import Dict, Optional

# 🔐 ONE LINE - Secure your agent!
agent = secure(
    name="weather-agent",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

class WeatherAgent:
    """Simple weather agent secured by AIM"""

    def __init__(self):
        self.api_key = os.getenv("OPENWEATHER_API_KEY")
        self.base_url = "https://api.openweathermap.org/data/2.5/weather"

    def get_weather(self, city: str, units: str = "imperial") -> Dict:
        """
        Get current weather for a city

        Args:
            city: City name (e.g., "San Francisco", "New York", "London")
            units: Temperature units ("imperial" for °F, "metric" for °C)

        Returns:
            Weather data dictionary

        Example:
            >>> weather = agent.get_weather("San Francisco")
            >>> print(f"Temperature: {weather['main']['temp']}°F")
        """
        # AIM automatically verifies this action before execution
        response = requests.get(
            self.base_url,
            params={
                "q": city,
                "appid": self.api_key,
                "units": units
            }
        )
        response.raise_for_status()
        return response.json()

    def get_temperature(self, city: str) -> float:
        """Get just the temperature for a city"""
        weather = self.get_weather(city)
        return weather['main']['temp']

    def get_forecast(self, city: str) -> str:
        """Get human-readable weather forecast"""
        weather = self.get_weather(city)

        temp = weather['main']['temp']
        feels_like = weather['main']['feels_like']
        description = weather['weather'][0]['description'].capitalize()
        humidity = weather['main']['humidity']
        wind_speed = weather['wind']['speed']

        return f"""
🌤️  Weather in {city}:
   Temperature: {temp}°F (feels like {feels_like}°F)
   Conditions: {description}
   Humidity: {humidity}%
   Wind: {wind_speed} mph
        """

    def is_good_weather(self, city: str, min_temp: float = 60, max_temp: float = 80) -> bool:
        """Check if weather is pleasant (good for outdoor activities)"""
        weather = self.get_weather(city)

        temp = weather['main']['temp']
        conditions = weather['weather'][0]['main']

        # Good weather = comfortable temperature + no rain/snow
        is_comfortable = min_temp <= temp <= max_temp
        is_clear = conditions in ["Clear", "Clouds"]

        return is_comfortable and is_clear


def main():
    """Demo the weather agent"""
    agent = WeatherAgent()

    # Example 1: Get basic weather
    print("📍 Example 1: Basic Weather")
    weather = agent.get_weather("San Francisco")
    print(f"Temperature: {weather['main']['temp']}°F")
    print(f"Conditions: {weather['weather'][0]['description']}")
    print()

    # Example 2: Get formatted forecast
    print("📍 Example 2: Formatted Forecast")
    forecast = agent.get_forecast("New York")
    print(forecast)

    # Example 3: Check if weather is good for outdoor activity
    print("📍 Example 3: Good Weather Check")
    is_good = agent.is_good_weather("Los Angeles")
    if is_good:
        print("✅ Great day for outdoor activities in LA!")
    else:
        print("❌ Maybe stay indoors in LA today")
    print()

    # Example 4: Compare temperatures across cities
    print("📍 Example 4: Temperature Comparison")
    cities = ["San Francisco", "New York", "Miami", "Seattle"]
    temps = {city: agent.get_temperature(city) for city in cities}

    print("Current temperatures:")
    for city, temp in sorted(temps.items(), key=lambda x: x[1], reverse=True):
        print(f"  {city}: {temp}°F")


if __name__ == "__main__":
    main()
```

---

## Step 3: Run It! (30 seconds)

```bash
# Make sure environment variables are set
export AIM_PRIVATE_KEY="your-key"
export OPENWEATHER_API_KEY="your-openweather-key"
export AIM_URL="http://localhost:8080"

# Run the agent
python weather_agent.py
```

**Expected Output**:
```
📍 Example 1: Basic Weather
Temperature: 62.5°F
Conditions: clear sky

📍 Example 2: Formatted Forecast

🌤️  Weather in New York:
   Temperature: 58.3°F (feels like 55.1°F)
   Conditions: Partly cloudy
   Humidity: 68%
   Wind: 8.5 mph

📍 Example 3: Good Weather Check
✅ Great day for outdoor activities in LA!

📍 Example 4: Temperature Comparison
Current temperatures:
  Miami: 78.2°F
  Los Angeles: 72.1°F
  San Francisco: 62.5°F
  New York: 58.3°F
```

---

## Step 4: Check Your Dashboard (Instant Feedback!)

Open http://localhost:3000 → Agents → weather-agent

### Agent Status

```
Agent: weather-agent
Status: ✅ ACTIVE
Trust Score: 0.95 (Excellent)
Last Verified: 15 seconds ago
Total Actions: 8
```

### Recent Activity

```
✅ get_weather("San Francisco")   |  15s ago  |  SUCCESS  |  Response: 245ms
✅ get_weather("New York")         |  12s ago  |  SUCCESS  |  Response: 198ms
✅ get_weather("Los Angeles")      |  10s ago  |  SUCCESS  |  Response: 212ms
✅ get_weather("Seattle")          |   8s ago  |  SUCCESS  |  Response: 223ms
```

### Trust Score Breakdown

```
✅ Verification Status:     100%  (1.00)  [Weight: 25%]
✅ Uptime & Availability:   100%  (1.00)  [Weight: 15%]
✅ Action Success Rate:     100%  (1.00)  [Weight: 15%]
✅ Security Alerts:           0   (1.00)  [Weight: 15%]
✅ Compliance Score:        100%  (1.00)  [Weight: 10%]
⚠️  Age & History:          New   (0.50)  [Weight: 10%]
✅ Drift Detection:         None  (1.00)  [Weight:  5%]
✅ User Feedback:           None  (1.00)  [Weight:  5%]

Overall Trust Score: 0.95 / 1.00
```

### Security Alerts

```
No security alerts. Your agent is behaving normally. ✅
```

### Audit Trail

```
📝 2025-10-21 14:32:15 UTC  |  Agent registered
📝 2025-10-21 14:35:42 UTC  |  Action verified: get_weather("San Francisco")
📝 2025-10-21 14:35:45 UTC  |  Action verified: get_weather("New York")
📝 2025-10-21 14:35:47 UTC  |  Action verified: get_weather("Los Angeles")
📝 2025-10-21 14:35:49 UTC  |  Action verified: get_weather("Seattle")
```

---

## 🎓 Understanding the Code

### What Does `secure()` Do?

```python
agent = secure(
    name="weather-agent",
    aim_url="http://localhost:8080",
    private_key=os.getenv("AIM_PRIVATE_KEY")
)
```

Behind this one line, AIM:
1. ✅ Creates cryptographic identity (Ed25519 keypair)
2. ✅ Registers agent with AIM platform
3. ✅ Enables automatic action verification
4. ✅ Starts real-time trust scoring
5. ✅ Begins audit logging
6. ✅ Monitors for security threats

### How Are Actions Verified?

Every time your agent calls an external API:
```python
response = requests.get("https://api.openweathermap.org/...")
```

AIM automatically:
1. **Captures** the action context (URL, parameters)
2. **Signs** the request with Ed25519 private key
3. **Verifies** the signature with AIM platform
4. **Logs** the action to audit trail
5. **Updates** trust score based on result
6. **Monitors** for anomalies

**Zero code changes required!**

### Trust Score Calculation

Your agent's trust score (0.95) is calculated from 8 factors:

1. **Verification Status** (25%): 100% of actions verified successfully
2. **Uptime** (15%): Agent always responsive
3. **Success Rate** (15%): 100% of actions succeeded
4. **Security Alerts** (15%): Zero alerts triggered
5. **Compliance** (10%): Following all security policies
6. **Age** (10%): New agent (score improves over time)
7. **Drift Detection** (5%): No behavioral anomalies
8. **User Feedback** (5%): No negative feedback

---

## 🚀 Advanced Usage

### Use with Async/Await

```python
import asyncio
from aim_sdk import secure
import aiohttp

agent = secure("weather-agent")

class AsyncWeatherAgent:
    """Async version for better performance"""

    async def get_weather(self, city: str) -> dict:
        """Async weather fetch"""
        async with aiohttp.ClientSession() as session:
            async with session.get(
                "https://api.openweathermap.org/data/2.5/weather",
                params={"q": city, "appid": os.getenv("OPENWEATHER_API_KEY")}
            ) as response:
                return await response.json()

    async def get_multiple_cities(self, cities: list[str]) -> dict:
        """Fetch weather for multiple cities in parallel"""
        tasks = [self.get_weather(city) for city in cities]
        results = await asyncio.gather(*tasks)
        return dict(zip(cities, results))

# Usage
agent = AsyncWeatherAgent()
weather_data = asyncio.run(agent.get_multiple_cities(["SF", "NYC", "LA"]))
```

### Add Error Handling

```python
from aim_sdk import secure
import requests
from typing import Optional

agent = secure("weather-agent")

def get_weather_safe(city: str) -> Optional[dict]:
    """Weather fetch with error handling"""
    try:
        response = requests.get(
            "https://api.openweathermap.org/data/2.5/weather",
            params={"q": city, "appid": os.getenv("OPENWEATHER_API_KEY")},
            timeout=5  # 5 second timeout
        )
        response.raise_for_status()
        return response.json()
    except requests.exceptions.Timeout:
        print(f"⚠️  Timeout fetching weather for {city}")
        return None
    except requests.exceptions.RequestException as e:
        print(f"❌ Error fetching weather for {city}: {e}")
        return None
```

### Cache Results

```python
from aim_sdk import secure
import requests
from functools import lru_cache
from datetime import datetime, timedelta

agent = secure("weather-agent")

class CachedWeatherAgent:
    """Weather agent with caching"""

    def __init__(self):
        self.cache = {}
        self.cache_duration = timedelta(minutes=10)

    def get_weather(self, city: str) -> dict:
        """Get weather with 10-minute cache"""
        # Check cache
        if city in self.cache:
            cached_data, cached_time = self.cache[city]
            if datetime.now() - cached_time < self.cache_duration:
                print(f"🔄 Using cached data for {city}")
                return cached_data

        # Fetch fresh data
        print(f"🌐 Fetching fresh data for {city}")
        response = requests.get(
            "https://api.openweathermap.org/data/2.5/weather",
            params={"q": city, "appid": os.getenv("OPENWEATHER_API_KEY")}
        )
        data = response.json()

        # Update cache
        self.cache[city] = (data, datetime.now())
        return data
```

---

## 💡 Real-World Use Cases

### 1. Slack Bot Integration

```python
from slack_bolt import App
from aim_sdk import secure

agent = secure("weather-slack-bot")
slack_app = App(token=os.getenv("SLACK_BOT_TOKEN"))

@slack_app.command("/weather")
def handle_weather(ack, command, respond):
    ack()
    city = command['text']

    # AIM verifies this action
    weather = get_weather(city)

    respond(f"🌤️  Weather in {city}: {weather['main']['temp']}°F")
```

### 2. Discord Bot

```python
import discord
from aim_sdk import secure

agent = secure("weather-discord-bot")
client = discord.Client()

@client.event
async def on_message(message):
    if message.content.startswith("!weather"):
        city = message.content.split(" ")[1]

        # AIM verifies this action
        weather = get_weather(city)

        await message.channel.send(
            f"🌤️  **{city}**: {weather['main']['temp']}°F - {weather['weather'][0]['description']}"
        )
```

### 3. Scheduled Reports

```python
from aim_sdk import secure
import schedule
import time

agent = secure("weather-reporter")

def daily_weather_report():
    """Send daily weather report"""
    cities = ["San Francisco", "New York", "London"]
    report = "📊 Daily Weather Report\n\n"

    for city in cities:
        # AIM verifies each action
        weather = get_weather(city)
        report += f"{city}: {weather['main']['temp']}°F - {weather['weather'][0]['description']}\n"

    print(report)
    # Send via email/Slack/SMS...

# Run daily at 8 AM
schedule.every().day.at("08:00").do(daily_weather_report)

while True:
    schedule.run_pending()
    time.sleep(60)
```

---

## 🐛 Troubleshooting

### Issue: "Invalid API key"

**Error**: `401 Unauthorized from OpenWeatherMap`

**Solution**:
1. Get free API key: https://openweathermap.org/api
2. Verify key is set: `echo $OPENWEATHER_API_KEY`
3. Wait 10 minutes for API key activation

### Issue: "Connection refused to AIM"

**Error**: `Connection refused to http://localhost:8080`

**Solution**:
```bash
# Check if AIM backend is running
docker ps | grep aim-backend

# Restart if needed
docker compose restart aim-backend
```

### Issue: "City not found"

**Error**: `404 from OpenWeatherMap`

**Solution**:
- Use exact city names: "San Francisco" not "SF"
- Try with country code: "London,UK"
- Check spelling

---

## ✅ Checklist

- [ ] Agent registered in AIM dashboard
- [ ] Private key saved securely
- [ ] OpenWeatherMap API key obtained
- [ ] Code runs without errors
- [ ] Dashboard shows agent status
- [ ] Trust score visible (should be ~0.95)
- [ ] Recent actions logged in audit trail
- [ ] No security alerts

**All checked?** 🎉 **Your weather agent is production-ready!**

---

## 🚀 Next Steps

### Explore More Examples

- [Flight Tracker Agent →](./flight-tracker.md) - Real-time flight tracking
- [Database Agent →](./database-agent.md) - Enterprise database security

### Learn Advanced Features

- [SDK Documentation](../sdk/python.md) - Complete SDK reference
- [Trust Scoring](../sdk/trust-scoring.md) - Deep dive into trust algorithm
- [Auto-Detection](../sdk/auto-detection.md) - MCP auto-discovery

### Deploy to Production

- [Azure Deployment](../deployment/azure.md) - Production Azure setup
- [Security Best Practices](../security/best-practices.md) - Harden your deployment

---

<div align="center">

**Next**: [Flight Tracker Agent →](./flight-tracker.md)

[🏠 Back to Home](../../README.md) • [📚 All Examples](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
