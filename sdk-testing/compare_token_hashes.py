#!/usr/bin/env python3
"""
Compare token hashes to identify why SDK fails but curl succeeds.
"""

import json
import hashlib
from pathlib import Path

# Read the credentials file
creds_path = Path("/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json")
with open(creds_path) as f:
    creds = json.load(f)

token = creds['refresh_token']

# Compute SHA-256 hash (same as backend does)
hasher = hashlib.sha256()
hasher.update(token.encode('utf-8'))
token_hash = hasher.hexdigest()

print("=" * 80)
print("🔍 TOKEN HASH COMPARISON")
print("=" * 80)
print(f"\n📄 Credentials file: {creds_path}")
print(f"\n🔑 Token details:")
print(f"   - Token ID (JTI): {creds.get('sdk_token_id')}")
print(f"   - Token length: {len(token)} chars")
print(f"   - Token first 30 chars: {token[:30]}")
print(f"   - Token last 30 chars: {token[-30:]}")
print(f"\n🔒 SHA-256 hash:")
print(f"   {token_hash}")

# Also print the raw token for manual verification
print(f"\n📋 Full token (for manual curl test):")
print(token)
