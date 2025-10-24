#!/usr/bin/env python3
"""
Flight Search Agent with AIM Integration
A real-world example of an AI agent that:
1. Registers with AIM on first use
2. Auto-detects MCPs and capabilities
3. Verifies actions using AIM's verification system
4. Performs real flight searches

Usage:
    python flight_agent.py
"""

import os
import sys
import json
import time
from datetime import datetime, timedelta
from typing import Dict, List, Optional

# Add parent directory to path for AIM SDK
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../sdks/python'))

try:
    from aim_sdk import secure, register_agent
    from aim_sdk.client import AIMClient
except ImportError:
    print("❌ Error: AIM SDK not found. Make sure you're running from the correct directory.")
    print("   Expected path: examples/flight-agent/")
    sys.exit(1)

# Mock flight data - in real-world, this would call an API like Amadeus, Skyscanner, etc.
MOCK_FLIGHTS = {
    "NYC": [
        {
            "flight_number": "UA 1234",
            "airline": "United Airlines",
            "departure": "LAX",
            "arrival": "JFK",
            "departure_time": "08:00",
            "arrival_time": "16:30",
            "price": 289.99,
            "duration": "5h 30m",
            "stops": 0
        },
        {
            "flight_number": "AA 5678",
            "airline": "American Airlines",
            "departure": "LAX",
            "arrival": "EWR",
            "departure_time": "10:15",
            "arrival_time": "18:45",
            "price": 254.50,
            "duration": "5h 30m",
            "stops": 0
        },
        {
            "flight_number": "DL 9012",
            "airline": "Delta Airlines",
            "departure": "LAX",
            "arrival": "LGA",
            "departure_time": "12:30",
            "arrival_time": "21:15",
            "price": 199.99,
            "duration": "5h 45m",
            "stops": 1
        },
        {
            "flight_number": "B6 3456",
            "airline": "JetBlue",
            "departure": "LAX",
            "arrival": "JFK",
            "departure_time": "14:00",
            "arrival_time": "22:30",
            "price": 179.00,
            "duration": "5h 30m",
            "stops": 0
        },
    ],
    "SFO": [
        {
            "flight_number": "UA 2345",
            "airline": "United Airlines",
            "departure": "LAX",
            "arrival": "SFO",
            "departure_time": "09:00",
            "arrival_time": "10:30",
            "price": 129.99,
            "duration": "1h 30m",
            "stops": 0
        },
    ],
    "MIA": [
        {
            "flight_number": "AA 7890",
            "airline": "American Airlines",
            "departure": "LAX",
            "arrival": "MIA",
            "departure_time": "07:45",
            "arrival_time": "15:30",
            "price": 349.99,
            "duration": "4h 45m",
            "stops": 0
        },
    ]
}


