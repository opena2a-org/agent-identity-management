/**
 * Auto-Detect MCPs from Claude Desktop Config
 * 
 * This is the easiest way to add MCPs to your agent.
 * It scans your Claude Desktop config and automatically registers all MCPs.
 */

const axios = require('axios');
const os = require('os');
const path = require('path');

// Your configuration
const AGENT_ID = 'b5dc9a74-a98a-44b2-baba-b4814489ea33'; // From your dashboard
const AIM_API_KEY = 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM=';
const AIM_API_URL = 'http://localhost:8080';

// Claude Desktop config path (default location)
const CLAUDE_CONFIG_PATH = path.join(
    os.homedir(),
    'Library/Application Support/Claude/claude_desktop_config.json'
);

async function autoDetectMCPs() {
    console.log('\n🔍 Auto-Detecting MCPs from Claude Desktop');
    console.log('==========================================\n');

    console.log(`Agent ID: ${AGENT_ID}`);
    console.log(`Config Path: ${CLAUDE_CONFIG_PATH}`);
    console.log('');

    try {
        // Note: For auto-detection, you need to use the dashboard (requires JWT auth)
        // For SDK-based MCP registration, use the SDK endpoint
        
        console.log('⚠️  Auto-detection requires dashboard access (JWT auth)');
        console.log('');
        console.log('Please use one of these methods:');
        console.log('');
        console.log('1. Dashboard Auto-Detect (Easiest):');
        console.log(`   - Go to: http://localhost:3000/dashboard/agents/${AGENT_ID}`);
        console.log('   - Click the "Auto-Detect MCPs" button');
        console.log('   - It will scan and add MCPs automatically');
        console.log('');
        console.log('2. Manual MCP Addition via SDK:');
        console.log('   - Use the SDK to register MCPs programmatically');
        console.log('   - Endpoint: POST /sdk/agents/:id/mcp-servers');
        console.log('   - Requires: API key authentication');
        console.log('');
        console.log('For now, let\'s add a sample MCP via the SDK endpoint...');
        console.log('');

        // Add MCP servers using SDK endpoint (API key auth)
        const sampleMCPs = [
            'filesystem-mcp',  // Example MCP server names
            'brave-search-mcp',
            'postgres-mcp'
        ];

        const response = await axios.post(
            `${AIM_API_URL}/sdk/agents/${AGENT_ID}/mcp-servers`,
            {
                mcp_server_ids: sampleMCPs
            },
            {
                headers: {
                    'X-API-Key': AIM_API_KEY,
                    'Content-Type': 'application/json'
                }
            }
        );

        const result = response.data;

        console.log('✅ MCP Detection Complete!');
        console.log('==========================\n');
        console.log(`Total MCPs Found: ${result.total_detected || 0}`);
        console.log(`New MCPs Registered: ${result.newly_registered || 0}`);
        console.log(`Already Registered: ${result.already_registered || 0}`);
        console.log('');

        if (result.detected_servers && result.detected_servers.length > 0) {
            console.log('Detected MCP Servers:');
            result.detected_servers.forEach((mcp, index) => {
                console.log(`\n${index + 1}. ${mcp.name}`);
                console.log(`   URL: ${mcp.url || mcp.command}`);
                console.log(`   Status: ${mcp.status || 'registered'}`);
                if (mcp.capabilities) {
                    console.log(`   Capabilities: ${mcp.capabilities.join(', ')}`);
                }
            });
        }

        console.log('\n📊 View your agent with MCPs:');
        console.log(`   http://localhost:3000/dashboard/agents/${AGENT_ID}`);

    } catch (error) {
        console.error('\n❌ Error:', error.message);
        
        if (error.response) {
            console.error('Status:', error.response.status);
            console.error('Response:', JSON.stringify(error.response.data, null, 2));
            
            if (error.response.status === 404) {
                console.log('\n💡 Tips:');
                console.log('1. Make sure Claude Desktop is installed');
                console.log('2. Check if the config file exists at:');
                console.log(`   ${CLAUDE_CONFIG_PATH}`);
                console.log('3. You may need to adjust the path for your OS');
            }
        }
    }
}

// Also provide manual MCP addition example
console.log('📚 Two Ways to Add MCPs:');
console.log('========================\n');
console.log('Option 1: Auto-Detect (Easiest)');
console.log('  - Scans Claude Desktop config');
console.log('  - Automatically registers all MCPs');
console.log('  - Run: node auto-detect-mcps.js');
console.log('');
console.log('Option 2: Manual via Dashboard');
console.log('  - Go to your agent page');
console.log('  - Click "Add MCP Servers"');
console.log('  - Select from existing MCPs or create new ones');
console.log('');
console.log('Option 3: Manual via SDK (Advanced)');
console.log('  - Register MCP with cryptographic signature');
console.log('  - Requires agent private key');
console.log('  - See register-mcp.js for example');
console.log('\n');

autoDetectMCPs();

