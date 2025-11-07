# Direct API Usage (Without SDK)

**For users of Go, JavaScript, Ruby, or other languages until official SDKs are released.**

If you're not using the Python SDK, you can register and verify agents directly using the AIM REST API. This guide shows you how to implement the same security features using HTTP requests.

## Who Should Use This Guide?

- ✅ Developers using Go, JavaScript, Ruby, Java, or other languages
- ✅ Users who prefer direct API control
- ✅ Teams building custom integrations
- ✅ Anyone waiting for official SDKs (Go and JavaScript coming in 2026)

**Note**: Official SDKs planned for 2026:
- Go SDK (Q1-Q2 2026)
- JavaScript/TypeScript SDK (Q1-Q2 2026)

Until then, use the REST API directly as shown in this guide.

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ API key from AIM dashboard (Settings → API Keys)
3. ✅ Ed25519 cryptographic library for your language
4. ✅ HTTP client library

---

## Step 1: Generate Ed25519 Keypair

AIM uses Ed25519 cryptographic signatures for agent authentication. You need to generate a keypair.

### Go Example
```go
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "fmt"
)

func generateKeypair() (string, string, error) {
    publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return "", "", err
    }

    pubKeyB64 := base64.StdEncoding.EncodeToString(publicKey)
    privKeyB64 := base64.StdEncoding.EncodeToString(privateKey)

    return pubKeyB64, privKeyB64, nil
}

func main() {
    pubKey, privKey, err := generateKeypair()
    if err != nil {
        panic(err)
    }

    fmt.Printf("Public Key: %s\n", pubKey)
    fmt.Printf("Private Key: %s\n", privKey)
    fmt.Println("⚠️  Save your private key securely - it will NOT be retrievable!")
}
```

### JavaScript/Node.js Example
```javascript
const crypto = require('crypto');

function generateKeypair() {
    const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519', {
        publicKeyEncoding: { type: 'spki', format: 'der' },
        privateKeyEncoding: { type: 'pkcs8', format: 'der' }
    });

    const pubKeyB64 = publicKey.toString('base64');
    const privKeyB64 = privateKey.toString('base64');

    return { publicKey: pubKeyB64, privateKey: privKeyB64 };
}

const { publicKey, privateKey } = generateKeypair();
console.log('Public Key:', publicKey);
console.log('Private Key:', privateKey);
console.log('⚠️  Save your private key securely - it will NOT be retrievable!');
```

### Ruby Example
```ruby
require 'ed25519'
require 'base64'

signing_key = Ed25519::SigningKey.generate
verify_key = signing_key.verify_key

private_key = Base64.strict_encode64(signing_key.to_bytes)
public_key = Base64.strict_encode64(verify_key.to_bytes)

puts "Public Key: #{public_key}"
puts "Private Key: #{private_key}"
puts "⚠️  Save your private key securely - it will NOT be retrievable!"
```

**Important**: Save the private key securely! You'll need it for all agent operations.

---

## Step 2: Register Your Agent

Use the `/api/agents/register` endpoint to register your agent with AIM.

### API Endpoint
```
POST /api/agents/register
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

### Request Body
```json
{
  "name": "my-go-agent",
  "agent_type": "ai_agent",
  "public_key": "YOUR_BASE64_PUBLIC_KEY",
  "description": "My agent written in Go",
  "version": "1.0.0",
  "capabilities": ["read_database", "send_email"],
  "repository_url": "https://github.com/yourorg/your-agent",
  "documentation_url": "https://docs.yourorg.com/agent"
}
```

### Go Example
```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type RegisterAgentRequest struct {
    Name             string   `json:"name"`
    AgentType        string   `json:"agent_type"`
    PublicKey        string   `json:"public_key"`
    Description      string   `json:"description"`
    Version          string   `json:"version"`
    Capabilities     []string `json:"capabilities"`
    RepositoryURL    string   `json:"repository_url,omitempty"`
    DocumentationURL string   `json:"documentation_url,omitempty"`
}

type RegisterAgentResponse struct {
    AgentID    string  `json:"agent_id"`
    Name       string  `json:"name"`
    Status     string  `json:"status"`
    TrustScore float64 `json:"trust_score"`
    Message    string  `json:"message"`
}

