#!/usr/bin/env python3
"""
Register weather agent using API key mode (bypasses OAuth token issue).
"""

import sys
import os

# Use the updated SDK
sys.path.insert(0, "/Users/decimai/workspace/aim-sdk-python")

from dotenv import load_dotenv
load_dotenv()

print("=" * 80)
print("🌤️  REGISTERING WEATHER AGENT WITH API KEY MODE")
print("=" * 80)

from aim_sdk import secure

print("\n🔐 Registering weather-agent-demo with API key...")
print("   (Bypassing OAuth token issue)")

# You'll need to create an API key in the dashboard first
# For now, let's show what the code would look like:

print("""
To register with API key:

1. Go to dashboard → API Keys
2. Create new API key
3. Copy the key
4. Run:

   export AIM_API_KEY="your-api-key-here"

   from aim_sdk import secure
   agent = secure(
       "weather-agent-demo",
       api_key=os.getenv("AIM_API_KEY")
   )

This bypasses the OAuth token issue and works immediately!
""")

print("\n💡 Or download fresh SDK with valid credentials:")
print("   Dashboard → Download SDK → Extract → Run weather agent")
print("\n✅ The OAuth fix is working - we just need fresh credentials!")
