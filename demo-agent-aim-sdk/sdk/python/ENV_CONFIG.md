# AIM SDK Environment Variable Configuration

The AIM Python SDK supports automatic configuration through environment variables, enabling zero-configuration deployments and seamless CI/CD integration.

## 📋 Quick Start

```bash
# Minimal configuration (agent auto-registers if not found)
export AIM_AGENT_NAME="my-agent"
export AIM_URL="https://aim.example.com"

# Your code can now use AIM with zero configuration
python your_app.py
```

```python
# No manual configuration needed!
from aim_sdk import aim_verify

@aim_verify(auto_init=True)
def my_function():
    return "Hello, AIM!"
```

---

## 🔑 Environment Variables Reference

### Core Configuration

#### `AIM_AGENT_NAME` (Required)
**Description**: Name of your AI agent
**Type**: String
**Example**: `export AIM_AGENT_NAME="my-chatbot"`
**Notes**: Used for agent registration and credential lookup

#### `AIM_URL` (Optional)
**Description**: AIM backend server URL
**Type**: String
**Default**: `http://localhost:8080`
**Example**: `export AIM_URL="https://aim.example.com"`
**Notes**: Include protocol (http:// or https://)

### Advanced Configuration

#### `AIM_AUTO_REGISTER` (Optional)
**Description**: Automatically register agent if credentials not found
**Type**: Boolean
**Default**: `true`
**Example**: `export AIM_AUTO_REGISTER="false"`
**Values**:
- `true` - Auto-register new agents
- `false` - Fail if credentials not found (production mode)

#### `AIM_STRICT_MODE` (Optional)
**Description**: Block function execution if verification fails
**Type**: Boolean
**Default**: `false`
**Example**: `export AIM_STRICT_MODE="true"`
**Values**:
- `true` - Block execution on verification failure (production)
- `false` - Log warning and continue (development)

#### `AIM_CREDENTIALS_PATH` (Optional)
**Description**: Custom path for storing agent credentials
**Type**: String
**Default**: `~/.aim/credentials.json`
**Example**: `export AIM_CREDENTIALS_PATH="/etc/aim/creds.json"`
**Notes**: Useful for containerized environments

#### `AIM_LOG_LEVEL` (Optional)
**Description**: SDK logging verbosity
**Type**: String
**Default**: `INFO`
**Example**: `export AIM_LOG_LEVEL="DEBUG"`
**Values**: `DEBUG`, `INFO`, `WARNING`, `ERROR`, `CRITICAL`

---

## 🚀 Usage Examples

### Example 1: Development Environment

```bash
# .env file
AIM_AGENT_NAME=dev-chatbot
AIM_URL=http://localhost:8080
AIM_AUTO_REGISTER=true
AIM_STRICT_MODE=false
AIM_LOG_LEVEL=DEBUG
```

```python
# main.py
from aim_sdk import aim_verify

@aim_verify(auto_init=True, action_type="api_call")
def call_api(endpoint: str):
    return requests.get(f"https://api.example.com{endpoint}").json()

# No manual configuration needed!
result = call_api("/users")
```

### Example 2: Production Environment

```bash
# Production .env
AIM_AGENT_NAME=prod-chatbot
AIM_URL=https://aim.example.com
AIM_AUTO_REGISTER=false  # Must use pre-registered credentials
AIM_STRICT_MODE=true     # Block execution if verification fails
AIM_LOG_LEVEL=WARNING
AIM_CREDENTIALS_PATH=/etc/aim/credentials.json
```

```python
# production_app.py
from aim_sdk import secure, aim_verify

# Auto-loads from /etc/aim/credentials.json
@aim_verify(auto_init=True, action_type="database_query", risk_level="high")
def delete_user_data(user_id: str):
    db.execute("DELETE FROM users WHERE id = ?", user_id)

# Verification required before execution
delete_user_data("user123")
```

### Example 3: Docker Container

```dockerfile
# Dockerfile
FROM python:3.11-slim

# Install dependencies
RUN pip install aim-sdk

# Set AIM configuration
ENV AIM_AGENT_NAME=container-agent
ENV AIM_URL=https://aim.example.com
ENV AIM_CREDENTIALS_PATH=/app/.aim/credentials.json

# Copy pre-registered credentials
COPY credentials.json /app/.aim/credentials.json

# Run application
CMD ["python", "app.py"]
```

```python
# app.py
from aim_sdk import aim_verify

# Automatically configured from environment
@aim_verify(auto_init=True)
def process_request(data: dict):
    return {"processed": True, "data": data}
```

### Example 4: Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: v1
kind: Secret
metadata:
  name: aim-credentials
type: Opaque
stringData:
  credentials.json: |
    {
      "agent_id": "...",
      "private_key": "...",
      "public_key": "..."
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aim-agent
spec:
  template:
    spec:
      containers:
      - name: agent
        image: my-agent:latest
        env:
        - name: AIM_AGENT_NAME
          value: "k8s-agent"
        - name: AIM_URL
          value: "https://aim.example.com"
        - name: AIM_CREDENTIALS_PATH
          value: "/etc/aim/credentials.json"
        - name: AIM_STRICT_MODE
          value: "true"
        volumeMounts:
        - name: credentials
          mountPath: /etc/aim
          readOnly: true
      volumes:
      - name: credentials
        secret:
          secretName: aim-credentials
```

### Example 5: CI/CD Pipeline (GitHub Actions)

```yaml
# .github/workflows/deploy.yml
name: Deploy with AIM

on: [push]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Set up Python
        uses: actions/setup-python@v2
        with:
          python-version: 3.11

      - name: Install dependencies
        run: pip install aim-sdk

      - name: Configure AIM
        env:
          AIM_AGENT_NAME: ${{ secrets.AIM_AGENT_NAME }}
          AIM_URL: ${{ secrets.AIM_URL }}
          AIM_CREDENTIALS: ${{ secrets.AIM_CREDENTIALS }}
        run: |
          mkdir -p ~/.aim
          echo "$AIM_CREDENTIALS" > ~/.aim/credentials.json

      - name: Run tests with AIM verification
        run: pytest tests/
```

---

## 🔒 Security Best Practices

### 1. Never Commit Credentials
```bash
# ❌ WRONG - Don't put credentials in .env files
AIM_PRIVATE_KEY=ed25519_private_key_abc123...

# ✅ CORRECT - Use credential file with restricted permissions
chmod 600 ~/.aim/credentials.json
export AIM_CREDENTIALS_PATH=~/.aim/credentials.json
```

### 2. Use Strict Mode in Production
```bash
# Development: Allow execution even if verification fails
export AIM_STRICT_MODE=false

# Production: Block execution if verification fails
export AIM_STRICT_MODE=true
```

### 3. Separate Dev and Prod Agents
```bash
# Development agent
export AIM_AGENT_NAME=myapp-dev

# Production agent (different keys, higher trust score)
export AIM_AGENT_NAME=myapp-prod
```

### 4. Restrict Credential File Permissions
```bash
# Linux/Mac
chmod 600 ~/.aim/credentials.json
chown myuser:mygroup ~/.aim/credentials.json

# Kubernetes
# Use secrets with readOnly: true volume mounts
```

---

## 📦 Integration with Popular Tools

### Django
```python
# settings.py
import os

AIM_CONFIG = {
    'AGENT_NAME': os.getenv('AIM_AGENT_NAME'),
    'URL': os.getenv('AIM_URL', 'http://localhost:8080'),
    'STRICT_MODE': os.getenv('AIM_STRICT_MODE', 'true').lower() == 'true',
}
```

### Flask
```python
# app.py
from flask import Flask
from aim_sdk import aim_verify
import os

app = Flask(__name__)

@app.route('/users/<user_id>')
@aim_verify(auto_init=True, action_type="api_call")
def get_user(user_id):
    return {"user_id": user_id}
```

### FastAPI
```python
# main.py
from fastapi import FastAPI
from aim_sdk import aim_verify

app = FastAPI()

@app.get("/data")
@aim_verify(auto_init=True, action_type="api_call")
async def get_data():
    return {"data": "sensitive info"}
```

### LangChain
```python
# chain.py
from langchain import LLMChain
from aim_sdk.integrations.langchain import AIMCallbackHandler
import os

# Auto-configured from environment
handler = AIMCallbackHandler()  # Uses AIM_AGENT_NAME, AIM_URL

chain = LLMChain(llm=llm, callbacks=[handler])
```

---

## 🐳 Docker Compose Example

```yaml
# docker-compose.yml
version: '3.8'

services:
  aim-backend:
    image: aim-backend:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://aim:password@db:5432/aim
      REDIS_URL: redis://redis:6379

  my-agent:
    image: my-agent:latest
    environment:
      AIM_AGENT_NAME: my-agent
      AIM_URL: http://aim-backend:8080
      AIM_AUTO_REGISTER: "true"
      AIM_STRICT_MODE: "false"
      AIM_LOG_LEVEL: DEBUG
    depends_on:
      - aim-backend
```

---

## 🧪 Testing with Environment Variables

```python
# test_with_env.py
import os
import pytest
from aim_sdk import aim_verify

def test_aim_auto_init():
    """Test that AIM auto-initializes from environment"""
    os.environ['AIM_AGENT_NAME'] = 'test-agent'
    os.environ['AIM_URL'] = 'http://localhost:8080'
    os.environ['AIM_AUTO_REGISTER'] = 'true'

    @aim_verify(auto_init=True)
    def test_function():
        return "success"

    result = test_function()
    assert result == "success"
```

---

## ❓ Troubleshooting

### Error: "AIM client not provided and auto_init failed"
**Cause**: `AIM_AGENT_NAME` environment variable not set
**Solution**:
```bash
export AIM_AGENT_NAME="my-agent"
```

### Error: "Failed to connect to AIM backend"
**Cause**: `AIM_URL` points to unreachable server
**Solution**:
```bash
# Check if server is running
curl http://localhost:8080/health

# Update URL
export AIM_URL="http://localhost:8080"
```

### Error: "Agent credentials not found"
**Cause**: `AIM_AUTO_REGISTER=false` and no credentials file exists
**Solution**:
```bash
# Option 1: Enable auto-registration
export AIM_AUTO_REGISTER="true"

# Option 2: Register agent manually first
python -c "from aim_sdk import secure; secure('my-agent')"
```

---

## 📚 See Also

- [AIM SDK Documentation](../README.md)
- [Authentication Guide](./AUTHENTICATION.md)
- [Deployment Guide](./DEPLOYMENT.md)
- [API Reference](./API_REFERENCE.md)
