#!/usr/bin/env node

/**
 * Test Dangerous Operations Only
 * Demonstrates AIM blocking dangerous operations and maintaining security
 */

const SampleAIMAgent = require('./index');

async function testDangerousOperations() {
    console.log('⚠️  Testing Dangerous Operations Only');
    console.log('====================================\n');

    const agent = new SampleAIMAgent();

    // Initialize
    await agent.initialize();
    await agent.register();

    console.log('\n💀 Testing Dangerous Operations...\n');

    // Dangerous code execution (should be blocked)
    await agent.dangerousCodeExecution(`
        // This should be blocked by AIM
        console.log("Attempting to execute dangerous code...");
        process.exit(1); // This would be very bad!
    `);

    // Dangerous file deletion (should be blocked)
    await agent.dangerousFileDeletion('/etc/passwd'); // Critical system file

    // Another dangerous operation
    await agent.dangerousCodeExecution(`
        // Another dangerous attempt
        const fs = require('fs');
        fs.writeFileSync('/tmp/malicious-file', 'malicious content');
    `);

    console.log('\n✅ Dangerous operations test complete');
    console.log(`Final trust score: ${agent.client.trustScore}`);
    console.log(`Agent status: ${JSON.stringify(agent.client.getStatus(), null, 2)}`);
    console.log('\n🛡️ Notice: Trust score should be maintained since dangerous operations were blocked');
}

testDangerousOperations().catch(console.error);

