#!/usr/bin/env python3
"""
AIM Demo Agent - see your dashboard update in real time.

The demo now ships inside the SDK package. The canonical way to run it:

    pip install aim-sdk
    aim-sdk login
    aim-sdk demo                 # bounded scripted pass
    aim-sdk demo --interactive   # this menu experience
    aim-sdk demo --cleanup       # delete the demo agent again

This script remains for the offline ZIP download and simply opens the same
interactive menu.
"""

import sys

try:
    from aim_sdk.demo import run
except ImportError:
    print("""
================================================================================
                     ERROR: Could not import aim_sdk
================================================================================

Install the SDK first:

  pip install aim-sdk

Or, from the extracted offline ZIP:

  pip install -e .
  python demo_agent.py
================================================================================
""")
    sys.exit(1)

if __name__ == "__main__":
    sys.exit(run(interactive=True))
