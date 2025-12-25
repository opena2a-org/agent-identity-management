/**
 * OAuth Token Management for AIM SDK
 */

import type { TokenResponse, AgentCredentials } from '../types';
import { createRequestSignature, toBase64, fromBase64 } from '../crypto/ed25519';

/**
 * Token cache entry
 */
interface CachedToken {
  accessToken: string;
  expiresAt: number;
}

/**
 * OAuth Token Manager
 * Handles token acquisition, caching, and refresh
 */
export class OAuthTokenManager {
  private readonly baseUrl: string;
  private readonly agentId: string;
  private readonly privateKey: Uint8Array;
  private cachedToken: CachedToken | null = null;
  private refreshPromise: Promise<string> | null = null;

  constructor(baseUrl: string, credentials: AgentCredentials) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.agentId = credentials.agentId;
    this.privateKey = fromBase64(credentials.privateKey);
  }

  /**
   * Get a valid access token, refreshing if necessary
   */
  async getAccessToken(): Promise<string> {
    // Check if we have a valid cached token
    if (this.cachedToken && this.cachedToken.expiresAt > Date.now() + 60000) {
      return this.cachedToken.accessToken;
    }

    // Prevent concurrent refresh requests
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = this.acquireToken();
    try {
      const token = await this.refreshPromise;
      return token;
    } finally {
      this.refreshPromise = null;
    }
  }

  /**
   * Acquire a new access token using client credentials grant
   */
  private async acquireToken(): Promise<string> {
    const tokenEndpoint = `${this.baseUrl}/oauth/token`;
    const timestamp = Date.now();

    // Create signed assertion
    const { signature } = await createRequestSignature(
      this.privateKey,
      'POST',
      '/oauth/token',
      undefined,
      timestamp
    );

    // Create JWT-like client assertion
    const header = toBase64(
      new TextEncoder().encode(JSON.stringify({ alg: 'EdDSA', typ: 'JWT' }))
    );
    const payload = toBase64(
      new TextEncoder().encode(
        JSON.stringify({
          iss: this.agentId,
          sub: this.agentId,
          aud: this.baseUrl,
          iat: Math.floor(timestamp / 1000),
          exp: Math.floor(timestamp / 1000) + 300,
        })
      )
    );
    const clientAssertion = `${header}.${payload}.${signature}`;

    const response = await fetch(tokenEndpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({
        grant_type: 'client_credentials',
        client_id: this.agentId,
        client_assertion_type: 'urn:ietf:params:oauth:client-assertion-type:jwt-bearer',
        client_assertion: clientAssertion,
      }),
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Failed to acquire token: ${response.status} ${error}`);
    }

    const tokenResponse = (await response.json()) as TokenResponse;

    // Cache the token
    this.cachedToken = {
      accessToken: tokenResponse.accessToken,
      expiresAt: Date.now() + tokenResponse.expiresIn * 1000,
    };

    return tokenResponse.accessToken;
  }

  /**
   * Clear the cached token
   */
  clearToken(): void {
    this.cachedToken = null;
  }

  /**
   * Check if we have a valid cached token
   */
  hasValidToken(): boolean {
    return this.cachedToken !== null && this.cachedToken.expiresAt > Date.now() + 60000;
  }
}

/**
 * Load credentials from environment variables
 */
export function loadCredentialsFromEnv(): AgentCredentials | null {
  const agentId = process.env.AIM_AGENT_ID;
  const privateKey = process.env.AIM_PRIVATE_KEY;
  const publicKey = process.env.AIM_PUBLIC_KEY;
  const organizationId = process.env.AIM_ORGANIZATION_ID;

  if (!agentId || !privateKey || !publicKey || !organizationId) {
    return null;
  }

  return {
    agentId,
    privateKey,
    publicKey,
    organizationId,
    createdAt: new Date().toISOString(),
  };
}

/**
 * Load credentials from a file
 */
export async function loadCredentialsFromFile(filePath: string): Promise<AgentCredentials> {
  const fs = await import('fs/promises');
  const content = await fs.readFile(filePath, 'utf-8');
  return JSON.parse(content) as AgentCredentials;
}

/**
 * Save credentials to a file
 */
export async function saveCredentialsToFile(
  credentials: AgentCredentials,
  filePath: string
): Promise<void> {
  const fs = await import('fs/promises');
  const path = await import('path');

  // Ensure directory exists
  const dir = path.dirname(filePath);
  await fs.mkdir(dir, { recursive: true });

  // Write credentials with restrictive permissions
  await fs.writeFile(filePath, JSON.stringify(credentials, null, 2), {
    mode: 0o600,
  });
}
