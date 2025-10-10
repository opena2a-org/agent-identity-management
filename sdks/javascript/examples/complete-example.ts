/**
 * Complete Example: Agent Identity Management JavaScript SDK
 *
 * This example demonstrates all features of the AIM JavaScript SDK:
 * 1. Agent registration with Ed25519 signing
 * 2. OAuth registration (Google, Microsoft, Okta)
 * 3. Credential storage in system keyring
 * 4. Automatic MCP detection
 * 5. MCP server reporting
 *
 * Prerequisites:
 * - AIM backend running at http://localhost:8080
 * - Node.js 16+ installed
 * - System keyring access (may prompt for password)
 */

import { AIMClient, hasCredentials, loadCredentials, clearCredentials } from '../src';
import { autoDetectMCPs } from '../src/detection/capability-detection';
import { OAuthProvider } from '../src/oauth';

// Configuration
const API_URL = process.env.AIM_API_URL || 'http://localhost:8080';

/**
 * Example 1: Register a new agent with Ed25519 signing
 */
async function example1_RegisterAgent() {
  console.log('\n=== Example 1: Register New Agent ===\n');

  const client = new AIMClient({
    apiUrl: API_URL,
  });

  try {
    const registration = await client.registerAgent({
      name: 'my-javascript-agent',
      type: 'ai_agent',
      description: 'JavaScript SDK demo agent',
    });

    console.log('✅ Agent registered successfully!');
    console.log(`   Agent ID: ${registration.id}`);
    console.log(`   API Key: ${registration.apiKey.substring(0, 20)}...`);
    console.log(`   Public Key: ${registration.publicKey.substring(0, 40)}...`);
    console.log('   Credentials stored in system keyring');
  } catch (error) {
    console.error('❌ Registration failed:', error);
    throw error;
  }
}

/**
 * Example 2: Register agent with OAuth (Google)
 */
async function example2_RegisterWithOAuth() {
  console.log('\n=== Example 2: Register with OAuth (Google) ===\n');

  const client = new AIMClient({
    apiUrl: API_URL,
  });

  try {
    console.log('🌐 Opening browser for Google OAuth...');
    console.log('   Please authorize the application in your browser');
    console.log('   Listening for callback on http://localhost:8080/callback\n');

    const registration = await client.registerAgentWithOAuth({
      name: 'oauth-agent',
      type: 'ai_agent',
      oauthProvider: OAuthProvider.Google,
      redirectUrl: 'http://localhost:8080/callback',
    });

    console.log('✅ OAuth registration successful!');
    console.log(`   Agent ID: ${registration.id}`);
    console.log(`   Authenticated with Google`);
  } catch (error) {
    console.error('❌ OAuth registration failed:', error);
    throw error;
  }
}

/**
 * Example 3: Use existing agent from keyring
 */
async function example3_UseExistingAgent() {
  console.log('\n=== Example 3: Use Existing Agent ===\n');

  // Check if credentials exist
  const exists = await hasCredentials();
  if (!exists) {
    console.log('⚠️  No credentials found in keyring');
    console.log('   Please run Example 1 or 2 first to register an agent\n');
    return;
  }

  try {
    // Load client from stored credentials
    const client = await AIMClient.fromKeyring(API_URL);
    console.log('✅ Client loaded from keyring');

    // Get current credentials
    const creds = await loadCredentials();
    console.log(`   Agent ID: ${creds?.agentId}`);
    console.log(`   API Key: ${creds?.apiKey?.substring(0, 20)}...`);
  } catch (error) {
    console.error('❌ Failed to load credentials:', error);
    throw error;
  }
}

/**
 * Example 4: Auto-detect MCP servers
 */
async function example4_AutoDetectMCPs() {
  console.log('\n=== Example 4: Auto-Detect MCP Servers ===\n');

  try {
    const detection = await autoDetectMCPs();

    console.log(`✅ MCP detection complete`);
    console.log(`   Runtime: ${detection.runtime.runtime}`);
    console.log(`   Node Version: ${detection.runtime.node_version}`);
    console.log(`   Platform: ${detection.runtime.platform}`);
    console.log(`   Detected At: ${detection.detectedAt}\n`);

    if (detection.mcps.length === 0) {
      console.log('   No MCP servers found');
      console.log('   Create an MCP config file to test detection:');
      console.log('   - ~/.config/mcp/servers.json');
      console.log('   - ~/.mcp/config.json');
      console.log('   - ./mcp.json\n');
    } else {
      console.log(`   Found ${detection.mcps.length} MCP server(s):\n`);
      detection.mcps.forEach((mcp, index) => {
        console.log(`   ${index + 1}. ${mcp.name}`);
        console.log(`      Command: ${mcp.command}`);
        console.log(`      Capabilities: ${mcp.capabilities.join(', ')}`);
        console.log(`      Detected From: ${mcp.detectedFrom}\n`);
      });
    }

    return detection;
  } catch (error) {
    console.error('❌ MCP detection failed:', error);
    throw error;
  }
}

