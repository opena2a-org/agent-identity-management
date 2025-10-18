# Understanding "Recent Activity" in AIM

## What is "Recent Activity"?

The "Recent Activity" tab on your agent page shows:
- **Verification Events**: When actions are verified/blocked
- **Status Changes**: Agent verification, trust score updates
- **Security Events**: Anomalies, drift detection, alerts

## How Activity is Generated

### **Method 1: Dashboard Actions (Current)**

When you use the dashboard, activity is automatically logged:

✅ **Agent Creation** → Logged as creation event  
✅ **Agent Verification** → Logged as verification event  
✅ **MCP Connection** → Logged as connection event  
✅ **Trust Score Updates** → Logged automatically  
✅ **Manual Actions** → Any dashboard action creates activity  

### **Method 2: Runtime Verification (Requires JWT)**

The `/api/v1/agents/:id/verify-action` endpoint requires JWT authentication (user login), not API keys. This is by design for security.

**Why JWT and not API Key?**
- Action verification involves policy decisions
- Requires user/organization context
- Needs audit trail with user attribution
- Security-sensitive operations

### **Method 3: SDK Integration (Future)**

For production agents, the SDK would:
1. Register with API key
2. Get JWT token for runtime operations
3. Use JWT for action verification
4. Report activities in real-time

## Current Workaround

Since your agent is registered via API key, you can generate activity by:

### **Option A: Use the Dashboard**

1. Go to your agent page
2. Perform actions:
   - Edit agent details
   - Add/remove MCPs
   - Update capabilities
   - Change settings
3. Each action creates activity entries

### **Option B: Trigger Events via Dashboard**

1. **Add MCPs** → Creates "MCP Connected" event
2. **Remove MCPs** → Creates "MCP Disconnected" event  
3. **Edit Agent** → Creates "Agent Updated" event
4. **Verify Agent** → Creates "Verification" event

### **Option C: Wait for Real Usage**

When your agent actually runs and:
- Connects to MCPs
- Performs operations
- Triggers security policies
- Updates trust scores

These will automatically create activity entries.

## What You're Seeing Now

Your agent page shows:
- ✅ **Agent Created**: When you registered it
- ✅ **MCP Connected**: When you added the MCP
- ✅ **Trust Score: 64%**: Calculated based on configuration

**"No recent activity"** means:
- No runtime verification requests yet
- No policy violations
- No security events
- No manual updates via dashboard

## How to Generate Activity Right Now

### **Quick Test:**

1. **Go to your agent page**:
   ```
   http://localhost:3000/dashboard/agents/b5dc9a74-a98a-44b2-baba-b4814489ea33
   ```

2. **Click "Edit"** button

3. **Change something** (e.g., description)

4. **Save**

5. **Refresh** → You'll see "Agent Updated" in Recent Activity!

### **More Activity:**

1. **Add another MCP** → Creates activity
2. **Remove an MCP** → Creates activity
3. **Click "Verify"** button → Creates verification activity
4. **Update capabilities** → Creates activity

## Production Usage

In a real production scenario:

```javascript
// Agent performs action
const action = {
    type: 'read_file',
    resource: '/data/sensitive.csv'
};

// Agent calls AIM for verification (with JWT)
const decision = await aimClient.verifyAction(action);

if (decision.approved) {
    // Perform action
    // This creates activity in AIM
} else {
    // Block action
    // This also creates activity (blocked event)
}
```

## Summary

**For now, to see activity:**
1. Use the dashboard to make changes
2. Each change creates activity entries
3. Activity will populate as you use the system

**In production:**
- Agents would have JWT tokens
- Runtime verification would work
- Activity would be generated automatically
- Real-time monitoring would show live events

The system is working correctly - it just needs JWT authentication for runtime verification, which is a security best practice! 🔐


