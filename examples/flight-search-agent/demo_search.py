#!/usr/bin/env python3
"""
Demo script to run flight searches and show results
This simulates interactive usage of the flight agent
"""

import sys
import os

# Add SDK to path. Repo checkout path first, dashboard-bundle path second.
_AGENT_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(_AGENT_DIR, '../../sdk/python'))
sys.path.insert(0, os.path.join(_AGENT_DIR, 'aim-sdk-python'))

def main():
    """Run demo flight searches"""
    from flight_agent import FlightAgent

    print("\n" + "="*80)
    print("FLIGHT AGENT DEMO - Searching for Flights to NYC")
    print("="*80 + "\n")

    # Create agent (uses existing registration)
    agent = FlightAgent()

    if not agent.client or not agent.agent_id:
        print("❌ Agent failed to initialize")
        return

    print(f"\n{'='*80}")
    print("SEARCHING FOR FLIGHTS TO NYC")
    print("="*80 + "\n")

    # Search for flights to NYC
    flights = agent.search_flights("NYC")

    # Display results
    agent.display_flights(flights)

    print(f"\n{'='*80}")
    print("DEMO COMPLETE")
    print("="*80)
    print("\n✅ Agent successfully:")
    print("   1. Registered with AIM")
    print("   2. Requested verification for flight search")
    print("   3. Executed flight search")
    print("   4. Logged results to AIM")
    print("\n💡 Check the dashboard at http://localhost:3000/dashboard")
    print("   - Agent appears in Agents list")
    print("   - Verification requests visible")
    print("   - Activity logged\n")

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
