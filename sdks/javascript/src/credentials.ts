import * as keytar from 'keytar';
import { encodePrivateKey, decodePrivateKey } from './signing';

const SERVICE_NAME = 'aim_sdk';

export interface Credentials {
  agentId: string;
  apiKey: string;
  privateKey?: Uint8Array;
}

/**
 * Store credentials securely in system keyring
 * - macOS: Keychain
 * - Windows: Credential Locker
 * - Linux: Secret Service (GNOME Keyring, KWallet)
 */
export async function storeCredentials(credentials: Credentials): Promise<void> {
  await keytar.setPassword(SERVICE_NAME, 'agent_id', credentials.agentId);
  await keytar.setPassword(SERVICE_NAME, 'api_key', credentials.apiKey);

  if (credentials.privateKey) {
    const encodedKey = encodePrivateKey(credentials.privateKey);
    await keytar.setPassword(SERVICE_NAME, 'private_key', encodedKey);
  }
}

/**
 * Load credentials from system keyring
 * Returns null if no credentials are stored
 */
export async function loadCredentials(): Promise<Credentials | null> {
  const agentId = await keytar.getPassword(SERVICE_NAME, 'agent_id');
  const apiKey = await keytar.getPassword(SERVICE_NAME, 'api_key');

  if (!agentId || !apiKey) {
    return null;
  }

  const privateKeyB64 = await keytar.getPassword(SERVICE_NAME, 'private_key');
  let privateKey: Uint8Array | undefined;

  if (privateKeyB64) {
    try {
      privateKey = decodePrivateKey(privateKeyB64);
    } catch (error) {
      // Invalid private key - continue without it
      privateKey = undefined;
    }
  }

  return {
    agentId,
    apiKey,
    privateKey,
  };
}

/**
 * Clear all stored credentials from system keyring
 */
export async function clearCredentials(): Promise<void> {
  const keys = ['agent_id', 'api_key', 'private_key', 'oauth_token'];

  for (const key of keys) {
    try {
      await keytar.deletePassword(SERVICE_NAME, key);
    } catch (error) {
      // Ignore errors for missing keys
    }
  }
}

/**
 * Check if credentials are stored in keyring
 */
export async function hasCredentials(): Promise<boolean> {
  const agentId = await keytar.getPassword(SERVICE_NAME, 'agent_id');
  return agentId !== null;
}

/**
 * Get agent ID from keyring
 */
export async function getAgentId(): Promise<string | null> {
  return await keytar.getPassword(SERVICE_NAME, 'agent_id');
}

/**
 * Get API key from keyring
 */
export async function getAPIKey(): Promise<string | null> {
  return await keytar.getPassword(SERVICE_NAME, 'api_key');
}

/**
 * Store OAuth token separately
 */
export async function storeOAuthToken(token: string): Promise<void> {
  await keytar.setPassword(SERVICE_NAME, 'oauth_token', token);
}

/**
 * Get stored OAuth token
 */
export async function getOAuthToken(): Promise<string | null> {
  return await keytar.getPassword(SERVICE_NAME, 'oauth_token');
}

/**
 * Clear OAuth token from keyring
 */
export async function clearOAuthToken(): Promise<void> {
  await keytar.deletePassword(SERVICE_NAME, 'oauth_token');
}
