#!/usr/bin/env python3
"""
Get a fresh Python SDK download: the steps to sign in and download it from the
dashboard.

A fresh download starts with a sign-in. GET /api/v1/sdk/download refuses, with
401, an access token minted by refreshing a downloaded SDK's credential, and
downloads for a sign-in's access token, as the dashboard's SDK page sends. This
helper prints the dashboard steps; it sends no request and leaves ~/.aim as it
is.
"""

# The dashboard follows AIM_FRONTEND_PORT; see stack_env.py.
from stack_env import FRONTEND_URL

print("=" * 80)
print("GETTING A FRESH SDK DOWNLOAD")
print("=" * 80 + "\n")

print("A fresh SDK download starts with a sign-in. The backend does not download")
print("an SDK for an access token refreshed from a downloaded SDK's credential.")
print()
print(f"1. Open {FRONTEND_URL}/auth/login and sign in.")
print(f"2. Open {FRONTEND_URL}/dashboard/sdk and click 'Download Python SDK'.")
print("3. Extract the downloaded aim-sdk-python-v<version>.zip into ./fresh-sdk/.")
print()
print("The download carries a new SDK credential.")
