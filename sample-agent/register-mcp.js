/**
 * Register MCP Server for Sample Agent
 * 
 * This demonstrates how to:
 * 1. Create an MCP server registration
 * 2. Sign the registration with agent's private key
 * 3. Connect the MCP to the agent
 */

const { registerMCP } = require('@aim/sdk');
const crypto = require('crypto');

// Your agent details from the dashboard
const AGENT_ID = 'b5dc9a74-a98a-44b2-baba-b4814489ea33'; // From the screenshot
const AGENT_PRIVATE_KEY = 'YOUR_AGENT_PRIVATE_KEY_HERE'; // You need to get this from agent creation
const AIM_API_URL = 'http://localhost:8080';

// MCP Server details (example)
const MCP_SERVER = {
    name: 'filesystem-mcp',
    description: 'File system access MCP server',
    url: 'http://localhost:3001',
    version: '1.0.0',
    capabilities: ['read_file', 'write_file', 'list_directory']
};

async function registerMCPServer() {
    console.log('\n🔌 Registering MCP Server');
    console.log('=========================\n');

    try {
        // Generate MCP keypair (in real scenario, MCP would have its own keys)
        const mcpKeyPair = crypto.generateKeyPairSync('ed25519', {
            publicKeyEncoding: { type: 'spki', format: 'der' },
            privateKeyEncoding: { type: 'pkcs8', format: 'der' }
        });

        const publicKeyHex = mcpKeyPair.publicKey.toString('hex');

        console.log('MCP Server Details:');
        console.log(`  Name: ${MCP_SERVER.name}`);
        console.log(`  URL: ${MCP_SERVER.url}`);
        console.log(`  Capabilities: ${MCP_SERVER.capabilities.join(', ')}`);
        console.log(`  Public Key: ${publicKeyHex.substring(0, 20)}...`);
        console.log('');

        // Create signature message
        const timestamp = Date.now();
        const message = `register_mcp_server:${AGENT_ID}:${MCP_SERVER.name}:${MCP_SERVER.url}:${timestamp}`;
        
        console.log('⚠️  Note: You need the agent\'s private key to sign MCP registrations.');
        console.log('The private key was shown only once when you created the agent.');
        console.log('');
        console.log('For now, you can manually add MCPs via the dashboard:');
        console.log('1. Go to your agent page');
        console.log('2. Click "Add MCP Servers"');
        console.log('3. Or click "Auto-Detect MCPs" to scan Claude Desktop config');
        console.log('');

        // If you have the private key, uncomment this:
        /*
        const signature = crypto.sign(null, Buffer.from(message), {
            key: AGENT_PRIVATE_KEY,
            format: 'der',
            type: 'pkcs8'
        });

        const result = await registerMCP(AIM_API_URL, {
            agent_id: AGENT_ID,
            server_name: MCP_SERVER.name,
            server_url: MCP_SERVER.url,
            description: MCP_SERVER.description,
            version: MCP_SERVER.version,
            public_key: publicKeyHex,
            capabilities: MCP_SERVER.capabilities,
            signature: signature.toString('base64'),
            timestamp: timestamp
        });

        console.log('✅ MCP Server Registered!');
        console.log(`   Server ID: ${result.id}`);
        console.log(`   Name: ${result.name}`);
        console.log(`   Status: ${result.status}`);
        */

    } catch (error) {
        console.error('❌ Error:', error.message);
    }
}

registerMCPServer();