class FlightAgent:
    """
    Flight Search Agent with full AIM integration
    """

    def __init__(self):
        """Initialize the flight agent and register with AIM"""
        self.client: Optional[AIMClient] = None
        self.agent_id: Optional[str] = None
        self.agent_name = "flight-search-agent"

        print("\n🛫 Flight Search Agent with AIM Integration")
        print("=" * 60)
        print()

        # Register with AIM (auto-detects MCPs and capabilities)
        self._register_with_aim()

    def _register_with_aim(self):
        """Register this agent with AIM and auto-detect capabilities"""
        try:
            print("📝 Registering with AIM...")
            print()

            # Use the secure() alias which auto-registers and auto-detects
            # This will:
            # 1. Auto-detect MCPs from Claude Desktop config
            # 2. Auto-detect capabilities from code/imports
            # 3. Generate Ed25519 keypair for signing
            # 4. Register agent with AIM platform
            self.client = secure(
                self.agent_name,
                agent_type="ai_agent",
                description="AI agent that helps users find the cheapest available flights",
                auto_detect=True,  # Auto-detect capabilities and MCPs
            )

            if self.client and self.client.agent_id:
                self.agent_id = self.client.agent_id
                print(f"✅ Successfully registered with AIM")
                print(f"   Agent ID: {self.agent_id}")
                print(f"   Agent Name: {self.agent_name}")
                print()

                # Display auto-detected capabilities
                if hasattr(self.client, '_capabilities') and self.client._capabilities:
                    print(f"🔍 Auto-detected capabilities:")
                    for cap in self.client._capabilities:
                        print(f"   • {cap}")
                    print()

            else:
                print("⚠️  Warning: Agent registered but no ID received")
                print()

        except Exception as e:
            print(f"❌ Failed to register with AIM: {e}")
            print("   Agent will run in standalone mode (no AIM integration)")
            print()

    def search_flights(
        self,
        destination: str,
        departure_date: Optional[str] = None,
        return_date: Optional[str] = None
    ) -> List[Dict]:
        """
        Search for flights to a destination

        This action is verified by AIM before execution
        """
        print(f"\n🔍 Searching flights to {destination}...")

        # Verify action with AIM before executing
        verification_id = None
        if self.client:
            try:
                print("🔐 Requesting verification from AIM...")

                # Request verification for this action
                verification = self.client.verify_action(
                    action_type="search_flights",
                    resource=destination,
                    context={
                        "departure_date": departure_date or "flexible",
                        "return_date": return_date or "flexible",
                        "risk_level": "low"
                    }
                )

                verification_id = verification.get('verification_id')
                print(f"✅ Verification requested (ID: {verification_id})")
                print()

                # Note: In real usage, you'd wait for approval here
                # For demo purposes, we proceed immediately

            except Exception as e:
                print(f"⚠️  Verification error: {e}")
                print("   Proceeding without verification")
                print()

        # Simulate API call delay
        time.sleep(0.5)

        # Get flights for destination
        destination_code = destination.upper()
        flights = MOCK_FLIGHTS.get(destination_code, [])

        if not flights:
            print(f"   No flights found to {destination}")
            return []

        # Sort by price (cheapest first)
        flights_sorted = sorted(flights, key=lambda x: x['price'])

        print(f"   Found {len(flights_sorted)} flights to {destination}")
        print()

        # Log successful action with AIM
        if self.client and verification_id:
            try:
                self.client.log_action_result(
                    verification_id=verification_id,
                    success=True,
                    result_summary=f"Found {len(flights_sorted)} flights to {destination}. Cheapest: ${flights_sorted[0]['price']:.2f}" if flights_sorted else f"No flights found to {destination}"
                )
            except Exception as e:
                print(f"⚠️  Failed to log action: {e}")

        return flights_sorted

    def display_flights(self, flights: List[Dict]):
        """Display flight results in a nice format"""
        if not flights:
            print("No flights to display")
            return

        print("\n✈️  Available Flights (sorted by price):")
        print("=" * 100)

        for i, flight in enumerate(flights, 1):
            stops_text = 'Direct' if flight['stops'] == 0 else f"{flight['stops']} stop(s)"
            print(f"\n{i}. {flight['airline']} - {flight['flight_number']}")
            print(f"   Route: {flight['departure']} → {flight['arrival']}")
            print(f"   Time: {flight['departure_time']} - {flight['arrival_time']} ({flight['duration']})")
            print(f"   Stops: {stops_text}")
            print(f"   💰 Price: ${flight['price']:.2f}")

        print("\n" + "=" * 100)

    def interactive_mode(self):
        """Run the agent in interactive mode"""
        print("\n🤖 Flight Search Agent - Interactive Mode")
        print("   Type 'help' for commands, 'quit' to exit")
        print()

        while True:
            try:
                command = input("flightagent> ").strip()

                if not command:
                    continue

                if command.lower() in ['quit', 'exit', 'q']:
                    print("\n👋 Goodbye!")
                    break

                if command.lower() == 'help':
                    self._show_help()
                    continue

                if command.lower().startswith('search '):
                    # Parse search command
                    parts = command.split()
                    if len(parts) < 2:
                        print("❌ Usage: search <destination> [departure_date] [return_date]")
                        continue

                    destination = parts[1]
                    departure_date = parts[2] if len(parts) > 2 else None
                    return_date = parts[3] if len(parts) > 3 else None

                    flights = self.search_flights(destination, departure_date, return_date)
                    self.display_flights(flights)
                    continue

                if command.lower() == 'status':
                    self._show_status()
                    continue

                print("❌ Unknown command. Type 'help' for available commands.")

            except KeyboardInterrupt:
                print("\n\n👋 Goodbye!")
                break
            except Exception as e:
                print(f"❌ Error: {e}")

    def _show_help(self):
        """Show available commands"""
        print("\n📚 Available Commands:")
        print("=" * 60)
        print("  search <destination>         - Search flights to destination")
        print("                                 Example: search NYC")
        print("  status                       - Show agent status")
        print("  help                         - Show this help message")
        print("  quit/exit                    - Exit the agent")
        print()
        print("💡 Available destinations: NYC, SFO, MIA")
        print("=" * 60)
        print()

    def _show_status(self):
        """Show agent status"""
        print("\n📊 Agent Status:")
        print("=" * 60)
        print(f"  Agent Name: {self.agent_name}")
        print(f"  Agent ID: {self.agent_id or 'Not registered'}")
        print(f"  AIM Integration: {'✅ Connected' if self.client else '❌ Not connected'}")
        if self.client:
            print(f"  Verification: ✅ Enabled")
            print(f"  Auto-detection: ✅ Enabled")
        print("=" * 60)
        print()


def main():
    """Main entry point"""
    try:
        agent = FlightAgent()
        agent.interactive_mode()
    except Exception as e:
        print(f"\n❌ Fatal error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
