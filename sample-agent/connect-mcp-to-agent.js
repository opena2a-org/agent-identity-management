/**
 * Connect MCP Server to Agent
 * 
 * This adds the MCP server to your agent's "talks_to" list
 */

const axios = require('axios');

const AIM_API_URL = 'http://localhost:8080';
const AGENT_ID = 'b5dc9a74-a98a-44b2-baba-b4814489ea33';
const AIM_API_KEY = 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM=';

async function connectMCPToAgent() {
    console.log('\n🔗 Connecting MCP to Agent');
    console.log('==========================\n');

    console.log('⚠️  Note: MCP connection via API requires JWT authentication.');
    console.log('The easiest way is to use the dashboard.\n');

    console.log('📋 Step-by-Step Guide:');
    console.log('=======================\n');
    
    console.log('1. Go to your agent page:');
    console.log(`   http://localhost:3000/dashboard/agents/${AGENT_ID}`);
    console.log('');
    
    console.log('2. Click the "Add MCP Servers" button');
    console.log('');
    
    console.log('3. You should see "Sample Agent MCP" in the list');
    console.log('');
    
    console.log('4. Check the box next to "Sample Agent MCP"');
    console.log('');
    
    console.log('5. Click "Add Selected" button');
    console.log('');
    
    console.log('6. Go back to the MCP page:');
    console.log('   http://localhost:3000/dashboard/mcp-servers');
    console.log('');
    
    console.log('7. Click on "Sample Agent MCP"');
    console.log('');
    
    console.log('8. You should now see "1 Connected Agent"!');
    console.log('');
    
    console.log('✨ Alternative: Let me try the SDK endpoint directly...\n');

    try {
        // Try to add MCP using SDK endpoint (this should work with API key)
        // We need to know the MCP ID, so let's try with a known name pattern
        console.log('🔍 Attempting to connect via SDK endpoint...');
        console.log('   (This requires knowing the MCP server ID)\n');

        const mcpServers = mcpListResponse.data.mcp_servers || mcpListResponse.data;
        
        if (!mcpServers || mcpServers.length === 0) {
            console.log('❌ No MCP servers found');
            console.log('\n💡 Make sure you created the MCP server first:');
            console.log('   1. Go to http://localhost:3000/dashboard/mcp-servers');
            console.log('   2. Click "Register MCP Server"');
            console.log('   3. Fill in the details');
            return;
        }

        console.log(`✅ Found ${mcpServers.length} MCP server(s):\n`);
        mcpServers.forEach((mcp, index) => {
            console.log(`${index + 1}. ${mcp.name} (ID: ${mcp.id})`);
        });

        // Find the sample-agent-mcp
        const sampleMCP = mcpServers.find(mcp => 
            mcp.name === 'sample-agent-mcp' || 
            mcp.name.includes('Sample Agent')
        );

        if (!sampleMCP) {
            console.log('\n❌ Could not find "sample-agent-mcp"');
            console.log('Available MCPs:', mcpServers.map(m => m.name).join(', '));
            console.log('\nUsing the first MCP instead...');
            var mcpToConnect = mcpServers[0];
        } else {
            var mcpToConnect = sampleMCP;
        }

        console.log(`\n🔌 Connecting "${mcpToConnect.name}" to agent...`);

        // Add MCP to agent using SDK endpoint
        const response = await axios.post(
            `${AIM_API_URL}/sdk/agents/${AGENT_ID}/mcp-servers`,
            {
                mcp_server_ids: [mcpToConnect.id]
            },
            {
                headers: {
                    'X-API-Key': AIM_API_KEY,
                    'Content-Type': 'application/json'
                }
            }
        );

        console.log('\n✅ MCP Connected Successfully!');
        console.log('==============================\n');
        console.log(`MCP: ${mcpToConnect.name}`);
        console.log(`Agent: ${AGENT_ID}`);
        console.log('');
        console.log('📊 View the connection:');
        console.log(`   Agent: http://localhost:3000/dashboard/agents/${AGENT_ID}`);
        console.log(`   MCP: http://localhost:3000/dashboard/mcp-servers/${mcpToConnect.id}`);
        console.log('');
        console.log('🎉 Your agent is now connected to the MCP!');
        console.log('   The MCP page should now show "1" connected agent.');
        console.log('');

    } catch (error) {
        console.error('\n❌ Error:', error.message);
        
        if (error.response) {
            console.error('Status:', error.response.status);
            console.error('Response:', JSON.stringify(error.response.data, null, 2));
            
            if (error.response.status === 401) {
                console.log('\n💡 Authentication failed. Check your API key.');
            } else if (error.response.status === 404) {
                console.log('\n💡 Agent or MCP not found. Check the IDs.');
            }
        }
    }
}

connectMCPToAgent();

