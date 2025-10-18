# How to Add MCPs to Your Agent

Your agent is registered! Now let's add MCP servers to it.

## Agent Details
- **Agent ID**: `b5dc9a74-a98a-44b2-baba-b4814489ea33`
- **Agent Name**: `sample-agent-1760777001362`
- **Dashboard URL**: http://localhost:3000/dashboard/agents/b5dc9a74-a98a-44b2-baba-b4814489ea33

## Method 1: Auto-Detect MCPs (Easiest) ⭐

1. **Open your agent page**:
   - Go to: http://localhost:3000/dashboard/agents/b5dc9a74-a98a-44b2-baba-b4814489ea33

2. **Click "Auto-Detect MCPs"** (blue button at the top)

3. **The system will**:
   - Scan your Claude Desktop config file
   - Find all configured MCPs
   - Automatically register them
   - Connect them to your agent

4. **Refresh the page** to see your MCPs!

## Method 2: Manual Addition via Dashboard

1. **Go to MCP Servers page**:
   - http://localhost:3000/dashboard/mcp-servers

2. **Click "Register MCP Server"**

3. **Fill in the details**:
   - Name: e.g., "filesystem-mcp"
   - URL: e.g., "http://localhost:3001"
   - Capabilities: e.g., "read_file, write_file, list_directory"

4. **Go back to your agent page**

5. **Click "Add MCP Servers"**

6. **Select the MCP** you just created

## Method 3: SDK Integration (Advanced)

If you want to programmatically register MCPs from your agent code, use the SDK:

```javascript
const { AIMClient } = require('@aim/sdk');

const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
    agentId: 'b5dc9a74-a98a-44b2-baba-b4814489ea33',
    apiKey: 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM='
});

// Register an MCP
await client.registerMCP({
    name: 'my-custom-mcp',
    url: 'http://localhost:3001',
    capabilities: ['read_file', 'write_file']
});
```

## What Happens After Adding MCPs?

Once MCPs are connected, AIM will:

✅ **Monitor MCP Usage**: Track which MCPs your agent uses
✅ **Real-time Connections**: See active MCP connections
✅ **Trust Score Updates**: Trust score adjusts based on MCP usage patterns
✅ **Anomaly Detection**: Alert if agent uses unexpected MCPs
✅ **Audit Logging**: Log all MCP interactions
✅ **Security Analysis**: Detect suspicious MCP communication patterns

## Common MCPs to Add

Here are some popular MCPs you might want to add:

- **filesystem**: Read/write files on your system
- **brave-search**: Search the web
- **postgres**: Query PostgreSQL databases
- **github**: Interact with GitHub repositories
- **slack**: Send Slack messages
- **puppeteer**: Browser automation
- **memory**: Persistent memory for agents

## Next Steps

1. Add MCPs using Method 1 (Auto-Detect)
2. Go back to your agent page
3. Click on the "Connections" tab
4. You should see your MCPs listed!
5. Run your agent and watch the dashboard update in real-time

## Troubleshooting

**No MCPs detected?**
- Make sure Claude Desktop is installed
- Check if you have MCPs configured in Claude Desktop
- Config file location: `~/Library/Application Support/Claude/claude_desktop_config.json`

**Can't add MCPs?**
- Make sure you're logged into the dashboard
- Check that your agent is verified (green checkmark)
- Try refreshing the page

**Need help?**
- Check the agent details page for more options
- Look at the audit logs for any errors
- Review the security dashboard for alerts


