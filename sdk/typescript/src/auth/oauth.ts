/**
 * OAuth Token Management for AIM SDK
 */

import type { TokenResponse, AgentCredentials } from '../types';
import { sign, toBase64 } from '../crypto/ed25519';
import { decodeAgentPrivateKey } from '../crypto/agent-request';
import { AuthenticationError, ConfigurationError, parseAPIError } from '../exceptions';

/**
 * The grant the AIM token endpoint accepts (RFC 7523 §2.1). It answers any
 * other grant type, `client_credentials` included, with 400
 * `unsupported_grant_type`.
 */
const JWT_BEARER_GRANT_TYPE = 'urn:ietf:params:oauth:grant-type:jwt-bearer';

const JWT_BEARER_ASSERTION_TYPE = 'urn:ietf:params:oauth:client-assertion-type:jwt-bearer';

/**
 * Seconds the client assertion stays valid. The server refuses an assertion
 * whose `exp` is more than five minutes (plus clock skew) ahead.
 */
const ASSERTION_LIFETIME_SECONDS = 300;

/**
 * Base64url without padding (RFC 7515 §2), the encoding of every JWS segment.
 */
function toBase64Url(data: Uint8Array): string {
  return toBase64(data).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

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
  /** The configured key as given; decoded to its 32-byte seed when an assertion is signed. */
  private readonly privateKey: string;
  private cachedToken: CachedToken | null = null;
  private refreshPromise: Promise<string> | null = null;

  constructor(baseUrl: string, credentials: AgentCredentials) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.agentId = credentials.agentId;
    this.privateKey = credentials.privateKey;
  }

  /**
   * Get a valid access token, refreshing if necessary
   *
   * @param deadlineAt Optional absolute instant (`Date.now()` ms) by which the
   *   token fetch must have settled — the enforcement deadline, threaded from
   *   the verification call so the exchange spends the same budget as the
   *   verify POST rather than getting a fresh one. When omitted, the fetch is
   *   unbounded, exactly as before.
   */
  async getAccessToken(deadlineAt?: number): Promise<string> {
    // Check if we have a valid cached token
    if (this.cachedToken && this.cachedToken.expiresAt > Date.now() + 60000) {
      return this.cachedToken.accessToken;
    }

    // Prevent concurrent refresh requests
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = this.acquireToken(deadlineAt);
    try {
      const token = await this.refreshPromise;
      return token;
    } finally {
      this.refreshPromise = null;
    }
  }

  /**
   * Acquire a new access token with the JWT-bearer grant: the agent proves its
   * identity with a client assertion signed by its Ed25519 key, which the
   * server verifies against the public key registered for the agent.
   */
  private async acquireToken(deadlineAt?: number): Promise<string> {
    const tokenEndpoint = `${this.baseUrl}/api/v1/oauth/token`;
    const clientAssertion = await this.createClientAssertion(Date.now());

    // The abort fires at the time LEFT on the enforcement deadline — the same
    // deadline the verify POST that follows will finish on, never a fresh
    // budget for the exchange alone.
    let signal: AbortSignal | undefined;
    let abortTimer: ReturnType<typeof setTimeout> | undefined;
    if (deadlineAt !== undefined) {
      const controller = new AbortController();
      abortTimer = setTimeout(() => controller.abort(), Math.max(deadlineAt - Date.now(), 0));
      signal = controller.signal;
    }

    let response: Response;
    try {
      response = await fetch(tokenEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams({
          grant_type: JWT_BEARER_GRANT_TYPE,
          client_id: this.agentId,
          client_assertion_type: JWT_BEARER_ASSERTION_TYPE,
          client_assertion: clientAssertion,
        }),
        signal,
      });
    } finally {
      if (abortTimer !== undefined) {
        clearTimeout(abortTimer);
      }
    }

    if (!response.ok) {
      const raw = await response.text().catch(() => '');
      // Cap the embedded body excerpt: a proxy's HTML error page can run to
      // kilobytes, and the whole point of the message is the leading context.
      const excerpt = raw.slice(0, 500);
      let errorBody: Record<string, unknown> = {};
      try {
        errorBody = JSON.parse(raw) as Record<string, unknown>;
      } catch {
        // Non-JSON body; the excerpt in the message is all we can carry.
      }
      throw parseAPIError(
        response.status,
        { ...errorBody, message: `Failed to acquire token: ${response.status} ${excerpt}` },
        response.headers
      );
    }

    const tokenResponse = (await response.json()) as TokenResponse;
    // The server may answer in RFC 6749 snake_case ({"access_token", ...}) or
    // the SDK's historical camelCase; accept both.
    const accessToken = tokenResponse.accessToken ?? tokenResponse.access_token;
    if (typeof accessToken !== 'string' || accessToken === '') {
      throw new AuthenticationError(
        'Token endpoint returned no access token (expected "access_token" or "accessToken" in the response)'
      );
    }
    const expiresIn = Number(tokenResponse.expiresIn ?? tokenResponse.expires_in);
    // A missing or unparseable expiry must not poison the cache into NaN
    // (which makes every validity check false and forces a refetch per
    // request); fall back to a conservative 5 minutes.
    const expiresInSeconds = Number.isFinite(expiresIn) && expiresIn > 0 ? expiresIn : 300;

    // Cache the token
    this.cachedToken = {
      accessToken,
      expiresAt: Date.now() + expiresInSeconds * 1000,
    };

    return accessToken;
  }

  /**
   * Build the client assertion as a compact JWS (RFC 7515): three base64url
   * segments without padding, the Ed25519 signature taken over the ASCII bytes
   * of `header.payload`. Those are the bytes the server verifies; a signature
   * over anything else is refused.
   *
   * The signature is made with the key's 32-byte seed. The configured key may
   * be that seed or the 64-byte form AIM issues (the seed, then its public key):
   * Ed25519 signing takes the seed alone, and the signer refuses the 64 bytes as
   * given. Any other key throws `ConfigurationError` before a request is sent.
   */
  private async createClientAssertion(timestamp: number): Promise<string> {
    const { seed } = await decodeAgentPrivateKey(this.privateKey, 'no token was requested');
    const encoder = new TextEncoder();
    const iat = Math.floor(timestamp / 1000);
    const header = toBase64Url(encoder.encode(JSON.stringify({ alg: 'EdDSA', typ: 'JWT' })));
    const payload = toBase64Url(
      encoder.encode(
        JSON.stringify({
          iss: this.agentId,
          sub: this.agentId,
          aud: this.baseUrl,
          iat,
          exp: iat + ASSERTION_LIFETIME_SECONDS,
        })
      )
    );
    const signingInput = `${header}.${payload}`;
    const signature = await sign(encoder.encode(signingInput), seed);
    return `${signingInput}.${toBase64Url(signature)}`;
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
 * Load credentials from environment variables.
 *
 * All four of AIM_AGENT_ID, AIM_PRIVATE_KEY, AIM_PUBLIC_KEY and
 * AIM_ORGANIZATION_ID are required. A clean environment (none set) returns
 * null silently; a PARTIAL environment also returns null but warns naming the
 * missing variable(s), so a one-variable typo does not present as the same
 * "No credentials available" a clean environment does.
 */
function agentCredentialEnv() {
  return {
    AIM_AGENT_ID: process.env.AIM_AGENT_ID,
    AIM_PRIVATE_KEY: process.env.AIM_PRIVATE_KEY,
    AIM_PUBLIC_KEY: process.env.AIM_PUBLIC_KEY,
    AIM_ORGANIZATION_ID: process.env.AIM_ORGANIZATION_ID,
  };
}

/**
 * The agent-credential variables missing from a PARTIALLY set environment, by
 * name. Empty when none are set (a clean environment) or all four are, so a
 * caller can tell "no credentials configured" from "one variable short".
 */
export function missingAgentCredentialEnvVars(): string[] {
  const vars = agentCredentialEnv();
  const missing = Object.keys(vars).filter((name) => !vars[name as keyof typeof vars]);
  return missing.length === Object.keys(vars).length ? [] : missing;
}

export function loadCredentialsFromEnv(): AgentCredentials | null {
  const vars = agentCredentialEnv();
  const missing = Object.keys(vars).filter((name) => !vars[name as keyof typeof vars]);

  if (missing.length === Object.keys(vars).length) {
    return null;
  }
  if (missing.length > 0) {
    console.warn(
      `[AIM] Incomplete agent credentials in environment: missing ${missing.join(', ')} ` +
        '(all of AIM_AGENT_ID, AIM_PRIVATE_KEY, AIM_PUBLIC_KEY, AIM_ORGANIZATION_ID are ' +
        'required); loadCredentialsFromEnv() returns null.'
    );
    return null;
  }

  return {
    agentId: vars.AIM_AGENT_ID!,
    privateKey: vars.AIM_PRIVATE_KEY!,
    publicKey: vars.AIM_PUBLIC_KEY!,
    organizationId: vars.AIM_ORGANIZATION_ID!,
    createdAt: new Date().toISOString(),
  };
}

/**
 * Load credentials from a file
 */
export async function loadCredentialsFromFile(filePath: string): Promise<AgentCredentials> {
  const fs = await import('fs/promises');

  let content: string;
  try {
    content = await fs.readFile(filePath, 'utf-8');
  } catch (err) {
    const code = (err as NodeJS.ErrnoException)?.code;
    if (code === 'ENOENT') {
      throw new ConfigurationError(`Credential file not found: ${filePath}`, { filePath, cause: code });
    }
    throw new ConfigurationError(
      `Could not read credential file ${filePath}: ${(err as Error)?.message ?? 'unknown error'}`,
      { filePath, cause: code },
    );
  }

  try {
    return JSON.parse(content) as AgentCredentials;
  } catch (err) {
    throw new ConfigurationError(
      `Credential file ${filePath} is not valid JSON: ${(err as Error)?.message ?? 'parse error'}`,
      { filePath },
    );
  }
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
