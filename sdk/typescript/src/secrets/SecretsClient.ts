/**
 * AIM Secrets Client - Identity-native secrets management for AI agents.
 *
 * Credentials never enter agent process memory. The SDK signs a resolution
 * request with the agent's Ed25519 key, the server performs ECDH + ChaCha20-Poly1305
 * encryption, and the SDK decrypts the response locally.
 *
 * @example
 * ```typescript
 * import { AIMClient } from '@opena2a/aim-sdk';
 *
 * const client = new AIMClient({ baseUrl: '...', apiKey: '...' });
 * await client.registerAgent({ name: 'my-agent' });
 *
 * // Resolve a credential (agent auth via Ed25519 signature)
 * const result = await client.secrets.resolve('github-creds', 'read');
 * const token = result.credential; // decrypted credential bytes
 *
 * // Namespace management (JWT/API key auth)
 * await client.secrets.createNamespace(agentId, 'github-creds', ['read']);
 * const namespaces = await client.secrets.listNamespaces(agentId);
 * ```
 */

import * as nodeCrypto from 'crypto';
import { sign, toBase64, fromBase64 } from '../crypto/ed25519';
import { SecretsError } from '../exceptions';

/** Result of a credential resolution. */
export interface ResolvedCredential {
  credential: Uint8Array;
  encryptionAlg: string;
  namespace: string;
}

/** Options for creating a namespace. */
export interface CreateNamespaceOptions {
  agentId: string;
  namespace: string;
  operations: string[];
  urlPatterns?: string[];
}

/** Options for storing/rotating a credential. */
export interface StoreCredentialOptions {
  namespaceId: string;
  encryptedBlob: Uint8Array;
  encryptionAlgorithm?: string;
  ephemeralPublicKey?: Uint8Array;
}

/** Options for querying audit logs. */
export interface AuditLogOptions {
  agentId: string;
  since?: string;
  limit?: number;
  offset?: number;
}

/**
 * Interface for the parent client that SecretsClient depends on.
 * Keeps SecretsClient decoupled from the full AIMClient type.
 */
export interface SecretsClientHost {
  /** Make an authenticated HTTP request. */
  request<T>(method: string, path: string, body?: unknown, useApiKey?: boolean): Promise<T>;
  /** Agent's Ed25519 private key (base64). */
  getPrivateKey(): string | null;
  /** Agent's Ed25519 public key (base64). */
  getPublicKey(): string | null;
}

/**
 * Identity-native secrets client.
 *
 * Handles the cryptographic resolution flow:
 * 1. Sign request with agent's Ed25519 key
 * 2. Server performs X25519 ECDH + ChaCha20-Poly1305 encryption
 * 3. Client decrypts response using Ed25519->X25519 converted private key
 */
export class SecretsClient {
  constructor(private readonly host: SecretsClientHost) {}

  // ---------------------------------------------------------------------------
  // Resolve (agent auth — Ed25519 signature)
  // ---------------------------------------------------------------------------

  /**
   * Resolve a credential from a namespace.
   *
   * The server encrypts the credential with an ephemeral X25519 key via ECDH.
   * This method decrypts the response locally. The raw credential exists only
   * in the SDK's memory, never in the agent's application memory if consumed
   * immediately and discarded.
   */
  async resolve(namespace: string, operation = 'read'): Promise<ResolvedCredential> {
    const privateKeyB64 = this.host.getPrivateKey();
    const publicKeyB64 = this.host.getPublicKey();

    if (!privateKeyB64) {
      throw new SecretsError('Ed25519 private key required for secrets resolution');
    }
    if (!publicKeyB64) {
      throw new SecretsError('Public key required for secrets resolution');
    }

    // Build nonce: RFC3339Nano:UUID (server rejects nonces older than 30s)
    const now = new Date().toISOString().replace(/\.\d{3}Z$/, '.000000000Z');
    const uuid = nodeCrypto.randomUUID();
    const nonce = `${now}:${uuid}`;

    // Sign: namespace|operation|nonce
    const message = `${namespace}|${operation}|${nonce}`;
    const encoder = new TextEncoder();
    const messageBytes = encoder.encode(message);
    const privateKey = fromBase64(privateKeyB64);
    const signatureBytes = await sign(messageBytes, privateKey);
    const signatureB64 = toBase64(signatureBytes);

    // Build request
    const payload = {
      namespace,
      operation,
      nonce,
      signature: signatureB64,
      agentPublicKey: publicKeyB64,
    };

    // POST /api/v1/secrets/resolve uses PQCAgentMiddleware (agent auth)
    const result = await this.host.request<Record<string, string>>(
      'POST',
      '/api/v1/secrets/resolve',
      payload,
    );

    // Decrypt the response
    const encryptedBlobB64 = result.encryptedBlob ?? '';
    const ephemeralPubB64 = result.ephemeralPubKey ?? '';

    if (!encryptedBlobB64 || !ephemeralPubB64) {
      throw new SecretsError('Server returned incomplete resolution response');
    }

    const encryptedBlob = fromBase64(encryptedBlobB64);
    const ephemeralPubBytes = fromBase64(ephemeralPubB64);

    const credential = this.decryptResolved(privateKey, encryptedBlob, ephemeralPubBytes);

    return {
      credential,
      encryptionAlg: result.encryptionAlg ?? '',
      namespace,
    };
  }

