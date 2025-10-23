#!/usr/bin/env python3
import json
import base64
import time

with open('/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json') as f:
    creds = json.load(f)

token = creds['refresh_token']
parts = token.split('.')
payload = parts[1]

# Add padding
padding = 4 - len(payload) % 4
if padding != 4:
    payload += '=' * padding

decoded = json.loads(base64.b64decode(payload))
print('Token JTI:', decoded.get('jti'))
print('Token exp:', decoded.get('exp'))

exp_time = decoded.get('exp')
now = time.time()
days_remaining = (exp_time - now) / 86400
print(f'Days until expiration: {days_remaining:.1f}')