func registerAgent(apiKey, publicKey string) (*RegisterAgentResponse, error) {
    reqBody := RegisterAgentRequest{
        Name:         "my-go-agent",
        AgentType:    "ai_agent",
        PublicKey:    publicKey,
        Description:  "My agent written in Go",
        Version:      "1.0.0",
        Capabilities: []string{"read_database", "send_email"},
    }

    jsonBody, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequest("POST", "http://localhost:8080/api/agents/register", bytes.NewBuffer(jsonBody))
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var response RegisterAgentResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return nil, err
    }

    return &response, nil
}

func main() {
    apiKey := "your-api-key"
    publicKey := "your-base64-public-key"

    response, err := registerAgent(apiKey, publicKey)
    if err != nil {
        panic(err)
    }

    fmt.Printf("✅ Agent registered successfully!\n")
    fmt.Printf("   Agent ID: %s\n", response.AgentID)
    fmt.Printf("   Trust Score: %.3f\n", response.TrustScore)
    fmt.Printf("   Status: %s\n", response.Status)
}
```

### JavaScript/Node.js Example
```javascript
const axios = require('axios');

async function registerAgent(apiKey, publicKey) {
    const response = await axios.post('http://localhost:8080/api/agents/register', {
        name: 'my-node-agent',
        agent_type: 'ai_agent',
        public_key: publicKey,
        description: 'My agent written in Node.js',
        version: '1.0.0',
        capabilities: ['read_database', 'send_email']
    }, {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${apiKey}`
        }
    });

    return response.data;
}

const apiKey = 'your-api-key';
const publicKey = 'your-base64-public-key';

registerAgent(apiKey, publicKey)
    .then(data => {
        console.log('✅ Agent registered successfully!');
        console.log('   Agent ID:', data.agent_id);
        console.log('   Trust Score:', data.trust_score);
        console.log('   Status:', data.status);
    })
    .catch(error => {
        console.error('❌ Registration failed:', error.response?.data || error.message);
    });
```

### cURL Example
```bash
curl -X POST http://localhost:8080/api/agents/register \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "name": "my-curl-agent",
    "agent_type": "ai_agent",
    "public_key": "YOUR_BASE64_PUBLIC_KEY",
    "description": "My agent registered via cURL",
    "version": "1.0.0",
    "capabilities": ["read_database", "send_email"]
  }'
```

### Response
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-go-agent",
  "status": "verified",
  "trust_score": 0.907,
  "message": "Agent registered successfully"
}
```

**Save the `agent_id`** - you'll need it for all subsequent operations!

---

## Step 3: Verify Actions Before Execution

Before performing sensitive operations, verify them with AIM for audit trail and security monitoring.

### API Endpoint
```
POST /api/agents/{agent_id}/verify
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
X-Agent-Signature: BASE64_SIGNATURE
```

### Signing the Request

You must sign each request with your Ed25519 private key.

#### Go Example - Sign and Verify
```go
package main

import (
    "crypto/ed25519"
    "encoding/base64"
    "encoding/json"
    "fmt"
)

type ActionRequest struct {
    ActionType    string                 `json:"action_type"`
    ActionDetails map[string]interface{} `json:"action_details"`
    ResourceName  string                 `json:"resource_name"`
    RiskLevel     string                 `json:"risk_level"`
}

func signRequest(privateKeyB64 string, payload interface{}) (string, error) {
    // Decode private key
    privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
    if err != nil {
        return "", err
    }
    privateKey := ed25519.PrivateKey(privateKeyBytes)

    // Serialize payload
    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return "", err
    }

    // Sign the payload
    signature := ed25519.Sign(privateKey, payloadJSON)

    // Encode signature as base64
    return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyAction(agentID, apiKey, privateKeyB64 string) error {
    actionReq := ActionRequest{
        ActionType: "database_query",
        ActionDetails: map[string]interface{}{
            "query": "SELECT * FROM users",
            "table": "users",
        },
        ResourceName: "users_table",
        RiskLevel:    "medium",
    }

    // Sign the request
    signature, err := signRequest(privateKeyB64, actionReq)
    if err != nil {
        return err
    }

    // Make API request
    jsonBody, _ := json.Marshal(actionReq)
    req, _ := http.NewRequest("POST",
        fmt.Sprintf("http://localhost:8080/api/agents/%s/verify", agentID),
        bytes.NewBuffer(jsonBody))

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("X-Agent-Signature", signature)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Parse response
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)

    if result["approved"].(bool) {
        fmt.Println("✅ Action approved - safe to execute")
        return nil
    } else {
        fmt.Printf("❌ Action denied: %s\n", result["reason"])
        return fmt.Errorf("action denied")
    }
}
```

#### JavaScript/Node.js Example
```javascript
const crypto = require('crypto');
const axios = require('axios');

function signRequest(privateKeyPem, payload) {
    const payloadJSON = JSON.stringify(payload);
    const sign = crypto.createSign('SHA512');
    sign.update(payloadJSON);
    sign.end();

    const signature = sign.sign(privateKeyPem);
    return signature.toString('base64');
}

async function verifyAction(agentId, apiKey, privateKey) {
    const actionReq = {
        action_type: 'database_query',
        action_details: {
            query: 'SELECT * FROM users',
            table: 'users'
        },
        resource_name: 'users_table',
        risk_level: 'medium'
    };

    const signature = signRequest(privateKey, actionReq);

    const response = await axios.post(
        `http://localhost:8080/api/agents/${agentId}/verify`,
        actionReq,
        {
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${apiKey}`,
                'X-Agent-Signature': signature
            }
        }
    );

    if (response.data.approved) {
        console.log('✅ Action approved - safe to execute');
        return true;
    } else {
        console.log('❌ Action denied:', response.data.reason);
        return false;
    }
}
```

### Response
```json
{
  "approved": true,
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "action_type": "database_query",
  "risk_assessment": {
    "risk_level": "medium",
    "confidence": 0.95
  },
  "verification_id": "ver_abc123xyz",
  "timestamp": "2025-11-06T18:00:00Z"
}
```

---

## Step 4: Log Action Results

After executing an action, log the result back to AIM for audit trail and trust score updates.

### API Endpoint
```
POST /api/agents/{agent_id}/actions/log
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
X-Agent-Signature: BASE64_SIGNATURE
```

### Request Body
```json
{
  "action_type": "database_query",
  "success": true,
  "duration_ms": 245,
  "result_summary": "Retrieved 150 user records",
  "metadata": {
    "rows_returned": 150,
    "execution_time": "245ms"
  }
}
```

### Go Example
```go
func logActionResult(agentID, apiKey, privateKeyB64 string, success bool) error {
    logReq := map[string]interface{}{
        "action_type": "database_query",
        "success":     success,
        "duration_ms": 245,
        "result_summary": "Retrieved 150 user records",
        "metadata": map[string]interface{}{
            "rows_returned":  150,
            "execution_time": "245ms",
        },
    }

    signature, _ := signRequest(privateKeyB64, logReq)
    jsonBody, _ := json.Marshal(logReq)

    req, _ := http.NewRequest("POST",
        fmt.Sprintf("http://localhost:8080/api/agents/%s/actions/log", agentID),
        bytes.NewBuffer(jsonBody))

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("X-Agent-Signature", signature)

    client := &http.Client{}
    resp, _ := client.Do(req)
    defer resp.Body.Close()

    fmt.Println("✅ Action logged successfully")
    return nil
}
```

---

## Complete Example: Full Agent Lifecycle

Here's a complete example showing the full agent lifecycle:

### Go - Complete Agent
```go
package main

import (
    "bytes"
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type AIMAgent struct {
    AgentID    string
    PublicKey  string
    PrivateKey string
    APIKey     string
    BaseURL    string
}

func NewAIMAgent(apiKey, baseURL string) (*AIMAgent, error) {
    // Generate keypair
    publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, err
    }

    return &AIMAgent{
        PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
        PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
        APIKey:     apiKey,
        BaseURL:    baseURL,
    }, nil
}

func (a *AIMAgent) Register(name string, capabilities []string) error {
    reqBody := map[string]interface{}{
        "name":         name,
        "agent_type":   "ai_agent",
        "public_key":   a.PublicKey,
        "capabilities": capabilities,
        "version":      "1.0.0",
    }

    jsonBody, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", a.BaseURL+"/api/agents/register", bytes.NewBuffer(jsonBody))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+a.APIKey)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    a.AgentID = result["agent_id"].(string)

    fmt.Printf("✅ Agent registered: %s\n", a.AgentID)
    return nil
}

func (a *AIMAgent) VerifyAction(actionType string, details map[string]interface{}) (bool, error) {
    actionReq := map[string]interface{}{
        "action_type":    actionType,
        "action_details": details,
        "risk_level":     "medium",
    }

    signature := a.signRequest(actionReq)
    jsonBody, _ := json.Marshal(actionReq)

    req, _ := http.NewRequest("POST",
        fmt.Sprintf("%s/api/agents/%s/verify", a.BaseURL, a.AgentID),
        bytes.NewBuffer(jsonBody))

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+a.APIKey)
    req.Header.Set("X-Agent-Signature", signature)

    client := &http.Client{}
    resp, _ := client.Do(req)
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)

    return result["approved"].(bool), nil
}

func (a *AIMAgent) signRequest(payload interface{}) string {
    privateKeyBytes, _ := base64.StdEncoding.DecodeString(a.PrivateKey)
    privateKey := ed25519.PrivateKey(privateKeyBytes)

    payloadJSON, _ := json.Marshal(payload)
    signature := ed25519.Sign(privateKey, payloadJSON)

    return base64.StdEncoding.EncodeToString(signature)
}

func main() {
    agent, _ := NewAIMAgent("your-api-key", "http://localhost:8080")

    // Register agent
    agent.Register("my-go-agent", []string{"read_database", "send_email"})

    // Verify action
    approved, _ := agent.VerifyAction("database_query", map[string]interface{}{
        "query": "SELECT * FROM users",
    })

    if approved {
        fmt.Println("✅ Action approved - executing query")
        // Execute your database query here
    }
}
```

---

## API Reference

### Complete Endpoint List

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agents/register` | POST | Register a new agent |
| `/api/agents/{id}` | GET | Get agent details |
| `/api/agents/{id}/verify` | POST | Verify an action before execution |
| `/api/agents/{id}/actions/log` | POST | Log action result |
| `/api/agents/{id}/trust-score` | GET | Get current trust score |
| `/api/agents/{id}/audit-trail` | GET | Get complete audit trail |
| `/api/auth/token/refresh` | POST | Refresh OAuth token (for SDK mode) |

For complete API documentation, see the [API Reference](./api-reference.md).

---

## Security Best Practices

1. **Never commit private keys** to version control
2. **Store private keys securely** in environment variables or secret managers
3. **Rotate keys regularly** (every 90 days recommended)
4. **Use HTTPS in production** - never send keys over HTTP
5. **Validate all responses** from AIM API
6. **Log all verification failures** for security monitoring
7. **Implement request signing** for all sensitive operations

---

## Coming Soon: Official SDKs

Official SDKs planned for 2026:

### Go SDK (Q1 2026)
```go
// Preview of future Go SDK
import "github.com/opena2a-org/aim-go-sdk"

agent := aim.Secure("my-go-agent")
// Same zero-config experience as Python!
```

### JavaScript/TypeScript SDK (Q2 2026)
```typescript
// Preview of future JS SDK
import { secure } from '@opena2a/aim-sdk';

const agent = secure('my-node-agent');
// Same zero-config experience as Python!
```

**Want to help build these SDKs?** We're looking for contributors! Join our [Discord](https://discord.gg/uRZa3KXgEn) or check [GitHub Issues](https://github.com/opena2a-org/agent-identity-management/issues).

---

## Troubleshooting

### Issue: "Invalid signature"

**Cause**: Request payload doesn't match signature

**Solution**:
```go
// Make sure you sign the EXACT JSON that you send
payload := map[string]interface{}{"action_type": "read"}
signature := signRequest(privateKey, payload)

// Send the SAME payload (don't modify it)
jsonBody, _ := json.Marshal(payload)  // Same object!
```

### Issue: "Agent not found"

**Cause**: Using wrong agent_id or agent not registered

**Solution**:
```bash
# Verify your agent exists
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/agents/YOUR_AGENT_ID
```

### Issue: "Unauthorized"

**Cause**: Invalid or expired API key

**Solution**:
1. Go to AIM Dashboard → Settings → API Keys
2. Create a new API key
3. Update your code with the new key

---

## Need Help?

- 💬 **Discord**: https://discord.gg/uRZa3KXgEn
- 📧 **Email**: support@opena2a.org
- 🐛 **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- 📚 **API Docs**: https://docs.opena2a.org/api

---

## Summary

**For non-Python users:**
1. Generate Ed25519 keypair
2. Register agent with `/api/agents/register`
3. Sign requests with private key
4. Verify actions with `/api/agents/{id}/verify`
5. Log results with `/api/agents/{id}/actions/log`

**Until official SDKs are released**, use the REST API directly. The experience is straightforward and well-documented.

**When SDKs are released in 2026**, you'll be able to migrate to the same zero-config experience as Python users enjoy today!
