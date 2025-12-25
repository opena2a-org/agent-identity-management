/**
 * Ed25519 cryptographic operations for AIM SDK
 * Uses @noble/ed25519 for cross-platform compatibility
 */

import * as ed from '@noble/ed25519';

/**
 * Key pair for Ed25519 signing
 */
export interface KeyPair {
  privateKey: Uint8Array;
  publicKey: Uint8Array;
}

/**
 * Generate a new Ed25519 key pair
 */
export async function generateKeyPair(): Promise<KeyPair> {
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = await ed.getPublicKeyAsync(privateKey);
  return { privateKey, publicKey };
}

/**
 * Sign a message with Ed25519 private key
 */
export async function sign(message: Uint8Array, privateKey: Uint8Array): Promise<Uint8Array> {
  return await ed.signAsync(message, privateKey);
}

/**
 * Verify an Ed25519 signature
 */
export async function verify(
  signature: Uint8Array,
  message: Uint8Array,
  publicKey: Uint8Array
): Promise<boolean> {
  try {
    return await ed.verifyAsync(signature, message, publicKey);
  } catch {
    return false;
  }
}

/**
 * Convert Uint8Array to base64 string
 */
export function toBase64(data: Uint8Array): string {
  if (typeof Buffer !== 'undefined') {
    return Buffer.from(data).toString('base64');
  }
  // Browser fallback
  return btoa(String.fromCharCode(...data));
}

/**
 * Convert base64 string to Uint8Array
 */
export function fromBase64(base64: string): Uint8Array {
  if (typeof Buffer !== 'undefined') {
    return new Uint8Array(Buffer.from(base64, 'base64'));
  }
  // Browser fallback
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/**
 * Convert Uint8Array to hex string
 */
export function toHex(data: Uint8Array): string {
  return Array.from(data)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Convert hex string to Uint8Array
 */
export function fromHex(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.slice(i, i + 2), 16);
  }
  return bytes;
}

/**
 * Create a signature for API requests
 * Format: timestamp:method:path:bodyHash
 */
export async function createRequestSignature(
  privateKey: Uint8Array,
  method: string,
  path: string,
  body?: string | object,
  timestamp?: number
): Promise<{ signature: string; timestamp: number }> {
  const ts = timestamp ?? Date.now();

  // Create body hash
  let bodyHash = '';
  if (body) {
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body);
    const encoder = new TextEncoder();
    const data = encoder.encode(bodyStr);
    // Use a simple hash for the body
    bodyHash = toHex(await hashData(data));
  }

  // Create message to sign
  const message = `${ts}:${method.toUpperCase()}:${path}:${bodyHash}`;
  const encoder = new TextEncoder();
  const messageBytes = encoder.encode(message);

  // Sign the message
  const signatureBytes = await sign(messageBytes, privateKey);
  const signature = toBase64(signatureBytes);

  return { signature, timestamp: ts };
}

/**
 * Simple hash function using SHA-256
 */
async function hashData(data: Uint8Array): Promise<Uint8Array> {
  if (typeof crypto !== 'undefined' && crypto.subtle) {
    const hashBuffer = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(hashBuffer);
  }
  // Node.js fallback
  const nodeCrypto = await import('crypto');
  const hash = nodeCrypto.createHash('sha256');
  hash.update(data);
  return new Uint8Array(hash.digest());
}
