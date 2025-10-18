/**
 * Create Sample MCP Servers
 * 
 * This creates sample MCP servers that you can connect to your agent.
 * No Claude Desktop required!
 */

const axios = require('axios');

const AIM_API_URL = 'http://localhost:8080';
const AGENT_ID = 'b5dc9a74-a98a-44b2-baba-b4814489ea33';

// Sample MCP servers to create
const SAMPLE_MCPS = [
    {
        name: 'filesystem-mcp',
        display_name: 'File System MCP',
        description: 'Provides file system access capabilities',
        url: 'http://localhost:3001',
        version: '1.0.0',
        capabilities: ['read_file', 'write_file', 'list_directory', 'create_directory']
    },
    {
        name: 'web-search-mcp',
        display_name: 'Web Search MCP',
        description: 'Enables web search capabilities',
        url: 'http://localhost:3002',
        version: '1.0.0',
        capabilities: ['search_web', 'fetch_url', 'extract_content']
    },
    {
        name: 'database-mcp',
        display_name: 'Database MCP',
        description: 'Database query and management',
        url: 'http://localhost:3003',
        version: '1.0.0',
        capabilities: ['query_database', 'execute_sql', 'list_tables']
    }
];

async function createSampleMCPs() {
    console.log('\n🔧 Creating Sample MCP Servers');
    console.log('==============================\n');

    console.log('⚠️  Note: This requires you to be logged into the dashboard');
    console.log('Because MCP creation requires JWT authentication.\n');
    console.log('Instead, let me show you how to create MCPs via the dashboard:\n');

    console.log('📋 Manual Steps to Create MCPs:');
    console.log('================================\n');

    SAMPLE_MCPS.forEach((mcp, index) => {
        console.log(`${index + 1}. Create "${mcp.display_name}":`);
        console.log(`   - Go to: http://localhost:3000/dashboard/mcp-servers`);
        console.log(`   - Click "Register MCP Server"`);
        console.log(`   - Name: ${mcp.name}`);
        console.log(`   - Display Name: ${mcp.display_name}`);
        console.log(`   - Description: ${mcp.description}`);
        console.log(`   - URL: ${mcp.url}`);
        console.log(`   - Capabilities: ${mcp.capabilities.join(', ')}`);
        console.log('');
    });

    console.log('\n📊 After Creating MCPs:');
    console.log('=======================\n');
    console.log('1. Go back to your agent page:');
    console.log(`   http://localhost:3000/dashboard/agents/${AGENT_ID}`);
    console.log('');
    console.log('2. Click "Add MCP Servers"');
    console.log('');
    console.log('3. Select the MCPs you just created');
    console.log('');
    console.log('4. Click "Add Selected"');
    console.log('');
    console.log('5. Your agent will now be connected to those MCPs!');
    console.log('');

    console.log('\n💡 What Are MCPs?');
    console.log('==================\n');
    console.log('MCP (Model Context Protocol) servers are tools/services that');
    console.log('AI agents can use to perform actions:');
    console.log('');
    console.log('• File System MCP: Read/write files, manage directories');
    console.log('• Web Search MCP: Search the internet, fetch web pages');
    console.log('• Database MCP: Query databases, manage data');
    console.log('• GitHub MCP: Interact with repositories');
    console.log('• Slack MCP: Send messages, manage channels');
    console.log('• And many more...');
    console.log('');

    console.log('\n🔐 What AIM Does With MCPs:');
    console.log('============================\n');
    console.log('Once MCPs are connected, AIM will:');
    console.log('');
    console.log('✅ Monitor which MCPs your agent uses');
    console.log('✅ Track MCP connection patterns');
    console.log('✅ Calculate trust scores based on MCP usage');
    console.log('✅ Detect anomalies (unexpected MCP usage)');
    console.log('✅ Log all MCP interactions for audit');
    console.log('✅ Alert on suspicious MCP communication');
    console.log('');

    console.log('\n🚀 Quick Start:');
    console.log('================\n');
    console.log('For now, just create 1-2 sample MCPs via the dashboard');
    console.log('to see how AIM tracks and monitors them!');
    console.log('');
}

createSampleMCPs();


