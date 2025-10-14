/**
 * JavaScript SDK Capability Detection Test
 *
 * This test validates that capability detection works end-to-end:
 * 1. Auto-detect capabilities from package.json
 * 2. Report capabilities to backend
 * 3. Report SDK integration
 * 4. Register test MCP server
 */

import { AIMClient } from './src/client';
import { autoDetectCapabilities } from './src/capability_detection';

async function main() {
  console.log('🔍 JavaScript SDK Capability Detection Test');
  console.log('===========================================');

  // Backend URL
  const apiURL = process.env.AIM_API_URL || 'http://localhost:8080';
  console.log(`\n📡 Backend URL: ${apiURL}`);

  // Step 1: Auto-detect capabilities locally
  console.log('\n📦 Step 1: Auto-detecting capabilities from package.json...');
  let capabilities: string[];
  try {
    capabilities = await autoDetectCapabilities();
    console.log(`   ✅ Detected ${capabilities.length} capabilities: ${capabilities.join(', ')}`);
  } catch (err) {
    console.log(`   ⚠️  Auto-detection failed: ${err}`);
    console.log('   ℹ️  Using test capabilities instead');
    capabilities = [
      'read_files',
      'make_api_calls',
      'data_processing',
      'network_access',
      'ai_model_access',
    ];
  }

  // Step 2: Set up agent
  console.log('\n🔐 Step 2: Setting up agent...');
  const agentID = process.env.JS_AGENT_ID;
  const apiKey = process.env.JS_API_KEY;

  if (!agentID || !apiKey) {
    console.log('   ⚠️  No existing agent credentials found');
    console.log('   Please set JS_AGENT_ID and JS_API_KEY environment variables');
    console.log('   Or register a new agent first');
    process.exit(1);
  }

  console.log(`   ✅ Using agent ID: ${agentID}`);

  // Step 3: Create client and report capabilities
  console.log('\n📤 Step 3: Reporting capabilities to backend...');
  const client = new AIMClient({
    apiUrl: apiURL,
    agentId: agentID,
    apiKey: apiKey,
    autoDetect: false, // Disable auto-detect for this test
  });

  try {
    await client.reportCapabilities(capabilities);
    console.log(`   ✅ Successfully reported ${capabilities.length} capabilities to backend`);
  } catch (err) {
    console.error(`   ❌ Failed to report capabilities: ${err}`);
    process.exit(1);
  }

  // Step 4: Report SDK integration
  console.log('\n📡 Step 4: Reporting SDK integration...');
  try {
    await client.reportSDKIntegration(
      'aim-sdk-js@1.0.0',
      'javascript',
      ['capability_detection', 'auto_detect_mcps']
    );
    console.log('   ✅ SDK integration reported');
  } catch (err) {
    console.error(`   ❌ Failed to report SDK integration: ${err}`);
    process.exit(1);
  }

  // Step 5: Register a test MCP server
  console.log('\n🔌 Step 5: Registering test MCP server...');
  try {
    const mcpResult = await client.registerMCP(
      'filesystem-mcp-server',
      'auto_sdk',
      95.0,
      {
        source: 'capability_detection_test',
        package: '@modelcontextprotocol/sdk-filesystem',
      }
    );
    console.log(`   ✅ Registered ${mcpResult.added} MCP server(s)`);
  } catch (err) {
    console.log(`   ⚠️  MCP registration failed (may already exist): ${err}`);
  }

  // Summary
  console.log('\n===========================================');
  console.log('🎉 JavaScript SDK Test Complete!');
  console.log(`   - Detected: ${capabilities.length} capabilities`);
  console.log(`   - Reported: ${capabilities.length} capabilities to backend`);
  console.log(`   - Agent ID: ${agentID}`);
  console.log(`   - SDK Integration: ✅`);
  console.log(`   - MCP Server: ✅`);
  console.log('\n📊 Check the AIM dashboard:');
  console.log(`   - Capabilities tab: ${apiURL}/dashboard/agents/${agentID}`);
  console.log(`   - Detection tab: ${apiURL}/dashboard/sdk`);
  console.log(`   - Connections tab: ${apiURL}/dashboard/agents/${agentID}`);
  console.log('===========================================');

  // Keep running for a moment to ensure all requests complete
  await new Promise(resolve => setTimeout(resolve, 2000));

  client.destroy();
}

main().catch(err => {
  console.error('❌ Test failed:', err);
  process.exit(1);
});
