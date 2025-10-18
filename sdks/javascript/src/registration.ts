import axios from 'axios';
import { KeyPair } from './signing';
import { storeCredentials, storeOAuthToken } from './credentials';
import {
  OAuthProvider,
  getAuthorizationUrl,
  startCallbackServer,
  exchangeCodeForToken,
  openBrowser,
} from './oauth';

export interface RegisterOptions {
  name: string;
  type?: string;
  description?: string;
  version?: string;
  repositoryUrl?: string;
  documentationUrl?: string;
  apiKey?: string;
  oauthProvider?: OAuthProvider;
  redirectUrl?: string;
}

export interface AgentRegistration {
  id: string;
  name: string;
  apiKey: string;
  publicKey: string;
  privateKey?: string; // Only returned on initial registration
  trustScore?: number;
  status?: string;
}

/**
 * Secure is an alias for registerAgent
 * One-line agent registration
 */
export async function secure(
  apiUrl: string,
  options: RegisterOptions
): Promise<AgentRegistration> {
  return registerAgent(apiUrl, options);
}

/**
 * Register a new agent with the AIM backend
 * 
 * IMPORTANT: Agent registration requires a valid API key from a registered user.
 * 
 * To get an API key:
 * 1. Register/login to the AIM dashboard
 * 2. Navigate to Dashboard → API Keys
 * 3. Generate a new API key
 * 4. Pass the API key in the options.apiKey field
 * 
 * The backend generates Ed25519 keypairs automatically and returns credentials.
 * 
 * @param apiUrl - The AIM backend URL (e.g., http://localhost:8080)
 * @param options - Registration options including name, type, and API key
 * @returns Agent registration details including credentials (private key only returned once!)
 */
export async function registerAgent(
  apiUrl: string,
  options: RegisterOptions & { apiKey: string }
): Promise<AgentRegistration> {
  const { name, type = 'ai_agent', apiKey } = options;

  if (!apiKey) {
    throw new Error(
      'API key is required for agent registration. ' +
      'Get your API key from the AIM dashboard: Dashboard → API Keys'
    );
  }

  // Prepare registration payload
  const payload: Record<string, any> = {
    name,
    display_name: name,
    description: options.description || `${name} - AI Agent`,
    agent_type: type,
    version: options.version || '1.0.0',
    repository_url: options.repositoryUrl || '',
    documentation_url: options.documentationUrl || '',
  };

  // Send registration request with API key
  const response = await axios.post(`${apiUrl}/api/v1/public/agents/register`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-AIM-API-Key': apiKey,
    },
  });

  const result = response.data;

  // Extract credentials from response
  const agentRegistration: AgentRegistration = {
    id: result.agent_id || result.agentID,
    name: result.name,
    apiKey: apiKey, // Use the same API key for subsequent operations
    publicKey: result.public_key || result.publicKey,
    privateKey: result.private_key || result.privateKey, // ⚠️ Only returned once!
  };

  // Store credentials securely in system keyring
  try {
    await storeCredentials({
      agentId: agentRegistration.id,
      apiKey: agentRegistration.apiKey,
      privateKey: Buffer.from(agentRegistration.privateKey || '', 'base64'),
    });
  } catch (error) {
    console.warn('Warning: Could not store credentials in system keyring:', error);
    console.warn('You will need to manage credentials manually.');
  }

  return agentRegistration;
}

/**
 * SecureWithOAuth is an alias for registerAgentWithOAuth
 */
export async function secureWithOAuth(
  apiUrl: string,
  options: RegisterOptions
): Promise<AgentRegistration> {
  return registerAgentWithOAuth(apiUrl, options);
}

/**
 * Register an agent using OAuth authentication
 * Opens browser for OAuth consent and completes registration
 */
export async function registerAgentWithOAuth(
  apiUrl: string,
  options: RegisterOptions
): Promise<AgentRegistration> {
  const { name, type = 'ai_agent', oauthProvider, redirectUrl = 'http://localhost:8080/callback' } = options;

  if (!oauthProvider) {
    throw new Error('OAuth provider is required for OAuth registration');
  }

  // Get OAuth configuration from environment
  const clientId = process.env[`${oauthProvider.toUpperCase()}_CLIENT_ID`];
  const clientSecret = process.env[`${oauthProvider.toUpperCase()}_CLIENT_SECRET`];

  if (!clientId || !clientSecret) {
    throw new Error(`Missing OAuth credentials for ${oauthProvider}. Set environment variables.`);
  }

  // Generate authorization URL
  const authUrl = getAuthorizationUrl(oauthProvider, clientId, redirectUrl);
  console.log(`🔐 Opening browser for authorization...`);

  // Open browser
  await openBrowser(authUrl);

  // Wait for OAuth callback
  const code = await startCallbackServer(8080);

  // Exchange code for access token
  const token = await exchangeCodeForToken(
    oauthProvider,
    code,
    clientId,
    clientSecret,
    redirectUrl
  );

  // Generate Ed25519 keypair
  const keyPair = KeyPair.generate();

  // Prepare registration payload with OAuth token
  const payload: Record<string, any> = {
    name,
    type,
    public_key: keyPair.publicKeyBase64(),
    oauth_provider: oauthProvider,
    oauth_token: token.accessToken,
  };

  // Sign the payload
  const signature = keyPair.signPayload(payload);
  payload.signature = signature;

  // Send registration request with OAuth token
  const response = await axios.post(`${apiUrl}/api/v1/agents/register`, payload, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token.accessToken}`,
    },
  });

  const result: AgentRegistration = response.data;

  // Store credentials and OAuth token
  await storeCredentials({
    agentId: result.id,
    apiKey: result.apiKey,
    privateKey: keyPair.privateKey,
  });

  await storeOAuthToken(token.accessToken);

  return result;
}
