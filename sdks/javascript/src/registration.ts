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
 * Generates Ed25519 keypair, signs the request, and stores credentials securely
 */
export async function registerAgent(
  apiUrl: string,
  options: RegisterOptions
): Promise<AgentRegistration> {
  const { name, type = 'ai_agent' } = options;

  // Generate Ed25519 keypair for agent identity
  const keyPair = KeyPair.generate();

  // Prepare registration payload
  const payload: Record<string, any> = {
    name,
    type,
    public_key: keyPair.publicKeyBase64(),
  };

  // Sign the payload for cryptographic verification
  const signature = keyPair.signPayload(payload);
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
    privateKey: keyPair.privateKey,
  });

  return result;
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
