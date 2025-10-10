import axios from 'axios';
import {
  generateEd25519Keypair,
  signRequest,
  encodePublicKey,
  encodePrivateKey,
} from './signing';
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
  oauthProvider?: OAuthProvider;
  redirectUrl?: string;
}

export interface AgentRegistration {
  id: string;
  name: string;
  apiKey: string;
  publicKey: string;
}

/**
 * Register a new agent with the AIM backend
 * Generates Ed25519 keypair, signs the request, and stores credentials securely
 */
export async function registerAgent(
  apiUrl: string,
  options: RegisterOptions
): Promise<AgentRegistration> {
  const { name, type = 'ai_agent' } = options;

  // Generate Ed25519 keypair for agent identity
  const { privateKey, publicKey } = generateEd25519Keypair();

  // Prepare registration payload
  const payload: Record<string, any> = {
    name,
    type,
    public_key: encodePublicKey(publicKey),
  };

  // Sign the payload for cryptographic verification
  const signature = signRequest(privateKey, payload);
  payload.signature = signature;

  // Send registration request
  const response = await axios.post(`${apiUrl}/api/v1/agents/register`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const result: AgentRegistration = response.data;

  // Store credentials securely in system keyring
  await storeCredentials({
    agentId: result.id,
    apiKey: result.apiKey,
    privateKey,
  });

  return result;
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
  const { privateKey, publicKey } = generateEd25519Keypair();

  // Prepare registration payload with OAuth token
  const payload: Record<string, any> = {
    name,
    type,
    public_key: encodePublicKey(publicKey),
    oauth_provider: oauthProvider,
    oauth_token: token.accessToken,
  };

  // Sign the payload
  const signature = signRequest(privateKey, payload);
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
    privateKey,
  });

  await storeOAuthToken(token.accessToken);

  return result;
}
