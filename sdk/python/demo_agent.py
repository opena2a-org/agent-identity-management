#!/usr/bin/env python3
"""
AIM Demo Agent - See Your Dashboard Update in Real-Time!

This interactive demo lets you perform actions and watch your AIM dashboard
update instantly. No API keys needed - just download the SDK and run!

SETUP (30 seconds):
  1. Download SDK from AIM dashboard (Settings -> SDK Download)
  2. Extract to any folder
  3. Run: python demo_agent.py

Then open your AIM dashboard and watch the magic happen!
"""

import sys
import time
import random
from datetime import datetime

# Banner
print("""
================================================================================
                     AIM DEMO AGENT - Interactive Demo
================================================================================

Watch your AIM dashboard update in real-time as you perform actions!

Dashboard: http://localhost:3000/dashboard/agents

================================================================================
""")

# Try to import the SDK
try:
    from aim_sdk import secure
except ImportError:
    print("ERROR: Could not import aim_sdk")
    print()
    print("Make sure you:")
    print("  1. Downloaded the SDK from AIM dashboard (Settings -> SDK Download)")
    print("  2. Extracted the ZIP file")
    print("  3. Are running this script from inside the extracted folder")
    print()
    print("Quick fix:")
    print("  cd aim-sdk-python")
    print("  pip install -e .")
    print("  python demo_agent.py")
    sys.exit(1)

# Register the demo agent
print("Registering demo agent with AIM...")
print()

try:
    agent = secure("demo-agent")
    print(f"Agent registered successfully!")
    print(f"  Agent ID: {agent.agent_id}")
    print(f"  AIM URL: {agent.aim_url}")
    print()
except Exception as e:
    print(f"ERROR: Could not register agent: {e}")
    print()
    print("Make sure:")
    print("  1. AIM is running (docker compose up -d)")
    print("  2. You downloaded the SDK from YOUR AIM dashboard")
    print("  3. The SDK has valid OAuth credentials embedded")
    print()
    print("Try downloading a fresh SDK from: http://localhost:3000/dashboard/sdk")
    sys.exit(1)


# Define demo actions with different risk levels
@agent.track_action(risk_level="low")
def check_weather(city: str) -> dict:
    """Simulate checking weather - LOW risk action"""
    conditions = ["Sunny", "Cloudy", "Rainy", "Windy", "Snowy"]
    return {
        "city": city,
        "temperature": random.randint(32, 95),
        "condition": random.choice(conditions),
        "humidity": random.randint(30, 90)
    }


@agent.track_action(risk_level="low")
def search_products(query: str) -> dict:
    """Simulate product search - LOW risk action"""
    return {
        "query": query,
        "results": random.randint(10, 500),
        "top_result": f"Best {query} - $" + str(random.randint(10, 200))
    }


@agent.track_action(risk_level="medium", resource="database:read")
def get_user_profile(user_id: str) -> dict:
    """Simulate reading user data - MEDIUM risk action"""
    return {
        "user_id": user_id,
        "name": f"User_{user_id}",
        "email": f"user_{user_id}@example.com",
        "created": "2024-01-15"
    }


@agent.track_action(risk_level="medium", resource="database:read")
def query_orders(user_id: str) -> dict:
    """Simulate querying orders - MEDIUM risk action"""
    return {
        "user_id": user_id,
        "total_orders": random.randint(1, 50),
        "total_spent": f"${random.randint(100, 5000)}"
    }


@agent.track_action(risk_level="high", resource="notification:send")
def send_notification(user_id: str, message: str) -> dict:
    """Simulate sending notification - HIGH risk action"""
    return {
        "user_id": user_id,
        "message": message,
        "status": "sent",
        "timestamp": datetime.now().isoformat()
    }


@agent.track_action(risk_level="high", resource="payment:process")
def process_refund(order_id: str, amount: float) -> dict:
    """Simulate processing refund - HIGH risk action"""
    return {
        "order_id": order_id,
        "amount": amount,
        "status": "processed",
        "refund_id": f"REF-{random.randint(10000, 99999)}"
    }


def print_menu():
    """Print the action menu"""
    print("""
================================================================================
                           CHOOSE AN ACTION
================================================================================

  LOW RISK (logged, minimal monitoring):
    1. Check Weather        - Simulate checking weather for a city
    2. Search Products      - Simulate searching for products

  MEDIUM RISK (logged, monitored for patterns):
    3. Get User Profile     - Simulate reading user data from database
    4. Query Orders         - Simulate querying order history

  HIGH RISK (logged, closely monitored, affects trust score):
    5. Send Notification    - Simulate sending a notification
    6. Process Refund       - Simulate processing a payment refund

  OTHER:
    7. Run All Actions      - Run all actions in sequence (great for demo!)
    8. Run 10 Random Actions- Bulk test with random actions
    0. Exit

================================================================================
""")


