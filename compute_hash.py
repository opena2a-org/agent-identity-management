import json
import hashlib

# Load credentials
with open('/Users/decimai/.aim/credentials.json') as f:
    creds = json.load(f)

refresh_token = creds['refresh_token']

# Compute SHA-256 hash exactly as backend does
token_hash = hashlib.sha256(refresh_token.encode()).hexdigest()

print(f"Refresh token (first 30): {refresh_token[:30]}...")
print(f"SHA-256 hash: {token_hash}")
