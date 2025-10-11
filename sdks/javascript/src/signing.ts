import * as nacl from 'tweetnacl';
import * as util from 'tweetnacl-util';

/**
 * Ed25519 keypair interface
 */
export interface Ed25519Keypair {
  privateKey: Uint8Array;
  publicKey: Uint8Array;
}

/**
 * Ed25519 KeyPair class for agent signing
 * Provides OOP encapsulation similar to Go SDK
 */
export class KeyPair {
  public readonly publicKey: Uint8Array;
  public readonly privateKey: Uint8Array;

  constructor(keypair: Ed25519Keypair) {
    this.publicKey = keypair.publicKey;
    this.privateKey = keypair.privateKey;
  }

  /**
   * Generate a new Ed25519 keypair
   */
  static generate(): KeyPair {
    const keypair = generateEd25519Keypair();
    return new KeyPair(keypair);
  }

  /**
   * Create keypair from base64-encoded private key
   * @param privateKeyBase64 Base64-encoded private key (64 bytes)
   */
  static fromBase64(privateKeyBase64: string): KeyPair {
    const privateKey = decodePrivateKey(privateKeyBase64);
    const publicKey = privateKey.slice(32); // Last 32 bytes are public key
    return new KeyPair({ privateKey, publicKey });
  }

  /**
   * Create keypair from raw private key bytes
   * Supports both 32-byte seed and 64-byte full private key
   * @param privateKey Private key as Uint8Array
   */
  static fromPrivateKey(privateKey: Uint8Array): KeyPair {
    if (privateKey.length === 32) {
      // 32-byte seed - derive full keypair
      const keypair = nacl.sign.keyPair.fromSeed(privateKey);
      return new KeyPair({
        privateKey: keypair.secretKey,
        publicKey: keypair.publicKey,
      });
    } else if (privateKey.length === 64) {
      // 64-byte full private key
      const publicKey = privateKey.slice(32);
      return new KeyPair({ privateKey, publicKey });
    } else {
      throw new Error(
        `Invalid private key length: expected 32 or 64 bytes, got ${privateKey.length}`
      );
    }
  }

  /**
   * Sign a message
   * @param message Message string to sign
   * @returns Base64-encoded signature
   */
  sign(message: string): string {
    const messageBytes = util.decodeUTF8(message);
    const signedMessage = nacl.sign(messageBytes, this.privateKey);
    const signature = signedMessage.slice(0, nacl.sign.signatureLength);
    return util.encodeBase64(signature);
  }

  /**
   * Verify a signature
   * @param message Original message string
   * @param signatureBase64 Base64-encoded signature
   * @returns True if signature is valid
   */
  verify(message: string, signatureBase64: string): boolean {
    try {
      const messageBytes = util.decodeUTF8(message);
      const signature = util.decodeBase64(signatureBase64);
      const signedMessage = new Uint8Array(signature.length + messageBytes.length);
      signedMessage.set(signature);
      signedMessage.set(messageBytes, signature.length);
      const opened = nacl.sign.open(signedMessage, this.publicKey);
      return opened !== null;
    } catch {
      return false;
    }
  }

  /**
   * Sign a JSON payload
   * @param payload Object to sign
   * @returns Base64-encoded signature
   */
  signPayload(payload: Record<string, any>): string {
    return signRequest(this.privateKey, payload);
  }

  /**
   * Get public key as base64 string
   */
  publicKeyBase64(): string {
    return encodePublicKey(this.publicKey);
  }

  /**
   * Get private key as base64 string
   */
  privateKeyBase64(): string {
    return encodePrivateKey(this.privateKey);
  }

  /**
   * Get 32-byte seed as base64 string
   */
  seedBase64(): string {
    const seed = this.privateKey.slice(0, 32);
    return util.encodeBase64(seed);
  }
}

/**
 * Generate Ed25519 keypair for agent signing
 * @returns Keypair with private and public keys
 */
