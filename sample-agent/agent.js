/**
 * Simple AIM Sample Agent
 * Demonstrates agent registration and monitoring using the AIM JavaScript SDK
 */

const { registerAgent, AIMClient } = require('@aim/sdk');

// Configuration
const AIM_API_KEY = process.env.AIM_API_KEY || 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM=';
const AIM_API_URL = process.env.AIM_API_URL || 'http://localhost:8080';
const AIM_DASHBOARD_URL = process.env.AIM_DASHBOARD_URL || 'http://localhost:3000';

async function main() {
    console.log('\n🤖 AIM Sample Agent');
    console.log('===================\n');

    try {
        // Step 1: Register the agent
        console.log('📝 Registering agent with AIM...');
        
        const agent = await registerAgent(AIM_API_URL, {
            name: `sample-agent-${Date.now()}`,
            type: 'ai_agent',
            description: 'Sample agent demonstrating AIM SDK integration',
            version: '1.0.0',
            apiKey: AIM_API_KEY
        });

        console.log('✅ Agent registered successfully!');
        console.log(`   Agent ID: ${agent.id}`);
        console.log(`   Name: ${agent.name}`);
        console.log(`   Status: ${agent.status || 'active'}`);
        console.log(`   Trust Score: ${agent.trustScore || 'N/A'}`);

        // Step 2: Initialize AIM Client
        console.log('\n🔐 Initializing AIM client...');
        
        const client = new AIMClient({
            apiUrl: AIM_API_URL,
            agentId: agent.id,
            apiKey: AIM_API_KEY,
            autoDetect: true
        });

        console.log('✅ AIM client initialized');

        // Step 3: Simulate operations
        console.log('\n📋 Simulating agent operations...');
        console.log('   ✅ Safe operation: Reading file');
        console.log('   ✅ Safe operation: API call');
        console.log('   🚫 Dangerous operation: System command (blocked)');

        // Step 4: Show dashboard link
        console.log('\n🎉 Success!');
        console.log(`\n📊 View your agent: ${AIM_DASHBOARD_URL}/dashboard/agents/${agent.id}`);
        console.log('\nYour agent is now monitored by AIM for:');
        console.log('  • Real-time behavior tracking');
        console.log('  • Trust score calculation');
        console.log('  • Security threat detection');
        console.log('  • Audit trail logging\n');

        client.destroy();

    } catch (error) {
        console.error('\n❌ Error:', error.message);
        if (error.response) {
            console.error('Response:', error.response.status, error.response.data);
        }
        process.exit(1);
    }
}

main();

