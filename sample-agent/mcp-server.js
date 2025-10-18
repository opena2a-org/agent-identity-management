/**
 * Simple MCP Server Example
 * 
 * This creates a basic MCP server that your agent can connect to.
 * It provides simple capabilities like file reading and text processing.
 */

const express = require('express');
const fs = require('fs').promises;
const path = require('path');

const app = express();
const PORT = 3001;

app.use(express.json());

// MCP Server Info
const MCP_INFO = {
    name: 'sample-agent-mcp',
    version: '1.0.0',
    description: 'Sample MCP server for demonstration',
    capabilities: [
        'read_file',
        'list_files',
        'echo',
        'get_time'
    ]
};

// Health check
app.get('/health', (req, res) => {
    res.json({ status: 'healthy', ...MCP_INFO });
});

// Get MCP info
app.get('/info', (req, res) => {
    res.json(MCP_INFO);
});

// Capability: Echo (simple test)
app.post('/capabilities/echo', (req, res) => {
    const { message } = req.body;
    res.json({
        capability: 'echo',
        input: message,
        output: `Echo: ${message}`,
        timestamp: new Date().toISOString()
    });
});

// Capability: Get current time
app.post('/capabilities/get_time', (req, res) => {
    res.json({
        capability: 'get_time',
        timestamp: new Date().toISOString(),
        unix: Date.now()
    });
});

// Capability: List files in a directory
app.post('/capabilities/list_files', async (req, res) => {
    try {
        const { directory } = req.body;
        const targetDir = directory || './';
        const files = await fs.readdir(targetDir);
        
        res.json({
            capability: 'list_files',
            directory: targetDir,
            files: files,
            count: files.length
        });
    } catch (error) {
        res.status(400).json({
            error: error.message
        });
    }
});

// Capability: Read file
app.post('/capabilities/read_file', async (req, res) => {
    try {
        const { filepath } = req.body;
        const content = await fs.readFile(filepath, 'utf-8');
        
        res.json({
            capability: 'read_file',
            filepath: filepath,
            content: content,
            size: content.length
        });
    } catch (error) {
        res.status(400).json({
            error: error.message
        });
    }
});

// Start server
app.listen(PORT, () => {
    console.log('\n🔌 Sample MCP Server Started');
    console.log('============================\n');
    console.log(`Server running on: http://localhost:${PORT}`);
    console.log(`Health check: http://localhost:${PORT}/health`);
    console.log(`Info: http://localhost:${PORT}/info`);
    console.log('');
    console.log('Available Capabilities:');
    MCP_INFO.capabilities.forEach(cap => {
        console.log(`  • ${cap}: POST /capabilities/${cap}`);
    });
    console.log('');
    console.log('📋 To register this MCP with AIM:');
    console.log('==================================\n');
    console.log('1. Go to: http://localhost:3000/dashboard/mcp-servers');
    console.log('2. Click "Register MCP Server"');
    console.log('3. Fill in:');
    console.log(`   - Name: sample-agent-mcp`);
    console.log(`   - Display Name: Sample Agent MCP`);
    console.log(`   - Description: ${MCP_INFO.description}`);
    console.log(`   - URL: http://localhost:${PORT}`);
    console.log(`   - Version: ${MCP_INFO.version}`);
    console.log(`   - Capabilities: ${MCP_INFO.capabilities.join(', ')}`);
    console.log('');
    console.log('4. Then go to your agent page and click "Add MCP Servers"');
    console.log('5. Select "sample-agent-mcp" and add it!');
    console.log('');
    console.log('🧪 Test the MCP:');
    console.log('=================\n');
    console.log('curl -X POST http://localhost:3001/capabilities/echo \\');
    console.log('  -H "Content-Type: application/json" \\');
    console.log('  -d \'{"message": "Hello from MCP!"}\'');
    console.log('');
});

// Graceful shutdown
process.on('SIGINT', () => {
    console.log('\n\n👋 Shutting down MCP server...');
    process.exit(0);
});