  /**
   * Decrypt a resolved credential using X25519 ECDH + ChaCha20-Poly1305.
   *
   * The server used:
   *   1. Generate ephemeral X25519 key pair
   *   2. Convert agent's Ed25519 pub -> X25519 pub
   *   3. ECDH(ephemeral_priv, agent_x25519_pub) -> shared_secret
   *   4. SHA-256(shared_secret) -> 256-bit key
   *   5. ChaCha20-Poly1305 encrypt: nonce (12) || ciphertext || tag (16)
   *
   * We mirror steps 1-4 but use agent's private key:
   *   1. Convert agent's Ed25519 priv -> X25519 priv
   *   2. Load ephemeral X25519 pub from response
   *   3. ECDH(agent_x25519_priv, ephemeral_pub) -> same shared_secret
   *   4. SHA-256(shared_secret) -> same 256-bit key
   *   5. ChaCha20-Poly1305 decrypt
   */
  private decryptResolved(
    ed25519PrivateKey: Uint8Array,
    ciphertext: Uint8Array,
    ephemeralPubBytes: Uint8Array,
  ): Uint8Array {
    // Convert Ed25519 private key to X25519 private key
    const x25519PrivateKey = edwardsToMontgomeryPrivate(ed25519PrivateKey);

    // Build Node.js key objects via DER encoding
    const agentX25519Priv = nodeCrypto.createPrivateKey({
      key: Buffer.concat([
        // PKCS#8 DER wrapper for X25519 private key (48 bytes total)
        Buffer.from('302e020100300506032b656e04220420', 'hex'),
        Buffer.from(x25519PrivateKey),
      ]),
      format: 'der',
      type: 'pkcs8',
    });

    const ephemeralX25519Pub = nodeCrypto.createPublicKey({
      key: Buffer.concat([
        // SubjectPublicKeyInfo DER wrapper for X25519 public key (44 bytes total)
        Buffer.from('302a300506032b656e032100', 'hex'),
        Buffer.from(ephemeralPubBytes),
      ]),
      format: 'der',
      type: 'spki',
    });

    // ECDH shared secret
    const sharedSecret = nodeCrypto.diffieHellman({
      privateKey: agentX25519Priv,
      publicKey: ephemeralX25519Pub,
    });

    // Derive key: SHA-256(shared_secret)
    const encKey = nodeCrypto.createHash('sha256').update(sharedSecret).digest();

    // ChaCha20-Poly1305 decrypt
    // Format: nonce (12 bytes) || ciphertext || tag (16 bytes)
    const NONCE_SIZE = 12;
    const TAG_SIZE = 16;

    if (ciphertext.length < NONCE_SIZE + TAG_SIZE) {
      throw new SecretsError('Encrypted blob too short to contain nonce and authentication tag');
    }

    const nonce = ciphertext.slice(0, NONCE_SIZE);
    const encryptedData = ciphertext.slice(NONCE_SIZE, ciphertext.length - TAG_SIZE);
    const authTag = ciphertext.slice(ciphertext.length - TAG_SIZE);

    try {
      const decipher = (nodeCrypto.createDecipheriv as Function)(
        'chacha20-poly1305',
        encKey,
        nonce,
        { authTagLength: TAG_SIZE },
      ) as nodeCrypto.DecipherGCM;
      decipher.setAuthTag(authTag);
      const decrypted = Buffer.concat([decipher.update(encryptedData), decipher.final()]);
      return new Uint8Array(decrypted);
    } catch (e) {
      throw new SecretsError(`Failed to decrypt credential: ${e instanceof Error ? e.message : e}`);
    }
  }