/**
 * Example 5: Report MCP servers to backend
 */
async function example5_ReportMCPs() {
  console.log('\n=== Example 5: Report MCP Servers ===\n');

  // Check if credentials exist
  const exists = await hasCredentials();
  if (!exists) {
    console.log('⚠️  No credentials found in keyring');
    console.log('   Please run Example 1 or 2 first to register an agent\n');
    return;
  }

  try {
    // Load client from keyring
    const client = await AIMClient.fromKeyring(API_URL);

    // Auto-detect and report all MCPs
    console.log('🔍 Auto-detecting MCP servers...');
    await client.autoDetectAndReport();

    console.log('✅ All MCPs reported successfully');
  } catch (error) {
    console.error('❌ Failed to report MCPs:', error);
    throw error;
  }
}

/**
 * Example 6: Manual MCP reporting
 */
async function example6_ManualMCPReporting() {
  console.log('\n=== Example 6: Manual MCP Reporting ===\n');

  const exists = await hasCredentials();
  if (!exists) {
    console.log('⚠️  No credentials found in keyring');
    console.log('   Please run Example 1 or 2 first to register an agent\n');
    return;
  }

  try {
    const client = await AIMClient.fromKeyring(API_URL);

    // Report specific MCP servers manually
    const mcpServers = ['filesystem', 'github', 'sequential-thinking'];

    console.log(`📡 Reporting ${mcpServers.length} MCP servers...`);
    for (const mcpName of mcpServers) {
      try {
        await client.reportMCP(mcpName);
        console.log(`   ✅ ${mcpName}`);
      } catch (error) {
        console.log(`   ❌ ${mcpName}: ${error}`);
      }
    }

    console.log('\n✅ Manual reporting complete');
  } catch (error) {
    console.error('❌ Failed to report MCPs:', error);
    throw error;
  }
}

/**
 * Example 7: Clear all credentials
 */
async function example7_ClearCredentials() {
  console.log('\n=== Example 7: Clear Credentials ===\n');

  const exists = await hasCredentials();
  if (!exists) {
    console.log('⚠️  No credentials found in keyring\n');
    return;
  }

  try {
    await clearCredentials();
    console.log('✅ All credentials cleared from keyring');
    console.log('   You will need to register again\n');
  } catch (error) {
    console.error('❌ Failed to clear credentials:', error);
    throw error;
  }
}

/**
 * Main function: Run all examples
 */
async function main() {
  console.log('╔════════════════════════════════════════════════════════════╗');
  console.log('║   Agent Identity Management - Complete Example            ║');
  console.log('║   JavaScript SDK Feature Demonstration                     ║');
  console.log('╚════════════════════════════════════════════════════════════╝');

  const args = process.argv.slice(2);
  const exampleNumber = args[0] ? parseInt(args[0]) : null;

  try {
    if (exampleNumber === 1) {
      await example1_RegisterAgent();
    } else if (exampleNumber === 2) {
      await example2_RegisterWithOAuth();
    } else if (exampleNumber === 3) {
      await example3_UseExistingAgent();
    } else if (exampleNumber === 4) {
      await example4_AutoDetectMCPs();
    } else if (exampleNumber === 5) {
      await example5_ReportMCPs();
    } else if (exampleNumber === 6) {
      await example6_ManualMCPReporting();
    } else if (exampleNumber === 7) {
      await example7_ClearCredentials();
    } else {
      // Run all examples in sequence
      console.log('\nRunning all examples in sequence...\n');
      console.log('Press Ctrl+C to stop at any time\n');

      // Example 1: Register agent
      await example1_RegisterAgent();
      await sleep(1000);

      // Example 3: Use existing agent
      await example3_UseExistingAgent();
      await sleep(1000);

      // Example 4: Auto-detect MCPs
      await example4_AutoDetectMCPs();
      await sleep(1000);

      // Example 5: Report MCPs
      await example5_ReportMCPs();
      await sleep(1000);

      // Example 6: Manual reporting
      await example6_ManualMCPReporting();
      await sleep(1000);

      // Note: OAuth example (2) and clear credentials (7) are not run by default
      console.log('\n📝 Additional examples:');
      console.log('   - Run `npm run example 2` for OAuth registration');
      console.log('   - Run `npm run example 7` to clear credentials\n');
    }

    console.log('╔════════════════════════════════════════════════════════════╗');
    console.log('║   ✅ Example completed successfully!                       ║');
    console.log('╚════════════════════════════════════════════════════════════╝\n');
  } catch (error) {
    console.error('\n❌ Example failed:', error);
    process.exit(1);
  }
}

// Helper function
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// Run examples if executed directly
if (require.main === module) {
  main().catch(console.error);
}

// Export for testing
export {
  example1_RegisterAgent,
  example2_RegisterWithOAuth,
  example3_UseExistingAgent,
  example4_AutoDetectMCPs,
  example5_ReportMCPs,
  example6_ManualMCPReporting,
  example7_ClearCredentials,
};
