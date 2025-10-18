# 🔗 AIM Live Integration Guide

## 🚀 Quick Start - Connect to Running AIM Backend

Since your AIM platform is already running, you can connect the sample agent immediately!

### **Run the Live Integration Demo**

```bash
cd sample-agent
npm run connect
```

This will:
1. ✅ **Register** a new agent with your AIM backend
2. ✅ **Get credentials** (Agent ID + API Key) automatically
3. ✅ **Demonstrate** safe operations (approved)
4. ✅ **Demonstrate** dangerous operations (blocked)
5. ✅ **Show** agent status in dashboard

---

## 📋 What Happens During Connection

### **Step 1: Agent Registration**
```
🔐 Registering Agent with AIM Backend
✅ Generated Ed25519 keypair
📝 Registering with AIM backend...
   API URL: http://localhost:8080

✅ Agent Registered Successfully!
   Agent ID: [auto-generated UUID]
   API Key: [auto-generated key]
   Trust Score: 75 (starting score)
```

### **Step 2: Safe Operations**
```
📋 Performing Safe Operations
🔒 Safe Operation: Reading file...
   Verification: ✅ APPROVED
   ✅ File read successfully

🔒 Safe Operation: HTTP request...
   Verification: ✅ APPROVED
   ✅ HTTP request successful
```

### **Step 3: Dangerous Operations**
```
⚠️  Attempting Dangerous Operations
💀 Dangerous Operation: Code execution...
   🚫 BLOCKED by AIM
   Reason: High-risk operation

💀 Dangerous Operation: File deletion...
   🚫 BLOCKED by AIM
   Reason: Critical system operation
```

### **Step 4: Dashboard Link**
```
🌐 View your agent in the dashboard:
   http://localhost:3000/dashboard/agents/[your-agent-id]
```

---

## 🔑 No Manual Setup Required!

**The script handles everything automatically:**

✅ **No API Key needed** - Generated during registration  
✅ **No Agent ID needed** - Created by backend  
✅ **No Manual Config** - All automatic  
✅ **Cryptographic Signing** - Ed25519 keypair auto-generated  

---

## 🎯 What You'll See in AIM Dashboard

### **1. Agent Registry** (`/dashboard/agents`)
- **New Agent Listed**: `sample-connected-agent`
- **Status**: Verified ✅
- **Trust Score**: 75 (starting score)
- **Type**: AI Agent
- **Last Active**: Just now

### **2. Agent Details Page** (`/dashboard/agents/[id]`)
- **Overview**: Agent information and status
- **Trust Score**: Behavior-based risk assessment
- **Capabilities**: Detected capabilities
- **Activity Log**: All actions performed
- **Verification History**: Approved/blocked operations

### **3. Security Dashboard** (`/dashboard/security`)
- **Threats Detected**: Dangerous operations attempted
- **Risk Assessment**: Trust score changes
- **Security Alerts**: Blocked operations logged

### **4. Audit Logs** (`/dashboard/admin/audit-logs`)
- **Complete Trail**: Every action logged
- **Timestamps**: Precise timing
- **Risk Levels**: Color-coded by severity

---

## 🔧 Advanced Integration Options

### **Option 1: Use Existing Credentials**

If you already have an agent registered:

```javascript
const { AIMClient } = require('@opena2a/aim-sdk');

const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
    apiKey: 'your-existing-api-key',
    agentId: 'your-existing-agent-id'
});
```

### **Option 2: OAuth Registration**

For production with SSO:

```javascript
const registration = await client.registerAgentWithOAuth({
    name: 'my-agent',
    oauthProvider: 'google' // or 'microsoft', 'okta'
});
```

### **Option 3: Manual Registration via Dashboard**

1. Go to `http://localhost:3000/dashboard/agents`
2. Click "Register New Agent"
3. Fill in details
4. Copy API Key and Agent ID
5. Use in your code

---

## 📊 Understanding Trust Scores

### **How AIM Calculates Trust**

**8-Factor Algorithm:**
1. **Verification Status** (30%) - Cryptographic verification success
2. **Security Audit Score** (20%) - Passed security checks
3. **Community Trust** (15%) - Reviews and ratings
4. **Uptime & Availability** (15%) - Agent reliability
5. **Action Success Rate** (15%) - Operation success rate
6. **Security Alerts** (15%) - Incident history
7. **Compliance Score** (10%) - Policy adherence
8. **Age & History** (10%) - Behavioral consistency

### **Trust Score Ranges**
- **90-100**: Highly Trusted (green)
- **75-89**: Trusted (light green)
- **50-74**: Neutral (yellow)
- **25-49**: Low Trust (orange)
- **0-24**: Untrusted (red)

### **What Affects Trust Score**

**Increases Trust:**
- ✅ Successful safe operations
- ✅ Proper verification usage
- ✅ Consistent behavior
- ✅ No security incidents