  // ---------------------------------------------------------------------------
  // Namespace CRUD (JWT/API key auth)
  // ---------------------------------------------------------------------------

  async createNamespace(
    agentId: string,
    namespace: string,
    operations: string[],
    urlPatterns: string[] = [],
  ): Promise<Record<string, unknown>> {
    return this.host.request('POST', '/api/v1/secrets/namespaces', {
      agentId,
      namespace,
      operations,
      urlPatterns,
    });
  }

  async listNamespaces(agentId: string): Promise<Record<string, unknown>[]> {
    const result = await this.host.request<{ namespaces?: Record<string, unknown>[] }>(
      'GET',
      `/api/v1/secrets/namespaces?agentId=${agentId}`,
    );
    return result.namespaces ?? [];
  }

  async getNamespace(namespaceId: string): Promise<Record<string, unknown>> {
    return this.host.request('GET', `/api/v1/secrets/namespaces/${namespaceId}`);
  }

  async deleteNamespace(namespaceId: string): Promise<Record<string, unknown>> {
    return this.host.request('DELETE', `/api/v1/secrets/namespaces/${namespaceId}`);
  }

  // ---------------------------------------------------------------------------
  // Credential storage (JWT/API key auth)
  // ---------------------------------------------------------------------------

  async storeCredential(options: StoreCredentialOptions): Promise<Record<string, unknown>> {
    const payload: Record<string, string> = {
      encryptedBlob: toBase64(options.encryptedBlob),
      encryptionAlgorithm: options.encryptionAlgorithm ?? 'X25519-ChaCha20-Poly1305',
    };
    if (options.ephemeralPublicKey) {
      payload.ephemeralPublicKey = toBase64(options.ephemeralPublicKey);
    }
    return this.host.request(
      'POST',
      `/api/v1/secrets/namespaces/${options.namespaceId}/credentials`,
      payload,
    );
  }

  async rotateCredential(options: StoreCredentialOptions): Promise<Record<string, unknown>> {
    const payload: Record<string, string> = {
      encryptedBlob: toBase64(options.encryptedBlob),
      encryptionAlgorithm: options.encryptionAlgorithm ?? 'X25519-ChaCha20-Poly1305',
    };
    if (options.ephemeralPublicKey) {
      payload.ephemeralPublicKey = toBase64(options.ephemeralPublicKey);
    }
    return this.host.request(
      'POST',
      `/api/v1/secrets/namespaces/${options.namespaceId}/rotate`,
      payload,
    );
  }

  // ---------------------------------------------------------------------------
  // Audit (JWT/API key auth)
  // ---------------------------------------------------------------------------

  async getAuditLog(options: AuditLogOptions): Promise<Record<string, unknown>[]> {
    const { agentId, since, limit = 50, offset = 0 } = options;
    let params = `agentId=${agentId}&limit=${limit}&offset=${offset}`;
    if (since) {
      params += `&since=${since}`;
    }
    const result = await this.host.request<{ entries?: Record<string, unknown>[] }>(
      'GET',
      `/api/v1/secrets/audit?${params}`,
    );
    return result.entries ?? [];
  }
}

// ---------------------------------------------------------------------------
// Ed25519 -> X25519 private key conversion
// ---------------------------------------------------------------------------

/**
 * Convert an Ed25519 private key (seed) to an X25519 private key.
 *
 * The Ed25519 seed is hashed with SHA-512, and the first 32 bytes are clamped
 * per RFC 7748 to produce the X25519 scalar. This matches Go's crypto/ecdh
 * and PyNaCl's crypto_sign_ed25519_sk_to_curve25519.
 *
 * @param ed25519Key - 32-byte Ed25519 seed OR 64-byte Ed25519 secret key (seed+pub)
 * @returns 32-byte X25519 private key (clamped scalar)
 */
export function edwardsToMontgomeryPrivate(ed25519Key: Uint8Array): Uint8Array {
  // Extract the 32-byte seed from either format
  const seed = ed25519Key.length === 64 ? ed25519Key.slice(0, 32) : ed25519Key;

  // SHA-512 hash of the seed, take first 32 bytes
  const hash = nodeCrypto.createHash('sha512').update(seed).digest();

  // Clamp per RFC 7748 section 5
  const scalar = new Uint8Array(hash.slice(0, 32));
  scalar[0] &= 248;
  scalar[31] &= 127;
  scalar[31] |= 64;

  return scalar;
}