export function generateEd25519Keypair(): Ed25519Keypair {
  const keypair = nacl.sign.keyPair();
  return {
    privateKey: keypair.secretKey,
    publicKey: keypair.publicKey,
  };
}

/**
 * Sign request data using Ed25519 private key
 * Data is converted to canonical JSON (sorted keys) for consistency
 * @param privateKey Ed25519 private key (64 bytes)
 * @param data Data to sign
 * @returns Base64-encoded signature
 */
export function signRequest(privateKey: Uint8Array, data: Record<string, any>): string {
  // Convert data to canonical JSON (sorted keys)
  const jsonString = JSON.stringify(sortObject(data));
  const message = util.decodeUTF8(jsonString);

  // Sign the message
  const signedMessage = nacl.sign(message, privateKey);

  // Extract signature (first 64 bytes)
  const signature = signedMessage.slice(0, nacl.sign.signatureLength);

  // Return base64-encoded signature
  return util.encodeBase64(signature);
}

/**
 * Verify Ed25519 signature
 * Used for testing and validation
 * @param publicKey Ed25519 public key (32 bytes)
 * @param data Original data
 * @param signatureB64 Base64-encoded signature
 * @returns True if signature is valid
 */
export function verifySignature(
  publicKey: Uint8Array,
  data: Record<string, any>,
  signatureB64: string
): boolean {
  try {
    // Convert data to canonical JSON
    const jsonString = JSON.stringify(sortObject(data));
    const message = util.decodeUTF8(jsonString);

    // Decode signature
    const signature = util.decodeBase64(signatureB64);

    // Reconstruct signed message
    const signedMessage = new Uint8Array(signature.length + message.length);
    signedMessage.set(signature);
    signedMessage.set(message, signature.length);

    // Verify signature
    const opened = nacl.sign.open(signedMessage, publicKey);
    return opened !== null;
  } catch (error) {
    return false;
  }
}

/**
 * Encode public key to base64
 * @param publicKey Ed25519 public key
 * @returns Base64-encoded public key
 */
export function encodePublicKey(publicKey: Uint8Array): string {
  return util.encodeBase64(publicKey);
}

/**
 * Decode base64-encoded public key
 * @param publicKeyB64 Base64-encoded public key
 * @returns Ed25519 public key
 */
export function decodePublicKey(publicKeyB64: string): Uint8Array {
  const decoded = util.decodeBase64(publicKeyB64);
  if (decoded.length !== nacl.sign.publicKeyLength) {
    throw new Error(
      `Invalid public key size: expected ${nacl.sign.publicKeyLength}, got ${decoded.length}`
    );
  }
  return decoded;
}

/**
 * Encode private key to base64
 * @param privateKey Ed25519 private key
 * @returns Base64-encoded private key
 */
export function encodePrivateKey(privateKey: Uint8Array): string {
  return util.encodeBase64(privateKey);
}

/**
 * Decode base64-encoded private key
 * @param privateKeyB64 Base64-encoded private key
 * @returns Ed25519 private key
 */
export function decodePrivateKey(privateKeyB64: string): Uint8Array {
  const decoded = util.decodeBase64(privateKeyB64);
  if (decoded.length !== nacl.sign.secretKeyLength) {
    throw new Error(
      `Invalid private key size: expected ${nacl.sign.secretKeyLength}, got ${decoded.length}`
    );
  }
  return decoded;
}

/**
 * Sort object keys recursively for canonical JSON
 * @param obj Object to sort
 * @returns Sorted object
 */
function sortObject(obj: any): any {
  if (obj === null || typeof obj !== 'object' || Array.isArray(obj)) {
    return obj;
  }

  const sorted: Record<string, any> = {};
  const keys = Object.keys(obj).sort();

  for (const key of keys) {
    sorted[key] = sortObject(obj[key]);
  }

  return sorted;
}
