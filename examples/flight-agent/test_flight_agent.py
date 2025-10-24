#!/usr/bin/env python3
"""
Test script for the flight agent
This tests the complete flow: registration, verification, flight search
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

def test_flight_agent():
    """Test the flight agent end-to-end"""
    print("\n" + "="*60)
    print("TESTING FLIGHT AGENT - END-TO-END FLOW")
    print("="*60 + "\n")

    # Import the agent
    from flight_agent import FlightAgent

    # Create and initialize agent (this triggers registration)
    print("Step 1: Creating Flight Agent (will auto-register)...")
    agent = FlightAgent()

    if not agent.client or not agent.agent_id:
        print("❌ FAILED: Agent did not register successfully")
        return False

    print(f"✅ PASSED: Agent registered with ID: {agent.agent_id}\n")

    # Test flight search (this triggers verification and logging)
    print("Step 2: Testing flight search to NYC (will verify action)...")
    flights = agent.search_flights("NYC")

    if not flights:
        print("❌ FAILED: No flights returned")
        return False

    print(f"✅ PASSED: Found {len(flights)} flights\n")

    # Display results
    print("Step 3: Verifying flight results...")
    agent.display_flights(flights)

    print("\n" + "="*60)
    print("ALL TESTS PASSED!")
    print("="*60)
    print("\nNext steps:")
    print("1. Check dashboard at http://localhost:3000/dashboard")
    print("2. Verify agent appears in Agents page")
    print("3. Verify verification requests appear")
    print("4. Verify activity logs in Analytics\n")

    return True

if __name__ == "__main__":
    try:
        success = test_flight_agent()
        sys.exit(0 if success else 1)
    except Exception as e:
        print(f"\n❌ ERROR: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