**Decreases Trust:**
- ❌ Failed operations
- ❌ Attempting dangerous operations
- ❌ Verification failures
- ❌ Anomalous behavior

---

## 🛡️ Security Features Demonstrated

### **1. Ed25519 Cryptographic Signing**
- Military-grade digital signatures
- Every request cryptographically signed
- Prevents impersonation attacks

### **2. Action Verification**
- Real-time approval for all operations
- Risk-based decision making
- Automatic blocking of dangerous operations

### **3. Trust Scoring**
- Behavior-based risk assessment
- Continuous monitoring
- Adaptive security posture

### **4. Audit Logging**
- Complete activity trail
- Immutable records
- Compliance-ready

---

## 🚨 Troubleshooting

### **Issue: Connection Refused**
```
❌ Registration Failed: connect ECONNREFUSED
```

**Solution:**
- Check if backend is running: `curl http://localhost:8080/api/v1/health`
- Verify port 8080 is accessible
- Check firewall settings

### **Issue: 401 Unauthorized**
```
❌ Registration Failed: 401 Unauthorized
```

**Solution:**
- Backend authentication might be required
- Check if OAuth is configured
- Verify API endpoint is correct

### **Issue: 500 Internal Server Error**
```
❌ Registration Failed: 500 Internal Server Error
```

**Solution:**
- Check backend logs: `docker logs aim-backend`
- Verify database is running: `docker ps | grep postgres`
- Check migrations are applied

### **Issue: Agent Not Showing in Dashboard**
```
✅ Agent Registered Successfully!
But not visible in dashboard
```

**Solution:**
- Refresh dashboard page
- Check browser console for errors
- Verify frontend is running: `curl http://localhost:3000`
- Check if agent ID is correct

---

## 📝 Example Output

```bash
$ npm run connect

🎭 AIM Live Integration Demonstration
====================================

This demo will:
1. Register a new agent with AIM backend
2. Perform safe operations (approved)
3. Attempt dangerous operations (blocked)
4. Check agent status in AIM dashboard

🔐 Step 1: Registering Agent with AIM Backend
==============================================

✅ Generated Ed25519 keypair
   Public Key: 7Xk9mP2vQ4rT6wY8zN1bC3dF5g...
📝 Registering with AIM backend...
   API URL: http://localhost:8080

✅ Agent Registered Successfully!
   Agent ID: 550e8400-e29b-41d4-a716-446655440000
   API Key: aim_sk_1234567890abcdef...
   Trust Score: 75
✅ AIM Client initialized with credentials

📋 Step 2: Performing Safe Operations
=====================================

🔒 Safe Operation: Reading file...
   Verification: ✅ APPROVED
   ✅ File read successfully (1247 characters)

🔒 Safe Operation: HTTP request...
   Verification: ✅ APPROVED
   ✅ HTTP request successful (200)

⚠️  Step 3: Attempting Dangerous Operations
==========================================

💀 Dangerous Operation: Code execution...
   🚫 BLOCKED by AIM
   Reason: High-risk operation

💀 Dangerous Operation: File deletion...
   🚫 BLOCKED by AIM
   Reason: Critical system operation

📊 Step 4: Checking Agent Status in AIM
========================================

✅ Agent Status Retrieved:
   Name: sample-connected-agent
   Status: verified
   Trust Score: 77
   Created: 2025-10-18T07:45:23Z
   Last Active: Just now

🎉 Demo Complete!
=================

✅ Agent successfully registered with AIM
✅ Safe operations demonstrated
✅ Dangerous operations blocked
✅ Agent visible in AIM dashboard

🌐 View your agent in the dashboard:
   http://localhost:3000/dashboard/agents/550e8400-e29b-41d4-a716-446655440000

📊 Credentials saved:
   Agent ID: 550e8400-e29b-41d4-a716-446655440000
   API Key: aim_sk_1234567890abcdef...
   Public Key: 7Xk9mP2vQ4rT6wY8zN1bC3dF5g...
```

---

## 🎯 Next Steps

### **1. Explore Dashboard**
- View agent details
- Check trust score
- Review activity logs
- Monitor security alerts

### **2. Test More Operations**
- Modify `connect-to-aim.js`
- Add custom operations
- Test different risk levels
- Observe trust score changes

### **3. Production Integration**
- Use OAuth for authentication
- Implement proper error handling
- Add comprehensive logging
- Configure appropriate risk thresholds

### **4. Scale Up**
- Register multiple agents
- Monitor agent fleets
- Set up alerting
- Generate compliance reports

---

## 🌟 Key Takeaways

✅ **Zero Configuration** - Agent registers automatically  
✅ **Cryptographic Security** - Ed25519 signing built-in  
✅ **Real-time Verification** - Every action checked  
✅ **Trust Scoring** - Behavior-based risk assessment  
✅ **Complete Auditability** - All activities logged  
✅ **Dashboard Visibility** - Live monitoring and alerts  

**AIM provides enterprise-grade security for AI agents with minimal code changes!**

