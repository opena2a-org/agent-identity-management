import * as nacl from 'tweetnacl';
import * as util from 'tweetnacl-util';

/**
 * Ed25519 keypair
 */
export interface Ed25519Keypair {
  privateKey: Uint8Array;
  publicKey: Uint8Array;
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