def run_action(choice: str):
    """Execute the selected action"""
    try:
        if choice == "1":
            city = input("Enter city name [San Francisco]: ").strip() or "San Francisco"
            print(f"\nChecking weather for {city}...")
            result = check_weather(city)
            print(f"  Result: {result['temperature']}F, {result['condition']}")

        elif choice == "2":
            query = input("Enter search query [laptop]: ").strip() or "laptop"
            print(f"\nSearching for '{query}'...")
            result = search_products(query)
            print(f"  Found {result['results']} results. Top: {result['top_result']}")

        elif choice == "3":
            user_id = input("Enter user ID [123]: ").strip() or "123"
            print(f"\nGetting profile for user {user_id}...")
            result = get_user_profile(user_id)
            print(f"  User: {result['name']} ({result['email']})")

        elif choice == "4":
            user_id = input("Enter user ID [123]: ").strip() or "123"
            print(f"\nQuerying orders for user {user_id}...")
            result = query_orders(user_id)
            print(f"  Orders: {result['total_orders']}, Total: {result['total_spent']}")

        elif choice == "5":
            user_id = input("Enter user ID [123]: ").strip() or "123"
            message = input("Enter message [Hello!]: ").strip() or "Hello!"
            print(f"\nSending notification to user {user_id}...")
            result = send_notification(user_id, message)
            print(f"  Status: {result['status']}")

        elif choice == "6":
            order_id = input("Enter order ID [ORD-001]: ").strip() or "ORD-001"
            amount = input("Enter refund amount [50.00]: ").strip() or "50.00"
            print(f"\nProcessing refund for order {order_id}...")
            result = process_refund(order_id, float(amount))
            print(f"  Refund ID: {result['refund_id']}, Status: {result['status']}")

        elif choice == "7":
            print("\nRunning all actions in sequence...\n")
            actions = [
                ("Check Weather", lambda: check_weather("New York")),
                ("Search Products", lambda: search_products("headphones")),
                ("Get User Profile", lambda: get_user_profile("user_456")),
                ("Query Orders", lambda: query_orders("user_456")),
                ("Send Notification", lambda: send_notification("user_456", "Your order shipped!")),
                ("Process Refund", lambda: process_refund("ORD-789", 29.99)),
            ]
            for name, action in actions:
                print(f"  Running: {name}...")
                action()
                time.sleep(0.5)  # Small delay so dashboard updates are visible
            print("\n  All actions completed!")

        elif choice == "8":
            print("\nRunning 10 random actions...\n")
            all_actions = [
                lambda: check_weather(random.choice(["NYC", "LA", "Chicago", "Miami", "Seattle"])),
                lambda: search_products(random.choice(["phone", "shoes", "camera", "book", "watch"])),
                lambda: get_user_profile(f"user_{random.randint(100, 999)}"),
                lambda: query_orders(f"user_{random.randint(100, 999)}"),
                lambda: send_notification(f"user_{random.randint(100, 999)}", "Test notification"),
                lambda: process_refund(f"ORD-{random.randint(1000, 9999)}", random.uniform(10, 100)),
            ]
            for i in range(10):
                action = random.choice(all_actions)
                print(f"  Action {i+1}/10...", end=" ")
                action()
                print("Done")
                time.sleep(0.3)
            print("\n  All 10 actions completed!")

        else:
            print("Invalid choice. Please try again.")
            return

        print("\n  Check your AIM dashboard to see this action logged!")
        print(f"  Dashboard: http://localhost:3000/dashboard/agents")

    except Exception as e:
        print(f"\n  ERROR: {e}")
        print("  Make sure AIM backend is running (docker compose up -d)")


def main():
    """Main loop"""
    print("READY! Open your AIM dashboard to watch actions in real-time.")
    print(f"Dashboard URL: http://localhost:3000/dashboard/agents")

    while True:
        print_menu()
        choice = input("Enter your choice (0-8): ").strip()

        if choice == "0":
            print("\nThanks for trying AIM! Check your dashboard for the full activity log.")
            print("Dashboard: http://localhost:3000/dashboard/agents")
            break

        run_action(choice)
        print()


if __name__ == "__main__":
    main()
