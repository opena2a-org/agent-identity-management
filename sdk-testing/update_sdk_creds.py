import json

# New refresh token from successful curl rotation
new_refresh_token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwib3JnYW5pemF0aW9uX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAxIiwiZW1haWwiOiJhZG1pbkBvcGVuYTJhLm9yZyIsInJvbGUiOiJhZG1pbiIsImlzcyI6ImFnZW50LWlkZW50aXR5LW1hbmFnZW1lbnQtc2RrIiwic3ViIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwiZXhwIjoxNzY4OTc4NjYwLCJuYmYiOjE3NjEyMDI2NjAsImlhdCI6MTc2MTIwMjY2MCwianRpIjoiNzAyZjM0OTMtZWRjYy00MjQzLTg0M2QtNzA1MDE1OGMyZWQ4In0.SLfzr3U60MRCR6HJ3Mkyj_clANXKc7wqGlCdpR4FfXQ"

# Update SDK package credentials
with open('/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json', 'r') as f:
    creds = json.load(f)

import base64
payload = new_refresh_token.split('.')[1]
padding = 4 - len(payload) % 4
if padding != 4:
    payload += '=' * padding
decoded = json.loads(base64.b64decode(payload))

creds['refresh_token'] = new_refresh_token
creds['sdk_token_id'] = decoded['jti']

with open('/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json', 'w') as f:
    json.dump(creds, f, indent=2)

print(f"✅ Updated SDK credentials with rotated token")
print(f"New token ID: {decoded['jti']}")
