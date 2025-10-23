import json

# The new refresh token from curl response
new_refresh_token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwib3JnYW5pemF0aW9uX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAxIiwiZW1haWwiOiJhZG1pbkBvcGVuYTJhLm9yZyIsInJvbGUiOiJhZG1pbiIsImlzcyI6ImFnZW50LWlkZW50aXR5LW1hbmFnZW1lbnQtc2RrIiwic3ViIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwiZXhwIjoxNzY4OTc3NzU3LCJuYmYiOjE3NjEyMDE3NTcsImlhdCI6MTc2MTIwMTc1NywianRpIjoiYTExODI4MTYtMDhmNi00ZTQ0LWE0OWYtOGM5ZWViNmZkMWFlIn0.1ygr4OXQ3MRMfdrkMT01JCwjmE1KgmMUAi2b_YcYj4U"

# Update SDK credentials
with open('/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json', 'r') as f:
    creds = json.load(f)

# Update refresh token and token_id
import base64
payload = new_refresh_token.split('.')[1]
padding = 4 - len(payload) % 4
if padding != 4:
    payload += '=' * padding
decoded = json.loads(base64.b64decode(payload))

creds['refresh_token'] = new_refresh_token
creds['sdk_token_id'] = decoded['jti']

# Save updated credentials
with open('/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json', 'w') as f:
    json.dump(creds, f, indent=2)

with open('/Users/decimai/.aim/credentials.json', 'w') as f:
    json.dump(creds, f, indent=2)

print(f"✅ Updated SDK credentials with new token")
print(f"New token ID: {decoded['jti']}")
print(f"Expires: {decoded['exp']}")
