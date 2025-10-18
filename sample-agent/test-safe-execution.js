#!/usr/bin/env node

/**
 * Test Safe Operations Only
 * Demonstrates AIM approving safe operations and increasing trust score
 */

const SampleAIMAgent = require('./index');

async function testSafeOperations() {
    console.log('🧪 Testing Safe Operations Only');
    console.log('================================\n');

    const agent = new SampleAIMAgent();

    // Initialize
    await agent.initialize();
    await agent.register();

    console.log('\n📋 Testing Safe Operations...\n');

    // Safe file read
    await agent.safeFileRead('./README.md');

    // Safe HTTP request
    await agent.safeHttpRequest('https://httpbin.org/json');

    // Another safe file read
    await agent.safeFileRead('./package.json');

    console.log('\n✅ Safe operations test complete');
    console.log(`Final trust score: ${agent.client.trustScore}`);
    console.log(`Agent status: ${JSON.stringify(agent.client.getStatus(), null, 2)}`);
}

testSafeOperations().catch(console.error);

