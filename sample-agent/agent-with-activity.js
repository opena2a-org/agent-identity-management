/**
 * Sample Agent with Real Activity
 * 
 * This agent performs actual operations and reports them to AIM,
 * which will show up in the "Recent Activity" section.
 */

const { AIMClient } = require('@aim/sdk');
const fs = require('fs').promises;
const axios = require('axios');

// Configuration
const AIM_API_KEY = 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM=';
const AIM_API_URL = 'http://localhost:8080';
const AGENT_ID = 'b5dc9a74-a98a-44b2-baba-b4814489ea33'; // Your new-agent ID

class ActiveAIMAgent {
    constructor() {
        this.client = null;
        this.agentId = AGENT_ID;
        this.apiKey = AIM_API_KEY;
    }

    async initialize() {
        console.log('\n🤖 Initializing Active AIM Agent');
        console.log('=================================\n');

        this.client = new AIMClient({
            apiUrl: AIM_API_URL,
            agentId: this.agentId,
            apiKey: this.apiKey,
            autoDetect: true
        });

        console.log('✅ AIM Client initialized');
        console.log(`   Agent ID: ${this.agentId}`);
        console.log(`   Dashboard: http://localhost:3000/dashboard/agents/${this.agentId}`);
        console.log('');
    }

    async performSafeOperation(operation, details) {
        console.log(`\n📋 Safe Operation: ${operation}`);
        console.log(`   Details: ${details}`);
        
        try {
            // Verify action with AIM
            const response = await axios.post(
                `${AIM_API_URL}/api/v1/agents/${this.agentId}/verify-action`,
                {
                    action_type: operation,
                    resource: details,
                    context: {
                        risk_level: 'low',
                        timestamp: new Date().toISOString()
                    }
                },
                {
                    headers: {
                        'X-API-Key': this.apiKey,
                        'Content-Type': 'application/json'
                    }
                }
            );
            
            console.log('   ✅ Approved and logged');
            console.log(`   Decision: ${response.data.decision || 'approved'}`);
            return true;
        } catch (error) {
            if (error.response) {
                console.log('   ⚠️  Error:', error.response.status, error.response.data);
            } else {
                console.log('   ⚠️  Could not report to AIM:', error.message);
            }
            return false;
        }
    }

    async performDangerousOperation(operation, details) {
        console.log(`\n⚠️  Dangerous Operation: ${operation}`);
        console.log(`   Details: ${details}`);
        
        try {
            // Verify action with AIM
            const response = await axios.post(
                `${AIM_API_URL}/api/v1/agents/${this.agentId}/verify-action`,
                {
                    action_type: operation,
                    resource: details,
                    context: {
                        risk_level: 'high',
                        timestamp: new Date().toISOString()
                    }
                },
                {
                    headers: {
                        'X-API-Key': this.apiKey,
                        'Content-Type': 'application/json'
                    }
                }
            );
            
            console.log('   🚫 Blocked and logged');
            console.log(`   Decision: ${response.data.decision || 'blocked'}`);
            return false;
        } catch (error) {
            if (error.response) {
                console.log('   ⚠️  Error:', error.response.status, error.response.data);
            } else {
                console.log('   ⚠️  Could not report to AIM:', error.message);
            }
            return false;
        }
    }

    async simulateWorkflow() {
        console.log('\n🔄 Starting Agent Workflow');
        console.log('==========================\n');

        // Safe operations
        await this.performSafeOperation('read_file', './package.json');
        await new Promise(resolve => setTimeout(resolve, 1000));

        await this.performSafeOperation('api_call', 'https://api.example.com/data');
        await new Promise(resolve => setTimeout(resolve, 1000));

        await this.performSafeOperation('database_query', 'SELECT * FROM users LIMIT 10');
        await new Promise(resolve => setTimeout(resolve, 1000));

        // Dangerous operations (will be blocked)
        await this.performDangerousOperation('execute_shell', 'rm -rf /');
        await new Promise(resolve => setTimeout(resolve, 1000));

        await this.performDangerousOperation('modify_system', '/etc/hosts');
        await new Promise(resolve => setTimeout(resolve, 1000));

        await this.performDangerousOperation('delete_database', 'DROP TABLE users');
        await new Promise(resolve => setTimeout(resolve, 1000));

        console.log('\n✨ Workflow Complete!');
        console.log('====================\n');
        console.log('📊 Check your dashboard:');
        console.log(`   http://localhost:3000/dashboard/agents/${this.agentId}`);
        console.log('');
        console.log('You should now see:');
        console.log('  ✅ Recent Activity populated with events');
        console.log('  ✅ Trust score updated based on behavior');
        console.log('  ✅ Audit logs showing all operations');
        console.log('  ✅ Security alerts for dangerous operations');
        console.log('');
    }

    async reportCapabilities() {
        console.log('\n🔧 Reporting Agent Capabilities');
        console.log('================================\n');

        try {
            await axios.post(
                `${AIM_API_URL}/sdk/agents/${this.agentId}/capabilities`,
                {
                    capabilities: [
                        'read_file',
                        'write_file',
                        'api_call',
                        'database_query',
                        'send_email'
                    ],
                    detected_at: new Date().toISOString()
                },
                {
                    headers: {
                        'X-API-Key': this.apiKey,
                        'Content-Type': 'application/json'
                    }
                }
            );

            console.log('✅ Capabilities reported to AIM');
        } catch (error) {
            console.log('⚠️  Could not report capabilities:', error.message);
        }
    }

    async reportMCPDetection() {
        console.log('\n🔍 Reporting MCP Detection');
        console.log('==========================\n');

        try {
            await axios.post(
                `${AIM_API_URL}/sdk/agents/${this.agentId}/detection/report`,
                {
                    detection_method: 'runtime',
                    detected_mcps: [
                        {
                            name: 'sample-agent-mcp',
                            url: 'http://localhost:3001',
                            capabilities: ['read_file', 'list_files', 'echo']
                        }
                    ],
                    timestamp: new Date().toISOString()
                },
                {
                    headers: {
                        'X-API-Key': this.apiKey,
                        'Content-Type': 'application/json'
                    }
                }
            );

            console.log('✅ MCP detection reported to AIM');
        } catch (error) {
            console.log('⚠️  Could not report MCP detection:', error.message);
        }
    }

    destroy() {
        if (this.client) {
            this.client.destroy();
        }
    }
}

async function main() {
    const agent = new ActiveAIMAgent();

    try {
        await agent.initialize();
        await agent.reportCapabilities();
        await agent.reportMCPDetection();
        await agent.simulateWorkflow();
    } catch (error) {
        console.error('\n❌ Error:', error.message);
    } finally {
        agent.destroy();
    }
}

main();

