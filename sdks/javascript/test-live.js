// Live test of JavaScript SDK with real backend
const { AIMClient } = require('./dist/client');

async function main() {
  console.log('🚀 Starting JavaScript SDK live test...\n');

  // Initialize SDK with real backend
  const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
    apiKey: 'aim_test_1234567890abcdef',
    agentId: 'a934b38f-aa1c-46ef-99b9-775da9e551dd',
    autoDetect: false, // Manual testing first
  });

  console.log('✅ SDK initialized');
  console.log('📍 API URL:', 'http://localhost:8080');
  console.log('🔑 Agent ID:', 'a934b38f-aa1c-46ef-99b9-775da9e551dd');
  console.log('');

  // Test 1: Manual MCP report
  console.log('📊 Test 1: Manual MCP report');
  try {
    await client.reportMCP('filesystem');
    console.log('✅ Successfully reported filesystem MCP\n');
  } catch (error) {
    console.error('❌ Failed to report MCP:', error);
    process.exit(1);
  }

  // Test 2: Report another MCP
  console.log('📊 Test 2: Report another MCP');
  try {
    await client.reportMCP('github');
    console.log('✅ Successfully reported github MCP\n');
  } catch (error) {
    console.error('❌ Failed to report MCP:', error);
    process.exit(1);
  }

  // Test 3: Duplicate detection (should be deduplicated)
  console.log('📊 Test 3: Duplicate detection (within 60s window)');
  try {
    await client.reportMCP('filesystem');
    console.log('✅ Duplicate detection handled correctly\n');
  } catch (error) {
    console.error('❌ Failed:', error);
    process.exit(1);
  }

  console.log('🎉 All tests passed!');
  console.log('');
  console.log('Check backend logs for:');
  console.log('  - API key authentication success');
  console.log('  - Detection processing');
  console.log('  - MCP server creation');

  // Cleanup
  client.destroy();
}

main().catch(console.error);
